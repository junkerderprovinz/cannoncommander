package main

import (
	"bytes"
	"debug/pe"
	"encoding/binary"
	"fmt"
	"image"
	"image/png"
	"os"
	"path/filepath"
	"testing"
)

// A corner of the icon that is not fully clear shows a tile behind the
// cannons in shortcuts, the taskbar and the tray, where the small sizes go.
func clearCorners(t *testing.T, name string, img image.Image) {
	t.Helper()
	b := img.Bounds()
	for _, p := range []image.Point{b.Min, {b.Max.X - 1, b.Min.Y}, {b.Min.X, b.Max.Y - 1}, {b.Max.X - 1, b.Max.Y - 1}} {
		if _, _, _, a := img.At(p.X, p.Y).RGBA(); a != 0 {
			t.Errorf("%s at %d px: corner %v has alpha %d", name, b.Dx(), p, a>>8)
		}
	}
}

// iconImage decodes one image of an icon: a PNG, or a 32-bit DIB as the
// small sizes of a Windows icon usually are.
func iconImage(raw []byte) (image.Image, error) {
	if bytes.HasPrefix(raw, []byte("\x89PNG")) {
		return png.Decode(bytes.NewReader(raw))
	}
	if len(raw) < 40 {
		return nil, fmt.Errorf("short icon image")
	}
	w := int(int32(binary.LittleEndian.Uint32(raw[4:])))
	h := int(int32(binary.LittleEndian.Uint32(raw[8:]))) / 2
	if bpp := binary.LittleEndian.Uint16(raw[14:]); bpp != 32 {
		return nil, fmt.Errorf("%d px image with %d bits per pixel", w, bpp)
	}
	px := raw[binary.LittleEndian.Uint32(raw[0:]):]
	if len(px) < w*h*4 {
		return nil, fmt.Errorf("%d px image cut short", w)
	}
	img := image.NewNRGBA(image.Rect(0, 0, w, h))
	for y := 0; y < h; y++ {
		row := px[(h-1-y)*w*4:]
		for x := 0; x < w; x++ {
			c := row[x*4:]
			img.Pix[img.PixOffset(x, y)+0], img.Pix[img.PixOffset(x, y)+1], img.Pix[img.PixOffset(x, y)+2], img.Pix[img.PixOffset(x, y)+3] = c[2], c[1], c[0], c[3]
		}
	}
	return img, nil
}

// icoImages returns the images of an .ico file.
func icoImages(raw []byte) ([]image.Image, error) {
	if len(raw) < 6 {
		return nil, fmt.Errorf("short icon file")
	}
	var out []image.Image
	for i := 0; i < int(binary.LittleEndian.Uint16(raw[4:])); i++ {
		e := raw[6+16*i:]
		size, at := binary.LittleEndian.Uint32(e[8:]), binary.LittleEndian.Uint32(e[12:])
		img, err := iconImage(raw[at : at+size])
		if err != nil {
			return nil, err
		}
		out = append(out, img)
	}
	return out, nil
}

// exeIcons returns the RT_ICON images in the resources of a Windows program.
func exeIcons(path string) ([]image.Image, error) {
	f, err := pe.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	sec := f.Section(".rsrc")
	if sec == nil {
		return nil, fmt.Errorf("no resources")
	}
	data, err := sec.Data()
	if err != nil {
		return nil, err
	}
	entries := func(at uint32) [][2]uint32 {
		d := data[at:]
		n := int(binary.LittleEndian.Uint16(d[12:])) + int(binary.LittleEndian.Uint16(d[14:]))
		var out [][2]uint32
		for i := 0; i < n; i++ {
			e := d[16+8*i:]
			out = append(out, [2]uint32{binary.LittleEndian.Uint32(e), binary.LittleEndian.Uint32(e[4:])})
		}
		return out
	}
	const rtIcon, subdir = 3, 0x80000000
	var out []image.Image
	for _, typ := range entries(0) {
		if typ[0] != rtIcon {
			continue
		}
		for _, id := range entries(typ[1] &^ subdir) {
			for _, lang := range entries(id[1] &^ subdir) {
				leaf := data[lang[1]:]
				rva, size := binary.LittleEndian.Uint32(leaf), binary.LittleEndian.Uint32(leaf[4:])
				off := rva - sec.VirtualAddress
				img, err := iconImage(data[off : off+size])
				if err != nil {
					return nil, err
				}
				out = append(out, img)
			}
		}
	}
	return out, nil
}

// icnsImages returns the PNG images of a macOS .icns file.
func icnsImages(raw []byte) ([]image.Image, error) {
	if len(raw) < 8 || string(raw[:4]) != "icns" {
		return nil, fmt.Errorf("not an icns file")
	}
	var out []image.Image
	for at := 8; at+8 <= len(raw); {
		size := int(binary.BigEndian.Uint32(raw[at+4:]))
		if size < 8 || at+size > len(raw) {
			return nil, fmt.Errorf("broken entry at %d", at)
		}
		if body := raw[at+8 : at+size]; bytes.HasPrefix(body, []byte("\x89PNG")) {
			img, err := png.Decode(bytes.NewReader(body))
			if err != nil {
				return nil, err
			}
			out = append(out, img)
		}
		at += size
	}
	return out, nil
}

func sizesOf(imgs []image.Image) map[int]bool {
	got := map[int]bool{}
	for _, img := range imgs {
		got[img.Bounds().Dx()] = true
	}
	return got
}

func TestTheWindowsIconHasEverySizeWithClearCorners(t *testing.T) {
	raw, err := os.ReadFile("icons/icon.ico")
	if err != nil {
		t.Fatal(err)
	}
	imgs, err := icoImages(raw)
	if err != nil {
		t.Fatal(err)
	}
	got := sizesOf(imgs)
	for _, want := range []int{16, 24, 32, 48, 64, 128, 256} {
		if !got[want] {
			t.Errorf("no %d px image in %v", want, got)
		}
	}
	for _, img := range imgs {
		clearCorners(t, "icon.ico", img)
	}
}

func TestTheMacIconHasClearCorners(t *testing.T) {
	raw, err := os.ReadFile("icons/icons.icns")
	if err != nil {
		t.Fatal(err)
	}
	imgs, err := icnsImages(raw)
	if err != nil {
		t.Fatal(err)
	}
	if got := sizesOf(imgs); !got[16] || !got[32] || !got[1024] {
		t.Errorf("sizes %v", got)
	}
	for _, img := range imgs {
		clearCorners(t, "icons.icns", img)
	}
}

func TestTheTrayIconHasClearCorners(t *testing.T) {
	img, err := png.Decode(bytes.NewReader(trayIcon))
	if err != nil {
		t.Fatal(err)
	}
	clearCorners(t, "tray", img)
}

// CC_EXE names built programs, a list as PATH writes it, so the desktop build
// checks what it really ships, with every icon size each program carries.
func TestTheBuiltProgramShowsNoTileAtAnySize(t *testing.T) {
	paths := filepath.SplitList(os.Getenv("CC_EXE"))
	if len(paths) == 0 {
		t.Skip("CC_EXE names no built program")
	}
	for _, path := range paths {
		imgs, err := exeIcons(path)
		if err != nil {
			t.Fatal(err)
		}
		if len(imgs) < 4 {
			t.Fatalf("%d icon images in %s", len(imgs), path)
		}
		for _, img := range imgs {
			clearCorners(t, filepath.Base(path), img)
		}
	}
}
