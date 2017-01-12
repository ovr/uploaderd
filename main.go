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
)

type ErrorJson struct {
	Error map[string]string `json:"error"`
}

func ErrorResponse(rw http.ResponseWriter, message string, args ...interface{}) {
	data := &ErrorJson{make(map[string]string)}
	data.Error["message"] = fmt.Sprintf(message, args...)
	resp, _ := json.Marshal(data)

	rw.Write(resp)
	//http.Error(rw, "Internal Server Error", http.StatusInternalServerError)
}

type ImageDim struct {
	Width uint
	Height uint
}

func uploadImageHandler(rw http.ResponseWriter, req *http.Request) {
	multiPartFile, _, err := req.FormFile("file")
	if err != nil {
		panic(err)
	}

	defer multiPartFile.Close()

	buff, err := ioutil.ReadAll(multiPartFile)
	imageBox, err := NewImageFromByteSlice(buff)
	if err != nil {
		panic(err)
	}

	defer imageBox.Destroy();

	imageBox.FixOrientation()

	// Upload original file
	uploadOriginalChannel <- ImageUploadTask{
		Buffer: imageBox.GetImageBlob(),
		Path: "orig/1.jpg",
	}

	for _, imgDim := range resizeImageDimmention {
		err = imageBox.ResizeImage(imgDim.Width, imgDim.Height);
		if err != nil {
			panic(err)
		}

		uploadThumbnailChannel <- ImageUploadTask{
			Buffer: imageBox.GetImageBlob(),
			Path: fmt.Sprintf("photos/%dx%d.jpg", imgDim.Width, imgDim.Height),
		}
	}

	ErrorResponse(rw, imageBox.GetImageFormat())
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
)

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

func main() {
	configuration := &Configuration{};
	configuration.Init("./config.json")

	client, _ := zmq.NewSocket(zmq.REQ)
	client.Connect(configuration.CruftFlake.Uri)

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
