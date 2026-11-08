package winshell

import (
	"errors"
	"image"
	"unsafe"

	"golang.org/x/sys/windows"
)

var (
	procExtractIcon  = shell32.NewProc("SHDefExtractIconW")
	procDestroyIcon  = user32.NewProc("DestroyIcon")
	procGetIconInfo  = user32.NewProc("GetIconInfo")
	procGetDC        = user32.NewProc("GetDC")
	procReleaseDC    = user32.NewProc("ReleaseDC")
	procGetObject    = gdi32.NewProc("GetObjectW")
	procGetDIBits    = gdi32.NewProc("GetDIBits")
	procDeleteObject = gdi32.NewProc("DeleteObject")
)

// ErrNoImage is returned for an icon or a bitmap that holds no picture this
// package can read.
var ErrNoImage = errors.New("no picture")

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

// ExtractIcon loads the icon at index in file, size pixels square.
func ExtractIcon(file string, index, size int) (image.Image, error) {
	p, err := windows.UTF16PtrFromString(file)
	if err != nil {
		return nil, err
	}
	var icon windows.Handle
	r, _, _ := procExtractIcon.Call(uintptr(unsafe.Pointer(p)), uintptr(int32(index)), 0, uintptr(unsafe.Pointer(&icon)), 0, uintptr(size))
	if r != 0 || icon == 0 {
		return nil, ErrNoImage
	}
	defer DestroyIcon(icon)
	return IconImage(icon)
}

// DestroyIcon gives back an icon the caller was handed.
func DestroyIcon(icon windows.Handle) { procDestroyIcon.Call(uintptr(icon)) }

// DeleteBitmap gives back a bitmap the caller was handed.
func DeleteBitmap(bm windows.Handle) { procDeleteObject.Call(uintptr(bm)) }

// IconImage copies the picture of icon, which stays the caller's.
func IconImage(icon windows.Handle) (image.Image, error) {
	var info iconInfo
	if ok, _, _ := procGetIconInfo.Call(uintptr(icon), uintptr(unsafe.Pointer(&info))); ok == 0 {
		return nil, ErrNoImage
	}
	defer DeleteBitmap(info.mask)
	// A black and white icon keeps both of its halves in the mask.
	if info.color == 0 {
		return nil, ErrNoImage
	}
	defer DeleteBitmap(info.color)
	w, h, err := bitmapSize(info.color)
	if err != nil {
		return nil, err
	}
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

// BitmapImage copies the picture of a bitmap the shell drew, such as a
// thumbnail or the picture of a menu row. Those carry their alpha already
// multiplied in, or none at all for a picture that covers its square.
func BitmapImage(bm windows.Handle) (image.Image, error) {
	w, h, err := bitmapSize(bm)
	if err != nil {
		return nil, err
	}
	pix, err := dibits(bm, w, h)
	if err != nil {
		return nil, err
	}
	alpha, premultiplied := false, true
	for i := 0; i < len(pix); i += 4 {
		pix[i], pix[i+2] = pix[i+2], pix[i]
		a := pix[i+3]
		alpha = alpha || a != 0
		premultiplied = premultiplied && pix[i] <= a && pix[i+1] <= a && pix[i+2] <= a
	}
	rect := image.Rect(0, 0, w, h)
	switch {
	case !alpha:
		for i := 3; i < len(pix); i += 4 {
			pix[i] = 0xff
		}
		return &image.RGBA{Pix: pix, Stride: 4 * w, Rect: rect}, nil
	case premultiplied:
		return &image.RGBA{Pix: pix, Stride: 4 * w, Rect: rect}, nil
	}
	return &image.NRGBA{Pix: pix, Stride: 4 * w, Rect: rect}, nil
}

func bitmapSize(bm windows.Handle) (int, int, error) {
	var b bitmap
	if n, _, _ := procGetObject.Call(uintptr(bm), unsafe.Sizeof(b), uintptr(unsafe.Pointer(&b))); n == 0 || b.width <= 0 || b.height == 0 {
		return 0, 0, ErrNoImage
	}
	return int(b.width), int(max(b.height, -b.height)), nil
}

// dibits copies a bitmap's pixels as 32-bit BGRA rows from the top down.
func dibits(bm windows.Handle, w, h int) ([]byte, error) {
	dc, _, _ := procGetDC.Call(0)
	if dc == 0 {
		return nil, ErrNoImage
	}
	defer procReleaseDC.Call(0, dc)
	info := bitmapInfo{width: int32(w), height: -int32(h), planes: 1, bitCount: 32}
	info.size = uint32(unsafe.Offsetof(info.colors))
	pix := make([]byte, w*h*4)
	const rgbColors = 0
	n, _, _ := procGetDIBits.Call(dc, uintptr(bm), 0, uintptr(h), uintptr(unsafe.Pointer(&pix[0])), uintptr(unsafe.Pointer(&info)), rgbColors)
	if int(n) != h {
		return nil, ErrNoImage
	}
	return pix, nil
}
