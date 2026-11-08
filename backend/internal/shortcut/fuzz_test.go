package shortcut

import (
	"encoding/binary"
	"os"
	"path/filepath"
	"testing"
)

// A file item of the shell that ends before its name must not take down the
// listing of the folder the shortcut lies in.
func TestAShortItemInTheIDListDoesNotPanic(t *testing.T) {
	var ids []byte
	add := func(data []byte) {
		ids = binary.LittleEndian.AppendUint16(ids, uint16(len(data)+2))
		ids = append(ids, data...)
	}
	add(append([]byte{0x1F, 0x50}, myComputer...))
	add(append([]byte{0x2F}, []byte(`C:\`)...))
	add([]byte{0x31})
	ids = append(ids, 0, 0)

	lnk := make([]byte, headerSize)
	binary.LittleEndian.PutUint32(lnk, headerSize)
	copy(lnk[4:], linkCLSID)
	binary.LittleEndian.PutUint32(lnk[20:], hasIDList)
	lnk = binary.LittleEndian.AppendUint16(lnk, uint16(len(ids)))
	lnk = append(lnk, ids...)
	if _, err := parseLnk(lnk); err != nil {
		t.Logf("refused: %v", err)
	}
}

func FuzzParseLnk(f *testing.F) {
	seeds, _ := filepath.Glob(filepath.Join("testdata", "*.lnk"))
	for _, p := range seeds {
		data, err := os.ReadFile(p)
		if err != nil {
			f.Fatal(err)
		}
		f.Add(data)
	}
	f.Fuzz(func(t *testing.T, data []byte) {
		_, _ = parseLnk(data)
	})
}
