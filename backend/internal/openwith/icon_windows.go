//go:build desktop

package openwith

import (
	"bytes"
	"image"
	"image/png"
	"os"
	"strconv"
	"strings"

	"github.com/junkerderprovinz/cannoncommander/backend/pkg/winshell"
)

// iconImage loads the icon at where, which is a file and the index of an
// icon in it as "path,index", or the indirect string of a packaged app's
// image, which starts with "@{".
func iconImage(where string) (image.Image, error) {
	if strings.HasPrefix(where, "@{") {
		file, err := winshell.IndirectString(where)
		if err != nil {
			return nil, err
		}
		b, err := os.ReadFile(file)
		if err != nil {
			return nil, err
		}
		img, err := png.Decode(bytes.NewReader(b))
		if err != nil {
			return nil, err
		}
		return trimmed(img), nil
	}
	at := strings.LastIndexByte(where, ',')
	if at < 0 {
		return nil, errNoIcon
	}
	index, err := strconv.Atoi(where[at+1:])
	if err != nil {
		return nil, errNoIcon
	}
	return winshell.ExtractIcon(where[:at], index, iconSize)
}

// trimmed cuts the clear margin off a packaged app's image, which leaves
// room around the picture for a tile that a menu row does not draw.
func trimmed(img image.Image) image.Image {
	b := img.Bounds()
	opaque := image.Rectangle{}
	for y := b.Min.Y; y < b.Max.Y; y++ {
		for x := b.Min.X; x < b.Max.X; x++ {
			if _, _, _, a := img.At(x, y).RGBA(); a != 0 {
				opaque = opaque.Union(image.Rect(x, y, x+1, y+1))
			}
		}
	}
	sub, ok := img.(interface {
		SubImage(image.Rectangle) image.Image
	})
	if opaque.Empty() || !ok {
		return img
	}
	return sub.SubImage(opaque)
}
