//go:build !linux && !windows

package watch

import (
	"errors"
	"io/fs"
)

// notifier stands in where the system tells of no changes, so every folder
// is polled.
type notifier struct{}

func newNotifier(*Watcher) (*notifier, error) { return nil, errors.ErrUnsupported }

func (*notifier) add(string, string) (int, error) { return -1, errors.ErrUnsupported }
func (*notifier) remove(int, string)              {}
func (*notifier) close()                          {}

func needsPoll(string) bool { return true }

func inoOf(fs.FileInfo) uint64 { return 0 }
