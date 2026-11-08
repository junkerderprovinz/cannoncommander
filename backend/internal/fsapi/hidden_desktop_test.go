//go:build desktop

package fsapi

import (
	"os"
	"path/filepath"
	"runtime"
	"testing"

	"github.com/junkerderprovinz/cannoncommander/backend/pkg/desktop"
)

type quietApp struct{}

func (quietApp) AutoUpdate() bool         { return false }
func (quietApp) SetAutoUpdate(bool) error { return nil }
func (quietApp) UpdateReady() string      { return "" }
func (quietApp) Restart()                 {}

func asDesktop(t *testing.T) {
	t.Helper()
	desktop.Use(quietApp{}, desktop.Options{})
	t.Cleanup(func() { desktop.Use(nil, desktop.Options{}) })
}

func TestTheDesktopAppOnLinuxHidesTheNamesInDotHidden(t *testing.T) {
	if runtime.GOOS != "linux" {
		t.Skip("the .hidden file belongs to the Linux desktops")
	}
	f := newFixture(t)
	dir := filepath.Join(f.share, "fotos")
	write(t, filepath.Join(dir, "snap"), 1)
	if err := os.WriteFile(filepath.Join(dir, ".hidden"), []byte("snap\n2025\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	l := decode[Listing](t, f.get(t, "jdp", "/fs/list", "/mnt/user/fotos"))
	if byName(t, l.Items, "snap").Hidden || byName(t, l.Items, "2025").Hidden {
		t.Error("the container hides what .hidden names")
	}

	asDesktop(t)
	l = decode[Listing](t, f.get(t, "jdp", "/fs/list", "/mnt/user/fotos"))
	if !byName(t, l.Items, "snap").Hidden || !byName(t, l.Items, "2025").Hidden {
		t.Errorf("listing = %+v", l.Items)
	}
}
