package templates

import (
	"os"
	"path/filepath"
	"strings"

	"github.com/junkerderprovinz/cannoncommander/backend/internal/winres"
	"golang.org/x/sys/windows/registry"
)

// find reads the ShellNew keys that programs register for their file types,
// under the ending or under its program id. Entries that run a handler or
// a command of their own, such as the shortcut wizard, stay out.
func find() []Template {
	root, err := registry.OpenKey(registry.CLASSES_ROOT, "", registry.ENUMERATE_SUB_KEYS)
	if err != nil {
		return nil
	}
	names, err := root.ReadSubKeyNames(-1)
	root.Close()
	if err != nil {
		return nil
	}
	var out []Template
	for _, ext := range names {
		if !strings.HasPrefix(ext, ".") {
			continue
		}
		prog := defaultValue(ext)
		keys := []string{ext + `\ShellNew`}
		if prog != "" {
			keys = append(keys, ext+`\`+prog+`\ShellNew`)
		}
		for _, at := range keys {
			k, err := registry.OpenKey(registry.CLASSES_ROOT, at, registry.QUERY_VALUE)
			if err != nil {
				continue
			}
			t, ok := shellNew(k, ext, prog)
			k.Close()
			if ok {
				out = append(out, t)
				break
			}
		}
	}
	return out
}

func shellNew(k registry.Key, ext, prog string) (Template, bool) {
	has := func(name string) bool {
		_, _, err := k.GetValue(name, nil)
		return err == nil
	}
	if has("Handler") || has("Command") || has("Directory") {
		return Template{}, false
	}
	t := Template{ID: "shellnew:" + strings.ToLower(ext), Ext: strings.ToLower(ext)}
	switch {
	case has("NullFile"):
	case has("Data"):
		if _, typ, _ := k.GetValue("Data", nil); typ == registry.BINARY {
			t.data, _, _ = k.GetBinaryValue("Data")
		} else {
			s, _, _ := k.GetStringValue("Data")
			t.data = []byte(s)
		}
	case has("FileName"):
		name, _, _ := k.GetStringValue("FileName")
		if t.from = templateFile(name); t.from == "" {
			return Template{}, false
		}
	default:
		return Template{}, false
	}
	item, _, _ := k.GetStringValue("ItemName")
	item = winres.String(item)
	t.Name = typeName(prog)
	if t.Name == "" {
		t.Name = item
	}
	if t.Name == "" {
		return Template{}, false
	}
	t.File = item
	if t.File == "" {
		t.File = t.Name
	}
	return t, true
}

// templateFile finds a FileName where Explorer looks for it: in the
// templates folders of the user and of all users, then in ShellNew.
func templateFile(name string) string {
	if filepath.IsAbs(name) {
		return name
	}
	for _, dir := range []string{
		filepath.Join(os.Getenv("APPDATA"), `Microsoft\Windows\Templates`),
		filepath.Join(os.Getenv("ProgramData"), `Microsoft\Windows\Templates`),
		filepath.Join(os.Getenv("SystemRoot"), "ShellNew"),
	} {
		p := filepath.Join(dir, name)
		if info, err := os.Stat(p); err == nil && info.Mode().IsRegular() {
			return p
		}
	}
	return ""
}

func defaultValue(key string) string {
	k, err := registry.OpenKey(registry.CLASSES_ROOT, key, registry.QUERY_VALUE)
	if err != nil {
		return ""
	}
	defer k.Close()
	v, _, _ := k.GetStringValue("")
	return v
}

// typeName is the name of the type in the language of the system, as the
// Type column of Explorer shows it.
func typeName(prog string) string {
	if prog == "" {
		return ""
	}
	k, err := registry.OpenKey(registry.CLASSES_ROOT, prog, registry.QUERY_VALUE)
	if err != nil {
		return ""
	}
	defer k.Close()
	if v, _, err := k.GetStringValue("FriendlyTypeName"); err == nil {
		if s := winres.String(v); s != "" {
			return s
		}
	}
	v, _, _ := k.GetStringValue("")
	return v
}
