package openwith

import (
	"bytes"
	"image"
	"image/png"
	"os"
	"strconv"
	"strings"
	"unsafe"

	"golang.org/x/sys/windows"
)

// iconImage loads the icon at where, which is a file and the index of an
// icon in it as "path,index", or the indirect string of a packaged app's
// image, which starts with "@{".
func iconImage(where string) (image.Image, error) {
	if strings.HasPrefix(where, "@{") {
		file, err := indirectString(where)
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
	return extractIcon(where[:at], index)
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

func indirectString(s string) (string, error) {
	p, err := windows.UTF16PtrFromString(s)
	if err != nil {
		return "", err
	}
	buf := make([]uint16, 1024)
	r, _, _ := procLoadIndirectString.Call(uintptr(unsafe.Pointer(p)), uintptr(unsafe.Pointer(&buf[0])), uintptr(len(buf)), 0)
	if err := failed(r); err != nil {
		return "", err
	}
	return windows.UTF16ToString(buf), nil
}

type iconInfo struct {
	icon        int32
	xHot, yHot  uint32
	mask, color windows.Handle
}

type bitmap struct {
	kind, width, height, widthBytes int32
	planes, bitsPerPixel            uint16
	bits                            uintptr
}

type bitmapInfo struct {
	size                        uint32
	width, height               int32
	planes, bitCount            uint16
	compression, sizeImage      uint32
	xPerMeter, yPerMeter        int32
	colorsUsed, colorsImportant uint32
	// Room for the colour table GetDIBits may write after the header.
	colors [256]uint32
}

func extractIcon(file string, index int) (image.Image, error) {
	p, err := windows.UTF16PtrFromString(file)
	if err != nil {
		return nil, err
	}
	var icon windows.Handle
	r, _, _ := procExtractIcon.Call(uintptr(unsafe.Pointer(p)), uintptr(int32(index)), 0, uintptr(unsafe.Pointer(&icon)), 0, iconSize)
	if r != 0 || icon == 0 {
		return nil, errNoIcon
	}
	defer procDestroyIcon.Call(uintptr(icon))
	var info iconInfo
	if ok, _, _ := procGetIconInfo.Call(uintptr(icon), uintptr(unsafe.Pointer(&info))); ok == 0 {
		return nil, errNoIcon
	}
	defer procDeleteObject.Call(uintptr(info.mask))
	// A black and white icon keeps both of its halves in the mask.
	if info.color == 0 {
		return nil, errNoIcon
	}
	defer procDeleteObject.Call(uintptr(info.color))
	var bm bitmap
	if n, _, _ := procGetObject.Call(uintptr(info.color), unsafe.Sizeof(bm), uintptr(unsafe.Pointer(&bm))); n == 0 {
		return nil, errNoIcon
	}
	w, h := int(bm.width), int(bm.height)
	color, err := dibits(info.color, w, h)
	if err != nil {
		return nil, err
	}
	img := image.NewNRGBA(image.Rect(0, 0, w, h))
	alpha := false
	for i := 0; i < len(color); i += 4 {
		img.Pix[i], img.Pix[i+1], img.Pix[i+2], img.Pix[i+3] = color[i+2], color[i+1], color[i], color[i+3]
		alpha = alpha || color[i+3] != 0
	}
	// An icon from before alpha channels leaves them empty and keeps what
	// shows through in its mask, where white is transparent.
	if !alpha {
		mask, err := dibits(info.mask, w, h)
		if err != nil {
			return nil, err
		}
		for i := 3; i < len(mask); i += 4 {
			if mask[i-1] == 0 {
				img.Pix[i] = 0xff
			}
		}
	}
	return img, nil
}

// dibits copies a bitmap's pixels as 32-bit BGRA rows from the top down.
func dibits(bm windows.Handle, w, h int) ([]byte, error) {
	dc, _, _ := procGetDC.Call(0)
	if dc == 0 {
		return nil, errNoIcon
	}
	defer procReleaseDC.Call(0, dc)
	info := bitmapInfo{width: int32(w), height: -int32(h), planes: 1, bitCount: 32}
	info.size = uint32(unsafe.Offsetof(info.colors))
	pix := make([]byte, w*h*4)
	const rgbColors = 0
	n, _, _ := procGetDIBits.Call(dc, uintptr(bm), 0, uintptr(h), uintptr(unsafe.Pointer(&pix[0])), uintptr(unsafe.Pointer(&info)), rgbColors)
	if int(n) != h {
		return nil, errNoIcon
	}
	return pix, nil
}
