package meta

import (
	"archive/tar"
	"archive/zip"
	"bufio"
	"compress/gzip"
	"context"
	"encoding/binary"
	"errors"
	"io"
	"os"
	"strconv"
	"strings"
)

// streamLimit is the largest compressed tar whose contents are counted by
// unpacking it. The other formats keep a table of contents and are cheap
// at any size.
const streamLimit = 1 << 30

var zipMethods = map[uint16]string{
	zip.Store: "Ohne", zip.Deflate: "Deflate", 9: "Deflate64", 12: "BZip2", 14: "LZMA", 93: "Zstandard", 95: "XZ", 98: "PPMd",
}

// methodByExt is what an archive uses when nothing better can be read.
var methodByExt = map[string]string{
	"zip": "Deflate", "7z": "LZMA2", "rar": "RAR", "tar": "Ohne", "gz": "Gzip", "tgz": "Gzip", "bz2": "BZip2",
	"xz": "XZ", "zst": "Zstandard", "lz4": "LZ4", "cab": "MSZIP",
}

func readArchive(ctx context.Context, s *Service, f File, rec Record) error {
	ext := extOf(f.Path)
	rec["method"] = methodByExt[ext]
	switch ext {
	case "zip":
		return readZip(f, rec)
	case "tar":
		return readTar(ctx, f.Real, nil, rec)
	case "gz", "tgz":
		if ext == "tgz" || strings.HasSuffix(strings.ToLower(f.Path), ".tar.gz") {
			if f.Size > streamLimit {
				return nil
			}
			return readTar(ctx, f.Real, gzip.NewReader, rec)
		}
		return readGzip(f, rec)
	}
	return readListing(ctx, s, f, rec)
}

func readZip(f File, rec Record) error {
	zr, err := zip.OpenReader(f.Real)
	if err != nil {
		return err
	}
	defer zr.Close()
	var files, size int64
	crypt := false
	method := ""
	for _, zf := range zr.File {
		if zf.Flags&1 != 0 {
			crypt = true
		}
		if zf.FileInfo().IsDir() {
			continue
		}
		files++
		size += int64(zf.UncompressedSize64)
		if method == "" {
			method = zipMethod(zf)
		}
	}
	rec["entries"], rec["unpacked"], rec["archcrypt"] = files, size, crypt
	if method != "" {
		rec["method"] = method
	}
	return nil
}

// zipMethod names the compression of one entry. Entries encrypted with AES
// carry the real method in their AES extra field.
func zipMethod(zf *zip.File) string {
	m := zf.Method
	if m == 99 {
		for x := zf.Extra; len(x) >= 4; {
			id, n := binary.LittleEndian.Uint16(x), int(binary.LittleEndian.Uint16(x[2:]))
			if len(x) < 4+n {
				break
			}
			if id == 0x9901 && n >= 7 {
				m = binary.LittleEndian.Uint16(x[4+5:])
			}
			x = x[4+n:]
		}
	}
	if name, ok := zipMethods[m]; ok {
		return name
	}
	return "Methode " + strconv.Itoa(int(m))
}

func readTar(ctx context.Context, p string, unpack func(io.Reader) (*gzip.Reader, error), rec Record) error {
	fh, err := os.Open(p)
	if err != nil {
		return err
	}
	defer fh.Close()
	// A plain tar skips the file bodies by seeking, since the file is a
	// Seeker. Through gzip every byte has to be unpacked.
	var r io.Reader = fh
	if unpack != nil {
		gz, err := unpack(bufio.NewReaderSize(fh, 1<<20))
		if err != nil {
			return err
		}
		defer gz.Close()
		r = gz
	}
	tr := tar.NewReader(r)
	var files, size int64
	for {
		if ctx.Err() != nil {
			return ctx.Err()
		}
		h, err := tr.Next()
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil {
			return err
		}
		if h.Typeflag == tar.TypeReg {
			files++
			size += h.Size
		}
	}
	rec["entries"], rec["unpacked"], rec["archcrypt"] = files, size, false
	return nil
}

// readGzip takes the size from the gzip trailer, which holds the unpacked
// length modulo 4 GiB. Less than half the packed size cannot be right, so
// such a number has wrapped around.
func readGzip(f File, rec Record) error {
	fh, err := os.Open(f.Real)
	if err != nil {
		return err
	}
	defer fh.Close()
	if f.Size < 18 {
		return nil
	}
	tail := make([]byte, 4)
	if _, err := fh.ReadAt(tail, f.Size-4); err != nil {
		return err
	}
	size := int64(binary.LittleEndian.Uint32(tail))
	for size < f.Size/2 {
		size += 1 << 32
	}
	rec["entries"], rec["unpacked"], rec["archcrypt"] = int64(1), size, false
	return nil
}

// readListing asks 7-Zip for the table of contents of the formats Go
// cannot read on its own, such as 7z and rar.
func readListing(ctx context.Context, s *Service, f File, rec Record) error {
	sz, ok := s.tools.find("7zz", "7z")
	if !ok {
		return errNoTool
	}
	// A wrong password makes 7-Zip fail instead of asking for one, which
	// is how an archive with encrypted headers shows itself.
	out, err := run(ctx, sz, "l", "-slt", "-sccUTF-8", "-pcannoncommander", "--", f.Real)
	if err != nil && strings.Contains(string(out)+err.Error(), "Wrong password") {
		rec["archcrypt"] = true
		return nil
	}
	if err != nil && len(out) == 0 {
		return err
	}
	parseListing(string(out), rec)
	return nil
}

func parseListing(out string, rec Record) {
	_, items, ok := strings.Cut(out, "\n----------\n")
	if !ok {
		return
	}
	var files, size int64
	crypt := false
	method := ""
	for _, block := range strings.Split(items, "\n\n") {
		props := map[string]string{}
		for _, line := range strings.Split(block, "\n") {
			if k, v, ok := strings.Cut(line, " = "); ok {
				props[strings.TrimSpace(k)] = strings.TrimSpace(v)
			}
		}
		if props["Path"] == "" {
			continue
		}
		if props["Encrypted"] == "+" {
			crypt = true
		}
		if props["Folder"] == "+" || strings.HasPrefix(props["Attributes"], "D") {
			continue
		}
		files++
		n, _ := strconv.ParseInt(props["Size"], 10, 64)
		size += n
		if method == "" {
			method = listedMethod(props["Method"])
		}
	}
	rec["entries"], rec["unpacked"], rec["archcrypt"] = files, size, crypt
	if method != "" {
		rec["method"] = method
	}
}

// listedMethod turns 7-Zip's method strings such as "LZMA2:24 BCJ" or
// "v5:m3:128M" into the name of the compression.
func listedMethod(m string) string {
	first, _, _ := strings.Cut(m, " ")
	name, _, _ := strings.Cut(first, ":")
	switch {
	case name == "":
		return ""
	case name == "Copy" || name == "Store":
		return "Ohne"
	case name == "v5":
		return "RAR5"
	case strings.HasPrefix(name, "v"):
		return "RAR"
	case name == "PPMD":
		return "PPMd"
	case name == "ZSTD":
		return "Zstandard"
	}
	return name
}
