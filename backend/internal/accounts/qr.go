package accounts

import (
	"fmt"
	"strings"

	"github.com/boombuler/barcode/qr"
)

// qrSVG draws content as a QR code with a quiet zone of two modules. The
// modules are rects on a viewBox, so the code stays sharp at any size.
func qrSVG(content string) (string, error) {
	code, err := qr.Encode(content, qr.M, qr.Auto)
	if err != nil {
		return "", err
	}
	n := code.Bounds().Dx()
	var b strings.Builder
	fmt.Fprintf(&b, `<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 %d %d" shape-rendering="crispEdges" role="img" aria-label="QR code">`, n+4, n+4)
	fmt.Fprintf(&b, `<rect width="%d" height="%d" fill="#fff"/><g fill="#161616">`, n+4, n+4)
	for y := range n {
		for x := range n {
			if dark, _, _, _ := code.At(x, y).RGBA(); dark < 0x8000 {
				fmt.Fprintf(&b, `<rect x="%d" y="%d" width="1" height="1"/>`, x+2, y+2)
			}
		}
	}
	b.WriteString(`</g></svg>`)
	return b.String(), nil
}
