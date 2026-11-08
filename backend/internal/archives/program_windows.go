package archives

import (
	"os"
	"path/filepath"

	"golang.org/x/sys/windows/registry"
)

// installed looks where the installers of 7-Zip and WinRAR record their
// folder, and in Program Files when the registry knows nothing.
var installed = func(name string) string {
	var key, value, exe, folder string
	switch name {
	case "7z", "7zz":
		key, value, exe, folder = `SOFTWARE\7-Zip`, "Path", "7z.exe", "7-Zip"
	case "rar":
		key, value, exe, folder = `SOFTWARE\WinRAR`, "exe64", "Rar.exe", "WinRAR"
	default:
		return ""
	}
	var dirs []string
	for _, root := range []registry.Key{registry.LOCAL_MACHINE, registry.CURRENT_USER} {
		k, err := registry.OpenKey(root, key, registry.QUERY_VALUE|registry.WOW64_64KEY)
		if err != nil {
			continue
		}
		if v, _, err := k.GetStringValue(value); err == nil && v != "" {
			// WinRAR records its program, 7-Zip the folder.
			if filepath.Ext(v) == ".exe" {
				v = filepath.Dir(v)
			}
			dirs = append(dirs, v)
		}
		k.Close()
	}
	for _, env := range []string{"ProgramFiles", "ProgramFiles(x86)"} {
		if pf := os.Getenv(env); pf != "" {
			dirs = append(dirs, filepath.Join(pf, folder))
		}
	}
	for _, d := range dirs {
		p := filepath.Join(d, exe)
		if info, err := os.Stat(p); err == nil && info.Mode().IsRegular() {
			return p
		}
	}
	return ""
}
