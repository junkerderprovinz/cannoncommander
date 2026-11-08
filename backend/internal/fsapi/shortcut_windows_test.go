package fsapi

import (
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/junkerderprovinz/cannoncommander/backend/internal/shortcut"
)

func TestWindowsListsJunctionsAndShortcutsByWhereTheyLead(t *testing.T) {
	f := newFixture(t)
	filme := filepath.Join(f.share, "filme")
	if out, err := exec.Command("cmd", "/c", "mklink", "/J", filepath.Join(f.share, "fotos", "Kino"), filme).CombinedOutput(); err != nil {
		t.Skipf("no junction here: %v %s", err, out)
	}
	if err := shortcut.Write(filepath.Join(f.share, "fotos", "Filme.lnk"), filme, true); err != nil {
		t.Fatal(err)
	}

	l := decode[Listing](t, f.get(t, "jdp", "/fs/list", "/mnt/user/fotos"))
	if kino := byName(t, l.Items, "Kino"); !kino.Dir || kino.Link != "/mnt/user/filme" {
		t.Errorf("junction = %+v", kino)
	}
	if lnk := byName(t, l.Items, "Filme.lnk"); lnk.Shortcut != nil {
		t.Errorf("the server on its own reads shortcuts: %+v", lnk.Shortcut)
	}

	asDesktop(t)
	l = decode[Listing](t, f.get(t, "jdp", "/fs/list", "/mnt/user/fotos"))
	lnk := byName(t, l.Items, "Filme.lnk")
	if lnk.Shortcut == nil || lnk.Shortcut.Path != "/mnt/user/filme" || !lnk.Shortcut.Dir || lnk.Shortcut.Kind != shortcut.Lnk {
		t.Errorf("shortcut = %+v", lnk.Shortcut)
	}
}
