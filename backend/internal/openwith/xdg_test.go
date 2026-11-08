package openwith

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
	x                  xdg
}

func newDesk(t *testing.T, installed ...string) *desk {
	t.Helper()
	base := t.TempDir()
	d := &desk{t: t, home: filepath.Join(base, "home"), user: filepath.Join(base, "home", ".local", "share"), system: filepath.Join(base, "usr", "share"), config: filepath.Join(base, "home", ".config")}
	d.x = xdg{
		data: []string{d.user, d.system}, config: []string{d.config}, home: d.home, langs: []string{"de_AT", "de"},
		look: func(name string) (string, error) {
			if slices.Contains(installed, name) {
				return "/usr/bin/" + name, nil
			}
			return "", errors.New("not installed")
		},
	}
	d.write(filepath.Join(d.system, "mime", "globs2"), "# weight:type:glob\n50:text/plain:*.txt\n50:text/markdown:*.md\n50:image/png:*.png\n20:text/x-readme:README*\n50:text/x-makefile:Makefile:cs\n")
	d.write(filepath.Join(d.system, "mime", "subclasses"), "text/markdown text/plain\n")
	d.write(filepath.Join(d.system, "mime", "aliases"), "text/x-markdown text/markdown\n")
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

// program installs a desktop entry into the system's data folder.
func (d *desk) program(id string, lines ...string) {
	d.write(filepath.Join(d.system, "applications", id), "[Desktop Entry]\nType=Application\n"+strings.Join(lines, "\n")+"\n")
}

func ids(apps []app) string {
	var out []string
	for _, a := range apps {
		id := a.id
		if a.def {
			id += "*"
		}
		out = append(out, id)
	}
	return strings.Join(out, " ")
}

func TestATypeIsKnownByItsName(t *testing.T) {
	d := newDesk(t)
	db := d.x.mimeDB()
	cases := map[string]string{
		"notes.TXT":    "text/plain",
		"README.md":    "text/markdown",
		"README":       "text/x-readme",
		"Makefile":     "text/x-makefile",
		"makefile.txt": "text/plain",
	}
	for name, want := range cases {
		if got := db.mimeType(filepath.Join(d.home, name)); got != want {
			t.Errorf("%s: got %s, want %s", name, got, want)
		}
	}
}

func TestAFileWithoutAKnownNameIsTextOrData(t *testing.T) {
	d := newDesk(t)
	db := d.x.mimeDB()
	text := d.write(filepath.Join(d.home, "notes.zzz"), "just some words\n")
	data := d.write(filepath.Join(d.home, "blob.zzz"), "\x00\x01\x02\xff")
	if got := db.mimeType(text); got != "text/plain" {
		t.Errorf("text: got %s", got)
	}
	if got := db.mimeType(data); got != "application/octet-stream" {
		t.Errorf("data: got %s", got)
	}
}

func TestTheDefaultComesBeforeTheProgramsForTheTypeAndItsParents(t *testing.T) {
	d := newDesk(t)
	d.program("gedit.desktop", "Name=Text Editor", "Exec=gedit %U", "MimeType=text/plain;")
	d.program("apostrophe.desktop", "Name=Apostrophe", "Exec=apostrophe %f", "MimeType=text/x-markdown;")
	d.program("marker.desktop", "Name=Marker", "Exec=marker %f", "MimeType=text/markdown;")
	d.program("vim.desktop", "Name=Vim", "Exec=vim %F", "MimeType=text/plain;")
	d.write(filepath.Join(d.config, "mimeapps.list"), "[Default Applications]\ntext/markdown=gone.desktop;marker.desktop;\n")
	got := d.x.apps(d.write(filepath.Join(d.home, "notes.md"), "# hi"))
	if ids(got) != "marker.desktop* apostrophe.desktop gedit.desktop vim.desktop" {
		t.Fatalf("got %s", ids(got))
	}
}

func TestAddedAndRemovedAssociationsCount(t *testing.T) {
	d := newDesk(t)
	d.program("gedit.desktop", "Name=Text Editor", "Exec=gedit %U", "MimeType=text/plain;")
	d.program("vim.desktop", "Name=Vim", "Exec=vim %F", "MimeType=text/plain;")
	d.program("kate.desktop", "Name=Kate", "Exec=kate %U")
	d.write(filepath.Join(d.config, "mimeapps.list"), "[Added Associations]\ntext/plain=kate.desktop;\n\n[Removed Associations]\ntext/plain=vim.desktop;\n")
	d.write(filepath.Join(d.system, "applications", "defaults.list"), "[Default Applications]\ntext/plain=vim.desktop\n")
	got := d.x.apps(d.write(filepath.Join(d.home, "notes.txt"), "hi"))
	if ids(got) != "kate.desktop gedit.desktop" {
		t.Fatalf("got %s", ids(got))
	}
}

func TestAParentsDefaultLeadsWhenTheTypeHasNone(t *testing.T) {
	d := newDesk(t)
	d.program("gedit.desktop", "Name=Text Editor", "Exec=gedit %U", "MimeType=text/plain;")
	d.program("marker.desktop", "Name=Marker", "Exec=marker %f", "MimeType=text/markdown;")
	d.write(filepath.Join(d.config, "kde-mimeapps.list"), "[Default Applications]\ntext/plain=gedit.desktop\n")
	d.x.desktops = []string{"kde"}
	got := d.x.apps(d.write(filepath.Join(d.home, "notes.md"), "# hi"))
	if ids(got) != "gedit.desktop* marker.desktop" {
		t.Fatalf("got %s", ids(got))
	}
}

func TestEntriesThatAreNotThereToRunStayAway(t *testing.T) {
	d := newDesk(t, "present")
	d.program("hidden.desktop", "Name=Hidden", "Exec=hidden", "Hidden=true", "MimeType=text/plain;")
	d.program("missing.desktop", "Name=Missing", "Exec=missing", "TryExec=missing", "MimeType=text/plain;")
	d.program("present.desktop", "Name=Present", "Exec=present", "TryExec=present", "MimeType=text/plain;")
	d.program("link.desktop", "Type=Link", "Name=Link", "URL=https://example.com", "MimeType=text/plain;")
	d.program("noexec.desktop", "Name=Nothing", "MimeType=text/plain;")
	// The user's own entry hides the system's under the same id.
	d.program("mine.desktop", "Name=Mine", "Exec=mine", "MimeType=text/plain;")
	d.write(filepath.Join(d.user, "applications", "mine.desktop"), "[Desktop Entry]\nType=Application\nName=Mine\nExec=mine\nHidden=true\n")
	got := d.x.apps(d.write(filepath.Join(d.home, "notes.txt"), "hi"))
	if ids(got) != "present.desktop" {
		t.Fatalf("got %s", ids(got))
	}
}

func TestAProgramInASubfolderIsNamedWithADash(t *testing.T) {
	d := newDesk(t)
	d.write(filepath.Join(d.system, "applications", "kde4", "okular.desktop"), "[Desktop Entry]\nType=Application\nName=Okular\nExec=okular %U\nMimeType=image/png;\n")
	got := d.x.apps(d.write(filepath.Join(d.home, "a.png"), "\x89PNG"))
	if ids(got) != "kde4-okular.desktop" {
		t.Fatalf("got %s", ids(got))
	}
}

func TestANameIsInTheUsersLanguage(t *testing.T) {
	d := newDesk(t)
	d.program("a.desktop", "Name=Text Editor", "Name[de]=Texteditor", "Name[fr]=Éditeur", "Exec=a", "MimeType=text/plain;")
	d.program("b.desktop", "Name=Files", "Name[de_AT]=Dateien\\sund\\sOrdner", "Name[de]=Dateien", "Exec=b", "MimeType=text/plain;")
	got := d.x.apps(d.write(filepath.Join(d.home, "notes.txt"), "hi"))
	names := []string{got[0].name, got[1].name}
	if !slices.Equal(names, []string{"Dateien und Ordner", "Texteditor"}) {
		t.Fatalf("got %q", names)
	}
}

func TestEveryProgramOfTheMenusCanBePicked(t *testing.T) {
	d := newDesk(t)
	d.program("zed.desktop", "Name=zed", "Exec=zed")
	d.program("calc.desktop", "Name=Calculator", "Exec=calc", "Icon=calc")
	d.program("helper.desktop", "Name=Helper", "Exec=helper", "NoDisplay=true")
	if got := ids(d.x.every()); got != "calc.desktop zed.desktop" {
		t.Fatalf("got %s", got)
	}
}

func TestAnIconIsFoundInTheThemes(t *testing.T) {
	d := newDesk(t)
	big := d.write(filepath.Join(d.system, "icons", "hicolor", "48x48", "apps", "gedit.png"), "png")
	d.write(filepath.Join(d.system, "icons", "hicolor", "256x256", "apps", "gedit.png"), "png")
	themed := d.write(filepath.Join(d.home, ".icons", "Papirus", "64x64", "apps", "vlc.svg"), "<svg/>")
	old := d.write(filepath.Join(d.system, "pixmaps", "xterm.png"), "png")
	abs := d.write(filepath.Join(d.home, "apps", "own.png"), "png")
	cases := map[string]string{"gedit": big, "vlc": themed, "xterm": old, abs: abs}
	for name, want := range cases {
		if got, err := d.x.iconFile(name); err != nil || got != want {
			t.Errorf("%s: got %s %v, want %s", name, got, err, want)
		}
	}
	for _, name := range []string{"", "missing", "../../pixmaps/xterm", filepath.Join(d.home, "nothing.png")} {
		if got, err := d.x.iconFile(name); err == nil {
			t.Errorf("%q: found %s", name, got)
		}
	}
}

func TestTheExecLineGetsTheFile(t *testing.T) {
	e := entry{name: "Text Editor", icon: "gedit", file: "/usr/share/applications/gedit.desktop"}
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
		e.exec = c.exec
		got, err := command(e, "/home/jdp/a b.txt")
		if err != nil || strings.Join(got, "|") != c.want {
			t.Errorf("%s: got %q %v", c.exec, got, err)
		}
	}
	for _, bad := range []string{`app "open`, "", "%d"} {
		e.exec = bad
		if got, err := command(e, "/x"); err == nil {
			t.Errorf("%q: got %q", bad, got)
		}
	}
}

func TestTheDesktopIsReadFromTheEnvironment(t *testing.T) {
	env := map[string]string{"HOME": "/home/jdp", "XDG_DATA_DIRS": "/usr/share::/var/lib/flatpak/exports/share", "XDG_CURRENT_DESKTOP": "ubuntu:GNOME", "LANG": "de_AT.UTF-8"}
	x := xdgFrom(func(k string) string { return env[k] }, nil)
	if !slices.Equal(x.data, []string{"/home/jdp/.local/share", "/usr/share", "/var/lib/flatpak/exports/share"}) {
		t.Errorf("data %q", x.data)
	}
	if !slices.Equal(x.config, []string{"/home/jdp/.config", "/etc/xdg"}) {
		t.Errorf("config %q", x.config)
	}
	if !slices.Equal(x.desktops, []string{"ubuntu", "gnome"}) || !slices.Equal(x.langs, []string{"de_AT", "de"}) {
		t.Errorf("desktops %q langs %q", x.desktops, x.langs)
	}
	env = map[string]string{"HOME": "/root", "LANG": "C.UTF-8", "LC_ALL": "", "XDG_DATA_HOME": "/data"}
	x = xdgFrom(func(k string) string { return env[k] }, nil)
	if len(x.langs) != 0 || x.data[0] != "/data" || !slices.Equal(x.data[1:], []string{"/usr/local/share", "/usr/share"}) {
		t.Errorf("langs %q data %q", x.langs, x.data)
	}
}
