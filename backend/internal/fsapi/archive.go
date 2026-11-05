package fsapi

import (
	"archive/tar"
	"archive/zip"
	"compress/flate"
	"compress/gzip"
	"context"
	"io"
	"io/fs"
	"mime"
	"net/http"
	"os"
	"path"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/gtsteffaniak/go-logger/logger"

	v1 "github.com/junkerderprovinz/cannoncommander/backend/internal/api/v1"
	"github.com/junkerderprovinz/cannoncommander/backend/internal/ccfs"
)

// Archive formats for downloads.
const (
	FormatZip = "zip"
	FormatTgz = "tgz"
)

const maxArchivePaths = 10000

// ArchiveEntry is one top level entry of a download archive.
type ArchiveEntry struct {
	// Name is what the entry is called inside the archive.
	Name string
	Loc  ccfs.Location
	// Bound is the real folder a symbolic link inside the entry may point
	// into. Empty means the source of the entry.
	Bound string
}

// ArchiveOptions say what goes into a download archive.
type ArchiveOptions struct {
	Format string
	// Visible decides about every entry below the top level, which is
	// where the access rules of a user or a share come in.
	Visible func(dir ccfs.Location, name string) bool
	// Follows decides about a link below the top level, which comes along as
	// the file it leads to.
	Follows func(ccfs.Location) bool
	// KeepOwners writes user and group into a tar archive. Visitors of a
	// share get no such details.
	KeepOwners bool
}

// ArchiveName is the file name a download of entries gets.
func ArchiveName(base, format string) string {
	if format == FormatTgz {
		return base + ".tar.gz"
	}
	return base + ".zip"
}

// ServeArchive streams entries as one archive file called name. The server
// packs while it sends, so nothing is left behind on disk.
func ServeArchive(w http.ResponseWriter, r *http.Request, fsys *ccfs.Resolver, name string, entries []ArchiveEntry, opts ArchiveOptions) {
	h := w.Header()
	h.Set("Content-Type", "application/zip")
	if opts.Format == FormatTgz {
		h.Set("Content-Type", "application/gzip")
	}
	h.Set("Content-Disposition", mime.FormatMediaType("attachment", map[string]string{"filename": name}))
	h.Set("Cache-Control", "no-store")
	h.Set("X-Content-Type-Options", "nosniff")
	w.WriteHeader(http.StatusOK)
	if err := WriteArchive(r.Context(), w, fsys, entries, opts); err != nil {
		if r.Context().Err() == nil {
			logger.Errorf("archive %s: %v", name, err)
		}
		// The status is out already. Breaking the connection makes the
		// browser report a failed download instead of keeping half a file.
		panic(http.ErrAbortHandler)
	}
}

// WriteArchive writes entries to w as zip or tar.gz.
func WriteArchive(ctx context.Context, w io.Writer, fsys *ccfs.Resolver, entries []ArchiveEntry, opts ArchiveOptions) error {
	var a archiver
	switch opts.Format {
	case FormatTgz:
		gz, _ := gzip.NewWriterLevel(w, gzip.DefaultCompression)
		a = &tarArchiver{gz: gz, tw: tar.NewWriter(gz), owners: opts.KeepOwners}
	default:
		zw := zip.NewWriter(w)
		zw.RegisterCompressor(zip.Deflate, func(out io.Writer) (io.WriteCloser, error) {
			return flate.NewWriter(out, flate.BestSpeed)
		})
		a = &zipArchiver{zw: zw}
	}
	walk := archiveWalk{ctx: ctx, fs: fsys, a: a, visible: opts.Visible, follows: opts.Follows}
	for _, e := range entries {
		bound := e.Bound
		if bound == "" && e.Loc.Source != nil {
			bound = e.Loc.Source.Root
		}
		real, err := filepath.EvalSymlinks(bound)
		if err != nil {
			return err
		}
		walk.bound = real
		if err := walk.entry(e.Loc, e.Name, true); err != nil {
			return err
		}
	}
	return a.Close()
}

type archiver interface {
	Dir(name string, info fs.FileInfo) error
	File(name string, info fs.FileInfo, r io.Reader) error
	Close() error
}

type archiveWalk struct {
	ctx     context.Context
	fs      *ccfs.Resolver
	a       archiver
	visible func(dir ccfs.Location, name string) bool
	follows func(ccfs.Location) bool
	bound   string
}

func (wk *archiveWalk) entry(loc ccfs.Location, name string, top bool) error {
	if err := wk.ctx.Err(); err != nil {
		return err
	}
	info, err := os.Lstat(loc.Real)
	if err != nil {
		if top {
			return err
		}
		// Gone between reading the folder and getting here.
		return nil
	}
	if info.Mode()&fs.ModeSymlink != 0 {
		// Linked folders could loop or lead anywhere, linked files come
		// along when they stay inside the bound.
		target, err := filepath.EvalSymlinks(loc.Real)
		if err != nil || !within(wk.bound, target) || wk.follows != nil && !wk.follows(loc) {
			return nil
		}
		if info, err = os.Stat(target); err != nil || !info.Mode().IsRegular() {
			return nil
		}
	}
	switch {
	case info.IsDir():
		return wk.dir(loc, name, info)
	case info.Mode().IsRegular():
		f, err := os.Open(loc.Real)
		if err != nil {
			if top {
				return err
			}
			return nil
		}
		defer f.Close()
		return wk.a.File(name, info, f)
	}
	return nil
}

func (wk *archiveWalk) dir(loc ccfs.Location, name string, info fs.FileInfo) error {
	if err := wk.a.Dir(name, info); err != nil {
		return err
	}
	dirents, err := os.ReadDir(loc.Real)
	if err != nil {
		return err
	}
	for _, d := range dirents {
		n := d.Name()
		if bookkeeping(n) || (wk.visible != nil && !wk.visible(loc, n)) {
			continue
		}
		child, err := wk.fs.Resolve(path.Join(loc.Path, n))
		if err != nil {
			continue
		}
		if err := wk.entry(child, name+"/"+n, false); err != nil {
			return err
		}
	}
	return nil
}

type zipArchiver struct{ zw *zip.Writer }

func (z *zipArchiver) Dir(name string, info fs.FileInfo) error {
	_, err := z.zw.CreateHeader(&zip.FileHeader{Name: name + "/", Modified: info.ModTime(), Method: zip.Store})
	return err
}

func (z *zipArchiver) File(name string, info fs.FileInfo, r io.Reader) error {
	hdr := &zip.FileHeader{Name: name, Modified: info.ModTime(), Method: zip.Deflate}
	hdr.SetMode(info.Mode().Perm())
	if packed(name) {
		hdr.Method = zip.Store
	}
	out, err := z.zw.CreateHeader(hdr)
	if err != nil {
		return err
	}
	_, err = io.Copy(out, r)
	return err
}

func (z *zipArchiver) Close() error { return z.zw.Close() }

type tarArchiver struct {
	gz     *gzip.Writer
	tw     *tar.Writer
	owners bool
}

func (t *tarArchiver) header(name string, info fs.FileInfo) (*tar.Header, error) {
	hdr, err := tar.FileInfoHeader(info, "")
	if err != nil {
		return nil, err
	}
	hdr.Name = name
	if !t.owners {
		hdr.Uid, hdr.Gid, hdr.Uname, hdr.Gname = 0, 0, "", ""
	}
	return hdr, nil
}

func (t *tarArchiver) Dir(name string, info fs.FileInfo) error {
	hdr, err := t.header(name+"/", info)
	if err != nil {
		return err
	}
	return t.tw.WriteHeader(hdr)
}

func (t *tarArchiver) File(name string, info fs.FileInfo, r io.Reader) error {
	hdr, err := t.header(name, info)
	if err != nil {
		return err
	}
	if err := t.tw.WriteHeader(hdr); err != nil {
		return err
	}
	// A file that grows while it is read must not overrun its header.
	_, err = io.Copy(t.tw, io.LimitReader(r, info.Size()))
	return err
}

func (t *tarArchiver) Close() error {
	if err := t.tw.Close(); err != nil {
		return err
	}
	return t.gz.Close()
}

// packed reports formats that are compressed already, where deflate only
// costs time.
func packed(name string) bool {
	switch strings.ToLower(path.Ext(name)) {
	case ".jpg", ".jpeg", ".png", ".gif", ".webp", ".heic", ".avif", ".mp4", ".m4v", ".mkv", ".mov", ".avi", ".webm",
		".mp3", ".m4a", ".aac", ".ogg", ".opus", ".flac", ".zip", ".7z", ".rar", ".gz", ".tgz", ".bz2", ".xz", ".zst", ".pdf", ".docx", ".xlsx", ".pptx":
		return true
	}
	return false
}

// NumberDuplicates gives entries of the same name from different folders a
// number, as the other side of a collection sees them: the second
// IMG_1.jpg becomes IMG_1 (2).jpg.
func NumberDuplicates(names []string, dirs []bool) []string {
	seen := map[string]int{}
	out := make([]string, len(names))
	for i, n := range names {
		seen[n]++
		k := seen[n]
		if k == 1 {
			out[i] = n
			continue
		}
		dot := strings.LastIndex(n, ".")
		if dirs[i] || dot <= 0 {
			out[i] = n + " (" + strconv.Itoa(k) + ")"
		} else {
			out[i] = n[:dot] + " (" + strconv.Itoa(k) + ")" + n[dot:]
		}
	}
	return out
}

// within reports whether the real path p is root or lies below it.
func within(root, p string) bool {
	rel, err := filepath.Rel(root, p)
	return err == nil && rel != ".." && !strings.HasPrefix(rel, ".."+string(filepath.Separator)) && !filepath.IsAbs(rel)
}

type archiveRequest struct {
	Paths  []string `json:"paths"`
	Format string   `json:"format"`
	Name   string   `json:"name"`
}

// readArchiveRequest takes JSON from scripts and a plain form from the
// browser, which can then save the stream as it arrives.
func readArchiveRequest(r *http.Request) (archiveRequest, error) {
	var req archiveRequest
	ctype, _, _ := mime.ParseMediaType(r.Header.Get("Content-Type"))
	if ctype == "application/x-www-form-urlencoded" || ctype == "multipart/form-data" {
		r.Body = http.MaxBytesReader(nil, r.Body, 4<<20)
		if err := r.ParseForm(); err != nil {
			return req, v1.Errorf(http.StatusBadRequest, "invalid form")
		}
		req = archiveRequest{Paths: r.PostForm["paths"], Format: r.PostForm.Get("format"), Name: r.PostForm.Get("name")}
	} else if err := v1.DecodeJSON(r, &req); err != nil {
		return req, err
	}
	switch req.Format {
	case "":
		req.Format = FormatZip
	case FormatZip, FormatTgz:
	default:
		return req, v1.Errorf(http.StatusBadRequest, "format must be zip or tgz")
	}
	if len(req.Paths) == 0 {
		return req, v1.Errorf(http.StatusBadRequest, "choose at least one path")
	}
	if len(req.Paths) > maxArchivePaths {
		return req, v1.Errorf(http.StatusBadRequest, "too many paths for one download")
	}
	return req, nil
}

func (h handlers) archiveStream(w http.ResponseWriter, r *http.Request) error {
	u := v1.UserFrom(r)
	req, err := readArchiveRequest(r)
	if err != nil {
		return err
	}
	entries := make([]ArchiveEntry, 0, len(req.Paths))
	names := make([]string, 0, len(req.Paths))
	dirs := make([]bool, 0, len(req.Paths))
	for _, p := range req.Paths {
		loc, err := h.fs.Entry(u, p, ccfs.View|ccfs.Download)
		if err != nil {
			return err
		}
		if loc.Virtual() || loc.InArchive() {
			return v1.Errorf(http.StatusBadRequest, "only entries inside a source can be downloaded")
		}
		info, err := os.Stat(loc.Real)
		if err != nil {
			return err
		}
		entries = append(entries, ArchiveEntry{Loc: loc})
		names = append(names, path.Base(loc.Path))
		dirs = append(dirs, info.IsDir())
	}
	for i, n := range NumberDuplicates(names, dirs) {
		entries[i].Name = n
	}
	name := req.Name
	if name == "" || !ccfs.ValidName(name) {
		base := names[0]
		if len(names) > 1 {
			if base = path.Base(path.Dir(entries[0].Loc.Path)); base == "/" || base == "." {
				base = "download"
			}
		}
		name = ArchiveName(base, req.Format)
	}
	visible := func(dir ccfs.Location, n string) bool { return h.fs.Visible(u, dir, n) }
	follows := func(loc ccfs.Location) bool { return h.fs.Follows(u, loc, ccfs.View|ccfs.Download) == nil }
	ServeArchive(w, r, h.fs, name, entries, ArchiveOptions{Format: req.Format, Visible: visible, Follows: follows, KeepOwners: true})
	return nil
}
