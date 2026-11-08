package fsapi

import "testing"

func TestAttributesReadAsExplorerShowsThem(t *testing.T) {
	cases := map[uint32]string{
		0:                                    "",
		attrArchive:                          "A",
		attrReadOnly | attrHidden | attrArchive: "RHA",
		attrHidden | attrSystem | attrReparse:   "HSL",
		attrOffline | attrRecallOnDataAccess:    "OM",
		attrPinned | attrArchive:                "AP",
	}
	for attrs, want := range cases {
		if got := attrLetters(attrs); got != want {
			t.Errorf("attrLetters(%#x) = %q, want %q", attrs, got, want)
		}
	}
}
