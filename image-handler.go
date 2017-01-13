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
)

func isImageContentType(contentType string) bool {
	return contentType == "image/png" || contentType == "image/jpeg" || contentType == "image/gif"
}

type ImagePostHandler struct {

}

func (m ImagePostHandler) Serve(ctx *iris.Context) {

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

	photoId := generateUUID(zmqClient);

	hasher := md5.New()
	hasher.Write(buff)
	hash := hex.EncodeToString(hasher.Sum(nil))

	hashPathPart := hash[0:2] + "/" + hash[2:4] + "/";

	uploadOriginalChannel <- ImageUploadTask{
		Buffer: buff,
		Path: "orig/" + hashPathPart + fmt.Sprintf("%dx%d_%d_%d.jpg", width, height, uid, photoId),
	}

	imageBox.FixOrientation()
	imageBox.ResizeImage(500, 500)

	uploadOriginalChannel <- ImageUploadTask{
		Buffer: imageBox.GetImageBlob(),
		Path: "photo/" + hashPathPart + fmt.Sprintf("%dx%d_%d_%d.jpg", 500, 500, uid, photoId),
	}

	for _, imgDim := range resizeImageDimmention {
		err = imageBox.ThumbnailImage(imgDim.Width, imgDim.Height);
		if err != nil {
			panic(err)
		}

		uploadThumbnailChannel <- ImageUploadTask{
			Buffer: imageBox.GetImageBlob(),
			Path: "photos/" + hashPathPart + fmt.Sprintf("%dx%d_%d_%d.jpg", width, height, uid, photoId),
		}
	}

	ctx.JSON(
		http.StatusOK,
		&ImageJson{
			Id: photoId,
		},
	)
}
