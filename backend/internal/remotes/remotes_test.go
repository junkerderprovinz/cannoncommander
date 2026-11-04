package remotes

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	v1 "github.com/junkerderprovinz/cannoncommander/backend/internal/api/v1"
)

func ptr(s string) *string { return &s }

func TestCreateObscuresPasswordsAndMounts(t *testing.T) {
	f := newFixture(t)
	var r Remote
	code := f.call("jdp", "POST", "/remotes", Input{Name: "nas", Type: "sftp", Params: map[string]string{"host": "nas.lan", "user": "jdp", "pass": "geheim"}}, &r)
	if code != 201 {
		t.Fatalf("create: %d", code)
	}
	want := filepath.Join(f.root, "nas")
	if r.Mount != want || !r.Mounted || !r.MountOnStart || r.Label != "rclone: nas" || r.Error != "" {
		t.Fatalf("remote %+v", r)
	}
	if sec := f.rc.section("nas"); sec["pass"] != "obs(geheim)" || sec["host"] != "nas.lan" || sec["type"] != "sftp" {
		t.Fatalf("section %v", sec)
	}
	if r.Params["pass"] != "" || !slices.Equal(r.Secrets, []string{"pass"}) || r.Params["host"] != "nas.lan" {
		t.Fatalf("params %v secrets %v", r.Params, r.Secrets)
	}
	if v := f.rc.vfs[want]; v["CacheMode"] != "writes" || v["CacheMaxSize"] != "10240M" {
		t.Fatalf("vfs %v", v)
	}
	if got := f.s.labelAt(want); got != "rclone: nas" {
		t.Fatalf("label %q", got)
	}

	var list List
	if code := f.call("anna", "GET", "/remotes", nil, &list); code != 200 {
		t.Fatalf("list: %d", code)
	}
	if len(list.Remotes) != 1 || list.Remotes[0].Params != nil || list.Remotes[0].Secrets != nil || !list.Rclone.Running {
		t.Fatalf("a user sees %+v", list)
	}
	if code := f.call("anna", "POST", "/remotes", Input{Name: "x", Type: "sftp"}, nil); code != 403 {
		t.Fatalf("user creates: %d", code)
	}
	if code := f.call("jdp", "POST", "/remotes", Input{Name: "nas", Type: "sftp", Params: map[string]string{"host": "h"}}, nil); code != 409 {
		t.Fatalf("same name: %d", code)
	}
}

func TestCreateRefusesBadNamesTypesAndFolders(t *testing.T) {
	f := newFixture(t)
	if code := f.call("jdp", "POST", "/remotes", Input{Name: "a", Type: "sftp", Mount: filepath.Join(f.root, "a")}, nil); code != 201 {
		t.Fatalf("first: %d", code)
	}
	for _, in := range []Input{
		{Name: "", Type: "sftp"},
		{Name: "Gross", Type: "sftp"},
		{Name: "mit leer", Type: "sftp"},
		{Name: "cc-try-1234", Type: "sftp"},
		{Name: "b", Type: "nosuchtype"},
		{Name: "b", Type: ""},
		{Name: "b", Type: "sftp", Mount: filepath.Join(f.mnt, "b")},
		{Name: "b", Type: "sftp", Mount: filepath.Join(f.mnt, "user", "b")},
		{Name: "b", Type: "sftp", Mount: filepath.Join(f.mnt, "disk3", "b")},
		{Name: "b", Type: "sftp", Mount: filepath.Join(f.root, "a", "inner")},
		{Name: "b", Type: "sftp", Mount: f.root},
		{Name: "b", Type: "sftp", Mount: "relative/b"},
		{Name: "b", Type: "sftp", Mount: filepath.Join(filepath.Dir(f.mnt), "elsewhere", "b")},
		{Name: "b", Type: "drive", Params: map[string]string{"token": "not a token"}},
	} {
		if code := f.call("jdp", "POST", "/remotes", in, nil); code != 400 {
			t.Errorf("%+v: %d, want 400", in, code)
		}
	}
	if sec := f.rc.section("b"); sec != nil {
		t.Fatalf("a refused remote left a section: %v", sec)
	}
}

func TestUpdateKeepsSecretsLeftBlankAndRenames(t *testing.T) {
	f := newFixture(t)
	f.call("jdp", "POST", "/remotes", Input{Name: "nas", Type: "smb", Path: ptr("archiv"), Params: map[string]string{"host": "keller", "user": "jdp", "pass": "alt", "domain": "HOME"}}, nil)
	var r Remote
	code := f.call("jdp", "PUT", "/remotes/nas", Input{Params: map[string]string{"host": "keller2", "pass": "", "domain": ""}}, &r)
	if code != 200 {
		t.Fatalf("update: %d", code)
	}
	sec := f.rc.section("nas")
	if sec["pass"] != "obs(alt)" || sec["host"] != "keller2" || sec["user"] != "jdp" {
		t.Fatalf("section %v", sec)
	}
	if _, ok := sec["domain"]; ok {
		t.Fatal("an emptied parameter stayed")
	}
	if r.Label != "SMB: //keller2/archiv" || !r.Mounted {
		t.Fatalf("remote %+v", r)
	}

	newMount := filepath.Join(f.root, "keller")
	if code := f.call("jdp", "PUT", "/remotes/nas", Input{Name: "keller", Mount: newMount, Params: map[string]string{"pass": "neu"}}, &r); code != 200 {
		t.Fatalf("rename: %d", code)
	}
	if f.rc.section("nas") != nil {
		t.Fatal("the old section is still there")
	}
	sec = f.rc.section("keller")
	if sec["pass"] != "obs(neu)" || sec["host"] != "keller2" || sec["type"] != "smb" {
		t.Fatalf("moved section %v", sec)
	}
	if r.Name != "keller" || r.Mount != newMount || !r.Mounted || f.rc.mounted(filepath.Join(f.root, "nas")) {
		t.Fatalf("renamed %+v", r)
	}
	if _, err := os.Stat(filepath.Join(f.root, "nas")); !os.IsNotExist(err) {
		t.Fatal("the old empty mount folder stayed")
	}
	if code := f.call("jdp", "PUT", "/remotes/keller", Input{Type: "sftp"}, nil); code != 400 {
		t.Fatalf("type change: %d", code)
	}
	if code := f.call("jdp", "PUT", "/remotes/nope", Input{}, nil); code != 404 {
		t.Fatalf("unknown: %d", code)
	}
}

func TestDriveTokenAnswersRclonesQuestions(t *testing.T) {
	f := newFixture(t)
	pasted := "Paste the following into your remote machine --->\n{\"access_token\":\"ya29\",\"token_type\":\"Bearer\",\"refresh_token\":\"1//x\",\"expiry\":\"2030-01-01T00:00:00Z\"}\n<---End paste\n"
	var tok OAuthResult
	if code := f.call("jdp", "POST", "/remotes/oauth", OAuthInput{Type: "drive", Token: pasted}, &tok); code != 200 {
		t.Fatalf("oauth: %d", code)
	}
	if !strings.HasPrefix(tok.Token, `{"access_token":"ya29"`) || tok.Expiry != "2030-01-01T00:00:00Z" || tok.Remote != nil {
		t.Fatalf("token %+v", tok)
	}
	if code := f.call("jdp", "POST", "/remotes", Input{Name: "gdrive", Type: "drive", Params: map[string]string{"token": tok.Token}}, nil); code != 201 {
		t.Fatalf("create: %d", code)
	}
	if sec := f.rc.section("gdrive"); sec["token"] != tok.Token {
		t.Fatalf("section %v", sec)
	}

	fresh := `{"access_token":"ya30","refresh_token":"1//y"}`
	if code := f.call("jdp", "POST", "/remotes/oauth", OAuthInput{Name: "gdrive", Token: fresh}, &tok); code != 200 || tok.Remote == nil || !tok.Remote.Mounted {
		t.Fatalf("new token: %+v", tok)
	}
	if sec := f.rc.section("gdrive"); sec["token"] != fresh {
		t.Fatalf("token not replaced: %v", sec)
	}
	for _, bad := range []OAuthInput{{Type: "drive", Token: "nothing here"}, {Type: "drive", Token: `{"foo":1}`}, {Type: "sftp", Token: fresh}} {
		if code := f.call("jdp", "POST", "/remotes/oauth", bad, nil); code != 400 {
			t.Errorf("%+v: %d, want 400", bad, code)
		}
	}
}

func TestDriveWithoutTokenSaysWhatRcloneNeeds(t *testing.T) {
	f := newFixture(t)
	if code := f.call("jdp", "POST", "/remotes", Input{Name: "gdrive", Type: "drive"}, nil); code != 400 {
		t.Fatalf("create without token: %d", code)
	}
	if f.rc.section("gdrive") != nil {
		t.Fatal("the half made section stayed")
	}
}

func TestTestUsesSavedSecretsAndCleansUp(t *testing.T) {
	f := newFixture(t)
	f.call("jdp", "POST", "/remotes", Input{Name: "nas", Type: "sftp", Params: map[string]string{"host": "nas.lan", "pass": "geheim"}}, nil)
	var res TestResult
	if code := f.call("jdp", "POST", "/remotes/test", Input{Name: "nas", Type: "sftp", Params: map[string]string{"host": "nas.lan", "pass": ""}}, &res); code != 200 {
		t.Fatalf("test: %d", code)
	}
	if !res.OK || res.Entries != 2 || res.Free == nil || *res.Free != 60 {
		t.Fatalf("result %+v", res)
	}
	if code := f.call("jdp", "POST", "/remotes/test", Input{Type: "sftp", Params: map[string]string{"host": "down"}}, &res); code != 200 {
		t.Fatalf("test down: %d", code)
	}
	if res.OK || !strings.Contains(res.Error, "connection refused") {
		t.Fatalf("result %+v", res)
	}
	f.rc.mu.Lock()
	defer f.rc.mu.Unlock()
	for name := range f.rc.sections {
		if strings.HasPrefix(name, tryPrefix) {
			t.Fatalf("test section %s stayed", name)
		}
	}
}

func TestMountUnmountAndDelete(t *testing.T) {
	f := newFixture(t)
	f.call("jdp", "POST", "/remotes", Input{Name: "mem", Type: "memory"}, nil)
	dir := filepath.Join(f.root, "mem")
	var r Remote
	if code := f.call("jdp", "POST", "/remotes/mem/unmount", nil, &r); code != 200 || r.Mounted || r.MountOnStart {
		t.Fatalf("unmount: %d %+v", code, r)
	}
	if err := os.WriteFile(filepath.Join(dir, "stray.txt"), []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	if code := f.call("jdp", "POST", "/remotes/mem/mount", nil, nil); code != 409 {
		t.Fatalf("mount on a folder with files: %d", code)
	}
	if got := f.s.lastError("mem"); !strings.Contains(got, "not empty") {
		t.Fatalf("error %q", got)
	}
	if err := os.Remove(filepath.Join(dir, "stray.txt")); err != nil {
		t.Fatal(err)
	}
	if code := f.call("jdp", "POST", "/remotes/mem/mount", nil, &r); code != 200 || !r.Mounted || !r.MountOnStart || r.Error != "" {
		t.Fatalf("mount: %+v", r)
	}

	f.rc.unmountRefuse = true
	if code := f.call("jdp", "DELETE", "/remotes/mem", nil, nil); code != 409 {
		t.Fatalf("delete while busy: %d", code)
	}
	f.rc.unmountRefuse = false
	if code := f.call("jdp", "DELETE", "/remotes/mem", nil, nil); code != 204 {
		t.Fatalf("delete: %d", code)
	}
	if f.rc.section("mem") != nil || f.rc.mounted(dir) || f.s.labelAt(dir) != "" {
		t.Fatal("the remote is not gone")
	}
	if _, err := os.Stat(dir); !os.IsNotExist(err) {
		t.Fatal("the empty mount folder stayed")
	}
	if code := f.call("jdp", "DELETE", "/remotes/mem", nil, nil); code != 404 {
		t.Fatalf("second delete: %d", code)
	}
}

func TestMountWithoutAllowOtherWhenFuseRefusesIt(t *testing.T) {
	f := newFixture(t)
	f.rc.noAllowOther = true
	var r Remote
	if code := f.call("jdp", "POST", "/remotes", Input{Name: "mem", Type: "memory"}, &r); code != 201 || !r.Mounted {
		t.Fatalf("create: %d %+v", code, r)
	}
	if len(f.rc.refused) != 1 {
		t.Fatalf("refused %v", f.rc.refused)
	}
}

func TestStartMountsWhatShouldBeMountedAndTakesInHandMadeRemotes(t *testing.T) {
	f := newFixture(t)
	f.call("jdp", "POST", "/remotes", Input{Name: "keep", Type: "memory"}, nil)
	f.call("jdp", "POST", "/remotes", Input{Name: "off", Type: "memory"}, nil)
	f.call("jdp", "POST", "/remotes/off/unmount", nil, nil)
	f.rc.mu.Lock()
	f.rc.mounts = map[string]string{}
	f.rc.sections["byhand"] = map[string]string{"type": "memory"}
	f.rc.sections[tryPrefix+"dead"] = map[string]string{"type": "memory"}
	f.rc.mu.Unlock()

	f.s.up(context.Background(), true)
	if !f.rc.mounted(filepath.Join(f.root, "keep")) || f.rc.mounted(filepath.Join(f.root, "off")) {
		t.Fatalf("mounts %v", f.rc.mounts)
	}
	if f.rc.section(tryPrefix+"dead") != nil {
		t.Fatal("a stale test section stayed")
	}
	r, err := f.s.Get(context.Background(), "byhand", true)
	if err != nil || r.Mount != filepath.Join(f.root, "byhand") || r.MountOnStart {
		t.Fatalf("taken in %+v %v", r, err)
	}
}

func TestTypesPutTheCommonOnesFirst(t *testing.T) {
	f := newFixture(t)
	var out struct {
		Types []Type `json:"types"`
	}
	if code := f.call("anna", "GET", "/remotes/types", nil, &out); code != 200 {
		t.Fatalf("types: %d", code)
	}
	var ids []string
	for _, ty := range out.Types {
		ids = append(ids, ty.ID)
	}
	if strings.Join(ids, ",") != "sftp,smb,drive,memory" {
		t.Fatalf("order %v", ids)
	}
	drive, sftp := out.Types[2], out.Types[0]
	if !drive.OAuth || drive.Authorize != `rclone authorize "drive"` || sftp.OAuth || out.Types[1].PathHint != "share" {
		t.Fatalf("types %+v", out.Types)
	}
	for _, o := range sftp.Options {
		if o.Name == "key_file" {
			t.Fatal("an advanced option came without asking")
		}
		if o.Name == "pass" && !o.Secret {
			t.Fatal("the password is not marked secret")
		}
	}
	f.call("anna", "GET", "/remotes/types?advanced=true", nil, &out)
	if !slices.ContainsFunc(out.Types[0].Options, func(o Option) bool { return o.Name == "key_file" }) {
		t.Fatal("advanced options missing")
	}
}

func TestWithoutRcloneTheListStillAnswers(t *testing.T) {
	f := newFixture(t)
	f.s.rc = nil
	var list List
	if code := f.call("jdp", "GET", "/remotes", nil, &list); code != 200 || list.Rclone.Installed || list.Remotes == nil {
		t.Fatalf("list: %+v", list)
	}
	if code := f.call("jdp", "GET", "/remotes/types", nil, nil); code != 503 {
		t.Fatalf("types: %d", code)
	}
}

func TestCacheIsSharedByTheMountedRemotes(t *testing.T) {
	rows := []row{{MountOnStart: true}, {MountOnStart: true}, {MountOnStart: false}}
	if got := cacheShare(50, rows); got != 25600 {
		t.Fatalf("share %d", got)
	}
	if got := cacheShare(10, nil); got != 10240 {
		t.Fatalf("share alone %d", got)
	}
}

func TestImportAddsMissingRemotesUnmountedAndWithoutSecrets(t *testing.T) {
	f := newFixture(t)
	f.call("jdp", "POST", "/remotes", Input{Name: "nas", Type: "sftp", Params: map[string]string{"host": "nas.lan", "pass": "geheim"}}, nil)
	sec := f.s.exportSection()
	raw, err := sec.Export(context.Background(), v1.Deps{})
	if err != nil {
		t.Fatal(err)
	}
	b, _ := json.Marshal(raw)
	if strings.Contains(string(b), "geheim") || strings.Contains(string(b), "obs(") {
		t.Fatalf("a secret went into the export: %s", b)
	}
	var list []exported
	if err := json.Unmarshal(b, &list); err != nil || len(list) != 1 {
		t.Fatalf("export %s", b)
	}
	list = append(list, exported{Name: "keller", Type: "sftp", Params: map[string]string{"host": "keller.lan"}})
	in, _ := json.Marshal(list)
	notes, err := sec.Import(context.Background(), v1.Deps{}, in)
	if err != nil || len(notes) != 2 {
		t.Fatalf("notes %v %v", notes, err)
	}
	r, err := f.s.Get(context.Background(), "keller", true)
	if err != nil || r.Mounted || r.MountOnStart || r.Params["host"] != "keller.lan" {
		t.Fatalf("imported %+v %v", r, err)
	}
}
