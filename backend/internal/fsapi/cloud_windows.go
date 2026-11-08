package fsapi

import "golang.org/x/sys/windows"

// cloudTagMask leaves out the bits that number the providers, so every
// IO_REPARSE_TAG_CLOUD_n matches IO_REPARSE_TAG_CLOUD.
const (
	cloudTag     = 0x9000001A
	cloudTagMask = 0xFFFF0FFF
)

// cloudTagged asks for the reparse tag, which the attributes leave out. Only
// reparse points pay for the call, and outside a synced folder there are few.
func cloudTagged(real string, attrs uint32) bool {
	if attrs&attrReparse == 0 {
		return false
	}
	name, err := windows.UTF16PtrFromString(real)
	if err != nil {
		return false
	}
	var data windows.Win32finddata
	h, err := windows.FindFirstFile(name, &data)
	if err != nil {
		return false
	}
	windows.FindClose(h)
	return data.Reserved0&cloudTagMask == cloudTag
}
