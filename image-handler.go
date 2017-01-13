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

	width := imageBox.Width;
	height := imageBox.Height;
	token := ctx.Get("jwt").(*jwt.Token)
	uid, _ := token.Claims.(jwt.MapClaims)["uid"].(json.Number).Int64()

	hasher := md5.New()
	hasher.Write(buff)
	hash := hex.EncodeToString(hasher.Sum(nil))
	hashPathPart := hash[0:2] + "/" + hash[2:4] + "/";

	photoId := generateUUID(zmqClient);
	photo := Photo{
		Id:photoId,
		Added:time.Now(),
		FileName: hashPathPart + fmt.Sprintf("%dx%d_%d_%d.jpg", 500, 500, uid, photoId),
		Width: 500,
		Height: 500,
		UserId: uint64(uid),
		ThumbVersion: 0,
		ModApproved: false,
		Hidden: false,
	}
	go this.DB.Save(photo);

	uploadOriginalChannel <- ImageUploadTask{
		Buffer: buff,
		Path:   "orig/" + hashPathPart + fmt.Sprintf("%dx%d_%d_%d.jpg", width, height, uid, photoId),
	}

	imageBox.SetImageCompressionQuality(80)
	imageBox.FixOrientation()
	imageBox.ResizeImage(500, 500)

	uploadOriginalChannel <- ImageUploadTask{
		Buffer: imageBox.GetImageBlob(),
		Path:   "photo/" + hashPathPart + fmt.Sprintf("%dx%d_%d_%d.jpg", 500, 500, uid, photoId),
	}

	for _, imgDim := range resizeImageDimmention {
		imageBox.NormalizeImage();
		imageBox.UnsharpMaskImage(0, 0.5, 1, 0.05);

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
