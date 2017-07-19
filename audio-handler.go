package main

import (
	"encoding/json"
	"fmt"
	"github.com/dgrijalva/jwt-go"
	"github.com/jinzhu/gorm"
	zmq "github.com/pebbe/zmq4"
	"io/ioutil"
	"net/http"
	"os/exec"
	"strconv"
	"strings"
	"time"
	"os"
	"io"
)

const (
	// 3 Mb - max raw audio size that we allow upload
	MAX_AUDIO_FILE_SIZE = 1024 * 1024 * 3

	// 1 Mb - max original audio size that we store
	MAX_ORIGINAL_FILE_SIZE = 1024 * 1024

	// 5 min + 10 sec buff
	MAX_AUDIO_LENGTH = 310
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

	buff, err := os.Create("/tmp/" + audioInfo.Filename)
	if err != nil {
		writeJSONResponse(
			response,
			http.StatusInternalServerError,
			newErrorJson(
				fmt.Sprintf("Cannot create audio file %s", audioInfo.Filename),
			),
		)

		return
	}
	defer buff.Close()

	_, err = io.Copy(buff, multiPartFile)
	if err != nil {
		writeJSONResponse(
			response,
			http.StatusInternalServerError,
			newErrorJson(
				fmt.Sprintf("Cannot save audio file %s", audioInfo.Filename),
			),
		)

		return
	}

	rawFile, err := ioutil.ReadAll(buff)
	if err != nil {
		writeJSONResponse(
			response,
			http.StatusBadRequest,
			newErrorJson(
				fmt.Sprintf("Cannot read file %s", audioInfo.Filename),
			),
		)

		return
	}

	if len(rawFile) > MAX_AUDIO_FILE_SIZE {
		writeJSONResponse(
			response,
			http.StatusBadRequest,
			newErrorJson(
				fmt.Sprintf(
					"File is too big, actual: %d, max: %d",
					len(rawFile),
					MAX_AUDIO_FILE_SIZE,
				),
			),
		)

		return
	}

	cmd, err := exec.Command(
		"ffprobe",
		"-i",
		audioInfo.Filename,
		"-v",
		"quiet",
		"-show_entries",
		"format=duration",
		"-of",
		"default=noprint_wrappers=1:nokey=1",
	).Output()

	if err != nil {
		writeJSONResponse(
			response,
			http.StatusInternalServerError,
			newErrorJson(
				fmt.Sprintf("Cannot process audio file %s via ffprobe", audioInfo.Filename),
			),
		)

		return
	}

	audioDuration, err := strconv.ParseFloat(strings.Trim(string(cmd), "\n"), 32)
	if err != nil {
		writeJSONResponse(
			response,
			http.StatusInternalServerError,
			newErrorJson(
				fmt.Sprintf("Error when parse %s", audioInfo.Filename),
			),
		)
	}

	if audioDuration > MAX_AUDIO_LENGTH {
		writeJSONResponse(
			response,
			http.StatusBadRequest,
			newErrorJson("File length is too big"),
		)

		return
	}

	audioId := generateUUID(this.ZMQ)

	fileName := fmt.Sprintf("%d_%d_%s.mp3", uid, audioId, strings.TrimRight(audioInfo.Filename, ".aac"))

	exec.Command(
		"ffmpeg",
		"-v",
		"quiet",
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
		writeJSONResponse(
			response,
			http.StatusInternalServerError,
			newErrorJson(
				fmt.Sprintf("Cannot read file %s after proccesing via ffmpeg", fileName),
			),
		)
	}

	if len(formattedFile) > MAX_ORIGINAL_FILE_SIZE {
		writeJSONResponse(
			response,
			http.StatusBadRequest,
			newErrorJson(
				fmt.Sprintf(
					"File is too big, actual: %d, max: %d",
					len(formattedFile),
					MAX_ORIGINAL_FILE_SIZE,
				),
			),
		)

		return
	}

	audio := Audio{
		Id:      audioId,
		UserId:  uint64(uid),
		Path:    getHashPath(formattedFile) + fmt.Sprintf("%s", fileName),
		Created: time.Now(),
	}
	go this.DB.Save(audio)

	uploadOriginalAudioChannel <- AudioUploadTask{
		Buffer: formattedFile,
		Path:   "audios/" + getHashPath(formattedFile) + fmt.Sprintf("%s", fileName),
	}

	writeJSONResponse(
		response,
		http.StatusCreated,
		audio.getApiData(),
	)
}
