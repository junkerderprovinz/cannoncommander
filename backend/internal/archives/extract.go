package archives

import (
	"errors"
	"io"
	"io/fs"
	"os"
	"path"
	"path/filepath"
	"slices"
	"strings"
	"time"

	"github.com/junkerderprovinz/cannoncommander/backend/internal/ccfs"
)

// What happens to a top level entry that exists at the target already.
const (
	ClashSkip    = "skip"
	ClashReplace = "replace"
	ClashBoth    = "both"
)

// ExtractOptions choose what is unpacked and how.
type ExtractOptions struct {
	// Clash is ClashSkip, ClashReplace or ClashBoth, the last by default.
	Clash string
	// Prefix unpacks only the folder of that name inside the archive, with
	// its own path left off.
	Prefix string
	// Only limits the unpacking to these names directly below Prefix.
	Only []string
	// Wrote is called after every chunk with its size. An error stops the
	// unpacking, which is how a job pauses and keeps to its speed.
	Wrote func(n int) error
	// Next is called before every entry with its target path.
	Next func(name string) error
}

// ExtractResult says what landed where.
type ExtractResult struct {
	// Top lists the names at the target that were written to, in order.
	Top     []string `json:"top"`
	Files   int64    `json:"files"`
	Dirs    int64    `json:"dirs"`
	Bytes   int64    `json:"bytes"`
	Skipped []string `json:"skipped,omitempty"`
}

type topChoice struct {
	name string
	skip bool
}

type extractor struct {
	a     Archive
	root  *os.Root
	dest  string
	o     ExtractOptions
	res   ExtractResult
	tops  map[string]topChoice
	dirs  []dirTime
	links map[string]string
}

type dirTime struct {
	name string
	t    time.Time
}

// Extract unpacks a into the folder dest. Every write goes through an
// os.Root on dest, so neither a name nor a symlink planted by an earlier
// entry can reach a file outside it. Links that point outside are skipped.
func Extract(a Archive, dest string, o ExtractOptions) (ExtractResult, error) {
	if o.Clash == "" {
		o.Clash = ClashBoth
	}
	root, err := os.OpenRoot(dest)
	if err != nil {
		return ExtractResult{}, err
	}
	defer root.Close()
	x := &extractor{a: a, root: root, dest: dest, o: o, tops: map[string]topChoice{}, links: map[string]string{}}
	err = a.Walk(x.entry)
	for _, d := range slices.Backward(x.dirs) {
		if !d.t.IsZero() {
			_ = root.Chtimes(d.name, d.t, d.t)
		}
	}
	return x.res, err
}

// target maps an entry name to its place below dest, or "" when the entry
// is not unpacked.
func (x *extractor) target(name string) (string, error) {
	rel := name
	if x.o.Prefix != "" {
		prefix := strings.Trim(x.o.Prefix, "/")
		if rel == prefix || !strings.HasPrefix(rel, prefix+"/") {
			return "", nil
		}
		rel = strings.TrimPrefix(rel, prefix+"/")
	}
	top, rest, _ := strings.Cut(rel, "/")
	if len(x.o.Only) > 0 && !slices.Contains(x.o.Only, top) {
		return "", nil
	}
	c, seen := x.tops[top]
	if !seen {
		c = topChoice{name: top}
		if _, err := x.root.Lstat(top); err == nil {
			switch x.o.Clash {
			case ClashSkip:
				c.skip = true
				x.res.Skipped = append(x.res.Skipped, top)
			case ClashReplace:
				if err := x.root.RemoveAll(top); err != nil {
					return "", err
				}
			default:
				c.name = ccfs.FreeName(x.dest, top)
			}
		}
		x.tops[top] = c
		if !c.skip {
			x.res.Top = append(x.res.Top, c.name)
		}
	}
	if c.skip {
		return "", nil
	}
	if rest == "" {
		return c.name, nil
	}
	return c.name + "/" + rest, nil
}

func (x *extractor) entry(e Entry, open Opener) error {
	p, err := x.target(e.Name)
	if err != nil || p == "" {
		return err
	}
	if x.o.Next != nil {
		if err := x.o.Next(p); err != nil {
			return err
		}
	}
	if parent := path.Dir(p); parent != "." {
		if err := x.root.MkdirAll(parent, 0o777); err != nil {
			return err
		}
	}
	switch {
	case e.Dir:
		if err := x.root.MkdirAll(p, 0o777); err != nil {
			return err
		}
		if perm := e.Mode.Perm(); perm != 0 {
			_ = x.root.Chmod(p, perm|0o700)
		}
		x.dirs = append(x.dirs, dirTime{p, e.ModTime})
		x.res.Dirs++
		return nil
	case e.Symlink():
		if e.Outside {
			x.res.Skipped = append(x.res.Skipped, p)
			return nil
		}
		if err := x.clear(p); err != nil {
			return err
		}
		if err := x.root.Symlink(e.Link, p); err != nil {
			return err
		}
		x.res.Files++
		return nil
	case e.HardLink != "":
		return x.hardLink(e, p)
	}
	if err := x.clear(p); err != nil {
		return err
	}
	rc, err := open()
	if err != nil {
		return err
	}
	defer rc.Close()
	if err := x.write(p, e, rc); err != nil {
		_ = x.root.Remove(p)
		return err
	}
	x.links[e.Name] = p
	x.res.Files++
	return nil
}

// clear removes what an earlier entry of the same archive left at p, so a
// file never writes through a symlink and a link never lands on a file.
func (x *extractor) clear(p string) error {
	info, err := x.root.Lstat(p)
	if errors.Is(err, fs.ErrNotExist) {
		return nil
	}
	if err != nil {
		return err
	}
	if info.IsDir() {
		return x.root.RemoveAll(p)
	}
	return x.root.Remove(p)
}

func (x *extractor) write(p string, e Entry, r io.Reader) error {
	perm := e.Mode.Perm()
	if perm == 0 {
		perm = 0o666
	}
	f, err := x.root.OpenFile(p, os.O_WRONLY|os.O_CREATE|os.O_EXCL, perm)
	if err != nil {
		return err
	}
	buf := make([]byte, 1<<20)
	for {
		n, rerr := r.Read(buf)
		if n > 0 {
			if _, err := f.Write(buf[:n]); err != nil {
				f.Close()
				return err
			}
			x.res.Bytes += int64(n)
			if x.o.Wrote != nil {
				if err := x.o.Wrote(n); err != nil {
					f.Close()
					return err
				}
			}
		}
		if errors.Is(rerr, io.EOF) {
			break
		}
		if rerr != nil {
			f.Close()
			return rerr
		}
	}
	if err := f.Close(); err != nil {
		return err
	}
	if !e.ModTime.IsZero() {
		_ = x.root.Chtimes(p, e.ModTime, e.ModTime)
	}
	return nil
}

// hardLink links p to a file unpacked earlier. A target that was not
// unpacked, because it was skipped or comes later, leaves the entry out.
func (x *extractor) hardLink(e Entry, p string) error {
	src, ok := x.links[e.HardLink]
	if !ok {
		x.res.Skipped = append(x.res.Skipped, p)
		return nil
	}
	if src == p {
		return nil
	}
	if err := x.clear(p); err != nil {
		return err
	}
	if err := x.root.Link(src, p); err != nil {
		in, oerr := x.root.Open(src)
		if oerr != nil {
			return err
		}
		defer in.Close()
		info, serr := in.Stat()
		if serr != nil {
			return serr
		}
		if werr := x.write(p, Entry{Mode: info.Mode(), ModTime: info.ModTime()}, in); werr != nil {
			return werr
		}
	}
	x.links[e.Name] = p
	x.res.Files++
	return nil
}

// Into returns the folder an archive unpacks into when every archive gets
// its own, as fotos for fotos.zip.
func Into(archive string) string {
	name := StripExt(filepath.Base(archive))
	if name == "" {
		return "archive"
	}
	return name
}
