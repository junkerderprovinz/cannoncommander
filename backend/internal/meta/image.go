package meta

import (
	"bufio"
	"context"
	"encoding/binary"
	"fmt"
	"image"
	"image/color"
	_ "image/gif"
	_ "image/jpeg"
	_ "image/png"
	"io"
	"math"
	"os"
	"strings"
	"time"

	_ "golang.org/x/image/bmp"
	_ "golang.org/x/image/tiff"
	_ "golang.org/x/image/webp"
)

// photo is what the EXIF block of a picture says.
type photo struct {
	width, height int64
	make, model   string
	lens          string
	focal         float64
	aperture      float64
	exposure      float64
	iso           int64
	flash         *bool
	taken         time.Time
	lat, lon      float64
	rotated       bool
	dpi           float64
}

func readImage(_ context.Context, _ *Service, f File, rec Record) error {
	fh, err := os.Open(f.Real)
	if err != nil {
		return err
	}
	defer fh.Close()
	ext := extOf(f.Path)
	var w, h int64
	if cfg, _, err := image.DecodeConfig(bufio.NewReader(fh)); err == nil {
		w, h = int64(cfg.Width), int64(cfg.Height)
		if d := depthOf(cfg.ColorModel); d > 0 {
			rec["depth"] = fmt.Sprintf("%d Bit", d)
		}
	}
	if _, err := fh.Seek(0, io.SeekStart); err != nil {
		return err
	}
	p := readExif(fh, ext)
	if w == 0 || h == 0 {
		w, h = p.width, p.height
	}
	if w > 0 && h > 0 {
		rec["width"], rec["height"] = w, h
		ww, hh := w, h
		if p.rotated {
			ww, hh = h, w
		}
		switch {
		case ww > hh:
			rec["orientation"] = "landscape"
		case ww < hh:
			rec["orientation"] = "portrait"
		default:
			rec["orientation"] = "square"
		}
	}
	rec["camera"] = strings.TrimSpace(p.model)
	rec["make"] = maker(p.make)
	rec["lens"] = strings.TrimSpace(p.lens)
	if p.focal > 0 {
		rec["focal"] = decimal(p.focal, 1) + " mm"
	}
	if p.aperture > 0 {
		rec["aperture"] = "f/" + decimal(p.aperture, 1)
	}
	if p.exposure > 0 {
		rec["exposure"] = exposure(p.exposure)
	}
	if p.iso > 0 {
		rec["iso"] = p.iso
	}
	if p.flash != nil {
		rec["flash"] = "off"
		if *p.flash {
			rec["flash"] = "fired"
		}
	}
	if !p.taken.IsZero() {
		rec["taken"] = p.taken.UnixMilli()
	}
	if p.lat != 0 || p.lon != 0 {
		rec["lat"], rec["lon"] = p.lat, p.lon
	}
	dpi := p.dpi
	if dpi == 0 {
		if _, err := fh.Seek(0, io.SeekStart); err == nil {
			dpi = densityOf(fh, ext)
		}
	}
	if dpi > 0 {
		rec["dpi"] = fmt.Sprintf("%d dpi", int(math.Round(dpi)))
	}
	return nil
}

// depthOf returns the bits per pixel of a colour model.
func depthOf(m color.Model) int {
	switch m {
	case color.RGBAModel, color.NRGBAModel, color.CMYKModel:
		return 32
	case color.RGBA64Model, color.NRGBA64Model:
		return 64
	case color.YCbCrModel:
		return 24
	case color.GrayModel, color.AlphaModel:
		return 8
	case color.Gray16Model, color.Alpha16Model:
		return 16
	}
	if _, ok := m.(color.Palette); ok {
		return 8
	}
	return 0
}

// maker tidies the maker names cameras write in capitals or with their
// company form, so SONY reads Sony and NIKON CORPORATION reads Nikon.
func maker(s string) string {
	s = strings.TrimSpace(s)
	for _, cut := range []string{" CORPORATION", " Corporation", " IMAGING CORP.", " Imaging Corp.", " COMPANY, LIMITED", " Co., Ltd.", " CO.,LTD", " AG", " GmbH"} {
		s = strings.TrimSuffix(s, cut)
	}
	if len(s) > 3 && s == strings.ToUpper(s) && !strings.Contains(s, " ") {
		lower := strings.ToLower(s)
		s = strings.ToUpper(lower[:1]) + lower[1:]
	}
	return s
}

func exposure(t float64) string {
	if t < 1 {
		return fmt.Sprintf("1/%d s", int(math.Round(1/t)))
	}
	return decimal(t, 1) + " s"
}

// densityOf reads the resolution a PNG or JFIF file states, for pictures
// without EXIF.
func densityOf(r io.Reader, ext string) float64 {
	head := make([]byte, 4096)
	n, _ := io.ReadFull(r, head)
	head = head[:n]
	switch ext {
	case "jpg", "jpeg":
		// FF D8, then the APP0 segment: FF E0, length, "JFIF\0", version,
		// unit, x density, y density.
		if len(head) >= 18 && head[2] == 0xFF && head[3] == 0xE0 && string(head[6:11]) == "JFIF\x00" {
			x := float64(binary.BigEndian.Uint16(head[14:16]))
			switch head[13] {
			case 1:
				return x
			case 2:
				return x * 2.54
			}
		}
	case "png":
		// Chunks after the 8 byte signature: length, type, data, CRC.
		for i := 8; i+12 <= len(head); {
			size := int(binary.BigEndian.Uint32(head[i:]))
			typ := string(head[i+4 : i+8])
			if typ == "pHYs" && i+8+9 <= len(head) && head[i+16] == 1 {
				return float64(binary.BigEndian.Uint32(head[i+8:])) * 0.0254
			}
			if typ == "IDAT" || size < 0 {
				break
			}
			i += 12 + size
		}
	}
	return 0
}
