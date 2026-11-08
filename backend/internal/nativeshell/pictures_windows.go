package nativeshell

import (
	"bytes"
	"context"
	"encoding/base64"
	"image"
	"image/png"
	"os"
	"path/filepath"
	"runtime"
	"strconv"
	"sync"
	"unsafe"

	"github.com/junkerderprovinz/cannoncommander/backend/pkg/winshell"
	"golang.org/x/sys/windows"
)

var (
	procGetFileInfo     = shell32.NewProc("SHGetFileInfoW")
	procGetImageList    = shell32.NewProc("SHGetImageList")
	procItemFromParsing = shell32.NewProc("SHCreateItemFromParsingName")
)

var (
	iidImageList         = windows.GUID{Data1: 0x46eb5926, Data2: 0x582e, Data3: 0x4017, Data4: [8]byte{0x9f, 0xdf, 0xe8, 0x99, 0x8d, 0xaa, 0x09, 0x50}}
	iidShellItemImageFac = windows.GUID{Data1: 0xbcc18b79, Data2: 0xba16, Data3: 0x442f, Data4: [8]byte{0x80, 0xc4, 0x8a, 0x59, 0xc3, 0x0c, 0x46, 0x3b}}
)

const (
	imageListGetIcon      = 10
	imageFactoryGetImage  = 3
	shgfiSysIconIndex     = 0x4000
	shgfiUseFileAttribute = 0x10
	ildTransparent        = 0x1

	siigbfBiggerSizeOK  = 0x1
	siigbfThumbnailOnly = 0x8
	maxThumbnailsAtOnce = 4
	maxIconsKept        = 4000
)

// The system's lists of icons by the size of their pictures.
const (
	shilLarge      = 0
	shilSmall      = 1
	shilExtraLarge = 2
	shilJumbo      = 4
)

type fileInfo struct {
	icon       windows.Handle
	index      int32
	attributes uint32
	name       [windows.MAX_PATH]uint16
	typeName   [80]uint16
}

var (
	iconsMu sync.Mutex
	icons   = map[string][]byte{}

	thumbSlots = make(chan struct{}, maxThumbnailsAtOnce)
)

// Icon is the icon Explorer shows for the file or folder at file, about px
// pixels square, as a PNG. A program, a shortcut or a folder with an icon of
// its own gets that one, unless it lies on another computer.
func Icon(file string, px int) ([]byte, string, error) {
	if reachesOut(file) {
		info, err := os.Stat(file)
		return TypeIcon(filepath.Base(file), err == nil && info.IsDir(), px)
	}
	return systemIcon(file, 0, 0, px)
}

// TypeIcon is the icon Explorer shows for a file called name, or a folder
// when dir is set, without looking at any file.
func TypeIcon(name string, dir bool, px int) ([]byte, string, error) {
	attr := uint32(windows.FILE_ATTRIBUTE_NORMAL)
	if dir {
		attr = windows.FILE_ATTRIBUTE_DIRECTORY
	}
	return systemIcon(name, attr, shgfiUseFileAttribute, px)
}

// systemIcon looks the icon up in the system's lists, which Explorer fills
// and shares, so a picture is drawn once for each icon and size.
func systemIcon(name string, attr uint32, flags uintptr, px int) ([]byte, string, error) {
	p, err := windows.UTF16PtrFromString(name)
	if err != nil {
		return nil, "", err
	}
	var out []byte
	err = onIconThread(func() error {
		var info fileInfo
		if r, _, _ := procGetFileInfo.Call(uintptr(unsafe.Pointer(p)), uintptr(attr), uintptr(unsafe.Pointer(&info)), unsafe.Sizeof(info), shgfiSysIconIndex|flags); r == 0 {
			return ErrNoPicture
		}
		list := listFor(px)
		key := strconv.Itoa(list) + "/" + strconv.Itoa(int(info.index))
		iconsMu.Lock()
		out = icons[key]
		iconsMu.Unlock()
		if out != nil {
			return nil
		}
		img, err := listIcon(list, info.index)
		// The largest list holds a small program icon in its top corner, so
		// such a program shows the sharper one of the list below.
		if err == nil && list == shilJumbo && small(img) {
			img, err = listIcon(shilExtraLarge, info.index)
		}
		if err != nil {
			return err
		}
		if out, err = pngBytes(img); err != nil {
			return err
		}
		iconsMu.Lock()
		if len(icons) >= maxIconsKept {
			clear(icons)
		}
		icons[key] = out
		iconsMu.Unlock()
		return nil
	})
	return out, "image/png", err
}

var (
	iconStart sync.Once
	iconCalls = make(chan func())
)

// onIconThread runs fn on the one thread that looks up icons, in the shell's
// apartment. The system's lists of icons hand out wrong or no pictures when
// several threads ask at once.
func onIconThread(fn func() error) error {
	iconStart.Do(func() {
		go func() {
			runtime.LockOSThread()
			// A fresh thread always enters the apartment.
			_ = windows.CoInitializeEx(0, windows.COINIT_APARTMENTTHREADED|windows.COINIT_DISABLE_OLE1DDE)
			for call := range iconCalls {
				call()
			}
		}()
	})
	done := make(chan error, 1)
	iconCalls <- func() { done <- fn() }
	return <-done
}

func listFor(px int) int {
	switch {
	case px <= 16:
		return shilSmall
	case px <= 32:
		return shilLarge
	case px <= 48:
		return shilExtraLarge
	}
	return shilJumbo
}

func listIcon(list int, index int32) (image.Image, error) {
	var il unsafe.Pointer
	r, _, _ := procGetImageList.Call(uintptr(list), uintptr(unsafe.Pointer(&iidImageList)), uintptr(unsafe.Pointer(&il)))
	if err := winshell.Failed(r); err != nil {
		return nil, err
	}
	defer winshell.Release(il)
	var icon windows.Handle
	if err := winshell.Failed(winshell.Call(il, imageListGetIcon, uintptr(index), ildTransparent, uintptr(unsafe.Pointer(&icon)))); err != nil {
		return nil, err
	}
	defer winshell.DestroyIcon(icon)
	return winshell.IconImage(icon)
}

// small reports whether everything visible of img lies in its top left 48
// pixels.
func small(img image.Image) bool {
	b := img.Bounds()
	for y := b.Min.Y; y < b.Max.Y; y++ {
		for x := b.Min.X; x < b.Max.X; x++ {
			if _, _, _, a := img.At(x, y).RGBA(); a != 0 && (x-b.Min.X >= 48 || y-b.Min.Y >= 48) {
				return false
			}
		}
	}
	return true
}

// Thumbnail is the picture Explorer shows for file in its large views,
// drawn by whatever handler the computer has for the type, as a PNG of
// about px pixels. A file in the cloud may be fetched for it, which the
// cloud's own handler does the way Explorer expects.
func Thumbnail(ctx context.Context, file string, px int) ([]byte, error) {
	select {
	case thumbSlots <- struct{}{}:
	case <-ctx.Done():
		return nil, ctx.Err()
	}
	defer func() { <-thumbSlots }()
	p, err := windows.UTF16PtrFromString(file)
	if err != nil {
		return nil, err
	}
	var out []byte
	err = winshell.Apartment(func() error {
		var factory unsafe.Pointer
		r, _, _ := procItemFromParsing.Call(uintptr(unsafe.Pointer(p)), 0, uintptr(unsafe.Pointer(&iidShellItemImageFac)), uintptr(unsafe.Pointer(&factory)))
		if winshell.Failed(r) != nil {
			return ErrNoPicture
		}
		defer winshell.Release(factory)
		// GetImage takes the size as a SIZE by value, which fits a register.
		size := uintptr(uint32(px)) | uintptr(uint32(px))<<32
		var bm windows.Handle
		if winshell.Failed(winshell.Call(factory, imageFactoryGetImage, size, siigbfThumbnailOnly|siigbfBiggerSizeOK, uintptr(unsafe.Pointer(&bm)))) != nil {
			return ErrNoPicture
		}
		defer winshell.DeleteBitmap(bm)
		img, err := winshell.BitmapImage(bm)
		if err != nil {
			return ErrNoPicture
		}
		out, err = pngBytes(img)
		return err
	})
	return out, err
}

func pngBytes(img image.Image) ([]byte, error) {
	var b bytes.Buffer
	if err := png.Encode(&b, img); err != nil {
		return nil, err
	}
	return b.Bytes(), nil
}

func pngDataURL(img image.Image) (string, error) {
	b, err := pngBytes(img)
	if err != nil {
		return "", err
	}
	return "data:image/png;base64," + base64.StdEncoding.EncodeToString(b), nil
}
