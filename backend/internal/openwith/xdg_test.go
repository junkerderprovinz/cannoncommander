//go:build desktop

package openwith

import (
	"errors"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/junkerderprovinz/cannoncommander/backend/internal/xdg"
)

// desk is a Linux desktop laid out in a temporary folder: a data folder of
// the user and one of the system, and the user's config folder.
type desk struct {
	t                  *testing.T
	home, user, system string
	config             string
	x                  xdg.Desktop
}

func newDesk(t *testing.T, installed ...string) *desk {
	t.Helper()
	base := t.TempDir()
	d := &desk{t: t, home: filepath.Join(base, "home"), user: filepath.Join(base, "home", ".local", "share"), system: filepath.Join(base, "usr", "share"), config: filepath.Join(base, "home", ".config")}
	d.x = xdg.Desktop{
		Data: []string{d.user, d.system}, Config: []string{d.config}, Home: d.home, Langs: []string{"de_AT", "de"},
		Look: func(name string) (string, error) {
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

func TestTheDefaultComesBeforeTheProgramsForTheTypeAndItsParents(t *testing.T) {
	d := newDesk(t)
	d.program("gedit.desktop", "Name=Text Editor", "Exec=gedit %U", "MimeType=text/plain;")
	d.program("apostrophe.desktop", "Name=Apostrophe", "Exec=apostrophe %f", "MimeType=text/x-markdown;")
	d.program("marker.desktop", "Name=Marker", "Exec=marker %f", "MimeType=text/markdown;")
	d.program("vim.desktop", "Name=Vim", "Exec=vim %F", "MimeType=text/plain;")
	d.write(filepath.Join(d.config, "mimeapps.list"), "[Default Applications]\ntext/markdown=gone.desktop;marker.desktop;\n")
	got := apps(d.x, d.write(filepath.Join(d.home, "notes.md"), "# hi"))
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
	got := apps(d.x, d.write(filepath.Join(d.home, "notes.txt"), "hi"))
	if ids(got) != "kate.desktop gedit.desktop" {
		t.Fatalf("got %s", ids(got))
	}
}

func TestAParentsDefaultLeadsWhenTheTypeHasNone(t *testing.T) {
	d := newDesk(t)
	d.program("gedit.desktop", "Name=Text Editor", "Exec=gedit %U", "MimeType=text/plain;")
	d.program("marker.desktop", "Name=Marker", "Exec=marker %f", "MimeType=text/markdown;")
	d.write(filepath.Join(d.config, "kde-mimeapps.list"), "[Default Applications]\ntext/plain=gedit.desktop\n")
	d.x.Desktops = []string{"kde"}
	got := apps(d.x, d.write(filepath.Join(d.home, "notes.md"), "# hi"))
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
	got := apps(d.x, d.write(filepath.Join(d.home, "notes.txt"), "hi"))
	if ids(got) != "present.desktop" {
		t.Fatalf("got %s", ids(got))
	}
}

func TestAProgramInASubfolderIsNamedWithADash(t *testing.T) {
	d := newDesk(t)
	d.write(filepath.Join(d.system, "applications", "kde4", "okular.desktop"), "[Desktop Entry]\nType=Application\nName=Okular\nExec=okular %U\nMimeType=image/png;\n")
	got := apps(d.x, d.write(filepath.Join(d.home, "a.png"), "\x89PNG"))
	if ids(got) != "kde4-okular.desktop" {
		t.Fatalf("got %s", ids(got))
	}
}

func TestANameIsInTheUsersLanguage(t *testing.T) {
	d := newDesk(t)
	d.program("a.desktop", "Name=Text Editor", "Name[de]=Texteditor", "Name[fr]=Éditeur", "Exec=a", "MimeType=text/plain;")
	d.program("b.desktop", "Name=Files", "Name[de_AT]=Dateien\\sund\\sOrdner", "Name[de]=Dateien", "Exec=b", "MimeType=text/plain;")
	got := apps(d.x, d.write(filepath.Join(d.home, "notes.txt"), "hi"))
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
	if got := ids(every(d.x)); got != "calc.desktop zed.desktop" {
		t.Fatalf("got %s", got)
	}
}
