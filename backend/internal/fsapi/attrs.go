package fsapi

// The file attributes of Windows, with the values of winnt.h, so the rules
// built on them can be tested anywhere.
const (
	attrReadOnly           = 0x1
	attrHidden             = 0x2
	attrSystem             = 0x4
	attrArchive            = 0x20
	attrReparse            = 0x400
	attrCompressed         = 0x800
	attrOffline            = 0x1000
	attrEncrypted          = 0x4000
	attrPinned             = 0x80000
	attrUnpinned           = 0x100000
	attrRecallOnDataAccess = 0x400000
)

// letters are the attributes in the order and with the letters of
// Explorer's attribute column.
var letters = []struct {
	bit    uint32
	letter byte
}{
	{attrReadOnly, 'R'}, {attrHidden, 'H'}, {attrSystem, 'S'}, {attrArchive, 'A'},
	{attrCompressed, 'C'}, {attrEncrypted, 'E'}, {attrOffline, 'O'}, {attrRecallOnDataAccess, 'M'},
	{attrPinned, 'P'}, {attrUnpinned, 'U'}, {attrReparse, 'L'},
}

func attrLetters(attrs uint32) string {
	var out []byte
	for _, l := range letters {
		if attrs&l.bit != 0 {
			out = append(out, l.letter)
		}
	}
	return string(out)
}
