package archives

import (
	"archive/tar"
	"bufio"
	"compress/bzip2"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"

	"github.com/klauspost/compress/gzip"
	"github.com/klauspost/compress/zstd"
	"github.com/ulikunitz/xz"
)

type tarArchive struct {
	f     *os.File
	r     io.Reader
	close func()
}

func openTar(file string, f Format) (Archive, error) {
	fh, err := os.Open(file)
	if err != nil {
		return nil, err
	}
	a := &tarArchive{f: fh, close: func() {}}
	src := bufio.NewReaderSize(fh, 1<<20)
	switch f {
	case Tar:
		a.r = src
	case TarGz:
		zr, err := gzip.NewReader(src)
		if err != nil {
			fh.Close()
			return nil, fmt.Errorf("%w: %v", ErrUnsupported, err)
		}
		a.r = zr
	case TarZst:
		zr, err := zstd.NewReader(src, zstd.WithDecoderMaxWindow(1<<31))
		if err != nil {
			fh.Close()
			return nil, err
		}
		a.r, a.close = zr, zr.Close
	case TarXz:
		zr, err := xz.NewReader(src)
		if err != nil {
			fh.Close()
			return nil, fmt.Errorf("%w: %v", ErrUnsupported, err)
		}
		a.r = zr
	case TarBz2:
		a.r = bzip2.NewReader(src)
	}
	return a, nil
}

func (a *tarArchive) Close() error {
	a.close()
	return a.f.Close()
}

func (a *tarArchive) Walk(fn func(Entry, Opener) error) error {
	tr := tar.NewReader(a.r)
	for {
		h, err := tr.Next()
		if errors.Is(err, io.EOF) {
			return nil
		}
		if err != nil {
			return err
		}
		var mode fs.FileMode
		switch h.Typeflag {
		case tar.TypeReg, tar.TypeLink:
			mode = fs.FileMode(h.Mode).Perm()
		case tar.TypeDir:
			mode = fs.ModeDir | fs.FileMode(h.Mode).Perm()
		case tar.TypeSymlink:
			mode = fs.ModeSymlink | 0o777
		default:
			// Devices, pipes and global headers are nothing a file manager
			// unpacks.
			continue
		}
		e, err := entryFor(h.Name, mode, h.Size, h.ModTime, h.Linkname)
		if err != nil {
			return err
		}
		if h.Typeflag == tar.TypeLink {
			if e.HardLink, err = CleanName(h.Linkname); err != nil {
				return err
			}
			e.Size = 0
		}
		open := func() (io.ReadCloser, error) { return io.NopCloser(tr), nil }
		if err := fn(e, open); err != nil {
			return err
		}
	}
}
