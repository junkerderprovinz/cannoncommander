package archives

import (
	"errors"
	"fmt"
	"io"
	"io/fs"

	"github.com/nwaples/rardecode/v2"
)

// Redirection types of RAR 5.
const (
	rarUnixLink    = 1
	rarWindowsLink = 2
	rarJunction    = 3
	rarHardLink    = 4
	rarFileCopy    = 5
)

type rarArchive struct {
	r *rardecode.ReadCloser
}

func openRar(file string, o Options) (Archive, error) {
	var opts []rardecode.Option
	if o.Password != "" {
		opts = append(opts, rardecode.Password(o.Password))
	}
	r, err := rardecode.OpenReader(file, opts...)
	if err != nil {
		if rarError(err) == ErrPassword {
			return nil, ErrPassword
		}
		return nil, fmt.Errorf("%w: %v", ErrUnsupported, err)
	}
	return &rarArchive{r: r}, nil
}

func (a *rarArchive) Close() error { return a.r.Close() }

func (a *rarArchive) Walk(fn func(Entry, Opener) error) error {
	for {
		h, err := a.r.Next()
		if errors.Is(err, io.EOF) {
			return nil
		}
		if err != nil {
			return rarError(err)
		}
		mode := h.Mode()
		link := ""
		switch h.LinkType {
		case rarUnixLink, rarWindowsLink:
			mode, link = fs.ModeSymlink|0o777, h.LinkTarget
		case rarJunction:
			// A junction always points at an absolute path on the machine
			// that made the archive.
			continue
		}
		e, err := entryFor(h.Name, mode, h.UnPackedSize, h.ModificationTime, link)
		if err != nil {
			return err
		}
		if h.LinkType == rarHardLink || h.LinkType == rarFileCopy {
			if e.HardLink, err = CleanName(h.LinkTarget); err != nil {
				return err
			}
			e.Size = 0
		}
		e.Encrypted = h.Encrypted
		open := func() (io.ReadCloser, error) { return io.NopCloser(rarReader{a.r}), nil }
		if err := fn(e, open); err != nil {
			return err
		}
	}
}

type rarReader struct{ r io.Reader }

func (r rarReader) Read(p []byte) (int, error) {
	n, err := r.r.Read(p)
	return n, rarError(err)
}

func rarError(err error) error {
	if errors.Is(err, rardecode.ErrArchiveEncrypted) || errors.Is(err, rardecode.ErrArchivedFileEncrypted) || errors.Is(err, rardecode.ErrBadPassword) {
		return ErrPassword
	}
	return err
}
