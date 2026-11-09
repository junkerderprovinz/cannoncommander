package mcpapi

import (
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestAReadKeyChangesNothing(t *testing.T) {
	e := newEnv(t)
	e.write("/mnt/user/filme/a.txt", "x")
	key, _ := e.key("anna", false)
	s := e.client(key)
	for name, args := range map[string]map[string]any{
		"create_folder":      {"path": "/mnt/user/filme/neu"},
		"copy_items":         {"sources": []any{"/mnt/user/filme/a.txt"}, "destination": "/mnt/user/fotos"},
		"move_items":         {"sources": []any{"/mnt/user/filme/a.txt"}, "destination": "/mnt/user/fotos"},
		"rename_item":        {"path": "/mnt/user/filme/a.txt", "newName": "b.txt"},
		"move_to_trash":      {"paths": []any{"/mnt/user/filme/a.txt"}},
		"restore_from_trash": {"ids": []any{"x"}},
	} {
		if _, code := e.tool(s, name, args); code != "read_only_key" {
			t.Fatalf("%s with a read key: %q", name, code)
		}
	}
	if !e.exists("/mnt/user/filme/a.txt") || e.exists("/mnt/user/filme/neu") || e.exists("/mnt/user/fotos/a.txt") {
		t.Fatal("a read key changed a file")
	}
}

func TestTheWriteSwitchCountsFromTheNextCall(t *testing.T) {
	e := newEnv(t)
	key, id := e.key("anna", true)
	s := e.client(key)
	if _, code := e.tool(s, "create_folder", map[string]any{"path": "/mnt/user/filme/eins"}); code != "" {
		t.Fatalf("with the switch on: %q", code)
	}
	e.api("anna", http.MethodPatch, "/mcp/keys/"+id, map[string]any{"canWrite": false}, nil)
	if _, code := e.tool(s, "create_folder", map[string]any{"path": "/mnt/user/filme/zwei"}); code != "read_only_key" {
		t.Fatalf("after switching off: %q", code)
	}
}

func TestAWriteKeyHasNoMoreRightsThanItsAccount(t *testing.T) {
	e := newEnv(t)
	e.write("/mnt/user/archiv/alt.txt", "x")
	e.write("/mnt/user/geheim/x.txt", "x")
	annaKey, _ := e.key("anna", true)
	s := e.client(annaKey)
	if _, code := e.tool(s, "create_folder", map[string]any{"path": "/mnt/user/archiv/neu"}); code != "read_only" {
		t.Fatalf("a folder in a read-only folder: %q", code)
	}
	if _, code := e.tool(s, "rename_item", map[string]any{"path": "/mnt/user/geheim/x.txt", "newName": "y.txt"}); code != "not_found" {
		t.Fatalf("renaming in a hidden folder: %q", code)
	}
	if _, code := e.tool(s, "move_to_trash", map[string]any{"paths": []any{"/mnt/user/archiv/alt.txt"}}); code == "" {
		t.Fatal("trashed a file in a read-only folder")
	}
	gastKey, _ := e.key("gast", true)
	if _, code := e.tool(e.client(gastKey), "create_folder", map[string]any{"path": "/mnt/user/filme/neu"}); code == "" {
		t.Fatal("an account that may only look made a folder")
	}
	if e.exists("/mnt/user/archiv/neu") || e.exists("/mnt/user/geheim/y.txt") || !e.exists("/mnt/user/archiv/alt.txt") || e.exists("/mnt/user/filme/neu") {
		t.Fatal("something changed")
	}
}

func TestCopyRunsAsAJobTaggedWithTheKey(t *testing.T) {
	e := newEnv(t)
	e.write("/mnt/user/filme/a.txt", "inhalt")
	key, id := e.key("anna", true)
	out, code := e.tool(e.client(key), "copy_items", map[string]any{"sources": []any{"/mnt/user/filme/a.txt"}, "destination": "/mnt/user/fotos"})
	if code != "" {
		t.Fatalf("copy_items: %s", code)
	}
	jobID := out["jobId"].(string)
	if state := e.waitJob("anna", jobID); state != "done" {
		t.Fatalf("the job ended %s", state)
	}
	if !e.exists("/mnt/user/fotos/a.txt") || !e.exists("/mnt/user/filme/a.txt") {
		t.Fatal("the copy is missing")
	}
	var j apiJob
	e.api("anna", http.MethodGet, "/jobs/"+jobID, nil, &j)
	var list struct{ Keys []KeyView }
	e.api("anna", http.MethodGet, "/mcp/keys", nil, &list)
	if j.Via == "" || j.Via != list.Keys[0].Label || j.Undo == "" {
		t.Fatalf("the job = %+v", j)
	}
	var log struct{ Events []Event }
	e.api("anna", http.MethodGet, "/mcp/keys/"+id+"/activity", nil, &log)
	if log.Events[0].Tool != "copy_items" || log.Events[0].JobID != jobID {
		t.Fatalf("the key's log = %+v", log.Events)
	}
	var undo struct{ Steps []struct{ ID string } }
	e.api("anna", http.MethodGet, "/undo", nil, &undo)
	if len(undo.Steps) == 0 {
		t.Fatal("the copy left nothing to undo")
	}
}

func TestATakenNameIsSkippedOrNumberedButNeverReplaced(t *testing.T) {
	e := newEnv(t)
	e.write("/mnt/user/filme/a.txt", "neu")
	e.write("/mnt/user/fotos/a.txt", "alt")
	key, _ := e.key("anna", true)
	s := e.client(key)
	out, _ := e.tool(s, "copy_items", map[string]any{"sources": []any{"/mnt/user/filme/a.txt"}, "destination": "/mnt/user/fotos"})
	e.waitJob("anna", out["jobId"].(string))
	if b, _ := os.ReadFile(e.real("/mnt/user/fotos/a.txt")); string(b) != "alt" {
		t.Fatalf("the taken name holds %q", b)
	}
	out, _ = e.tool(s, "copy_items", map[string]any{"sources": []any{"/mnt/user/filme/a.txt"}, "destination": "/mnt/user/fotos", "onConflict": "keepBoth"})
	e.waitJob("anna", out["jobId"].(string))
	entries, _ := os.ReadDir(e.real("/mnt/user/fotos"))
	if len(entries) != 2 {
		t.Fatalf("keepBoth left %d entries", len(entries))
	}
	if _, code := e.tool(s, "copy_items", map[string]any{"sources": []any{"/mnt/user/filme/a.txt"}, "destination": "/mnt/user/fotos", "onConflict": "replace"}); code != "invalid_arguments" {
		t.Fatalf("replace: %q", code)
	}
}

func TestMoveRenameTrashAndRestore(t *testing.T) {
	e := newEnv(t)
	e.write("/mnt/user/filme/a.txt", "x")
	key, _ := e.key("anna", true)
	s := e.client(key)
	out, code := e.tool(s, "move_items", map[string]any{"sources": []any{"/mnt/user/filme/a.txt"}, "destination": "/mnt/user/fotos"})
	if code != "" || e.waitJob("anna", out["jobId"].(string)) != "done" || !e.exists("/mnt/user/fotos/a.txt") || e.exists("/mnt/user/filme/a.txt") {
		t.Fatalf("move: %v %s", out, code)
	}
	if out, code := e.tool(s, "rename_item", map[string]any{"path": "/mnt/user/fotos/a.txt", "newName": "b.txt"}); code != "" || out["path"] != "/mnt/user/fotos/b.txt" || !e.exists("/mnt/user/fotos/b.txt") {
		t.Fatalf("rename: %v %s", out, code)
	}
	out, code = e.tool(s, "move_to_trash", map[string]any{"paths": []any{"/mnt/user/fotos/b.txt"}})
	if code != "" || e.waitJob("anna", out["jobId"].(string)) != "done" || e.exists("/mnt/user/fotos/b.txt") {
		t.Fatalf("trash: %v %s", out, code)
	}
	list, _ := e.tool(s, "list_trash", nil)
	id := list["items"].([]any)[0].(map[string]any)["id"].(string)
	out, code = e.tool(s, "restore_from_trash", map[string]any{"ids": []any{id}})
	if code != "" || e.waitJob("anna", out["jobId"].(string)) != "done" || !e.exists("/mnt/user/fotos/b.txt") {
		t.Fatalf("restore: %v %s", out, code)
	}
	if _, code := e.tool(s, "restore_from_trash", map[string]any{"ids": []any{"nicht-da"}}); code != "not_found" {
		t.Fatalf("an unknown id: %q", code)
	}
}

func TestNoToolChangesAnArchiveOrTheTopOfASource(t *testing.T) {
	e := newEnv(t)
	key, _ := e.key("chef", true)
	s := e.client(key)
	if _, code := e.tool(s, "create_folder", map[string]any{"path": "/mnt/user/filme/a.zip::/neu"}); code != "in_archive" {
		t.Fatalf("a folder in an archive: %q", code)
	}
	if _, code := e.tool(s, "copy_items", map[string]any{"sources": []any{"/mnt/user/filme"}, "destination": "/mnt/user/x.zip::/"}); code != "in_archive" {
		t.Fatalf("a copy into an archive: %q", code)
	}
	for _, p := range []string{"/mnt/user", "/mnt", "/"} {
		if _, code := e.tool(s, "move_to_trash", map[string]any{"paths": []any{p}}); code != "source_root" {
			t.Fatalf("trashing %s: %q", p, code)
		}
		if _, code := e.tool(s, "rename_item", map[string]any{"path": p, "newName": "x"}); code != "source_root" {
			t.Fatalf("renaming %s: %q", p, code)
		}
	}
}

func TestOneCallChangesAtMostTwentyFiveEntries(t *testing.T) {
	e := newEnv(t)
	var paths []any
	for i := 0; i <= itemsPerCall; i++ {
		p := fmt.Sprintf("/mnt/user/fotos/%02d.jpg", i)
		e.write(p, "x")
		paths = append(paths, p)
	}
	key, _ := e.key("anna", true)
	if _, code := e.tool(e.client(key), "move_to_trash", map[string]any{"paths": paths}); code == "" {
		t.Fatalf("%d entries went through", len(paths))
	}
	if !e.exists("/mnt/user/fotos/00.jpg") {
		t.Fatal("an entry went")
	}
}

func TestAFolderWithTooManyFilesIsLeftAlone(t *testing.T) {
	e := newEnv(t)
	dir := e.real("/mnt/user/fotos/viele")
	if err := os.MkdirAll(dir, 0o777); err != nil {
		t.Fatal(err)
	}
	for i := 0; i <= filesPerCall; i++ {
		if err := os.WriteFile(filepath.Join(dir, fmt.Sprintf("%05d", i)), nil, 0o666); err != nil {
			t.Fatal(err)
		}
	}
	key, _ := e.key("anna", true)
	s := e.client(key)
	if _, code := e.tool(s, "move_to_trash", map[string]any{"paths": []any{"/mnt/user/fotos/viele"}}); code != "too_big" {
		t.Fatalf("trashing %d files: %q", filesPerCall+1, code)
	}
	if _, code := e.tool(s, "copy_items", map[string]any{"sources": []any{"/mnt/user/fotos/viele"}, "destination": "/mnt/user/filme"}); code != "too_big" {
		t.Fatalf("copying %d files: %q", filesPerCall+1, code)
	}
	if !e.exists("/mnt/user/fotos/viele/00000") || e.exists("/mnt/user/filme/viele") {
		t.Fatal("something changed")
	}
}

func TestAKeyChangesAtMostTwoHundredEntriesAnHour(t *testing.T) {
	e := newEnv(t)
	key, _ := e.key("anna", true)
	s := e.client(key)
	for i := 0; i < itemsPerHour; i++ {
		if _, code := e.tool(s, "create_folder", map[string]any{"path": fmt.Sprintf("/mnt/user/fotos/%03d", i)}); code != "" {
			t.Fatalf("folder %d: %q", i, code)
		}
		// The calls budget is per minute; the hour's budget is what this is about.
		e.advance(time.Second)
	}
	if _, code := e.tool(s, "create_folder", map[string]any{"path": "/mnt/user/fotos/zuviel"}); code != "hourly_limit" {
		t.Fatalf("one over the hour: %q", code)
	}
	e.advance(time.Hour)
	if _, code := e.tool(s, "create_folder", map[string]any{"path": "/mnt/user/fotos/spaeter"}); code != "" {
		t.Fatalf("an hour later: %q", code)
	}
}

func TestAFailedChangeCostsNothing(t *testing.T) {
	e := newEnv(t)
	key, _ := e.key("anna", true)
	s := e.client(key)
	e.tool(s, "create_folder", map[string]any{"path": "/mnt/user/archiv/nein"})
	out, _ := e.tool(s, "get_access", nil)
	if left := out["limits"].(map[string]any)["entriesLeftThisHour"]; left != float64(itemsPerHour) {
		t.Fatalf("after a refused change %v are left", left)
	}
}

func TestTheTopOfASourceTheAccountCannotSeeAnswersNotFound(t *testing.T) {
	e := newEnv(t)
	annaKey, _ := e.key("anna", true)
	s := e.client(annaKey)
	for _, p := range []string{"/mnt/disks/extern", "/mnt/disks"} {
		if _, code := e.tool(s, "move_to_trash", map[string]any{"paths": []any{p}}); code != "not_found" {
			t.Fatalf("anna trashing %s: %q", p, code)
		}
		if _, code := e.tool(s, "rename_item", map[string]any{"path": p, "newName": "x"}); code != "not_found" {
			t.Fatalf("anna renaming %s: %q", p, code)
		}
	}
	chefKey, _ := e.key("chef", true)
	if _, code := e.tool(e.client(chefKey), "move_to_trash", map[string]any{"paths": []any{"/mnt/disks/extern"}}); code != "source_root" {
		t.Fatalf("the admin trashing a source: %q", code)
	}
}

func TestACopyIsHeldToItsBytes(t *testing.T) {
	e := newEnv(t)
	big := e.real("/mnt/user/filme/gross.mkv")
	if err := os.WriteFile(big, nil, 0o666); err != nil {
		t.Fatal(err)
	}
	if err := os.Truncate(big, bytesPerCall+1); err != nil {
		t.Skip("no sparse files here:", err)
	}
	key, _ := e.key("anna", true)
	if _, code := e.tool(e.client(key), "copy_items", map[string]any{"sources": []any{"/mnt/user/filme/gross.mkv"}, "destination": "/mnt/user/fotos"}); code != "too_big" {
		t.Fatalf("copying %d bytes: %q", bytesPerCall+1, code)
	}
	if e.exists("/mnt/user/fotos/gross.mkv") {
		t.Fatal("the copy started")
	}
}

func TestATransferWithoutRoomIsRefused(t *testing.T) {
	for _, c := range []struct {
		name string
		pre  preflight
		want string
	}{
		{"room", preflight{Bytes: 10, Space: &space{Free: 20, Need: 10, Enough: true}}, ""},
		{"no room", preflight{Bytes: 30, Space: &space{Free: 20, Need: 30, Enough: false}}, "no_space"},
		{"a move on one disk", preflight{Bytes: bytesPerCall * 2, SameDevice: true}, ""},
		{"a copy too big", preflight{Bytes: bytesPerCall + 1}, "too_big"},
		{"too many files", preflight{Files: filesPerCall + 1}, "too_big"},
	} {
		got := ""
		if r := c.pre.refusal(); r != nil {
			got = codeOf(r)
		}
		if got != c.want {
			t.Fatalf("%s: %q, want %q", c.name, got, c.want)
		}
	}
}
