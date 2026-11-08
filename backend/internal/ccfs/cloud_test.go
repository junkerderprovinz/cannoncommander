package ccfs

import "testing"

func TestAPlaceholderOfTheCloudIsOneThatWouldBeFetched(t *testing.T) {
	for attrs, want := range map[uint32]bool{
		0x20:             false,
		0x20 | 0x80000:   false,
		0x400000:         true,
		0x40000 | 0x20:   true,
		0x1000:           true,
		0x400000 | 0x400: true,
		0x100000 | 0x400: false,
	} {
		if got := CloudOnly(attrs); got != want {
			t.Errorf("CloudOnly(%#x) = %v, want %v", attrs, got, want)
		}
	}
}
