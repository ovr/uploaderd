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
	}
	return imgBox, nil
}

func (this *ImageBox) GetImageBlob() []byte {
	return this.mw.GetImageBlob()
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

	this.Width = uint64(this.mw.GetImageWidth());
	this.Height = uint64(this.mw.GetImageHeight());
}

func (this *ImageBox) CropImage(width, height uint, x, y int) error {
	this.Width = uint64(width)
	this.Height = uint64(height)

	return this.mw.CropImage(width, height, x, y)
}

func (this *ImageBox) SetImageFormat(format string) error {
	return this.mw.SetImageFormat(format)
}

func (this *ImageBox) SetImageInterpolateMethod(method imagick.PixelInterpolateMethod) error {
	return this.mw.SetImageInterpolateMethod(method)
}

func (this *ImageBox) GetImageInterlaceMethod() imagick.PixelInterpolateMethod {
	return this.mw.GetImageInterpolateMethod()
}

func (this *ImageBox) GetImageInterlaceScheme() imagick.InterlaceType {
	return this.mw.GetImageInterlaceScheme()
}

func (this *ImageBox) SetImageInterlaceScheme(interlace imagick.InterlaceType) error {
	return this.mw.SetImageInterlaceScheme(interlace)
}

func (this *ImageBox) SetImageCompression(compression imagick.CompressionType) error {
	return this.mw.SetImageCompression(compression)
}

func (this *ImageBox) SetImageCompressionQuality(quality uint) error {
	return this.mw.SetImageCompressionQuality(quality)
}

func (this *ImageBox) StripImage() error {
	return this.mw.StripImage()
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

func (this *ImageBox) MaxDimensionResize(maxWidth, maxHeight uint) error {
	if this.Width > this.Height {
		if uint(this.Width) > maxWidth {
			proportion := float64(this.Height) / float64(this.Width)
			height := float64(maxWidth) * proportion

			return this.ResizeImage(maxWidth, uint(height))
		}
	} else {
		if uint(this.Height) > maxHeight {
			proportion := float64(this.Width) / float64(this.Height)
			width := float64(maxHeight) * proportion

			return this.ResizeImage(uint(width), maxHeight)
		}
	}

	return nil
}

func (this *ImageBox) ResizeImage(width, height uint) error {
	this.Width = uint64(width)
	this.Height = uint64(height)

	return this.mw.ResizeImage(width, height, imagick.FILTER_UNDEFINED)
}

func (this *ImageBox) Destroy() {
	this.mw.Destroy()
}

func (this *ImageBox) GetImageFormat() string {
	return this.mw.GetImageFormat()
}
