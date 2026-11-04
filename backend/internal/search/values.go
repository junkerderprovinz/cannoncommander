package search

import (
	"fmt"
	"math"
	"os"
	"path"
	"strconv"
	"strings"
	"time"

	"github.com/junkerderprovinz/cannoncommander/backend/internal/fsindex"
	"github.com/junkerderprovinz/cannoncommander/backend/internal/meta"
)

// row is one entry while the search looks at it. What costs a disk access
// is read on first use only.
type row struct {
	fsindex.Entry
	// At is the folder the entry was found in.
	At     string
	meta   meta.Values
	tagged func(path, tag string) bool

	stated  bool
	st      statInfo
	counted bool
	items   int
	// line is where a content search found its text.
	line int
}

func (r *row) stat() statInfo {
	if !r.stated {
		r.stated, r.st = true, statOf(r.Real)
	}
	return r.st
}

func (r *row) mode() uint32 {
	if r.Mode != 0 {
		return r.Mode
	}
	return r.stat().mode
}

func (r *row) kind() string { return KindOf(r.Name, r.Dir) }

func (r *row) metaNum(col string) (float64, bool) {
	switch v := r.meta[col].(type) {
	case int64:
		return float64(v), true
	case float64:
		return v, true
	}
	return 0, false
}

func (r *row) metaTime(col string) (time.Time, bool) {
	if n, ok := r.metaNum(col); ok {
		return time.UnixMilli(int64(n)), true
	}
	return time.Time{}, false
}

// column is one column of the list as a search field.
type column struct {
	// text is what the column shows, which plain and = tests compare.
	text func(r *row) string
	// num is what < and > compare, where it is not the number the text
	// starts with.
	num func(r *row) (float64, bool)
	// at makes the field compare like a date, with ages such as 7d.
	at func(r *row) (time.Time, bool)
	// unit reads the wanted value of fields that compare numbers only,
	// such as sizes and durations.
	unit func(string) float64
	meta string
}

func (c column) test(op, v string, now time.Time) test {
	switch {
	case c.at != nil:
		w := when(v)
		if !w.ok {
			return func(*row) bool { return false }
		}
		// An age turns the comparison round: geändert:<30d is less than 30
		// days ago.
		if w.ok && w.at.IsZero() {
			if op == "" {
				op = "<="
			}
			return func(r *row) bool {
				t, ok := c.at(r)
				return ok && compare(float64(now.Sub(t)), op, float64(w.age))
			}
		}
		if op == "" || op == "=" {
			return func(r *row) bool {
				t, ok := c.at(r)
				return ok && !t.Before(w.at) && t.Before(w.at.Add(w.span))
			}
		}
		return func(r *row) bool {
			t, ok := c.at(r)
			return ok && compare(float64(t.UnixMilli()), op, float64(w.at.UnixMilli()))
		}
	case c.unit != nil:
		want := c.unit(v)
		if math.IsNaN(want) {
			return func(*row) bool { return false }
		}
		if op == "" {
			op = "="
		}
		return func(r *row) bool {
			n, ok := c.num(r)
			return ok && n >= 0 && compare(n, op, want)
		}
	}
	low := strings.ToLower(v)
	switch op {
	case "<", ">", "<=", ">=":
		want := number(v)
		return func(r *row) bool {
			var n float64
			ok := false
			if c.num != nil {
				n, ok = c.num(r)
			} else {
				n = number(c.text(r))
				ok = !math.IsNaN(n)
			}
			return ok && n >= 0 && compare(n, op, want)
		}
	case "=":
		return func(r *row) bool { return strings.ToLower(c.text(r)) == low }
	case "!=":
		return func(r *row) bool { return !strings.Contains(strings.ToLower(c.text(r)), low) }
	}
	return func(r *row) bool { return strings.Contains(strings.ToLower(c.text(r)), low) }
}

var typeNames = map[string]string{
	"img": "Bild", "video": "Video", "audio": "Audio", "arch": "Archiv", "disk": "Abbild", "pdf": "PDF-Dokument",
	"office": "Office-Dokument", "text": "Textdatei", "code": "Quelltext", "file": "Datei",
}

var kindNames = map[string]string{
	"img": "Bild", "video": "Video", "audio": "Musik", "arch": "Archiv", "disk": "Abbild", "pdf": "Dokument",
	"office": "Dokument", "text": "Text", "code": "Quelltext",
}

var mimes = map[string]string{
	"jpg": "image/jpeg", "jpeg": "image/jpeg", "png": "image/png", "webp": "image/webp", "heic": "image/heic", "gif": "image/gif",
	"mkv": "video/x-matroska", "mp4": "video/mp4", "webm": "video/webm", "mov": "video/quicktime",
	"mp3": "audio/mpeg", "flac": "audio/flac", "ogg": "audio/ogg", "wav": "audio/wav",
	"zip": "application/zip", "7z": "application/x-7z-compressed", "rar": "application/vnd.rar", "tar": "application/x-tar", "gz": "application/gzip", "zst": "application/zstd",
	"iso": "application/x-iso9660-image", "pdf": "application/pdf", "odt": "application/vnd.oasis.opendocument.text",
	"docx": "application/vnd.openxmlformats-officedocument.wordprocessingml.document", "xlsx": "application/vnd.openxmlformats-officedocument.spreadsheetml.sheet",
	"md": "text/markdown", "txt": "text/plain", "log": "text/plain", "json": "application/json", "xml": "application/xml", "yml": "application/yaml", "yaml": "application/yaml",
	"sh": "application/x-sh", "js": "text/javascript", "ts": "text/typescript", "go": "text/x-go",
}

// How fs/meta values read in the list, which text tests compare with.
const (
	asText = iota
	asSize
	asDate
	asDuration
	asCount
	asKbps
	asNumber
	asBool
)

var metaFormats = map[string]int{
	"accessed": asDate, "statchange": asDate, "taken": asDate, "duration": asDuration, "vbitrate": asKbps,
	"season": asNumber, "episode": asNumber, "track": asNumber, "year": asNumber, "iso": asNumber, "pages": asNumber, "entries": asNumber,
	"words": asCount, "lines": asCount, "unpacked": asSize,
	"encrypted": asBool, "archcrypt": asBool, "bootable": asBool, "signed": asBool,
}

// numericMeta are the metadata columns the list sorts as numbers.
var numericMeta = set("accessed", "statchange", "duration", "vbitrate", "season", "episode", "track", "taken", "iso", "pages", "words", "lines", "unpacked", "entries")

func metaText(r *row, col string) string {
	v, ok := r.meta[col]
	if !ok {
		return ""
	}
	switch x := v.(type) {
	case string:
		return x
	case bool:
		if x {
			return "Ja"
		}
		return "Nein"
	}
	n, _ := r.metaNum(col)
	switch metaFormats[col] {
	case asSize:
		return fmtSize(int64(n))
	case asDate:
		return fmtDate(time.UnixMilli(int64(n)))
	case asDuration:
		return mmss(int64(n))
	case asCount:
		return fmtCount(int64(n))
	case asKbps:
		return fmtCount(int64(n)) + " kBit/s"
	}
	return strconv.FormatFloat(n, 'f', -1, 64)
}

func metaColumn(id string) column {
	c := column{meta: id, text: func(r *row) string { return metaText(r, id) }}
	if numericMeta[id] {
		c.num = func(r *row) (float64, bool) { return r.metaNum(id) }
	}
	if metaFormats[id] == asDate {
		c.at = func(r *row) (time.Time, bool) { return r.metaTime(id) }
	}
	return c
}

func rights(mode uint32, dir bool) string {
	out := "-"
	if dir {
		out = "d"
	}
	for shift := 6; shift >= 0; shift -= 3 {
		d := (mode >> shift) & 7
		out += [8]string{"---", "--x", "-w-", "-wx", "r--", "r-x", "rw-", "rwx"}[d]
	}
	return out
}

func diskSize(size int64) int64 { return (size + 4095) / 4096 * 4096 }

// columns are the list's columns by id, as search fields.
var columns = withMeta(map[string]column{
	"size":     {text: func(r *row) string { return fmtSize(r.Size) }, num: sizeOf},
	"modified": {at: func(r *row) (time.Time, bool) { return r.MTime, true }, text: func(r *row) string { return fmtDate(r.MTime) }},
	"created": {at: func(r *row) (time.Time, bool) { return r.Created, !r.Created.IsZero() }, text: func(r *row) string {
		if r.Created.IsZero() {
			return ""
		}
		return fmtDate(r.Created)
	}},
	"type": {text: func(r *row) string {
		switch {
		case r.Dir:
			return "Ordner"
		case r.stat().link:
			return "Verknüpfung"
		case extOf(r.Name) == "":
			return "Datei"
		}
		return typeNames[r.kind()]
	}},
	"ext": {text: func(r *row) string {
		if r.Dir {
			return ""
		}
		return extOf(r.Name)
	}},
	"items": {text: func(r *row) string {
		if !r.Dir {
			return ""
		}
		return strconv.Itoa(r.count())
	}, num: func(r *row) (float64, bool) {
		if !r.Dir {
			return 0, false
		}
		return float64(r.count()), true
	}},
	"path": {text: func(r *row) string { return r.At }},
	"link": {text: func(r *row) string { return r.stat().target }},
	"owner": {text: func(r *row) string {
		if o := r.stat().owner; o != "" {
			return o
		}
		return "nobody"
	}},
	"group": {text: func(r *row) string {
		if g := r.stat().group; g != "" {
			return g
		}
		return "users"
	}},
	"rights":   {text: func(r *row) string { return rights(r.mode(), r.Dir) }},
	"octal":    {text: func(r *row) string { return fmt.Sprintf("%03o", r.mode()&0o777) }},
	"disksize": {text: func(r *row) string { return fmtSize(diskSize(r.Size)) }, num: fileNum(func(r *row) float64 { return float64(diskSize(r.Size)) })},
	"kind": {text: func(r *row) string {
		if r.Dir {
			return "Ordner"
		}
		if n, ok := kindNames[r.kind()]; ok {
			return n
		}
		return "Sonstiges"
	}},
	"mime": {text: func(r *row) string {
		if r.Dir {
			return "inode/directory"
		}
		if m, ok := mimes[extOf(r.Name)]; ok {
			return m
		}
		return "application/octet-stream"
	}},
	"attributes": {text: func(r *row) string {
		var b strings.Builder
		if r.Hidden || strings.HasPrefix(r.Name, ".") {
			b.WriteByte('H')
		}
		if r.stat().link {
			b.WriteByte('L')
		}
		if (r.mode()>>6)&7 < 6 {
			b.WriteByte('R')
		}
		return b.String()
	}},
	"share": {text: func(r *row) string {
		if rest, ok := strings.CutPrefix(r.Path, "/mnt/user/"); ok {
			share, _, _ := strings.Cut(rest, "/")
			return share
		}
		return ""
	}},
	"hardlinks": {text: func(r *row) string { return strconv.FormatUint(r.stat().nlink, 10) }},
	"inode":     {text: func(r *row) string { return strconv.FormatUint(r.stat().ino, 10) }},
	"blocks":    {text: func(r *row) string { return strconv.FormatInt((r.Size+511)/512, 10) }, num: fileNum(func(r *row) float64 { return float64((r.Size + 511) / 512) })},
	"uid":       {text: func(r *row) string { return strconv.FormatUint(uint64(r.stat().uid), 10) }},
	"gid":       {text: func(r *row) string { return strconv.FormatUint(uint64(r.stat().gid), 10) }},
	"special": {text: func(r *row) string {
		m := r.mode()
		var out []string
		for _, s := range []struct {
			bit  uint32
			name string
		}{{0o1000, "Sticky"}, {0o2000, "SGID"}, {0o4000, "SUID"}} {
			if m&s.bit != 0 {
				out = append(out, s.name)
			}
		}
		return strings.Join(out, ", ")
	}},
})

// withMeta adds the columns fs/meta fills.
func withMeta(m map[string]column) map[string]column {
	for _, id := range []string{
		"accessed", "statchange", "disk", "dimensions", "duration", "artist", "album", "camera", "taken", "pages",
		"sha256", "vcodec", "acodec", "framerate", "vbitrate", "aspect", "hdr", "audiotracks", "subtitles", "season", "episode",
		"title", "albumartist", "track", "year", "genre", "composer", "abitrate", "samplerate", "channels", "make", "lens", "focal",
		"aperture", "exposure", "iso", "flash", "place", "orientation", "depth", "dpi", "author", "lastauthor", "doctitle", "subject",
		"words", "lines", "encoding", "app", "encrypted", "unpacked", "ratio", "entries", "method", "archcrypt", "fs", "volume",
		"bootable", "version", "vendor", "arch", "signed", "crc32", "md5", "sha1",
	} {
		m[id] = metaColumn(id)
	}
	return m
}

func sizeOf(r *row) (float64, bool) {
	if r.Dir {
		return 0, false
	}
	return float64(r.Size), true
}

func fileNum(f func(r *row) float64) func(r *row) (float64, bool) {
	return func(r *row) (float64, bool) {
		if r.Dir {
			return 0, false
		}
		return f(r), true
	}
}

func (r *row) count() int {
	if !r.counted {
		r.counted = true
		if d, err := os.ReadDir(r.Real); err == nil {
			r.items = len(d)
		}
	}
	return r.items
}

// labels are the German column names of the prototype, which the field
// search accepts next to the ids.
var labels = map[string]string{
	"size": "Größe", "modified": "Geändert", "created": "Erstellt", "accessed": "Letzter Zugriff", "type": "Typ", "ext": "Endung",
	"items": "Elemente", "path": "Ordner", "link": "Verknüpfungsziel", "owner": "Besitzer", "group": "Gruppe", "rights": "Rechte",
	"octal": "Oktal", "dimensions": "Abmessungen", "duration": "Dauer", "artist": "Interpret", "album": "Album", "camera": "Kameramodell",
	"taken": "Aufgenommen", "pages": "Seiten", "sha256": "SHA-256", "disksize": "Größe auf Datenträger", "kind": "Art", "mime": "MIME-Typ",
	"attributes": "Attribute", "statchange": "Statusänderung", "disk": "Laufwerk", "share": "Freigabe", "hardlinks": "Harte Links",
	"inode": "Inode", "blocks": "Blöcke", "uid": "UID", "gid": "GID", "special": "Sonderrechte", "vcodec": "Videocodec",
	"acodec": "Audiocodec", "framerate": "Bildrate", "vbitrate": "Datenrate", "aspect": "Seitenverhältnis", "hdr": "HDR",
	"audiotracks": "Tonspuren", "subtitles": "Untertitel", "season": "Staffel", "episode": "Folge", "title": "Titel",
	"albumartist": "Albuminterpret", "track": "Nr.", "year": "Jahr", "genre": "Genre", "composer": "Komponist", "abitrate": "Bitrate",
	"samplerate": "Abtastrate", "channels": "Kanäle", "make": "Kamerahersteller", "lens": "Objektiv", "focal": "Brennweite",
	"aperture": "Blende", "exposure": "Belichtungszeit", "iso": "ISO", "flash": "Blitz", "place": "Ort", "orientation": "Ausrichtung",
	"depth": "Farbtiefe", "dpi": "Auflösung", "author": "Autor", "lastauthor": "Zuletzt gespeichert von", "doctitle": "Dokumenttitel",
	"subject": "Betreff", "words": "Wörter", "lines": "Zeilen", "encoding": "Kodierung", "app": "Programm", "encrypted": "Verschlüsselt",
	"unpacked": "Entpackte Größe", "ratio": "Kompressionsrate", "entries": "Dateien im Archiv", "method": "Packverfahren",
	"archcrypt": "Passwortschutz", "fs": "Dateisystem", "volume": "Bezeichnung", "bootable": "Startfähig", "version": "Version",
	"vendor": "Hersteller", "arch": "Architektur", "signed": "Signiert", "crc32": "CRC32", "md5": "MD5", "sha1": "SHA-1",
}

// aliases are words that read better than the column names.
var aliases = map[string]string{
	"codec": "vcodec", "kamera": "camera", "interpret": "artist", "kuenstler": "artist", "kunstler": "artist", "jahr": "year",
	"seiten": "pages", "typ": "type", "art": "kind", "endung": "ext", "ordner": "path", "besitzer": "owner", "titel": "title",
	"objektiv": "lens", "blende": "aperture", "aufloesung": "dimensions", "bildrate": "framerate", "tonspuren": "audiotracks",
}

// fields maps every name a field answers to onto its column. The special
// ones come last and win: sizes count files only, as Explorer does,
// because a folder's total would match almost any size.
var fields = func() map[string]column {
	m := map[string]column{}
	for id, label := range labels {
		m[fold(label)] = columns[id]
		m[fold(id)] = columns[id]
	}
	for from, id := range aliases {
		m[from] = columns[id]
	}
	m[fold("auflösung")] = columns["dimensions"]
	side := func(i int) func(r *row) (float64, bool) {
		return func(r *row) (float64, bool) {
			w, h, ok := strings.Cut(metaText(r, "dimensions"), " × ")
			if !ok {
				return 0, false
			}
			n, err := strconv.Atoi([]string{w, h}[i])
			return float64(n), err == nil
		}
	}
	plain := func(s string) float64 { return number(s) }
	special := []struct {
		names []string
		c     column
	}{
		{[]string{"größe", "groesse", "size"}, column{unit: Bytes, num: sizeOf}},
		{[]string{"geändert", "geaendert", "datum", "modified"}, columns["modified"]},
		{[]string{"breite", "width"}, column{unit: plain, num: side(0), meta: "dimensions"}},
		{[]string{"höhe", "hoehe", "height"}, column{unit: plain, num: side(1), meta: "dimensions"}},
		{[]string{"dauer", "duration", "länge", "laenge"}, column{unit: secs, num: func(r *row) (float64, bool) { return r.metaNum("duration") }, meta: "duration"}},
	}
	for _, s := range special {
		for _, n := range s.names {
			m[fold(n)] = s.c
		}
	}
	return m
}()

func fmtSize(b int64) string {
	for _, u := range []struct {
		name string
		v    float64
	}{{"TB", tb}, {"GB", gb}, {"MB", mb}, {"KB", kb}} {
		if float64(b) >= u.v {
			x := float64(b) / u.v
			prec := 0
			if x < 10 {
				prec = 1
			}
			return germanDecimal(x, prec) + " " + u.name
		}
	}
	return strconv.FormatInt(b, 10) + " B"
}

func germanDecimal(v float64, prec int) string {
	s := strconv.FormatFloat(v, 'f', prec, 64)
	if strings.Contains(s, ".") {
		s = strings.TrimRight(strings.TrimRight(s, "0"), ".")
	}
	return strings.Replace(s, ".", ",", 1)
}

func fmtDate(t time.Time) string { return t.Local().Format("02.01.2006") }

func fmtCount(n int64) string {
	s := strconv.FormatInt(n, 10)
	neg := strings.HasPrefix(s, "-")
	s = strings.TrimPrefix(s, "-")
	var b strings.Builder
	for i, c := range s {
		if i > 0 && (len(s)-i)%3 == 0 {
			b.WriteByte('.')
		}
		b.WriteRune(c)
	}
	if neg {
		return "-" + b.String()
	}
	return b.String()
}

func mmss(s int64) string {
	h, m, sec := s/3600, (s/60)%60, s%60
	if h > 0 {
		return fmt.Sprintf("%d:%02d:%02d", h, m, sec)
	}
	return fmt.Sprintf("%d:%02d", m, sec)
}

// parentOf is the folder an API path lies in.
func parentOf(p string) string { return path.Dir(p) }
