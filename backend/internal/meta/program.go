package meta

import (
	"archive/tar"
	"archive/zip"
	"bufio"
	"bytes"
	"compress/gzip"
	"context"
	"debug/elf"
	"debug/pe"
	"encoding/binary"
	"errors"
	"fmt"
	"io"
	"os"
	"path"
	"regexp"
	"strings"
	"unicode/utf16"
)

func readProgram(_ context.Context, _ *Service, f File, rec Record) error {
	switch extOf(f.Path) {
	case "exe", "dll":
		return readPE(f.Real, rec)
	case "appimage":
		return readELF(f.Real, rec)
	case "deb":
		return readDeb(f, rec)
	case "rpm":
		return readRPM(f.Real, rec)
	case "jar", "apk":
		return readJar(f.Real, rec)
	}
	return nil
}

var peMachines = map[uint16]string{
	pe.IMAGE_FILE_MACHINE_I386: "x86", pe.IMAGE_FILE_MACHINE_AMD64: "x86-64", pe.IMAGE_FILE_MACHINE_ARM64: "ARM64",
	pe.IMAGE_FILE_MACHINE_ARMNT: "ARM", pe.IMAGE_FILE_MACHINE_ARM: "ARM",
}

func readPE(p string, rec Record) error {
	pf, err := pe.Open(p)
	if err != nil {
		return err
	}
	defer pf.Close()
	rec["arch"] = peMachines[pf.Machine]
	var dirs []pe.DataDirectory
	switch oh := pf.OptionalHeader.(type) {
	case *pe.OptionalHeader32:
		dirs = oh.DataDirectory[:oh.NumberOfRvaAndSizes]
	case *pe.OptionalHeader64:
		dirs = oh.DataDirectory[:oh.NumberOfRvaAndSizes]
	}
	rec["signed"] = len(dirs) > pe.IMAGE_DIRECTORY_ENTRY_SECURITY && dirs[pe.IMAGE_DIRECTORY_ENTRY_SECURITY].Size > 0
	if s := pf.Section(".rsrc"); s != nil {
		if data, err := s.Data(); err == nil {
			info := versionStrings(data)
			rec["version"] = cleanVersion(firstOf(info["ProductVersion"], info["FileVersion"]))
			rec["vendor"] = info["CompanyName"]
		}
	}
	return nil
}

func firstOf(s ...string) string {
	for _, v := range s {
		if v != "" {
			return v
		}
	}
	return ""
}

// cleanVersion drops the trailing zero parts Windows pads versions with,
// so 1.2.0.0 reads 1.2.
func cleanVersion(v string) string {
	v = strings.TrimSpace(strings.ReplaceAll(v, ", ", "."))
	for strings.Count(v, ".") > 1 && strings.HasSuffix(v, ".0") {
		v = strings.TrimSuffix(v, ".0")
	}
	return v
}

// versionStrings finds the StringFileInfo values of a version resource in
// the resource section. Each value is a UTF-16 key, padded to four bytes,
// followed by its UTF-16 text, which is how every compiler writes them.
func versionStrings(rsrc []byte) map[string]string {
	out := map[string]string{}
	for _, key := range []string{"CompanyName", "ProductVersion", "FileVersion"} {
		needle := utf16le(key + "\x00")
		i := bytes.Index(rsrc, needle)
		if i < 0 {
			continue
		}
		j := i + len(needle)
		for j%4 != 0 && j+1 < len(rsrc) && rsrc[j] == 0 && rsrc[j+1] == 0 {
			j += 2
		}
		var units []uint16
		for ; j+1 < len(rsrc); j += 2 {
			u := binary.LittleEndian.Uint16(rsrc[j:])
			if u == 0 || len(units) > 256 {
				break
			}
			units = append(units, u)
		}
		out[key] = strings.TrimSpace(string(utf16.Decode(units)))
	}
	return out
}

func utf16le(s string) []byte {
	units := utf16.Encode([]rune(s))
	b := make([]byte, 2*len(units))
	for i, u := range units {
		binary.LittleEndian.PutUint16(b[2*i:], u)
	}
	return b
}

var elfMachines = map[elf.Machine]string{
	elf.EM_X86_64: "x86-64", elf.EM_386: "x86", elf.EM_AARCH64: "ARM64", elf.EM_ARM: "ARM", elf.EM_RISCV: "RISC-V",
}

func readELF(p string, rec Record) error {
	ef, err := elf.Open(p)
	if err != nil {
		return err
	}
	defer ef.Close()
	rec["arch"] = elfMachines[ef.Machine]
	rec["signed"] = ef.Section(".sha256_sig") != nil || ef.Section(".sig_key") != nil
	return nil
}

var debArches = map[string]string{
	"amd64": "x86-64", "i386": "x86", "arm64": "ARM64", "armhf": "ARM", "armel": "ARM", "riscv64": "RISC-V", "all": "Alle",
}

// debName is the file name Debian packages get: name_version_arch.deb.
var debName = regexp.MustCompile(`^[^_]+_([^_]+)_([^_.]+)\.deb$`)

// readDeb reads the control file of a package. Only control archives packed
// with gzip are opened, the version and the architecture of the others
// come from the file name.
func readDeb(f File, rec Record) error {
	if m := debName.FindStringSubmatch(path.Base(f.Path)); m != nil {
		rec["version"], rec["arch"] = m[1], debArches[m[2]]
	}
	fh, err := os.Open(f.Real)
	if err != nil {
		return err
	}
	defer fh.Close()
	r := bufio.NewReader(fh)
	magic := make([]byte, 8)
	if _, err := io.ReadFull(r, magic); err != nil || string(magic) != "!<arch>\n" {
		return errors.New("not an ar archive")
	}
	rec["signed"] = false
	head := make([]byte, 60)
	for {
		if _, err := io.ReadFull(r, head); err != nil {
			return nil
		}
		name := strings.TrimRight(strings.TrimSpace(string(head[:16])), "/")
		var size int64
		if _, err := fmt.Sscan(strings.TrimSpace(string(head[48:58])), &size); err != nil || size < 0 {
			return errors.New("broken ar member")
		}
		body := io.LimitReader(r, size)
		switch {
		case strings.HasPrefix(name, "_gpg"):
			rec["signed"] = true
		case name == "control.tar.gz":
			if fields, err := debControl(body); err == nil {
				rec["version"] = firstOf(fields["Version"], str(rec["version"]))
				rec["vendor"] = maintainer(firstOf(fields["Vendor"], fields["Maintainer"]))
				if a := debArches[fields["Architecture"]]; a != "" {
					rec["arch"] = a
				}
			}
		}
		if _, err := io.Copy(io.Discard, body); err != nil {
			return nil
		}
		if size%2 == 1 {
			_, _ = r.Discard(1)
		}
	}
}

func str(v any) string {
	s, _ := v.(string)
	return s
}

func debControl(r io.Reader) (map[string]string, error) {
	gz, err := gzip.NewReader(r)
	if err != nil {
		return nil, err
	}
	defer gz.Close()
	tr := tar.NewReader(gz)
	for {
		h, err := tr.Next()
		if err != nil {
			return nil, err
		}
		if path.Clean(h.Name) != "control" {
			continue
		}
		raw, err := io.ReadAll(io.LimitReader(tr, 1<<20))
		if err != nil {
			return nil, err
		}
		fields := map[string]string{}
		for _, line := range strings.Split(string(raw), "\n") {
			if k, v, ok := strings.Cut(line, ":"); ok && !strings.HasPrefix(line, " ") {
				fields[k] = strings.TrimSpace(v)
			}
		}
		return fields, nil
	}
}

// maintainer drops the mail address from "Name <mail>".
func maintainer(s string) string {
	name, _, _ := strings.Cut(s, "<")
	return strings.TrimSpace(name)
}

// RPM header tags.
const (
	rpmVersion = 1001
	rpmRelease = 1002
	rpmVendor  = 1011
	rpmArch    = 1022
)

var rpmSignatureTags = map[uint32]bool{267: true, 268: true, 1002: true, 1005: true}

var rpmArches = map[string]string{"x86_64": "x86-64", "i386": "x86", "i686": "x86", "aarch64": "ARM64", "armv7hl": "ARM", "noarch": "Alle", "riscv64": "RISC-V"}

func readRPM(p string, rec Record) error {
	fh, err := os.Open(p)
	if err != nil {
		return err
	}
	defer fh.Close()
	lead := readAt(fh, 0, 96)
	if len(lead) < 96 || !bytes.Equal(lead[:4], []byte{0xED, 0xAB, 0xEE, 0xDB}) {
		return errors.New("not an rpm package")
	}
	sig, sigEnd, err := rpmHeader(fh, 96)
	if err != nil {
		return err
	}
	signed := false
	for tag := range sig {
		if rpmSignatureTags[tag] {
			signed = true
		}
	}
	rec["signed"] = signed
	// The main header starts on the next eight byte boundary.
	main, _, err := rpmHeader(fh, (sigEnd+7)&^7)
	if err != nil {
		return err
	}
	version := main[rpmVersion]
	if rel := main[rpmRelease]; version != "" && rel != "" {
		version += "-" + rel
	}
	rec["version"], rec["vendor"] = version, main[rpmVendor]
	rec["arch"] = firstOf(rpmArches[main[rpmArch]], main[rpmArch])
	return nil
}

// rpmHeader reads the string tags of the header at off and returns where
// it ends.
func rpmHeader(r io.ReaderAt, off int64) (map[uint32]string, int64, error) {
	h := readAt(r, off, 16)
	if len(h) < 16 || !bytes.Equal(h[:3], []byte{0x8E, 0xAD, 0xE8}) {
		return nil, 0, errors.New("broken rpm header")
	}
	n := int64(binary.BigEndian.Uint32(h[8:]))
	size := int64(binary.BigEndian.Uint32(h[12:]))
	if n > 10000 || size > 64<<20 {
		return nil, 0, errors.New("rpm header too large")
	}
	index := readAt(r, off+16, int(n*16))
	store := readAt(r, off+16+n*16, int(size))
	out := map[uint32]string{}
	for i := 0; i+16 <= len(index); i += 16 {
		tag := binary.BigEndian.Uint32(index[i:])
		typ := binary.BigEndian.Uint32(index[i+4:])
		at := int(binary.BigEndian.Uint32(index[i+8:]))
		out[tag] = ""
		// Types 6, 8 and 9 are a string, a string array and an i18n string.
		if (typ == 6 || typ == 8 || typ == 9) && at < len(store) {
			s := store[at:]
			if z := bytes.IndexByte(s, 0); z >= 0 {
				s = s[:z]
			}
			out[tag] = string(s)
		}
	}
	return out, off + 16 + n*16 + size, nil
}

// readJar reads the manifest of a Java archive. A signed jar and an
// Android package signed the old way carry a signature file under
// META-INF, the newer Android scheme a signing block before the zip's
// central directory.
func readJar(p string, rec Record) error {
	zr, err := zip.OpenReader(p)
	if err != nil {
		return err
	}
	defer zr.Close()
	rec["signed"] = apkSigned(p)
	for _, zf := range zr.File {
		name := strings.ToUpper(zf.Name)
		if strings.HasPrefix(name, "META-INF/") && (strings.HasSuffix(name, ".SF") || strings.HasSuffix(name, ".RSA") || strings.HasSuffix(name, ".EC")) {
			rec["signed"] = true
		}
		if name != "META-INF/MANIFEST.MF" {
			continue
		}
		raw, err := readPart(zf)
		if err != nil {
			continue
		}
		fields := map[string]string{}
		for _, line := range strings.Split(strings.ReplaceAll(string(raw), "\r\n", "\n"), "\n") {
			if k, v, ok := strings.Cut(line, ":"); ok {
				fields[k] = strings.TrimSpace(v)
			}
		}
		rec["version"] = firstOf(fields["Implementation-Version"], fields["Bundle-Version"])
		rec["vendor"] = firstOf(fields["Implementation-Vendor"], fields["Bundle-Vendor"])
	}
	return nil
}

// apkSigned looks for the APK signing block, whose magic ends right where
// the central directory starts.
func apkSigned(p string) bool {
	fh, err := os.Open(p)
	if err != nil {
		return false
	}
	defer fh.Close()
	info, err := fh.Stat()
	if err != nil || info.Size() < 22 {
		return false
	}
	tailSize := min(info.Size(), 1<<16+22)
	tail := readAt(fh, info.Size()-tailSize, int(tailSize))
	end := bytes.LastIndex(tail, []byte("PK\x05\x06"))
	if end < 0 || end+20 > len(tail) {
		return false
	}
	dir := int64(binary.LittleEndian.Uint32(tail[end+16:]))
	if dir < 16 {
		return false
	}
	return string(readAt(fh, dir-16, 16)) == "APK Sig Block 42"
}
