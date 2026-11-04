package meta

import (
	"archive/tar"
	"archive/zip"
	"bytes"
	"compress/gzip"
	"encoding/binary"
	"fmt"
	"hash/crc32"
	"image"
	"image/color"
	"image/jpeg"
	"image/png"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func writeFile(t *testing.T, p string, data []byte) string {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(p, data, 0o644); err != nil {
		t.Fatal(err)
	}
	return p
}

// tiffBuilder lays out a little-endian TIFF block as EXIF wants it: IFDs
// with their entries, and values longer than four bytes behind them.
type tiffBuilder struct {
	ifds [][]ifdEntry
}

type ifdEntry struct {
	tag, typ uint16
	count    uint32
	data     []byte
	// next points the entry at another IFD of the builder.
	next int
}

func ascii(tag uint16, s string) ifdEntry {
	return ifdEntry{tag: tag, typ: 2, count: uint32(len(s) + 1), data: append([]byte(s), 0), next: -1}
}

func short(tag uint16, v uint16) ifdEntry {
	b := make([]byte, 2)
	binary.LittleEndian.PutUint16(b, v)
	return ifdEntry{tag: tag, typ: 3, count: 1, data: b, next: -1}
}

func rational(tag uint16, pairs ...uint32) ifdEntry {
	b := make([]byte, 4*len(pairs))
	for i, v := range pairs {
		binary.LittleEndian.PutUint32(b[4*i:], v)
	}
	return ifdEntry{tag: tag, typ: 5, count: uint32(len(pairs) / 2), data: b, next: -1}
}

func pointer(tag uint16, ifd int) ifdEntry {
	return ifdEntry{tag: tag, typ: 4, count: 1, data: make([]byte, 4), next: ifd}
}

func (tb *tiffBuilder) bytes() []byte {
	// Offsets of every IFD, each followed by its own long values.
	offsets := make([]int, len(tb.ifds))
	pos := 8
	for i, ifd := range tb.ifds {
		offsets[i] = pos
		pos += 2 + 12*len(ifd) + 4
		for _, e := range ifd {
			if len(e.data) > 4 {
				pos += len(e.data) + len(e.data)%2
			}
		}
	}
	out := make([]byte, pos)
	copy(out, "II*\x00")
	binary.LittleEndian.PutUint32(out[4:], 8)
	for i, ifd := range tb.ifds {
		at := offsets[i]
		binary.LittleEndian.PutUint16(out[at:], uint16(len(ifd)))
		extra := at + 2 + 12*len(ifd) + 4
		for k, e := range ifd {
			p := at + 2 + 12*k
			binary.LittleEndian.PutUint16(out[p:], e.tag)
			binary.LittleEndian.PutUint16(out[p+2:], e.typ)
			binary.LittleEndian.PutUint32(out[p+4:], e.count)
			data := e.data
			if e.next >= 0 {
				data = make([]byte, 4)
				binary.LittleEndian.PutUint32(data, uint32(offsets[e.next]))
			}
			if len(data) > 4 {
				binary.LittleEndian.PutUint32(out[p+8:], uint32(extra))
				copy(out[extra:], data)
				extra += len(data) + len(data)%2
			} else {
				copy(out[p+8:], data)
			}
		}
	}
	return out
}

// photoJPEG is a 64 x 48 JPEG taken with a made up camera in Vienna,
// turned on its side.
func photoJPEG(t *testing.T) []byte {
	t.Helper()
	img := image.NewRGBA(image.Rect(0, 0, 64, 48))
	for i := range img.Pix {
		img.Pix[i] = byte(i)
	}
	var buf bytes.Buffer
	if err := jpeg.Encode(&buf, img, nil); err != nil {
		t.Fatal(err)
	}
	tb := tiffBuilder{ifds: [][]ifdEntry{
		{
			ascii(0x010F, "FUJIFILM"), ascii(0x0110, "X100VI"), short(0x0112, 6),
			rational(0x011A, 300, 1), short(0x0128, 2), pointer(0x8769, 1), pointer(0x8825, 2),
		},
		{
			rational(0x829A, 1, 250), rational(0x829D, 28, 10), short(0x8827, 400),
			ascii(0x9003, "2026:06:21 18:30:00"), short(0x9209, 0x10), rational(0x920A, 23, 1),
			ascii(0xA434, "Fujinon 23 mm F2"),
		},
		{
			ascii(0x0001, "N"), rational(0x0002, 48, 1, 12, 1, 3060, 100),
			ascii(0x0003, "E"), rational(0x0004, 16, 1, 22, 1, 2316, 100),
		},
	}}
	exif := append([]byte("Exif\x00\x00"), tb.bytes()...)
	app1 := []byte{0xFF, 0xE1, 0, 0}
	binary.BigEndian.PutUint16(app1[2:], uint16(len(exif)+2))
	jpg := buf.Bytes()
	out := append([]byte{}, jpg[:2]...)
	out = append(out, app1...)
	out = append(out, exif...)
	return append(out, jpg[2:]...)
}

// densePNG is a 10 x 20 RGBA PNG that states 300 dpi in its pHYs chunk.
func densePNG(t *testing.T) []byte {
	t.Helper()
	img := image.NewNRGBA(image.Rect(0, 0, 10, 20))
	img.Set(1, 1, color.NRGBA{R: 255, A: 128})
	var buf bytes.Buffer
	if err := png.Encode(&buf, img); err != nil {
		t.Fatal(err)
	}
	raw := buf.Bytes()
	body := make([]byte, 9)
	binary.BigEndian.PutUint32(body, 11811)
	binary.BigEndian.PutUint32(body[4:], 11811)
	body[8] = 1
	chunk := make([]byte, 4)
	binary.BigEndian.PutUint32(chunk, 9)
	chunk = append(chunk, "pHYs"...)
	chunk = append(chunk, body...)
	sum := make([]byte, 4)
	binary.BigEndian.PutUint32(sum, crc32.ChecksumIEEE(chunk[4:]))
	chunk = append(chunk, sum...)
	// The IHDR chunk ends 33 bytes in.
	return append(append(append([]byte{}, raw[:33]...), chunk...), raw[33:]...)
}

// twoPagePDF is a small PDF with an information dictionary.
func twoPagePDF() []byte {
	objs := []string{
		"<< /Type /Catalog /Pages 2 0 R >>",
		"<< /Type /Pages /Kids [3 0 R 4 0 R] /Count 2 >>",
		"<< /Type /Page /Parent 2 0 R /MediaBox [0 0 200 200] >>",
		"<< /Type /Page /Parent 2 0 R /MediaBox [0 0 200 200] >>",
		"<< /Title (Steuer 2025) /Author (jdp) /Subject (Belege) /Creator (LibreOffice Writer) >>",
	}
	var b bytes.Buffer
	b.WriteString("%PDF-1.4\n")
	offs := make([]int, len(objs))
	for i, o := range objs {
		offs[i] = b.Len()
		fmt.Fprintf(&b, "%d 0 obj\n%s\nendobj\n", i+1, o)
	}
	xref := b.Len()
	fmt.Fprintf(&b, "xref\n0 %d\n0000000000 65535 f \n", len(objs)+1)
	for _, o := range offs {
		fmt.Fprintf(&b, "%010d 00000 n \n", o)
	}
	fmt.Fprintf(&b, "trailer\n<< /Size %d /Root 1 0 R /Info 5 0 R >>\nstartxref\n%d\n%%%%EOF\n", len(objs)+1, xref)
	return b.Bytes()
}

func zipOf(t *testing.T, files map[string]string, encrypted ...string) []byte {
	t.Helper()
	var buf bytes.Buffer
	zw := zip.NewWriter(&buf)
	for name, body := range files {
		h := &zip.FileHeader{Name: name, Method: zip.Deflate}
		for _, e := range encrypted {
			if e == name {
				h.Flags |= 1
			}
		}
		w, err := zw.CreateHeader(h)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := w.Write([]byte(body)); err != nil {
			t.Fatal(err)
		}
	}
	if err := zw.Close(); err != nil {
		t.Fatal(err)
	}
	return buf.Bytes()
}

func docx(t *testing.T) []byte {
	return zipOf(t, map[string]string{
		"word/document.xml": "<w:document/>",
		"docProps/core.xml": `<?xml version="1.0" encoding="UTF-8"?>
<cp:coreProperties xmlns:cp="http://schemas.openxmlformats.org/package/2006/metadata/core-properties" xmlns:dc="http://purl.org/dc/elements/1.1/">
<dc:title>Angebot Dachboden</dc:title><dc:subject>Angebot</dc:subject><dc:creator>Anna</dc:creator><cp:lastModifiedBy>jdp</cp:lastModifiedBy></cp:coreProperties>`,
		"docProps/app.xml": `<?xml version="1.0" encoding="UTF-8"?>
<Properties xmlns="http://schemas.openxmlformats.org/officeDocument/2006/extended-properties"><Application>Microsoft Office Word</Application><Pages>3</Pages><Words>812</Words></Properties>`,
	})
}

func odt(t *testing.T) []byte {
	return zipOf(t, map[string]string{
		"mimetype": "application/vnd.oasis.opendocument.text",
		"meta.xml": `<?xml version="1.0" encoding="UTF-8"?>
<office:document-meta xmlns:office="urn:oasis:names:tc:opendocument:xmlns:office:1.0" xmlns:meta="urn:oasis:names:tc:opendocument:xmlns:meta:1.0" xmlns:dc="http://purl.org/dc/elements/1.1/">
<office:meta><meta:generator>LibreOffice/7.6.4.1$Linux_X86_64 LibreOffice_project/e19e193f88cd6c0525a17fb7a176ed8e6a3e2aa1</meta:generator>
<dc:title>Vertrag</dc:title><meta:initial-creator>Verwaltung</meta:initial-creator><dc:creator>jdp</dc:creator>
<meta:document-statistic meta:page-count="2" meta:word-count="140"/></office:meta></office:document-meta>`,
	})
}

func tarOf(t *testing.T, files map[string]string, gz bool) []byte {
	t.Helper()
	var buf bytes.Buffer
	var w = &buf
	var zw *gzip.Writer
	tw := tar.NewWriter(w)
	if gz {
		zw = gzip.NewWriter(w)
		tw = tar.NewWriter(zw)
	}
	if err := tw.WriteHeader(&tar.Header{Name: "docs/", Typeflag: tar.TypeDir, Mode: 0o755}); err != nil {
		t.Fatal(err)
	}
	for name, body := range files {
		if err := tw.WriteHeader(&tar.Header{Name: name, Typeflag: tar.TypeReg, Mode: 0o644, Size: int64(len(body))}); err != nil {
			t.Fatal(err)
		}
		if _, err := tw.Write([]byte(body)); err != nil {
			t.Fatal(err)
		}
	}
	if err := tw.Close(); err != nil {
		t.Fatal(err)
	}
	if zw != nil {
		if err := zw.Close(); err != nil {
			t.Fatal(err)
		}
	}
	return buf.Bytes()
}

// isoImage has the volume descriptor of a CD image and an El Torito boot
// record behind it.
func isoImage() []byte {
	b := make([]byte, 0x9000)
	b[0x8000] = 1
	copy(b[0x8001:], "CD001")
	copy(b[0x8028:], "UNRAID_7_3                      ")
	copy(b[0x8801:], "CD001")
	copy(b[0x8807:], "EL TORITO SPECIFICATION")
	return b
}

func fat32Image() []byte {
	b := make([]byte, 4096)
	copy(b[3:], "MSDOS5.0")
	copy(b[0x47:], "BACKUP     ")
	copy(b[0x52:], "FAT32   ")
	b[510], b[511] = 0x55, 0xAA
	return b
}

// mbrExt4 is a disk with an active partition at sector 2048 that holds an
// ext4 file system.
func mbrExt4() []byte {
	const start = 2048 * 512
	b := make([]byte, start+0x2000)
	b[446] = 0x80
	b[446+4] = 0x83
	binary.LittleEndian.PutUint32(b[446+8:], 2048)
	b[510], b[511] = 0x55, 0xAA
	sb := b[start+1024:]
	binary.LittleEndian.PutUint16(sb[56:], 0xEF53)
	binary.LittleEndian.PutUint32(sb[92:], 0x4)
	binary.LittleEndian.PutUint32(sb[96:], 0x40)
	copy(sb[120:], "vmdisk")
	return b
}

// debPackage is an ar archive with a gzip control archive.
func debPackage(t *testing.T, control string, signed bool) []byte {
	t.Helper()
	member := func(name string, body []byte) []byte {
		h := fmt.Sprintf("%-16s%-12d%-6d%-6d%-8s%-10d`\n", name, 0, 0, 0, "100644", len(body))
		out := append([]byte(h), body...)
		if len(body)%2 == 1 {
			out = append(out, '\n')
		}
		return out
	}
	ctl := tarOf(t, map[string]string{"./control": control}, true)
	b := []byte("!<arch>\n")
	b = append(b, member("debian-binary", []byte("2.0\n"))...)
	b = append(b, member("control.tar.gz", ctl)...)
	b = append(b, member("data.tar.xz", []byte("x"))...)
	if signed {
		b = append(b, member("_gpgorigin", []byte("sig"))...)
	}
	return b
}

// rpmPackage has a lead, a signature header with an RSA signature tag and
// a main header with the version, release, vendor and architecture.
func rpmPackage() []byte {
	header := func(tags map[uint32]string) []byte {
		var index, store []byte
		for _, tag := range []uint32{1001, 1002, 1011, 1022, 268} {
			v, ok := tags[tag]
			if !ok {
				continue
			}
			e := make([]byte, 16)
			binary.BigEndian.PutUint32(e, tag)
			binary.BigEndian.PutUint32(e[4:], 6)
			binary.BigEndian.PutUint32(e[8:], uint32(len(store)))
			binary.BigEndian.PutUint32(e[12:], 1)
			index = append(index, e...)
			store = append(store, append([]byte(v), 0)...)
		}
		h := []byte{0x8E, 0xAD, 0xE8, 0x01, 0, 0, 0, 0}
		h = binary.BigEndian.AppendUint32(h, uint32(len(index)/16))
		h = binary.BigEndian.AppendUint32(h, uint32(len(store)))
		return append(append(h, index...), store...)
	}
	lead := make([]byte, 96)
	copy(lead, []byte{0xED, 0xAB, 0xEE, 0xDB})
	sig := header(map[uint32]string{268: "rsa"})
	b := append(lead, sig...)
	for len(b)%8 != 0 {
		b = append(b, 0)
	}
	return append(b, header(map[uint32]string{1001: "2.4.1", 1002: "3.fc41", 1011: "Fedora Project", 1022: "x86_64"})...)
}

func lines(n int) string {
	return strings.Repeat("eins zwei drei\n", n)
}
