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
)

func isAudioContentType(contentType string) bool {
	return contentType == "audio/aac"
}

type AudioPostHandler struct {
	http.Handler

	DB            *gorm.DB
	UUIDGenerator *UUIDGenerator
}

func (this *AudioPostHandler) AudioHandler(response http.ResponseWriter, request *http.Request) {

	token := request.Context().Value("jwt").(*jwt.Token)
	uid, _ := token.Claims.(jwt.MapClaims)["uid"].(json.Number).Int64()

	out := exec.Command("ffprobe", "-print_format", "json", "-show_entries", "format=size,filename", "file.mp3")

	// open the out file for writing
	outfile, err := os.Create("./out.json")
	if err != nil {
		panic(err)
	}
	defer outfile.Close()
	out.Stdout = outfile

	err = out.Start()
	if err != nil {
		panic(err)
	}
	out.Wait()

	audioData := &AudioData{}
	audioData.getAudioData("./out.json")

	fmt.Println(audioData.Data.Path)
	fmt.Println(audioData.Data.Size)

	audio := Audio{
		Id:      this.UUIDGenerator.Get(),
		UserId:  uint64(uid),
		Size:    audioData.Data.Size,
		Path:    audioData.Data.Path,
		Created: time.Now().Format(time.RFC3339),
	}

	fmt.Println(audio)
}
