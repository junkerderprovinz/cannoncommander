package xdg

import (
	"bytes"
	"image"
	"image/png"
	"path/filepath"
	"strings"
	"testing"
)

func TestAFileURIIsEscapedTheWayGLibDoes(t *testing.T) {
	cases := map[string]string{
		"/home/jdp/a b.png":          "file:///home/jdp/a%20b.png",
		"/tmp/Ä#1?;x%.txt":           "file:///tmp/%C3%84%231%3F%3Bx%25.txt",
		"/x/!$&'()*+,-.:=@_~/[a]{b}": "file:///x/!$&'()*+,-.:=@_~/%5Ba%5D%7Bb%7D",
	}
	for path, want := range cases {
		if got := FileURI(path); got != want {
			t.Errorf("%s: got %s, want %s", path, got, want)
		}
	}
	// The example of the thumbnail specification.
	if got := ThumbnailName(FileURI("/home/jens/photos/me.png")); got != "c6ee772d9e49320e97ec29a7eb5b1697.png" {
		t.Errorf("name %s", got)
	}
}

func TestAThumbnailSizeCoversThePixels(t *testing.T) {
	cases := map[int]string{1: "normal", 128: "normal", 129: "large", 256: "large", 400: "x-large", 1024: "xx-large", 5000: "xx-large"}
	for px, want := range cases {
		if got := ThumbnailSizes[ThumbnailSize(px)].Dir; got != want {
			t.Errorf("%d: got %s, want %s", px, got, want)
		}
	}
}

func TestTheTextOfAThumbnailSurvivesTheRoundTrip(t *testing.T) {
	var b bytes.Buffer
	if err := png.Encode(&b, image.NewNRGBA(image.Rect(0, 0, 4, 3))); err != nil {
		t.Fatal(err)
	}
	uri := FileURI("/home/jdp/a b.jpg")
	once, err := SetPNGText(b.Bytes(), map[string]string{"Thumb::URI": "file:///old", "Software": "test"})
	if err != nil {
		t.Fatal(err)
	}
	thumb, err := SetPNGText(once, map[string]string{"Thumb::URI": uri, "Thumb::MTime": "1700000000"})
	if err != nil {
		t.Fatal(err)
	}
	img, err := png.Decode(bytes.NewReader(thumb))
	if err != nil || img.Bounds() != image.Rect(0, 0, 4, 3) {
		t.Fatalf("the PNG broke: %v", err)
	}
	text, err := PNGText(thumb)
	if err != nil || len(text) != 3 || text["Thumb::URI"] != uri || text["Software"] != "test" || text["Thumb::MTime"] != "1700000000" {
		t.Fatalf("got %v %v", text, err)
	}
	if !ThumbnailFits(thumb, uri, 1700000000) {
		t.Error("the thumbnail does not fit its file")
	}
	if ThumbnailFits(thumb, uri, 1700000001) || ThumbnailFits(thumb, FileURI("/other"), 1700000000) {
		t.Error("the thumbnail fits a changed file")
	}
	if ThumbnailFits(b.Bytes(), uri, 0) {
		t.Error("a thumbnail without text fits")
	}
	if _, err := SetPNGText([]byte("GIF89a"), nil); err == nil {
		t.Error("a GIF took text")
	}
	if _, err := PNGText(thumb[:len(thumb)-5]); err == nil {
		t.Error("a cut PNG was read")
	}
}

func TestAThumbnailerIsReadFromItsFile(t *testing.T) {
	d := newDesk(t, "gdk-pixbuf-thumbnailer")
	d.write(filepath.Join(d.system, "thumbnailers", "gdk.thumbnailer"), "[Thumbnailer Entry]\nTryExec=gdk-pixbuf-thumbnailer\nExec=gdk-pixbuf-thumbnailer -s %s %u %o\nMimeType=image/png;image/jpeg;\n")
	d.write(filepath.Join(d.user, "thumbnailers", "gdk.thumbnailer"), "[Thumbnailer Entry]\nExec=\"/opt/my thumbs\" --in=%i --out %o 100%%\nMimeType=image/png;\n")
	d.write(filepath.Join(d.system, "thumbnailers", "missing.thumbnailer"), "[Thumbnailer Entry]\nTryExec=missing\nExec=missing %i %o\nMimeType=video/mp4;\n")
	d.write(filepath.Join(d.system, "thumbnailers", "pixbuf.thumbnailer"), "[Thumbnailer Entry]\nTryExec=gdk-pixbuf-thumbnailer\nExec=gdk-pixbuf-thumbnailer -s %s %u %o\nMimeType=image/gif;\n")
	all := d.Thumbnailers()
	if len(all) != 2 {
		t.Fatalf("got %+v", all)
	}
	args, err := all[0].Command("/home/jdp/a b.png", "/tmp/out.png", 256)
	if err != nil || strings.Join(args, "|") != "/opt/my thumbs|--in=/home/jdp/a b.png|--out|/tmp/out.png|100%" {
		t.Errorf("own: %q %v", args, err)
	}
	args, err = all[1].Command("/home/jdp/a b.gif", "/tmp/out.png", 128)
	if err != nil || strings.Join(args, "|") != "gdk-pixbuf-thumbnailer|-s|128|file:///home/jdp/a%20b.gif|/tmp/out.png" {
		t.Errorf("system: %q %v", args, err)
	}
}
