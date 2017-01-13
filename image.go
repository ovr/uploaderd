package main

import "gopkg.in/gographics/imagick.v3/imagick" // v3 for 7+

type ImageBox struct {
	mw            *imagick.MagickWand
	Width, Height uint64
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
		mw:     mw,
		Width:  uint64(mw.GetImageWidth()),
		Height: uint64(mw.GetImageHeight()),
	};
	return imgBox, nil
}

func (this *ImageBox) GetImageBlob() []byte {
	return this.mw.GetImageBlob();
}

func (this *ImageBox) FixOrientation() {
	orientation := this.mw.GetImageOrientation()

	switch orientation {
	case imagick.ORIENTATION_TOP_LEFT:
		// skip it
		break
	case imagick.ORIENTATION_TOP_RIGHT:
		this.mw.FlopImage()
		break
	case imagick.ORIENTATION_BOTTOM_RIGHT:
		this.mw.RotateImage(imagick.NewPixelWand(), 180)
		break
	case imagick.ORIENTATION_BOTTOM_LEFT:
		this.mw.FlipImage()
		break
	case imagick.ORIENTATION_LEFT_TOP:
		this.mw.FlipImage()
		this.mw.RotateImage(imagick.NewPixelWand(), 90)
		break
	case imagick.ORIENTATION_RIGHT_TOP:
		this.mw.RotateImage(imagick.NewPixelWand(), 90)
		break
	case imagick.ORIENTATION_RIGHT_BOTTOM:
		this.mw.FlopImage()
		this.mw.RotateImage(imagick.NewPixelWand(), -90)
		break
	case imagick.ORIENTATION_LEFT_BOTTOM:
		this.mw.RotateImage(imagick.NewPixelWand(), -90)
		break
	}
}

func (this *ImageBox) CropImage(width, height uint, x, y int) error {
	return this.mw.CropImage(width, height, x, y)
}

func (this *ImageBox) SetImageCompressionQuality(quality uint) error {
	return this.mw.SetImageCompressionQuality(quality)
}

func (this *ImageBox) NormalizeImage() error {
	return this.mw.NormalizeImage()
}

func (this *ImageBox) UnsharpMaskImage(radius, sigma, amount, threshold float64) error {
	return this.mw.UnsharpMaskImage(radius, sigma, amount, threshold)
}

func (this *ImageBox) ThumbnailImage(width, height uint) error {
	return this.mw.ThumbnailImage(width, height)
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
