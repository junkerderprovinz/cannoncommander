package indexing

import (
	"github.com/junkerderprovinz/cannoncommander/backend/pkg/settings"
)

// SetTestIndex allows tests to register mock indices without database initialization
// This is useful for testing code that depends on GetIndex() without needing full setup
func SetTestIndex(name string, path string) {
	indexesMutex.Lock()
	defer indexesMutex.Unlock()

	idx := &Index{
		Source: settings.Source{
			Name: name,
			Path: path,
		},
		mock:                true,
		folderSizes:         make(map[string]uint64),
		folderSizesUnsynced: make(map[string]struct{}),
	}
	indexes[name] = idx
}

// ClearTestIndices removes all test indices - call in test cleanup
func ClearTestIndices() {
	indexesMutex.Lock()
	defer indexesMutex.Unlock()
	indexes = make(map[string]*Index)
}
