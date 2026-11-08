//go:build linux

package sysbin

import (
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"testing"
	"time"
)

func homeBin(t *testing.T, mounts ...string) *freedesktop {
	t.Helper()
	return &freedesktop{
		home:   filepath.Join(t.TempDir(), "Trash"),
		uid:    os.Getuid(),
		mounts: func() ([]string, error) { return mounts, nil },
	}
}

func write(t *testing.T, p, body string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(p, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
}

func TestAFileGoesToTheHomeTrashAndComesBack(t *testing.T) {
	b := homeBin(t)
	p := filepath.Join(t.TempDir(), "Bilder", "Strand 2025.jpg")
	write(t, p, "sand")
	e, err := b.Put(p)
	if err != nil {
		t.Fatal(err)
	}
	if e.Origin != p || e.Name != "Strand 2025.jpg" || e.Dir || e.Size != 4 || time.Since(e.Deleted) > time.Minute {
		t.Fatalf("entry %+v", e)
	}
	if _, err := os.Lstat(p); !errors.Is(err, fs.ErrNotExist) {
		t.Fatal("the file is still there")
	}
	if e.Path != filepath.Join(b.home, "files", "Strand 2025.jpg") {
		t.Fatalf("lies at %s", e.Path)
	}
	info, err := os.ReadFile(filepath.Join(b.home, "info", "Strand 2025.jpg.trashinfo"))
	if err != nil || !strings.Contains(string(info), "Path="+strings.ReplaceAll(p, " ", "%20")+"\n") {
		t.Fatalf("info %q, %v", info, err)
	}
	list, err := b.List()
	if err != nil || len(list) != 1 || list[0].ID != e.ID || !list[0].Deleted.Equal(e.Deleted) {
		t.Fatalf("list %+v, %v", list, err)
	}
	if err := b.Restore(e.ID, p); err != nil {
		t.Fatal(err)
	}
	if body, err := os.ReadFile(p); err != nil || string(body) != "sand" {
		t.Fatalf("restored %q, %v", body, err)
	}
	if list, _ := b.List(); len(list) != 0 {
		t.Fatalf("left in the trash: %+v", list)
	}
}

func TestTheSameNameTwiceKeepsBoth(t *testing.T) {
	b := homeBin(t)
	dir := t.TempDir()
	var ids []string
	for _, body := range []string{"eins", "zwei", "drei"} {
		p := filepath.Join(dir, "notiz.txt")
		write(t, p, body)
		e, err := b.Put(p)
		if err != nil {
			t.Fatal(err)
		}
		ids = append(ids, e.ID)
	}
	if ids[0] == ids[1] || ids[1] == ids[2] {
		t.Fatalf("ids %v", ids)
	}
	e, err := b.Get(ids[1])
	if err != nil || e.Name != "notiz.txt" || filepath.Base(e.Path) == "notiz.txt" {
		t.Fatalf("second %+v, %v", e, err)
	}
	if body, _ := os.ReadFile(e.Path); string(body) != "zwei" {
		t.Fatalf("second holds %q", body)
	}
}

func TestAFolderCountsWhatIsInside(t *testing.T) {
	b := homeBin(t)
	p := filepath.Join(t.TempDir(), "Urlaub")
	write(t, filepath.Join(p, "a.jpg"), "12345")
	write(t, filepath.Join(p, "tief", "b.jpg"), "123")
	e, err := b.Put(p)
	if err != nil {
		t.Fatal(err)
	}
	if !e.Dir || e.Size != 8 {
		t.Fatalf("entry %+v", e)
	}
}

func TestAdoptKeepsWhereAndWhen(t *testing.T) {
	b := homeBin(t)
	dir := t.TempDir()
	held := filepath.Join(dir, ".cc-trash", "x1", "brief.odt")
	write(t, held, "post")
	at := time.Date(2026, 10, 3, 9, 15, 0, 0, time.Local)
	e, err := b.Adopt(held, filepath.Join(dir, "Briefe", "brief.odt"), at)
	if err != nil {
		t.Fatal(err)
	}
	got, err := b.Get(e.ID)
	if err != nil || got.Origin != filepath.Join(dir, "Briefe", "brief.odt") || !got.Deleted.Equal(at) {
		t.Fatalf("got %+v, %v", got, err)
	}
}

func TestRestoreLeavesATakenNameAlone(t *testing.T) {
	b := homeBin(t)
	p := filepath.Join(t.TempDir(), "a.txt")
	write(t, p, "alt")
	e, err := b.Put(p)
	if err != nil {
		t.Fatal(err)
	}
	write(t, p, "neu")
	if err := b.Restore(e.ID, p); !errors.Is(err, fs.ErrExist) {
		t.Fatalf("restore over a file: %v", err)
	}
	if body, _ := os.ReadFile(p); string(body) != "neu" {
		t.Fatal("the new file was replaced")
	}
}

func TestForgetDropsTheRecord(t *testing.T) {
	b := homeBin(t)
	p := filepath.Join(t.TempDir(), "a.txt")
	write(t, p, "x")
	e, err := b.Put(p)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.RemoveAll(e.Path); err != nil {
		t.Fatal(err)
	}
	if err := b.Forget(e.ID); err != nil {
		t.Fatal(err)
	}
	if _, err := b.Get(e.ID); !errors.Is(err, ErrNotFound) {
		t.Fatalf("still found: %v", err)
	}
}

func TestEntriesWithoutTheirFilesAreLeftOut(t *testing.T) {
	b := homeBin(t)
	p := filepath.Join(t.TempDir(), "a.txt")
	write(t, p, "x")
	e, err := b.Put(p)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Remove(e.Path); err != nil {
		t.Fatal(err)
	}
	if list, _ := b.List(); len(list) != 0 {
		t.Fatalf("list %+v", list)
	}
}

func TestForgedIDsFindNothing(t *testing.T) {
	b := homeBin(t)
	write(t, filepath.Join(b.home, "info", "a.trashinfo"), "[Trash Info]\nPath=/a\n")
	for _, id := range []string{makeID("h", "../../x"), makeID("ffffffff", "a"), makeID("h", "b"), "h", ""} {
		if _, err := b.Get(id); !errors.Is(err, ErrNotFound) {
			t.Errorf("%q: %v", id, err)
		}
	}
}

// otherVolume returns a writable folder on another file system than the
// temporary folder, such as /dev/shm in a container, or skips.
func otherVolume(t *testing.T) string {
	t.Helper()
	tmp, err := devOf(os.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	for _, top := range []string{"/dev/shm", "/run/user/" + strconv.Itoa(os.Getuid())} {
		if dev, err := devOf(top); err == nil && dev != tmp && syscall.Access(top, 2) == nil {
			return top
		}
	}
	t.Skip("no writable file system besides the temporary folder")
	return ""
}

func TestAnotherVolumeGetsATrashAtItsTop(t *testing.T) {
	top := otherVolume(t)
	bin := filepath.Join(top, ".Trash-"+strconv.Itoa(os.Getuid()))
	if _, err := os.Lstat(bin); err == nil {
		t.Skip(bin + " exists already")
	}
	t.Cleanup(func() { os.RemoveAll(bin) })
	dir, err := os.MkdirTemp(top, "cc-sysbin-")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { os.RemoveAll(dir) })
	b := homeBin(t, top)
	p := filepath.Join(dir, "film.mkv")
	write(t, p, "bild")
	e, err := b.Put(p)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(e.Path, bin+"/files/") {
		t.Fatalf("lies at %s", e.Path)
	}
	info, _ := os.ReadFile(filepath.Join(bin, "info", "film.mkv.trashinfo"))
	if !strings.Contains(string(info), "Path="+filepath.Base(dir)+"/film.mkv\n") {
		t.Fatalf("the path is not relative to the top: %q", info)
	}
	if st, _ := os.Stat(bin); st.Mode().Perm() != 0o700 {
		t.Fatalf("the trash is open to others: %v", st.Mode())
	}
	list, err := b.List()
	if err != nil || len(list) != 1 || list[0].Origin != p {
		t.Fatalf("list %+v, %v", list, err)
	}
	if err := b.Restore(e.ID, p); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(p); err != nil {
		t.Fatal(err)
	}
}

func TestASharedTrashNeedsTheStickyBit(t *testing.T) {
	top := otherVolume(t)
	shared := filepath.Join(top, ".Trash")
	own := filepath.Join(top, ".Trash-"+strconv.Itoa(os.Getuid()))
	for _, p := range []string{shared, own} {
		if _, err := os.Lstat(p); err == nil {
			t.Skip(p + " exists already")
		}
	}
	t.Cleanup(func() { os.RemoveAll(shared); os.RemoveAll(own) })
	dir, err := os.MkdirTemp(top, "cc-sysbin-")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { os.RemoveAll(dir) })
	b := homeBin(t, top)

	if err := os.Mkdir(shared, 0o777); err != nil {
		t.Fatal(err)
	}
	write(t, filepath.Join(dir, "a.txt"), "a")
	e, err := b.Put(filepath.Join(dir, "a.txt"))
	if err != nil || !strings.HasPrefix(e.Path, own+"/") {
		t.Fatalf("without the sticky bit: %s, %v", e.Path, err)
	}

	if err := os.Chmod(shared, 0o777|fs.ModeSticky); err != nil {
		t.Fatal(err)
	}
	write(t, filepath.Join(dir, "b.txt"), "b")
	e, err = b.Put(filepath.Join(dir, "b.txt"))
	if err != nil || !strings.HasPrefix(e.Path, filepath.Join(shared, strconv.Itoa(os.Getuid()))+"/") {
		t.Fatalf("with the sticky bit: %s, %v", e.Path, err)
	}
	if list, _ := b.List(); len(list) != 2 {
		t.Fatalf("list %+v", list)
	}
}

func TestMountsLeaveOutTheKernelsOwn(t *testing.T) {
	info := "22 1 0:21 / /proc rw,nosuid - proc proc rw\n" +
		"36 1 8:1 / / rw,relatime shared:1 - ext4 /dev/sda1 rw\n" +
		"40 36 8:17 / /media/anna/USB\040Stick rw,nosuid shared:7 - vfat /dev/sdb1 rw\n" +
		"broken line\n"
	got := parseMounts(info)
	if len(got) != 2 || got[0] != "/" || got[1] != "/media/anna/USB Stick" {
		t.Fatalf("got %q", got)
	}
}
