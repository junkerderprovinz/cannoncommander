package shortcut

import (
	"encoding/binary"
	"testing"
)

// linkInfoOnShare builds the LinkInfo of a target on a share that is mapped
// to a drive letter.
func linkInfoOnShare(share, device, suffix string) []byte {
	cnrl := make([]byte, 0x14)
	binary.LittleEndian.PutUint32(cnrl[4:], 1)
	binary.LittleEndian.PutUint32(cnrl[8:], 0x14)
	binary.LittleEndian.PutUint32(cnrl[12:], uint32(0x14+len(share)+1))
	cnrl = append(append(append(cnrl, share...), 0), append([]byte(device), 0)...)
	binary.LittleEndian.PutUint32(cnrl[0:], uint32(len(cnrl)))
	li := make([]byte, 0x1C)
	binary.LittleEndian.PutUint32(li[4:], 0x1C)
	binary.LittleEndian.PutUint32(li[8:], 2)
	binary.LittleEndian.PutUint32(li[20:], 0x1C)
	binary.LittleEndian.PutUint32(li[24:], uint32(0x1C+len(cnrl)))
	li = append(append(append(li, cnrl...), suffix...), 0)
	binary.LittleEndian.PutUint32(li[0:], uint32(len(li)))
	return li
}

func TestAShortcutOnAMappedShareGoesByTheDriveLetter(t *testing.T) {
	old := driveExists
	t.Cleanup(func() { driveExists = old })
	li := linkInfoOnShare(`\nas\daten`, "Z:", `Fotos\2025`)

	driveExists = func(string) bool { return true }
	if got := linkInfoPath(li); got != `Z:\Fotos\2025` {
		t.Errorf("mapped share: %q", got)
	}
	driveExists = func(string) bool { return false }
	if got := linkInfoPath(li); got != `\nas\daten\Fotos\2025` {
		t.Errorf("share without its drive: %q", got)
	}
}
