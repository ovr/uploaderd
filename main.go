package main

import (
	_ "github.com/go-sql-driver/mysql"
	"github.com/iris-contrib/middleware/logger"
	"github.com/iris-contrib/middleware/pprof"
	"github.com/iris-contrib/middleware/recovery"
	"github.com/jinzhu/gorm"
	"github.com/kataras/iris"
	zmq "github.com/pebbe/zmq4"
	"gopkg.in/gographics/imagick.v3/imagick" // v3 for 7+
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

type ImageDim struct {
	Width  uint
	Height uint
}

type ImageUploadTask struct {
	// Array in slice, in-memory file
	Buffer []byte
	Path   string
}

var (
	resizeImageDimmention []ImageDim = []ImageDim{
		ImageDim{
			Width:  180,
			Height: 180,
		},
		ImageDim{
			Width:  100,
			Height: 100,
		},
		ImageDim{
			Width:  75,
			Height: 75,
		},
		ImageDim{
			Width:  50,
			Height: 50,
		},
	}

	// upload to S3 channel
	uploadThumbnailChannel chan ImageUploadTask
	uploadOriginalChannel  chan ImageUploadTask

	// tmp workaround @todo
	zmqClient *zmq.Socket
)

func main() {
	configuration := &Configuration{}
	configuration.Init("./config.json")

	zmqClient, _ = zmq.NewSocket(zmq.REQ)
	zmqClient.Connect(configuration.CruftFlake.Uri)

	imagick.Initialize() // LOAD ONLY ONCE, because DEAD LOCK!! @ovr
	defer imagick.Terminate()

	uploadThumbnailChannel = make(chan ImageUploadTask, configuration.S3.UploadThumbnailChannelSize)
	uploadOriginalChannel = make(chan ImageUploadTask, configuration.S3.UploadOriginalChannelSize)

	db, err := gorm.Open(configuration.DB.Dialect, configuration.DB.Uri)
	if err != nil {
		panic(err)
	}
	defer db.Close()

	db.LogMode(configuration.DB.ShowLog)
	db.DB().SetMaxIdleConns(configuration.DB.MaxIdleConnections)
	db.DB().SetMaxOpenConns(configuration.DB.MaxOpenConnections)

	go startUploader(uploadThumbnailChannel, configuration.S3)
	go startUploader(uploadOriginalChannel, configuration.S3)

	api := iris.New()

	api.Use(logger.New())
	api.Use(recovery.Handler)

	pprof := pprof.New()
	api.Get("/debug/pprof/*action", pprof)
	api.Handle(
		"POST",
		"/v1/image",
		createJWTMiddelWare(configuration.JWT),
		ImagePostHandler{
			DB: db,
		},
	)

	api.Listen(":8989")
}
