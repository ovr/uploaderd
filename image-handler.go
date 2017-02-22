package main

import (
	"crypto/md5"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"github.com/dgrijalva/jwt-go"
	"github.com/jinzhu/gorm"
	"github.com/kataras/iris"
	"gopkg.in/gographics/imagick.v3/imagick"
	"io/ioutil"
	"log"
	"net/http"
	"time"
)

const (
	MAX_PHOTO_FILE_SIZE = 20 * 1024 * 1024

	MAX_PHOTO_WIDTH  = 6000
	MAX_PHOTO_HEIGHT = 6000

	MIN_PHOTO_WIDTH  = 200
	MIN_PHOTO_HEIGHT = 200

	// 1920x1080 FULL HD - max original photo size that We store
	RESIZE_PHOTO_WIDTH  = 1920
	RESIZE_PHOTO_HEIGHT = 1080

	// 1280/720 HD
	MAX_BIG_PHOTO_WIDHT  = 1280
	MAX_BIG_PHOTO_HEIGHT = 720
)

func isImageContentType(contentType string) bool {
	return contentType == "image/png" || contentType == "image/jpeg" || contentType == "image/gif"
}

type ImagePostHandler struct {
	DB *gorm.DB
}

func (this ImagePostHandler) Serve(ctx *iris.Context) {
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

	token := ctx.Get("jwt").(*jwt.Token)
	uid, _ := token.Claims.(jwt.MapClaims)["uid"].(json.Number).Int64()

	hasher := md5.New()
	hasher.Write(buff)
	hash := hex.EncodeToString(hasher.Sum(nil))
	hashPathPart := hash[0:2] + "/" + hash[2:4] + "/"

	photoId := generateUUID(zmqClient)

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

	if imageBox.Width > imageBox.Height {
		if imageBox.Width > MAX_BIG_PHOTO_WIDHT {
			proportion := float64(imageBox.Height) / float64(imageBox.Width)
			imageBox.ResizeImage(MAX_BIG_PHOTO_WIDHT, uint(MAX_BIG_PHOTO_WIDHT*proportion))
		}
	} else {
		if imageBox.Height > MAX_BIG_PHOTO_HEIGHT {
			proportion := float64(imageBox.Width) / float64(imageBox.Height)
			imageBox.ResizeImage(uint(MAX_BIG_PHOTO_HEIGHT*proportion), MAX_BIG_PHOTO_HEIGHT)
		}
	}

	photo := Photo{
		Id:           photoId,
		Added:        time.Now(),
		FileName:     hashPathPart + fmt.Sprintf("%dx%d_%d_%d.jpg", imageBox.Width, imageBox.Height, uid, photoId),
		Width:        imageBox.Width,
		Height:       imageBox.Height,
		UserId:       uint64(uid),
		ThumbVersion: 0,
		ModApproved:  false,
		Hidden:       false,
	}
	go this.DB.Save(photo)

	uploadOriginalChannel <- ImageUploadTask{
		Buffer: imageBox.GetImageBlob(),
		Path:   "photo/" + hashPathPart + fmt.Sprintf("%dx%d_%d_%d.jpg", imageBox.Width, imageBox.Height, uid, photoId),
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
			Path:   "photos/" + hashPathPart + fmt.Sprintf("%dx%d_%d_%d.jpg", imgDim.Width, imgDim.Height, uid, photoId),
		}
	}

	ctx.JSON(
		http.StatusOK,
		photo.getApiData(),
	)
}
