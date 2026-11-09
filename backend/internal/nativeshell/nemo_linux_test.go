package nativeshell

import (
	"path/filepath"
	"slices"
	"strings"
	"testing"
)

// nemoAction installs a Nemo action into the system's folder.
func (f *fixture) nemoAction(name string, lines ...string) {
	f.write(filepath.Join(f.system, "nemo", "actions", name+".nemo_action"), "[Nemo Action]\nName="+name+"\nExec="+name+" %F\n"+strings.Join(lines, "\n")+"\n")
}

func TestANemoActionFitsBySelectionExtensionAndType(t *testing.T) {
	f := newFixture(t)
	f.nemoAction("image", "Selection=s", "Mimetypes=image/*;")
	f.nemoAction("many", "Selection=m", "Extensions=any;")
	f.nemoAction("two", "Selection=2", "Extensions=any;")
	f.nemoAction("folder", "Selection=notnone", "Extensions=dir;")
	f.nemoAction("files", "Selection=any", "Extensions=nodirs;")
	f.nemoAction("pngtxt", "Selection=any", "Extensions=PNG;", "Mimetypes=text/plain;")
	f.nemoAction("bare", "Extensions=none;")
	f.nemoAction("off", "Active=false", "Selection=any", "Extensions=any;")
	f.nemoAction("ondesktop", "Selection=any", "Extensions=any;", "Conditions=desktop;")
	f.nemoAction("needs", "Selection=any", "Extensions=any;", "Dependencies=present;")
	f.nemoAction("needsgone", "Selection=any", "Extensions=any;", "Dependencies=present;gone;")
	f.nemoAction("without", "Selection=any", "Extensions=any;", "Dependencies=!present;")
	f.nemoAction("nothing", "Selection=any")
	f.nemoAction("none", "Selection=none", "Extensions=any;")
	k := f.desk()
	cases := []struct {
		files []string
		want  string
	}{
		{[]string{"a.png"}, "files image needs pngtxt"},
		{[]string{"notes.txt"}, "files needs pngtxt"},
		{[]string{"Makefile"}, "bare files needs pngtxt"},
		{[]string{"photos/"}, "folder needs"},
		{[]string{"a.png", "b.jpg"}, "files many needs two"},
		{[]string{"a.png", "notes.txt"}, "files many needs pngtxt two"},
		{[]string{"a.png", "photos/", "c.png"}, "many needs"},
	}
	for _, c := range cases {
		var got []string
		for _, a := range k.nemo(f.selection(k, c.files...)) {
			got = append(got, strings.TrimSuffix(strings.TrimPrefix(a.Key, "nemo:"), ".nemo_action"))
		}
		slices.Sort(got)
		if s := strings.Join(got, " "); s != c.want {
			t.Errorf("%v: got %s, want %s", c.files, s, c.want)
		}
	}
}

func TestANemoActionNamesTheFileAndRunsItsScriptFromItsFolder(t *testing.T) {
	f := newFixture(t)
	f.write(filepath.Join(f.system, "nemo", "actions", "shrink.nemo_action"),
		"[Nemo Action]\nName=Shrink %N\nName[de]=%N verkleinern\nIcon-Name=zoom-out\nSelection=s\nExtensions=png;\nExec=<shrink.sh %F>\n")
	k := f.desk()
	sel := f.selection(k, "it's.png")
	all := k.nemo(sel)
	if len(all) != 1 || all[0].Label != "it's.png verkleinern" || all[0].icon != "zoom-out" || all[0].Group != "" {
		t.Fatalf("got %+v", all)
	}
	runs, err := all[0].launches(sel)
	want := filepath.Join(f.system, "nemo", "actions", "shrink.sh") + "|" + filepath.Join(f.work, "it's.png")
	if err != nil || len(runs) != 1 || strings.Join(runs[0].args, "|") != want || runs[0].dir != f.work {
		t.Fatalf("got %v %v", runs, err)
	}
}

func TestANemoCommandKeepsEveryNameOneArgument(t *testing.T) {
	f := newFixture(t)
	k := f.desk()
	sel := f.selection(k, "a;rm -rf ~.txt", `say "hi" $HOME.txt`)
	a, b := filepath.Join(f.work, "a;rm -rf ~.txt"), filepath.Join(f.work, `say "hi" $HOME.txt`)
	cases := []struct {
		exec, quote, sep string
		want             []string
	}{
		{"tool %F", "", "", []string{"tool", a, b}},
		{"tool %F", "double", "", []string{"tool", a, b}},
		{"tool %F", "", ",", []string{"tool", a + "," + b}},
		{"tool --dir=%P %p %f %e 100%%", "", "", []string{"tool", "--dir=" + f.work, "work", "a;rm -rf ~.txt", "a;rm -rf ~", "100%"}},
		{"tool %U", "", "", []string{"tool", "file://" + f.work + "/a%3Brm%20-rf%20~.txt", "file://" + f.work + "/say%20%22hi%22%20$HOME.txt"}},
		{`sh -c "ls %f"`, "", "", []string{"sh", "-c", `ls 'a;rm -rf ~.txt'`}},
		{"tool %D%X", "", "", []string{"tool"}},
	}
	for _, c := range cases {
		g := map[string]string{"Quote": c.quote, "Separator": c.sep}
		got, err := nemoCommand(c.exec, g, sel)
		if err != nil || !slices.Equal(got, c.want) {
			t.Errorf("%s quote %q: got %q %v", c.exec, c.quote, got, err)
		}
	}
}

func TestALabelWithFileNamesIsRememberedWithoutThem(t *testing.T) {
	f := newFixture(t)
	for name, label := range map[string]string{"open": "Open %N", "here": "Copy to %p", "first": "Shrink %e here", "plain": "Plain", "percent": "Plain 100%%"} {
		f.write(filepath.Join(f.system, "nemo", "actions", name+".nemo_action"), "[Nemo Action]\nName="+label+"\nExec=x %F\nSelection=any\nExtensions=any;\n")
	}
	f.write(filepath.Join(f.config, "Thunar", "uca.xml"), "<actions><action><name>Thunar</name><unique-id>t</unique-id><command>x %f</command><patterns>*</patterns><image-files/></action></actions>")
	f.script(filepath.Join(f.user, "nautilus", "scripts", "Script %N"), "#!/bin/sh\n")
	f.install()
	got, err := Actions(f.files("a b.png"))
	if err != nil {
		t.Fatal(err)
	}
	var rows []string
	for _, a := range got {
		rows = append(rows, a.Label+" = "+a.Remember)
	}
	want := []string{"Copy to work = Copy to …", "Open a b.png = Open …", "Plain = ", "Plain 100% = ", "Script %N = ", "Shrink a b here = Shrink … here", "Thunar = "}
	if !slices.Equal(rows, want) {
		t.Errorf("got %q", rows)
	}
}
