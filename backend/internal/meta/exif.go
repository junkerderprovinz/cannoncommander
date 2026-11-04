//go:build !386 && !arm

package meta

import (
	"io"
	"time"

	extimagemeta "github.com/evanoberholster/imagemeta"
	"github.com/evanoberholster/imagemeta/meta"
	"github.com/evanoberholster/imagemeta/meta/exif"
)

func readExif(r io.ReadSeeker, ext string) photo {
	var ex exif.Exif
	var err error
	switch ext {
	case "heic", "heif", "avif":
		ex, err = extimagemeta.DecodeHeif(r)
	default:
		ex, err = extimagemeta.Decode(r)
	}
	if err != nil {
		return photo{}
	}
	p := photo{
		make:     ex.IFD0.Make,
		model:    ex.IFD0.Model,
		lens:     ex.ExifIFD.LensModel,
		focal:    float64(ex.ExifIFD.FocalLength),
		aperture: float64(ex.ExifIFD.FNumber),
		exposure: float64(ex.ExifIFD.ExposureTime),
		iso:      int64(ex.ExifIFD.ISOSpeedRatings),
		lat:      ex.GPS.Latitude(),
		lon:      ex.GPS.Longitude(),
		width:    int64(ex.ExifIFD.PixelXDimension),
		height:   int64(ex.ExifIFD.PixelYDimension),
	}
	if p.aperture == 0 {
		p.aperture = float64(ex.ExifIFD.ApertureValue)
	}
	if p.width == 0 || p.height == 0 {
		p.width, p.height = int64(ex.IFD0.ImageWidth), int64(ex.IFD0.ImageHeight)
	}
	switch ex.IFD0.Orientation {
	case meta.OrientationMirrorHorizontalRotate270, meta.OrientationRotate90, meta.OrientationMirrorHorizontalRotate90, meta.OrientationRotate270:
		p.rotated = true
	}
	// A camera writes the flash tag along with its model, a picture
	// without any camera details would only claim there was no flash.
	if p.model != "" {
		fired := ex.ExifIFD.Flash.Fired()
		p.flash = &fired
	}
	p.taken = ex.ExifIFD.DateTimeOriginal
	if p.taken.IsZero() {
		p.taken = ex.ExifIFD.CreateDate
	}
	p.taken = wallClock(p.taken)
	switch ex.IFD0.ResolutionUnit {
	case meta.ResolutionUnitInches:
		p.dpi = ex.IFD0.XResolution
	case meta.ResolutionUnitCentimeters:
		p.dpi = ex.IFD0.XResolution * 2.54
	}
	return p
}

// wallClock reads a time without a zone as local time, which is what a
// camera means when it writes the date without an offset.
func wallClock(t time.Time) time.Time {
	if t.IsZero() || t.Location() != time.UTC {
		return t
	}
	return time.Date(t.Year(), t.Month(), t.Day(), t.Hour(), t.Minute(), t.Second(), t.Nanosecond(), time.Local)
}
