package unraid

import (
	"os"
	"strconv"
	"time"
)

// Config says where the Unraid files live. Inside the container the paths
// are the host paths, so the defaults fit a normal Unraid template.
type Config struct {
	// Emhttp holds disks.ini, shares.ini and var.ini.
	Emhttp string
	// Shares holds the per share settings, one <share>.cfg each.
	Shares string
	// BootConfig holds docker.cfg and domain.cfg.
	BootConfig string
	// Mdstat is the md driver state with the live parity progress.
	Mdstat string
	// Mnt is the directory with user, user0, the disks and the pools.
	Mnt string
	// Socket is the emhttpd control socket.
	Socket string
	// Notifications is the folder the Unraid notify script writes to.
	Notifications string
	// Write allows spinning disks down and sending notifications. Both go
	// through files and a socket that belong to root on the host, so an
	// admin has to switch it on.
	Write bool
	// Poll is how often the monitor rereads the spin and parity state.
	Poll time.Duration
	// LinkBudget caps the time spent walking a disk for the other names of
	// a hard link.
	LinkBudget time.Duration
}

// DefaultConfig returns the paths of a stock Unraid server.
func DefaultConfig() Config {
	return Config{
		Emhttp:        "/var/local/emhttp",
		Shares:        "/boot/config/shares",
		BootConfig:    "/boot/config",
		Mdstat:        "/proc/mdstat",
		Mnt:           "/mnt",
		Socket:        "/var/run/emhttpd.socket",
		Notifications: "/tmp/notifications",
		Poll:          5 * time.Second,
		LinkBudget:    3 * time.Second,
	}
}

// ConfigFromEnv returns the defaults with the CC_UNRAID_* variables applied.
func ConfigFromEnv() Config {
	c := DefaultConfig()
	for env, dst := range map[string]*string{
		"CC_UNRAID_EMHTTP":        &c.Emhttp,
		"CC_UNRAID_SHARES":        &c.Shares,
		"CC_UNRAID_BOOT_CONFIG":   &c.BootConfig,
		"CC_UNRAID_MDSTAT":        &c.Mdstat,
		"CC_UNRAID_MNT":           &c.Mnt,
		"CC_UNRAID_SOCKET_PATH":   &c.Socket,
		"CC_UNRAID_NOTIFICATIONS": &c.Notifications,
	} {
		if v := os.Getenv(env); v != "" {
			*dst = v
		}
	}
	c.Write, _ = strconv.ParseBool(os.Getenv("CC_UNRAID_SOCKET"))
	if d, err := time.ParseDuration(os.Getenv("CC_UNRAID_POLL")); err == nil && d > 0 {
		c.Poll = d
	}
	return c
}
