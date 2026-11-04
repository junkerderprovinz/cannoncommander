package settings

import (
	"strings"

	"github.com/gtsteffaniak/go-logger/logger"
)

// retiredKeys are settings that older versions knew. A config may still
// carry them, and the strict decoder would refuse to start over them.
var retiredKeys = [][]string{
	{"auth", "methods", "ldap"},
	{"auth", "methods", "proxy"},
	{"server", "database", "quotas"},
	{"auth", "methods", "password", "recaptcha"},
	{"frontend", "description"},
	{"frontend", "disableDefaultLinks"},
	{"frontend", "disableNavButtons"},
	{"frontend", "disablePWAInstall"},
	{"frontend", "externalLinks"},
	{"frontend", "favicon"},
	{"frontend", "loginButtonText"},
	{"frontend", "loginIcon"},
	{"frontend", "styling"},
	{"shareDefaults"},
	{"http", "disableWebDAV"},
	{"userDefaults"},
	{"server", "database", "migrateFrom"},
	{"auth", "methods", "jwt"},
	{"auth", "methods", "password", "signup"},
}

// dropRetiredKeys removes the retired keys from a parsed config and says so
// in the log.
func dropRetiredKeys(cfg map[string]interface{}) {
	for _, path := range retiredKeys {
		parent := cfg
		for _, key := range path[:len(path)-1] {
			parent, _ = parent[key].(map[string]interface{})
		}
		last := path[len(path)-1]
		if _, ok := parent[last]; ok {
			delete(parent, last)
			logger.Warningf("%s is no longer a setting and is ignored", strings.Join(path, "."))
		}
	}
}

// warnDenyByDefault says that a source's denyByDefault has no effect of its
// own. It stays readable for the one start that turns it into an access rule
// hiding the source from everyone.
func warnDenyByDefault(cfg map[string]interface{}) {
	server, _ := cfg["server"].(map[string]interface{})
	sources, _ := server["sources"].([]interface{})
	for i, s := range sources {
		source, _ := s.(map[string]interface{})
		config, _ := source["config"].(map[string]interface{})
		if _, ok := config["denyByDefault"]; ok {
			logger.Warningf("server.sources[%d].config.denyByDefault is no longer a setting. The first start of this version made it an access rule, after that it is ignored", i)
		}
	}
}
