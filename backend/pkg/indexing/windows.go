//go:build windows

package indexing

import (
	"os"
	"strings"

	"github.com/junkerderprovinz/cannoncommander/backend/pkg/indexing/iteminfo"
	"golang.org/x/sys/windows"
)

// CheckWindowsHidden reports the hidden attribute. A system file without it
// shows in Explorer, so the system attribute alone hides nothing.
func CheckWindowsHidden(realpath string) bool {
	pointer, err := windows.UTF16PtrFromString(realpath)
	if err != nil {
		return false
	}
	attributes, err := windows.GetFileAttributes(pointer)
	return err == nil && attributes&windows.FILE_ATTRIBUTE_HIDDEN != 0
}

// getFileSizeByMode returns file size based on config mode
// Centralizes the logic for choosing between disk usage and logical size
func getFileSizeByMode(logicalSize int64, useLogicalSize bool) uint64 {
	if useLogicalSize {
		// Logical size mode: return actual file size
		return uint64(logicalSize)
	}

	// Disk usage mode: calculate disk space used
	// If file is empty, return 0
	if logicalSize == 0 {
		return 0
	}

	// On Windows NTFS, cluster size is typically 4KB
	// Round up to nearest 4KB cluster
	const clusterSize = 4096
	clusters := (logicalSize + clusterSize - 1) / clusterSize
	return uint64(clusters * clusterSize)
}

// handleFile processes a file and returns its size and whether it should be counted
// On Windows, calculates size based on configuration (disk usage or logical size)
// scanner parameter is accepted for signature compatibility but not used on Windows (no hardlink tracking)
func (idx *Index) handleFile(file os.FileInfo, fullCombined string, realFilePath string, isRoutineScan bool, scanner *Scanner) (size uint64, shouldCountSize bool) {
	// Use centralized size calculation logic
	size = getFileSizeByMode(file.Size(), idx.Config.UseLogicalSize)
	return size, true
}

// input should be non-index path.
func (idx *Index) MakeIndexPathPlatform(path string) string {
	split := strings.Split(path, "\\")
	if len(split) > 1 {
		path = strings.Join(split, "/")
	} else {
		path = "/" + strings.TrimPrefix(path, "/")
	}
	return path
}

// unixAttrsOf has nothing to report on Windows, which only runs in development.
func unixAttrsOf(os.FileInfo) iteminfo.UnixAttrs { return iteminfo.UnixAttrs{} }
