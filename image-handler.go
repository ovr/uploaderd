package main

import (
	"log"
	"crypto/md5"
	"encoding/hex"
	"fmt"
	"io/ioutil"
	"github.com/kataras/iris"
	"net/http"
	"github.com/dgrijalva/jwt-go"
	"encoding/json"
	"github.com/jinzhu/gorm"
	"time"
	"gopkg.in/gographics/imagick.v3/imagick"
)

const (
	MAX_PHOTO_WIDTH = 6000
	MAX_PHOTO_HEIGHT = 6000

	// 1920x1080 FULL HD - max original photo size that We store
	RESIZE_PHOTO_WIDHT = 1920
	RESIZE_PHOTO_HEIGHT = 1080

	// 1280/720 HD
	MAX_BIG_PHOTO_WIDHT = 1280
	MAX_BIG_PHOTO_HEIGHT = 720
)

func isImageContentType(contentType string) bool {
	return contentType == "image/png" || contentType == "image/jpeg" || contentType == "image/gif"
}

type ImagePostHandler struct {
	DB *gorm.DB
}

func (this ImagePostHandler) Serve(ctx *iris.Context) {

	multiPartFile, info, err := ctx.Request.FormFile("file")
	if err != nil {
		ctx.JSON(
			http.StatusBadRequest,
			newErrorJson("We cannot find upload file inside file field"),
		)

		log.Print(err)
		return;
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

		return;
	}

	buff, err := ioutil.ReadAll(multiPartFile)
	if err != nil {
		ctx.JSON(
			http.StatusBadRequest,
			newErrorJson("Cannot read file, it's not correct"),
		)

		log.Print(err)
		return;
	}

	imageBox, err := NewImageFromByteSlice(buff)
	if err != nil {
		ctx.SetStatusCode(http.StatusBadRequest);
		ctx.WriteString("Uploaded Image is not correct")

		log.Print(err)
		return;
	}
	defer imageBox.Destroy();

	if imageBox.Width > MAX_PHOTO_WIDTH || imageBox.Height > MAX_PHOTO_HEIGHT {
		ctx.SetStatusCode(http.StatusBadRequest);
		ctx.WriteString(
			fmt.Sprintf("Image is large, max %dx%d", MAX_PHOTO_WIDTH, MAX_PHOTO_HEIGHT),
		)
		return;
	}

	token := ctx.Get("jwt").(*jwt.Token)
	uid, _ := token.Claims.(jwt.MapClaims)["uid"].(json.Number).Int64()

	hasher := md5.New()
	hasher.Write(buff)
	hash := hex.EncodeToString(hasher.Sum(nil))
	hashPathPart := hash[0:2] + "/" + hash[2:4] + "/";

	photoId := generateUUID(zmqClient);

	uploadOriginalChannel <- ImageUploadTask{
		Buffer: buff,
		Path:   "orig/" + hashPathPart + fmt.Sprintf("%dx%d_%d_%d.jpg", imageBox.Width, imageBox.Height, uid, photoId),
	}

	imageBox.SetImageFormat("PJPEG")
	imageBox.SetImageCompression(imagick.COMPRESSION_JPEG)
	imageBox.SetImageCompressionQuality(85)

	imageBox.StripImage();
	imageBox.NormalizeImage();
	imageBox.FixOrientation();

	err = imageBox.SetImageInterlaceScheme(imagick.INTERLACE_JPEG);
	if err != nil {
		log.Print(err)
	}

	err = imageBox.SetImageInterpolateMethod(imagick.INTERPOLATE_PIXEL_BACKGROUND);
	if err != nil {
		log.Print(err)
	}

	if (imageBox.Width > imageBox.Height) {
		if (imageBox.Width > MAX_BIG_PHOTO_WIDHT) {
			proportion := float64(imageBox.Height) / float64(imageBox.Width);
			imageBox.ResizeImage(MAX_BIG_PHOTO_WIDHT, uint(MAX_BIG_PHOTO_WIDHT * proportion))
		}
	} else {
		if (imageBox.Height > MAX_BIG_PHOTO_HEIGHT) {
			proportion := float64(imageBox.Width) / float64(imageBox.Height);
			imageBox.ResizeImage(uint(MAX_BIG_PHOTO_HEIGHT * proportion), MAX_BIG_PHOTO_HEIGHT)
		}
	}

	photo := Photo{
		Id:photoId,
		Added:time.Now(),
		FileName: hashPathPart + fmt.Sprintf("%dx%d_%d_%d.jpg", imageBox.Width, imageBox.Height, uid, photoId),
		Width: imageBox.Width,
		Height: imageBox.Height,
		UserId: uint64(uid),
		ThumbVersion: 0,
		ModApproved: false,
		Hidden: false,
	}
	go this.DB.Save(photo);

	uploadOriginalChannel <- ImageUploadTask{
		Buffer: imageBox.GetImageBlob(),
		Path:   "photo/" + hashPathPart + fmt.Sprintf("%dx%d_%d_%d.jpg", imageBox.Width, imageBox.Height, uid, photoId),
	}

	imageBox.UnsharpMaskImage(0, 0.5, 1, 0.05);

	for _, imgDim := range resizeImageDimmention {
		err = imageBox.ThumbnailImage(imgDim.Width, imgDim.Height);
		if err != nil {
			panic(err)
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
