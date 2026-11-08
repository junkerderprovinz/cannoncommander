package version

import "testing"

func TestBuildWithoutStampStillHasAVersion(t *testing.T) {
	tests := []struct {
		version, commit, vcs string
		wantVersion          string
		wantCommit           string
	}{
		{"", "", "", "dev", "unknown"},
		{"", "", "0123456789abcdef0123456789abcdef01234567", "dev-0123456", "0123456789abcdef0123456789abcdef01234567"},
		{"", "abc1234", "", "dev-abc1234", "abc1234"},
		{"v1.4.0", "", "", "v1.4.0", "unknown"},
		{"v1.4.0", "abc1234", "fedcba9", "v1.4.0", "abc1234"},
	}
	for _, tt := range tests {
		v, c := settle(tt.version, tt.commit, tt.vcs)
		if v != tt.wantVersion || c != tt.wantCommit {
			t.Errorf("settle(%q, %q, %q) = %q, %q, want %q, %q", tt.version, tt.commit, tt.vcs, v, c, tt.wantVersion, tt.wantCommit)
		}
	}
}

func TestVersionAndCommitAreNeverEmpty(t *testing.T) {
	if Version == "" || CommitSHA == "" {
		t.Fatalf("version %q, commit %q", Version, CommitSHA)
	}
}

func TestOnlyATaggedBuildIsARelease(t *testing.T) {
	tests := []struct {
		version string
		want    string
		ok      bool
	}{
		{"v1.4.0", "v1.4.0", true},
		{"1.4.0", "v1.4.0", true},
		{"v2.1.4-beta", "v2.1.4-beta", true},
		{"v1.4.0-3-gabc1234", "", false},
		{"v1.4.0-3-gabc1234-dirty", "", false},
		{"v1.4.0-dirty", "", false},
		{"abc1234", "", false},
		{"dev", "", false},
		{"dev-abc1234", "", false},
		{"testing", "", false},
	}
	saved := Version
	t.Cleanup(func() { Version = saved })
	for _, tt := range tests {
		Version = tt.version
		got, ok := Release()
		if got != tt.want || ok != tt.ok {
			t.Errorf("Release() with %q = %q, %v, want %q, %v", tt.version, got, ok, tt.want, tt.ok)
		}
	}
}
