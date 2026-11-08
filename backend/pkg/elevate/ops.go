// Package elevate does file operations with administrator rights for the
// desktop app, the way the Explorer goes on after "You need administrator
// permission". The app starts a second copy of itself through UAC on Windows
// or pkexec on Linux, hands it one job over a loopback connection that both
// sides prove with a secret, and the copy exits once the job is through. It
// knows a handful of operations on plain absolute paths and runs no programs.
package elevate

import (
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"syscall"
)

// The operations the helper knows.
const (
	Copy   = "copy"
	Move   = "move"
	Delete = "delete"
	// Recycle puts From in the Recycle Bin, which only Windows does with
	// administrator rights; on Linux root has no trash of the person.
	Recycle = "recycle"
	Mkdir   = "mkdir"
)

// ErrBadOp is an operation the helper refuses to look at.
var ErrBadOp = errors.New("not an operation the helper does")

// Op is one operation. Copy and Move take From to To, which must not exist
// unless Replace is set; then a folder is merged into the one at To and
// anything else at To is replaced.
type Op struct {
	Kind    string `json:"kind"`
	From    string `json:"from,omitempty"`
	To      string `json:"to,omitempty"`
	Replace bool   `json:"replace,omitempty"`
}

func (o Op) check() error {
	var need, none []string
	switch o.Kind {
	case Copy, Move:
		need = []string{o.From, o.To}
	case Delete, Recycle:
		need, none = []string{o.From}, []string{o.To}
	case Mkdir:
		need, none = []string{o.To}, []string{o.From}
	default:
		return ErrBadOp
	}
	for _, p := range none {
		if p != "" {
			return ErrBadOp
		}
	}
	for _, p := range need {
		if !plain(p) {
			return ErrBadOp
		}
	}
	if (o.Kind == Copy || o.Kind == Move) && within(o.To, o.From) {
		return ErrBadOp
	}
	return nil
}

// plain accepts an absolute, cleaned path below the root of a drive or of
// the file system, and on Windows only on a drive letter: no share, no
// device and no \\?\ form, which skip the checks Windows does on a name.
func plain(p string) bool {
	if p == "" || strings.ContainsRune(p, 0) || !filepath.IsAbs(p) || filepath.Clean(p) != p || filepath.Dir(p) == p {
		return false
	}
	if vol := filepath.VolumeName(p); vol != "" && (len(vol) != 2 || vol[1] != ':') {
		return false
	}
	return true
}

func within(p, root string) bool {
	rel, err := filepath.Rel(root, p)
	return err == nil && rel != ".." && !strings.HasPrefix(rel, ".."+string(filepath.Separator))
}

// recycle is set on Windows, where the helper can put things in the Recycle
// Bin.
var recycle func(path string) error

func apply(o Op) error {
	if err := o.check(); err != nil {
		return err
	}
	switch o.Kind {
	case Delete:
		if _, err := os.Lstat(o.From); err != nil {
			return err
		}
		return os.RemoveAll(o.From)
	case Recycle:
		if recycle == nil {
			return fmt.Errorf("%w: no Recycle Bin here", ErrBadOp)
		}
		return recycle(o.From)
	case Mkdir:
		return os.Mkdir(o.To, 0o777)
	}
	info, err := os.Lstat(o.From)
	if err != nil {
		return err
	}
	if st, err := os.Stat(filepath.Dir(o.To)); err != nil {
		return err
	} else if !st.IsDir() {
		return &fs.PathError{Op: o.Kind, Path: filepath.Dir(o.To), Err: syscall.ENOTDIR}
	}
	target, err := os.Lstat(o.To)
	switch {
	case err == nil && !o.Replace:
		return &fs.PathError{Op: o.Kind, Path: o.To, Err: fs.ErrExist}
	case err == nil && !(info.IsDir() && target.IsDir()):
		if err := os.RemoveAll(o.To); err != nil {
			return err
		}
	case err != nil && !errors.Is(err, fs.ErrNotExist):
		return err
	}
	if o.Kind == Move {
		if errors.Is(err, fs.ErrNotExist) || !info.IsDir() || !target.IsDir() {
			if err := os.Rename(o.From, o.To); err == nil {
				return nil
			}
		}
		if err := copyTree(o.From, o.To); err != nil {
			return err
		}
		return os.RemoveAll(o.From)
	}
	return copyTree(o.From, o.To)
}

// copyTree copies from to to, merging into a folder that is there and
// replacing files in it. Links are copied as links, never followed.
func copyTree(from, to string) error {
	info, err := os.Lstat(from)
	if err != nil {
		return err
	}
	switch {
	case info.Mode()&fs.ModeSymlink != 0:
		dest, err := os.Readlink(from)
		if err != nil {
			return err
		}
		os.Remove(to)
		return os.Symlink(dest, to)
	case info.IsDir():
		if err := os.Mkdir(to, info.Mode().Perm()); err != nil && !errors.Is(err, fs.ErrExist) {
			return err
		}
		entries, err := os.ReadDir(from)
		if err != nil {
			return err
		}
		for _, e := range entries {
			if err := copyTree(filepath.Join(from, e.Name()), filepath.Join(to, e.Name())); err != nil {
				return err
			}
		}
		return os.Chtimes(to, info.ModTime(), info.ModTime())
	case info.Mode().IsRegular():
		return copyFile(from, to, info)
	}
	return &fs.PathError{Op: "copy", Path: from, Err: errors.ErrUnsupported}
}

func copyFile(from, to string, info fs.FileInfo) error {
	in, err := os.Open(from)
	if err != nil {
		return err
	}
	defer in.Close()
	if t, err := os.Lstat(to); err == nil && t.IsDir() {
		if err := os.RemoveAll(to); err != nil {
			return err
		}
	}
	out, err := os.OpenFile(to, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, info.Mode().Perm())
	if err != nil {
		return err
	}
	if _, err := io.Copy(out, in); err != nil {
		out.Close()
		return err
	}
	if err := out.Close(); err != nil {
		return err
	}
	return os.Chtimes(to, info.ModTime(), info.ModTime())
}

// codeOf names a failure the way the server's problems do, so the app shows
// it in its own words.
func codeOf(err error) string {
	switch {
	case errors.Is(err, ErrBadOp):
		return "badPath"
	case errors.Is(err, fs.ErrPermission):
		return "denied"
	case errors.Is(err, fs.ErrExist):
		return "nameTaken"
	case errors.Is(err, fs.ErrNotExist):
		return "notFound"
	}
	return ""
}

// Recycles reports whether the helper can put things in the Recycle Bin.
func Recycles() bool { return recycle != nil }
