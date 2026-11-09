//go:build desktop

package system

import (
	"testing"

	"github.com/junkerderprovinz/cannoncommander/backend/pkg/desktop"
)

type desktopApp struct{}

func (desktopApp) AutoUpdate() bool         { return false }
func (desktopApp) SetAutoUpdate(bool) error { return nil }
func (desktopApp) UpdateReady() string      { return "" }
func (desktopApp) Restart()                 {}

// The page shows the Flatpak's update switch off with the reason, and no
// terminal, since the sandbox holds none of the computer's.
func TestAFlatpakSaysSoAndOpensNoTerminal(t *testing.T) {
	f := newFixture(t)
	desktop.Use(desktopApp{}, desktop.Options{})
	t.Cleanup(func() { desktop.Use(nil, desktop.Options{}) })

	t.Setenv("FLATPAK_ID", "")
	var in Info
	if code := f.call("jdp", "GET", "/system", "", &in); code != 200 || in.Flatpak || !in.Feature.Terminal {
		t.Fatalf("outside a Flatpak: %d, flatpak %v, terminal %v", code, in.Flatpak, in.Feature.Terminal)
	}

	t.Setenv("FLATPAK_ID", "io.github.junkerderprovinz.cannoncommander")
	in = Info{}
	if code := f.call("jdp", "GET", "/system", "", &in); code != 200 || in.Deployment != "desktop" || !in.Flatpak || in.Feature.Terminal {
		t.Fatalf("in a Flatpak: %d, %s, flatpak %v, terminal %v", code, in.Deployment, in.Flatpak, in.Feature.Terminal)
	}
}
