// Package elevate does file operations with administrator rights for the
// desktop app, the way the Explorer goes on after "You need administrator
// permission". The app starts a second copy of itself through UAC on Windows
// or pkexec on Linux and hands it one job, on Windows over a pipe where each
// side checks the other's process, on Linux over a loopback connection that
// both sides prove with a secret from stdin. The copy exits once the job is
// through. It knows a handful of operations on plain absolute paths, follows
// no links and runs no programs.
package elevate

import (
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path"
	"path/filepath"
	"runtime"
	"strings"

	"github.com/junkerderprovinz/cannoncommander/backend/internal/plainpath"
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
// anything else at To is replaced. Skip names what lies below From and stays
// behind, which is what the app hides from the person. Parents makes the
// missing folders above To first.
type Op struct {
	Kind    string   `json:"kind"`
	From    string   `json:"from,omitempty"`
	To      string   `json:"to,omitempty"`
	Replace bool     `json:"replace,omitempty"`
	Skip    []string `json:"skip,omitempty"`
	Parents bool     `json:"parents,omitempty"`
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
	if len(o.Skip) > 0 && o.Kind != Copy && o.Kind != Move || o.Parents && o.Kind != Copy && o.Kind != Move {
		return ErrBadOp
	}
	for _, p := range o.Skip {
		if !plain(p) || p == o.From || !within(p, o.From) {
			return ErrBadOp
		}
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

// apply does one operation. The paths were chosen while the prompt was open,
// and whoever runs the app could have put a link into them since, so right
// before the work every part of both paths is checked, and the work itself
// goes through folder handles opened one level at a time, which no link
// leads out of. A merge never goes into a link at the target, and a file is
// made anew rather than written through whatever was there.
func apply(o Op) error {
	if err := o.check(); err != nil {
		return err
	}
	for _, p := range []string{o.From, o.To} {
		if p == "" {
			continue
		}
		if err := plainpath.Path(p); err != nil {
			return err
		}
	}
	switch o.Kind {
	case Delete:
		parent, err := openDir(filepath.Dir(o.From), false)
		if err != nil {
			return err
		}
		defer parent.Close()
		name := filepath.Base(o.From)
		if _, err := parent.Lstat(name); err != nil {
			return err
		}
		return parent.RemoveAll(name)
	case Recycle:
		if recycle == nil {
			return fmt.Errorf("%w: no Recycle Bin here", ErrBadOp)
		}
		return recycle(o.From)
	case Mkdir:
		parent, err := openDir(filepath.Dir(o.To), false)
		if err != nil {
			return err
		}
		defer parent.Close()
		return parent.Mkdir(filepath.Base(o.To), 0o777)
	}
	src, err := openDir(filepath.Dir(o.From), false)
	if err != nil {
		return err
	}
	defer src.Close()
	dst, err := openDir(filepath.Dir(o.To), o.Parents)
	if err != nil {
		return err
	}
	defer dst.Close()
	from, to := filepath.Base(o.From), filepath.Base(o.To)
	info, err := src.Lstat(from)
	if err != nil {
		return err
	}
	target, err := dst.Lstat(to)
	switch {
	case err == nil && !o.Replace:
		return &fs.PathError{Op: o.Kind, Path: o.To, Err: fs.ErrExist}
	case err == nil && !(info.IsDir() && target.IsDir()):
		if err := dst.RemoveAll(to); err != nil {
			return err
		}
	case err != nil && !errors.Is(err, fs.ErrNotExist):
		return err
	}
	skip := make(map[string]bool, len(o.Skip))
	for _, p := range o.Skip {
		rel, err := filepath.Rel(o.From, p)
		if err != nil {
			return ErrBadOp
		}
		skip[filepath.ToSlash(rel)] = true
	}
	// Within one volume a move is a rename relative to the two checked
	// folders, which is quick and leaves nothing half done. Only across
	// volumes, or to merge or leave parts behind, it copies and deletes.
	if o.Kind == Move && len(skip) == 0 && (errors.Is(err, fs.ErrNotExist) || !(info.IsDir() && target.IsDir())) {
		err := renameAt(src, from, dst, to)
		if err == nil || !crossVolume(err) {
			return err
		}
	}
	if err := copyTree(src, from, dst, to, ".", skip); err != nil {
		return err
	}
	if o.Kind == Move {
		return removeTree(src, from, ".", skip)
	}
	return nil
}

// openDir opens dir as a handle, going down from the top of its volume one
// folder at a time and refusing a link on the way. With create, the missing
// folders are made.
func openDir(dir string, create bool) (*os.Root, error) {
	vol := filepath.VolumeName(dir)
	r, err := os.OpenRoot(vol + string(filepath.Separator))
	if err != nil {
		return nil, err
	}
	at := vol
	for _, part := range strings.Split(dir[len(vol):], string(filepath.Separator)) {
		if part == "" {
			continue
		}
		at += string(filepath.Separator) + part
		fi, err := r.Lstat(part)
		if create && errors.Is(err, fs.ErrNotExist) {
			if err = r.Mkdir(part, 0o777); err == nil || errors.Is(err, fs.ErrExist) {
				fi, err = r.Lstat(part)
			}
		}
		if err == nil && plainpath.Link(fi, at) {
			err = &fs.PathError{Op: "open", Path: at, Err: plainpath.ErrLink}
		}
		var next *os.Root
		if err == nil {
			next, err = r.OpenRoot(part)
		}
		r.Close()
		if err != nil {
			return nil, err
		}
		r = next
	}
	return r, nil
}

// copyTree copies from in src to to in dst, merging into a folder that is
// there and replacing files in it. Links in the source are copied as links,
// and what skip names, by its path below the item with slashes, is left out.
func copyTree(src *os.Root, from string, dst *os.Root, to, rel string, skip map[string]bool) error {
	if skip[rel] {
		return nil
	}
	info, err := src.Lstat(from)
	if err != nil {
		return err
	}
	switch {
	case info.Mode()&fs.ModeSymlink != 0 && runtime.GOOS != "windows":
		dest, err := src.Readlink(from)
		if err != nil {
			return err
		}
		if err := dst.Remove(to); err != nil && !errors.Is(err, fs.ErrNotExist) {
			return err
		}
		return dst.Symlink(dest, to)
	case link(src, from, info):
		return &fs.PathError{Op: "copy", Path: from, Err: plainpath.ErrLink}
	case info.IsDir():
		return copyDir(src, from, dst, to, rel, skip, info)
	// A cloud placeholder or deduplicated file on Windows is a reparse point
	// with data of its own, which reading fetches.
	case info.Mode().IsRegular(), runtime.GOOS == "windows" && info.Mode().Type() == fs.ModeIrregular:
		return copyFile(src, from, dst, to, info)
	}
	return &fs.PathError{Op: "copy", Path: from, Err: errors.ErrUnsupported}
}

func copyDir(src *os.Root, from string, dst *os.Root, to, rel string, skip map[string]bool, info fs.FileInfo) error {
	t, err := dst.Lstat(to)
	switch {
	case errors.Is(err, fs.ErrNotExist):
		err = dst.Mkdir(to, info.Mode().Perm())
	case err == nil && link(dst, to, t):
		err = &fs.PathError{Op: "copy", Path: to, Err: plainpath.ErrLink}
	case err == nil && !t.IsDir():
		if err = dst.Remove(to); err == nil {
			err = dst.Mkdir(to, info.Mode().Perm())
		}
	}
	if err != nil {
		return err
	}
	in, err := src.OpenRoot(from)
	if err != nil {
		return err
	}
	defer in.Close()
	out, err := dst.OpenRoot(to)
	if err != nil {
		return err
	}
	defer out.Close()
	entries, err := readDir(in)
	if err != nil {
		return err
	}
	for _, e := range entries {
		if err := copyTree(in, e.Name(), out, e.Name(), path.Join(rel, e.Name()), skip); err != nil {
			return err
		}
	}
	return dst.Chtimes(to, info.ModTime(), info.ModTime())
}

// copyFile writes a new file at to. Whatever was there goes first, so a link
// planted at to is replaced, never written through.
func copyFile(src *os.Root, from string, dst *os.Root, to string, info fs.FileInfo) error {
	in, err := src.OpenFile(from, os.O_RDONLY|noFollow, 0)
	if err != nil {
		return err
	}
	defer in.Close()
	if t, err := dst.Lstat(to); err == nil {
		if t.IsDir() && !link(dst, to, t) {
			err = dst.RemoveAll(to)
		} else {
			err = dst.Remove(to)
		}
		if err != nil {
			return err
		}
	}
	out, err := dst.OpenFile(to, os.O_CREATE|os.O_EXCL|os.O_WRONLY|noFollow, info.Mode().Perm())
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
	return dst.Chtimes(to, info.ModTime(), info.ModTime())
}

// removeTree removes what a move took along: all of name but what skip
// names, and the folders that still hold something of that stay.
func removeTree(r *os.Root, name, rel string, skip map[string]bool) error {
	if len(skip) == 0 {
		return r.RemoveAll(name)
	}
	if skip[rel] {
		return nil
	}
	info, err := r.Lstat(name)
	if err != nil {
		return err
	}
	if !info.IsDir() || link(r, name, info) {
		return r.Remove(name)
	}
	sub, err := r.OpenRoot(name)
	if err != nil {
		return err
	}
	entries, err := readDir(sub)
	if err == nil {
		for _, e := range entries {
			if err = removeTree(sub, e.Name(), path.Join(rel, e.Name()), skip); err != nil {
				break
			}
		}
	}
	left, _ := readDir(sub)
	sub.Close()
	if err != nil || len(left) > 0 {
		return err
	}
	return r.Remove(name)
}

// link reports whether name in r, which fi describes, is a link.
func link(r *os.Root, name string, fi fs.FileInfo) bool {
	return plainpath.Link(fi, filepath.Join(r.Name(), name))
}

func readDir(r *os.Root) ([]fs.DirEntry, error) {
	f, err := r.Open(".")
	if err != nil {
		return nil, err
	}
	defer f.Close()
	return f.ReadDir(-1)
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
