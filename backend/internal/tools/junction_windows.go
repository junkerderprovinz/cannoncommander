package tools

import (
	"bytes"
	"encoding/binary"
	"errors"
	"os"
	"unicode/utf16"

	"golang.org/x/sys/windows"
)

// linkWithoutRights makes a junction when Windows refuses a symbolic link
// to a folder. Symbolic links need admin rights or the developer mode,
// a junction needs neither and leads the same way.
func linkWithoutRights(target, at string, err error) error {
	if !errors.Is(err, windows.ERROR_PRIVILEGE_NOT_HELD) {
		return err
	}
	if info, serr := os.Stat(target); serr != nil || !info.IsDir() {
		return err
	}
	return junction(target, at)
}

// junction writes the mount point reparse data that mklink /J writes.
func junction(target, at string) error {
	if err := os.Mkdir(at, 0o777); err != nil {
		return err
	}
	if err := setMountPoint(target, at); err != nil {
		_ = os.Remove(at)
		return err
	}
	return nil
}

func setMountPoint(target, at string) error {
	name, err := windows.UTF16PtrFromString(at)
	if err != nil {
		return err
	}
	h, err := windows.CreateFile(name, windows.GENERIC_WRITE, 0, nil, windows.OPEN_EXISTING,
		windows.FILE_FLAG_OPEN_REPARSE_POINT|windows.FILE_FLAG_BACKUP_SEMANTICS, 0)
	if err != nil {
		return err
	}
	defer windows.CloseHandle(h)

	sub := utf16.Encode([]rune(`\??\` + target))
	shown := utf16.Encode([]rune(target))
	paths := append(append(append(sub, 0), shown...), 0)
	var b bytes.Buffer
	put := func(v any) { _ = binary.Write(&b, binary.LittleEndian, v) }
	put(uint32(windows.IO_REPARSE_TAG_MOUNT_POINT))
	put(uint16(8 + 2*len(paths)))
	put(uint16(0))
	put(uint16(0))
	put(uint16(2 * len(sub)))
	put(uint16(2 * (len(sub) + 1)))
	put(uint16(2 * len(shown)))
	put(paths)
	data := b.Bytes()
	var got uint32
	return windows.DeviceIoControl(h, windows.FSCTL_SET_REPARSE_POINT, &data[0], uint32(len(data)), nil, 0, &got, nil)
}
