package fsapi

import (
	"bufio"
	"io"
	"os"
	"path/filepath"
	"runtime"
	"strings"

	"github.com/junkerderprovinz/cannoncommander/backend/internal/ccfs"
	"github.com/junkerderprovinz/cannoncommander/backend/pkg/desktop"
)

// hiddenOf decides as the file manager of the system does. Explorer hides
// by the attribute and keeps what is also a system file, such as
// desktop.ini, back for a second switch.
func hiddenOf(name string, attrs uint32) (hidden, system bool) {
	byAttr := attrs&attrHidden != 0
	return byAttr || ccfs.HiddenByName(name), byAttr && attrs&attrSystem != 0
}

// hiddenListed reads the .hidden file that Nautilus, Dolphin and Thunar
// keep in a folder, one name per line. Only the desktop app on Linux
// follows it, the container shows what the shares hold.
func hiddenListed(dir string) map[string]bool {
	if runtime.GOOS != "linux" || !desktop.Active() {
		return nil
	}
	f, err := os.Open(filepath.Join(dir, ".hidden"))
	if err != nil {
		return nil
	}
	defer f.Close()
	out := map[string]bool{}
	lines := bufio.NewScanner(io.LimitReader(f, 64<<10))
	for lines.Scan() {
		if name := strings.TrimSuffix(lines.Text(), "\r"); name != "" {
			out[name] = true
		}
	}
	return out
}
