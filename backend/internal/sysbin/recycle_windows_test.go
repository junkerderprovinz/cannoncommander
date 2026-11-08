package sysbin

import (
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// realBin works on the Recycle Bin of this computer, so it only runs on
// request, and it removes whatever entry from its folder is still there.
func realBin(t *testing.T) (Bin, string) {
	t.Helper()
	if os.Getenv("CC_TEST_RECYCLE_BIN") == "" {
		t.Skip("uses the Recycle Bin of this computer; set CC_TEST_RECYCLE_BIN=1")
	}
	b, err := Open()
	if err != nil {
		t.Fatal(err)
	}
	dir := t.TempDir()
	if _, ok := hasBin(dir); !ok {
		t.Skip("the temporary folder is on a drive without a Recycle Bin")
	}
	t.Cleanup(func() {
		list, _ := b.List()
		for _, e := range list {
			if strings.HasPrefix(strings.ToLower(e.Origin), strings.ToLower(dir)) {
				os.RemoveAll(e.Path)
				b.Forget(e.ID)
			}
		}
	})
	return b, dir
}

func TestRecycledFilesComeBack(t *testing.T) {
	b, dir := realBin(t)
	p := filepath.Join(dir, "Strand 2025 ü.txt")
	if err := os.WriteFile(p, []byte("sand"), 0o644); err != nil {
		t.Fatal(err)
	}
	e, err := b.Put(p)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.EqualFold(e.Origin, p) || e.Name != "Strand 2025 ü.txt" || e.Size != 4 || time.Since(e.Deleted) > time.Minute {
		t.Fatalf("entry %+v", e)
	}
	if _, err := os.Lstat(p); !errors.Is(err, fs.ErrNotExist) {
		t.Fatal("the file is still there")
	}
	list, _ := b.List()
	found := false
	for _, x := range list {
		found = found || x.ID == e.ID
	}
	if !found {
		t.Fatal("the Recycle Bin does not list it")
	}
	if err := b.Restore(e.ID, p); err != nil {
		t.Fatal(err)
	}
	if body, err := os.ReadFile(p); err != nil || string(body) != "sand" {
		t.Fatalf("restored %q, %v", body, err)
	}
	if _, err := b.Get(e.ID); !errors.Is(err, ErrNotFound) {
		t.Fatalf("still listed: %v", err)
	}
}

func TestRecycledFoldersComeBackWhole(t *testing.T) {
	b, dir := realBin(t)
	p := filepath.Join(dir, "Urlaub")
	os.MkdirAll(filepath.Join(p, "tief"), 0o755)
	os.WriteFile(filepath.Join(p, "tief", "a.jpg"), []byte("12345"), 0o644)
	e, err := b.Put(p)
	if err != nil {
		t.Fatal(err)
	}
	if !e.Dir {
		t.Fatalf("entry %+v", e)
	}
	if err := b.Restore(e.ID, p); err != nil {
		t.Fatal(err)
	}
	if body, _ := os.ReadFile(filepath.Join(p, "tief", "a.jpg")); string(body) != "12345" {
		t.Fatal("the folder came back without its file")
	}
}

func TestAdoptedEntriesShowTheirOldPlace(t *testing.T) {
	b, dir := realBin(t)
	held := filepath.Join(dir, ".cc-trash", "x1", "brief.txt")
	os.MkdirAll(filepath.Dir(held), 0o755)
	os.WriteFile(held, []byte("post"), 0o644)
	origin := filepath.Join(dir, "Briefe", "brief.txt")
	at := time.Date(2026, 10, 3, 9, 15, 0, 0, time.UTC)
	e, err := b.Adopt(held, origin, at)
	if err != nil {
		t.Fatal(err)
	}
	got, err := b.Get(e.ID)
	if err != nil || got.Origin != origin || !got.Deleted.Equal(at) {
		t.Fatalf("got %+v, %v", got, err)
	}
	if err := os.MkdirAll(filepath.Dir(origin), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := b.Restore(e.ID, origin); err != nil {
		t.Fatal(err)
	}
}

func TestNetworkDrivesHaveNoBin(t *testing.T) {
	if _, ok := hasBin(`\\server\share\a.txt`); ok {
		t.Fatal("a UNC path has a Recycle Bin")
	}
	b := &recycleBin{sid: "S-1-0-0"}
	if _, err := b.Put(`\\server\share\a.txt`); !errors.Is(err, ErrNoBin) {
		t.Fatalf("got %v", err)
	}
}
