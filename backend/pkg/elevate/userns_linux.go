package elevate

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

// ProfileFile is the one AppArmor profile the helper writes. It lets the
// app's own program make user namespaces where Ubuntu restricts them since
// 24.04, so bwrap can shut the system's thumbnailers in, the way Ubuntu
// ships such profiles for Chrome and Steam.
const ProfileFile = "/etc/apparmor.d/cannoncommander-userns"

// ProfileName is the name the profile goes by in the kernel.
const ProfileName = "cannoncommander-userns"

var (
	profileFile = ProfileFile
	// parsers are the places apparmor_parser lives, looked at in turn
	// rather than along a PATH that root inherits.
	parsers = []string{"/usr/sbin/apparmor_parser", "/sbin/apparmor_parser"}
)

// usernsProfile is the profile for the program at exe.
func usernsProfile(exe string) (string, error) {
	if !filepath.IsAbs(exe) || strings.ContainsAny(exe, "\"\\{}\x00\n\r") {
		return "", fmt.Errorf("%w: %q cannot stand in a profile", ErrBadOp, exe)
	}
	return `# CannonCommander lets its own program make user namespaces here, which bwrap
# needs to shut the system's thumbnailers in. It writes and removes this file.
abi <abi/4.0>,
include <tunables/global>

profile ` + ProfileName + ` "` + exe + `" flags=(unconfined) {
  userns,

  include if exists <local/cannoncommander-userns>
}
`, nil
}

// userNamespaces puts the profile for this program in place and loads it,
// or unloads and removes it.
func userNamespaces(allow bool) error {
	parser, err := apparmorParser()
	if err != nil {
		return err
	}
	if !allow {
		if _, err := os.Stat(profileFile); errors.Is(err, fs.ErrNotExist) {
			return nil
		}
		// A profile that was never loaded cannot be unloaded, and the file
		// goes all the same.
		_ = exec.Command(parser, "-R", profileFile).Run()
		return os.Remove(profileFile)
	}
	exe, err := os.Executable()
	if err != nil {
		return err
	}
	if exe, err = filepath.EvalSymlinks(exe); err != nil {
		return err
	}
	text, err := usernsProfile(exe)
	if err != nil {
		return err
	}
	if err := writeProfile(text); err != nil {
		return err
	}
	if out, err := exec.Command(parser, "-r", profileFile).CombinedOutput(); err != nil {
		os.Remove(profileFile)
		return fmt.Errorf("apparmor_parser: %w: %s", err, strings.TrimSpace(string(out)))
	}
	return nil
}

func apparmorParser() (string, error) {
	for _, p := range parsers {
		if info, err := os.Stat(p); err == nil && info.Mode().IsRegular() {
			return p, nil
		}
	}
	return "", fmt.Errorf("%w: no apparmor_parser", ErrUnsupported)
}

// writeProfile puts text in place so the parser never reads half of it.
func writeProfile(text string) error {
	f, err := os.CreateTemp(filepath.Dir(profileFile), ".cannoncommander-userns-*")
	if err != nil {
		return err
	}
	_, err = f.WriteString(text)
	if err == nil {
		err = f.Chmod(0o644)
	}
	if cerr := f.Close(); err == nil {
		err = cerr
	}
	if err == nil {
		err = os.Rename(f.Name(), profileFile)
	}
	if err != nil {
		os.Remove(f.Name())
	}
	return err
}
