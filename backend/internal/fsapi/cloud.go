package fsapi

import "github.com/junkerderprovinz/cannoncommander/backend/internal/ccfs"

// cloudState is what Explorer's status column shows for a file of a cloud
// provider: online only, on this device, or always kept on it. tagged says
// that the reparse tag names a cloud provider, which a file that is fully
// on the device keeps as its only sign.
func cloudState(attrs uint32, tagged bool) string {
	switch {
	case ccfs.CloudOnly(attrs):
		return "online"
	case attrs&attrPinned != 0:
		return "pinned"
	case tagged || attrs&attrUnpinned != 0:
		return "local"
	}
	return ""
}
