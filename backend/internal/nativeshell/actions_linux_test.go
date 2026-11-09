package nativeshell

import (
	"errors"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/junkerderprovinz/cannoncommander/backend/internal/xdg"
)

func TestNautilusScriptsAreTheExecutableFilesWithFoldersAsSubmenus(t *testing.T) {
	f := newFixture(t)
	scripts := filepath.Join(f.user, "nautilus", "scripts")
	f.script(filepath.Join(scripts, "Resize"), "#!/bin/sh\n")
	f.write(filepath.Join(scripts, "readme.txt"), "not a script")
	f.script(filepath.Join(scripts, ".hidden"), "#!/bin/sh\n")
	f.script(filepath.Join(scripts, ".git", "hook"), "#!/bin/sh\n")
	f.script(filepath.Join(scripts, "Images", "To PNG"), "#!/bin/sh\n")
	k := f.desk()
	sel := f.selection(k, "a.png", "b c.jpg")
	all := k.nautilus(sel)
	var got []string
	for _, a := range all {
		got = append(got, a.Key+"@"+a.Group)
	}
	slices.Sort(got)
	if strings.Join(got, " ") != "nautilus:Images/To PNG@Images nautilus:Resize@" {
		t.Fatalf("got %q", got)
	}
	runs, err := all[slices.IndexFunc(all, func(a action) bool { return a.Label == "Resize" })].launches(sel)
	if err != nil || len(runs) != 1 {
		t.Fatalf("got %v %v", runs, err)
	}
	a, b := filepath.Join(f.work, "a.png"), filepath.Join(f.work, "b c.jpg")
	if !slices.Equal(runs[0].args, []string{filepath.Join(scripts, "Resize"), a, b}) || runs[0].dir != f.work {
		t.Errorf("args %q in %s", runs[0].args, runs[0].dir)
	}
	wantEnv := []string{
		"NAUTILUS_SCRIPT_SELECTED_FILE_PATHS=" + a + "\n" + b + "\n",
		"NAUTILUS_SCRIPT_SELECTED_URIS=" + xdg.FileURI(a) + "\n" + xdg.FileURI(b) + "\n",
		"NAUTILUS_SCRIPT_CURRENT_URI=" + xdg.FileURI(f.work),
	}
	if !slices.Equal(runs[0].env, wantEnv) {
		t.Errorf("env %q", runs[0].env)
	}
}

func TestActionsAreSortedBySubmenuThenName(t *testing.T) {
	f := newFixture(t)
	scripts := filepath.Join(f.user, "nautilus", "scripts")
	f.script(filepath.Join(scripts, "zip"), "#!/bin/sh\n")
	f.script(filepath.Join(scripts, "Archive"), "#!/bin/sh\n")
	f.script(filepath.Join(scripts, "Tools", "b"), "#!/bin/sh\n")
	f.script(filepath.Join(scripts, "tools", "A"), "#!/bin/sh\n")
	f.write(filepath.Join(f.system, "nemo", "actions", "m.nemo_action"), "[Nemo Action]\nName=mail\nExec=mail %F\nSelection=any\nExtensions=any;\n")
	k := f.desk()
	var got []string
	for _, a := range k.actions(f.selection(k, "a.png")) {
		got = append(got, a.Group+"/"+a.Label)
	}
	if strings.Join(got, " ") != "/Archive /mail /zip tools/A Tools/b" {
		t.Fatalf("got %q", got)
	}
}

func TestRunActionRefusesAnActionTheFilesDoNotHave(t *testing.T) {
	f := newFixture(t)
	f.write(filepath.Join(f.system, "nemo", "actions", "img.nemo_action"), "[Nemo Action]\nName=Image\nExec=false %F\nSelection=s\nMimetypes=image/*;\n")
	f.install()
	png, txt := f.files("a.png")[0], f.files("notes.txt")[0]
	if err := RunAction([]string{txt}, "nemo:img.nemo_action"); !errors.Is(err, ErrUnknownAction) {
		t.Errorf("a text: %v", err)
	}
	if err := RunAction([]string{png}, "nemo:other.nemo_action"); !errors.Is(err, ErrUnknownAction) {
		t.Errorf("an unknown key: %v", err)
	}
	if err := RunAction([]string{filepath.Join(f.work, "gone.png")}, "nemo:img.nemo_action"); err == nil {
		t.Error("a missing file ran")
	}
	if got, err := Actions([]string{png}); err != nil || len(got) != 1 || got[0].Key != "nemo:img.nemo_action" {
		t.Errorf("got %+v %v", got, err)
	}
}

// recorder is a script that writes its arguments one per line, the
// folder it runs in and what Nautilus told it, then a last line "end".
const recorder = "#!/bin/sh\nout=\"$(dirname \"$0\")/ran.txt\"\n{ for a in \"$@\"; do printf '[%s]\\n' \"$a\"; done; printf 'pwd=%s\\n' \"$PWD\"; " +
	"printf 'paths=%s' \"$NAUTILUS_SCRIPT_SELECTED_FILE_PATHS\"; echo end; } > \"$out.tmp\" && mv \"$out.tmp\" \"$out\"\n"

func TestAStartedActionGetsEachHostileNameAsOneArgument(t *testing.T) {
	f := newFixture(t)
	scripts := filepath.Join(f.user, "nautilus", "scripts")
	script := f.script(filepath.Join(scripts, "Record"), recorder)
	f.write(filepath.Join(f.config, "Thunar", "uca.xml"), "<actions><action><name>Record</name><unique-id>rec</unique-id>"+
		"<command>"+shellQuote(script)+" %F</command><patterns>*</patterns><other-files/><text-files/></action></actions>")
	f.install()
	files := f.files("a;rm -rf ~", "it's \"quoted\" $HOME", "--help")
	want := "[" + files[0] + "]\n[" + files[1] + "]\n[" + files[2] + "]\npwd=" + f.work + "\n"

	if err := RunAction(files, "nautilus:Record"); err != nil {
		t.Fatal(err)
	}
	got := waitFor(t, filepath.Join(scripts, "ran.txt"))
	if wantPaths := "paths=" + strings.Join(files, "\n") + "\nend\n"; got != want+wantPaths {
		t.Errorf("nautilus: got %q", got)
	}
	os.Remove(filepath.Join(scripts, "ran.txt"))

	if err := RunAction(files, "thunar:rec"); err != nil {
		t.Fatal(err)
	}
	if got := waitFor(t, filepath.Join(scripts, "ran.txt")); got != want+"paths=end\n" {
		t.Errorf("thunar: got %q", got)
	}
}

func TestAnActionsIconIsADataURL(t *testing.T) {
	f := newFixture(t)
	f.write(filepath.Join(f.system, "icons", "hicolor", "index.theme"), "[Icon Theme]\nDirectories=scalable/apps\n\n[scalable/apps]\nSize=48\nMinSize=8\nMaxSize=512\nType=Scalable\n")
	f.write(filepath.Join(f.system, "icons", "hicolor", "scalable", "apps", "own-tool.svg"), "<svg/>")
	own := f.write(filepath.Join(f.home, "icons", "mine.svg"), "<svg>mine</svg>")
	k := f.desk()
	cases := map[string]string{
		"own-tool": "data:image/svg+xml;base64,PHN2Zy8+",
		own:        "data:image/svg+xml;base64,PHN2Zz5taW5lPC9zdmc+",
		"missing":  "",
		"":         "",
	}
	for name, want := range cases {
		if got := k.iconURL(name); got != want {
			t.Errorf("%q: got %s", name, got)
		}
	}
}

func TestAFlatpakOffersNoActionsOfOtherFileManagers(t *testing.T) {
	f := newFixture(t)
	f.script(filepath.Join(f.user, "nautilus", "scripts", "Resize"), "#!/bin/sh\n")
	f.write(filepath.Join(f.system, "nemo", "actions", "m.nemo_action"), "[Nemo Action]\nName=mail\nExec=mail %F\nSelection=any\nExtensions=any;\n")
	f.install()
	t.Setenv("FLATPAK_ID", "io.github.junkerderprovinz.cannoncommander")
	png := f.files("a.png")[0]
	if got, err := Actions([]string{png}); err != nil || len(got) != 0 {
		t.Errorf("got %v %v", got, err)
	}
	if err := RunAction([]string{png}, "nautilus:Resize"); !errors.Is(err, ErrUnknownAction) {
		t.Errorf("ran an action: %v", err)
	}
}
