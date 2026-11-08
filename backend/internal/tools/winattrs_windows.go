package tools

import "golang.org/x/sys/windows"

// settable are the attributes SetFileAttributes takes. The others, such as
// the folder bit, come from the file system and stay out of the call.
const settable = windows.FILE_ATTRIBUTE_READONLY | windows.FILE_ATTRIBUTE_HIDDEN | windows.FILE_ATTRIBUTE_SYSTEM |
	windows.FILE_ATTRIBUTE_ARCHIVE | windows.FILE_ATTRIBUTE_TEMPORARY | windows.FILE_ATTRIBUTE_OFFLINE |
	windows.FILE_ATTRIBUTE_NOT_CONTENT_INDEXED | attrPinned | attrUnpinned

func setFlags(p string, flags map[string]bool) error {
	name, err := windows.UTF16PtrFromString(p)
	if err != nil {
		return err
	}
	attrs, err := windows.GetFileAttributes(name)
	if err != nil {
		return err
	}
	attrs = withFlags(attrs, flags) & settable
	if attrs == 0 {
		attrs = windows.FILE_ATTRIBUTE_NORMAL
	}
	return windows.SetFileAttributes(name, attrs)
}
