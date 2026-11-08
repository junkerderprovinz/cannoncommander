// Package templates finds what the desktop offers under New: the ShellNew
// entries Windows registers for a file type and the files in the templates
// folder of a Linux desktop.
package templates

import (
	"errors"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"
)

// Template is one kind of file New can make.
type Template struct {
	// ID names the template for Make.
	ID string `json:"id"`
	// Name is what the menu shows, such as the type name of Windows.
	Name string `json:"name"`
	// Ext is the ending of the new file, with its dot.
	Ext string `json:"ext"`
	// File is the name a new file starts out with, without its ending.
	File string `json:"file"`

	data []byte
	from string
}

// ErrUnknown answers an id that names no template.
var ErrUnknown = errors.New("there is no such template")

var (
	mu    sync.Mutex
	known []Template
	at    time.Time
)

// List returns the templates, sorted by name. Programs register theirs when
// they are installed, so the list is read again after a minute.
func List() []Template {
	mu.Lock()
	defer mu.Unlock()
	if known == nil || time.Since(at) > time.Minute {
		known = find()
		sort.Slice(known, func(i, j int) bool { return strings.ToLower(known[i].Name) < strings.ToLower(known[j].Name) })
		at = time.Now()
	}
	return known
}

// Make writes a new file at p from the template id. It does not replace a
// file that is there.
func Make(id, p string) error {
	var t *Template
	for _, x := range List() {
		if x.ID == id {
			t = &x
			break
		}
	}
	if t == nil {
		return ErrUnknown
	}
	data := t.data
	if t.from != "" {
		var err error
		if data, err = os.ReadFile(t.from); err != nil {
			return err
		}
	}
	f, err := os.OpenFile(p, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o666)
	if err != nil {
		return err
	}
	if _, err := f.Write(data); err != nil {
		f.Close()
		os.Remove(p)
		return err
	}
	return f.Close()
}

// fromFolder offers every file in dir, as Nautilus and Dolphin do with the
// templates folder.
func fromFolder(dir, prefix string) []Template {
	list, err := os.ReadDir(dir)
	if err != nil {
		return nil
	}
	var out []Template
	for _, d := range list {
		if strings.HasPrefix(d.Name(), ".") || !d.Type().IsRegular() {
			continue
		}
		ext := filepath.Ext(d.Name())
		name := strings.TrimSuffix(d.Name(), ext)
		out = append(out, Template{ID: prefix + d.Name(), Name: name, Ext: ext, File: name, from: filepath.Join(dir, d.Name())})
	}
	return out
}
