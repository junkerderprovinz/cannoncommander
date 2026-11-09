package fsapi

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"mime"
	"net/http"
	"os"
	"path"
	"strconv"
	"strings"
	"unicode/utf8"

	v1 "github.com/junkerderprovinz/cannoncommander/backend/internal/api/v1"
	"github.com/junkerderprovinz/cannoncommander/backend/internal/ccfs"
	"github.com/junkerderprovinz/cannoncommander/backend/internal/database/users"
	"golang.org/x/text/encoding/charmap"
	"golang.org/x/text/encoding/unicode"
)

const (
	textDefault = 1 << 20
	textMax     = 16 << 20
	hexDefault  = 4096
	hexMax      = 64 << 10
)

// Text is the answer of GET /fs/text.
type Text struct {
	Path     string `json:"path"`
	Size     int64  `json:"size"`
	MTime    string `json:"mtime"`
	Encoding string `json:"encoding"`
	// EOL is lf, crlf, cr or mixed, and empty for a single line.
	EOL       string `json:"eol,omitempty"`
	Text      string `json:"text"`
	Truncated bool   `json:"truncated,omitempty"`
}

// Hex is the answer of GET /fs/hex. Data is base64 in JSON.
type Hex struct {
	Path   string `json:"path"`
	Size   int64  `json:"size"`
	Offset int64  `json:"offset"`
	Data   []byte `json:"data"`
}

// file checks that p is a regular file the user holds perm on.
func (h handlers) file(u *users.User, p string, perm ccfs.Perm) (ccfs.Location, os.FileInfo, error) {
	if p == "" {
		return ccfs.Location{}, nil, v1.Errorf(http.StatusBadRequest, "path is required")
	}
	loc, err := h.fs.Check(u, p, perm)
	if err != nil {
		return loc, nil, err
	}
	if loc.InArchive() {
		return loc, nil, errArchived
	}
	if loc.Virtual() {
		return loc, nil, v1.Coded(http.StatusBadRequest, "isFolder", "path is a folder")
	}
	if err := h.fs.Follows(u, loc, perm); err != nil {
		return loc, nil, err
	}
	info, err := os.Stat(loc.Real)
	if err != nil {
		return loc, nil, err
	}
	if info.IsDir() {
		return loc, nil, v1.Coded(http.StatusBadRequest, "isFolder", "path is a folder")
	}
	// Reading a pipe or a device would hang or never end.
	if !info.Mode().IsRegular() {
		return loc, nil, v1.Coded(http.StatusBadRequest, "notAFile", "path is not a regular file")
	}
	return loc, info, nil
}

func (h handlers) raw(w http.ResponseWriter, r *http.Request) error {
	q := r.URL.Query()
	loc, info, err := h.file(v1.UserFrom(r), q.Get("path"), ccfs.Download)
	if errors.Is(err, errArchived) {
		return h.archiveRaw(w, r, loc)
	}
	if err != nil {
		return err
	}
	return ServeFile(w, r, loc.Real, path.Base(loc.Path), info, flag(q.Get("download")))
}

// ServeFile sends the regular file at real under name, with range and
// conditional requests. download asks the browser to save it.
func ServeFile(w http.ResponseWriter, r *http.Request, real, name string, info os.FileInfo, download bool) error {
	f, err := os.Open(real)
	if err != nil {
		return err
	}
	defer f.Close()
	ctype, err := contentType(name, f)
	if err != nil {
		return err
	}
	hdr := w.Header()
	setDelivery(hdr, ctype, name, download)
	hdr.Set("ETag", strongTag(info))
	hdr.Set("Cache-Control", "private, no-cache")
	http.ServeContent(w, r, name, info.ModTime(), f)
	return nil
}

// SandboxPolicy gives a response from the share a unique origin, so a page
// or a drawing in it cannot run scripts with the app's origin and session.
const SandboxPolicy = "sandbox; default-src 'none'; img-src data:; style-src 'unsafe-inline'; frame-ancestors 'self'"

// setDelivery sets how a browser may treat a file from the share. What it
// does not know to be harmless is sandboxed, because browsers run scripts in
// more types than anyone lists, such as any XML with an XHTML element in it.
func setDelivery(hdr http.Header, ctype, name string, download bool) {
	hdr.Set("Content-Type", ctype)
	hdr.Set("X-Content-Type-Options", "nosniff")
	if !passive(ctype) {
		hdr.Set("Content-Security-Policy", SandboxPolicy)
	}
	disposition := "inline"
	if download {
		disposition = "attachment"
	}
	hdr.Set("Content-Disposition", mime.FormatMediaType(disposition, map[string]string{"filename": name}))
}

func (h handlers) text(w http.ResponseWriter, r *http.Request) error {
	q := r.URL.Query()
	limit, err := intParam(q.Get("max"), textDefault, 1, textMax)
	if err != nil {
		return err
	}
	loc, info, err := h.file(v1.UserFrom(r), q.Get("path"), ccfs.Download)
	if errors.Is(err, errArchived) {
		return h.archiveText(w, r, loc, limit, q.Get("encoding"))
	}
	if err != nil {
		return err
	}
	f, err := os.Open(loc.Real)
	if err != nil {
		return err
	}
	defer f.Close()
	buf, err := io.ReadAll(io.LimitReader(f, limit))
	if err != nil {
		return err
	}
	truncated := int64(len(buf)) < info.Size()
	enc, text, err := readText(buf, q.Get("encoding"), truncated)
	if err != nil {
		return err
	}
	out := Text{
		Path:      loc.Path,
		Size:      info.Size(),
		MTime:     formatTime(info.ModTime()),
		Encoding:  enc,
		EOL:       lineEnding(text),
		Text:      text,
		Truncated: truncated,
	}
	return v1.WriteJSON(w, r, http.StatusOK, out)
}

func (h handlers) hex(w http.ResponseWriter, r *http.Request) error {
	q := r.URL.Query()
	offset, err := intParam(q.Get("offset"), 0, 0, 1<<62)
	if err != nil {
		return err
	}
	length, err := intParam(q.Get("length"), hexDefault, 1, hexMax)
	if err != nil {
		return err
	}
	loc, info, err := h.file(v1.UserFrom(r), q.Get("path"), ccfs.Download)
	if errors.Is(err, errArchived) {
		return h.archiveHex(w, r, loc, offset, length)
	}
	if err != nil {
		return err
	}
	f, err := os.Open(loc.Real)
	if err != nil {
		return err
	}
	defer f.Close()
	data := make([]byte, length)
	n, err := f.ReadAt(data, offset)
	if err != nil && !errors.Is(err, io.EOF) {
		return err
	}
	return v1.WriteJSON(w, r, http.StatusOK, Hex{Path: loc.Path, Size: info.Size(), Offset: offset, Data: data[:n]})
}

func errBinary() error {
	return v1.Coded(http.StatusUnsupportedMediaType, "notText", "file is not text")
}

// decodeText finds the encoding of buf and returns its text. The checks
// run from the surest sign to the weakest: a byte order mark, then valid
// UTF-8, then Windows-1252, which older Windows tools write in Europe.
func decodeText(buf []byte, truncated bool) (encoding, text string, err error) {
	switch {
	case bytes.HasPrefix(buf, []byte{0xEF, 0xBB, 0xBF}):
		return "utf-8-bom", string(trimRune(buf[3:], truncated)), nil
	case bytes.HasPrefix(buf, []byte{0xFF, 0xFE}):
		return decodeUTF16("utf-16le", unicode.LittleEndian, buf, truncated)
	case bytes.HasPrefix(buf, []byte{0xFE, 0xFF}):
		return decodeUTF16("utf-16be", unicode.BigEndian, buf, truncated)
	}
	if bytes.IndexByte(buf[:min(len(buf), 8000)], 0) >= 0 {
		return "", "", errBinary()
	}
	cut := trimRune(buf, truncated)
	if utf8.Valid(cut) {
		for _, b := range cut {
			if b >= 0x80 {
				return "utf-8", string(cut), nil
			}
		}
		return "ascii", string(cut), nil
	}
	out, err := charmap.Windows1252.NewDecoder().Bytes(buf)
	if err != nil {
		return "", "", errBinary()
	}
	return "windows-1252", string(out), nil
}

func decodeUTF16(name string, order unicode.Endianness, buf []byte, truncated bool) (string, string, error) {
	if truncated && len(buf)%2 == 1 {
		buf = buf[:len(buf)-1]
	}
	out, err := unicode.UTF16(order, unicode.ExpectBOM).NewDecoder().Bytes(buf)
	if err != nil {
		return "", "", errBinary()
	}
	return name, string(out), nil
}

// trimRune drops a character the read limit cut in half.
func trimRune(b []byte, truncated bool) []byte {
	if !truncated {
		return b
	}
	for i := 0; i < utf8.UTFMax && len(b) > 0; i++ {
		r, size := utf8.DecodeLastRune(b)
		if r != utf8.RuneError || size > 1 {
			break
		}
		b = b[:len(b)-1]
	}
	return b
}

func lineEnding(s string) string {
	crlf := strings.Count(s, "\r\n")
	lf := strings.Count(s, "\n") - crlf
	cr := strings.Count(s, "\r") - crlf
	switch crlf + lf + cr {
	case 0:
		return ""
	case crlf:
		return "crlf"
	case lf:
		return "lf"
	case cr:
		return "cr"
	}
	return "mixed"
}

func contentType(name string, f *os.File) (string, error) {
	if t := mime.TypeByExtension(path.Ext(name)); t != "" {
		return t, nil
	}
	head := make([]byte, 512)
	n, err := io.ReadFull(f, head)
	if err != nil && !errors.Is(err, io.ErrUnexpectedEOF) && !errors.Is(err, io.EOF) {
		return "", err
	}
	if _, err := f.Seek(0, io.SeekStart); err != nil {
		return "", err
	}
	return http.DetectContentType(head[:n]), nil
}

// passive reports whether browsers only ever display ctype. The viewers need
// these without the sandbox: a browser's PDF viewer refuses a sandboxed
// document, and pictures, films and sound go to their own elements.
func passive(ctype string) bool {
	t, _, _ := mime.ParseMediaType(ctype)
	switch t {
	case "image/png", "image/apng", "image/jpeg", "image/gif", "image/webp", "image/avif", "image/bmp", "image/tiff",
		"image/x-icon", "image/vnd.microsoft.icon", "application/pdf", "text/plain", "text/markdown":
		return true
	}
	return strings.HasPrefix(t, "video/") || strings.HasPrefix(t, "audio/")
}

// strongTag changes whenever the file is written, which is what range
// requests need to resume safely.
func strongTag(info os.FileInfo) string {
	return fmt.Sprintf(`"%x-%x"`, info.Size(), info.ModTime().UnixNano())
}

func flag(s string) bool {
	b, _ := strconv.ParseBool(s)
	return b
}

func intParam(s string, def, lo, hi int64) (int64, error) {
	if s == "" {
		return def, nil
	}
	n, err := strconv.ParseInt(s, 10, 64)
	if err != nil || n < lo || n > hi {
		return 0, v1.Errorf(http.StatusBadRequest, fmt.Sprintf("expected a number from %d to %d, got %q", lo, hi, s))
	}
	return n, nil
}
