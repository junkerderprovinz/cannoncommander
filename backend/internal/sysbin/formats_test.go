package sysbin

import (
	"encoding/binary"
	"strings"
	"testing"
	"time"
	"unicode/utf16"
)

func TestRecycleInfoRoundTrips(t *testing.T) {
	at := time.Date(2026, 11, 8, 14, 3, 7, 123400000, time.UTC)
	in := recycleInfo{Size: 4711, Deleted: at, Origin: `C:\Users\anna\Bilder\Strand 2025 ü.jpg`}
	out, err := parseRecycleInfo(in.bytes())
	if err != nil {
		t.Fatal(err)
	}
	if out.Size != in.Size || !out.Deleted.Equal(at) || out.Origin != in.Origin {
		t.Fatalf("got %+v", out)
	}
}

func TestRecycleInfoReadsTheOldLayout(t *testing.T) {
	b := make([]byte, 24+520)
	binary.LittleEndian.PutUint64(b, 1)
	binary.LittleEndian.PutUint64(b[8:], 12)
	binary.LittleEndian.PutUint64(b[16:], uint64(toFiletime(time.Date(2015, 3, 1, 8, 0, 0, 0, time.UTC))))
	for i, u := range utf16.Encode([]rune(`D:\alt.txt`)) {
		binary.LittleEndian.PutUint16(b[24+2*i:], u)
	}
	got, err := parseRecycleInfo(b)
	if err != nil {
		t.Fatal(err)
	}
	if got.Origin != `D:\alt.txt` || got.Size != 12 || got.Deleted.Year() != 2015 {
		t.Fatalf("got %+v", got)
	}
}

func TestRecycleInfoRefusesWhatIsCut(t *testing.T) {
	full := recycleInfo{Size: 1, Deleted: time.Now(), Origin: `C:\a.txt`}.bytes()
	for _, b := range [][]byte{nil, full[:20], full[:30], append([]byte{9}, full[1:]...)} {
		if _, err := parseRecycleInfo(b); err == nil {
			t.Errorf("%d bytes were read", len(b))
		}
	}
}

func TestTrashInfoRoundTrips(t *testing.T) {
	at := time.Date(2026, 11, 8, 22, 32, 8, 0, time.Local)
	for _, p := range []string{"/home/anna/Bilder/Strand 2025.jpg", "fotos/100%/ä#?.png", "/a/b"} {
		got, err := parseTrashInfo(formatTrashInfo(p, at))
		if err != nil {
			t.Fatal(err)
		}
		if got.Path != p || !got.Deleted.Equal(at) {
			t.Errorf("%s: got %+v", p, got)
		}
	}
}

func TestTrashInfoEscapesThePath(t *testing.T) {
	body := string(formatTrashInfo("/home/anna/a b%.txt", time.Date(2004, 8, 31, 22, 32, 8, 0, time.Local)))
	want := "[Trash Info]\nPath=/home/anna/a%20b%25.txt\nDeletionDate=2004-08-31T22:32:08\n"
	if body != want {
		t.Fatalf("got %q", body)
	}
}

func TestTrashInfoReadsWhatOtherProgramsWrite(t *testing.T) {
	body := "# made by hand\n[Other]\nPath=/wrong\n\n[Trash Info]\r\nDeletionDate=2004-08-31T22:32:08\r\nPath=foo/bar/meow.bmp\r\nX-Extra=1\r\n"
	got, err := parseTrashInfo([]byte(body))
	if err != nil {
		t.Fatal(err)
	}
	if got.Path != "foo/bar/meow.bmp" || got.Deleted.Hour() != 22 {
		t.Fatalf("got %+v", got)
	}
	for _, bad := range []string{"", "[Trash Info]\nDeletionDate=2004-08-31T22:32:08\n", "Path=/a\n"} {
		if _, err := parseTrashInfo([]byte(bad)); err == nil {
			t.Errorf("%q was read", bad)
		}
	}
	if got, err := parseTrashInfo([]byte("[Trash Info]\nPath=/a\nDeletionDate=gestern\n")); err != nil || !got.Deleted.IsZero() {
		t.Errorf("a broken date should leave the time empty: %+v, %v", got, err)
	}
}

func TestIDsKeepTheirNameInside(t *testing.T) {
	for _, name := range []string{"a.txt", "ä b#?%.png", "$IAB12CD.tar.gz"} {
		place, got, err := splitID(makeID("C", name))
		if err != nil || place != "C" || got != name {
			t.Errorf("%s: %s %s %v", name, place, got, err)
		}
	}
	for _, id := range []string{"", "C", ".612e747874", "C.zz", "C.2e2e", "C." + strings.Repeat("2f", 2), "C.612f62"} {
		if _, _, err := splitID(id); err == nil {
			t.Errorf("%q was taken", id)
		}
	}
}
