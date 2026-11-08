package nativeshell

import (
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
)

// serviceMenu installs a Dolphin service menu into the system's folder.
func (f *fixture) serviceMenu(name, entry string, actions ...string) {
	f.write(filepath.Join(f.system, "kio", "servicemenus", name), "[Desktop Entry]\nType=Service\n"+entry+"\n\n"+strings.Join(actions, "\n\n")+"\n")
}

func TestServiceMenusFitByTypeCountAndProtocol(t *testing.T) {
	f := newFixture(t)
	f.serviceMenu("images.desktop", "MimeType=image/*;\nActions=small;big;broken\nX-KDE-Submenu=Convert\nX-KDE-Submenu[de]=Umwandeln",
		"[Desktop Action small]\nName=Small\nName[de]=Klein\nIcon=image-x-generic\nExec=convert %f -resize 50% small",
		"[Desktop Action big]\nName=Big\nExec=convert %F big",
		"[Desktop Action broken]\nName=No command")
	f.serviceMenu("compare.desktop", "MimeType=all/allfiles;\nX-KDE-MinNumberOfUrls=2\nX-KDE-MaxNumberOfUrls=2\nActions=diff", "[Desktop Action diff]\nName=Compare\nExec=kompare %U")
	f.serviceMenu("terminal.desktop", "MimeType=inode/directory;\nActions=term", "[Desktop Action term]\nName=Terminal here\nExec=konsole --workdir %f")
	f.serviceMenu("remote.desktop", "MimeType=all/all;\nX-KDE-Protocols=ftp,sftp\nActions=up", "[Desktop Action up]\nName=Upload\nExec=up %U")
	f.serviceMenu("local.desktop", "MimeType=all/all;\nX-KDE-Protocols=file,smb\nActions=info", "[Desktop Action info]\nName=Info\nExec=info %F")
	f.serviceMenu("running.desktop", "MimeType=all/all;\nX-KDE-ShowIfRunning=org.kde.yakuake\nActions=x", "[Desktop Action x]\nName=X\nExec=x")
	f.serviceMenu("counts.desktop", "MimeType=all/all;\nX-KDE-RequiredNumberOfUrls=1;3\nActions=c", "[Desktop Action c]\nName=Counted\nExec=c %F")
	f.serviceMenu("stream.desktop", "MimeType=application/octet-stream;\nActions=hex", "[Desktop Action hex]\nName=Hex\nExec=okteta %F")
	f.serviceMenu("notpng.desktop", "MimeType=image/*;\nExcludeServiceTypes=image/png\nActions=j", "[Desktop Action j]\nName=Not PNG\nExec=j %F")
	f.serviceMenu("notaservice.desktop", "Type=Application\nMimeType=all/all;\nActions=n", "[Desktop Action n]\nName=N\nExec=n")
	f.write(filepath.Join(f.system, "kservices5", "ServiceMenus", "old.desktop"), "[Desktop Entry]\nType=Service\nServiceTypes=KonqPopupMenu/Plugin,text/plain\nActions=edit\n\n[Desktop Action edit]\nName=Edit\nExec=kate %U\n")
	// The user's own: one executable, one that only landed there, and one
	// that hides the system's menu of the same name.
	own := filepath.Join(f.user, "kio", "servicemenus")
	f.script(filepath.Join(own, "mine.desktop"), "[Desktop Entry]\nType=Service\nMimeType=all/all;\nActions=m\n\n[Desktop Action m]\nName=Mine\nExec=mine %F\n")
	f.write(filepath.Join(own, "dropped.desktop"), "[Desktop Entry]\nType=Service\nMimeType=all/all;\nActions=d\n\n[Desktop Action d]\nName=Dropped\nExec=evil %F\n")
	f.script(filepath.Join(own, "local.desktop"), "[Desktop Entry]\nType=Service\nMimeType=text/plain;\nActions=own\n\n[Desktop Action own]\nName=Own info\nExec=info %F\n")

	k := f.desk()
	cases := []struct {
		files []string
		want  string
	}{
		{[]string{"a.png"}, "counts.desktop#c images.desktop#big images.desktop#small mine.desktop#m stream.desktop#hex"},
		{[]string{"a.png", "b.jpg"}, "compare.desktop#diff images.desktop#big images.desktop#small mine.desktop#m stream.desktop#hex"},
		{[]string{"b.jpg"}, "counts.desktop#c images.desktop#big images.desktop#small mine.desktop#m notpng.desktop#j stream.desktop#hex"},
		{[]string{"photos/"}, "counts.desktop#c mine.desktop#m terminal.desktop#term"},
		{[]string{"notes.txt"}, "counts.desktop#c local.desktop#own mine.desktop#m old.desktop#edit stream.desktop#hex"},
		{[]string{"a.png", "photos/"}, "mine.desktop#m"},
		{[]string{"a.png", "b.jpg", "c.png"}, "counts.desktop#c images.desktop#big images.desktop#small mine.desktop#m stream.desktop#hex"},
	}
	for _, c := range cases {
		var got []string
		for _, a := range k.dolphin(f.selection(k, c.files...)) {
			got = append(got, strings.TrimPrefix(a.Key, "dolphin:"))
		}
		slices.Sort(got)
		if s := strings.Join(got, " "); s != c.want {
			t.Errorf("%v: got %s", c.files, s)
		}
	}
}

func TestAServiceMenuIsNamedInTheUsersLanguageAndRunsPerFile(t *testing.T) {
	f := newFixture(t)
	f.serviceMenu("images.desktop", "MimeType=image/*;\nActions=small;big\nX-KDE-Submenu=Convert\nX-KDE-Submenu[de]=Umwandeln",
		"[Desktop Action small]\nName=Small\nName[de]=Klein\nIcon=image-x-generic\nExec=convert %f -resize 50%% small",
		"[Desktop Action big]\nName=Big\nExec=\"/opt/my tools/big\" %F --caption %c")
	k := f.desk()
	sel := f.selection(k, "a.png", "b;rm -rf ~.png")
	all := k.dolphin(sel)
	if len(all) != 2 {
		t.Fatalf("got %d", len(all))
	}
	var small, big action
	for _, a := range all {
		if strings.HasSuffix(a.Key, "#small") {
			small = a
		} else {
			big = a
		}
	}
	if small.Label != "Klein" || small.Group != "Umwandeln" || small.icon != "image-x-generic" {
		t.Errorf("small %+v", small)
	}
	runs, err := small.launches(sel)
	if err != nil || len(runs) != 2 {
		t.Fatalf("got %v %v", runs, err)
	}
	for i, name := range []string{"a.png", "b;rm -rf ~.png"} {
		want := "convert|" + filepath.Join(f.work, name) + "|-resize|50%|small"
		if got := strings.Join(runs[i].args, "|"); got != want || runs[i].dir != f.work {
			t.Errorf("run %d: got %s in %s", i, got, runs[i].dir)
		}
	}
	runs, err = big.launches(sel)
	want := "/opt/my tools/big|" + filepath.Join(f.work, "a.png") + "|" + filepath.Join(f.work, "b;rm -rf ~.png") + "|--caption|Big"
	if err != nil || len(runs) != 1 || strings.Join(runs[0].args, "|") != want {
		t.Errorf("big: got %v %v", runs, err)
	}
}

func TestAServiceMenuOfTheUserNeedsTheExecutableBit(t *testing.T) {
	f := newFixture(t)
	file := f.write(filepath.Join(f.user, "kio", "servicemenus", "mine.desktop"), "[Desktop Entry]\nType=Service\nMimeType=all/all;\nActions=m\n\n[Desktop Action m]\nName=Mine\nExec=mine %F\n")
	k := f.desk()
	sel := f.selection(k, "a.png")
	if got := k.dolphin(sel); len(got) != 0 {
		t.Fatalf("got %s", keys(got))
	}
	if err := os.Chmod(file, 0o755); err != nil {
		t.Fatal(err)
	}
	if got := keys(k.dolphin(sel)); got != "dolphin:mine.desktop#m" {
		t.Fatalf("got %s", got)
	}
}
