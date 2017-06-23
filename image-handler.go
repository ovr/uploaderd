package main

import (
	"crypto/md5"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"github.com/dgrijalva/jwt-go"
	"github.com/jinzhu/gorm"
	"github.com/kataras/iris"
	zmq "github.com/pebbe/zmq4"
	"gopkg.in/gographics/imagick.v3/imagick"
	"io/ioutil"
	"log"
	"net/http"
	"strconv"
	"time"
)

const (
	MAX_PHOTO_FILE_SIZE = 20 * 1024 * 1024

	MAX_PHOTO_WIDTH  = 6000
	MAX_PHOTO_HEIGHT = 6000

	MIN_PHOTO_WIDTH  = 200
	MIN_PHOTO_HEIGHT = 200

	// 1920x1080 FULL HD - max original photo size that We store
	MAX_ORIGINAL_PHOTO_WIDTH = 1920
	MAX_ORIGINAL_PHOTO_HEIGHT = 1080

	// 1280/720 HD
	MAX_BIG_PHOTO_WIDHT  = 1280
	MAX_BIG_PHOTO_HEIGHT = 720
)

func isImageContentType(contentType string) bool {
	return contentType == "image/png" || contentType == "image/jpeg" || contentType == "image/gif"
}

type ImagePostHandler struct {
	DB  *gorm.DB
	ZMQ *zmq.Socket
}

func (this ImagePostHandler) Serve(ctx *iris.Context) {
	var (
		albumId *uint64
	)

	token := ctx.Get("jwt").(*jwt.Token)
	uid, _ := token.Claims.(jwt.MapClaims)["uid"].(json.Number).Int64()

	aid := ctx.Request.PostFormValue("aid")
	if len(aid) > 0 {
		aid, err := strconv.ParseUint(aid, 10, 64)
		if err != nil {
			ctx.JSON(
				http.StatusBadRequest,
				newErrorJson("Invalid request parameter 'aid'"),
			)

			return
		}

		var album Album

		if this.DB.Where(&Album{Id: aid}).First(&album).RecordNotFound() {
			ctx.JSON(
				http.StatusBadRequest,
				newErrorJson("Unknown album"),
			)

			return
		}

		if album.UserId != uint64(uid) {
			ctx.JSON(
				http.StatusForbidden,
				newErrorJson("It's not your album"),
			)

			return
		}

		albumId = &aid
	}

	var buff []byte

	link := ctx.Request.PostFormValue("link")
	if len(link) > 0 {
		netClient := http.Client{
			Timeout: time.Second * 30,
		}

		resp, err := netClient.Get(link)
		if err != nil {
			ctx.JSON(
				http.StatusBadRequest,
				newErrorJson("We cannot request/download image from link"),
			)

			log.Print(err)
			return
		}

		defer resp.Body.Close()

		if resp.StatusCode != 200 {
			ctx.JSON(
				http.StatusBadRequest,
				newErrorJson(
					fmt.Sprintf("Wrong status code: %d", resp.StatusCode),
				),
			)

			return
		}

		// Can be -1, indicates that the length is unknown
		if resp.ContentLength > MAX_PHOTO_FILE_SIZE {
			ctx.JSON(
				http.StatusBadRequest,
				newErrorJson(
					fmt.Sprintf(
						"File is too big, actual: %d, max: %d",
						resp.ContentLength,
						MAX_PHOTO_FILE_SIZE,
					),
				),
			)

			return
		}

		contentType := resp.Header.Get("Content-Type")
		if !isImageContentType(contentType) {
			ctx.JSON(
				http.StatusBadRequest,
				newErrorJson(
					fmt.Sprintf("Wrong content type: %s", contentType),
				),
			)

			return
		}

		buff, err = ioutil.ReadAll(resp.Body)
		if err != nil {
			ctx.JSON(
				http.StatusBadRequest,
				newErrorJson("Cannot read file, it's not correct"),
			)

			log.Print(err)
			return
		}
	} else {
		multiPartFile, info, err := ctx.Request.FormFile("file")
		if err != nil {
			ctx.JSON(
				http.StatusBadRequest,
				newErrorJson("We cannot find upload file inside file field"),
			)

			log.Print(err)
			return
		}

		defer multiPartFile.Close()

		contentType := info.Header.Get("Content-Type")
		if !isImageContentType(contentType) {
			ctx.JSON(
				http.StatusBadRequest,
				newErrorJson(
					fmt.Sprintf("Wrong content type: %s", contentType),
				),
			)

			return
		}

		buff, err = ioutil.ReadAll(multiPartFile)
		if err != nil {
			ctx.JSON(
				http.StatusBadRequest,
				newErrorJson("Cannot read file, it's not correct"),
			)

			log.Print(err)
			return
		}
	}

	if len(buff) > MAX_PHOTO_FILE_SIZE {
		ctx.JSON(
			http.StatusBadRequest,
			newErrorJson(
				fmt.Sprintf(
					"File is too big, actual: %d, max: %d",
					len(buff),
					MAX_PHOTO_FILE_SIZE,
				),
			),
		)

		return
	}

	imageBox, err := NewImageFromByteSlice(buff)
	if err != nil {
		ctx.JSON(
			http.StatusBadRequest,
			newErrorJson(
				"Uploaded Image is not correct",
			),
		)

		log.Print(err)
		return
	}
	defer imageBox.Destroy()

	if imageBox.Width > MAX_PHOTO_WIDTH || imageBox.Height > MAX_PHOTO_HEIGHT {
		ctx.JSON(
			http.StatusBadRequest,
			newErrorJson(
				fmt.Sprintf("Image is too large, max %dx%d", MAX_PHOTO_WIDTH, MAX_PHOTO_HEIGHT),
			),
		)

		return
	}

	if imageBox.Width < MIN_PHOTO_WIDTH || imageBox.Height < MIN_PHOTO_HEIGHT {
		ctx.JSON(
			http.StatusBadRequest,
			newErrorJson(
				fmt.Sprintf("Image is too small, min %dx%d", MIN_PHOTO_WIDTH, MIN_PHOTO_HEIGHT),
			),
		)

		return
	}

	hasher := md5.New()
	hasher.Write(buff)
	hash := hex.EncodeToString(hasher.Sum(nil))
	hashPathPart := hash[0:2] + "/" + hash[2:4] + "/"

	photoId := generateUUID(this.ZMQ)

	// We should resize photo "original" if it's bigger then MAX_ORIGINAL dimensions
	imageBox.MaxDimensionResize(MAX_ORIGINAL_PHOTO_WIDTH, MAX_ORIGINAL_PHOTO_HEIGHT)

	uploadOriginalChannel <- ImageUploadTask{
		Buffer: buff,
		Path:   "orig/" + hashPathPart + fmt.Sprintf("%dx%d_%d_%d.jpg", imageBox.Width, imageBox.Height, uid, photoId),
	}

	imageBox.SetImageFormat("PJPEG")
	imageBox.SetImageCompression(imagick.COMPRESSION_JPEG)
	imageBox.SetImageCompressionQuality(85)

	imageBox.StripImage()
	imageBox.NormalizeImage()
	imageBox.FixOrientation()

	err = imageBox.SetImageInterlaceScheme(imagick.INTERLACE_JPEG)
	if err != nil {
		ctx.JSON(
			http.StatusBadRequest,
			newErrorJson(
				"Sorry, but We cannot proccess your image",
			),
		)

		return
	}

	err = imageBox.SetImageInterpolateMethod(imagick.INTERPOLATE_PIXEL_BACKGROUND)
	if err != nil {
		ctx.JSON(
			http.StatusBadRequest,
			newErrorJson(
				"Sorry, but We cannot proccess your image",
			),
		)

		return
	}

	// We should resize photo "big photo" if it's bigger then MAX_BIG_PHOTO dimensions
	imageBox.MaxDimensionResize(MAX_BIG_PHOTO_WIDHT, MAX_BIG_PHOTO_HEIGHT)

	bigWidth := imageBox.Width;
	bigHeight := imageBox.Height;

	photo := Photo{
		Id:           photoId,
		Added:        time.Now(),
		FileName:     hashPathPart + fmt.Sprintf("%dx%d_%d_%d.jpg", imageBox.Width, imageBox.Height, uid, photoId),
		Width:        imageBox.Width,
		Height:       imageBox.Height,
		UserId:       uint64(uid),
		AlbumId:      albumId,
		ThumbVersion: 0,
		ModApproved:  false,
		Hidden:       false,
	}
	go this.DB.Save(photo)

	uploadOriginalChannel <- ImageUploadTask{
		Buffer: imageBox.GetImageBlob(),
		Path:   "photos/" + hashPathPart + fmt.Sprintf("%dx%d_%d_%d.jpg", imageBox.Width, imageBox.Height, uid, photoId),
	}

	imageBox.UnsharpMaskImage(0, 0.5, 1, 0.05)

	for _, imgDim := range resizeImageDimmention {

		// Image can be horizontal, vertical or square
		// on square image, it's not needed to crop
		if imageBox.Width > imageBox.Height {
			diff := imageBox.Width - imageBox.Height
			half := int(float64(diff) / 2)

			err = imageBox.CropImage(
				uint(imageBox.Height),
				uint(imageBox.Height),
				half,
				0,
			)
			if err != nil {
				ctx.JSON(
					http.StatusBadRequest,
					newErrorJson(
						"Sorry, but We cannot proccess your image",
					),
				)

				log.Print(err)
				return
			}

		} else if imageBox.Height > imageBox.Width {
			diff := imageBox.Height - imageBox.Width
			half := int(float64(diff) / 2)

			err = imageBox.CropImage(
				uint(imageBox.Width),
				uint(imageBox.Width),
				0,
				half,
			)
			if err != nil {
				ctx.JSON(
					http.StatusBadRequest,
					newErrorJson(
						"Sorry, but We cannot proccess your image",
					),
				)

				log.Print(err)
				return
			}
		}

		err = imageBox.ThumbnailImage(imgDim.Width, imgDim.Height)
		if err != nil {
			ctx.JSON(
				http.StatusBadRequest,
				newErrorJson(
					"Sorry, but We cannot proccess your image",
				),
			)

			log.Print(err)
			return
		}

		uploadThumbnailChannel <- ImageUploadTask{
			Buffer: imageBox.GetImageBlob(),
			Path:   "thumbs/" + fmt.Sprintf(
				"%dx%d/%s/%dx%d_%d_%d.jpg",
				imgDim.Width,
				imgDim.Height,
				hashPathPart,
				bigWidth,
				bigHeight,
				uid,
				photoId,
			),
		}
	}

	ctx.JSON(
		http.StatusOK,
		photo.getApiData(),
	)
}
