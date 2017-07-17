package main

import (
	"encoding/json"
	"fmt"
	"github.com/dgrijalva/jwt-go"
	"github.com/jinzhu/gorm"
	"net/http"
	"time"
	"io/ioutil"
	zmq "github.com/pebbe/zmq4"
	"os/exec"
	"strings"
)

const (
	MAX_AUDIO_FILE_SIZE = 1024 * 1024
)

func isAudioContentType(contentType string) bool {
	return contentType == "audio/aac"
}

type AudioPostHandler struct {
	http.Handler

	DB  *gorm.DB
	ZMQ *zmq.Socket
}

func (this AudioPostHandler) ServeHTTP(response http.ResponseWriter, request *http.Request) {

	token := request.Context().Value("jwt").(*jwt.Token)
	uid, _ := token.Claims.(jwt.MapClaims)["uid"].(json.Number).Int64()

	var buff []byte

	multiPartFile, audioInfo, err := request.FormFile("file")
	if err != nil {
		writeJSONResponse(
			response,
			http.StatusBadRequest,
			newErrorJson("We cannot find upload file inside file field"),
		)

		return
	}

	defer multiPartFile.Close()

	contentType := audioInfo.Header.Get("Content-Type")
	if !isAudioContentType(contentType) {
		writeJSONResponse(
			response,
			http.StatusBadRequest,
			newErrorJson(
				fmt.Sprintf("Wrong content type: %s", contentType),
			),
		)

		return
	}

	buff, err = ioutil.ReadAll(multiPartFile)
	if err != nil {
		writeJSONResponse(
			response,
			http.StatusBadRequest,
			newErrorJson("Cannot read file"))

		return
	}

	if len(buff) > MAX_AUDIO_FILE_SIZE {
		writeJSONResponse(
			response,
			http.StatusBadRequest,
			newErrorJson(
				fmt.Sprintf(
					"File is too big, actual: %d, max: %d",
					len(buff),
					MAX_AUDIO_FILE_SIZE,
				),
			),
		)

		return
	}

	audioId := generateUUID(this.ZMQ)

	fileName := fmt.Sprintf("%d_%d_%s.mp3", uid, audioId, strings.TrimRight(audioInfo.Filename, ".aac"))

	exec.Command(
		"ffmpeg",
		"-i",
		audioInfo.Filename,
		"-c:a",
		"libmp3lame",
		"-b:a",
		"32k",
		"-ac",
		"1",
		fileName,
	).Run()

	formattedFile, err := ioutil.ReadFile(fileName)
	if err != nil {
		panic(err)
	}

	audio := Audio{
		Id:      audioId,
		UserId:  uint64(uid),
		Size:    len(formattedFile),
		Path:    getHashPath(buff) + fmt.Sprintf("%s", fileName),
		Created: time.Now(),
	}
	go this.DB.Save(audio)

	writeJSONResponse(
		response,
		http.StatusCreated,
		audio.getApiData(),
	)
}