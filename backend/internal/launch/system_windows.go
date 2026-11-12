//go:build desktop

package launch

import (
	"path/filepath"

	"github.com/junkerderprovinz/cannoncommander/backend/pkg/winshell"
	"golang.org/x/sys/windows"
)

type shellSystem struct{}

func newSystem() system { return shellSystem{} }

// asks is false: Explorer starts a program on a double click, and the shell
// warns on its own about one that came from the internet.
func (shellSystem) asks(string) bool { return false }

func (shellSystem) open(file string) error {
	if to := toOpen(file); to != file {
		return shellExecute(to, "")
	}
	return shellExecute(file, filepath.Dir(file))
}

func (shellSystem) run(file string) error { return shellExecute(file, filepath.Dir(file)) }

// shellExecute opens file with its default verb, as Explorer does.
func shellExecute(file, dir string) error {
	f, err := windows.UTF16PtrFromString(file)
	if err != nil {
		return err
	}
	var in *uint16
	if dir != "" {
		if in, err = windows.UTF16PtrFromString(dir); err != nil {
			return err
		}
	}
	return winshell.Execute(&winshell.ExecuteInfo{Mask: winshell.SeeMaskNoAsync, File: f, Dir: in, Show: windows.SW_SHOWNORMAL})
}
