package search

import (
	"path"
	"regexp"
	"strings"
)

func set(exts ...string) map[string]bool {
	m := make(map[string]bool, len(exts))
	for _, e := range exts {
		m[e] = true
	}
	return m
}

// The kinds of the list: the prototype's extensions and the formats of
// the same groups that real shares are full of.
var kinds = []struct {
	kind string
	exts map[string]bool
}{
	{"img", set("jpg", "jpeg", "png", "webp", "heic", "heif", "gif", "avif", "tif", "tiff", "bmp", "cr2", "cr3", "nef", "arw", "dng", "orf", "raf", "rw2")},
	{"video", set("mkv", "mp4", "webm", "mov", "m4v", "avi", "wmv", "ts", "m2ts", "mpg", "mpeg", "flv", "3gp")},
	{"audio", set("mp3", "flac", "ogg", "wav", "opus", "m4a", "aac", "wma", "alac", "aiff")},
	{"arch", set("zip", "zst", "7z", "rar", "tar", "gz", "tgz", "bz2", "xz", "lz4", "cab")},
	{"disk", set("iso", "img", "vhd", "vhdx", "qcow2", "vmdk", "vdi", "dmg")},
	{"pdf", set("pdf")},
	{"office", set("docx", "xlsx", "pptx", "odt", "ods", "odp", "doc", "xls", "ppt", "rtf")},
	{"text", set("md", "txt", "srt", "xml", "json", "cfg", "log", "sha256", "conf", "1", "markdown", "nfo", "ini", "toml", "csv", "tsv", "vtt", "ass", "ssa")},
	{"code", set("yml", "yaml", "go", "sh", "js", "ts", "py", "rs", "c", "cpp", "h", "java", "php", "rb", "lua", "html", "css", "vue", "sql", "ps1", "bat")},
}

var rotatedLog = regexp.MustCompile(`\.log\.\d+$`)

func extOf(name string) string {
	if !strings.Contains(name, ".") {
		return ""
	}
	return strings.TrimPrefix(strings.ToLower(path.Ext(name)), ".")
}

// KindOf sorts an entry into one of the list's kinds: dir, img, video,
// audio, arch, disk, pdf, office, text, code or file.
func KindOf(name string, dir bool) string {
	if dir {
		return "dir"
	}
	if rotatedLog.MatchString(name) {
		return "text"
	}
	e := extOf(name)
	for _, k := range kinds {
		// "ts" is a video stream and TypeScript at once; the prototype
		// counts it as code.
		if k.exts[e] && !(k.kind == "video" && e == "ts") {
			return k.kind
		}
	}
	return "file"
}

// typeOK is the Art filter of the search window.
func typeOK(kind, want string) bool {
	switch want {
	case "", "all":
		return true
	case "doc":
		return kind == "pdf" || kind == "office" || kind == "text" || kind == "code"
	case "arch":
		return kind == "arch" || kind == "disk"
	}
	return kind == want
}

// ValidType reports whether t is one of the search window's types.
func ValidType(t string) bool {
	switch t {
	case "", "all", "dir", "img", "video", "doc", "arch", "audio":
		return true
	}
	return false
}
