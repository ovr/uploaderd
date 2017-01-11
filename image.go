package main

import "gopkg.in/gographics/imagick.v3/imagick" // v3 for 7+

type ImageBox struct {
	mw                   *imagick.MagickWand
	Width, Height        uint
}

func NewImageFromByteSlice(buff []byte) (*ImageBox, error) {
	mw := imagick.NewMagickWand()

	readImageBlobError := mw.ReadImageBlob(buff)
	if readImageBlobError != nil {
		// Destroy via exit, need to protect memory leak
		mw.Destroy()

		return nil, readImageBlobError
	}

	imgBox := &ImageBox{
		mw: mw,
		Width: mw.GetImageWidth(),
		Height: mw.GetImageHeight(),
	};

	return imgBox, nil
}

func (this *ImageBox) ResizeImage(width, height uint) error {
	return this.mw.ResizeImage(width, height, imagick.FILTER_QUADRATIC)
}

func (this *ImageBox) Destroy() {
	this.mw.Destroy()
}

func (this *ImageBox) GetImageFormat() string {
	return this.mw.GetImageFormat()
}
