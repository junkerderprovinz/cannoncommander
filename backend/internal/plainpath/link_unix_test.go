//go:build unix

package plainpath

import (
	"os"
	"testing"
)

func linkDir(t *testing.T, target, at string) {
	t.Helper()
	if err := os.Symlink(target, at); err != nil {
		t.Fatal(err)
	}
}
