package fsapi

import (
	"errors"
	"io"
	"mime"
	"net/http"
	"os"
	"path"
	"strconv"
	"strings"

	v1 "github.com/junkerderprovinz/cannoncommander/backend/internal/api/v1"
	"github.com/junkerderprovinz/cannoncommander/backend/internal/archives"
	"github.com/junkerderprovinz/cannoncommander/backend/internal/ccfs"
)

// errArchived tells the content handlers that a path lies inside an
// archive and has to be read through it.
var errArchived = errors.New("inside an archive")

// passwordHeader carries the password of an encrypted archive. A header
// stays out of the access logs that a query string would end up in.
const passwordHeader = "X-Archive-Password"

func archiveOptions(r *http.Request) archives.Options {
	return archives.Options{Password: r.Header.Get(passwordHeader)}
}

// archiveFile checks the archive a location points into.
func archiveFile(loc ccfs.Location) error {
	if err := ccfs.Contained(loc); err != nil {
		return err
	}
	info, err := os.Stat(loc.Real)
	if err != nil {
		return err
	}
	if !info.Mode().IsRegular() || archives.FormatOf(loc.Real) == "" {
		return v1.Errorf(http.StatusBadRequest, "path is not an archive")
	}
	return nil
}

func archiveItem(e archives.Entry) Item {
	name := path.Base(e.Name)
	it := Item{Name: name, Dir: e.Dir, Size: e.Size, MTime: formatTime(e.ModTime), Mode: modeString(e.Mode),
		Hidden: strings.HasPrefix(name, "."), Link: e.Link}
	if !e.Dir {
		it.Format = FormatOf(name)
	}
	return it
}

func (h handlers) archiveIndex(r *http.Request, loc ccfs.Location) (*archives.Index, error) {
	ix, err := archives.Load(loc.Real, archiveOptions(r))
	if err != nil {
		return nil, archives.Problem(err)
	}
	return ix, nil
}

func (h handlers) archiveList(w http.ResponseWriter, r *http.Request, loc ccfs.Location) error {
	ix, err := h.archiveIndex(r, loc)
	if err != nil {
		return err
	}
	kids, ok := ix.Children(loc.Inner)
	if !ok {
		return v1.Errorf(http.StatusNotFound, "path not found")
	}
	out := Listing{Path: loc.Path, Items: make([]Item, 0, len(kids))}
	for _, e := range kids {
		out.Items = append(out.Items, archiveItem(e))
	}
	return v1.WriteJSON(w, r, http.StatusOK, out)
}

func (h handlers) archiveTree(w http.ResponseWriter, r *http.Request, loc ccfs.Location) error {
	ix, err := h.archiveIndex(r, loc)
	if err != nil {
		return err
	}
	kids, ok := ix.Children(loc.Inner)
	if !ok {
		return v1.Errorf(http.StatusNotFound, "path not found")
	}
	out := Tree{Path: loc.Path, Items: []TreeItem{}}
	outer, _, _ := strings.Cut(loc.Path, ccfs.ArchiveSep)
	for _, e := range kids {
		if !e.Dir {
			continue
		}
		sub, _ := ix.Children(e.Name)
		hasDirs := false
		for _, s := range sub {
			hasDirs = hasDirs || s.Dir
		}
		out.Items = append(out.Items, TreeItem{Name: path.Base(e.Name), Path: outer + ccfs.ArchiveSep + "/" + e.Name, HasDirs: hasDirs})
	}
	return v1.WriteJSON(w, r, http.StatusOK, out)
}

func (h handlers) archiveStat(w http.ResponseWriter, r *http.Request, loc ccfs.Location) error {
	if err := archiveFile(loc); err != nil {
		return err
	}
	ix, err := h.archiveIndex(r, loc)
	if err != nil {
		return err
	}
	e, ok := ix.Stat(loc.Inner)
	if !ok {
		return v1.Errorf(http.StatusNotFound, "path not found")
	}
	out := Stat{Path: loc.Path, Item: archiveItem(e)}
	if loc.Inner == "/" {
		outer, _, _ := strings.Cut(loc.Path, ccfs.ArchiveSep)
		out.Name = path.Base(outer)
	}
	return v1.WriteJSON(w, r, http.StatusOK, out)
}

// readArchived hands fn the content of a file inside an archive.
func readArchived(r *http.Request, loc ccfs.Location, fn func(e archives.Entry, rd io.Reader) error) error {
	if err := archiveFile(loc); err != nil {
		return err
	}
	err := archives.ReadEntry(loc.Real, archiveOptions(r), loc.Inner, fn)
	if errors.Is(err, os.ErrNotExist) {
		return v1.Errorf(http.StatusNotFound, "path not found")
	}
	return archives.Problem(err)
}

// archiveRaw streams a file out of an archive. Most formats cannot seek
// inside an entry, so there are no range requests.
func (h handlers) archiveRaw(w http.ResponseWriter, r *http.Request, loc ccfs.Location) error {
	return readArchived(r, loc, func(e archives.Entry, rd io.Reader) error {
		name := path.Base(e.Name)
		ctype := mime.TypeByExtension(path.Ext(name))
		if ctype == "" {
			ctype = "application/octet-stream"
		}
		hdr := w.Header()
		hdr.Set("Content-Type", ctype)
		hdr.Set("X-Content-Type-Options", "nosniff")
		hdr.Set("Cache-Control", "private, no-cache")
		hdr.Set("Accept-Ranges", "none")
		hdr.Set("Content-Length", strconv.FormatInt(e.Size, 10))
		if active(ctype) {
			hdr.Set("Content-Security-Policy", "sandbox; default-src 'none'; img-src data:; style-src 'unsafe-inline'")
		}
		disposition := "inline"
		if flag(r.URL.Query().Get("download")) {
			disposition = "attachment"
		}
		hdr.Set("Content-Disposition", mime.FormatMediaType(disposition, map[string]string{"filename": name}))
		w.WriteHeader(http.StatusOK)
		if r.Method == http.MethodHead {
			return nil
		}
		_, err := io.Copy(w, rd)
		return err
	})
}

func (h handlers) archiveText(w http.ResponseWriter, r *http.Request, loc ccfs.Location, limit int64) error {
	return readArchived(r, loc, func(e archives.Entry, rd io.Reader) error {
		buf, err := io.ReadAll(io.LimitReader(rd, limit))
		if err != nil {
			return err
		}
		truncated := int64(len(buf)) < e.Size
		enc, text, err := decodeText(buf, truncated)
		if err != nil {
			return err
		}
		return v1.WriteJSON(w, r, http.StatusOK, Text{Path: loc.Path, Size: e.Size, MTime: formatTime(e.ModTime),
			Encoding: enc, EOL: lineEnding(text), Text: text, Truncated: truncated})
	})
}

func (h handlers) archiveHex(w http.ResponseWriter, r *http.Request, loc ccfs.Location, offset, length int64) error {
	return readArchived(r, loc, func(e archives.Entry, rd io.Reader) error {
		if _, err := io.CopyN(io.Discard, rd, offset); err != nil && !errors.Is(err, io.EOF) {
			return err
		}
		data, err := io.ReadAll(io.LimitReader(rd, length))
		if err != nil {
			return err
		}
		return v1.WriteJSON(w, r, http.StatusOK, Hex{Path: loc.Path, Size: e.Size, Offset: offset, Data: data})
	})
}
