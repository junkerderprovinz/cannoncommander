package tools

import (
	"net/http"
	"os"
	"strings"
	"testing"

	"github.com/junkerderprovinz/cannoncommander/backend/internal/shortcut"
)

func TestAShortcutOfWindowsGoesInAndOutAgain(t *testing.T) {
	e := newEnv(t)
	e.write("/mnt/user/fotos/a.txt", "a")
	var out map[string]string
	req := LinkRequest{Target: "/mnt/user/fotos", Path: "/mnt/user/backups/fotos - Verknüpfung.lnk", Shortcut: true}
	if code := e.call("anna", "POST", "/fs/link", req, &out); code != http.StatusCreated {
		t.Fatalf("shortcut %d", code)
	}
	sc, err := shortcut.Read(e.real(req.Path))
	if err != nil || !sc.Dir || !strings.EqualFold(sc.Target, e.real("/mnt/user/fotos")) {
		t.Errorf("shortcut %+v, %v", sc, err)
	}
	if code := e.call("anna", "POST", "/undo", map[string]string{"id": out["undo"]}, nil); code != http.StatusOK || e.exists(req.Path) {
		t.Errorf("undo of the shortcut %d", code)
	}
}

func TestAJunctionStandsInForALinkToAFolder(t *testing.T) {
	e := newEnv(t)
	e.write("/mnt/user/fotos/a.txt", "a")
	if err := junction(e.real("/mnt/user/fotos"), e.real("/mnt/user/backups/tor")); err != nil {
		t.Fatal(err)
	}
	if got := e.read("/mnt/user/backups/tor/a.txt"); got != "a" {
		t.Errorf("through the junction: %q", got)
	}
	if target, err := os.Readlink(e.real("/mnt/user/backups/tor")); err != nil || !strings.EqualFold(target, e.real("/mnt/user/fotos")) {
		t.Errorf("target %q, %v", target, err)
	}
	if err := os.Remove(e.real("/mnt/user/backups/tor")); err != nil || e.read("/mnt/user/fotos/a.txt") != "a" {
		t.Errorf("removing the junction: %v", err)
	}
}
