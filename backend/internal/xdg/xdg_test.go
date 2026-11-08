package xdg

import (
	"errors"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
)

// desk is a Linux desktop laid out in a temporary folder: a data folder of
// the user and one of the system, and the user's config folder.
type desk struct {
	t                  *testing.T
	home, user, system string
	config             string
	Desktop
}

func newDesk(t *testing.T, installed ...string) *desk {
	t.Helper()
	base := t.TempDir()
	d := &desk{t: t, home: filepath.Join(base, "home"), user: filepath.Join(base, "home", ".local", "share"), system: filepath.Join(base, "usr", "share"), config: filepath.Join(base, "home", ".config")}
	d.Desktop = Desktop{
		Data: []string{d.user, d.system}, Config: []string{d.config}, Cache: filepath.Join(d.home, ".cache"), Home: d.home, Langs: []string{"de_AT", "de"},
		Look: func(name string) (string, error) {
			if slices.Contains(installed, name) {
				return "/usr/bin/" + name, nil
			}
			return "", errors.New("not installed")
		},
	}
	d.write(filepath.Join(d.system, "mime", "globs2"), "# weight:type:glob\n50:text/plain:*.txt\n50:text/markdown:*.md\n50:image/png:*.png\n20:text/x-readme:README*\n50:text/x-makefile:Makefile:cs\n50:application/x-shellscript:*.sh\n")
	d.write(filepath.Join(d.system, "mime", "subclasses"), "text/markdown text/plain\napplication/x-shellscript text/plain\n")
	d.write(filepath.Join(d.system, "mime", "aliases"), "text/x-markdown text/markdown\n")
	d.write(filepath.Join(d.system, "mime", "generic-icons"), "application/x-shellscript:text-x-script\n")
	d.write(filepath.Join(d.system, "mime", "icons"), "text/markdown:text-markdown-own\n")
	return d
}

func (d *desk) write(file, content string) string {
	d.t.Helper()
	if err := os.MkdirAll(filepath.Dir(file), 0o777); err != nil {
		d.t.Fatal(err)
	}
	if err := os.WriteFile(file, []byte(content), 0o666); err != nil {
		d.t.Fatal(err)
	}
	return file
}

func TestTheDesktopIsReadFromTheEnvironment(t *testing.T) {
	env := map[string]string{"HOME": "/home/jdp", "XDG_DATA_DIRS": "/usr/share::/var/lib/flatpak/exports/share", "XDG_CURRENT_DESKTOP": "ubuntu:GNOME", "LANG": "de_AT.UTF-8"}
	d := FromEnv(func(k string) string { return env[k] }, nil)
	if !slices.Equal(d.Data, []string{"/home/jdp/.local/share", "/usr/share", "/var/lib/flatpak/exports/share"}) {
		t.Errorf("data %q", d.Data)
	}
	if !slices.Equal(d.Config, []string{"/home/jdp/.config", "/etc/xdg"}) || d.Cache != "/home/jdp/.cache" {
		t.Errorf("config %q cache %q", d.Config, d.Cache)
	}
	if !slices.Equal(d.Desktops, []string{"ubuntu", "gnome"}) || !slices.Equal(d.Langs, []string{"de_AT", "de"}) {
		t.Errorf("desktops %q langs %q", d.Desktops, d.Langs)
	}
	env = map[string]string{"HOME": "/root", "LANG": "C.UTF-8", "LANGUAGE": "de", "XDG_DATA_HOME": "/data", "XDG_CACHE_HOME": "/cache"}
	d = FromEnv(func(k string) string { return env[k] }, nil)
	if len(d.Langs) != 0 || d.Data[0] != "/data" || !slices.Equal(d.Data[1:], []string{"/usr/local/share", "/usr/share"}) || d.Cache != "/cache" {
		t.Errorf("langs %q data %q cache %q", d.Langs, d.Data, d.Cache)
	}
}

func TestTheLanguageListComesBeforeTheLocale(t *testing.T) {
	env := map[string]string{"LANG": "en_US.UTF-8", "LC_MESSAGES": "fr_FR.UTF-8", "LANGUAGE": "de_AT:pt_BR@latin"}
	d := FromEnv(func(k string) string { return env[k] }, nil)
	if !slices.Equal(d.Langs, []string{"de_AT", "de", "pt_BR", "pt", "fr_FR", "fr"}) {
		t.Fatalf("got %q", d.Langs)
	}
}

func TestAKeyFileKeepsItsGroupsAndEscapes(t *testing.T) {
	d := newDesk(t)
	file := d.write(filepath.Join(d.home, "a.desktop"), "# comment\n[Desktop Entry]\nName=Plain\nName[de]=Deutsch\nName[de_AT]=Österreichisch\\sauch\nMimeType=a/b;c\\;d; ;e/f\nOn=true\n\n[Desktop Action x]\nName = Spaced\n")
	kf, err := ReadKeyFile(file)
	if err != nil {
		t.Fatal(err)
	}
	g := kf["Desktop Entry"]
	if got := d.Localized(g, "Name"); got != "Österreichisch auch" {
		t.Errorf("name %q", got)
	}
	d.Langs = []string{"fr"}
	if got := d.Localized(g, "Name"); got != "Plain" {
		t.Errorf("fallback %q", got)
	}
	if got := g.List("MimeType"); !slices.Equal(got, []string{"a/b", "c;d", "e/f"}) {
		t.Errorf("list %q", got)
	}
	if !g.Bool("On") || g.Bool("Off") || kf["Desktop Action x"]["Name"] != "Spaced" {
		t.Errorf("group %v", kf)
	}
}

func TestATypeIsKnownByItsName(t *testing.T) {
	d := newDesk(t)
	db := d.MimeDB()
	cases := map[string]string{
		"notes.TXT":    "text/plain",
		"README.md":    "text/markdown",
		"README":       "text/x-readme",
		"Makefile":     "text/x-makefile",
		"makefile.txt": "text/plain",
	}
	for name, want := range cases {
		if got := db.TypeOf(filepath.Join(d.home, name)); got != want {
			t.Errorf("%s: got %s, want %s", name, got, want)
		}
	}
	if got := db.TypeByName("unknown.zzz"); got != "" {
		t.Errorf("unknown: got %s", got)
	}
}

func TestAFileWithoutAKnownNameIsTextOrData(t *testing.T) {
	d := newDesk(t)
	db := d.MimeDB()
	text := d.write(filepath.Join(d.home, "notes.zzz"), "just some words\n")
	data := d.write(filepath.Join(d.home, "blob.zzz"), "\x00\x01\x02\xff")
	if got := db.TypeOf(text); got != "text/plain" {
		t.Errorf("text: got %s", got)
	}
	if got := db.TypeOf(data); got != "application/octet-stream" {
		t.Errorf("data: got %s", got)
	}
}

func TestATypeKnowsItsParents(t *testing.T) {
	db := newDesk(t).MimeDB()
	if got := db.Lineage("application/x-shellscript"); !slices.Equal(got, []string{"application/x-shellscript", "text/plain"}) {
		t.Errorf("script %q", got)
	}
	if got := db.Lineage("text/x-c"); !slices.Equal(got, []string{"text/x-c", "text/plain"}) {
		t.Errorf("text %q", got)
	}
}

func TestATypesIconsGoFromItsOwnToTheGenericOne(t *testing.T) {
	db := newDesk(t).MimeDB()
	cases := map[string]string{
		"text/x-markdown":           "text-markdown-own text-markdown text-x-generic",
		"application/x-shellscript": "application-x-shellscript text-x-script application-x-generic",
		"image/png":                 "image-png image-x-generic",
	}
	for m, want := range cases {
		if got := strings.Join(db.IconNames(m), " "); got != want {
			t.Errorf("%s: got %s", m, got)
		}
	}
}

func TestAnEntryIsReadInTheUsersLanguage(t *testing.T) {
	d := newDesk(t, "present")
	d.write(filepath.Join(d.system, "applications", "kde4", "okular.desktop"), "[Desktop Entry]\nType=Application\nName=Okular\nName[de]=Okular\\sDE\nIcon=okular\nExec=okular %U\nMimeType=image/png;application/pdf;\nNoDisplay=true\n")
	d.write(filepath.Join(d.system, "applications", "gone.desktop"), "[Desktop Entry]\nType=Application\nName=Gone\nExec=gone\nTryExec=gone\n")
	d.write(filepath.Join(d.system, "applications", "here.desktop"), "[Desktop Entry]\nType=Application\nName=Here\nExec=present\nTryExec=present\n")
	all := d.Entries()
	if len(all) != 2 {
		t.Fatalf("got %v", all)
	}
	e := all["kde4-okular.desktop"]
	if e.Name != "Okular DE" || e.Icon != "okular" || e.Exec != "okular %U" || !slices.Equal(e.Mime, []string{"image/png", "application/pdf"}) || !e.NoDisplay {
		t.Fatalf("got %+v", e)
	}
}

func TestTheExecLineGetsTheFile(t *testing.T) {
	e := Entry{Name: "Text Editor", Icon: "gedit", File: "/usr/share/applications/gedit.desktop"}
	cases := []struct{ exec, want string }{
		{"gedit %U", "gedit|/home/jdp/a b.txt"},
		{"gedit", "gedit|/home/jdp/a b.txt"},
		{`"/opt/my app/run" --new-window %f`, "/opt/my app/run|--new-window|/home/jdp/a b.txt"},
		{`sh -c "echo \"%f\""`, `sh|-c|echo "/home/jdp/a b.txt"`},
		{"gedit %i --class=%c %k %F", "gedit|--icon|gedit|--class=Text Editor|/usr/share/applications/gedit.desktop|/home/jdp/a b.txt"},
		{"app --file=%f 100%%", "app|--file=/home/jdp/a b.txt|100%"},
		{"app %d %D %n %N %v %m %u", "app|/home/jdp/a b.txt"},
	}
	for _, c := range cases {
		e.Exec = c.exec
		got, err := Expand(e, []string{"/home/jdp/a b.txt"})
		if err != nil || len(got) != 1 || strings.Join(got[0], "|") != c.want {
			t.Errorf("%s: got %q %v", c.exec, got, err)
		}
	}
	for _, bad := range []string{`app "open`, "", "%d"} {
		e.Exec = bad
		if got, err := Expand(e, []string{"/x"}); err == nil {
			t.Errorf("%q: got %q", bad, got)
		}
	}
}

func TestALineForOneFileRunsOnceForEach(t *testing.T) {
	files := []string{"/a/1 x", "/a/2;rm -rf ~"}
	cases := []struct{ exec, want string }{
		{"conv %f", "conv|/a/1 x / conv|/a/2;rm -rf ~"},
		{"conv --in=%u", "conv|--in=/a/1 x / conv|--in=/a/2;rm -rf ~"},
		{"conv %F", "conv|/a/1 x|/a/2;rm -rf ~"},
		{"conv %f %U", "conv|/a/1 x|/a/1 x|/a/2;rm -rf ~"},
		{"conv", "conv|/a/1 x|/a/2;rm -rf ~"},
	}
	for _, c := range cases {
		got, err := Expand(Entry{Exec: c.exec}, files)
		if err != nil {
			t.Fatal(err)
		}
		var runs []string
		for _, args := range got {
			runs = append(runs, strings.Join(args, "|"))
		}
		if strings.Join(runs, " / ") != c.want {
			t.Errorf("%s: got %q", c.exec, runs)
		}
	}
}
