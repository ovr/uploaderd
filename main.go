package main

import (
	"github.com/donutloop/mux"
	"net/http"
	"fmt"
	"os"
	"io/ioutil"
	"gopkg.in/gographics/imagick.v3/imagick" // v1 for 6.7, v2 for 6.9, v3 for 7+
	"encoding/json"
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

func NewImageFromByteSlice(buff []byte) (*imagick.MagickWand, error) {
	mw := imagick.NewMagickWand()

	readImageBlobError := mw.ReadImageBlob(buff)
	if readImageBlobError != nil {
		// Destroy via exit, need to protect memory leak
		mw.Destroy()

		return nil, readImageBlobError
	}

	return mw, nil
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

	err = imageBox.ResizeImage(100, 100, imagick.FILTER_QUADRATIC);
	if err != nil {
		panic(err)
	}

	err = imageBox.ResizeImage(75, 75, imagick.FILTER_QUADRATIC);
	if err != nil {
		panic(err)
	}

	err = imageBox.ResizeImage(50, 50, imagick.FILTER_QUADRATIC);
	if err != nil {
		panic(err)
	}

	ErrorResponse(rw, imageBox.GetImageFormat())
}

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

	r.ListenAndServe(":8989", errorHandler)
}
