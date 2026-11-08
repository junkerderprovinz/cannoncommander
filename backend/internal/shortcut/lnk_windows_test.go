package shortcut

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func TestWindowsFindsTheTargetOfAWrittenShortcut(t *testing.T) {
	dir := t.TempDir()
	target := filepath.Join(dir, "Grüße")
	if err := os.Mkdir(target, 0o755); err != nil {
		t.Fatal(err)
	}
	p := filepath.Join(dir, "Grüße - Verknüpfung.lnk")
	if err := Write(p, target, true); err != nil {
		t.Fatal(err)
	}
	script := "[Console]::OutputEncoding = [Text.Encoding]::UTF8; (New-Object -ComObject WScript.Shell).CreateShortcut($env:CC_LNK).TargetPath"
	cmd := exec.Command("powershell", "-NoProfile", "-NonInteractive", "-Command", script)
	cmd.Env = append(os.Environ(), "CC_LNK="+p)
	out, err := cmd.Output()
	if err != nil {
		t.Skipf("PowerShell cannot read shortcuts here: %v", err)
	}
	if got := strings.TrimSpace(string(out)); !strings.EqualFold(got, target) {
		t.Errorf("Windows reads %q, want %q", got, target)
	}
}
