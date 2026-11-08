package fileops_test

import (
	"bytes"
	"encoding/json"
	"net/http"
	"os"
	"runtime"
	"testing"

	"github.com/junkerderprovinz/cannoncommander/backend/internal/fileops"
)

func TestARenameMayChangeOnlyTheCaseOfTheName(t *testing.T) {
	e := newEnv(t)
	e.write("/mnt/user/fotos/foto.JPG", "f", old)
	var ch fileops.Changed
	if code := e.call("anna", "POST", "/fs/rename", map[string]string{"path": "/mnt/user/fotos/foto.JPG", "name": "foto.jpg"}, &ch); code != http.StatusOK {
		t.Fatalf("rename to another case: %d", code)
	}
	names := func() []string {
		list, err := os.ReadDir(e.real("/mnt/user/fotos"))
		if err != nil {
			t.Fatal(err)
		}
		var out []string
		for _, d := range list {
			out = append(out, d.Name())
		}
		return out
	}
	if got := names(); len(got) != 1 || got[0] != "foto.jpg" {
		t.Fatalf("folder holds %v", got)
	}
	if code := e.call("anna", "POST", "/undo", map[string]string{"id": ch.Undo}, nil); code != http.StatusOK {
		t.Fatalf("undo: %d", code)
	}
	if got := names(); len(got) != 1 || got[0] != "foto.JPG" {
		t.Errorf("after the undo the folder holds %v", got)
	}
}

func TestWindowsTurnsAwayTheNamesExplorerRefuses(t *testing.T) {
	if runtime.GOOS != "windows" {
		t.Skip("the rules of Windows names")
	}
	e := newEnv(t)
	for name, code := range map[string]string{"CON": "nameReserved", "nul.txt": "nameReserved", "a<b": "nameChars", "Bericht.": "nameEnd", "Bericht ": "nameEnd"} {
		body, _ := json.Marshal(map[string]string{"path": "/mnt/user/fotos/" + name})
		req, _ := http.NewRequest("POST", e.srv.URL+"/fs/mkdir", bytes.NewReader(body))
		req.Header.Set("X-User", "anna")
		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			t.Fatal(err)
		}
		var p struct {
			Code string `json:"code"`
		}
		_ = json.NewDecoder(resp.Body).Decode(&p)
		resp.Body.Close()
		if resp.StatusCode != http.StatusBadRequest || p.Code != code {
			t.Errorf("%q: %d %q, want %s", name, resp.StatusCode, p.Code, code)
		}
	}
	if got := e.call("anna", "POST", "/fs/mkdir", map[string]string{"path": "/mnt/user/fotos/CONTRA"}, nil); got != http.StatusCreated {
		t.Errorf("a name that only starts like a device: %d", got)
	}
}
