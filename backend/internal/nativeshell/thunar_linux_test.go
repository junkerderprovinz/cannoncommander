package nativeshell

import (
	"path/filepath"
	"slices"
	"strings"
	"testing"
)

const ucaXML = `<?xml version="1.0" encoding="UTF-8"?>
<actions>
<action>
	<icon>utilities-terminal</icon>
	<name>Open Terminal Here</name>
	<name xml:lang="de">Terminal hier öffnen</name>
	<unique-id>1-term</unique-id>
	<command>exo-open --working-directory %f --launch TerminalEmulator</command>
	<patterns>*</patterns>
	<startup-notify/>
	<directories/>
</action>
<action>
	<icon>gimp</icon>
	<name>Edit in GIMP</name>
	<submenu>Pictures</submenu>
	<unique-id>2-gimp</unique-id>
	<command>gimp %F</command>
	<patterns>*.png;*.JPG</patterns>
	<image-files/>
</action>
<action>
	<name>Edit as text</name>
	<unique-id>3-text</unique-id>
	<command>mousepad %F</command>
	<patterns>*</patterns>
	<text-files/>
</action>
<action>
	<name>Anything else</name>
	<unique-id>4-other</unique-id>
	<command>xdg-open %f</command>
	<patterns></patterns>
	<other-files/>
</action>
<action>
	<name>Compare</name>
	<unique-id>5-compare</unique-id>
	<command>meld %F</command>
	<range>2-*</range>
	<patterns>*</patterns>
	<directories/><audio-files/><image-files/><other-files/><text-files/><video-files/>
</action>
<action>
	<name>Play</name>
	<unique-id>6-play</unique-id>
	<command>mpv %F</command>
	<patterns>?.mp3;*.mp4</patterns>
	<audio-files/><video-files/>
</action>
<action>
	<name>Untyped</name>
	<unique-id>7-none</unique-id>
	<command>x %f</command>
	<patterns>*</patterns>
</action>
</actions>
`

func TestAThunarActionFitsByPatternTypeAndRange(t *testing.T) {
	f := newFixture(t)
	f.write(filepath.Join(f.config, "Thunar", "uca.xml"), ucaXML)
	k := f.desk()
	cases := []struct {
		files []string
		want  string
	}{
		{[]string{"photos/"}, "1-term"},
		{[]string{"a.png"}, "2-gimp"},
		{[]string{"B.jpg"}, "2-gimp"},
		{[]string{"notes.txt"}, "3-text"},
		{[]string{"run.sh"}, "3-text"},
		{[]string{"a.svg"}, "3-text"},
		{[]string{"doc.pdf"}, "4-other"},
		{[]string{"a.mp3", "long.mp3"}, "5-compare"},
		{[]string{"a.mp3", "c.mp4"}, "5-compare 6-play"},
		{[]string{"a.png", "photos/"}, "5-compare"},
	}
	for _, c := range cases {
		var got []string
		for _, a := range k.thunar(f.selection(k, c.files...)) {
			got = append(got, strings.TrimPrefix(a.Key, "thunar:"))
		}
		slices.Sort(got)
		if s := strings.Join(got, " "); s != c.want {
			t.Errorf("%v: got %q, want %q", c.files, s, c.want)
		}
	}
	all := k.thunar(f.selection(k, "photos/"))
	if all[0].Label != "Terminal hier öffnen" || all[0].icon != "utilities-terminal" {
		t.Errorf("got %+v", all[0])
	}
	if got := k.thunar(f.selection(k, "x.png")); len(got) != 1 || got[0].Group != "Pictures" || got[0].Label != "Edit in GIMP" {
		t.Errorf("got %+v", got)
	}
}

func TestOnlyTheFirstUcaFileCounts(t *testing.T) {
	f := newFixture(t)
	system := filepath.Join(f.home, "etc", "xdg")
	f.write(filepath.Join(system, "Thunar", "uca.xml"), ucaXML)
	k := f.desk()
	k.Config = append(k.Config, system)
	if got := k.thunar(f.selection(k, "a.png")); len(got) != 1 {
		t.Fatalf("the system's file: got %s", keys(got))
	}
	f.write(filepath.Join(f.config, "Thunar", "uca.xml"), "<actions></actions>")
	if got := k.thunar(f.selection(k, "a.png")); len(got) != 0 {
		t.Fatalf("the user's file: got %s", keys(got))
	}
}

func TestAThunarCommandQuotesEachFileLikeThunar(t *testing.T) {
	f := newFixture(t)
	k := f.desk()
	sel := f.selection(k, "a;rm -rf ~", "it's")
	a, b := filepath.Join(f.work, "a;rm -rf ~"), filepath.Join(f.work, "it's")
	cases := []struct {
		command string
		want    []string
	}{
		{"tool %f", []string{"tool", a}},
		{"tool %F", []string{"tool", a, b}},
		{"tool %n %N", []string{"tool", "a;rm -rf ~", "a;rm -rf ~", "it's"}},
		{"tool %d %D", []string{"tool", f.work, f.work, f.work}},
		{"tool %u", []string{"tool", "file://" + f.work + "/a%3Brm%20-rf%20~"}},
		{`sh -c "cat %F > /dev/null"`, []string{"sh", "-c", "cat '" + a + "' '" + f.work + `/it'\''s' > /dev/null`}},
		{"tool --x=%f%%", []string{"tool", "--x=" + a + "%"}},
	}
	for _, c := range cases {
		got, err := thunarCommand(c.command, sel)
		if err != nil || !slices.Equal(got, c.want) {
			t.Errorf("%s: got %q %v", c.command, got, err)
		}
	}
}

func TestARangeCountsTheFiles(t *testing.T) {
	cases := []struct {
		r    string
		n    int
		want bool
	}{
		{"", 7, true}, {"1", 1, true}, {"1", 2, false}, {"2-4", 3, true}, {"2-4", 5, false},
		{"2-*", 9, true}, {"*-2", 1, true}, {"*-2", 3, false}, {"x", 1, false},
	}
	for _, c := range cases {
		if got := inRange(c.r, c.n); got != c.want {
			t.Errorf("%q with %d: got %v", c.r, c.n, got)
		}
	}
}

func TestAPatternMatchesLikeGLib(t *testing.T) {
	cases := []struct {
		pattern, name string
		want          bool
	}{
		{"*", "", true}, {"*.png", "a.png", true}, {"*.png", "a.png.txt", false},
		{"?.mp3", "a.mp3", true}, {"?.mp3", "ab.mp3", false}, {"?.txt", "ä.txt", true},
		{"a*b*c", "aXbYbZc", true}, {"a*b*c", "aXbYbZ", false}, {"[ab].txt", "a.txt", false}, {"[ab].txt", "[ab].txt", true},
	}
	for _, c := range cases {
		if got := globMatch(c.pattern, c.name); got != c.want {
			t.Errorf("%q on %q: got %v", c.pattern, c.name, got)
		}
	}
}
