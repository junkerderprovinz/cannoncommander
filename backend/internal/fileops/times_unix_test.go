//go:build !windows

package fileops_test

import (
	"errors"
	"fmt"
	"io/fs"
	"syscall"
	"testing"

	"github.com/junkerderprovinz/cannoncommander/backend/internal/fileops"
)

func TestACopyGoesOnWhereTheTargetCannotTakeTimes(t *testing.T) {
	for _, err := range []error{syscall.ENOTSUP, syscall.EPERM, &fs.PathError{Op: "utimes", Path: "x", Err: syscall.EOPNOTSUPP}} {
		if !fileops.TimesRefused(err) {
			t.Errorf("%v stops the copy", err)
		}
	}
	for _, err := range []error{syscall.EIO, syscall.ENOSPC, fmt.Errorf("wrapped: %w", errors.New("broken"))} {
		if fileops.TimesRefused(err) {
			t.Errorf("%v is passed over", err)
		}
	}
}
