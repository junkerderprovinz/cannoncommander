// Package archives reads zip, tar in its usual compressions, 7z and rar,
// writes zip, tar.gz, tar.zst and, through external programs, 7z and rar,
// and unpacks all of them without letting an entry land outside the target.
package archives

import (
	"errors"
	"fmt"
	"io"
	"io/fs"
	"net/http"
	"path"
	"strings"
	"time"

	v1 "github.com/junkerderprovinz/cannoncommander/backend/internal/api/v1"
)

// Format is an archive format, named as it appears at the end of a file.
type Format string

const (
	Zip    Format = "zip"
	Tar    Format = "tar"
	TarGz  Format = "tar.gz"
	TarZst Format = "tar.zst"
	TarXz  Format = "tar.xz"
	TarBz2 Format = "tar.bz2"
	SevenZ Format = "7z"
	Rar    Format = "rar"
)

var (
	// ErrPassword means the archive or an entry is encrypted and the
	// password is missing or wrong.
	ErrPassword = errors.New("the archive needs the right password")
	// ErrUnsupported means the file is not an archive this package reads.
	ErrUnsupported = errors.New("this is not an archive that can be opened")
)

// UnsafeError reports an entry whose name or link target would leave the
// folder it is unpacked into.
type UnsafeError struct {
	Name string
}

func (e *UnsafeError) Error() string {
	return fmt.Sprintf("entry %q points outside the archive", e.Name)
}

var suffixes = []struct {
	ext string
	f   Format
}{
	{".tar.gz", TarGz}, {".tgz", TarGz},
	{".tar.zst", TarZst}, {".tzst", TarZst},
	{".tar.xz", TarXz}, {".txz", TarXz},
	{".tar.bz2", TarBz2}, {".tbz2", TarBz2}, {".tbz", TarBz2},
	{".tar", Tar}, {".zip", Zip}, {".7z", SevenZ}, {".rar", Rar},
}

// FormatOf returns the format a file name says, or "" for none.
func FormatOf(name string) Format {
	lower := strings.ToLower(name)
	for _, s := range suffixes {
		if strings.HasSuffix(lower, s.ext) && len(lower) > len(s.ext) {
			return s.f
		}
	}
	return ""
}

// StripExt removes the archive ending from a name, so fotos.tar.zst
// unpacks into fotos.
func StripExt(name string) string {
	lower := strings.ToLower(name)
	for _, s := range suffixes {
		if strings.HasSuffix(lower, s.ext) && len(lower) > len(s.ext) {
			return name[:len(name)-len(s.ext)]
		}
	}
	return name
}

// Entry is one member of an archive. Name is a relative slash path without
// "..", already checked by CleanName.
type Entry struct {
	Name      string      `json:"name"`
	Dir       bool        `json:"dir"`
	Size      int64       `json:"size"`
	Mode      fs.FileMode `json:"-"`
	ModTime   time.Time   `json:"mtime"`
	Link      string      `json:"link,omitempty"`
	HardLink  string      `json:"-"`
	Encrypted bool        `json:"encrypted,omitempty"`
	// Outside marks a link whose target lies outside the archive. It is
	// listed but never unpacked.
	Outside bool `json:"outside,omitempty"`
}

// Symlink reports whether the entry is a symbolic link.
func (e Entry) Symlink() bool { return e.Mode&fs.ModeSymlink != 0 }

// Opener reads the content of the entry Walk is looking at. It is valid
// only during that call.
type Opener func() (io.ReadCloser, error)

// Archive is an opened archive.
type Archive interface {
	// Walk calls fn for every entry in the order the archive stores them.
	// Stream formats such as tar.gz can only be walked once.
	Walk(fn func(e Entry, open Opener) error) error
	Close() error
}

// Options tune how an archive is read.
type Options struct {
	Password string
	// Charset is the encoding of zip names that carry no UTF-8 flag. Empty
	// means a guess, which falls back to the zip default CP437.
	Charset string
}

// Open opens the archive at file, choosing the reader by its name.
func Open(file string, o Options) (Archive, error) {
	switch FormatOf(file) {
	case Zip:
		return openZip(file, o)
	case Tar, TarGz, TarZst, TarXz, TarBz2:
		return openTar(file, FormatOf(file))
	case SevenZ:
		return open7z(file, o)
	case Rar:
		return openRar(file, o)
	}
	return nil, ErrUnsupported
}

// CleanName turns an entry name into a relative slash path, or reports it
// as unsafe. Backslashes count as separators, because Windows tools write
// them and a name like "..\x" would otherwise slip through on Linux and
// climb out on Windows.
func CleanName(name string) (string, error) {
	s := strings.ReplaceAll(name, "\\", "/")
	if strings.ContainsRune(s, 0) {
		return "", &UnsafeError{Name: name}
	}
	if len(s) >= 2 && s[1] == ':' && (s[0]|0x20 >= 'a' && s[0]|0x20 <= 'z') {
		return "", &UnsafeError{Name: name}
	}
	for _, seg := range strings.Split(s, "/") {
		if seg == ".." {
			return "", &UnsafeError{Name: name}
		}
	}
	s = strings.TrimPrefix(path.Clean("/"+s), "/")
	if s == "" {
		return "", &UnsafeError{Name: name}
	}
	return s, nil
}

// linkStaysInside reports whether a symlink at name pointing to target
// resolves to a place inside the archive root.
func linkStaysInside(name, target string) bool {
	t := strings.ReplaceAll(target, "\\", "/")
	if t == "" || strings.HasPrefix(t, "/") || strings.ContainsRune(t, 0) || (len(t) >= 2 && t[1] == ':') {
		return false
	}
	rel := path.Join(path.Dir(name), t)
	return rel != ".." && !strings.HasPrefix(rel, "../")
}

// entryFor fills in an entry and checks its name and link.
func entryFor(raw string, mode fs.FileMode, size int64, mtime time.Time, link string) (Entry, error) {
	name, err := CleanName(raw)
	if err != nil {
		return Entry{}, err
	}
	e := Entry{Name: name, Dir: mode.IsDir() || strings.HasSuffix(raw, "/"), Size: size, Mode: mode, ModTime: mtime}
	if mode&fs.ModeSymlink != 0 {
		e.Link, e.Size, e.Dir = link, 0, false
		e.Outside = !linkStaysInside(name, link)
	}
	if e.Dir {
		e.Size = 0
	}
	return e, nil
}

// Problem gives the errors of the readers the status the API answers with.
func Problem(err error) error {
	var unsafe *UnsafeError
	switch {
	case errors.Is(err, ErrPassword):
		return v1.Errorf(http.StatusPreconditionRequired, "the archive needs the right password")
	case errors.Is(err, ErrUnsupported):
		return v1.Errorf(http.StatusUnprocessableEntity, "the archive cannot be read")
	case errors.As(err, &unsafe):
		return v1.Errorf(http.StatusUnprocessableEntity, unsafe.Error())
	}
	return err
}
