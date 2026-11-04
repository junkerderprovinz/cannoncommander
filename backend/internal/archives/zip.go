package archives

import (
	"archive/zip"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"

	yzip "github.com/yeka/zip"

	"github.com/junkerderprovinz/cannoncommander/backend/internal/archiveencoding"
)

// maxLink caps the content read as a link target.
const maxLink = 4096

type zipArchive struct {
	f        *os.File
	r        *zip.Reader
	names    []string
	password string
	// crypt reads the encrypted entries. The standard library cannot, and
	// both readers list the central directory in the same order.
	crypt *yzip.Reader
}

func openZip(file string, o Options) (Archive, error) {
	f, err := os.Open(file)
	if err != nil {
		return nil, err
	}
	st, err := f.Stat()
	if err != nil {
		f.Close()
		return nil, err
	}
	r, err := zip.NewReader(f, st.Size())
	if err != nil {
		f.Close()
		return nil, fmt.Errorf("%w: %v", ErrUnsupported, err)
	}
	names, err := archiveencoding.DecodeNames(r.File, o.Charset)
	if errors.Is(err, archiveencoding.ErrEncoding) && o.Charset == "" {
		names, err = archiveencoding.DecodeNames(r.File, "cp437")
	}
	if err != nil {
		f.Close()
		return nil, err
	}
	return &zipArchive{f: f, r: r, names: names, password: o.Password}, nil
}

func (z *zipArchive) Close() error { return z.f.Close() }

func (z *zipArchive) Walk(fn func(Entry, Opener) error) error {
	for i, zf := range z.r.File {
		encrypted := zf.Flags&1 != 0
		open := func() (io.ReadCloser, error) { return z.open(i, zf, encrypted) }
		mode := zf.Mode()
		link := ""
		if mode&fs.ModeSymlink != 0 {
			var err error
			if link, err = readLink(open); err != nil {
				return err
			}
		}
		e, err := entryFor(z.names[i], mode, int64(zf.UncompressedSize64), zf.Modified, link)
		if err != nil {
			return err
		}
		e.Encrypted = encrypted
		if err := fn(e, open); err != nil {
			return err
		}
	}
	return nil
}

func (z *zipArchive) open(i int, zf *zip.File, encrypted bool) (io.ReadCloser, error) {
	if !encrypted {
		return zf.Open()
	}
	if z.password == "" {
		return nil, ErrPassword
	}
	if z.crypt == nil {
		st, err := z.f.Stat()
		if err != nil {
			return nil, err
		}
		if z.crypt, err = yzip.NewReader(z.f, st.Size()); err != nil {
			return nil, err
		}
	}
	if i >= len(z.crypt.File) {
		return nil, ErrUnsupported
	}
	yf := z.crypt.File[i]
	yf.SetPassword(z.password)
	rc, err := yf.Open()
	if err != nil {
		return nil, zipError(err)
	}
	return zipErrors{rc}, nil
}

// zipErrors turns the decryption errors of a wrong password into
// ErrPassword, which is what they mean to the user.
type zipErrors struct{ io.ReadCloser }

func (z zipErrors) Read(p []byte) (int, error) {
	n, err := z.ReadCloser.Read(p)
	return n, zipError(err)
}

func zipError(err error) error {
	if errors.Is(err, yzip.ErrPassword) || errors.Is(err, yzip.ErrAuthentication) || errors.Is(err, yzip.ErrDecryption) {
		return ErrPassword
	}
	return err
}

func readLink(open Opener) (string, error) {
	rc, err := open()
	if err != nil {
		return "", err
	}
	defer rc.Close()
	b, err := io.ReadAll(io.LimitReader(rc, maxLink))
	return string(b), err
}
