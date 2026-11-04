package meta

import (
	"bytes"
	"context"
	"database/sql"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"testing"
	"time"

	_ "modernc.org/sqlite"

	"github.com/junkerderprovinz/cannoncommander/backend/internal/database/migrate"
)

func cacheDB(t *testing.T) *sql.DB {
	t.Helper()
	db, err := sql.Open("sqlite", filepath.Join(t.TempDir(), "index.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { db.Close() })
	var mine []migrate.Migration
	for _, m := range migrate.Registered(migrate.Index) {
		if m.Name == "meta/001_cache" {
			mine = append(mine, m)
		}
	}
	if _, err := migrate.ApplyList(context.Background(), db, mine); err != nil {
		t.Fatal(err)
	}
	return db
}

func newService(t *testing.T) *Service {
	t.Helper()
	db := cacheDB(t)
	s := New(func() *sql.DB { return db })
	s.devices = nil
	return s
}

func fileOf(t *testing.T, p string) File {
	t.Helper()
	info, err := os.Stat(p)
	if err != nil {
		t.Fatal(err)
	}
	return File{Path: "/mnt/user" + filepath.ToSlash(p[len(filepath.VolumeName(p)):]), Real: p, Size: info.Size(), MTime: info.ModTime(), Dir: info.IsDir()}
}

func valuesOf(t *testing.T, s *Service, p string, cols ...string) Values {
	t.Helper()
	f := fileOf(t, p)
	return s.Columns(context.Background(), []File{f}, cols)[f.Path]
}

func expect(t *testing.T, got Values, want Values) {
	t.Helper()
	for k, w := range want {
		if g, ok := got[k]; !ok || g != w {
			t.Errorf("%s = %#v, want %#v", k, got[k], w)
		}
	}
}

func TestPhotoColumnsComeFromExif(t *testing.T) {
	s := newService(t)
	p := writeFile(t, filepath.Join(t.TempDir(), "DSCF0001.jpg"), photoJPEG(t))
	got := valuesOf(t, s, p, "dimensions", "camera", "make", "lens", "focal", "aperture", "exposure", "iso", "flash", "taken", "place", "orientation", "depth", "dpi")
	expect(t, got, Values{
		"dimensions": "64 × 48", "camera": "X100VI", "make": "Fujifilm", "lens": "Fujinon 23 mm F2",
		"focal": "23 mm", "aperture": "f/2,8", "exposure": "1/250 s", "iso": int64(400), "flash": "Kein Blitz",
		"taken": time.Date(2026, 6, 21, 18, 30, 0, 0, time.Local).UnixMilli(), "place": "Wien",
		"orientation": "Hochformat", "depth": "24 Bit", "dpi": "300 dpi",
	})
}

func TestPNGStatesItsDensity(t *testing.T) {
	s := newService(t)
	p := writeFile(t, filepath.Join(t.TempDir(), "plan.png"), densePNG(t))
	got := valuesOf(t, s, p, "dimensions", "depth", "dpi", "orientation", "camera", "flash")
	expect(t, got, Values{"dimensions": "10 × 20", "depth": "32 Bit", "dpi": "300 dpi", "orientation": "Hochformat"})
	if _, ok := got["flash"]; ok {
		t.Error("a picture without camera details claims a flash state")
	}
}

func TestOfficeFilesGiveTheirDetails(t *testing.T) {
	s := newService(t)
	dir := t.TempDir()
	cols := []string{"pages", "words", "author", "lastauthor", "doctitle", "subject", "app", "encrypted"}
	expect(t, valuesOf(t, s, writeFile(t, filepath.Join(dir, "angebot.docx"), docx(t)), cols...), Values{
		"pages": int64(3), "words": int64(812), "author": "Anna", "lastauthor": "jdp", "doctitle": "Angebot Dachboden",
		"subject": "Angebot", "app": "Microsoft Office Word", "encrypted": false,
	})
	expect(t, valuesOf(t, s, writeFile(t, filepath.Join(dir, "vertrag.odt"), odt(t)), cols...), Values{
		"pages": int64(2), "words": int64(140), "author": "Verwaltung", "lastauthor": "jdp", "doctitle": "Vertrag",
		"app": "LibreOffice 7.6.4.1", "encrypted": false,
	})
	locked := append(append([]byte{}, oleMagic...), make([]byte, 512)...)
	expect(t, valuesOf(t, s, writeFile(t, filepath.Join(dir, "geheim.xlsx"), locked), cols...), Values{"encrypted": true})
}

func TestTextFilesAreCounted(t *testing.T) {
	s := newService(t)
	dir := t.TempDir()
	cases := []struct {
		name, body string
		want       Values
	}{
		{"notizen.txt", lines(3), Values{"lines": int64(3), "words": int64(9), "encoding": "ASCII"}},
		{"ohne-ende.md", "eins\nzwei drei", Values{"lines": int64(2), "words": int64(3)}},
		{"grüße.txt", "Grüße aus Wien\n", Values{"encoding": "UTF-8"}},
		{"bom.json", "\xEF\xBB\xBF{}\n", Values{"encoding": "UTF-8 mit BOM"}},
		{"alt.cfg", "Gr\xFC\xDFe\n", Values{"encoding": "Windows-1252"}},
		{"syslog.log.1", "a\nb\n", Values{"lines": int64(2)}},
	}
	for _, c := range cases {
		expect(t, valuesOf(t, s, writeFile(t, filepath.Join(dir, c.name), []byte(c.body)), "lines", "words", "encoding"), c.want)
	}
}

func TestArchivesListTheirContents(t *testing.T) {
	s := newService(t)
	dir := t.TempDir()
	cols := []string{"entries", "unpacked", "method", "archcrypt", "ratio"}
	body := string(bytes.Repeat([]byte("cannon "), 2000))
	z := writeFile(t, filepath.Join(dir, "fotos.zip"), zipOf(t, map[string]string{"a.txt": body, "b.txt": body, "c/": ""}, "b.txt"))
	got := valuesOf(t, s, z, cols...)
	expect(t, got, Values{"entries": int64(2), "unpacked": int64(28000), "method": "Deflate", "archcrypt": true})
	if r, _ := got["ratio"].(string); r == "" || r == "0 %" {
		t.Errorf("ratio = %q", r)
	}
	files := map[string]string{"docs/a.md": "# A\n", "docs/b.md": "## B\n"}
	expect(t, valuesOf(t, s, writeFile(t, filepath.Join(dir, "docs.tar"), tarOf(t, files, false)), cols...),
		Values{"entries": int64(2), "unpacked": int64(9), "method": "Ohne", "archcrypt": false})
	expect(t, valuesOf(t, s, writeFile(t, filepath.Join(dir, "docs.tar.gz"), tarOf(t, files, true)), cols...),
		Values{"entries": int64(2), "unpacked": int64(9), "method": "Gzip"})
}

func TestDiskImagesTellTheirFileSystem(t *testing.T) {
	s := newService(t)
	dir := t.TempDir()
	cols := []string{"fs", "volume", "bootable"}
	expect(t, valuesOf(t, s, writeFile(t, filepath.Join(dir, "unraid.iso"), isoImage()), cols...),
		Values{"fs": "ISO 9660", "volume": "UNRAID_7_3", "bootable": true})
	got := valuesOf(t, s, writeFile(t, filepath.Join(dir, "stick.img"), fat32Image()), cols...)
	expect(t, got, Values{"fs": "FAT32", "volume": "BACKUP"})
	expect(t, valuesOf(t, s, writeFile(t, filepath.Join(dir, "vm.img"), mbrExt4()), cols...),
		Values{"fs": "ext4", "volume": "vmdisk", "bootable": true})
}

func TestPackagesTellVersionAndVendor(t *testing.T) {
	s := newService(t)
	dir := t.TempDir()
	cols := []string{"version", "vendor", "arch", "signed"}
	control := "Package: plex\nVersion: 1.41.2\nArchitecture: amd64\nMaintainer: Plex Inc. <support@plex.tv>\nDescription: media\n more text\n"
	expect(t, valuesOf(t, s, writeFile(t, filepath.Join(dir, "plex_1.41.2_amd64.deb"), debPackage(t, control, true)), cols...),
		Values{"version": "1.41.2", "vendor": "Plex Inc.", "arch": "x86-64", "signed": true})
	expect(t, valuesOf(t, s, writeFile(t, filepath.Join(dir, "tool.rpm"), rpmPackage()), cols...),
		Values{"version": "2.4.1-3.fc41", "vendor": "Fedora Project", "arch": "x86-64", "signed": true})
	jar := zipOf(t, map[string]string{"META-INF/MANIFEST.MF": "Manifest-Version: 1.0\r\nImplementation-Version: 3.2\r\nImplementation-Vendor: Apache\r\n"})
	expect(t, valuesOf(t, s, writeFile(t, filepath.Join(dir, "lib.jar"), jar), cols...),
		Values{"version": "3.2", "vendor": "Apache", "signed": false})

	// The test binary is a program of the platform it runs on.
	self, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	raw, err := os.ReadFile(self)
	if err != nil {
		t.Fatal(err)
	}
	want := map[string]string{"amd64": "x86-64", "arm64": "ARM64", "386": "x86"}[runtime.GOARCH]
	name := "self.appimage"
	if runtime.GOOS == "windows" {
		name = "self.exe"
	}
	if runtime.GOOS == "windows" || runtime.GOOS == "linux" {
		expect(t, valuesOf(t, s, writeFile(t, filepath.Join(dir, name), raw), "arch"), Values{"arch": want})
	}
}

func TestVersionResourceStrings(t *testing.T) {
	var rsrc []byte
	for _, kv := range [][2]string{{"CompanyName", "VideoLAN"}, {"ProductVersion", "3.0.21.0"}} {
		rsrc = append(rsrc, make([]byte, 6)...)
		rsrc = append(rsrc, utf16le(kv[0]+"\x00")...)
		for len(rsrc)%4 != 0 {
			rsrc = append(rsrc, 0)
		}
		rsrc = append(rsrc, utf16le(kv[1]+"\x00")...)
		for len(rsrc)%4 != 0 {
			rsrc = append(rsrc, 0)
		}
	}
	got := versionStrings(rsrc)
	if got["CompanyName"] != "VideoLAN" || cleanVersion(got["ProductVersion"]) != "3.0.21" {
		t.Errorf("got %v", got)
	}
}

func TestChecksumsOfSmallFiles(t *testing.T) {
	s := newService(t)
	p := writeFile(t, filepath.Join(t.TempDir(), "abc.bin"), []byte("abc"))
	expect(t, valuesOf(t, s, p, "sha256", "sha1", "md5", "crc32"), Values{
		"sha256": "ba7816bf8f01cfea414140de5dae2223b00361a396177a9cb410ff61f20015ad",
		"sha1":   "a9993e364706816aba3e25717850c26c9cd0d89d",
		"md5":    "900150983cd24fb0d6963f7d28e17f72",
		"crc32":  "352441c2",
	})
}

func TestCacheServesUntilTheFileChanges(t *testing.T) {
	s := newService(t)
	p := writeFile(t, filepath.Join(t.TempDir(), "liste.txt"), []byte("a\nb\n"))
	stamp := time.Date(2026, 3, 1, 12, 0, 0, 0, time.UTC)
	if err := os.Chtimes(p, stamp, stamp); err != nil {
		t.Fatal(err)
	}
	expect(t, valuesOf(t, s, p, "lines"), Values{"lines": int64(2)})

	// Same size and time: the cache answers without reading the file.
	writeFile(t, p, []byte("abcd"))
	if err := os.Chtimes(p, stamp, stamp); err != nil {
		t.Fatal(err)
	}
	expect(t, valuesOf(t, s, p, "lines"), Values{"lines": int64(2)})

	later := stamp.Add(time.Hour)
	if err := os.Chtimes(p, later, later); err != nil {
		t.Fatal(err)
	}
	expect(t, valuesOf(t, s, p, "lines"), Values{"lines": int64(1)})
}

func TestEpisodesComeFromTheName(t *testing.T) {
	s := newService(t)
	dir := t.TempDir()
	expect(t, valuesOf(t, s, writeFile(t, filepath.Join(dir, "Serie.S02E05.1080p.mkv"), nil), "season", "episode"),
		Values{"season": int64(2), "episode": int64(5)})
	expect(t, valuesOf(t, s, writeFile(t, filepath.Join(dir, "serie 1x07.mp4"), nil), "season", "episode"),
		Values{"season": int64(1), "episode": int64(7)})
	if got := valuesOf(t, s, writeFile(t, filepath.Join(dir, "S02E05.txt"), nil), "season"); len(got) != 0 {
		t.Errorf("a text file has %v", got)
	}
	if got := valuesOf(t, s, writeFile(t, filepath.Join(dir, "a.txt"), []byte("x")), "accessed"); got["accessed"] == nil {
		t.Error("no access time")
	}
}

func TestFFprobeAnswerBecomesColumns(t *testing.T) {
	raw := `{"streams":[
		{"codec_type":"video","codec_name":"hevc","width":3840,"height":2160,"display_aspect_ratio":"16:9","avg_frame_rate":"24000/1001",
		 "color_transfer":"smpte2084","tags":{"BPS-eng":"15800000"},"side_data_list":[{"side_data_type":"DOVI configuration record"}]},
		{"codec_type":"audio","codec_name":"dts","profile":"DTS-HD MA","sample_rate":"48000","channels":8,"channel_layout":"7.1","bit_rate":"3000000","tags":{"language":"ger"}},
		{"codec_type":"audio","codec_name":"ac3","sample_rate":"48000","channels":6,"tags":{"language":"eng"}},
		{"codec_type":"audio","codec_name":"ac3","tags":{"language":"ger"}},
		{"codec_type":"subtitle","codec_name":"subrip","tags":{"language":"deu"}},
		{"codec_type":"subtitle","codec_name":"subrip"},
		{"codec_type":"video","codec_name":"mjpeg","width":600,"height":900,"disposition":{"attached_pic":1}}],
	 "format":{"duration":"7265.48","bit_rate":"20000000","tags":{"title":"Sintel","DATE":"2010-09-27"}}}`
	var p probe
	if err := json.Unmarshal([]byte(raw), &p); err != nil {
		t.Fatal(err)
	}
	rec := Record{}
	fillAV(&p, rec)
	expect(t, Values(rec), Values{
		"width": int64(3840), "height": int64(2160), "vcodec": "HEVC", "framerate": "23,976 fps", "vbitrate": int64(15800),
		"aspect": "16:9", "hdr": "Dolby Vision", "acodec": "DTS-HD MA", "abitrate": "3000 kBit/s", "samplerate": "48 kHz",
		"channels": "7.1", "duration": int64(7265), "audiotracks": "Deutsch, Englisch", "subtitles": "Deutsch, Unbekannt",
		"title": "Sintel", "year": int64(2010),
	})

	portrait := `{"streams":[{"codec_type":"video","codec_name":"h264","width":1920,"height":1080,"avg_frame_rate":"30/1",
		"color_transfer":"bt709","side_data_list":[{"side_data_type":"Display Matrix","rotation":-90}]}],"format":{"bit_rate":"8000000"}}`
	p = probe{}
	if err := json.Unmarshal([]byte(portrait), &p); err != nil {
		t.Fatal(err)
	}
	rec = Record{}
	fillAV(&p, rec)
	expect(t, Values(rec), Values{"width": int64(1080), "height": int64(1920), "aspect": "9:16", "hdr": "SDR", "vbitrate": int64(8000), "framerate": "30 fps"})
}

func TestSevenZipListingBecomesColumns(t *testing.T) {
	out := "7-Zip 24.08\n\nListing archive: a.7z\n\n--\nPath = a.7z\nType = 7z\nMethod = LZMA2:24 BCJ\n\n----------\n" +
		"Path = docs\nSize = 0\nFolder = +\nAttributes = D\n\n" +
		"Path = docs/a.txt\nSize = 1200\nFolder = -\nEncrypted = -\nMethod = LZMA2:24\n\n" +
		"Path = docs/b.txt\nSize = 800\nFolder = -\nEncrypted = +\nMethod = LZMA2:24 7zAES\n"
	rec := Record{}
	parseListing(out, rec)
	expect(t, Values(rec), Values{"entries": int64(2), "unpacked": int64(2000), "method": "LZMA2", "archcrypt": true})
	if listedMethod("v5:m3:128M") != "RAR5" || listedMethod("Copy") != "Ohne" {
		t.Error("rar or copy method misnamed")
	}
}

func haveTools(t *testing.T, names ...string) {
	t.Helper()
	for _, n := range names {
		if _, err := exec.LookPath(n); err != nil {
			t.Skipf("%s is not installed", n)
		}
	}
}

func TestMediaThroughFFprobe(t *testing.T) {
	haveTools(t, "ffmpeg", "ffprobe")
	s := newService(t)
	dir := t.TempDir()
	film := filepath.Join(dir, "Testfilm (2026).mkv")
	ffmpeg(t, "-f", "lavfi", "-i", "testsrc=size=320x240:rate=25", "-f", "lavfi", "-i", "sine=frequency=440:sample_rate=48000",
		"-t", "2", "-c:v", "mpeg4", "-c:a", "aac", "-ac", "2", "-metadata:s:a:0", "language=deu", film)
	expect(t, valuesOf(t, s, film, "duration", "dimensions", "vcodec", "framerate", "acodec", "audiotracks", "hdr", "aspect", "channels", "samplerate"), Values{
		"duration": int64(2), "dimensions": "320 × 240", "vcodec": "MPEG-4", "framerate": "25 fps", "acodec": "AAC",
		"audiotracks": "Deutsch", "hdr": "SDR", "aspect": "4:3", "channels": "Stereo", "samplerate": "48 kHz",
	})
	song := filepath.Join(dir, "01 Lied.flac")
	ffmpeg(t, "-f", "lavfi", "-i", "sine=frequency=440:sample_rate=44100", "-t", "3", "-ac", "1",
		"-metadata", "artist=Kevin MacLeod", "-metadata", "title=Calming", "-metadata", "album=Ruhe", "-metadata", "track=4", "-metadata", "date=2019", song)
	expect(t, valuesOf(t, s, song, "duration", "artist", "title", "album", "track", "year", "acodec", "samplerate", "channels"), Values{
		"duration": int64(3), "artist": "Kevin MacLeod", "title": "Calming", "album": "Ruhe", "track": int64(4), "year": int64(2019),
		"acodec": "FLAC", "samplerate": "44,1 kHz", "channels": "Mono",
	})
}

func ffmpeg(t *testing.T, args ...string) {
	t.Helper()
	out, err := exec.Command("ffmpeg", append([]string{"-v", "error", "-y"}, args...)...).CombinedOutput()
	if err != nil {
		t.Fatalf("ffmpeg: %v: %s", err, out)
	}
}

func TestDocumentsThroughMuPDF(t *testing.T) {
	haveTools(t, "mutool")
	s := newService(t)
	p := writeFile(t, filepath.Join(t.TempDir(), "steuer.pdf"), twoPagePDF())
	expect(t, valuesOf(t, s, p, "pages", "doctitle", "author", "subject", "app", "encrypted"), Values{
		"pages": int64(2), "doctitle": "Steuer 2025", "author": "jdp", "subject": "Belege", "app": "LibreOffice Writer", "encrypted": false,
	})
	png, err := s.Page(context.Background(), p, 2, 300)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.HasPrefix(png, []byte("\x89PNG")) {
		t.Errorf("page 2 is not a PNG: % x", png[:min(8, len(png))])
	}
	if _, err := s.Page(context.Background(), p, 3, 300); err == nil {
		t.Error("page 3 of 2 was drawn")
	}
}
