package nativeshell

import (
	"bytes"
	"context"
	"errors"
	"image"
	"image/color"
	"image/png"
	"os"
	"path/filepath"
	"testing"
	"unsafe"

	"golang.org/x/sys/windows"
)

func decodePNG(t *testing.T, b []byte) image.Image {
	t.Helper()
	img, err := png.Decode(bytes.NewReader(b))
	if err != nil {
		t.Fatal(err)
	}
	return img
}

func TestATypeHasTheIconOfItsSize(t *testing.T) {
	for _, px := range []int{16, 32, 48} {
		b, kind, err := TypeIcon("notes.txt", false, px)
		if err != nil {
			t.Fatal(err)
		}
		if kind != "image/png" {
			t.Fatalf("got %s", kind)
		}
		if got := decodePNG(t, b).Bounds().Dx(); got != px {
			t.Errorf("asked for %d, got %d", px, got)
		}
	}
	if _, _, err := TypeIcon("Documents", true, 32); err != nil {
		t.Fatal(err)
	}
}

func TestAProgramShowsItsOwnIcon(t *testing.T) {
	exe := filepath.Join(os.Getenv("SystemRoot"), "System32", "notepad.exe")
	own, _, err := Icon(exe, 32)
	if err != nil {
		t.Fatal(err)
	}
	generic, _, err := TypeIcon("other.exe", false, 32)
	if err != nil {
		t.Fatal(err)
	}
	if bytes.Equal(own, generic) {
		t.Fatal("notepad shows the icon of every program")
	}
	again, _, _ := Icon(exe, 32)
	if !bytes.Equal(own, again) {
		t.Fatal("the icon changed between two looks")
	}
}

func TestAPictureGetsAThumbnailAndTextDoesNot(t *testing.T) {
	dir := t.TempDir()
	pic := image.NewRGBA(image.Rect(0, 0, 400, 300))
	for i := range pic.Pix {
		pic.Pix[i] = 0xff
	}
	pic.Set(10, 10, color.RGBA{R: 0xff, A: 0xff})
	var b bytes.Buffer
	if err := png.Encode(&b, pic); err != nil {
		t.Fatal(err)
	}
	file := filepath.Join(dir, "photo.png")
	if err := os.WriteFile(file, b.Bytes(), 0o666); err != nil {
		t.Fatal(err)
	}
	thumb, err := Thumbnail(context.Background(), file, 128)
	if err != nil {
		t.Fatal(err)
	}
	if got := decodePNG(t, thumb).Bounds(); got.Dx() > 400 || got.Dx() < 64 {
		t.Fatalf("got %v", got)
	}
	text := filepath.Join(dir, "notes.txt")
	if err := os.WriteFile(text, []byte("hi"), 0o666); err != nil {
		t.Fatal(err)
	}
	if _, err := Thumbnail(context.Background(), text, 128); !errors.Is(err, ErrNoPicture) {
		t.Fatalf("text got %v", err)
	}
}

func TestOnlyProgramsRunAsAdministrator(t *testing.T) {
	if !CanRunAsAdmin(`C:\tools\setup.exe`) || !CanRunAsAdmin(`C:\tools\build.cmd`) {
		t.Fatal("a program cannot run as administrator")
	}
	for _, f := range []string{`C:\notes.txt`, `C:\README`, `C:\photo.png`} {
		if CanRunAsAdmin(f) {
			t.Errorf("%s runs as administrator", f)
		}
	}
	if err := RunAsAdmin(`C:\notes.txt`); !errors.Is(err, ErrUnsupported) {
		t.Fatalf("got %v", err)
	}
}

func TestALabelLosesItsAccessKeyAndShortcut(t *testing.T) {
	for in, want := range map[string]string{
		"&Open":                 "Open",
		"Copy &as path\tCtrl+C": "Copy as path",
		"Save && quit":          "Save & quit",
		"送る(&N)":                "送る",
		"  Plain  ":             "Plain",
	} {
		if got := cleanLabel(in); got != want {
			t.Errorf("%q: got %q, want %q", in, got, want)
		}
	}
}

func TestHiddenRowsLeaveNoStraySeparators(t *testing.T) {
	appendMenu := user32.NewProc("AppendMenuW")
	const mfString, mfSeparator = 0x0, 0x800
	menu, _, _ := procCreatePopupMenu.Call()
	defer procDestroyMenu.Call(menu)
	for _, row := range []string{"-", "a", "-", "-", "b", "-"} {
		if row == "-" {
			appendMenu.Call(menu, mfSeparator, 0, 0)
			continue
		}
		p, _ := windows.UTF16PtrFromString(row)
		appendMenu.Call(menu, mfString, 1, uintptr(unsafe.Pointer(p)))
	}
	tidy(windows.Handle(menu))
	var got []bool
	n, _, _ := procGetMenuItemCount.Call(menu)
	for i := 0; i < int(n); i++ {
		it := menuItemInfo{mask: miimFType}
		it.size = uint32(unsafe.Sizeof(it))
		procGetMenuItemInfo.Call(menu, uintptr(i), 1, uintptr(unsafe.Pointer(&it)))
		got = append(got, it.ftype&mftSep != 0)
	}
	if len(got) != 3 || got[0] || !got[1] || got[2] {
		t.Fatalf("separators at %v", got)
	}
}

func TestAMenuNeedsTheAppInFront(t *testing.T) {
	if _, err := ShowMenu([]string{os.TempDir()}, MenuOptions{}); !errors.Is(err, errNotInFront) {
		t.Fatalf("got %v", err)
	}
}
