//go:build desktop

package desktop

// Volume is a drive or a file system the computer reports.
type Volume struct {
	// ID names the volume for Eject and Mount: the drive letter on Windows,
	// the udisks2 object or the gvfs folder on Linux.
	ID string
	// Root is the folder the volume starts at, as E:\ or
	// /run/media/jdp/STICK, and "" while it waits to be mounted.
	Root string
	// Label is the name the volume was given, "" when it has none.
	Label string
	// Kind is fixed, removable, optical, network or virtual.
	Kind string
	// Remote is the share behind a network volume, as \\nas\daten or
	// smb://nas/daten.
	Remote string
	// Size is the volume's size in bytes, where the computer knows it
	// without mounting it.
	Size uint64
	// Eject says whether the volume can be ejected, as a stick, a disc or
	// an attached disk image can.
	Eject bool
}

// Volumes is what a desktop app offers that follows the computer's drives.
type Volumes interface {
	// Volumes lists the drives and file systems, mounted or not.
	Volumes() []Volume
	// OnVolumes calls changed whenever that list changes, on any goroutine.
	OnVolumes(changed func())
	// Eject ejects the volume, as Explorer and the file managers of Linux do.
	Eject(id string) error
	// Mount mounts the volume and returns where it went.
	Mount(id string) (string, error)
}

// CurrentVolumes returns what the desktop app knows about the computer's
// drives, or nil in the container and where the app does not follow them.
func CurrentVolumes() Volumes {
	v, _ := Current().(Volumes)
	return v
}
