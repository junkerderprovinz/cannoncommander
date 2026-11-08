package fsapi

import (
	"io"
	"os"
	"path/filepath"

	"github.com/junkerderprovinz/cannoncommander/backend/pkg/winshell"
	"golang.org/x/sys/windows"
)

// labelOf is the name Explorer shows for a folder, such as Dokumente for
// Documents or Benutzer for Users. Explorer reads desktop.ini only in a
// folder marked read-only or system, so the others cost nothing.
func labelOf(real string, attrs uint32) string {
	if attrs&(attrReadOnly|attrSystem) == 0 {
		return ""
	}
	f, err := os.Open(filepath.Join(real, "desktop.ini"))
	if err != nil {
		return ""
	}
	defer f.Close()
	ini, err := io.ReadAll(io.LimitReader(f, 16<<10))
	if err != nil {
		return ""
	}
	root, err := windows.GetSystemWindowsDirectory()
	if err != nil {
		return ""
	}
	ref, ok := localResource(localizedName(ini), root)
	if !ok {
		return ""
	}
	name, _ := winshell.IndirectString(ref)
	return name
}
