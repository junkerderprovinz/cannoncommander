package main

import (
	"errors"
	"log"
	"os"
	"path/filepath"
	"sync"
	"sync/atomic"

	"github.com/wailsapp/wails/v3/pkg/application"
	"github.com/wailsapp/wails/v3/pkg/events"

	"github.com/junkerderprovinz/cannoncommander/backend/pkg/desktop"
)

// folderHandler tells the computer which program opens its folders.
type folderHandler interface {
	set(exe string, w desktop.Words) error
	// unset gives the place back to the program that had it before.
	unset() error
	isSet(exe string) bool
}

// standby is the part of the shell that stands in for the computer's own file
// manager.
type standby struct {
	// folders is nil where the app cannot take that place.
	folders folderHandler
	trayMu  sync.Mutex
	tray    *application.SystemTray
	keep    atomic.Bool
	window  atomic.Pointer[application.WebviewWindow]
}

var _ desktop.FileManager = (*shell)(nil)

func executable() (string, error) {
	exe, err := os.Executable()
	if err != nil {
		return "", err
	}
	return filepath.EvalSymlinks(exe)
}

func (s *shell) IsFileManager() (bool, error) {
	if s.folders == nil {
		return false, errors.ErrUnsupported
	}
	exe, err := executable()
	if err != nil {
		return false, err
	}
	return s.folders.isSet(exe), nil
}

func (s *shell) SetFileManager(on bool, w desktop.Words) error {
	if s.folders == nil {
		return errors.ErrUnsupported
	}
	exe, err := executable()
	if err != nil {
		return err
	}
	if on {
		err = s.folders.set(exe, w)
	} else {
		err = s.folders.unset()
	}
	if err != nil {
		return err
	}
	if on {
		if err := s.state.update(func(st *State) { st.Words = w }); err != nil {
			log.Printf("could not keep the words of the tray: %v", err)
		}
	}
	s.standBy(on)
	return nil
}

// standBy keeps the app running in the tray after its window closes while it
// is the computer's file manager, so a folder another program opens shows
// without waiting for the server to start. A tray menu holds the way out.
func (s *shell) standBy(on bool) {
	s.trayMu.Lock()
	defer s.trayMu.Unlock()
	s.keep.Store(on)
	offerFolders(s, on)
	if s.tray != nil {
		s.tray.Destroy()
		s.tray = nil
	}
	if !on {
		return
	}
	w := s.state.get().Words
	if w.Show == "" {
		w.Show = "CannonCommander"
	}
	if w.Quit == "" {
		w.Quit = "Quit"
	}
	menu := application.NewMenu()
	menu.Add(w.Show).OnClick(func(*application.Context) { s.bringUp() })
	menu.AddSeparator()
	menu.Add(w.Quit).OnClick(func(*application.Context) { s.app.Quit() })
	s.tray = s.app.SystemTray.New().SetIcon(appIcon).SetMenu(menu).OnClick(s.bringUp)
	s.tray.SetTooltip("CannonCommander")
}

// followWindow hides the window instead of closing it while the app stands
// by.
func (s *shell) followWindow(w *application.WebviewWindow) {
	s.window.Store(w)
	w.RegisterHook(events.Common.WindowClosing, func(e *application.WindowEvent) {
		if s.keep.Load() {
			e.Cancel()
			w.Hide()
		}
	})
	if on, _ := s.IsFileManager(); on {
		s.standBy(true)
	} else if os.Getenv("DBUS_STARTER_BUS_TYPE") != "" {
		// The bus started the app for a request to show a folder and ends
		// it unless it takes the name, even where the switch reads off.
		offerFolders(s, true)
	}
}

// show opens what another program handed over and brings the window up.
func (s *shell) show(open []opening) {
	if len(open) > 0 {
		desktop.Emit("open", open)
	}
	s.bringUp()
}

func (s *shell) bringUp() {
	if w := s.window.Load(); w != nil {
		w.Restore()
		w.Show()
		w.Focus()
	}
}

// switchFileManager is the command line's way to the switch, for an
// installer that offers it and for its uninstaller, which gives the place
// back before the program is gone.
func switchFileManager(on bool) error {
	h := newFolderHandler()
	if h == nil {
		return errors.ErrUnsupported
	}
	if !on {
		return h.unset()
	}
	exe, err := executable()
	if err != nil {
		return err
	}
	return h.set(exe, desktop.Words{})
}
