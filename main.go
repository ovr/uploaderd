package main

import (
	"github.com/donutloop/mux"
	"net/http"
	"fmt"
	"os"
	"io/ioutil"
	"encoding/json"
	"gopkg.in/gographics/imagick.v3/imagick" // v3 for 7+
	"log"
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

	var dims []ImageDim = []ImageDim {
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
	};


	for _, dim := range dims {
		err = imageBox.ResizeImage(dim.Width, dim.Height);
		if err != nil {
			panic(err)
		}

		uploadChannel <- imageBox.GetImageBlob();
	}

	ErrorResponse(rw, imageBox.GetImageFormat())
}

var (
	// upload to S3 channel
	uploadChannel chan []byte
)

func main() {
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

	uploadChannel = make(chan []byte, 20); // Async channel but with small buffer 20 <= X <= THINK

	go func() {
		for {
			select {
			case imageBlob := <- uploadChannel:
				log.Print("[Event] New Image to Upload ", len(imageBlob));
			}
		}
	}();

	r.ListenAndServe(":8989", errorHandler)
}
