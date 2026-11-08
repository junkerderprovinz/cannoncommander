//go:build desktop && !windows

package places

import "github.com/junkerderprovinz/cannoncommander/backend/internal/ccfs"

// network is nil: Linux mounts a share into its tree, under
// /run/user/<uid>/gvfs for one the desktop opened, where / reaches it.
func network() ccfs.Network { return nil }
