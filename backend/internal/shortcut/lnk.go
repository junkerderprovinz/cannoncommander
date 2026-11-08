package shortcut

import (
	"bytes"
	"encoding/binary"
	"errors"
	"os"
	"strings"
	"unicode/utf16"
)

// The layout of a .lnk file follows [MS-SHLLINK]. A shortcut names its
// target in up to three places: the LinkInfo that Explorer writes for a
// target it could look at, an environment block for paths with variables,
// and the list of shell items, which is all a shortcut to a target that did
// not exist yet has.
const (
	headerSize = 0x4C

	hasIDList    = 0x1
	hasLinkInfo  = 0x2
	hasName      = 0x4
	hasRelPath   = 0x8
	hasWorkDir   = 0x10
	hasArgs      = 0x20
	hasIcon      = 0x40
	isUnicode    = 0x80
	hasExpString = 0x200

	attrDirectory = 0x10
	attrArchive   = 0x20

	envBlock = 0xA0000001
)

var (
	linkCLSID  = []byte{0x01, 0x14, 0x02, 0, 0, 0, 0, 0, 0xC0, 0, 0, 0, 0, 0, 0, 0x46}
	myComputer = []byte{0xE0, 0x4F, 0xD0, 0x20, 0xEA, 0x3A, 0x69, 0x10, 0xA2, 0xD8, 0x08, 0x00, 0x2B, 0x30, 0x30, 0x9D}
)

func le16(b []byte, off int) int {
	if off < 0 || off+2 > len(b) {
		return 0
	}
	return int(binary.LittleEndian.Uint16(b[off:]))
}

func le32(b []byte, off int) int {
	if off < 0 || off+4 > len(b) {
		return 0
	}
	return int(binary.LittleEndian.Uint32(b[off:]))
}

// ansiz reads a string that ends with a zero byte. The code page of the
// computer that wrote it is unknown, and Latin-1 gets the common letters
// of Western code pages right.
func ansiz(b []byte, off int) string {
	if off <= 0 || off >= len(b) {
		return ""
	}
	end := bytes.IndexByte(b[off:], 0)
	if end < 0 {
		end = len(b) - off
	}
	r := make([]rune, end)
	for i, c := range b[off : off+end] {
		r[i] = rune(c)
	}
	return string(r)
}

func utf16z(b []byte, off int) string {
	if off <= 0 {
		return ""
	}
	var u []uint16
	for ; off+2 <= len(b); off += 2 {
		c := binary.LittleEndian.Uint16(b[off:])
		if c == 0 {
			break
		}
		u = append(u, c)
	}
	return string(utf16.Decode(u))
}

func joinWin(base, suffix string) string {
	if suffix == "" || base == "" {
		return base + suffix
	}
	if strings.HasSuffix(base, `\`) {
		return base + suffix
	}
	return base + `\` + suffix
}

func parseLnk(b []byte) (Shortcut, error) {
	if len(b) < headerSize || le32(b, 0) != headerSize || !bytes.Equal(b[4:20], linkCLSID) {
		return Shortcut{}, errNotShortcut
	}
	flags, attrs := le32(b, 20), le32(b, 24)
	s := Shortcut{Kind: Lnk, Dir: attrs&attrDirectory != 0}
	off := headerSize
	var fromIDs string
	if flags&hasIDList != 0 {
		n := le16(b, off)
		if off+2+n > len(b) {
			return Shortcut{}, errNotShortcut
		}
		var dir bool
		if fromIDs, dir = idListPath(b[off+2 : off+2+n]); fromIDs != "" {
			s.Dir = s.Dir || dir
		}
		off += 2 + n
	}
	var fromInfo string
	if flags&hasLinkInfo != 0 {
		n := le32(b, off)
		if n < 4 || off+n > len(b) {
			return Shortcut{}, errNotShortcut
		}
		fromInfo = linkInfoPath(b[off : off+n])
		off += n
	}
	strs := map[int]string{}
	for _, bit := range []int{hasName, hasRelPath, hasWorkDir, hasArgs, hasIcon} {
		if flags&bit == 0 {
			continue
		}
		count := le16(b, off)
		off += 2
		size := count
		if flags&isUnicode != 0 {
			size *= 2
		}
		if off+size > len(b) {
			return Shortcut{}, errNotShortcut
		}
		raw := b[off : off+size]
		if flags&isUnicode != 0 {
			u := make([]uint16, count)
			for i := range u {
				u[i] = binary.LittleEndian.Uint16(raw[2*i:])
			}
			strs[bit] = string(utf16.Decode(u))
		} else {
			strs[bit] = ansiz(append(append([]byte{0}, raw...), 0), 1)
		}
		off += size
	}
	var fromEnv string
	if flags&hasExpString != 0 {
		fromEnv = envTarget(b[off:])
	}
	for _, t := range []string{fromInfo, fromEnv, fromIDs} {
		if t != "" {
			s.Target = t
			break
		}
	}
	s.Args = strs[hasArgs]
	s.Icon = expand(strs[hasIcon])
	return s, nil
}

// linkInfoPath reads the local path, or the network path for a target on a
// share, preferring the Unicode copies a newer Windows adds.
func linkInfoPath(li []byte) string {
	if len(li) < 0x1C {
		return ""
	}
	header, flags := le32(li, 4), le32(li, 8)
	suffix := ansiz(li, le32(li, 24))
	if header >= 0x24 {
		if u := le32(li, 32); u != 0 {
			suffix = utf16z(li, u)
		}
	}
	if flags&1 != 0 {
		base := ansiz(li, le32(li, 16))
		if header >= 0x24 {
			if u := le32(li, 28); u != 0 {
				base = utf16z(li, u)
			}
		}
		return joinWin(base, suffix)
	}
	if net := le32(li, 20); flags&2 != 0 && net > 0 && net < len(li) {
		link := li[net:]
		nameOff := le32(link, 8)
		name := ansiz(link, nameOff)
		if nameOff > 0x14 {
			if u := le32(link, 0x14); u != 0 {
				name = utf16z(link, u)
			}
		}
		return joinWin(name, suffix)
	}
	return ""
}

// envTarget reads the target from the environment block of the extra data,
// with its variables filled in.
func envTarget(extra []byte) string {
	for off := 0; off+8 <= len(extra); {
		size := le32(extra, off)
		if size < 8 || off+size > len(extra) {
			return ""
		}
		if le32(extra, off+4) == envBlock && size >= 8+260+520 {
			block := extra[off+8 : off+size]
			t := utf16z(block, 260)
			if t == "" {
				t = ansiz(append([]byte{0}, block[:260]...), 1)
			}
			return expand(t)
		}
		off += size
	}
	return ""
}

// expand fills in %NAME% the way Windows writes variables.
func expand(s string) string {
	parts := strings.Split(s, "%")
	if len(parts) < 3 {
		return s
	}
	var b strings.Builder
	b.WriteString(parts[0])
	for i := 1; i < len(parts); i++ {
		if i%2 == 1 && i < len(parts)-1 {
			if v, ok := os.LookupEnv(parts[i]); ok {
				b.WriteString(v)
				continue
			}
			b.WriteString("%" + parts[i] + "%")
			continue
		}
		b.WriteString(parts[i])
	}
	return b.String()
}

// idListPath follows a list of shell items from This PC over a drive to the
// folders and the file. Items of other roots, such as the control panel,
// name no path.
func idListPath(ids []byte) (path string, dir bool) {
	var parts []string
	for off := 0; off+2 <= len(ids); {
		n := le16(ids, off)
		if n == 0 {
			break
		}
		if n < 3 || off+n > len(ids) {
			return "", false
		}
		item := ids[off+2 : off+n]
		off += n
		switch typ := item[0]; {
		case typ == 0x1F:
			if len(item) < 18 || !bytes.Equal(item[2:18], myComputer) {
				return "", false
			}
		case typ&0x70 == 0x20:
			parts = []string{strings.TrimRight(ansiz(item, 1), `\`)}
		case typ&0x70 == 0x30 && len(parts) > 0:
			name := entryName(item)
			if name == "" {
				return "", false
			}
			parts = append(parts, name)
			dir = typ&0x01 != 0
		default:
			return "", false
		}
	}
	if len(parts) == 0 {
		return "", false
	}
	if len(parts) == 1 {
		return parts[0] + `\`, true
	}
	return strings.Join(parts, `\`), dir
}

// longNameAt is where the extension block of a file entry keeps the long
// name, by the block's version.
var longNameAt = map[int]int{3: 0x14, 7: 0x1C, 8: 0x1E, 9: 0x2E}

// entryName prefers the long name of the extension block to the 8.3 name
// the item starts with.
func entryName(item []byte) string {
	if item[0]&0x04 != 0 {
		return utf16z(item, 12)
	}
	short := ansiz(item, 12)
	if i := bytes.Index(item[12:], []byte{0x04, 0x00, 0xEF, 0xBE}); i >= 4 {
		ext := 12 + i - 4
		if at, ok := longNameAt[le16(item, ext+2)]; ok {
			if long := utf16z(item, ext+at); long != "" {
				return long
			}
		}
	}
	return short
}

// idList writes the shell items from This PC to target in the simple form
// Windows uses for a target it does not look at: one item per folder, with
// the name in UTF-16.
func idList(target string, dir bool) []byte {
	var b bytes.Buffer
	item := func(data []byte) {
		_ = binary.Write(&b, binary.LittleEndian, uint16(len(data)+2))
		b.Write(data)
	}
	item(append([]byte{0x1F, 0x50}, myComputer...))
	drive := make([]byte, 23)
	drive[0] = 0x2F
	copy(drive[1:], target[:3])
	item(drive)
	names := strings.Split(strings.Trim(target[3:], `\`), `\`)
	for i, name := range names {
		if name == "" {
			continue
		}
		typ, attrs := byte(0x35), uint16(attrDirectory)
		if i == len(names)-1 && !dir {
			typ, attrs = 0x36, attrArchive
		}
		data := []byte{typ, 0, 0, 0, 0, 0, 0, 0, 0, 0, byte(attrs), byte(attrs >> 8)}
		for _, c := range utf16.Encode([]rune(name)) {
			data = append(data, byte(c), byte(c>>8))
		}
		item(append(data, 0, 0))
	}
	_ = binary.Write(&b, binary.LittleEndian, uint16(0))
	return b.Bytes()
}

// Write makes a shortcut at p to target, a path on a drive of this computer,
// the way Explorer's "Create shortcut" does. Explorer finds the target by
// the LinkInfo alone.
func Write(p, target string, dir bool) error {
	if len(target) < 3 || target[1] != ':' || target[2] != '\\' {
		return errors.New("a shortcut needs a path on a drive, such as C:\\")
	}
	attrs := uint32(attrArchive)
	if dir {
		attrs = attrDirectory
	}
	var b bytes.Buffer
	put := func(v ...any) {
		for _, x := range v {
			_ = binary.Write(&b, binary.LittleEndian, x)
		}
	}
	put(uint32(headerSize))
	b.Write(linkCLSID)
	put(uint32(hasIDList|hasLinkInfo|isUnicode), attrs, make([]byte, 24), uint32(0), uint32(0), uint32(1), uint16(0), make([]byte, 10))

	// Explorer resolves by the LinkInfo, while IShellLink::GetPath and with
	// it most programs read the shell items, so both go in.
	ids := idList(target, dir)
	put(uint16(len(ids)))
	b.Write(ids)

	ansi := make([]byte, 0, len(target)+1)
	for _, r := range target {
		if r > 0xFF {
			r = '?'
		}
		ansi = append(ansi, byte(r))
	}
	ansi = append(ansi, 0)
	wide := append(utf16.Encode([]rune(target)), 0)

	const infoHeader, volumeSize = 0x24, 0x11
	local := infoHeader + volumeSize
	suffix := local + len(ansi)
	localU := suffix + 1
	suffixU := localU + 2*len(wide)
	size := suffixU + 2
	put(uint32(size), uint32(infoHeader), uint32(1), uint32(infoHeader), uint32(local), uint32(0), uint32(suffix), uint32(localU), uint32(suffixU))
	// The volume is a fixed disk, whose serial number and label Explorer
	// does not need to find the path.
	put(uint32(volumeSize), uint32(3), uint32(0), uint32(0x10), uint8(0))
	b.Write(ansi)
	b.WriteByte(0)
	put(wide, uint16(0))
	put(uint32(0))
	return os.WriteFile(p, b.Bytes(), 0o666)
}
