package indexing

import (
	"strings"
	"testing"
)

// Mock rows have no file on disk, so header sniffing would degrade to "blob"
// and undo the point of typing them. Types come from the extension alone.
func TestMockMimeTypeFromExtension(t *testing.T) {
	cases := map[string]string{
		"file-a.txt": "text/plain",
		"file-a.mp3": "audio/mpeg",
		"file-a.mov": "video/quicktime",
		"file-a.doc": "application/msword",
		"file-a.mp4": "video/mp4",
		"file-a.zip": "application/zip",
		"file-a.jpg": "image/jpeg",
		// Unknown or absent extensions fall back rather than erroring.
		"file-a.unknownext": "blob",
		"file-a":            "blob",
	}
	for name, want := range cases {
		if got := mockMimeType(name); got != want {
			t.Errorf("mockMimeType(%q) = %q, want %q", name, got, want)
		}
	}
}

// The charset parameter must be stripped so mock rows match the types real
// indexing produces (mime.TypeByExtension(".txt") appends "; charset=utf-8").
func TestMockMimeTypeStripsCharset(t *testing.T) {
	if got := mockMimeType("file-a.txt"); strings.Contains(got, ";") || strings.Contains(got, "charset") {
		t.Errorf("mockMimeType returned %q; expected the charset parameter stripped", got)
	}
	// The extension is matched case-insensitively.
	if got := mockMimeType("FILE-A.JPG"); got != "image/jpeg" {
		t.Errorf("mockMimeType(\"FILE-A.JPG\") = %q, want \"image/jpeg\"", got)
	}
}
