package archives

import (
	"archive/tar"
	"archive/zip"
	"bufio"
	"compress/flate"
	"context"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"

	"github.com/klauspost/compress/gzip"
	"github.com/klauspost/compress/zstd"
	yzip "github.com/yeka/zip"
)

// PackFormats are the formats Pack can write, given the programs it needs.
var PackFormats = []Format{Zip, TarGz, TarZst, SevenZ, Rar}

// PackOptions describe the archive Pack writes.
type PackOptions struct {
	Format Format
	// Level runs from 0, which stores without compression, to 9.
	Level    int
	Password string
	// Total is the size of everything packed, for the progress of the
	// external programs, which report a percentage.
	Total int64
	// Wrote is called with the bytes read from the sources since the last
	// call. An error stops the packing.
	Wrote func(n int) error
	// Next is called before every entry with its name in the archive.
	Next func(name string) error
}

// ErrNoProgram means a format needs a program that is not installed.
var ErrNoProgram = errors.New("the program for this format is not installed")

// SevenZip and RarProgram name the programs for 7z and rar. rar is not
// part of the image, its licence does not allow that, so an admin who
// wants rar archives puts the program in place and sets CC_RAR.
var (
	SevenZip   = envOr("CC_7ZZ", "7zz")
	RarProgram = envOr("CC_RAR", "rar")
)

func envOr(key, def string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return def
}

// CanPack reports whether f can be written on this server.
func CanPack(f Format) bool {
	switch f {
	case Zip, TarGz, TarZst:
		return true
	case SevenZ:
		_, err := exec.LookPath(SevenZip)
		return err == nil
	case Rar:
		_, err := exec.LookPath(RarProgram)
		return err == nil
	}
	return false
}

// Pack writes the entries names, which lie in the folder base, into the
// archive file out, an absolute path. Folders go in with everything below
// them, symlinks as links.
func Pack(ctx context.Context, out, base string, names []string, o PackOptions) error {
	if o.Level < 0 || o.Level > 9 {
		return fmt.Errorf("level %d is not between 0 and 9", o.Level)
	}
	switch o.Format {
	case SevenZ:
		return packTool(ctx, SevenZip, sevenArgs(out, names, o), base, o)
	case Rar:
		return packTool(ctx, RarProgram, rarArgs(out, names, o), base, o)
	case Zip, TarGz, TarZst:
	default:
		return fmt.Errorf("%w: %s", ErrUnsupported, o.Format)
	}
	f, err := os.OpenFile(out, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o666)
	if err != nil {
		return err
	}
	w := bufio.NewWriterSize(f, 1<<20)
	switch o.Format {
	case Zip:
		err = packZip(ctx, w, base, names, o)
	default:
		err = packTar(ctx, w, base, names, o)
	}
	if err == nil {
		err = w.Flush()
	}
	if cerr := f.Close(); err == nil {
		err = cerr
	}
	if err != nil {
		os.Remove(out)
	}
	return err
}

// walk visits every entry below base/names in a stable order and hands
// fn its name in the archive.
func walk(ctx context.Context, base string, names []string, fn func(rel, full string, d fs.DirEntry) error) error {
	for _, name := range names {
		root := filepath.Join(base, name)
		err := filepath.WalkDir(root, func(p string, d fs.DirEntry, err error) error {
			if err != nil {
				return err
			}
			if ctx.Err() != nil {
				return ctx.Err()
			}
			rel, err := filepath.Rel(base, p)
			if err != nil {
				return err
			}
			return fn(filepath.ToSlash(rel), p, d)
		})
		if err != nil {
			return err
		}
	}
	return nil
}

func (o PackOptions) next(name string) error {
	if o.Next == nil {
		return nil
	}
	return o.Next(name)
}

// copyFrom copies the file at p into w and reports the progress.
func (o PackOptions) copyFrom(w io.Writer, p string) error {
	f, err := os.Open(p)
	if err != nil {
		return err
	}
	defer f.Close()
	buf := make([]byte, 1<<20)
	for {
		n, rerr := f.Read(buf)
		if n > 0 {
			if _, err := w.Write(buf[:n]); err != nil {
				return err
			}
			if o.Wrote != nil {
				if err := o.Wrote(n); err != nil {
					return err
				}
			}
		}
		if errors.Is(rerr, io.EOF) {
			return nil
		}
		if rerr != nil {
			return rerr
		}
	}
}

func packZip(ctx context.Context, w io.Writer, base string, names []string, o PackOptions) error {
	if o.Password != "" {
		return packZipAES(ctx, w, base, names, o)
	}
	zw := zip.NewWriter(w)
	zw.RegisterCompressor(zip.Deflate, func(out io.Writer) (io.WriteCloser, error) {
		return flate.NewWriter(out, o.Level)
	})
	err := walk(ctx, base, names, func(rel, full string, d fs.DirEntry) error {
		if err := o.next(rel); err != nil {
			return err
		}
		info, err := d.Info()
		if err != nil {
			return err
		}
		h, err := zip.FileInfoHeader(info)
		if err != nil {
			return err
		}
		h.Name = rel
		switch {
		case info.IsDir():
			h.Name += "/"
			h.Method = zip.Store
			_, err = zw.CreateHeader(h)
			return err
		case info.Mode()&fs.ModeSymlink != 0:
			target, err := os.Readlink(full)
			if err != nil {
				return err
			}
			h.Method = zip.Store
			fw, err := zw.CreateHeader(h)
			if err != nil {
				return err
			}
			_, err = io.WriteString(fw, target)
			return err
		case !info.Mode().IsRegular():
			return nil
		}
		h.Method = zip.Deflate
		if o.Level == 0 {
			h.Method = zip.Store
		}
		fw, err := zw.CreateHeader(h)
		if err != nil {
			return err
		}
		return o.copyFrom(fw, full)
	})
	if err != nil {
		return err
	}
	return zw.Close()
}

// packZipAES writes a zip with every file encrypted with AES-256, which
// 7-Zip, WinRAR and the macOS and Windows archive tools open. Links go in
// as the files they point to, since an encrypted link would be unreadable
// anywhere else.
func packZipAES(ctx context.Context, w io.Writer, base string, names []string, o PackOptions) error {
	zw := yzip.NewWriter(w)
	err := walk(ctx, base, names, func(rel, full string, d fs.DirEntry) error {
		if err := o.next(rel); err != nil {
			return err
		}
		info, err := os.Stat(full)
		if err != nil {
			return err
		}
		if info.IsDir() {
			h := &yzip.FileHeader{Name: rel + "/", Method: yzip.Store}
			h.SetModTime(info.ModTime())
			h.SetMode(info.Mode())
			_, err := zw.CreateHeader(h)
			return err
		}
		if !info.Mode().IsRegular() {
			return nil
		}
		h := &yzip.FileHeader{Name: rel, Method: yzip.Deflate}
		if o.Level == 0 {
			h.Method = yzip.Store
		}
		h.SetModTime(info.ModTime())
		h.SetMode(info.Mode())
		h.SetPassword(o.Password)
		h.SetEncryptionMethod(yzip.AES256Encryption)
		fw, err := zw.CreateHeader(h)
		if err != nil {
			return err
		}
		return o.copyFrom(fw, full)
	})
	if err != nil {
		return err
	}
	return zw.Close()
}

func packTar(ctx context.Context, w io.Writer, base string, names []string, o PackOptions) error {
	var cw io.WriteCloser
	var err error
	if o.Format == TarGz {
		level := o.Level
		if level == 0 {
			level = gzip.NoCompression
		}
		cw, err = gzip.NewWriterLevel(w, level)
	} else {
		cw, err = zstd.NewWriter(w, zstd.WithEncoderLevel(zstd.EncoderLevelFromZstd(max(1, o.Level*2))))
	}
	if err != nil {
		return err
	}
	tw := tar.NewWriter(cw)
	err = walk(ctx, base, names, func(rel, full string, d fs.DirEntry) error {
		if err := o.next(rel); err != nil {
			return err
		}
		info, err := d.Info()
		if err != nil {
			return err
		}
		link := ""
		if info.Mode()&fs.ModeSymlink != 0 {
			if link, err = os.Readlink(full); err != nil {
				return err
			}
		} else if !info.IsDir() && !info.Mode().IsRegular() {
			return nil
		}
		h, err := tar.FileInfoHeader(info, link)
		if err != nil {
			return err
		}
		h.Name = rel
		if info.IsDir() {
			h.Name += "/"
		}
		h.Format = tar.FormatPAX
		if err := tw.WriteHeader(h); err != nil {
			return err
		}
		if info.Mode().IsRegular() {
			return o.copyFrom(tw, full)
		}
		return nil
	})
	if err != nil {
		return err
	}
	if err := tw.Close(); err != nil {
		return err
	}
	return cw.Close()
}

func sevenArgs(out string, names []string, o PackOptions) []string {
	args := []string{"a", "-t7z", "-mx=" + strconv.Itoa(o.Level), "-snl", "-bsp1", "-bso0", "-y"}
	if o.Password != "" {
		// The password shows in the process list of the container, which
		// only its own processes can read.
		args = append(args, "-p"+o.Password, "-mhe=on")
	}
	return append(append(args, "--", out), names...)
}

func rarArgs(out string, names []string, o PackOptions) []string {
	args := []string{"a", "-m" + strconv.Itoa(o.Level*5/9), "-r", "-ol", "-y", "-idcd"}
	if o.Password != "" {
		args = append(args, "-hp"+o.Password)
	}
	return append(append(args, "--", out), names...)
}

var percent = regexp.MustCompile(`(\d{1,3})%`)

// packTool runs an external packer in base and turns the percentages it
// prints into progress.
func packTool(ctx context.Context, program string, args []string, base string, o PackOptions) error {
	if _, err := exec.LookPath(program); err != nil {
		return ErrNoProgram
	}
	out := args[len(args)-1]
	for i, a := range args {
		if a == "--" {
			out = args[i+1]
			break
		}
	}
	cmd := exec.CommandContext(ctx, program, args...)
	cmd.Dir = base
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return err
	}
	var stderr strings.Builder
	cmd.Stderr = &stderr
	if err := cmd.Start(); err != nil {
		return err
	}
	var done int64
	sc := bufio.NewScanner(stdout)
	sc.Split(splitProgress)
	var werr error
	for sc.Scan() {
		m := percent.FindStringSubmatch(sc.Text())
		if m == nil || o.Wrote == nil || werr != nil {
			continue
		}
		pct, _ := strconv.ParseInt(m[1], 10, 64)
		if now := o.Total * min(pct, 100) / 100; now > done {
			werr = o.Wrote(int(now - done))
			done = now
			if werr != nil {
				_ = cmd.Process.Kill()
			}
		}
	}
	err = cmd.Wait()
	if werr != nil {
		err = werr
	} else if err != nil && ctx.Err() == nil {
		err = fmt.Errorf("%s failed: %s", filepath.Base(program), lastLine(stderr.String()))
	}
	if err != nil {
		os.Remove(out)
	}
	return err
}

// splitProgress splits on newlines and on the backspaces and carriage
// returns the packers use to redraw their percentage.
func splitProgress(data []byte, atEOF bool) (int, []byte, error) {
	for i, b := range data {
		if b == '\n' || b == '\r' || b == '\b' {
			return i + 1, data[:i], nil
		}
	}
	if atEOF && len(data) > 0 {
		return len(data), data, nil
	}
	return 0, nil, nil
}

func lastLine(s string) string {
	lines := strings.Split(strings.TrimSpace(s), "\n")
	return strings.TrimSpace(lines[len(lines)-1])
}
