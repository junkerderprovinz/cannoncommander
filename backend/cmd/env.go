package cmd

import (
	"os"
	"sort"
	"strings"
)

const (
	envPrefix       = "CC_"
	legacyEnvPrefix = "FILEBROWSER_"
)

// adoptLegacyEnv copies every FILEBROWSER_* variable to its CC_* name so
// containers set up for FileBrowser Quantum keep working. A CC_* variable that
// is already set wins. It returns the adopted legacy names for the start log.
func adoptLegacyEnv() []string {
	var adopted []string
	for _, kv := range os.Environ() {
		name, value, ok := strings.Cut(kv, "=")
		if !ok || !strings.HasPrefix(name, legacyEnvPrefix) {
			continue
		}
		target := envPrefix + strings.TrimPrefix(name, legacyEnvPrefix)
		if _, set := os.LookupEnv(target); set {
			continue
		}
		if err := os.Setenv(target, value); err == nil {
			adopted = append(adopted, name)
		}
	}
	sort.Strings(adopted)
	return adopted
}
