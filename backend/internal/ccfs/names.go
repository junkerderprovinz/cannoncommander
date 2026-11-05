package ccfs

import (
	"os"
	"path/filepath"
	"strconv"
	"strings"
)

// BinName is the trash folder of a share, PartSuffix marks a file that is still
// being written.
const (
	BinName    = ".cc-trash"
	PartSuffix = ".cc-part"
)

// Reserved reports the names the app keeps for itself. Nobody gets at them
// through a path, because a bin holds other people's files and a part file is
// written to without another look at its rights.
func Reserved(name string) bool {
	return name == BinName || strings.HasPrefix(name, ".") && strings.HasSuffix(name, PartSuffix)
}

func reservedPath(p string) bool {
	if !strings.Contains(p, ".cc-") {
		return false
	}
	outer, _, _ := strings.Cut(p, ArchiveSep)
	for seg := range strings.SplitSeq(outer, "/") {
		if Reserved(seg) {
			return true
		}
	}
	return false
}

// FreeName returns name, or when dir already holds an entry by that name
// the first free "name (n).ext", as the prototype names a second copy.
func FreeName(dir, name string) string {
	if !exists(filepath.Join(dir, name)) {
		return name
	}
	base, ext := name, ""
	if dot := strings.LastIndex(name, "."); dot > 0 {
		base, ext = name[:dot], name[dot:]
	}
	for n := 1; ; n++ {
		c := base + " (" + strconv.Itoa(n) + ")" + ext
		if !exists(filepath.Join(dir, c)) {
			return c
		}
	}
}

func exists(p string) bool {
	_, err := os.Lstat(p)
	return err == nil
}

// ValidName reports whether name can be one entry in a folder.
func ValidName(name string) bool {
	return name != "" && name != "." && name != ".." && len(name) <= 255 &&
		!strings.ContainsAny(name, "/\x00") && !strings.Contains(name, ArchiveSep)
}

// OpenRegular opens a file for reading if it is a plain file, for walks over
// what the index knows, where a link could be anything by now.
func OpenRegular(name string) (*os.File, error) {
	f, err := os.OpenFile(name, os.O_RDONLY|noFollow, 0)
	if err != nil {
		return nil, err
	}
	info, err := f.Stat()
	if err == nil && !info.Mode().IsRegular() {
		err = ErrEscape
	}
	if err != nil {
		f.Close()
		return nil, err
	}
	return f, nil
}

// OpenPart opens a part file for writing. A part file gets written without a
// new check of the path, so a link in its place would pass the bytes on to
// whatever it points at.
func OpenPart(name string, flag int) (*os.File, error) {
	f, err := os.OpenFile(name, flag|noFollow, 0o666)
	if err != nil {
		return nil, err
	}
	info, err := f.Stat()
	if err == nil && (!info.Mode().IsRegular() || !singleLink(info)) {
		err = ErrEscape
	}
	if err != nil {
		f.Close()
		return nil, err
	}
	return f, nil
}
