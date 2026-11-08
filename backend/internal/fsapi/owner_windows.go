package fsapi

import "golang.org/x/sys/windows"

// ownerNames has nothing to report for a listing on Windows, where the owner
// costs a call of its own for every file. ownerOf looks it up for one.
func ownerNames(fileStat) (owner, group string) { return "", "" }

// ownerOf names the owner as Explorer's security page does, with the
// computer or domain in front. Windows has no group of a file to show.
func ownerOf(real string, item *Item) {
	sd, err := windows.GetNamedSecurityInfo(real, windows.SE_FILE_OBJECT, windows.OWNER_SECURITY_INFORMATION)
	if err != nil {
		return
	}
	sid, _, err := sd.Owner()
	// FAT and exFAT keep no owner, and Windows names everyone instead.
	if err != nil || sid == nil || sid.IsWellKnown(windows.WinWorldSid) {
		return
	}
	account, domain, _, err := sid.LookupAccount("")
	switch {
	case err != nil:
		item.Owner = sid.String()
	case domain != "":
		item.Owner = domain + `\` + account
	default:
		item.Owner = account
	}
}

func usage(dir string) (free, total uint64, ok bool) {
	p, err := windows.UTF16PtrFromString(dir)
	if err != nil {
		return 0, 0, false
	}
	if err := windows.GetDiskFreeSpaceEx(p, &free, &total, nil); err != nil {
		return 0, 0, false
	}
	return free, total, true
}
