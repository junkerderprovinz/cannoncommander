package nativeshell

import (
	"crypto/md5"
	"encoding/hex"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"

	"github.com/junkerderprovinz/cannoncommander/backend/internal/xdg"
)

// Icon returns the system's icon for a file or folder on disk: a desktop
// entry's own, an AppImage's where desktop integration put one, a folder's
// that says what it holds, and otherwise the icon of its type.
func Icon(file string, px int) ([]byte, string, error) {
	src, ok := current().iconOf(file, px)
	if !ok {
		return nil, "", ErrNoPicture
	}
	return picture(src, px)
}

// TypeIcon returns the icon of files called name, or of folders, without
// looking at any file.
func TypeIcon(name string, dir bool, px int) ([]byte, string, error) {
	src, ok := current().typeIcon(name, dir, px)
	if !ok {
		return nil, "", ErrNoPicture
	}
	return picture(src, px)
}

func (k *desk) typeIcon(name string, dir bool, px int) (string, bool) {
	if dir {
		return k.icons.Find(px, "folder", "inode-directory")
	}
	m := k.db.TypeByName(name)
	if m == "" {
		m = "application/octet-stream"
	}
	return k.icons.Find(px, k.db.IconNames(m)...)
}

func (k *desk) iconOf(file string, px int) (string, bool) {
	info, err := os.Stat(file)
	if err != nil {
		return "", false
	}
	if info.IsDir() {
		return k.icons.Find(px, k.folderIcon(file), "folder")
	}
	name := filepath.Base(file)
	if strings.HasSuffix(name, ".desktop") {
		if kf, err := xdg.ReadKeyFile(file); err == nil {
			if icon := kf["Desktop Entry"].String("Icon"); icon != "" {
				if src, ok := k.namedIcon(icon, px); ok {
					return src, true
				}
			}
		}
	}
	if strings.EqualFold(filepath.Ext(name), ".appimage") {
		if src, ok := k.appImageIcon(file, px); ok {
			return src, true
		}
	}
	return k.icons.Find(px, k.db.IconNames(k.db.TypeOf(file))...)
}

// namedIcon finds an icon that a desktop entry or an action names, by its
// path or by its name.
func (k *desk) namedIcon(name string, px int) (string, bool) {
	if !filepath.IsAbs(name) {
		if src, ok := k.icons.Find(px, name); ok {
			return src, true
		}
	}
	return k.AppIcon(name)
}

var userDirIcons = map[string]string{
	"DESKTOP":     "user-desktop",
	"DOCUMENTS":   "folder-documents",
	"DOWNLOAD":    "folder-download",
	"MUSIC":       "folder-music",
	"PICTURES":    "folder-pictures",
	"VIDEOS":      "folder-videos",
	"TEMPLATES":   "folder-templates",
	"PUBLICSHARE": "folder-publicshare",
}

func (k *desk) folderIcon(dir string) string {
	dir = filepath.Clean(dir)
	if dir == filepath.Clean(k.Home) {
		return "user-home"
	}
	for name, path := range k.userDirs {
		if icon, ok := userDirIcons[name]; ok && path == dir {
			return icon
		}
	}
	return "folder"
}

// appImageIcon finds the icon that desktop integration, such as appimaged
// or AppImageLauncher, put into hicolor for an AppImage, named after the
// md5 of its URI, in the size nearest to px.
func (k *desk) appImageIcon(file string, px int) (string, bool) {
	sum := md5.Sum([]byte(xdg.FileURI(file)))
	prefix := "appimagekit_" + hex.EncodeToString(sum[:]) + "_"
	hicolor := filepath.Join(k.Data[0], "icons", "hicolor")
	sizes, _ := os.ReadDir(hicolor)
	best, nearest := "", 0
	for _, size := range sizes {
		edge, err := strconv.Atoi(strings.Split(size.Name(), "x")[0])
		if err != nil {
			edge = px
		}
		icons, _ := os.ReadDir(filepath.Join(hicolor, size.Name(), "apps"))
		for _, icon := range icons {
			if !strings.HasPrefix(icon.Name(), prefix) {
				continue
			}
			if d := max(edge-px, px-edge); best == "" || d < nearest {
				best, nearest = filepath.Join(hicolor, size.Name(), "apps", icon.Name()), d
			}
		}
	}
	return best, best != ""
}

type loaded struct {
	b         []byte
	mediaType string
}

var pictures = struct {
	sync.Mutex
	m map[string]loaded
}{m: map[string]loaded{}}

// picture loads an icon file for px pixels and keeps it, since a folder
// shows the same few icons again and again.
func picture(file string, px int) ([]byte, string, error) {
	key := strconv.Itoa(px) + "/" + file
	pictures.Lock()
	p, ok := pictures.m[key]
	pictures.Unlock()
	if ok {
		return p.b, p.mediaType, nil
	}
	b, mediaType, err := xdg.LoadIcon(file, px)
	if err != nil {
		return nil, "", ErrNoPicture
	}
	pictures.Lock()
	// Each AppImage brings an icon of its own, so the map is emptied
	// before it grows without end.
	if len(pictures.m) > 2000 {
		clear(pictures.m)
	}
	pictures.m[key] = loaded{b, mediaType}
	pictures.Unlock()
	return b, mediaType, nil
}
