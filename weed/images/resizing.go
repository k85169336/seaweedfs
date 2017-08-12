package images

import (
	"bytes"
	"image"
	"image/gif"
	"image/jpeg"
	"image/png"

	"github.com/chrislusf/seaweedfs/weed/glog"
	"github.com/disintegration/imaging"
)

func Resized(ext string, data []byte, width, height, fill int) (resized []byte, w int, h int) {
	if width == 0 && height == 0 {
		return data, 0, 0
	}
	srcImage, _, err := image.Decode(bytes.NewReader(data))
	if err == nil {
		dx := srcImage.Bounds().Dx()
		dy := srcImage.Bounds().Dy()
		var dstImage *image.NRGBA
		if (width*height != 0) && fill == 1 {
			dstImage = imaging.Thumbnail(srcImage, width, height, imaging.Lanczos)
		} else if (width*height != 0) && fill == 2 {
			if width/height > dx/dy { //定高
				width = 0
			} else { //定宽
				height = 0
			}
			dstImage = imaging.Resize(srcImage, width, height, imaging.Lanczos)
		} else {
			dstImage = imaging.Resize(srcImage, width, height, imaging.Lanczos)
		}
		var buf bytes.Buffer
		switch ext {
		case ".png":
			png.Encode(&buf, dstImage)
		case ".jpg", ".jpeg":
			jpeg.Encode(&buf, dstImage, nil)
		case ".gif":
			gif.Encode(&buf, dstImage, nil)
		}
		return buf.Bytes(), dstImage.Bounds().Dx(), dstImage.Bounds().Dy()
	} else {
		glog.Error(err)
	}
	return data, 0, 0
}
