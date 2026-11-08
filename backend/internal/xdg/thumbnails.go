package xdg

import (
	"bytes"
	"crypto/md5"
	"encoding/binary"
	"encoding/hex"
	"errors"
	"hash/crc32"
	"os"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
)

// FileURI is the file URI of an absolute path, escaped the way GLib's
// g_filename_to_uri does. It is what thumbnails are keyed by, so it has to
// match what other programs write byte for byte.
func FileURI(path string) string {
	const keep = "!$&'()*+,-./:=@_~"
	const hexDigits = "0123456789ABCDEF"
	var b strings.Builder
	b.WriteString("file://")
	for i := 0; i < len(path); i++ {
		c := path[i]
		if 'a' <= c && c <= 'z' || 'A' <= c && c <= 'Z' || '0' <= c && c <= '9' || strings.IndexByte(keep, c) >= 0 {
			b.WriteByte(c)
			continue
		}
		b.WriteByte('%')
		b.WriteByte(hexDigits[c>>4])
		b.WriteByte(hexDigits[c&15])
	}
	return b.String()
}

// ThumbnailSizes are the folders of thumbnails and the edge of the
// thumbnails each holds, smallest first.
var ThumbnailSizes = []struct {
	Dir  string
	Edge int
}{{"normal", 128}, {"large", 256}, {"x-large", 512}, {"xx-large", 1024}}

// ThumbnailSize is the index in ThumbnailSizes of the smallest thumbnails
// at least px wide, or of the largest ones.
func ThumbnailSize(px int) int {
	for i, s := range ThumbnailSizes {
		if px <= s.Edge {
			return i
		}
	}
	return len(ThumbnailSizes) - 1
}

// ThumbnailDir is the folder that holds the thumbnails.
func (d Desktop) ThumbnailDir() string { return filepath.Join(d.Cache, "thumbnails") }

// ThumbnailName is the file name of the thumbnail of uri.
func ThumbnailName(uri string) string {
	sum := md5.Sum([]byte(uri))
	return hex.EncodeToString(sum[:]) + ".png"
}

// ThumbnailFits reports whether a thumbnail was made of the file at uri as
// it was last changed at mtime, in seconds.
func ThumbnailFits(thumb []byte, uri string, mtime int64) bool {
	text, err := PNGText(thumb)
	if err != nil || text["Thumb::MTime"] != strconv.FormatInt(mtime, 10) {
		return false
	}
	u, ok := text["Thumb::URI"]
	return !ok || u == uri
}

var pngSignature = []byte("\x89PNG\r\n\x1a\n")

var errNotPNG = errors.New("not a PNG")

type chunk struct {
	kind string
	data []byte
}

func pngChunks(b []byte) ([]chunk, error) {
	if !bytes.HasPrefix(b, pngSignature) {
		return nil, errNotPNG
	}
	var out []chunk
	for rest := b[len(pngSignature):]; len(rest) > 0; {
		if len(rest) < 12 {
			return nil, errNotPNG
		}
		n := binary.BigEndian.Uint32(rest)
		if uint64(n)+12 > uint64(len(rest)) {
			return nil, errNotPNG
		}
		out = append(out, chunk{kind: string(rest[4:8]), data: rest[8 : 8+n]})
		rest = rest[12+n:]
	}
	if len(out) == 0 || out[0].kind != "IHDR" {
		return nil, errNotPNG
	}
	return out, nil
}

// PNGText returns the tEXt chunks of a PNG by their keyword.
func PNGText(b []byte) (map[string]string, error) {
	chunks, err := pngChunks(b)
	if err != nil {
		return nil, err
	}
	out := map[string]string{}
	for _, c := range chunks {
		if k, v, ok := bytes.Cut(c.data, []byte{0}); ok && c.kind == "tEXt" {
			out[string(k)] = string(v)
		}
	}
	return out, nil
}

// SetPNGText returns the PNG with a tEXt chunk for each keyword right after
// its header, in place of any it had for the same keyword.
func SetPNGText(b []byte, text map[string]string) ([]byte, error) {
	chunks, err := pngChunks(b)
	if err != nil {
		return nil, err
	}
	keys := make([]string, 0, len(text))
	for k := range text {
		keys = append(keys, k)
	}
	slices.Sort(keys)
	out := bytes.NewBuffer(slices.Clone(pngSignature))
	write := func(kind string, data []byte) {
		var n [4]byte
		binary.BigEndian.PutUint32(n[:], uint32(len(data)))
		out.Write(n[:])
		crc := crc32.NewIEEE()
		crc.Write([]byte(kind))
		crc.Write(data)
		out.WriteString(kind)
		out.Write(data)
		binary.BigEndian.PutUint32(n[:], crc.Sum32())
		out.Write(n[:])
	}
	for i, c := range chunks {
		if k, _, ok := bytes.Cut(c.data, []byte{0}); ok && c.kind == "tEXt" && slices.Contains(keys, string(k)) {
			continue
		}
		write(c.kind, c.data)
		if i == 0 {
			for _, k := range keys {
				write("tEXt", []byte(k+"\x00"+text[k]))
			}
		}
	}
	return out.Bytes(), nil
}

// Thumbnailer is a program that draws thumbnails, as a .thumbnailer file
// describes it.
type Thumbnailer struct {
	File, Exec string
	Mime       []string
}

// Thumbnailers lists the thumbnailers whose program is there. Where two
// data folders hold a file of the same name, the one that comes first wins.
func (d Desktop) Thumbnailers() []Thumbnailer {
	var out []Thumbnailer
	seen := map[string]bool{}
	for _, data := range d.Data {
		dir := filepath.Join(data, "thumbnailers")
		files, _ := os.ReadDir(dir)
		for _, f := range files {
			if seen[f.Name()] || !strings.HasSuffix(f.Name(), ".thumbnailer") {
				continue
			}
			seen[f.Name()] = true
			kf, err := ReadKeyFile(filepath.Join(dir, f.Name()))
			if err != nil {
				continue
			}
			g := kf["Thumbnailer Entry"]
			t := Thumbnailer{File: filepath.Join(dir, f.Name()), Exec: g.String("Exec"), Mime: g.List("MimeType")}
			if try := g.String("TryExec"); try != "" {
				if _, err := d.Look(try); err != nil {
					continue
				}
			}
			if t.Exec != "" && len(t.Mime) > 0 {
				out = append(out, t)
			}
		}
	}
	return out
}

// Command is how the thumbnailer draws file into out, a PNG with an edge
// of size pixels.
func (t Thumbnailer) Command(file, out string, size int) ([]string, error) {
	words, err := SplitExec(t.Exec)
	if err != nil {
		return nil, err
	}
	var args []string
	for _, w := range words {
		var b strings.Builder
		for i := 0; i < len(w); i++ {
			if w[i] != '%' || i == len(w)-1 {
				b.WriteByte(w[i])
				continue
			}
			i++
			switch w[i] {
			case 'i':
				b.WriteString(file)
			case 'u':
				b.WriteString(FileURI(file))
			case 'o':
				b.WriteString(out)
			case 's':
				b.WriteString(strconv.Itoa(size))
			case '%':
				b.WriteByte('%')
			}
		}
		args = append(args, b.String())
	}
	if len(args) == 0 {
		return nil, ErrNoExec
	}
	return args, nil
}
