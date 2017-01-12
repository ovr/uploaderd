package main

import (
	"github.com/donutloop/mux"
	"net/http"
	"fmt"
	"os"
	"io/ioutil"
	"encoding/json"
	"gopkg.in/gographics/imagick.v3/imagick" // v3 for 7+
	zmq "github.com/pebbe/zmq4"
	"strconv"
	"crypto/md5"
	"encoding/hex"
	"log"
)

type ErrorJson struct {
	Error map[string]string `json:"error"`
}

type ImageJson struct {
	Id uint64 `json:"id"`
}


func ErrorResponse(rw http.ResponseWriter, message string, args ...interface{}) {
	data := &ErrorJson{make(map[string]string)}
	data.Error["message"] = fmt.Sprintf(message, args...)
	resp, _ := json.Marshal(data)

	rw.Write(resp)
	//http.Error(rw, "Internal Server Error", http.StatusInternalServerError)
}

func SuccessResponse(rw http.ResponseWriter, result interface{}) {
	resp, _ := json.Marshal(result)

	rw.Write(resp)
}

type ImageDim struct {
	Width uint
	Height uint
}

func isImageContentType(contentType string) bool {
	return contentType == "image/png" || contentType == "image/jpeg" || contentType == "image/gif"
}

func uploadImageHandler(rw http.ResponseWriter, req *http.Request) {
	multiPartFile, info, err := req.FormFile("file")
	if err != nil {
		ErrorResponse(rw, "We cannot find upload file inside file field")
		log.Print(err)

		return;
	}

	defer multiPartFile.Close()

	contentType := info.Header.Get("Content-Type")
	if !isImageContentType(contentType) {
		ErrorResponse(rw, fmt.Sprintf("Wrong content type: %s", contentType))
		return;
	}

	buff, err := ioutil.ReadAll(multiPartFile)
	if err != nil {
		ErrorResponse(rw, "Cannot read file, it's not correct")
		log.Print(err)
	}

	imageBox, err := NewImageFromByteSlice(buff)
	if err != nil {
		ErrorResponse(rw, "Uploaded Image is not correct")
		log.Print(err)

		return;
	}

	defer imageBox.Destroy();

	imageBox.FixOrientation()

	width := imageBox.Width;
	height := imageBox.Height;
	imageBlob := imageBox.GetImageBlob();


	hasher := md5.New()
	hasher.Write(imageBlob)
	hash := hex.EncodeToString(hasher.Sum(nil))

	hashPathPart := hash[0:2] + "/" + hash[2:4] + "/";

	// Upload original file
	uploadOriginalChannel <- ImageUploadTask{
		Buffer: imageBlob,
		Path: "orig/" + hashPathPart + fmt.Sprintf("%dx%d.jpg", width, height),
	}

	for _, imgDim := range resizeImageDimmention {
		err = imageBox.ResizeImage(imgDim.Width, imgDim.Height);
		if err != nil {
			panic(err)
		}

		uploadThumbnailChannel <- ImageUploadTask{
			Buffer: imageBox.GetImageBlob(),
			Path: "photos/" + hashPathPart + fmt.Sprintf("%dx%d.jpg", imgDim.Width, imgDim.Height),
		}
	}

	photoId := generateUUID(zmqClient);


	SuccessResponse(
		rw,
		&ImageJson{
			Id: photoId,
		},
	)
}

type ImageUploadTask struct {
	// Array in slice, in-memory file
	Buffer []byte
	Path string
}

func tryUUID(client *zmq.Socket) (uint64, error) {
	_, err := client.SendMessage("GEN");
	if err != nil {
		return 0, err
	}

	reply, err := client.RecvMessage(0)
	if err != nil {
		return 0, err
	}

	res, err := strconv.ParseUint(reply[0], 10, 64)
	if err != nil {
		return 0, err
	}

	return res, nil
}

func generateUUID(client *zmq.Socket) (uint64)  {
	for i := 0; i < 5; i++ {
		res, err := tryUUID(client);
		if err == nil {
			return res;
		}
	}

	panic("Cannot generate UUID after N tries")
}


var (
	resizeImageDimmention []ImageDim = []ImageDim {
		ImageDim {
			Width: 180,
			Height: 180,
		},
		ImageDim {
			Width: 100,
			Height: 100,
		},
		ImageDim {
			Width: 75,
			Height: 75,
		},
		ImageDim {
			Width: 50,
			Height: 50,
		},
	}

	// upload to S3 channel
	uploadThumbnailChannel chan ImageUploadTask
	uploadOriginalChannel chan ImageUploadTask

	// tmp workaround @todo
	zmqClient *zmq.Socket
)

func main() {
	configuration := &Configuration{};
	configuration.Init("./config.json")

	zmqClient, _ = zmq.NewSocket(zmq.REQ)
	zmqClient.Connect(configuration.CruftFlake.Uri)

	imagick.Initialize() // LOAD ONLY ONCE, because DEAD LOCK!! @ovr
	defer imagick.Terminate()

	r := mux.Classic()

	r.HandleFunc(http.MethodPost, "/v1/upload/image", uploadImageHandler)

	errorHandler := func(errs []error) {
		for _ , err := range errs {
			fmt.Print(err)
		}

		if 0 != len(errs) {
			os.Exit(2)
		}
	}

	uploadThumbnailChannel = make(chan ImageUploadTask, 500); // Async channel but with small buffer 20 <= X <= THINK
	uploadOriginalChannel = make(chan ImageUploadTask, 1000); // Async channel but with small buffer 20 <= X <= THINK

	go startUploader(uploadThumbnailChannel, configuration.S3);
	go startUploader(uploadOriginalChannel, configuration.S3);

	r.ListenAndServe(":8989", errorHandler)
}
