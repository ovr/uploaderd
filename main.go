package main

import (
	"flag"
	"fmt"
	_ "github.com/go-sql-driver/mysql"
	"github.com/jinzhu/gorm"
	"github.com/newrelic/go-agent"
	log "github.com/sirupsen/logrus"
	"github.com/urfave/negroni"
	"gopkg.in/gographics/imagick.v3/imagick" // v3 for 7+
	"net/http"
	"os"
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
)

func main() {
	var (
		configFile string
		err        error
	)

	flag.StringVar(&configFile, "config", "./config.json", "Config filepath")
	flag.Parse()

	configuration := &Configuration{}
	configuration.Init(configFile)

	log.SetFormatter(&log.JSONFormatter{})

	if configuration.Debug {
		log.SetLevel(log.DebugLevel)
	}

	app, err := newrelic.NewApplication(
		newrelic.NewConfig(configuration.NewRelic.AppName, configuration.NewRelic.Key),
	)

	if err != nil {
		fmt.Println(err)
		os.Exit(1)
	}

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

	UUIDGenerator := NewUUIDGenerator(configuration.CruftFlake.Uri)
	go UUIDGenerator.Listen()

	mux := http.NewServeMux()

	mux.Handle(newrelic.WrapHandle(app, "/v1/image", ImagePostHandler{
		DB:            db,
		UUIDGenerator: UUIDGenerator,
	}))

	n := negroni.New(negroni.NewRecovery(), negroni.NewLogger(), NewJWT(configuration.JWT.SecretKey))
	n.UseHandler(mux)

	http.ListenAndServe(":8989", n)
}
