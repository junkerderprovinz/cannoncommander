package main

import (
	"bytes"
	"image"
	"image/png"

	"golang.org/x/image/draw"
)

// windowIcon is the mark at 256 pixels for the window. GTK leaves out an
// icon above about 500 pixels, which no longer fits an X11 property, and the
// window then has none in the task bars that read it.
func windowIcon() []byte {
	src, err := png.Decode(bytes.NewReader(appIcon))
	if err != nil {
		return appIcon
	}
	dst := image.NewNRGBA(image.Rect(0, 0, 256, 256))
	draw.CatmullRom.Scale(dst, dst.Bounds(), src, src.Bounds(), draw.Src, nil)
	var b bytes.Buffer
	if err := png.Encode(&b, dst); err != nil {
		return appIcon
	}
	return b.Bytes()
}
