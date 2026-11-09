package remotes

import (
	"slices"
	"strings"
	"testing"
)

func (f *fixture) test(in Input) TestResult {
	f.t.Helper()
	var res TestResult
	if code := f.call("jdp", "POST", "/remotes/test", in, &res); code != 200 {
		f.t.Fatalf("test %+v: %d", in, code)
	}
	return res
}

func (f *fixture) lastList() string {
	f.rc.mu.Lock()
	defer f.rc.mu.Unlock()
	return f.rc.lists[len(f.rc.lists)-1]
}

func TestTestGoesThroughAConnectionStringAndLeavesTheConfigAlone(t *testing.T) {
	f := newFixture(t)
	res := f.test(Input{Type: "webdav", Params: map[string]string{"url": "https://cloud.example.com/remote.php/webdav/", "vendor": "nextcloud", "user": "jdp", "pass": "geheim"}})
	if !res.OK || res.Entries != 2 {
		t.Fatalf("result %+v", res)
	}
	want := `:webdav,pass="obs(geheim)",url="https://cloud.example.com/remote.php/webdav/",user=jdp,vendor=nextcloud:`
	if got := f.lastList(); got != want {
		t.Fatalf("listed %s, want %s", got, want)
	}
	f.rc.mu.Lock()
	defer f.rc.mu.Unlock()
	if slices.Contains(f.rc.calls, "config/create") || len(f.rc.sections) != 0 {
		t.Fatalf("the test wrote to rclone.conf: %v", f.rc.calls)
	}
}

func TestTestShowsTheHostKeyOfAnSFTPServerAndSavingPinsIt(t *testing.T) {
	f := newFixture(t)
	res := f.test(Input{Type: "sftp", Params: map[string]string{"host": "nas.lan", "user": "jdp", "pass": "geheim"}})
	if !res.OK || res.HostKey == nil || res.HostKey.Fingerprint != fakeKey.Fingerprint || res.HostKey.Pin != fakeKey.Pin {
		t.Fatalf("result %+v", res)
	}
	if !strings.Contains(f.lastList(), "host_keys=") {
		t.Fatalf("the test did not hold the server to its key: %s", f.lastList())
	}
	f.call("jdp", "POST", "/remotes", Input{Name: "nas", Type: "sftp", Params: map[string]string{"host": "nas.lan", "pass": "geheim", hostKeysKey: res.HostKey.Pin}}, nil)
	if got := f.rc.section("nas")[hostKeysKey]; got != fakeKey.Pin {
		t.Fatalf("pinned %q", got)
	}
}

func TestAnSFTPRemoteSavedWithoutATestIsPinnedWhenItMounts(t *testing.T) {
	f := newFixture(t)
	f.call("jdp", "POST", "/remotes", Input{Name: "nas", Type: "sftp", Params: map[string]string{"host": "nas.lan", "pass": "geheim"}}, nil)
	if got := f.rc.section("nas")[hostKeysKey]; got != fakeKey.Pin {
		t.Fatalf("pinned %q", got)
	}
}

func TestAnotherHostKeyIsRefusedWithItsOwnReason(t *testing.T) {
	f := newFixture(t)
	res := f.test(Input{Type: "sftp", Params: map[string]string{"host": "moved", hostKeysKey: "ssh-ed25519 AAAAC3NzaC1lZDI1NTE5AAAAIOMqqnkVzrm0SdG6UOoqKLsabgH5C9okWi0dh2l9GKJl"}})
	if res.OK || res.Failure != failHostKey {
		t.Fatalf("result %+v", res)
	}
}

func TestASavedPasswordOnlyGoesToTheSavedServer(t *testing.T) {
	f := newFixture(t)
	f.call("jdp", "POST", "/remotes", Input{Name: "nas", Type: "sftp", Params: map[string]string{"host": "nas.lan", "user": "jdp", "pass": "geheim"}}, nil)
	f.test(Input{Name: "nas", Type: "sftp", Params: map[string]string{"host": "nas.lan", "user": "jdp", "pass": ""}})
	if !strings.Contains(f.lastList(), "obs(geheim)") {
		t.Fatalf("the saved password did not fill in: %s", f.lastList())
	}
	f.test(Input{Name: "nas", Type: "sftp", Params: map[string]string{"host": "elsewhere.example", "user": "jdp", "pass": ""}})
	if strings.Contains(f.lastList(), "geheim") {
		t.Fatalf("the saved password went to another server: %s", f.lastList())
	}
}

func TestAnSMBTestWithoutAShareOffersTheShares(t *testing.T) {
	f := newFixture(t)
	res := f.test(Input{Type: "smb", Params: map[string]string{"host": "keller", "user": "jdp", "pass": "geheim"}})
	if !res.OK || strings.Join(res.Shares, ",") != "archiv,fotos" {
		t.Fatalf("result %+v", res)
	}
	share := "archiv"
	if res := f.test(Input{Type: "smb", Path: &share, Params: map[string]string{"host": "keller"}}); !res.OK || res.Shares != nil {
		t.Fatalf("with a share %+v", res)
	}
}

func TestAFailedTestSaysWhy(t *testing.T) {
	f := newFixture(t)
	nope := "nope"
	if res := f.test(Input{Type: "sftp", Path: &nope, Params: map[string]string{"host": "nas.lan"}}); res.OK || res.Failure != failMissing {
		t.Fatalf("missing folder %+v", res)
	}
	if res := f.test(Input{Type: "webdav", Params: map[string]string{"url": "https://cloud.example.com/wrong"}}); res.OK || res.Failure != failAddress {
		t.Fatalf("wrong address %+v", res)
	}
	if res := f.test(Input{Type: "sftp", Params: map[string]string{"host": "down"}}); res.OK || res.Failure != "" || !strings.Contains(res.Error, "connection refused") {
		t.Fatalf("unreachable %+v", res)
	}
}

func TestAChoiceOfRclonesSetupComesBackAsAQuestion(t *testing.T) {
	f := newFixture(t)
	token := `{"access_token":"a","refresh_token":"two-drives"}`
	res := f.test(Input{Type: "drive", Params: map[string]string{"token": token}})
	if res.OK || res.Question == nil || res.Question.Name != "config_driveid" || len(res.Question.Choices) != 2 {
		t.Fatalf("result %+v", res)
	}
	if code := f.call("jdp", "POST", "/remotes", Input{Name: "gdrive", Type: "drive", Params: map[string]string{"token": token}}, nil); code != 409 {
		t.Fatalf("save without an answer: %d", code)
	}
	res = f.test(Input{Type: "drive", Params: map[string]string{"token": token, "config_driveid": "team"}})
	if !res.OK || res.Question != nil {
		t.Fatalf("answered %+v", res)
	}
	f.call("jdp", "POST", "/remotes", Input{Name: "gdrive", Type: "drive", Params: map[string]string{"token": token, "config_driveid": "team"}}, nil)
	if sec := f.rc.section("gdrive"); sec["drive_id"] != "team" || sec["config_driveid"] != "" {
		t.Fatalf("saved %v", sec)
	}
}

func TestQuoteValueLeavesSafeValuesBare(t *testing.T) {
	for in, want := range map[string]string{
		"nas":             "nas",
		"a.b-c_d~e":       "a.b-c_d~e",
		"https://x/y":     `"https://x/y"`,
		`say "hi", there`: `"say ""hi"", there"`,
		"":                `""`,
	} {
		if got := quoteValue(in); got != want {
			t.Errorf("quoteValue(%q) = %s, want %s", in, got, want)
		}
	}
}
