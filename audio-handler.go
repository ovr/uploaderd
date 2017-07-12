package main

import (
	"encoding/json"
	"fmt"
	"github.com/dgrijalva/jwt-go"
	"github.com/jinzhu/gorm"
	"net/http"
	"os"
	"os/exec"
	"time"
	"io/ioutil"
	zmq "github.com/pebbe/zmq4"
	"crypto/md5"
	"encoding/hex"
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

	// TODO: Move args to config ?
	file := exec.Command("ffprobe", "-print_format", "json", "-show_entries", "format=size,filename", audioInfo.Filename)

	// open the out file for writing
	outfile, err := os.Create("./audio_data.json")
	if err != nil {
		panic(err)
	}
	defer outfile.Close()
	file.Stdout = outfile

	err = file.Start()
	if err != nil {
		panic(err)
	}
	file.Wait()

	audioData := &AudioData{}
	audioData.getAudioData("./audio_data.json")

	// TODO: btw, extract to utils ?
	hasher := md5.New()
	hasher.Write(buff)
	hash := hex.EncodeToString(hasher.Sum(nil))
	hashPathPart := hash[0:2] + "/" + hash[2:4] + "/"

	audioId := generateUUID(this.ZMQ)

	audio := Audio{
		Id:      audioId,
		UserId:  uint64(uid),
		Size:    audioData.Data.Size,
		Path:    hashPathPart + fmt.Sprintf("%s_%d_%d", audioInfo.Filename, uid, audioId),
		Created: time.Now().Format(time.RFC3339),
	}
	go this.DB.Save(audio)

	writeJSONResponse(
		response,
		http.StatusCreated,
		audio.getApiData(),
	)
}