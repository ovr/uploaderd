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
	// 1Gb - max raw video size that we allow upload
	MAX_VIDEO_FILE_SIZE = 1024 * 1024 * 1024

	// 100 Mb - max original video size that we store
	MAX_ORIGINAL_VIDEO_FILE_SIZE = 1024 * 1024 * 100

	// 15 min + 10 sec buff
	MAX_VIDEO_LENGTH = 1510
)

func isVideoContentType(contentType string) bool {
	// application/octet-stream - binary file without format, some encoders can record video without mime type
	return contentType == "video/mpeg" || contentType == "video/mp4" || contentType == "video/ogg" || contentType == "video/quicktime" || contentType == "video/webm" || contentType == "video/x-ms-wmv" || contentType == "video/x-flv" || contentType == "video/3gpp" || contentType == "video/3gpp2" || contentType == "application/octet-stream"
}

type VideoPostHandler struct {
	http.Handler

	DB            *gorm.DB
	UUIDGenerator *UUIDGenerator
}

func (this VideoPostHandler) ServeHTTP(response http.ResponseWriter, request *http.Request) {

	token := request.Context().Value("jwt").(*jwt.Token)
	uid, _ := token.Claims.(jwt.MapClaims)["uid"].(json.Number).Int64()

	multiPartFile, videoInfo, err := request.FormFile("file")
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

	contentType := videoInfo.Header.Get("Content-Type")
	if !isVideoContentType(contentType) {
		writeJSONResponse(
			response,
			http.StatusBadRequest,
			newErrorJson(
				fmt.Sprintf("Wrong content type: %s", contentType),
			),
		)

		return
	}

	buff, err := os.Create("/tmp/" + videoInfo.Filename)
	if err != nil {
		writeJSONResponse(
			response,
			http.StatusInternalServerError,
			newErrorJson(
				fmt.Sprintf("Cannot create video file %s", videoInfo.Filename),
			),
		)

		log.Print(err)

		return
	}
	defer buff.Close()
	defer os.Remove("/tmp/" + videoInfo.Filename)

	_, err = io.Copy(buff, multiPartFile)
	if err != nil {
		writeJSONResponse(
			response,
			http.StatusInternalServerError,
			newErrorJson(
				fmt.Sprintf("Cannot save video file %s", videoInfo.Filename),
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
				fmt.Sprintf("Cannot read file %s", videoInfo.Filename),
			),
		)

		log.Print(err)

		return
	}

	if len(rawFile) > MAX_VIDEO_FILE_SIZE {
		writeJSONResponse(
			response,
			http.StatusBadRequest,
			newErrorJson(
				fmt.Sprintf(
					"File is too big, actual: %d, max: %d",
					len(rawFile),
					MAX_VIDEO_FILE_SIZE,
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
		"/tmp/"+videoInfo.Filename,
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
				fmt.Sprintf("Cannot process video file %s", videoInfo.Filename),
			),
		)

		log.Print("FFProbe error ", err)
		log.Print(string(ffprobeStdErr.Bytes()))

		return
	}

	ffprobeOutput := ffprobeStdOut.Bytes()
	ffprobeResult := VideoFFProbe{}

	err = json.Unmarshal(ffprobeOutput, &ffprobeResult)
	if err != nil {
		writeJSONResponse(
			response,
			http.StatusInternalServerError,
			newErrorJson(
				fmt.Sprintf("Error when parse %s", videoInfo.Filename),
			),
		)

		log.Print(err)
		log.Print(string(ffprobeOutput))

		return
	}

	log.Debug("Duration ", ffprobeResult.Format.Duration)

	if ffprobeResult.Format.Duration > MAX_VIDEO_LENGTH {
		writeJSONResponse(
			response,
			http.StatusBadRequest,
			newErrorJson("File length is too big"),
		)

		return
	}

	videoId := this.UUIDGenerator.Get()

	fileExt := filepath.Ext(videoInfo.Filename)
	fileName := fmt.Sprintf("%d_%d_%s.mp3", uid, videoId, strings.TrimRight(videoInfo.Filename, fileExt))

	cmd = exec.Command(
		"ffmpeg",
		"-i",
		"/tmp/"+videoInfo.Filename,
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
				fmt.Sprintf("Cannot process video file %s", videoInfo.Filename),
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

	if len(formattedFile) > MAX_ORIGINAL_VIDEO_FILE_SIZE {
		writeJSONResponse(
			response,
			http.StatusBadRequest,
			newErrorJson(
				fmt.Sprintf(
					"File is too big, actual: %d, max: %d",
					len(formattedFile),
					MAX_ORIGINAL_VIDEO_FILE_SIZE,
				),
			),
		)

		return
	}

	video := Video{
		Id:      videoId,
		UserId:  uint64(uid),
		Path:    getHashPath(formattedFile) + fmt.Sprintf("%s", fileName),
		Created: time.Now(),
	}
	go this.DB.Create(video)

	uploadVideoChannel <- VideoUploadTask{
		Buffer: formattedFile,
		Path:   "videos/" + getHashPath(formattedFile) + fmt.Sprintf("%s", fileName),
	}

	writeJSONResponse(
		response,
		http.StatusCreated,
		video.getApiData(),
	)
}
