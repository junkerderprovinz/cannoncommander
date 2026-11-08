package xdg

import (
	"errors"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
)

// gsettings answers for the gsettings program from values, keyed by
// schema and key, and notes every call.
func gsettings(values map[string]string, calls *[]string) func(string, ...string) ([]byte, error) {
	return func(program string, args ...string) ([]byte, error) {
		*calls = append(*calls, program+" "+strings.Join(args, " "))
		if len(args) != 3 || args[0] != "get" {
			return nil, errors.New("usage")
		}
		v, ok := values[args[1]+" "+args[2]]
		if !ok {
			return nil, errors.New("No such schema")
		}
		return []byte(v + "\n"), nil
	}
}

func TestAGSettingIsReadWithGsettingsWhereItIsThere(t *testing.T) {
	d := newDesk(t, "gsettings")
	var calls []string
	d.Output = gsettings(map[string]string{"org.gnome.desktop.interface icon-theme": "'Yaru'"}, &calls)
	if got, err := d.GSetting("org.gnome.desktop.interface", "icon-theme"); err != nil || got != "'Yaru'" {
		t.Errorf("got %q %v", got, err)
	}
	if !slices.Equal(calls, []string{"/usr/bin/gsettings get org.gnome.desktop.interface icon-theme"}) {
		t.Errorf("calls %q", calls)
	}
	calls = nil
	d = newDesk(t)
	d.Output = gsettings(nil, &calls)
	if _, err := d.GSetting("a", "b"); err == nil || len(calls) != 0 {
		t.Errorf("without gsettings: %v %q", err, calls)
	}
	d.Output = nil
	if _, err := d.GSetting("a", "b"); err == nil {
		t.Error("without a runner")
	}
}

func TestAGVariantStringIsUnquoted(t *testing.T) {
	cases := map[string]string{
		`'Adwaita'`:          "Adwaita",
		`"it's"`:             "it's",
		`'say \'hi\' \\ \n'`: "say 'hi' \\ \n",
		`'café \U0001F600'`:  "café \U0001F600",
		`''`:                 "",
	}
	for in, want := range cases {
		if got, ok := UnquoteGVariant(in); !ok || got != want {
			t.Errorf("%s: got %q %v", in, got, ok)
		}
	}
	for _, bad := range []string{"Adwaita", "'open", `'a'b'`, `'trail\'`, "true", "uint32 5", `'\u12'`, ""} {
		if got, ok := UnquoteGVariant(bad); ok {
			t.Errorf("%s: got %q", bad, got)
		}
	}
}

func TestADconfDatabaseGivesItsStrings(t *testing.T) {
	// testdata/dconf-user was written by dconf compile from a keyfile that
	// sets the GNOME and Cinnamon icon themes, a colour scheme, a double
	// and a key in another folder.
	b, err := os.ReadFile(filepath.Join("testdata", "dconf-user"))
	if err != nil {
		t.Fatal(err)
	}
	cases := map[string]string{
		"/org/gnome/desktop/interface/icon-theme":         "Papirus-Dark",
		"/org/gnome/desktop/interface/color-scheme":       "prefer-dark",
		"/org/cinnamon/desktop/interface/icon-theme":      "Mint-Y",
		"/org/gnome/desktop/wm/preferences/button-layout": "close:",
	}
	for key, want := range cases {
		if got, ok := gvdbString(b, key); !ok || got != want {
			t.Errorf("%s: got %q %v", key, got, ok)
		}
	}
	for _, key := range []string{"/org/gnome/desktop/interface/text-scaling-factor", "/org/gnome/desktop/interface/", "/org/mate/desktop/interface/icon-theme", "icon-theme", ""} {
		if got, ok := gvdbString(b, key); ok {
			t.Errorf("%s: got %q", key, got)
		}
	}
	for _, broken := range [][]byte{nil, b[:20], []byte("NotGVDB!" + string(b[8:])), b[:400]} {
		if got, ok := gvdbString(broken, "/org/gnome/desktop/interface/icon-theme"); ok {
			t.Errorf("a broken file gave %q", got)
		}
	}
}

func TestTheIconThemeOfGNOMEComesFromGSettings(t *testing.T) {
	dconf, err := os.ReadFile(filepath.Join("testdata", "dconf-user"))
	if err != nil {
		t.Fatal(err)
	}
	values := map[string]string{"org.gnome.desktop.interface icon-theme": "'Yaru'", "org.mate.interface icon-theme": "'menta'"}
	cases := []struct {
		desktops  []string
		gsettings bool
		dconf     bool
		gtk       bool
		want      string
	}{
		{[]string{"ubuntu", "gnome"}, true, true, true, "Yaru"},
		{[]string{"ubuntu", "gnome"}, false, true, true, "Papirus-Dark"},
		{[]string{"ubuntu", "gnome"}, false, false, true, "FromGTK"},
		{[]string{"gnome"}, false, false, false, "Adwaita"},
		{[]string{"x-cinnamon"}, false, true, true, "Mint-Y"},
		{[]string{"x-cinnamon"}, true, false, true, "FromGTK"},
		{[]string{"mate"}, true, true, false, "menta"},
		{[]string{"xfce"}, true, true, true, "FromGTK"},
		{[]string{"xfce"}, true, true, false, "Adwaita"},
		{[]string{"kde"}, true, true, true, "FromGTK"},
	}
	for _, c := range cases {
		var installed []string
		if c.gsettings {
			installed = []string{"gsettings"}
		}
		d := newDesk(t, installed...)
		d.Desktops = c.desktops
		var calls []string
		d.Output = gsettings(values, &calls)
		if c.dconf {
			d.write(filepath.Join(d.config, "dconf", "user"), string(dconf))
		}
		if c.gtk {
			d.write(filepath.Join(d.config, "gtk-3.0", "settings.ini"), "[Settings]\ngtk-icon-theme-name=FromGTK\n")
		}
		if got := d.ThemeName(); got != c.want {
			t.Errorf("%v gsettings %v dconf %v gtk %v: got %s, want %s", c.desktops, c.gsettings, c.dconf, c.gtk, got, c.want)
		}
	}
}

func TestAHugeDconfDatabaseIsNotRead(t *testing.T) {
	d := newDesk(t)
	d.Desktops = []string{"gnome"}
	if err := os.MkdirAll(filepath.Join(d.config, "dconf"), 0o755); err != nil {
		t.Fatal(err)
	}
	big, err := os.Create(filepath.Join(d.config, "dconf", "user"))
	if err != nil {
		t.Fatal(err)
	}
	if err := big.Truncate(9 << 20); err != nil {
		t.Fatal(err)
	}
	big.Close()
	if got := d.ThemeName(); got != "Adwaita" {
		t.Errorf("got %s", got)
	}
}
