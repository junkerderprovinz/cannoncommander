package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"net"
	"os"
	"path/filepath"
	"strconv"
)

// source is one place the panels can open, as config.yaml and sources.json
// name it.
type source struct {
	Name string
	Path string
	// Indexed sources take part in the search. A whole drive is left out,
	// since reading all of it would keep the computer busy for a long time.
	Indexed bool
}

// serverConfig is the configuration the desktop app hands the server, written
// anew on every start. YAML reads JSON, which escapes a Windows path safely.
func serverConfig(dir string, port int, sources []source) ([]byte, error) {
	type rule struct {
		FolderPath   string `json:"folderPath"`
		Viewable     bool   `json:"viewable,omitempty"`
		IgnoreHidden bool   `json:"ignoreHidden,omitempty"`
	}
	type src struct {
		Path   string `json:"path"`
		Name   string `json:"name"`
		Config struct {
			Rules []rule `json:"rules"`
		} `json:"config"`
	}
	list := make([]src, 0, len(sources))
	for _, s := range sources {
		e := src{Path: s.Path, Name: s.Name}
		e.Config.Rules = []rule{{FolderPath: "/", Viewable: !s.Indexed, IgnoreHidden: s.Indexed}}
		list = append(list, e)
	}
	cfg := map[string]any{
		// Loopback only: the window is the one client, and desktop.Guard
		// turns away everybody else on this computer.
		"http": map[string]any{"port": port, "listen": "127.0.0.1", "baseURL": "/"},
		"server": map[string]any{
			"cacheDir": filepath.Join(dir, "cache"),
			"database": map[string]any{"path": filepath.Join(dir, "cannoncommander.sqlite")},
			"logging": []map[string]any{{
				"levels": "info|warning|error", "output": filepath.Join(dir, logFile), "noColors": true,
			}},
			"sources": list,
		},
		// The person at this computer is the only user, so there is no
		// sign in.
		"auth": map[string]any{"methods": map[string]any{"noauth": true}},
	}
	return json.MarshalIndent(cfg, "", "  ")
}

// logFile is the server's log in the data folder. A windowed program has no
// console to write to.
const logFile = "cannoncommander.log"

// writeServerConfig writes config.yaml, and on the first start sources.json
// with the default places, which the settings page edits from then on. The
// sources in config.yaml only satisfy the server, which wants at least one;
// sources.json replaces them.
func writeServerConfig(dir string, port int, sources []source) (string, error) {
	body, err := serverConfig(dir, port, sources)
	if err != nil {
		return "", err
	}
	path := filepath.Join(dir, "config.yaml")
	if err := os.WriteFile(path, append(body, '\n'), 0o600); err != nil {
		return "", err
	}
	stored := filepath.Join(dir, "sources.json")
	if _, err := os.Stat(stored); errors.Is(err, fs.ErrNotExist) {
		type entry struct {
			Name       string   `json:"name"`
			Path       string   `json:"path"`
			ReadOnly   bool     `json:"readOnly"`
			Indexed    bool     `json:"indexed"`
			SkipHidden bool     `json:"skipHidden"`
			Exclude    []string `json:"exclude"`
		}
		list := make([]entry, 0, len(sources))
		for _, s := range sources {
			list = append(list, entry{Name: s.Name, Path: s.Path, Indexed: s.Indexed, SkipHidden: s.Indexed, Exclude: []string{}})
		}
		raw, err := json.MarshalIndent(list, "", "  ")
		if err != nil {
			return "", err
		}
		if err := os.WriteFile(stored, append(raw, '\n'), 0o600); err != nil {
			return "", err
		}
	}
	return path, nil
}

// pickPort keeps the port of the last start while it is free, and otherwise
// takes one the system hands out.
func pickPort(last int) (int, error) {
	if last > 0 {
		if l, err := net.Listen("tcp", "127.0.0.1:"+strconv.Itoa(last)); err == nil {
			l.Close()
			return last, nil
		}
	}
	l, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		return 0, fmt.Errorf("find a free port: %w", err)
	}
	defer l.Close()
	return l.Addr().(*net.TCPAddr).Port, nil
}

// trimLog moves a log grown past 4 MiB aside, so it never fills the disk.
func trimLog(path string) {
	if info, err := os.Stat(path); err == nil && info.Size() > 4<<20 {
		_ = os.Rename(path, path+".old")
	}
}
