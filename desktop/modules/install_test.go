package modules

import (
	"archive/zip"
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"testing"

	"github.com/junkerderprovinz/cannoncommander/desktop/update"
)

var platform = runtime.GOOS + "/" + runtime.GOARCH

// modulesZip is a modules zip as assemble writes one, with a single module.
func modulesZip(t *testing.T, program string) []byte {
	t.Helper()
	rec, _ := json.Marshal(Record{Release: "v1.4.0-dirty", Platform: platform, Modules: map[string]Module{
		"tool": {Version: "2.0", Path: "bin", Env: map[string]string{"CC_TOOL": "bin/tool"}},
	}})
	var buf bytes.Buffer
	w := zip.NewWriter(&buf)
	for name, body := range map[string][]byte{RecordFile: rec, "tool/bin/tool": []byte(program), "tool/LICENSE": []byte("licence")} {
		fw, err := w.Create(name)
		if err != nil {
			t.Fatal(err)
		}
		fw.Write(body)
	}
	if err := w.Close(); err != nil {
		t.Fatal(err)
	}
	return buf.Bytes()
}

// release stands in for GitHub with one release, tagged tag, holding files
// and a checksums.txt that lists sums, or the real sums where sums is nil.
func release(t *testing.T, tag string, files map[string][]byte, sums map[string]string) *update.Updater {
	t.Helper()
	var list strings.Builder
	for name, data := range files {
		s, ok := sums[name]
		if !ok {
			h := sha256.Sum256(data)
			s = hex.EncodeToString(h[:])
		}
		fmt.Fprintf(&list, "%s  %s\n", s, name)
	}
	files[update.ChecksumsFile] = []byte(list.String())
	var srv *httptest.Server
	mux := http.NewServeMux()
	mux.HandleFunc("GET /repos/owner/prog/releases/tags/"+tag, func(w http.ResponseWriter, r *http.Request) {
		type asset struct {
			Name string `json:"name"`
			URL  string `json:"browser_download_url"`
		}
		var body struct {
			TagName string  `json:"tag_name"`
			Assets  []asset `json:"assets"`
		}
		body.TagName = tag
		for name := range files {
			body.Assets = append(body.Assets, asset{name, srv.URL + "/download/" + name})
		}
		json.NewEncoder(w).Encode(body)
	})
	mux.HandleFunc("GET /download/{name}", func(w http.ResponseWriter, r *http.Request) {
		data := files[r.PathValue("name")]
		w.Header().Set("Content-Length", strconv.Itoa(len(data)))
		w.Write(data)
	})
	srv = httptest.NewServer(mux)
	t.Cleanup(srv.Close)
	return &update.Updater{Repo: "owner/prog", Version: "v1.3.0", API: srv.URL, Client: srv.Client()}
}

// oldModules puts an earlier release's modules where the program keeps them.
func oldModules(t *testing.T) string {
	t.Helper()
	dir := filepath.Join(t.TempDir(), "modules")
	if err := os.MkdirAll(filepath.Join(dir, "tool", "bin"), 0o755); err != nil {
		t.Fatal(err)
	}
	rec, _ := json.Marshal(Record{Release: "v1.3.0", Platform: platform})
	os.WriteFile(filepath.Join(dir, RecordFile), rec, 0o644)
	os.WriteFile(filepath.Join(dir, "tool", "bin", "tool"), []byte("old tool"), 0o755)
	return dir
}

var asset = AssetName(runtime.GOOS, runtime.GOARCH)

func TestInstallSwapsInTheModulesOfTheReleaseAndRecordsItsTag(t *testing.T) {
	dir := oldModules(t)
	u := release(t, "v1.4.0", map[string][]byte{asset: modulesZip(t, "new tool")}, nil)
	in := &Installer{Updater: u, Dir: dir}
	if err := in.Install(context.Background(), "v1.4.0"); err != nil {
		t.Fatal(err)
	}
	if got, _ := os.ReadFile(filepath.Join(dir, "tool", "bin", "tool")); string(got) != "new tool" {
		t.Errorf("tool is %q", got)
	}
	rec, err := Read(dir)
	if err != nil {
		t.Fatal(err)
	}
	if rec.Release != "v1.4.0" || rec.Modules["tool"].Version != "2.0" {
		t.Errorf("record %+v", rec)
	}
	entries, _ := os.ReadDir(filepath.Dir(dir))
	if len(entries) != 1 {
		var names []string
		for _, e := range entries {
			names = append(names, e.Name())
		}
		t.Errorf("beside the modules: %v", names)
	}
}

func TestInstallLeavesTheModulesAloneWhenTheZipIsNotTheOneTheReleaseLists(t *testing.T) {
	dir := oldModules(t)
	u := release(t, "v1.4.0", map[string][]byte{asset: modulesZip(t, "tampered tool")}, map[string]string{asset: strings.Repeat("ab", 32)})
	in := &Installer{Updater: u, Dir: dir}
	if err := in.Install(context.Background(), "v1.4.0"); err == nil {
		t.Fatal("a zip with another checksum was installed")
	}
	if got, _ := os.ReadFile(filepath.Join(dir, "tool", "bin", "tool")); string(got) != "old tool" {
		t.Errorf("tool is %q", got)
	}
	if entries, _ := os.ReadDir(filepath.Dir(dir)); len(entries) != 1 {
		t.Errorf("%d entries beside the modules, want the modules alone", len(entries))
	}
}

func TestInstallRefusesModulesForAnotherPlatform(t *testing.T) {
	var buf bytes.Buffer
	w := zip.NewWriter(&buf)
	fw, _ := w.Create(RecordFile)
	fw.Write([]byte(`{"release": "v1.4.0", "platform": "plan9/mips", "modules": {}}`))
	w.Close()
	dir := oldModules(t)
	u := release(t, "v1.4.0", map[string][]byte{asset: buf.Bytes()}, nil)
	if err := (&Installer{Updater: u, Dir: dir}).Install(context.Background(), "v1.4.0"); err == nil {
		t.Fatal("modules for plan9 were installed")
	}
	if rec, err := Read(dir); err != nil || rec.Release != "v1.3.0" {
		t.Errorf("record %+v, %v", rec, err)
	}
}

func TestInstallReportsProgressFromNothingToDone(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "modules")
	u := release(t, "v1.4.0", map[string][]byte{asset: modulesZip(t, strings.Repeat("big tool ", 50000))}, nil)
	var fractions []float64
	var statuses []string
	in := &Installer{Updater: u, Dir: dir, Report: func(f float64, status string) {
		fractions = append(fractions, f)
		if status != "" {
			statuses = append(statuses, status)
		}
	}}
	if err := in.Install(context.Background(), "v1.4.0"); err != nil {
		t.Fatal(err)
	}
	if len(fractions) < 3 || fractions[0] != 0 || fractions[len(fractions)-1] != 1 {
		t.Fatalf("fractions %v", fractions)
	}
	for i := 1; i < len(fractions); i++ {
		if fractions[i] < fractions[i-1] {
			t.Fatalf("progress went back: %v", fractions)
		}
	}
	if statuses[0] != "Downloading the modules" || statuses[len(statuses)-1] != "Done" {
		t.Errorf("statuses %v", statuses)
	}
}

func TestPrepareSwapsInModulesThatWaitedForTheNextStart(t *testing.T) {
	dir := oldModules(t)
	next := dir + ".next"
	if err := os.MkdirAll(next, 0o755); err != nil {
		t.Fatal(err)
	}
	rec, _ := json.Marshal(Record{Release: "v1.4.0", Platform: platform})
	os.WriteFile(filepath.Join(next, RecordFile), rec, 0o644)
	os.MkdirAll(filepath.Join(filepath.Dir(dir), stagePrefix(dir)+"123"), 0o755)

	if err := Prepare(dir); err != nil {
		t.Fatal(err)
	}
	if got, err := Read(dir); err != nil || got.Release != "v1.4.0" {
		t.Errorf("record %+v, %v", got, err)
	}
	if entries, _ := os.ReadDir(filepath.Dir(dir)); len(entries) != 1 {
		t.Errorf("%d entries beside the modules after Prepare", len(entries))
	}
}

func TestUsePutsTheModulesFirstOnThePathAndKeepsVariablesTheUserSet(t *testing.T) {
	t.Setenv("PATH", "elsewhere")
	t.Setenv("CC_TOOL", "")
	t.Setenv("CC_OTHER", "mine")
	dir := filepath.Join("base", "modules")
	Use(dir, Record{Platform: platform, Modules: map[string]Module{
		"tool":  {Path: "bin", Env: map[string]string{"CC_TOOL": "bin/tool"}},
		"other": {Path: ".", Env: map[string]string{"CC_OTHER": "other"}},
	}})
	want := strings.Join([]string{filepath.Join(dir, "other"), filepath.Join(dir, "tool", "bin"), "elsewhere"}, string(os.PathListSeparator))
	if got := os.Getenv("PATH"); got != want {
		t.Errorf("PATH = %q, want %q", got, want)
	}
	if got := os.Getenv("CC_TOOL"); got != filepath.Join(dir, "tool", "bin", "tool") {
		t.Errorf("CC_TOOL = %q", got)
	}
	if got := os.Getenv("CC_OTHER"); got != "mine" {
		t.Errorf("CC_OTHER = %q, want the user's own", got)
	}
}

func TestReadRefusesModulesOfAnotherPlatform(t *testing.T) {
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, RecordFile), []byte(`{"release": "v1.4.0", "platform": "plan9/mips"}`), 0o644)
	if _, err := Read(dir); err == nil {
		t.Fatal("read modules for plan9")
	}
	if _, err := Read(t.TempDir()); !errors.Is(err, os.ErrNotExist) {
		t.Errorf("a folder without modules: %v", err)
	}
}
