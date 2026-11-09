package xdg

import (
	"bytes"
	"compress/gzip"
	"image"
	"image/png"
	"os"
	"path/filepath"
	"testing"
)

func TestAnAppIconIsFoundInTheThemes(t *testing.T) {
	d := newDesk(t)
	big := d.write(filepath.Join(d.system, "icons", "hicolor", "48x48", "apps", "gedit.png"), "png")
	d.write(filepath.Join(d.system, "icons", "hicolor", "256x256", "apps", "gedit.png"), "png")
	themed := d.write(filepath.Join(d.home, ".icons", "Papirus", "64x64", "apps", "vlc.svg"), "<svg/>")
	old := d.write(filepath.Join(d.system, "pixmaps", "xterm.png"), "png")
	abs := d.write(filepath.Join(d.home, "apps", "own.png"), "png")
	cases := map[string]string{"gedit": big, "vlc": themed, "xterm": old, abs: abs}
	for name, want := range cases {
		if got, ok := d.AppIcon(name); !ok || got != want {
			t.Errorf("%s: got %s %v, want %s", name, got, ok, want)
		}
	}
	for _, name := range []string{"", "missing", "../../pixmaps/xterm", filepath.Join(d.home, "nothing.png")} {
		if got, ok := d.AppIcon(name); ok {
			t.Errorf("%q: found %s", name, got)
		}
	}
}

// theme installs an icon theme with the given index.theme into the
// system's data folder.
func (d *desk) theme(name, index string, icons ...string) {
	d.write(filepath.Join(d.system, "icons", name, "index.theme"), index)
	for _, icon := range icons {
		d.write(filepath.Join(d.system, "icons", name, icon), "x")
	}
}

func TestAnIconComesFromTheThemeItsParentsOrHicolor(t *testing.T) {
	d := newDesk(t)
	d.theme("Mine", "[Icon Theme]\nName=Mine\nInherits=Base\nDirectories=16x16/mimetypes,64x64/mimetypes,scalable/places\n\n"+
		"[16x16/mimetypes]\nSize=16\nType=Fixed\n\n[64x64/mimetypes]\nSize=64\nType=Fixed\n\n[scalable/places]\nSize=48\nMinSize=8\nMaxSize=512\nType=Scalable\n",
		"16x16/mimetypes/text-plain.png", "64x64/mimetypes/text-plain.png", "scalable/places/folder.svg", "64x64/mimetypes/text-x-generic.png")
	d.theme("Base", "[Icon Theme]\nDirectories=32x32/mimetypes,32@2x/mimetypes\n\n[32x32/mimetypes]\nSize=32\n\n[32@2x/mimetypes]\nSize=32\nScale=2\n",
		"32x32/mimetypes/image-png.png", "32@2x/mimetypes/video-mp4.png")
	d.theme("hicolor", "[Icon Theme]\nDirectories=48x48/apps\n\n[48x48/apps]\nSize=48\nType=Threshold\n", "48x48/apps/gimp.png", "48x48/apps/text-x-generic.png")
	d.write(filepath.Join(d.system, "pixmaps", "old.svg"), "<svg/>")
	ic := d.Icons("Mine")
	icons := filepath.Join(d.system, "icons")
	cases := []struct {
		px    int
		names []string
		want  string
	}{
		{16, []string{"text-plain"}, "Mine/16x16/mimetypes/text-plain.png"},
		{48, []string{"text-plain"}, "Mine/64x64/mimetypes/text-plain.png"},
		{22, []string{"text-plain"}, "Mine/16x16/mimetypes/text-plain.png"},
		{256, []string{"folder"}, "Mine/scalable/places/folder.svg"},
		{32, []string{"image-png"}, "Base/32x32/mimetypes/image-png.png"},
		{32, []string{"gimp"}, "hicolor/48x48/apps/gimp.png"},
		// The theme's generic icon wins over hicolor's.
		{32, []string{"text-x-python", "text-x-generic"}, "Mine/64x64/mimetypes/text-x-generic.png"},
		{32, []string{"old"}, "../pixmaps/old.svg"},
	}
	for _, c := range cases {
		got, ok := ic.Find(c.px, c.names...)
		if want := filepath.Join(icons, c.want); !ok || got != want {
			t.Errorf("%v at %d: got %s, want %s", c.names, c.px, got, want)
		}
	}
	for _, names := range [][]string{{"video-mp4"}, {"missing"}, nil} {
		if got, ok := ic.Find(32, names...); ok {
			t.Errorf("%v: found %s", names, got)
		}
	}
}

func TestTheIconThemeComesFromTheSettings(t *testing.T) {
	d := newDesk(t)
	if got := d.ThemeName(); got != "Adwaita" {
		t.Errorf("nothing set: %s", got)
	}
	d.Desktops = []string{"kde"}
	if got := d.ThemeName(); got != "breeze" {
		t.Errorf("nothing set on KDE: %s", got)
	}
	d.write(filepath.Join(d.config, "gtk-3.0", "settings.ini"), "[Settings]\ngtk-icon-theme-name=Papirus-Dark\n")
	if got := d.ThemeName(); got != "Papirus-Dark" {
		t.Errorf("GTK's on KDE: %s", got)
	}
	d.write(filepath.Join(d.config, "kdeglobals"), "[General]\nName=x\n\n[Icons]\nTheme=breeze-dark\n")
	if got := d.ThemeName(); got != "breeze-dark" {
		t.Errorf("KDE's: %s", got)
	}
	d.Desktops = []string{"gnome"}
	if got := d.ThemeName(); got != "Papirus-Dark" {
		t.Errorf("GTK's on GNOME: %s", got)
	}
}

func TestAnIconIsLoadedForItsSquare(t *testing.T) {
	d := newDesk(t)
	pic := func(w, h int) string {
		var b bytes.Buffer
		if err := png.Encode(&b, image.NewNRGBA(image.Rect(0, 0, w, h))); err != nil {
			t.Fatal(err)
		}
		return b.String()
	}
	exact := pic(32, 32)
	d.write(filepath.Join(d.home, "exact.png"), exact)
	d.write(filepath.Join(d.home, "wide.png"), pic(200, 100))
	d.write(filepath.Join(d.home, "plain.svg"), "<svg/>")
	var z bytes.Buffer
	w := gzip.NewWriter(&z)
	w.Write([]byte("<svg>zipped</svg>"))
	w.Close()
	d.write(filepath.Join(d.home, "zipped.svgz"), z.String())

	if b, typ, err := LoadIcon(filepath.Join(d.home, "exact.png"), 32); err != nil || typ != "image/png" || string(b) != exact {
		t.Errorf("exact: %s %v", typ, err)
	}
	b, typ, err := LoadIcon(filepath.Join(d.home, "wide.png"), 32)
	if err != nil || typ != "image/png" {
		t.Fatalf("wide: %s %v", typ, err)
	}
	if img, err := png.Decode(bytes.NewReader(b)); err != nil || img.Bounds() != image.Rect(0, 0, 32, 32) {
		t.Errorf("wide: %v", err)
	}
	if b, typ, err := LoadIcon(filepath.Join(d.home, "plain.svg"), 32); err != nil || typ != "image/svg+xml" || string(b) != "<svg/>" {
		t.Errorf("svg: %s %q %v", typ, b, err)
	}
	if b, typ, err := LoadIcon(filepath.Join(d.home, "zipped.svgz"), 32); err != nil || typ != "image/svg+xml" || string(b) != "<svg>zipped</svg>" {
		t.Errorf("svgz: %s %q %v", typ, b, err)
	}
	if _, _, err := LoadIcon(filepath.Join(d.home, "plain.svg.png"), 32); err == nil {
		t.Error("a missing file loaded")
	}
}

func TestAWideIconKeepsItsShape(t *testing.T) {
	wide := image.NewNRGBA(image.Rect(0, 0, 200, 100))
	for i := range wide.Pix {
		wide.Pix[i] = 0xff
	}
	b, err := ScaleIcon(wide, 24)
	if err != nil {
		t.Fatal(err)
	}
	img, err := png.Decode(bytes.NewReader(b))
	if err != nil || img.Bounds() != image.Rect(0, 0, 24, 24) {
		t.Fatalf("got %v %v", img, err)
	}
	if _, _, _, a := img.At(12, 0).RGBA(); a != 0 {
		t.Error("the top is not clear")
	}
	if _, _, _, a := img.At(0, 12).RGBA(); a == 0 {
		t.Error("the middle is clear")
	}
}

func TestTheUsersFoldersHaveTheHomeFilledIn(t *testing.T) {
	d := newDesk(t)
	d.write(filepath.Join(d.config, "user-dirs.dirs"), "# written by xdg-user-dirs-update\nXDG_DESKTOP_DIR=\"$HOME/Schreibtisch\"\nXDG_DOWNLOAD_DIR=\"/data/Downloads/\"\nXDG_TEMPLATES_DIR=\"$HOME/\"\nXDG_MUSIC_DIR=\"relative\"\nOTHER=\"$HOME/x\"\n")
	got := d.UserDirs()
	if len(got) != 2 || got["DESKTOP"] != filepath.Join(d.home, "Schreibtisch") || got["DOWNLOAD"] != "/data/Downloads" {
		t.Fatalf("got %v", got)
	}
}

func TestOnlySmallPicturesAreReadAsIcons(t *testing.T) {
	d := newDesk(t)
	var small bytes.Buffer
	if err := png.Encode(&small, image.NewNRGBA(image.Rect(0, 0, 16, 16))); err != nil {
		t.Fatal(err)
	}
	ok := d.write(filepath.Join(d.home, "ok.png"), small.String())
	passwd := d.write(filepath.Join(d.home, "passwd"), "root:x:0:0:root:/root:/bin/bash\n")
	named := d.write(filepath.Join(d.home, "passwd.png"), "root:x:0:0:root:/root:/bin/bash\n")
	xpm := d.write(filepath.Join(d.home, "old.xpm"), "/* XPM */\n")
	big := d.write(filepath.Join(d.home, "big.png"), small.String())
	if err := os.Truncate(big, 2<<20); err != nil {
		t.Fatal(err)
	}
	// A PNG of a few bytes that claims to be 20000 pixels wide.
	var wide bytes.Buffer
	if err := png.Encode(&wide, image.NewGray(image.Rect(0, 0, 20000, 1))); err != nil {
		t.Fatal(err)
	}
	huge := d.write(filepath.Join(d.home, "huge.png"), wide.String())

	if !IconFile(ok) {
		t.Error("a small PNG is no icon")
	}
	if _, _, err := LoadIcon(ok, 32); err != nil {
		t.Errorf("a small PNG: %v", err)
	}
	for _, file := range []string{passwd, named, xpm, big, huge, d.home} {
		if _, _, err := LoadIcon(file, 32); err == nil {
			t.Errorf("%s was read", filepath.Base(file))
		}
		if got, found := d.AppIcon(file); found && file != huge && file != named {
			t.Errorf("%s was taken as an icon: %s", filepath.Base(file), got)
		}
	}
}
