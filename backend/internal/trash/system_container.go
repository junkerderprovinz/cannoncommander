//go:build !desktop

package trash

import (
	"errors"
	"io/fs"
	"os"
)

// systemTrash is nil: the container keeps every deleted file in the app's own
// trash.
func systemTrash(*Store) systemBin { return nil }

// plainBin refuses a bin folder that is a link. The container runs on Linux,
// where a symlink is the only kind.
func plainBin(dir string) error {
	fi, err := os.Lstat(dir)
	if err != nil {
		return err
	}
	if fi.Mode()&fs.ModeSymlink != 0 {
		return &fs.PathError{Op: "check", Path: dir, Err: errors.New("the bin folder is a link")}
	}
	return nil
}
