package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strconv"
	"sync"
)

// State is what the desktop app keeps between two starts, beside the server's
// own settings in its database.
type State struct {
	// Port is the loopback port of the last start. The window's storage
	// belongs to its origin, port included, so a new one would lose it.
	Port int `json:"port,omitempty"`
	// AutoUpdate is nil until somebody switched it, which reads as on.
	AutoUpdate *bool `json:"autoUpdate,omitempty"`
	// Window is where the window was when it closed, and whether it filled
	// the screen. Zero means nobody moved it yet.
	Window Bounds `json:"window"`
}

// Bounds is a window's place in device independent pixels.
type Bounds struct {
	X         int  `json:"x"`
	Y         int  `json:"y"`
	Width     int  `json:"width"`
	Height    int  `json:"height"`
	Maximised bool `json:"maximised,omitempty"`
}

// autoUpdate reads the switch, which is on until somebody turns it off, since
// an app nobody updates keeps its bugs.
func (s State) autoUpdate() bool { return s.AutoUpdate == nil || *s.AutoUpdate }

// stateStore keeps State in desktop.json in the data folder.
type stateStore struct {
	path string
	mu   sync.Mutex
	cur  State
}

func openState(dir string) *stateStore {
	s := &stateStore{path: filepath.Join(dir, "desktop.json")}
	// An unreadable file costs the window's place and nothing else.
	if body, err := os.ReadFile(s.path); err == nil {
		_ = json.Unmarshal(body, &s.cur)
	}
	return s
}

func (s *stateStore) get() State {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.cur
}

// update changes the state and writes it at once.
func (s *stateStore) update(change func(*State)) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	next := s.cur
	change(&next)
	body, err := json.MarshalIndent(next, "", "  ")
	if err != nil {
		return err
	}
	tmp := s.path + ".tmp"
	if err := os.WriteFile(tmp, append(body, '\n'), 0o600); err != nil {
		return err
	}
	if err := os.Rename(tmp, s.path); err != nil {
		return err
	}
	s.cur = next
	return nil
}

// readAutoUpdate reads the switch from the file the installer keeps for all
// users. A missing or broken file reads as on, like a fresh install.
func readAutoUpdate(path string) bool {
	body, err := os.ReadFile(path)
	if err != nil {
		return true
	}
	f := struct {
		AutoUpdate bool `json:"autoUpdate"`
	}{AutoUpdate: true}
	if json.Unmarshal(body, &f) != nil {
		return true
	}
	return f.AutoUpdate
}

// writeAutoUpdate rewrites that file in place. Its folder belongs to the
// administrators and only the file is open to every user, so there is no
// folder for a temporary file. The installer's other keys stay as they were.
func writeAutoUpdate(path string, on bool) error {
	var fields map[string]json.RawMessage
	old, err := os.ReadFile(path)
	if err != nil && !errors.Is(err, fs.ErrNotExist) {
		return err
	}
	if json.Unmarshal(old, &fields) != nil || fields == nil {
		fields = map[string]json.RawMessage{}
	}
	fields["autoUpdate"] = json.RawMessage(strconv.FormatBool(on))
	body, err := json.Marshal(fields)
	if err != nil {
		return err
	}
	if err := os.WriteFile(path, append(body, '\n'), 0o644); err != nil {
		return fmt.Errorf("write %s: %w", path, err)
	}
	return nil
}
