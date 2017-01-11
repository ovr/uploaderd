package main

import (
	"github.com/donutloop/mux"
	"net/http"
	"fmt"
	"os"
	"io/ioutil"
	"gopkg.in/gographics/imagick.v3/imagick" // v1 for 6.7, v2 for 6.9, v3 for 7+
)

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

	imageBox.GetImageFormat()
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

	r.ListenAndServe(":7777", errorHandler)
}
