//go:build !linux && !windows

package elevate

import (
	"errors"
	"os"
)

var errNoRename = errors.New("no rename relative to folders here")

// renameAt is left to copy and delete on systems the helper does not run on.
func renameAt(*os.Root, string, *os.Root, string) error { return errNoRename }

func crossVolume(err error) bool { return errors.Is(err, errNoRename) }
