package elevate

import (
	"errors"
	"os"

	"golang.org/x/sys/unix"
)

// renameAt moves from in src to to in dst relative to the two folders'
// descriptors, so no path is looked up again, and never over something that
// appeared at to meanwhile.
func renameAt(src *os.Root, from string, dst *os.Root, to string) error {
	sd, err := src.Open(".")
	if err != nil {
		return err
	}
	defer sd.Close()
	dd, err := dst.Open(".")
	if err != nil {
		return err
	}
	defer dd.Close()
	err = unix.Renameat2(int(sd.Fd()), from, int(dd.Fd()), to, unix.RENAME_NOREPLACE)
	if errors.Is(err, unix.EINVAL) || errors.Is(err, unix.ENOSYS) {
		// Some file systems cannot refuse to replace; nothing was at to a
		// moment ago.
		err = unix.Renameat(int(sd.Fd()), from, int(dd.Fd()), to)
	}
	if err != nil {
		return &os.LinkError{Op: "rename", Old: from, New: to, Err: err}
	}
	return nil
}

func crossVolume(err error) bool { return errors.Is(err, unix.EXDEV) }
