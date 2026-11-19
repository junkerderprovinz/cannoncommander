package elevate

import (
	"context"
	"encoding/hex"
	"errors"
	"io"
	"os/exec"
	"strconv"
	"strings"

	"github.com/junkerderprovinz/cannoncommander/backend/pkg/appimage"
)

// launchHelper starts this program again through pkexec, which asks for the
// password with the desktop's own dialog. The secret goes through stdin,
// since every user can read the command line of a process.
func launchHelper(ctx context.Context) (io.ReadWriteCloser, []byte, error) {
	pkexec, err := exec.LookPath("pkexec")
	if err != nil {
		return nil, nil, ErrUnsupported
	}
	exe, err := appimage.Executable()
	if err != nil {
		return nil, nil, err
	}
	return overTCP(ctx, func(port int, secret []byte) (func() error, error) {
		cmd := exec.Command(pkexec, exe, "--elevated", strconv.Itoa(port))
		cmd.Stdin = strings.NewReader(hex.EncodeToString(secret) + "\n")
		if err := cmd.Start(); err != nil {
			return nil, err
		}
		return func() error {
			err := cmd.Wait()
			var exit *exec.ExitError
			// pkexec ends with 126 when the dialog was dismissed and with 127
			// when the password was wrong or not allowed.
			if errors.As(err, &exit) && (exit.ExitCode() == 126 || exit.ExitCode() == 127) {
				return ErrDeclined
			}
			return err
		}, nil
	})
}
