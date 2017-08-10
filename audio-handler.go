package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"github.com/dgrijalva/jwt-go"
	"github.com/jinzhu/gorm"
	log "github.com/sirupsen/logrus"
	"io"
	"io/ioutil"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
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
	// application/octet-stream - binary file without format, some encoders can record audio without mime type
	return contentType == "audio/aac" || contentType == "audio/wav" || contentType == "application/octet-stream"
}

type AudioPostHandler struct {
	http.Handler

	DB            *gorm.DB
	UUIDGenerator *UUIDGenerator
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

		log.Print(err)

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

		log.Print(err)

		return
	}
	defer buff.Close()
	defer os.Remove("/tmp/" + audioInfo.Filename)

	_, err = io.Copy(buff, multiPartFile)
	if err != nil {
		writeJSONResponse(
			response,
			http.StatusInternalServerError,
			newErrorJson(
				fmt.Sprintf("Cannot save audio file %s", audioInfo.Filename),
			),
		)

		log.Print(err)

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

		log.Print(err)

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

	cmd := exec.Command(
		"ffprobe",
		"-v",
		"error",
		"-i",
		"/tmp/"+audioInfo.Filename,
		"-print_format",
		"json",
		"-show_format",
	)

	var (
		// There are some uneeded information inside StdOut, skip it
		ffprobeStdOut bytes.Buffer
		ffprobeStdErr bytes.Buffer
	)

	cmd.Stdout = &ffprobeStdOut
	cmd.Stderr = &ffprobeStdErr

	err = cmd.Run()
	if err != nil {
		writeJSONResponse(
			response,
			http.StatusInternalServerError,
			newErrorJson(
				fmt.Sprintf("Cannot process audio file %s", audioInfo.Filename),
			),
		)

		log.Print("FFProbe error ", err)
		log.Print(string(ffprobeStdErr.Bytes()))

		return
	}

	ffprobeOutput := ffprobeStdOut.Bytes()
	ffprobeResult := AudioFFProbe{}

	err = json.Unmarshal(ffprobeOutput, &ffprobeResult)
	if err != nil {
		writeJSONResponse(
			response,
			http.StatusInternalServerError,
			newErrorJson(
				fmt.Sprintf("Error when parse %s", audioInfo.Filename),
			),
		)

		log.Print(err)
		log.Print(string(ffprobeOutput))

		return
	}

	log.Debug("Duration ", ffprobeResult.Format.Duration)

	if ffprobeResult.Format.Duration > MAX_AUDIO_LENGTH {
		writeJSONResponse(
			response,
			http.StatusBadRequest,
			newErrorJson("File length is too big"),
		)

		return
	}

	audioId := this.UUIDGenerator.Get()

	fileExt := filepath.Ext(audioInfo.Filename)
	fileName := fmt.Sprintf("%d_%d_%s.mp3", uid, audioId, strings.TrimRight(audioInfo.Filename, fileExt))

	cmd = exec.Command(
		"ffmpeg",
		"-i",
		"/tmp/"+audioInfo.Filename,
		"-c:a",
		"libmp3lame",
		"-b:a",
		"32k",
		"-ac",
		"1",
		"/tmp/"+fileName,
	)

	// We dont neeeded to catch StdOut
	var ffmpegStdErr bytes.Buffer
	cmd.Stderr = &ffmpegStdErr

	err = cmd.Run()
	if err != nil {
		writeJSONResponse(
			response,
			http.StatusInternalServerError,
			newErrorJson(
				fmt.Sprintf("Cannot process audio file %s", audioInfo.Filename),
			),
		)

		log.Print("FFMpeg error ", err)
		log.Print(string(ffmpegStdErr.Bytes()))

		return
	}

	defer os.Remove("/tmp/" + fileName)

	formattedFile, err := ioutil.ReadFile("/tmp/" + fileName)
	if err != nil {
		writeJSONResponse(
			response,
			http.StatusInternalServerError,
			newErrorJson(
				fmt.Sprintf("Cannot read file %s after proccesing", "/tmp/"+fileName),
			),
		)

		log.Print(err)

		return
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
	go this.DB.Create(audio)

	uploadAudioChannel <- AudioUploadTask{
		Buffer: formattedFile,
		Path:   "audios/" + getHashPath(formattedFile) + fmt.Sprintf("%s", fileName),
	}

	writeJSONResponse(
		response,
		http.StatusCreated,
		audio.getApiData(),
	)
}
