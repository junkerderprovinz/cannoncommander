package archives

import (
	"errors"
	"fmt"
	"io"
	"io/fs"

	"github.com/bodgit/sevenzip"
)

type sevenArchive struct {
	r *sevenzip.ReadCloser
}

func open7z(file string, o Options) (Archive, error) {
	r, err := sevenzip.OpenReaderWithPassword(file, o.Password)
	if err != nil {
		if pw := sevenError(err); pw == ErrPassword {
			return nil, pw
		}
		return nil, fmt.Errorf("%w: %v", ErrUnsupported, err)
	}
	return &sevenArchive{r: r}, nil
}

func (a *sevenArchive) Close() error { return a.r.Close() }

func (a *sevenArchive) Walk(fn func(Entry, Opener) error) error {
	for _, f := range a.r.File {
		open := func() (io.ReadCloser, error) {
			rc, err := f.Open()
			if err != nil {
				return nil, sevenError(err)
			}
			return sevenErrors{rc}, nil
		}
		mode := f.Mode()
		link := ""
		if mode&fs.ModeSymlink != 0 {
			var err error
			if link, err = readLink(open); err != nil {
				return err
			}
		}
		e, err := entryFor(f.Name, mode, int64(f.UncompressedSize), f.Modified, link)
		if err != nil {
			return err
		}
		if err := fn(e, open); err != nil {
			return err
		}
	}
	return nil
}

type sevenErrors struct{ io.ReadCloser }

func (s sevenErrors) Read(p []byte) (int, error) {
	n, err := s.ReadCloser.Read(p)
	return n, sevenError(err)
}

// sevenError reports a read error inside an encrypted stream as a password
// problem, since a wrong key is by far the most likely cause.
func sevenError(err error) error {
	var re *sevenzip.ReadError
	if errors.As(err, &re) && re.Encrypted {
		return ErrPassword
	}
	return err
}
