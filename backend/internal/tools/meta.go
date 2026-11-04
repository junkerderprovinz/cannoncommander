package tools

import (
	"image"
	_ "image/gif"
	_ "image/jpeg"
	_ "image/png"
	"os"
	"path"
	"regexp"
	"strconv"
	"strings"

	"github.com/dhowden/tag"
	"github.com/evanoberholster/imagemeta"
	_ "golang.org/x/image/webp"
)

// Meta reads the values the metadata tokens of a rename mask stand for:
// A taken, K camera, I artist, T title, R resolution, S season, F episode.
// A missing value is "".
type Meta func(real, name string) map[byte]string

var episode = regexp.MustCompile(`(?i)s(\d{1,2})e(\d{1,3})|\b(\d{1,2})x(\d{2,3})\b`)

var videoExts = map[string]bool{
	"mkv": true, "mp4": true, "m4v": true, "avi": true, "mov": true, "webm": true, "ts": true,
	"wmv": true, "mpg": true, "mpeg": true, "flv": true, "m2ts": true,
}

var audioExts = map[string]bool{
	"mp3": true, "flac": true, "m4a": true, "ogg": true, "opus": true, "aac": true, "wav": true, "aiff": true,
}

var imageExts = map[string]bool{
	"jpg": true, "jpeg": true, "png": true, "gif": true, "webp": true, "heic": true, "heif": true,
	"tif": true, "tiff": true, "cr2": true, "cr3": true, "nef": true, "arw": true, "dng": true, "raf": true,
}

// readMeta is the Meta of the server. It only opens files whose kind
// carries such values.
func readMeta(real, name string) map[byte]string {
	out := map[byte]string{}
	ext := strings.ToLower(strings.TrimPrefix(path.Ext(name), "."))
	switch {
	case videoExts[ext]:
		if m := episode.FindStringSubmatch(name); m != nil {
			out['S'] = trimZeros(m[1] + m[3])
			out['F'] = trimZeros(m[2] + m[4])
		}
	case audioExts[ext]:
		if f, err := os.Open(real); err == nil {
			if m, err := tag.ReadFrom(f); err == nil {
				out['I'], out['T'] = strings.TrimSpace(m.Artist()), strings.TrimSpace(m.Title())
			}
			f.Close()
		}
	case imageExts[ext]:
		if f, err := os.Open(real); err == nil {
			if e, err := imagemeta.Decode(f); err == nil {
				if t := e.OriginalDate(); !t.IsZero() {
					out['A'] = t.Format("2006-01-02")
				}
				out['K'] = strings.TrimSpace(e.IFD0.Model)
				if e.IFD0.ImageWidth > 0 && e.IFD0.ImageHeight > 0 {
					out['R'] = strconv.Itoa(int(e.IFD0.ImageWidth)) + "x" + strconv.Itoa(int(e.IFD0.ImageHeight))
				}
			}
			if out['R'] == "" {
				if _, err := f.Seek(0, 0); err == nil {
					if c, _, err := image.DecodeConfig(f); err == nil {
						out['R'] = strconv.Itoa(c.Width) + "x" + strconv.Itoa(c.Height)
					}
				}
			}
			f.Close()
		}
	}
	return out
}

func trimZeros(s string) string {
	n, err := strconv.Atoi(s)
	if err != nil {
		return s
	}
	return strconv.Itoa(n)
}
