package archives

import (
	"os"
	"os/exec"
)

// Find returns the first of names on the PATH, else where its installer put
// it, else "". 7-Zip and WinRAR install on Windows without a PATH entry.
func Find(names ...string) string {
	for _, n := range names {
		if p, err := exec.LookPath(n); err == nil {
			return p
		}
	}
	for _, n := range names {
		if p := installed(n); p != "" {
			return p
		}
	}
	return ""
}

// program picks the variable key or the first of names it finds. The image
// has 7zz, Debian and most distributions call the same program 7z.
func program(key string, names ...string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	if p := Find(names...); p != "" {
		return p
	}
	return names[0]
}
