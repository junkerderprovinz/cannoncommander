package remotes

import (
	"os"
	"path/filepath"

	"github.com/junkerderprovinz/cannoncommander/backend/internal/system"
	"github.com/junkerderprovinz/cannoncommander/backend/pkg/settings"
)

// Config says where rclone and its files live.
type Config struct {
	// Binary is the rclone program, empty when there is none.
	Binary string
	// Dir holds rclone.conf and the VFS cache. It belongs to the server, so
	// remotes set up by hand on the host do not show up here.
	Dir string
	// Socket is the Unix socket rclone rcd listens on. It stays out of Dir,
	// because Dir may sit on a share where sockets do not work.
	Socket string
	// Mnt is the directory mounts go below, Root the default folder for
	// them.
	Mnt  string
	Root string
}

// ConfigFromEnv returns the defaults with the CC_RCLONE_* variables applied.
// Dir defaults to a folder next to the database, which in the container is
// /config/rclone.
func ConfigFromEnv() Config {
	c := Config{
		Binary: system.RcloneBinary(),
		Dir:    filepath.Join(filepath.Dir(settings.Config.Server.DatabaseV2.Path), "rclone"),
		Socket: filepath.Join(os.TempDir(), "cannoncommander", "rclone.sock"),
		Mnt:    "/mnt",
		Root:   "/mnt/remotes",
	}
	for env, dst := range map[string]*string{
		"CC_RCLONE_DIR":    &c.Dir,
		"CC_RCLONE_SOCKET": &c.Socket,
		"CC_REMOTES_MNT":   &c.Mnt,
		"CC_REMOTES_ROOT":  &c.Root,
	} {
		if v := os.Getenv(env); v != "" {
			*dst = v
		}
	}
	return c
}

func (c Config) confFile() string { return filepath.Join(c.Dir, "rclone.conf") }
func (c Config) cacheDir() string { return filepath.Join(c.Dir, "cache") }
