package main

import (
	"gopkg.in/gographics/imagick.v3/imagick" // v3 for 7+
	zmq "github.com/pebbe/zmq4"
	"github.com/kataras/iris"
)

type ErrorJsonBody struct {
	Message string `json:"message"`	
} 

type ErrorJson struct {
	Error ErrorJsonBody `json:"error"`
}

func newErrorJson(message string) ErrorJson {
	return ErrorJson{
		Error: ErrorJsonBody{
			Message: message,
		},
	}
}

type ImageJson struct {
	Id uint64 `json:"id"`
}

type ImageDim struct {
	Width uint
	Height uint
}

type ImageUploadTask struct {
	// Array in slice, in-memory file
	Buffer []byte
	Path string
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

	uploadThumbnailChannel = make(chan ImageUploadTask, 500); // Async channel but with small buffer 20 <= X <= THINK
	uploadOriginalChannel = make(chan ImageUploadTask, 1000); // Async channel but with small buffer 20 <= X <= THINK

	go startUploader(uploadThumbnailChannel, configuration.S3);
	go startUploader(uploadOriginalChannel, configuration.S3);

	api := iris.New()

	api.Use(createJWTMiddelWare(configuration.JWT))

	api.Handle("POST", "/v1/upload/image", ImagePostHandler{});

	api.Listen(":8989")
}
