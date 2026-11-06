package fsapi

import (
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"errors"
	"io"
	"io/fs"
	"net/http"
	"os"
	"path"
	"path/filepath"
	"strings"

	v1 "github.com/junkerderprovinz/cannoncommander/backend/internal/api/v1"
	"github.com/junkerderprovinz/cannoncommander/backend/internal/ccfs"
	"github.com/junkerderprovinz/cannoncommander/backend/internal/fileops"
	"github.com/junkerderprovinz/cannoncommander/backend/internal/fschange"
	"golang.org/x/text/encoding/charmap"
	"golang.org/x/text/encoding/unicode"
)

// editMax is the largest file the editor opens. Past that a browser editor
// gets slow, and the file is better served by fs/text and fs/raw.
const editMax = 16 << 20

// EditFile is the answer of GET /fs/file.
type EditFile struct {
	Path     string `json:"path"`
	Size     int64  `json:"size"`
	MTime    string `json:"mtime"`
	Encoding string `json:"encoding"`
	EOL      string `json:"eol,omitempty"`
	Text     string `json:"text"`
	ETag     string `json:"etag"`
	// Writable says whether a save would be allowed, so the editor can
	// open read only right away.
	Writable bool `json:"writable"`
}

// SavedFile is the answer of PUT /fs/file.
type SavedFile struct {
	Path    string `json:"path"`
	Size    int64  `json:"size"`
	MTime   string `json:"mtime"`
	ETag    string `json:"etag"`
	Created bool   `json:"created,omitempty"`
}

type saveRequest struct {
	Text string `json:"text"`
	// Encoding defaults to the one the file has, and to UTF-8 for a new
	// file.
	Encoding string `json:"encoding"`
	// EOL rewrites every line break, because the editor works with \n.
	EOL string `json:"eol"`
}

// contentTag follows the bytes, not the time. A tool that keeps the mtime
// while it writes, or two saves within the clock's resolution, would get
// past a tag made from size and mtime.
func contentTag(b []byte) string {
	sum := sha256.Sum256(b)
	return `"` + base64.RawURLEncoding.EncodeToString(sum[:16]) + `"`
}

func (h handlers) fileGet(w http.ResponseWriter, r *http.Request) error {
	u := v1.UserFrom(r)
	loc, info, err := h.file(u, r.URL.Query().Get("path"), ccfs.Download)
	if err != nil {
		return err
	}
	if info.Size() > editMax {
		return v1.Coded(http.StatusRequestEntityTooLarge, "tooBigToEdit", "this file is too big for the editor")
	}
	buf, err := os.ReadFile(loc.Real)
	if err != nil {
		return err
	}
	tag := contentTag(buf)
	w.Header().Set("ETag", tag)
	w.Header().Set("Cache-Control", "private, no-cache")
	if etagMatches(r.Header.Get("If-None-Match"), tag) {
		w.WriteHeader(http.StatusNotModified)
		return nil
	}
	enc, text, err := decodeText(buf, false)
	if err != nil {
		return err
	}
	_, werr := h.fs.Check(u, loc.Path, ccfs.Modify)
	out := EditFile{Path: loc.Path, Size: info.Size(), MTime: formatTime(info.ModTime()), Encoding: enc,
		EOL: lineEnding(text), Text: text, ETag: tag, Writable: werr == nil}
	body, err := json.Marshal(out)
	if err != nil {
		return err
	}
	w.Header().Set("Content-Type", "application/json")
	_, err = w.Write(append(body, '\n'))
	return err
}

func (h handlers) filePut(w http.ResponseWriter, r *http.Request) error {
	u := v1.UserFrom(r)
	p := r.URL.Query().Get("path")
	if p == "" {
		return v1.Errorf(http.StatusBadRequest, "path is required")
	}
	var req saveRequest
	dec := json.NewDecoder(io.LimitReader(r.Body, 6*editMax))
	dec.DisallowUnknownFields()
	if err := dec.Decode(&req); err != nil {
		return v1.Errorf(http.StatusBadRequest, "invalid JSON body: "+err.Error())
	}

	loc, err := h.fs.Resolve(p)
	if err != nil {
		return err
	}
	if loc.Virtual() || loc.InArchive() || loc.Index == "/" || !ccfs.ValidName(path.Base(loc.Path)) {
		return v1.Coded(http.StatusBadRequest, "badPath", "invalid path")
	}
	old, err := os.ReadFile(loc.Real)
	exists := err == nil
	switch {
	case errors.Is(err, fs.ErrNotExist):
	case err != nil:
		if info, serr := os.Stat(loc.Real); serr == nil && info.IsDir() {
			return v1.Coded(http.StatusBadRequest, "isFolder", "path is a folder")
		}
		return err
	}
	perm := ccfs.Create
	if exists {
		perm = ccfs.Modify
	}
	if loc, err = h.fs.Check(u, p, perm); err != nil {
		return err
	}
	if err := h.fs.Follows(u, loc, perm); err != nil {
		return err
	}
	var info os.FileInfo
	if exists {
		if info, err = os.Stat(loc.Real); err != nil {
			return err
		}
		if !info.Mode().IsRegular() {
			return v1.Coded(http.StatusBadRequest, "notAFile", "path is not a regular file")
		}
	} else if dir, err := os.Stat(filepath.Dir(loc.Real)); err != nil || !dir.IsDir() {
		return v1.Coded(http.StatusConflict, "noFolderForFile", "the folder for this file does not exist")
	}

	// A save is checked against the file as it is on disk, so changes made in
	// the meantime by someone else are never overwritten unseen.
	if m := r.Header.Get("If-Match"); m != "" && (!exists || m != "*" && !etagMatches(m, contentTag(old))) {
		return v1.Coded(http.StatusPreconditionFailed, "changedWhileOpen", "the file changed since it was opened")
	}
	if r.Header.Get("If-None-Match") == "*" && exists {
		return v1.Coded(http.StatusPreconditionFailed, "nameTaken", "a file of this name exists already")
	}

	if req.Encoding == "" {
		req.Encoding = "utf-8"
		if exists {
			if enc, _, err := decodeText(old, false); err == nil {
				req.Encoding = enc
			}
		}
	}
	data, err := encodeText(withEOL(req.Text, req.EOL), req.Encoding)
	if err != nil {
		return err
	}
	if len(data) > editMax {
		return v1.Coded(http.StatusRequestEntityTooLarge, "tooBigToEdit", "this text is too big for the editor")
	}
	if err := writeFile(loc.Real, data, info); err != nil {
		return err
	}
	saved, err := os.Stat(loc.Real)
	if err != nil {
		return err
	}
	status := http.StatusOK
	if !exists {
		status = http.StatusCreated
		fschange.Created(loc.Path)
	}
	tag := contentTag(data)
	w.Header().Set("ETag", tag)
	return v1.WriteJSON(w, r, status, SavedFile{Path: loc.Path, Size: saved.Size(), MTime: formatTime(saved.ModTime()), ETag: tag, Created: !exists})
}

// writeFile saves data through a part file and a rename, so a crash leaves
// the old file whole. A file with hard links is written in place instead,
// because the rename would cut it off from its other names.
func writeFile(real string, data []byte, old os.FileInfo) error {
	if old != nil {
		if st, err := statFile(real); err == nil && st.nlink > 1 {
			return os.WriteFile(real, data, old.Mode().Perm())
		}
	}
	part := filepath.Join(filepath.Dir(real), "."+filepath.Base(real)+fileops.PartSuffix)
	f, err := os.OpenFile(part, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o666)
	if err != nil {
		if errors.Is(err, fs.ErrExist) {
			return v1.Coded(http.StatusConflict, "beingWritten", "this file is being written right now")
		}
		return err
	}
	_, err = f.Write(data)
	if err == nil {
		err = f.Sync()
	}
	if cerr := f.Close(); err == nil {
		err = cerr
	}
	if err == nil && old != nil {
		err = os.Chmod(part, old.Mode().Perm())
		if st, serr := statFile(real); serr == nil && st.ids {
			// Without the right to give files away the save still goes
			// through, with the container's owner.
			_ = os.Lchown(part, int(st.uid), int(st.gid))
		}
	}
	if err == nil {
		err = os.Rename(part, real)
	}
	if err != nil {
		_ = os.Remove(part)
	}
	return err
}

func withEOL(text, eol string) string {
	var sep string
	switch eol {
	case "lf":
		sep = "\n"
	case "crlf":
		sep = "\r\n"
	case "cr":
		sep = "\r"
	default:
		return text
	}
	text = strings.ReplaceAll(text, "\r\n", "\n")
	text = strings.ReplaceAll(text, "\r", "\n")
	if sep == "\n" {
		return text
	}
	return strings.ReplaceAll(text, "\n", sep)
}

// encodeText writes text back in the encoding the editor read it in.
func encodeText(text, encoding string) ([]byte, error) {
	switch encoding {
	case "utf-8", "ascii":
		return []byte(text), nil
	case "utf-8-bom":
		return append([]byte{0xEF, 0xBB, 0xBF}, text...), nil
	case "utf-16le", "utf-16be":
		order := unicode.LittleEndian
		if encoding == "utf-16be" {
			order = unicode.BigEndian
		}
		return unicode.UTF16(order, unicode.UseBOM).NewEncoder().Bytes([]byte(text))
	case "windows-1252":
		out, err := charmap.Windows1252.NewEncoder().Bytes([]byte(text))
		if err != nil {
			return nil, v1.Coded(http.StatusUnprocessableEntity, "notWindows1252", "the text holds characters Windows-1252 cannot store, save it as UTF-8 instead")
		}
		return out, nil
	}
	return nil, v1.Errorf(http.StatusBadRequest, "encoding must be utf-8, utf-8-bom, utf-16le, utf-16be, windows-1252 or ascii")
}
