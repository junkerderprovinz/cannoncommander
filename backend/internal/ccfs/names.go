package ccfs

import (
	"os"
	"path/filepath"
	"strconv"
	"strings"
)

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
