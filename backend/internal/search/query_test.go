package search

import (
	"math"
	"slices"
	"testing"
	"time"

	"github.com/junkerderprovinz/cannoncommander/backend/internal/fsindex"
	"github.com/junkerderprovinz/cannoncommander/backend/internal/meta"
)

var now = time.Date(2026, 11, 2, 9, 30, 0, 0, time.Local)

func entry(name string, size int64, mtime time.Time, values meta.Values) *row {
	return &row{Entry: fsindex.Entry{Path: "/mnt/user/x/" + name, Name: name, Size: size, MTime: mtime, Mode: 0o664}, At: "/mnt/user/x", meta: values}
}

func matches(t *testing.T, line string, r *row) bool {
	t.Helper()
	q := Parse(line, now)
	if len(q.Unknown) > 0 {
		t.Fatalf("%q: unknown %v", line, q.Unknown)
	}
	return q.matchName(r) && q.matchFields(r)
}

func TestNameWordsFollowTheListFilter(t *testing.T) {
	film := entry("Sintel (2010).mkv", 0, now, nil)
	cases := map[string]bool{
		"*.mkv": true, "*.MKV": true, "sintel": true, "SINTEL": true, "*.mp4": false,
		"*.mp4 sintel": true, "Sintel*": true, "intel*": false, "Sintel (????).mkv": true,
		`"(2010).mkv"`: true, "*.iso;*.mkv": true, "": true,
	}
	for line, want := range cases {
		if got := matches(t, line, film); got != want {
			t.Errorf("%q matches %v, want %v", line, got, want)
		}
	}
}

func TestFieldsCompareLikeThePrototype(t *testing.T) {
	uhd := entry("Film.mkv", 30<<30, now.Add(-3*day), meta.Values{
		"dimensions": "3840 × 2160", "vcodec": "HEVC", "duration": int64(7200), "year": int64(2010), "framerate": "23.976 fps",
	})
	song := entry("Queen - Bohemian.flac", 40<<20, now.Add(-400*day), meta.Values{"artist": "Queen", "duration": int64(355)})
	photo := entry("IMG_0001.jpg", 5<<20, now, meta.Values{
		"camera": "X100VI", "aperture": "f/2.8", "taken": time.Date(2026, 6, 21, 18, 30, 0, 0, time.Local).UnixMilli(), "place": "Wien",
	})
	cases := []struct {
		line string
		r    *row
		want bool
	}{
		{"breite:>=3840 codec:hevc", uhd, true},
		{"breite:>=3840", song, false},
		{"höhe:2160", uhd, true},
		{"codec:h.264", uhd, false},
		{"codec:!=h.264", uhd, true},
		{"codec:=hevc", uhd, true},
		{`interpret:"Queen"`, song, true},
		{"künstler:queen", song, true},
		{"kamera:X100", photo, true},
		{"ort:wien", photo, true},
		{"größe:>1GB", uhd, true},
		{"groesse:>1GB", song, false},
		{"Größe:<50", song, true},
		{"geändert:<7d", uhd, true},
		{"geändert:<7d", song, false},
		{"geändert:>1y", song, true},
		{"geändert:2026-10-30", uhd, true},
		{"datum:2026", uhd, true},
		{"datum:2025", song, true},
		{"modified:30.10.2026", uhd, true},
		{"aufgenommen:2026-06", photo, true},
		{"aufgenommen:<30d", photo, false},
		{"dauer:>90", uhd, true},
		{"dauer:>90", song, false},
		{"dauer:<6min", song, true},
		{"jahr:2010", uhd, true},
		{"jahr:>2000", uhd, true},
		{"bildrate:<24", uhd, true},
		{"bildrate:23,976", uhd, true},
		{"blende:f/2,8", photo, true},
		{"blende:=f/2.8", photo, true},
		{"endung:flac", song, true},
		{"art:musik", song, true},
		{"typ:audio", song, true},
		{"rechte:rw-rw-r", photo, true},
		{"oktal:664", photo, true},
		{"breite:", song, true},
	}
	for _, c := range cases {
		if got := matches(t, c.line, c.r); got != c.want {
			t.Errorf("%q on %s = %v, want %v", c.line, c.r.Name, got, c.want)
		}
	}
}

func TestUnknownFieldsAreNamed(t *testing.T) {
	q := Parse("*.mkv farbe:rot breite:>100 xyz:1", now)
	if !slices.Equal(q.Unknown, []string{"farbe", "xyz"}) {
		t.Errorf("unknown = %v", q.Unknown)
	}
	if !slices.Contains(q.Meta, "dimensions") {
		t.Errorf("meta = %v", q.Meta)
	}
}

func TestSizesReadLikeTheWindow(t *testing.T) {
	cases := map[string]float64{"300 KB": 300 * kb, "2,5 GB": 2.5 * gb, "1": mb, "12mb": 12 * mb, "1t": tb, "7 b": 7}
	for s, want := range cases {
		if got := Bytes(s); got != want {
			t.Errorf("Bytes(%q) = %v, want %v", s, got, want)
		}
	}
	if !math.IsNaN(Bytes("viel")) {
		t.Error("a word reads as a size")
	}
}

func TestTagsMatchByWord(t *testing.T) {
	r := entry("beleg.pdf", 1, now, nil)
	r.tagged = func(p, tag string) bool { return p == r.Path && tag == "steuer" }
	if !matches(t, "#Steuer", r) || matches(t, "#urlaub", r) {
		t.Error("tag matching is off")
	}
}
