package fsapi

import (
	"path/filepath"
	"strings"
	"testing"

	"golang.org/x/sys/windows"
)

func TestWindowsListsAttributesAndNamesTheOwner(t *testing.T) {
	f := newFixture(t)
	p := filepath.Join(f.share, "fotos", "vertrag.pdf")
	write(t, p, 1)
	setAttrs(t, p, windows.FILE_ATTRIBUTE_READONLY|windows.FILE_ATTRIBUTE_ARCHIVE)

	l := decode[Listing](t, f.get(t, "jdp", "/fs/list", "/mnt/user/fotos"))
	if got := byName(t, l.Items, "vertrag.pdf"); got.Attrs != "RA" || got.Mode != "0444" {
		t.Errorf("listed = %+v", got)
	}
	st := decode[Stat](t, f.get(t, "jdp", "/fs/stat", "/mnt/user/fotos/vertrag.pdf"))
	if !strings.Contains(st.Owner, `\`) || st.Group != "" {
		t.Errorf("owner %q, group %q", st.Owner, st.Group)
	}
}
