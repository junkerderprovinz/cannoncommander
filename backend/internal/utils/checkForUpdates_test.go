package utils

import (
	"testing"

	"github.com/junkerderprovinz/cannoncommander/backend/internal/version"
)

func TestStartupUpdateCheckSkipsABuildThatIsNoRelease(t *testing.T) {
	saved := version.Version
	t.Cleanup(func() { version.Version = saved })
	for _, v := range []string{"dev", "dev-abc1234", "v1.4.0-3-gabc1234", "abc1234"} {
		version.Version = v
		if info, err := CheckForUpdates(); err != nil || info.LatestVersion != "" {
			t.Errorf("%s: %+v, %v", v, info, err)
		}
	}
}
