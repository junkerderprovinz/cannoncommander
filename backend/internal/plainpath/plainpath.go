// Package plainpath tells plain files and folders from links, junctions and
// other reparse points, for code that deletes, moves or writes with more
// rights than whoever could have planted one.
package plainpath

import (
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
)

// ErrLink is a link or reparse point where a plain entry belongs.
var ErrLink = errors.New("a link or reparse point is in the way")

// Check refuses p when it is a link.
func Check(p string) error {
	fi, err := os.Lstat(p)
	if err != nil {
		return err
	}
	if Link(fi) {
		return &fs.PathError{Op: "check", Path: p, Err: ErrLink}
	}
	return nil
}

// Path refuses p when it or any folder above it is a link. The parts that do
// not exist yet pass, since nothing can be planted there.
func Path(p string) error {
	p = filepath.Clean(p)
	vol := filepath.VolumeName(p)
	cur := vol + string(filepath.Separator)
	for _, part := range strings.Split(strings.TrimPrefix(p[len(vol):], string(filepath.Separator)), string(filepath.Separator)) {
		if part == "" {
			continue
		}
		cur = filepath.Join(cur, part)
		err := Check(cur)
		if errors.Is(err, fs.ErrNotExist) {
			return nil
		}
		if err != nil {
			return err
		}
	}
	return nil
}
