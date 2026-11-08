package plainpath

import (
	"os/exec"
	"testing"
)

// linkDir makes a junction, which any account may make without a privilege.
func linkDir(t *testing.T, target, at string) {
	t.Helper()
	if out, err := exec.Command("cmd", "/c", "mklink", "/J", at, target).CombinedOutput(); err != nil {
		t.Fatalf("%v: %s", err, out)
	}
}
