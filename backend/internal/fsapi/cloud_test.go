package fsapi

import "testing"

func TestTheCloudStateReadsAsExplorersStatusColumn(t *testing.T) {
	cases := []struct {
		attrs  uint32
		tagged bool
		want   string
	}{
		{attrArchive, false, ""},
		{attrRecallOnDataAccess | attrReparse | attrUnpinned, true, "online"},
		{attrOffline, false, "online"},
		{attrReparse | attrArchive, true, "local"},
		{attrUnpinned | attrArchive, false, "local"},
		{attrPinned | attrReparse, true, "pinned"},
		{attrReparse, false, ""},
	}
	for _, c := range cases {
		if got := cloudState(c.attrs, c.tagged); got != c.want {
			t.Errorf("cloudState(%#x, %v) = %q, want %q", c.attrs, c.tagged, got, c.want)
		}
	}
}
