package unraid

import (
	"slices"
	"strings"
	"testing"
)

func TestATransferReportKeepsOnlyWhatTheUserMayKnow(t *testing.T) {
	r := TransferReport{
		Trap:  &Trap{Files: []TrapFile{{Path: "/mnt/user/frei/a"}, {Path: "/mnt/user/geheim/b"}}},
		Links: &LinkReport{Files: []LinkedFile{{Path: "/mnt/user/geheim/c"}, {Path: "/mnt/user/frei/d", Others: []string{"/mnt/user/geheim/e", "/mnt/user/frei/f"}}}},
		Space: &SpaceReport{Biggest: &FileSize{Path: "/mnt/user/geheim/g"}},
	}
	r.Keep(func(p string) bool { return strings.HasPrefix(p, "/mnt/user/frei/") })
	if len(r.Trap.Files) != 1 || r.Trap.Files[0].Path != "/mnt/user/frei/a" {
		t.Errorf("trap %+v", r.Trap.Files)
	}
	if len(r.Links.Files) != 1 || !slices.Equal(r.Links.Files[0].Others, []string{"/mnt/user/frei/f"}) {
		t.Errorf("links %+v", r.Links.Files)
	}
	if r.Space.Biggest.Path != "" {
		t.Errorf("biggest %q", r.Space.Biggest.Path)
	}
}
