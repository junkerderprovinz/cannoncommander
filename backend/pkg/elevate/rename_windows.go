package elevate

import (
	"errors"
	"os"
	"unsafe"

	"golang.org/x/sys/windows"
)

// renameAt moves from in src to to in dst. It opens the entry relative to its
// folder's handle without following it, and renames it with the target
// folder's handle as the root of the new name, so no path is looked up
// again. It never replaces what appeared at to meanwhile.
func renameAt(src *os.Root, from string, dst *os.Root, to string) error {
	sd, err := src.Open(".")
	if err != nil {
		return err
	}
	defer sd.Close()
	dd, err := dst.Open(".")
	if err != nil {
		return err
	}
	defer dd.Close()
	name, err := windows.NewNTUnicodeString(from)
	if err != nil {
		return err
	}
	oa := windows.OBJECT_ATTRIBUTES{RootDirectory: windows.Handle(sd.Fd()), ObjectName: name, Attributes: windows.OBJ_CASE_INSENSITIVE}
	oa.Length = uint32(unsafe.Sizeof(oa))
	var h windows.Handle
	var iosb windows.IO_STATUS_BLOCK
	err = windows.NtCreateFile(&h, windows.DELETE|windows.SYNCHRONIZE, &oa, &iosb, nil, 0,
		windows.FILE_SHARE_READ|windows.FILE_SHARE_WRITE|windows.FILE_SHARE_DELETE, windows.FILE_OPEN,
		windows.FILE_OPEN_REPARSE_POINT|windows.FILE_SYNCHRONOUS_IO_NONALERT|windows.FILE_OPEN_FOR_BACKUP_INTENT, 0, 0)
	if st, ok := err.(windows.NTStatus); ok {
		err = st.Errno()
	}
	if err != nil {
		return &os.LinkError{Op: "rename", Old: from, New: to, Err: err}
	}
	defer windows.CloseHandle(h)
	newName, err := windows.UTF16FromString(to)
	if err != nil {
		return err
	}
	newName = newName[:len(newName)-1]
	// FILE_RENAME_INFORMATION in its 64 bit layout: the flags, the handle of
	// the folder the name is relative to, the name's length in bytes, the
	// name. Windows wants at least the size of the struct, padding included,
	// plus the name.
	const nameAt, size = 20, 24
	buf := make([]uint64, (size+2*len(newName)+7)/8)
	raw := unsafe.Slice((*byte)(unsafe.Pointer(&buf[0])), len(buf)*8)
	*(*uintptr)(unsafe.Pointer(&raw[8])) = dd.Fd()
	*(*uint32)(unsafe.Pointer(&raw[16])) = uint32(2 * len(newName))
	copy(unsafe.Slice((*uint16)(unsafe.Pointer(&raw[nameAt])), len(newName)), newName)
	err = windows.NtSetInformationFile(h, &iosb, &raw[0], uint32(len(raw)), windows.FileRenameInformation)
	if st, ok := err.(windows.NTStatus); ok {
		err = st.Errno()
	}
	if err != nil {
		return &os.LinkError{Op: "rename", Old: from, New: to, Err: err}
	}
	return nil
}

func crossVolume(err error) bool { return errors.Is(err, windows.ERROR_NOT_SAME_DEVICE) }
