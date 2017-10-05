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
	MAX_VIDEO_LENGTH = 15*60 + 10
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
		"-select_streams", "v:0", "-show_entries", "stream=height,width",
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
	fileName := fmt.Sprintf("%d_%d_%s.mp4", uid, videoId, strings.TrimRight(videoInfo.Filename, fileExt))
	fileNameCover := fmt.Sprintf("%d_%d_%s_cover.mp4", uid, videoId, strings.TrimRight(videoInfo.Filename, fileExt))

	parameters := []string{
		"-i",
		"/tmp/" + videoInfo.Filename,
		"-vcodec", "libx264",
		"-f", "mp4",
	}

	if len(ffprobeResult.Streams) > 1 {
		if ffprobeResult.Streams[0].Width > 1920 || ffprobeResult.Streams[0].Height > 1080 {
			parameters = append(parameters, "-vf")

			if ffprobeResult.Streams[0].Rotation == 90 {
				if ffprobeResult.Streams[0].Height > ffprobeResult.Streams[0].Width {
					parameters = append(parameters, "scale='1920:1080'")
				} else {
					parameters = append(parameters, "scale='1080:1920'")
				}
			} else {
				if ffprobeResult.Streams[0].Width > ffprobeResult.Streams[0].Height {
					parameters = append(parameters, "scale='1920:1080'")
				} else {
					parameters = append(parameters, "scale='1080:1920'")
				}
			}
		}
	}

	parameters = append(parameters, "/tmp/"+fileName)
	cmd = exec.Command(
		"ffmpeg",
		parameters...,
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

	coverTime := time.Duration(int(ffprobeResult.Format.Duration/2)) * time.Second
	sec := coverTime.Seconds()
	min := coverTime.Minutes()
	if sec >= 60 {
		sec = 0
		min = min + 1
	}
	hour := coverTime.Hours()
	if min >= 60 {
		min = 0
		hour = hour + 1
	}
	parametersCover := []string{
		"-ss",
		fmt.Sprintf(
			"%02d:%02d:%02d",
			int(hour),
			int(min),
			int(sec),
		),
		"-i",
		"/tmp/" + videoInfo.Filename,
		"-y",
		"-vframes", "1",
	}
	parametersCover = append(parametersCover, "/tmp/"+fileNameCover)

	cmdCover := exec.Command("ffmpeg", parametersCover...)
	cmdCover.Stderr = &ffmpegStdErr
	log.Print(parameters)
	if err := cmdCover.Run(); err != nil {
		writeJSONResponse(
			response,
			http.StatusBadRequest,
			newErrorJson(
				"Cannot save cover video",
			),
		)

		log.Print("FFMpeg error ", err)
		log.Print(string(ffmpegStdErr.Bytes()))

		return
	}

	defer os.Remove("/tmp/" + fileName)
	defer os.Remove("/tmp/" + fileNameCover)

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
		Id:       videoId,
		UserId:   uint64(uid),
		Path:     getHashPath(formattedFile) + fmt.Sprintf("%s", fileName),
		Preview:  getHashPath(formattedFile) + fmt.Sprintf("%s", fileNameCover),
		Duration: uint64(ffprobeResult.Format.Duration),
		Created:  time.Now(),
	}
	go this.DB.Create(video)

	uploadVideoChannel <- VideoUploadTask{
		VideoId: 0, // not used here
		Buffer:  formattedFile,
		Path:    "videos/" + getHashPath(formattedFile) + fmt.Sprintf("%s", fileNameCover),
	}

	uploadVideoChannel <- VideoUploadTask{
		VideoId: videoId, // needed to mark processing complete
		Buffer:  formattedFile,
		Path:    "videos/" + getHashPath(formattedFile) + fmt.Sprintf("%s", fileName),
	}

	writeJSONResponse(
		response,
		http.StatusCreated,
		video.getApiData(),
	)
}
