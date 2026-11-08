package shortcut

import (
	"os"
	"path/filepath"
	"testing"
)

func TestReadFindsTheTargetOfWindowsShortcuts(t *testing.T) {
	cases := []struct {
		file string
		want Shortcut
	}{
		{"folder.lnk", Shortcut{Kind: Lnk, Target: `C:\Windows\System32`, Dir: true}},
		{"file.lnk", Shortcut{Kind: Lnk, Target: `C:\Windows\notepad.exe`, Args: `C:\x.txt`}},
		// Only the list of shell items leads to a target that did not exist yet.
		{"umlaut.lnk", Shortcut{Kind: Lnk, Target: `C:\Daten\Grüße Ordner`}},
		{"unc.lnk", Shortcut{Kind: Lnk, Target: `\\nas\filme\Sintel`}},
	}
	for _, c := range cases {
		got, err := Read(filepath.Join("testdata", c.file))
		if err != nil {
			t.Errorf("%s: %v", c.file, err)
			continue
		}
		got.Icon = ""
		if got != c.want {
			t.Errorf("%s = %+v, want %+v", c.file, got, c.want)
		}
	}
}

func TestReadTakesTheAddressOfAnInternetShortcut(t *testing.T) {
	p := filepath.Join(t.TempDir(), "Wetter.url")
	body := "[{000214A0-0000-0000-C000-000000000046}]\r\nProp3=19,11\r\n[InternetShortcut]\r\nIDList=\r\nURL=https://wetter.example/wien\r\nIconFile=C:\\icons\\w.ico\r\nIconIndex=0\r\n"
	if err := os.WriteFile(p, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
	got, err := Read(p)
	if err != nil || got != (Shortcut{Kind: URL, Target: "https://wetter.example/wien", Icon: `C:\icons\w.ico`}) {
		t.Errorf("Read = %+v, %v", got, err)
	}
}

func TestReadNamesALauncherOfTheLinuxDesktop(t *testing.T) {
	dir := t.TempDir()
	app := filepath.Join(dir, "editor.desktop")
	write := func(p, body string) {
		if err := os.WriteFile(p, []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	write(app, "# made by hand\n[Desktop Entry]\nType=Application\nName=Texteditor\nName[de]=Texteditor\nExec=gedit %U\nIcon=org.gnome.gedit\n\n[Desktop Action new]\nName=Neues Fenster\nExec=gedit --new-window\n")
	got, err := Read(app)
	if err != nil || got != (Shortcut{Kind: Desktop, Target: "gedit %U", Name: "Texteditor", Icon: "org.gnome.gedit"}) {
		t.Errorf("application = %+v, %v", got, err)
	}
	link := filepath.Join(dir, "musik.desktop")
	write(link, "[Desktop Entry]\nType=Link\nName=Musik\nURL=file:///home/anna/Musik\n")
	if got, err := Read(link); err != nil || got != (Shortcut{Kind: Desktop, Target: "/home/anna/Musik", Name: "Musik"}) {
		t.Errorf("link = %+v, %v", got, err)
	}
	bad := filepath.Join(dir, "kaputt.desktop")
	write(bad, "kein Starter\n")
	if _, err := Read(bad); err == nil {
		t.Error("a file without [Desktop Entry] reads as a launcher")
	}
}

func TestReadRefusesWhatIsNoShortcut(t *testing.T) {
	p := filepath.Join(t.TempDir(), "kaputt.lnk")
	if err := os.WriteFile(p, []byte("nichts"), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := Read(p); err == nil {
		t.Error("a broken .lnk reads")
	}
	if KindOf("notes.txt") != "" || KindOf(".lnk") != "" || KindOf("A.LNK") != Lnk {
		t.Error("KindOf goes by the wrong ending")
	}
}

func TestWriteMakesAShortcutThatReadsBack(t *testing.T) {
	dir := t.TempDir()
	for _, want := range []Shortcut{
		{Kind: Lnk, Target: `C:\Users\Öffentlich\Bilder`, Dir: true},
		{Kind: Lnk, Target: `D:\Filme\Sintel (2010).mkv`},
	} {
		p := filepath.Join(dir, "x.lnk")
		if err := Write(p, want.Target, want.Dir); err != nil {
			t.Fatal(err)
		}
		got, err := Read(p)
		if err != nil || got != want {
			t.Errorf("read back %+v, %v, want %+v", got, err, want)
		}
	}
}
