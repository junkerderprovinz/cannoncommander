package meta

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

// The extension lists follow the format groups of the interface, so a file
// gets the columns of the group it is shown in.
var (
	imageExts   = set("jpg", "jpeg", "png", "gif", "webp", "heic", "heif", "avif", "tif", "tiff", "bmp", "cr2", "cr3", "nef", "arw", "dng", "orf", "raf", "rw2")
	videoExts   = set("mkv", "mp4", "m4v", "mov", "avi", "wmv", "webm", "ts", "m2ts", "mpg", "mpeg", "flv", "3gp")
	audioExts   = set("mp3", "flac", "wav", "ogg", "opus", "m4a", "aac", "wma", "alac", "aiff")
	taggedExts  = set("mp3", "flac", "ogg", "opus", "m4a", "alac")
	pagedExts   = set("pdf", "epub", "xps", "oxps", "cbz", "fb2", "mobi")
	officeExts  = set("docx", "docm", "xlsx", "xlsm", "pptx", "pptm", "odt", "ods", "odp")
	archiveExts = set("zip", "rar", "7z", "tar", "gz", "tgz", "bz2", "xz", "zst", "lz4", "cab")
	diskExts    = set("iso", "img", "vhd", "vhdx", "qcow2", "vmdk", "vdi", "dmg")
	programExts = set("exe", "dll", "msi", "appimage", "deb", "rpm", "apk", "pkg", "jar")
	textExts    = set(
		"txt", "md", "markdown", "nfo", "rst",
		"js", "mjs", "ts", "jsx", "tsx", "py", "go", "rs", "c", "cpp", "h", "hpp", "java", "kt", "swift", "php", "rb", "lua",
		"sh", "bash", "zsh", "ps1", "bat", "cmd", "html", "htm", "css", "scss", "vue", "svelte", "sql",
		"json", "yaml", "yml", "toml", "xml", "ini", "cfg", "conf", "env", "plist", "properties",
		"log", "out", "srt", "ass", "ssa", "vtt", "csv", "tsv", "sha256", "sha1", "md5", "sfv",
	)
)

var rotatedLog = regexp.MustCompile(`\.log\.\d+$`)

// extOf returns the lower case extension without the dot. Rotated logs
// such as syslog.log.1 count as logs.
func extOf(name string) string {
	if rotatedLog.MatchString(name) {
		return "log"
	}
	return strings.TrimPrefix(strings.ToLower(path.Ext(name)), ".")
}
