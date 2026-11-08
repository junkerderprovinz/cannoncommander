package xdg

import (
	"bytes"
	"compress/gzip"
	"errors"
	"image"
	"image/png"
	"io"
	"os"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"sync"

	"golang.org/x/image/draw"
)

// ThemeName is the icon theme the user picked in the settings of GTK or
// KDE, whichever desktop runs, or that desktop's own theme.
func (d Desktop) ThemeName() string {
	gtk := func() string {
		for _, c := range d.Config {
			for _, v := range []string{"gtk-3.0", "gtk-4.0"} {
				if kf, err := ReadKeyFile(filepath.Join(c, v, "settings.ini")); err == nil {
					if name := strings.Trim(kf["Settings"]["gtk-icon-theme-name"], `"'`); name != "" {
						return name
					}
				}
			}
		}
		return ""
	}
	kde := func() string {
		for _, c := range d.Config {
			if kf, err := ReadKeyFile(filepath.Join(c, "kdeglobals")); err == nil && kf["Icons"]["Theme"] != "" {
				return kf["Icons"]["Theme"]
			}
		}
		return ""
	}
	order, fallback := []func() string{gtk, kde}, "Adwaita"
	if slices.Contains(d.Desktops, "kde") {
		order, fallback = []func() string{kde, gtk}, "breeze"
	}
	for _, read := range order {
		if name := read(); name != "" {
			return name
		}
	}
	return fallback
}

// Icons finds icons in an icon theme, the themes it inherits from and
// hicolor, the way the icon theme specification lays it out. It keeps what
// it has looked up, so one value serves many lookups.
type Icons struct {
	bases   []string
	pixmaps []string
	themes  []theme

	mu    sync.Mutex
	files map[string]map[string]bool
	found map[string]string
}

type theme struct {
	name string
	dirs []themeDir
}

type themeDir struct {
	path, kind                string
	size, min, max, threshold int
}

// Icons reads the theme called name and the ones it inherits from.
func (d Desktop) Icons(name string) *Icons {
	ic := &Icons{bases: []string{filepath.Join(d.Home, ".icons")}, files: map[string]map[string]bool{}, found: map[string]string{}}
	for _, data := range d.Data {
		ic.bases = append(ic.bases, filepath.Join(data, "icons"))
		ic.pixmaps = append(ic.pixmaps, filepath.Join(data, "pixmaps"))
	}
	seen := map[string]bool{}
	ic.add(name, seen)
	ic.add("hicolor", seen)
	return ic
}

func (ic *Icons) add(name string, seen map[string]bool) {
	if name == "" || seen[name] || strings.ContainsAny(name, `/\`) {
		return
	}
	seen[name] = true
	var kf KeyFile
	for _, b := range ic.bases {
		if f, err := ReadKeyFile(filepath.Join(b, name, "index.theme")); err == nil {
			kf = f
			break
		}
	}
	if kf == nil {
		return
	}
	t := theme{name: name}
	g := kf["Icon Theme"]
	for _, dir := range commaList(g["Directories"]) {
		dg := kf[dir]
		num := func(key string, fallback int) int {
			if n, err := strconv.Atoi(dg[key]); err == nil {
				return n
			}
			return fallback
		}
		size := num("Size", 0)
		if dg == nil || size <= 0 || num("Scale", 1) != 1 {
			continue
		}
		t.dirs = append(t.dirs, themeDir{
			path: dir, kind: dg["Type"], size: size,
			min: num("MinSize", size), max: num("MaxSize", size), threshold: num("Threshold", 2),
		})
	}
	ic.themes = append(ic.themes, t)
	for _, parent := range commaList(g["Inherits"]) {
		ic.add(parent, seen)
	}
}

func commaList(s string) []string {
	var out []string
	for _, item := range strings.Split(s, ",") {
		if item = strings.TrimSpace(item); item != "" {
			out = append(out, item)
		}
	}
	return out
}

func (t themeDir) matches(px int) bool {
	switch t.kind {
	case "Fixed":
		return t.size == px
	case "Scalable":
		return t.min <= px && px <= t.max
	default:
		return t.size-t.threshold <= px && px <= t.size+t.threshold
	}
}

func (t themeDir) distance(px int) int {
	switch t.kind {
	case "Fixed":
		return abs(t.size - px)
	case "Scalable":
		if px < t.min {
			return t.min - px
		}
		return max(0, px-t.max)
	default:
		if px < t.size-t.threshold || px > t.size+t.threshold {
			return abs(t.size - px)
		}
		return 0
	}
}

func abs(n int) int {
	if n < 0 {
		return -n
	}
	return n
}

var iconExts = []string{".png", ".svg"}

// Find returns the file of the first of names the theme has, in the size
// nearest to px. A theme goes through all the names before the next theme,
// so a theme's generic icon wins over a specific one only hicolor has.
func (ic *Icons) Find(px int, names ...string) (string, bool) {
	key := strconv.Itoa(px) + "\x00" + strings.Join(names, "\x00")
	ic.mu.Lock()
	defer ic.mu.Unlock()
	if file, ok := ic.found[key]; ok {
		return file, file != ""
	}
	file := ic.find(px, names)
	ic.found[key] = file
	return file, file != ""
}

func (ic *Icons) find(px int, names []string) string {
	for _, t := range ic.themes {
		for _, name := range names {
			if file := ic.lookup(t, name, px); file != "" {
				return file
			}
		}
	}
	for _, name := range names {
		for _, dir := range append(slices.Clone(ic.bases), ic.pixmaps...) {
			if file := ic.inDir(dir, name); file != "" {
				return file
			}
		}
	}
	return ""
}

func (ic *Icons) lookup(t theme, name string, px int) string {
	for _, dir := range t.dirs {
		if !dir.matches(px) {
			continue
		}
		for _, b := range ic.bases {
			if file := ic.inDir(filepath.Join(b, t.name, dir.path), name); file != "" {
				return file
			}
		}
	}
	best, nearest := "", 0
	for _, dir := range t.dirs {
		for _, b := range ic.bases {
			if file := ic.inDir(filepath.Join(b, t.name, dir.path), name); file != "" {
				if d := dir.distance(px); best == "" || d < nearest {
					best, nearest = file, d
				}
			}
		}
	}
	return best
}

// inDir finds name in dir, reading each folder once.
func (ic *Icons) inDir(dir, name string) string {
	have, ok := ic.files[dir]
	if !ok {
		have = map[string]bool{}
		entries, _ := os.ReadDir(dir)
		for _, e := range entries {
			if !e.IsDir() {
				have[e.Name()] = true
			}
		}
		ic.files[dir] = have
	}
	for _, ext := range iconExts {
		if have[name+ext] {
			return filepath.Join(dir, name+ext)
		}
	}
	return ""
}

// appIconSizes are the folders looked in for the icon of a program, the
// size of a menu icon first, then larger ones that scale down well.
var appIconSizes = []string{"32x32", "48x48", "64x64", "scalable", "128x128", "256x256", "512x512", "24x24", "22x22", "16x16"}

// AppIcon finds the file of the icon a desktop entry names in hicolor, the
// other themes or the old pixmaps folder. It needs no index.theme, so it
// also finds icons of themes a program brought along.
func (d Desktop) AppIcon(name string) (string, bool) {
	exists := func(p string) bool {
		info, err := os.Stat(p)
		return err == nil && info.Mode().IsRegular()
	}
	if filepath.IsAbs(name) {
		return name, exists(name)
	}
	if name == "" || strings.ContainsAny(name, `/\`) {
		return "", false
	}
	bases := []string{filepath.Join(d.Home, ".icons")}
	for _, data := range d.Data {
		bases = append(bases, filepath.Join(data, "icons"))
	}
	themes := []string{"hicolor"}
	for _, b := range bases {
		dirs, _ := os.ReadDir(b)
		for _, dir := range dirs {
			if dir.IsDir() && !slices.Contains(themes, dir.Name()) {
				themes = append(themes, dir.Name())
			}
		}
	}
	for _, t := range themes {
		for _, b := range bases {
			for _, size := range appIconSizes {
				for _, ext := range iconExts {
					if p := filepath.Join(b, t, size, "apps", name+ext); exists(p) {
						return p, true
					}
				}
			}
		}
	}
	for _, data := range d.Data {
		for _, ext := range iconExts {
			if p := filepath.Join(data, "pixmaps", name+ext); exists(p) {
				return p, true
			}
		}
	}
	return "", false
}

// LoadIcon reads an icon file for a square of px pixels: an SVG as it is,
// and any other picture as a PNG scaled into the square.
func LoadIcon(file string, px int) ([]byte, string, error) {
	switch strings.ToLower(filepath.Ext(file)) {
	case ".svg":
		b, err := os.ReadFile(file)
		return b, "image/svg+xml", err
	case ".svgz":
		f, err := os.Open(file)
		if err != nil {
			return nil, "", err
		}
		defer f.Close()
		z, err := gzip.NewReader(f)
		if err != nil {
			return nil, "", err
		}
		b, err := io.ReadAll(io.LimitReader(z, 16<<20))
		return b, "image/svg+xml", err
	}
	b, err := os.ReadFile(file)
	if err != nil {
		return nil, "", err
	}
	img, err := png.Decode(bytes.NewReader(b))
	if err != nil {
		return nil, "", err
	}
	if img.Bounds().Dx() == px && img.Bounds().Dy() == px {
		return b, "image/png", nil
	}
	b, err = ScaleIcon(img, px)
	return b, "image/png", err
}

// ScaleIcon draws img into a square of px pixels as a PNG, keeping its
// shape.
func ScaleIcon(img image.Image, px int) ([]byte, error) {
	b := img.Bounds()
	if b.Empty() {
		return nil, errors.New("empty image")
	}
	w, h := px, px
	if b.Dx() > b.Dy() {
		h = max(1, px*b.Dy()/b.Dx())
	} else {
		w = max(1, px*b.Dx()/b.Dy())
	}
	small := image.NewNRGBA(image.Rect(0, 0, px, px))
	at := image.Rect((px-w)/2, (px-h)/2, (px-w)/2+w, (px-h)/2+h)
	draw.CatmullRom.Scale(small, at, img, b, draw.Over, nil)
	var out bytes.Buffer
	if err := png.Encode(&out, small); err != nil {
		return nil, err
	}
	return out.Bytes(), nil
}

// UserDirs are the folders of user-dirs.dirs, such as DOWNLOAD, by their
// name. A folder set to the home folder itself is left out, as the
// specification asks.
func (d Desktop) UserDirs() map[string]string {
	out := map[string]string{}
	eachLine(filepath.Join(d.Config[0], "user-dirs.dirs"), func(line string) {
		k, v, ok := strings.Cut(line, "=")
		name, isDir := strings.CutSuffix(strings.TrimPrefix(k, "XDG_"), "_DIR")
		if !ok || !isDir || !strings.HasPrefix(k, "XDG_") {
			return
		}
		v = strings.Trim(v, `"`)
		if rest, ok := strings.CutPrefix(v, "$HOME"); ok {
			v = d.Home + rest
		}
		if filepath.IsAbs(v) && filepath.Clean(v) != filepath.Clean(d.Home) {
			out[name] = filepath.Clean(v)
		}
	})
	return out
}
