package tools

import (
	"bytes"
	"context"
	"io/fs"
	"net/http"
	"os"
	"path"
	"path/filepath"
	"regexp"
	"strings"

	v1 "github.com/junkerderprovinz/cannoncommander/backend/internal/api/v1"
	"github.com/junkerderprovinz/cannoncommander/backend/internal/ccfs"
	"github.com/junkerderprovinz/cannoncommander/backend/internal/database/users"
)

// Groups of the clean up. The first four go to the trash, the last two
// get a better name.
const (
	GroupEmptyDir  = "emptyDir"
	GroupEmptyFile = "emptyFile"
	GroupTemp      = "temp"
	GroupLink      = "link"
	GroupExt       = "ext"
	GroupName      = "name"
)

// maxFinds bounds each group, so a share full of leftovers still answers.
const maxFinds = 5000

// Find is one entry the clean up suggests. Note is kind specific: the
// size of a leftover, the target of a broken link, the real kind behind a
// wrong ending, or why a name is a problem (edgeSpace, trailingDot,
// windowsChars, tooLong). To is the better name.
type Find struct {
	Path string `json:"path"`
	Note string `json:"note,omitempty"`
	Size int64  `json:"size,omitempty"`
	To   string `json:"to,omitempty"`
}

// Scan is the answer of POST /tools/cleanup/scan.
type Scan struct {
	Path    string            `json:"path"`
	Scanned int               `json:"scanned"`
	Groups  map[string][]Find `json:"groups"`
	Capped  bool              `json:"capped,omitempty"`
}

var (
	tempName  = regexp.MustCompile(`(?i)(\.part|\.!qb|\.tmp|\.crdownload|\.partial)$`)
	junkNames = map[string]bool{".ds_store": true, "thumbs.db": true, "desktop.ini": true}
	badChars  = regexp.MustCompile(`[\\:*?"<>|]`)
)

// signatures are the kinds whose content gives itself away in its first
// bytes, with the endings that fit them.
var signatures = []struct {
	kind  string
	magic []byte
	exts  []string
}{
	{"png", []byte("\x89PNG\r\n\x1a\n"), []string{"png"}},
	{"jpg", []byte("\xff\xd8\xff"), []string{"jpg", "jpeg", "jpe", "jfif"}},
	{"pdf", []byte("%PDF-"), []string{"pdf"}},
	{"gif", []byte("GIF8"), []string{"gif"}},
	{"zip", []byte("PK\x03\x04"), []string{"zip", "docx", "xlsx", "pptx", "odt", "ods", "odp", "epub", "jar", "apk", "cbz", "3mf", "kmz", "xpi", "ipa", "aar"}},
}

func badName(name string) string {
	switch {
	case strings.TrimSpace(name) != name:
		return "edgeSpace"
	case strings.HasSuffix(name, "."):
		return "trailingDot"
	case badChars.MatchString(name):
		return "windowsChars"
	case len(name) > 255:
		return "tooLong"
	}
	return ""
}

func cleanName(name string) string {
	return badChars.ReplaceAllString(strings.TrimRight(strings.TrimSpace(name), "."), "_")
}

func sniff(real string) string {
	f, err := os.Open(real)
	if err != nil {
		return ""
	}
	defer f.Close()
	head := make([]byte, 16)
	n, _ := f.Read(head)
	for _, s := range signatures {
		if bytes.HasPrefix(head[:n], s.magic) {
			return s.kind
		}
	}
	return ""
}

type cleaner struct {
	ctx  context.Context
	fs   *ccfs.Resolver
	u    *users.User
	scan *Scan
}

func (c *cleaner) add(group string, f Find) {
	if len(c.scan.Groups[group]) >= maxFinds {
		c.scan.Capped = true
		return
	}
	c.scan.Groups[group] = append(c.scan.Groups[group], f)
}

// walk looks at the folder dir and reports whether it holds nothing but
// empty folders.
func (c *cleaner) walk(dir ccfs.Location) (bool, error) {
	list, err := os.ReadDir(dir.Real)
	if err != nil {
		return false, err
	}
	empty := true
	for _, d := range list {
		if err := c.ctx.Err(); err != nil {
			return false, err
		}
		name := d.Name()
		if bookkeeping(name) || !c.fs.Visible(c.u, dir, name) {
			empty = false
			continue
		}
		c.scan.Scanned++
		p := path.Join(dir.Path, name)
		real := filepath.Join(dir.Real, name)
		info, err := d.Info()
		if err != nil {
			empty = false
			continue
		}
		if why := badName(name); why != "" {
			c.add(GroupName, Find{Path: p, Note: why, To: cleanName(name)})
		}
		switch {
		case info.Mode()&fs.ModeSymlink != 0:
			empty = false
			if _, err := os.Stat(real); err != nil {
				target, _ := os.Readlink(real)
				c.add(GroupLink, Find{Path: p, Note: target})
			}
		case info.IsDir():
			sub := dir
			sub.Path, sub.Index, sub.Real = p, path.Join(dir.Index, name), real
			dirs, names := len(c.scan.Groups[GroupEmptyDir]), len(c.scan.Groups[GroupName])
			inner, err := c.walk(sub)
			if err != nil && c.ctx.Err() != nil {
				return false, err
			}
			if err == nil && inner {
				// The folder goes as a whole, so what lies inside it is no
				// find of its own.
				c.scan.Groups[GroupEmptyDir] = c.scan.Groups[GroupEmptyDir][:dirs]
				c.scan.Groups[GroupName] = c.scan.Groups[GroupName][:names]
				note := "empty"
				if kids, _ := os.ReadDir(real); len(kids) > 0 {
					note = "onlyEmptyDirs"
				}
				c.add(GroupEmptyDir, Find{Path: p, Note: note})
			} else {
				empty = false
			}
		default:
			empty = false
			lower := strings.ToLower(name)
			switch {
			case tempName.MatchString(name) || strings.HasPrefix(name, "~$") || junkNames[lower]:
				c.add(GroupTemp, Find{Path: p, Size: info.Size()})
			case info.Size() == 0:
				c.add(GroupEmptyFile, Find{Path: p})
			}
			if info.Size() > 0 && info.Mode().IsRegular() {
				c.checkExt(p, real, name)
			}
		}
	}
	return empty, nil
}

func (c *cleaner) checkExt(p, real, name string) {
	kind := sniff(real)
	if kind == "" {
		return
	}
	ext := strings.ToLower(strings.TrimPrefix(path.Ext(name), "."))
	for _, s := range signatures {
		if s.kind != kind {
			continue
		}
		for _, e := range s.exts {
			if e == ext {
				return
			}
		}
		// A zip is the container of many formats, and only a file named
		// as an image or document that is a zip is worth a word.
		if kind == "zip" && !imageExts[ext] && ext != "pdf" {
			return
		}
		base := strings.TrimSuffix(name, path.Ext(name))
		if base == "" {
			base = name
		}
		c.add(GroupExt, Find{Path: p, Note: kind, To: base + "." + s.exts[0]})
	}
}

type scanRequest struct {
	Path string `json:"path"`
}

func (h handlers) cleanupScan(w http.ResponseWriter, r *http.Request) error {
	u := v1.UserFrom(r)
	var req scanRequest
	if err := v1.DecodeJSON(r, &req); err != nil {
		return err
	}
	dir, err := folder(h.fs, u, req.Path, ccfs.View)
	if err != nil {
		return err
	}
	scan := &Scan{Path: dir.Path, Groups: map[string][]Find{}}
	for _, g := range []string{GroupEmptyDir, GroupEmptyFile, GroupTemp, GroupLink, GroupExt, GroupName} {
		scan.Groups[g] = []Find{}
	}
	c := &cleaner{ctx: r.Context(), fs: h.fs, u: u, scan: scan}
	if _, err := c.walk(dir); err != nil {
		return err
	}
	return v1.WriteJSON(w, r, http.StatusOK, scan)
}
