package indexing

import (
	"math/rand"
	"mime"
	"path/filepath"
	"strings"
	"time"

	"github.com/junkerderprovinz/cannoncommander/backend/internal/utils"
	"github.com/junkerderprovinz/cannoncommander/backend/pkg/indexing/iteminfo"
)

func (idx *Index) CreateMockData(numDirs, numFilesPerDir int) {
	r := rand.New(rand.NewSource(int64(numDirs*1_000_003 + numFilesPerDir)))
	for i := 0; i < numDirs; i++ {
		dirPath := utils.GenerateRandomPathSeeded(r, r.Intn(3)+1)
		files := []iteminfo.ExtendedItemInfo{}

		// Simulating files and directories with ExtendedItemInfo
		for j := 0; j < numFilesPerDir; j++ {
			name := "file-" + utils.GetRandomTermSeeded(r) + utils.GetRandomExtensionSeeded(r)
			newFile := iteminfo.ExtendedItemInfo{
				ItemInfo: iteminfo.ItemInfo{
					Name:    name,
					Size:    mockSizeFor(name, r),
					ModTime: time.Now().Add(-time.Duration(r.Intn(100)) * time.Hour),
					Type:    mockMimeType(name),
				},
			}
			files = append(files, newFile)
		}
		dirInfo := &iteminfo.FileInfo{
			Path:  dirPath,
			Files: files,
		}

		idx.UpdateMetadata(dirInfo, nil, true) // nil scanner for mock
	}
}

// mockMimeType resolves a MIME type from a filename's extension.
//
// Extension-only by design. Real indexing uses ItemInfo.DetectType, which
// additionally sniffs file headers to disambiguate extensions like `.ts`; mock
// rows have no file on disk, so sniffing would just return "blob" and undo the
// point of typing them at all. A plain extension lookup is enough to exercise
// the type-dependent UI paths (icons, preview eligibility, size formatting).
//
// The charset parameter is stripped so the values match what indexed items
// carry: mime.TypeByExtension(".txt") returns "text/plain; charset=utf-8",
// whereas a real indexed item has "text/plain".
//
// Fixed table first: mock types must not depend on host mailcap/shared-mime-info.
var mockMimeTypes = map[string]string{
	".txt": "text/plain",
	".mp3": "audio/mpeg",
	".mov": "video/quicktime",
	".doc": "application/msword",
	".mp4": "video/mp4",
	".bak": "application/x-trash",
	".zip": "application/zip",
	".jpg": "image/jpeg",
}

func mockMimeType(name string) string {
	ext := strings.ToLower(filepath.Ext(name))
	if m, ok := mockMimeTypes[ext]; ok {
		return m
	}
	if m := strings.Split(mime.TypeByExtension(ext), ";")[0]; m != "" {
		return m
	}
	return "blob"
}

// mockSizeFor returns a plausible size for a mock file.
//
// Sizes are drawn from the real size bucket for the extension rather than a
// flat 0-999 range, so the listing exercises realistic formatting: a `.txt`
// renders in bytes while a `.mp4` or `.zip` renders in MB/GB. A flat range made
// every cell show a near-identical tiny size.
func mockSizeFor(name string, r *rand.Rand) int64 {
	ext := strings.ToLower(filepath.Ext(name))
	switch ext {
	case ".jpg", ".mp3", ".doc":
		// Hundreds of KB to a few MB.
		return 50_000 + r.Int63n(4_000_000)
	case ".mp4", ".mov":
		// Tens to hundreds of MB.
		return 10_000_000 + r.Int63n(500_000_000)
	case ".zip", ".bak":
		// KB to tens of MB.
		return 1_000 + r.Int63n(20_000_000)
	default:
		// Small text-like files.
		return 10 + r.Int63n(50_000)
	}
}
