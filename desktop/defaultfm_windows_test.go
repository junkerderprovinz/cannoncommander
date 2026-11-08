package main

import (
	"errors"
	"fmt"
	"os"
	"slices"
	"testing"

	"golang.org/x/sys/windows/registry"

	"github.com/junkerderprovinz/cannoncommander/backend/pkg/desktop"
)

// testClasses is a classes key of its own below HKCU, so the test never
// touches the associations the computer uses.
func testClasses(t *testing.T) registryHandler {
	t.Helper()
	base := fmt.Sprintf(`Software\CannonCommanderTest-%d`, os.Getpid())
	t.Cleanup(func() {
		if err := deleteTree(base); err != nil {
			t.Errorf("clean up %s: %v", base, err)
		}
	})
	return registryHandler{classes: base + `\Classes`}
}

func value(t *testing.T, path, name string) string {
	t.Helper()
	k, err := registry.OpenKey(registry.CURRENT_USER, path, registry.QUERY_VALUE)
	if err != nil {
		return "<no key>"
	}
	defer k.Close()
	v, _, err := k.GetStringValue(name)
	if errors.Is(err, registry.ErrNotExist) {
		return "<no value>"
	}
	return v
}

func subKeys(t *testing.T, path string) []string {
	t.Helper()
	k, err := registry.OpenKey(registry.CURRENT_USER, path, registry.ENUMERATE_SUB_KEYS)
	if err != nil {
		return nil
	}
	defer k.Close()
	names, _ := k.ReadSubKeyNames(-1)
	slices.Sort(names)
	return names
}

func TestFoldersDrivesAndWinEOpenHereAndGoBack(t *testing.T) {
	h := testClasses(t)
	exe := `C:\Program Files\CannonCommander\CannonCommander.exe`
	// Another file manager had folders before.
	if err := setDefault(h.classes+`\Directory\shell`, "openinother"); err != nil {
		t.Fatal(err)
	}
	words := desktop.Words{Open: "In CannonCommander öffnen"}
	for range 2 {
		if err := h.set(exe, words); err != nil {
			t.Fatal(err)
		}
	}
	if !h.isSet(exe) || h.isSet(`D:\other\CannonCommander.exe`) {
		t.Fatal("not set for this program")
	}
	for path, want := range map[string]string{
		h.classes + `\Directory\shell`:                         verb,
		h.classes + `\Drive\shell`:                             verb,
		h.classes + `\Directory\shell\` + verb:                 "In CannonCommander öffnen",
		h.classes + `\Directory\shell\` + verb + `\command`:    `"` + exe + `" "%V"`,
		h.classes + `\Drive\shell\` + verb + `\command`:        `"` + exe + `" "%V"`,
		h.classes + `\` + openNewWindow + `\command`:           `"` + exe + `" --home`,
	} {
		if got := value(t, path, ""); got != want {
			t.Errorf("%s: got %q", path, got)
		}
	}
	if got := value(t, h.classes+`\`+openNewWindow+`\command`, "DelegateExecute"); got != "" {
		t.Errorf("DelegateExecute %q", got)
	}

	if err := h.unset(); err != nil {
		t.Fatal(err)
	}
	if h.isSet(exe) {
		t.Fatal("still set")
	}
	if got := value(t, h.classes+`\Directory\shell`, ""); got != "openinother" {
		t.Errorf("Directory goes back to %q", got)
	}
	// Only the key that was there before is left.
	if got := subKeys(t, h.classes); !slices.Equal(got, []string{"Directory"}) {
		t.Errorf("left behind %v", got)
	}
	if got := subKeys(t, h.classes+`\Directory\shell`); len(got) != 0 {
		t.Errorf("left behind %v", got)
	}
}
