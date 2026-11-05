package fileops_test

import (
	"context"
	"database/sql"
	"encoding/json"
	"io"
	"net/http"
	"os"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/junkerderprovinz/cannoncommander/backend/internal/database/users"
	"github.com/junkerderprovinz/cannoncommander/backend/internal/fileops"
)

type conflictAnswer struct {
	Upload   fileops.Upload `json:"upload"`
	Conflict struct {
		Key    string `json:"key"`
		Target struct {
			Size int64 `json:"size"`
		} `json:"target"`
	} `json:"conflict"`
}

func (e *env) startUpload(user string, req fileops.UploadRequest) (fileops.Upload, int) {
	e.t.Helper()
	var up fileops.Upload
	code := e.call(user, "POST", "/uploads", req, &up)
	return up, code
}

// chunk sends data at offset and returns the status and the raw answer.
func (e *env) chunk(user, id string, offset int64, data string) (int, []byte) {
	e.t.Helper()
	req, _ := http.NewRequest("PUT", e.srv.URL+"/uploads/"+id, strings.NewReader(data))
	req.Header.Set("X-User", user)
	req.Header.Set("Upload-Offset", strconv.FormatInt(offset, 10))
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		e.t.Fatal(err)
	}
	defer resp.Body.Close()
	raw, _ := io.ReadAll(resp.Body)
	return resp.StatusCode, raw
}

func (e *env) upload(user, path, content string, rule string) fileops.Upload {
	e.t.Helper()
	up, code := e.startUpload(user, fileops.UploadRequest{Path: path, Size: int64(len(content)), Rule: rule})
	if code != http.StatusCreated {
		e.t.Fatalf("start %s: %d", path, code)
	}
	code, raw := e.chunk(user, up.ID, 0, content)
	if code != http.StatusOK {
		e.t.Fatalf("chunk %s: %d %s", path, code, raw)
	}
	var done fileops.Upload
	if err := json.Unmarshal(raw, &done); err != nil {
		e.t.Fatal(err)
	}
	return done
}

func TestUploadInChunksLandsUnderItsNameWithItsTime(t *testing.T) {
	e := newEnv(t)
	when := time.Date(2024, 8, 1, 10, 0, 0, 0, time.UTC)
	up, code := e.startUpload("anna", fileops.UploadRequest{Path: "/mnt/user/fotos/strand.txt", Size: 11, MTime: &when})
	if code != http.StatusCreated || up.State != fileops.UploadOpen || up.Offset != 0 || up.Chunk == 0 {
		t.Fatalf("start: %d %+v", code, up)
	}
	if e.gone("/mnt/user/fotos/.strand.txt.cc-part") {
		t.Fatal("no part file while the upload runs")
	}
	if !e.gone("/mnt/user/fotos/strand.txt") {
		t.Fatal("the file has its name before it is complete")
	}
	if code, raw := e.chunk("anna", up.ID, 0, "sonne "); code != http.StatusOK || !strings.Contains(string(raw), `"offset":6`) {
		t.Fatalf("first chunk: %d %s", code, raw)
	}
	code, raw := e.chunk("anna", up.ID, 6, "strand")
	if code != http.StatusRequestEntityTooLarge {
		t.Fatalf("a chunk past the end: %d %s", code, raw)
	}
	code, raw = e.chunk("anna", up.ID, 6, "meer!")
	var done fileops.Upload
	_ = json.Unmarshal(raw, &done)
	if code != http.StatusOK || done.State != fileops.UploadDone || done.Path != "/mnt/user/fotos/strand.txt" {
		t.Fatalf("last chunk: %d %s", code, raw)
	}
	if e.read("/mnt/user/fotos/strand.txt") != "sonne meer!" || !e.gone("/mnt/user/fotos/.strand.txt.cc-part") {
		t.Fatal("the file did not land whole")
	}
	if info, _ := os.Stat(e.real("/mnt/user/fotos/strand.txt")); !info.ModTime().Equal(when) {
		t.Fatalf("mtime %s", info.ModTime())
	}
	if code := e.call("anna", "GET", "/uploads/"+up.ID, nil, nil); code != http.StatusNotFound {
		t.Fatalf("a finished upload is still there: %d", code)
	}
}

func TestUploadResumesWhereItStands(t *testing.T) {
	e := newEnv(t)
	req := fileops.UploadRequest{Path: "/mnt/user/fotos/film.bin", Size: 8}
	up, _ := e.startUpload("anna", req)
	e.chunk("anna", up.ID, 0, "abc")

	again, code := e.startUpload("anna", req)
	if code != http.StatusOK || again.ID != up.ID || again.Offset != 3 {
		t.Fatalf("starting again: %d %+v", code, again)
	}
	var state fileops.Upload
	if code := e.call("anna", "GET", "/uploads/"+up.ID, nil, &state); code != http.StatusOK || state.Offset != 3 {
		t.Fatalf("state: %d %+v", code, state)
	}
	code, raw := e.chunk("anna", up.ID, 0, "abc")
	if code != http.StatusConflict || !strings.Contains(string(raw), `"offset":3`) {
		t.Fatalf("a chunk at the wrong offset: %d %s", code, raw)
	}
	if code, _ := e.chunk("anna", up.ID, 3, "defgh"); code != http.StatusOK || e.read("/mnt/user/fotos/film.bin") != "abcdefgh" {
		t.Fatalf("resumed upload: %d", code)
	}
}

func TestUploadOfAnotherSizeStartsOver(t *testing.T) {
	e := newEnv(t)
	up, _ := e.startUpload("anna", fileops.UploadRequest{Path: "/mnt/user/fotos/a.txt", Size: 8})
	e.chunk("anna", up.ID, 0, "abc")
	fresh, code := e.startUpload("anna", fileops.UploadRequest{Path: "/mnt/user/fotos/a.txt", Size: 2})
	if code != http.StatusCreated || fresh.ID == up.ID || fresh.Offset != 0 {
		t.Fatalf("another file of the same name: %d %+v", code, fresh)
	}
	if code := e.call("anna", "GET", "/uploads/"+up.ID, nil, nil); code != http.StatusNotFound {
		t.Fatalf("the old upload lives on: %d", code)
	}
}

func TestUploadAsksBeforeTouchingAnExistingFile(t *testing.T) {
	e := newEnv(t)
	e.write("/mnt/user/fotos/a.txt", "alt", old)
	req := fileops.UploadRequest{Path: "/mnt/user/fotos/a.txt", Size: 3}
	var answer conflictAnswer
	raw, _ := json.Marshal(req)
	httpReq, _ := http.NewRequest("POST", e.srv.URL+"/uploads", strings.NewReader(string(raw)))
	httpReq.Header.Set("X-User", "anna")
	resp, err := http.DefaultClient.Do(httpReq)
	if err != nil {
		t.Fatal(err)
	}
	body, _ := io.ReadAll(resp.Body)
	resp.Body.Close()
	if resp.StatusCode != http.StatusConflict || json.Unmarshal(body, &answer) != nil || answer.Conflict.Key != "a.txt" || answer.Conflict.Target.Size != 3 {
		t.Fatalf("conflict: %d %s", resp.StatusCode, body)
	}

	req.Rule = "both"
	up, code := e.startUpload("anna", req)
	if code != http.StatusCreated {
		t.Fatalf("keep both: %d", code)
	}
	if code, raw := e.chunk("anna", up.ID, 0, "neu"); code != http.StatusOK || !strings.Contains(string(raw), `a (1).txt`) {
		t.Fatalf("keep both: %d %s", code, raw)
	}
	if e.read("/mnt/user/fotos/a.txt") != "alt" || e.read("/mnt/user/fotos/a (1).txt") != "neu" {
		t.Fatal("both files should be there")
	}
}

func TestUploadReplacesIntoTheTrash(t *testing.T) {
	e := newEnv(t)
	e.write("/mnt/user/fotos/a.txt", "alt", old)
	e.upload("anna", "/mnt/user/fotos/a.txt", "neu", "replace")
	if e.read("/mnt/user/fotos/a.txt") != "neu" {
		t.Fatal("not replaced")
	}
	if items := e.trashItems(); len(items) != 1 || items[0].Name != "a.txt" {
		t.Fatalf("trash %+v", items)
	}
}

func TestUploadRuleSkipAndNewer(t *testing.T) {
	e := newEnv(t)
	e.write("/mnt/user/fotos/a.txt", "alt", old)
	up, code := e.startUpload("anna", fileops.UploadRequest{Path: "/mnt/user/fotos/a.txt", Size: 3, Rule: "skip"})
	if code != http.StatusOK || up.State != fileops.UploadSkipped {
		t.Fatalf("skip: %d %+v", code, up)
	}

	older := old.Add(-time.Hour)
	up, _ = e.startUpload("anna", fileops.UploadRequest{Path: "/mnt/user/fotos/a.txt", Size: 3, MTime: &older, Rule: "newer"})
	if _, raw := e.chunk("anna", up.ID, 0, "neu"); !strings.Contains(string(raw), `"state":"skipped"`) || e.read("/mnt/user/fotos/a.txt") != "alt" {
		t.Fatalf("an older upload replaced the file: %s", raw)
	}
	if !e.gone("/mnt/user/fotos/.a.txt.cc-part") {
		t.Fatal("the skipped part file stays")
	}
	newer := old.Add(time.Hour)
	up, _ = e.startUpload("anna", fileops.UploadRequest{Path: "/mnt/user/fotos/a.txt", Size: 3, MTime: &newer, Rule: "newer"})
	if _, raw := e.chunk("anna", up.ID, 0, "neu"); !strings.Contains(string(raw), `"state":"done"`) || e.read("/mnt/user/fotos/a.txt") != "neu" {
		t.Fatalf("a newer upload did not replace the file: %s", raw)
	}
}

func TestUploadCreatesTheFoldersOfADroppedFolder(t *testing.T) {
	e := newEnv(t)
	done := e.upload("anna", "/mnt/user/fotos/Urlaub/2025/a.txt", "sonne", "")
	if done.Path != "/mnt/user/fotos/Urlaub/2025/a.txt" || e.read("/mnt/user/fotos/Urlaub/2025/a.txt") != "sonne" {
		t.Fatalf("upload into new folders: %+v", done)
	}
	e.write("/mnt/user/fotos/datei", "x", old)
	if _, code := e.startUpload("anna", fileops.UploadRequest{Path: "/mnt/user/fotos/datei/a.txt", Size: 1}); code != http.StatusConflict {
		t.Fatalf("a file in the way of a folder: %d", code)
	}
}

func TestEmptyUploadIsDoneAtOnce(t *testing.T) {
	e := newEnv(t)
	up, code := e.startUpload("anna", fileops.UploadRequest{Path: "/mnt/user/fotos/leer.txt", Size: 0})
	if code != http.StatusOK || up.State != fileops.UploadDone || e.read("/mnt/user/fotos/leer.txt") != "" {
		t.Fatalf("empty upload: %d %+v", code, up)
	}
}

func TestCancelledUploadLeavesNothing(t *testing.T) {
	e := newEnv(t)
	up, _ := e.startUpload("anna", fileops.UploadRequest{Path: "/mnt/user/fotos/a.txt", Size: 5})
	e.chunk("anna", up.ID, 0, "ab")
	if code := e.call("anna", "DELETE", "/uploads/"+up.ID, nil, nil); code != http.StatusNoContent {
		t.Fatalf("cancel: %d", code)
	}
	if !e.gone("/mnt/user/fotos/.a.txt.cc-part") || !e.gone("/mnt/user/fotos/a.txt") {
		t.Fatal("the cancelled upload left files")
	}
}

func TestUploadsBelongToTheirUserAndNeedTheCreateRight(t *testing.T) {
	e := newEnv(t)
	if _, code := e.startUpload("gast", fileops.UploadRequest{Path: "/mnt/user/fotos/a.txt", Size: 1}); code != http.StatusForbidden {
		t.Fatalf("upload without the create right: %d", code)
	}
	up, _ := e.startUpload("anna", fileops.UploadRequest{Path: "/mnt/user/fotos/a.txt", Size: 1})
	if code := e.call("jdp", "GET", "/uploads/"+up.ID, nil, nil); code != http.StatusNotFound {
		t.Fatalf("someone else sees the upload: %d", code)
	}
	if _, code := e.startUpload("jdp", fileops.UploadRequest{Path: "/mnt/user/fotos/a.txt", Size: 1}); code != http.StatusConflict {
		t.Fatalf("two users on one target: %d", code)
	}
	for _, p := range []string{"/mnt/user/../etc/a", "/mnt/user"} {
		if _, code := e.startUpload("anna", fileops.UploadRequest{Path: p, Size: 1}); code != http.StatusBadRequest {
			t.Fatalf("upload to %s: %d", p, code)
		}
	}
	if _, code := e.startUpload("anna", fileops.UploadRequest{Path: "/mnt/user/fotos/.a.cc-part", Size: 1}); code != http.StatusForbidden {
		t.Fatalf("upload to a part file: %d", code)
	}
}

func TestConcurrentChunksDoNotMixUp(t *testing.T) {
	e := newEnv(t)
	up, _ := e.startUpload("anna", fileops.UploadRequest{Path: "/mnt/user/fotos/a.txt", Size: 4})
	var wg sync.WaitGroup
	codes := make([]int, 8)
	for i := range codes {
		wg.Add(1)
		go func() {
			defer wg.Done()
			codes[i], _ = e.chunk("anna", up.ID, 0, "abcd")
		}()
	}
	wg.Wait()
	ok := 0
	for _, c := range codes {
		if c == http.StatusOK {
			ok++
		}
	}
	if ok != 1 || e.read("/mnt/user/fotos/a.txt") != "abcd" {
		t.Fatalf("codes %v", codes)
	}
}

func TestAClientLeavingInTheLastStepLeavesTheUploadOpen(t *testing.T) {
	e := newEnv(t)
	up, _ := e.startUpload("anna", fileops.UploadRequest{Path: "/mnt/user/fotos/a.txt", Size: 4})
	u := &users.User{BackendScopes: []users.BackendScope{{Path: e.root, Scope: "/", Permissions: full}}}
	u.Username = "anna"
	ctx, leave := context.WithCancel(t.Context())
	defer leave()
	_, err := fileops.For(e.deps).UploadChunk(ctx, u, "", up.ID, 0, strings.NewReader("abcd"), &fileops.Landing{
		Allow: func() error { return nil },
		Count: func(context.Context, *sql.Tx, int64) error {
			leave()
			return nil
		},
	})
	if err == nil {
		t.Fatal("the upload closed although its transaction ended with the client")
	}
	if _, err := os.Stat(e.real("/mnt/user/fotos/a.txt")); !os.IsNotExist(err) {
		t.Fatal("the file landed while its upload stayed open")
	}
	if code, raw := e.chunk("anna", up.ID, 4, ""); code != http.StatusOK {
		t.Fatalf("finishing: %d %s", code, raw)
	}
	if e.read("/mnt/user/fotos/a.txt") != "abcd" {
		t.Fatal("the content is wrong")
	}
}
