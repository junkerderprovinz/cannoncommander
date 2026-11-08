package nativeshell

import (
	"crypto/md5"
	"encoding/hex"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"syscall"
	"testing"

	"github.com/junkerderprovinz/cannoncommander/backend/internal/xdg"
)

// iconTheme installs the theme Test, picked in GTK's settings, with the
// given icons in its 32 pixel folders and a scalable one for programs.
func (f *fixture) iconTheme(icons ...string) {
	theme := filepath.Join(f.system, "icons", "Test")
	f.write(filepath.Join(theme, "index.theme"), "[Icon Theme]\nName=Test\nDirectories=32x32/places,32x32/mimetypes,scalable/apps\n\n"+
		"[32x32/places]\nSize=32\nType=Fixed\n\n[32x32/mimetypes]\nSize=32\nType=Fixed\n\n[scalable/apps]\nSize=48\nMinSize=16\nMaxSize=256\nType=Scalable\n")
	for _, icon := range icons {
		f.write(filepath.Join(theme, icon), "<svg>"+icon+"</svg>")
	}
	f.write(filepath.Join(f.config, "gtk-3.0", "settings.ini"), "[Settings]\ngtk-icon-theme-name=Test\n")
}

func TestAnItemOnDiskGetsTheIconOfWhatItIs(t *testing.T) {
	f := newFixture(t)
	f.iconTheme("32x32/places/folder.svg", "32x32/places/user-home.svg", "32x32/places/folder-download.svg",
		"32x32/mimetypes/image-png.svg", "32x32/mimetypes/text-x-generic.svg", "32x32/mimetypes/application-x-generic.svg", "scalable/apps/editor.svg")
	f.write(filepath.Join(f.config, "user-dirs.dirs"), "XDG_DOWNLOAD_DIR=\"$HOME/Downloads\"\nXDG_MUSIC_DIR=\"$HOME/Music\"\n")
	f.write(filepath.Join(f.home, "Downloads", "a.txt"), "")
	f.write(filepath.Join(f.home, "Music", "a.txt"), "")
	f.files("photos/")
	f.write(filepath.Join(f.work, "editor.desktop"), "[Desktop Entry]\nType=Application\nName=Editor\nIcon=editor\nExec=editor\n")
	own := f.write(filepath.Join(f.home, "own.svg"), "<svg/>")
	f.write(filepath.Join(f.work, "own.desktop"), "[Desktop Entry]\nType=Application\nName=Own\nIcon="+own+"\nExec=own\n")
	f.write(filepath.Join(f.work, "broken.desktop"), "[Desktop Entry]\nType=Application\nName=Broken\nIcon=no-such-icon\nExec=x\n")
	appImage := f.files("Tool.AppImage")[0]
	sum := md5.Sum([]byte(xdg.FileURI(appImage)))
	for _, size := range []string{"48x48", "256x256"} {
		f.write(filepath.Join(f.user, "icons", "hicolor", size, "apps", "appimagekit_"+hex.EncodeToString(sum[:])+"_tool.png"), "png")
	}
	f.write(filepath.Join(f.work, "other.AppImage"), "\x7fELF\x00\x01")
	f.files("a.png", "notes.txt")
	k := f.desk()
	theme := filepath.Join(f.system, "icons", "Test")
	cases := map[string]string{
		f.home:                                  filepath.Join(theme, "32x32/places/user-home.svg"),
		filepath.Join(f.home, "Downloads"):      filepath.Join(theme, "32x32/places/folder-download.svg"),
		filepath.Join(f.home, "Music"):          filepath.Join(theme, "32x32/places/folder.svg"),
		filepath.Join(f.work, "photos"):         filepath.Join(theme, "32x32/places/folder.svg"),
		filepath.Join(f.work, "a.png"):          filepath.Join(theme, "32x32/mimetypes/image-png.svg"),
		filepath.Join(f.work, "notes.txt"):      filepath.Join(theme, "32x32/mimetypes/text-x-generic.svg"),
		filepath.Join(f.work, "editor.desktop"): filepath.Join(theme, "scalable/apps/editor.svg"),
		filepath.Join(f.work, "own.desktop"):    own,
		filepath.Join(f.work, "broken.desktop"): filepath.Join(theme, "32x32/mimetypes/text-x-generic.svg"),
		appImage:                                filepath.Join(f.user, "icons", "hicolor", "48x48", "apps", "appimagekit_"+hex.EncodeToString(sum[:])+"_tool.png"),
		filepath.Join(f.work, "other.AppImage"): filepath.Join(theme, "32x32/mimetypes/application-x-generic.svg"),
		filepath.Join(f.work, "gone.txt"):       "",
	}
	for file, want := range cases {
		if got, _ := k.iconOf(file, 32); got != want {
			t.Errorf("%s: got %s, want %s", strings.TrimPrefix(file, f.home), got, want)
		}
	}
}

func TestATypeIconNeedsOnlyTheName(t *testing.T) {
	f := newFixture(t)
	f.iconTheme("32x32/places/folder.svg", "32x32/mimetypes/image-png.svg", "32x32/mimetypes/application-x-generic.svg")
	k := f.desk()
	theme := filepath.Join(f.system, "icons", "Test")
	cases := []struct {
		name string
		dir  bool
		want string
	}{
		{"never-made.png", false, "32x32/mimetypes/image-png.svg"},
		{"photos.png", true, "32x32/places/folder.svg"},
		{"unknown.zzz", false, "32x32/mimetypes/application-x-generic.svg"},
	}
	for _, c := range cases {
		if got, ok := k.typeIcon(c.name, c.dir, 32); !ok || got != filepath.Join(theme, c.want) {
			t.Errorf("%s: got %s", c.name, got)
		}
	}
}

func TestIconsComeBackAsPictures(t *testing.T) {
	f := newFixture(t)
	f.iconTheme("32x32/mimetypes/image-png.svg")
	f.install()
	png := f.files("a.png")[0]
	if b, mediaType, err := Icon(png, 32); err != nil || mediaType != "image/svg+xml" || string(b) != "<svg>32x32/mimetypes/image-png.svg</svg>" {
		t.Errorf("icon: %s %q %v", mediaType, b, err)
	}
	if b, mediaType, err := TypeIcon("b.png", false, 32); err != nil || mediaType != "image/svg+xml" || len(b) == 0 {
		t.Errorf("type icon: %s %v", mediaType, err)
	}
	if _, _, err := Icon(f.files("notes.txt")[0], 32); !errors.Is(err, ErrNoPicture) {
		t.Errorf("no icon: %v", err)
	}
	if _, _, err := TypeIcon("x", true, 32); !errors.Is(err, ErrNoPicture) {
		t.Errorf("no folder icon: %v", err)
	}
}

func TestAnIconNamedByAFileMustBeASmallPicture(t *testing.T) {
	f := newFixture(t)
	f.iconTheme("32x32/mimetypes/text-x-generic.svg")
	passwd := f.write(filepath.Join(f.home, "passwd"), "root:x:0:0:root:/root:/bin/bash\n")
	big := f.write(filepath.Join(f.home, "big.png"), "png")
	if err := os.Truncate(big, 2<<20); err != nil {
		t.Fatal(err)
	}
	fifo := filepath.Join(f.home, "fifo.png")
	if err := syscall.Mkfifo(fifo, 0o644); err != nil {
		t.Fatal(err)
	}
	ok := f.write(filepath.Join(f.home, "ok.png"), "png")
	for name, icon := range map[string]string{"text.desktop": passwd, "big.desktop": big, "fifo.desktop": fifo, "ok.desktop": ok} {
		f.write(filepath.Join(f.work, name), "[Desktop Entry]\nType=Application\nName=X\nIcon="+icon+"\nExec=x\n")
	}
	appImage := f.files("Tool.AppImage")[0]
	sum := md5.Sum([]byte(xdg.FileURI(appImage)))
	f.write(filepath.Join(f.user, "icons", "hicolor", "48x48", "apps", "appimagekit_"+hex.EncodeToString(sum[:])+"_tool.png"), "png")
	if err := os.Truncate(filepath.Join(f.user, "icons", "hicolor", "48x48", "apps", "appimagekit_"+hex.EncodeToString(sum[:])+"_tool.png"), 2<<20); err != nil {
		t.Fatal(err)
	}
	k := f.desk()
	generic := filepath.Join(f.system, "icons", "Test", "32x32/mimetypes/text-x-generic.svg")
	cases := map[string]string{"text.desktop": generic, "big.desktop": generic, "fifo.desktop": generic, "ok.desktop": ok, "Tool.AppImage": generic}
	for name, want := range cases {
		if got, _ := k.iconOf(filepath.Join(f.work, name), 32); got != want {
			t.Errorf("%s: got %s, want %s", name, got, want)
		}
	}
}
