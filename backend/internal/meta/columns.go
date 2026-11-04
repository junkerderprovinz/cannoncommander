package meta

import (
	"fmt"
	"math"
	"strconv"
	"strings"
)

// Record is what the readers found out about one file, keyed by the field
// names of the cache. Values are int64, float64, string or bool.
type Record map[string]any

type kind uint8

const (
	intField kind = iota
	realField
	textField
	boolField
)

// fields are the typed columns of meta_cache. Searches compare them as
// numbers or text, so each keeps its type in the database.
var fields = map[string]kind{
	"width": intField, "height": intField,
	"camera": textField, "make": textField, "lens": textField, "focal": textField, "aperture": textField,
	"exposure": textField, "iso": intField, "flash": textField, "taken": intField, "lat": realField, "lon": realField,
	"orientation": textField, "depth": textField, "dpi": textField,
	"duration": intField, "vcodec": textField, "acodec": textField, "framerate": textField, "vbitrate": intField,
	"aspect": textField, "hdr": textField, "audiotracks": textField, "subtitles": textField,
	"abitrate": textField, "samplerate": textField, "channels": textField,
	"artist": textField, "album": textField, "title": textField, "albumartist": textField, "track": intField,
	"year": intField, "genre": textField, "composer": textField,
	"pages": intField, "author": textField, "lastauthor": textField, "doctitle": textField, "subject": textField,
	"app": textField, "encrypted": boolField, "words": intField, "lines": intField, "encoding": textField,
	"unpacked": intField, "entries": intField, "method": textField, "archcrypt": boolField,
	"fs": textField, "volume": textField, "bootable": boolField,
	"version": textField, "vendor": textField, "arch": textField, "signed": boolField,
	"sha256": textField, "crc32": textField, "md5": textField, "sha1": textField,
}

// derived columns are worked out from stored fields when they are asked
// for, so a better place table or a rename needs no new reading.
var derived = map[string][]string{
	"dimensions": {"width", "height"},
	"ratio":      {"unpacked"},
	"place":      {"lat", "lon"},
}

// live columns come from the file system or the name every time, because
// they change without the file changing.
var live = map[string]bool{"accessed": true, "statchange": true, "disk": true, "season": true, "episode": true}

// Known reports whether fs/meta can fill the column.
func Known(col string) bool {
	_, stored := fields[col]
	_, made := derived[col]
	return stored || made || live[col]
}

// needs returns the stored fields behind a column.
func needs(col string) []string {
	if f, ok := derived[col]; ok {
		return f
	}
	if _, ok := fields[col]; ok {
		return []string{col}
	}
	return nil
}

// decimal writes a number the German way, with at most prec decimals and
// no trailing zeros.
func decimal(v float64, prec int) string {
	s := strconv.FormatFloat(v, 'f', prec, 64)
	if strings.Contains(s, ".") {
		s = strings.TrimRight(strings.TrimRight(s, "0"), ".")
	}
	return strings.Replace(s, ".", ",", 1)
}

func dimensions(rec Record) string {
	w, _ := rec["width"].(int64)
	h, _ := rec["height"].(int64)
	if w <= 0 || h <= 0 {
		return ""
	}
	return fmt.Sprintf("%d × %d", w, h)
}

// ratio is how much smaller the archive is than its contents.
func ratio(rec Record, size int64) string {
	u, _ := rec["unpacked"].(int64)
	if u <= 0 {
		return ""
	}
	return fmt.Sprintf("%d %%", max(0, int(math.Round(100-100*float64(size)/float64(u)))))
}
