package winshell

import (
	"errors"
	"unsafe"

	"golang.org/x/sys/windows"
)

var (
	procParseDisplayName = shell32.NewProc("SHParseDisplayName")
	procBindToParent     = shell32.NewProc("SHBindToParent")
	procCreateDataObject = shell32.NewProc("SHCreateDataObject")
	procPathFromIDList   = shell32.NewProc("SHGetPathFromIDListEx")
	procILClone          = shell32.NewProc("ILClone")
	procILRemoveLastID   = shell32.NewProc("ILRemoveLastID")
	procILFindLastID     = shell32.NewProc("ILFindLastID")
	procILIsEqual        = shell32.NewProc("ILIsEqual")
	procILFree           = shell32.NewProc("ILFree")
)

// shellFolderUIObjectOf is the slot of IShellFolder::GetUIObjectOf.
const shellFolderUIObjectOf = 10

var (
	IIDShellFolder = windows.GUID{Data1: 0x000214e6, Data2: 0x0000, Data3: 0x0000, Data4: [8]byte{0xc0, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x46}}
	IIDDataObject  = windows.GUID{Data1: 0x0000010e, Data2: 0x0000, Data3: 0x0000, Data4: [8]byte{0xc0, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x46}}
)

// IDList is an absolute item ID list, the shell's own name for a file or a
// folder. It belongs to the caller, who gives it back with Free.
type IDList uintptr

// ParseName finds the shell's name for the file or folder at path.
func ParseName(path string) (IDList, error) {
	p, err := windows.UTF16PtrFromString(path)
	if err != nil {
		return 0, err
	}
	var l IDList
	r, _, _ := procParseDisplayName.Call(uintptr(unsafe.Pointer(p)), 0, uintptr(unsafe.Pointer(&l)), 0, 0)
	if err := Failed(r); err != nil {
		return 0, err
	}
	return l, nil
}

// Free gives the list back to the shell.
func (l IDList) Free() {
	if l != 0 {
		procILFree.Call(uintptr(l))
	}
}

// Path is the file system path the list names, for a list that names one.
func (l IDList) Path() (string, error) {
	buf := make([]uint16, windows.MAX_LONG_PATH)
	if ok, _, _ := procPathFromIDList.Call(uintptr(l), uintptr(unsafe.Pointer(&buf[0])), uintptr(len(buf)), 0); ok == 0 {
		return "", errors.New("the item has no path")
	}
	return windows.UTF16ToString(buf), nil
}

// ErrSeveralFolders is returned for items that are not all in one folder,
// which the shell's menus and data objects need.
var ErrSeveralFolders = errors.New("the items are not all in one folder")

// Selection is items of one folder as the shell's context menus and data
// objects take them: the folder, and each item by its name in there.
type Selection struct {
	folder   unsafe.Pointer
	parent   IDList
	items    []IDList
	children []uintptr
}

// Select names the items at paths, which have to share a folder. The caller
// closes the selection.
func Select(paths []string) (*Selection, error) {
	if len(paths) == 0 {
		return nil, errors.New("nothing selected")
	}
	s := &Selection{}
	for _, p := range paths {
		l, err := ParseName(p)
		if err != nil {
			s.Close()
			return nil, err
		}
		s.items = append(s.items, l)
		last, _, _ := procILFindLastID.Call(uintptr(l))
		s.children = append(s.children, last)
		parent, _, _ := procILClone.Call(uintptr(l))
		if parent == 0 {
			s.Close()
			return nil, windows.ERROR_NOT_ENOUGH_MEMORY
		}
		procILRemoveLastID.Call(parent)
		if s.parent == 0 {
			s.parent = IDList(parent)
			continue
		}
		same, _, _ := procILIsEqual.Call(uintptr(s.parent), parent)
		IDList(parent).Free()
		if same == 0 {
			s.Close()
			return nil, ErrSeveralFolders
		}
	}
	var child uintptr
	r, _, _ := procBindToParent.Call(uintptr(s.items[0]), uintptr(unsafe.Pointer(&IIDShellFolder)), uintptr(unsafe.Pointer(&s.folder)), uintptr(unsafe.Pointer(&child)))
	if err := Failed(r); err != nil {
		s.Close()
		return nil, err
	}
	return s, nil
}

// Close lets go of everything the selection holds.
func (s *Selection) Close() {
	Release(s.folder)
	s.folder = nil
	s.parent.Free()
	s.parent = 0
	for _, l := range s.items {
		l.Free()
	}
	s.items, s.children = nil, nil
}

// UIObject asks the folder for an object that works on the selected items,
// such as their context menu. owner is the window its messages go to.
func (s *Selection) UIObject(owner windows.HWND, iid *windows.GUID) (unsafe.Pointer, error) {
	var out unsafe.Pointer
	r := Call(s.folder, shellFolderUIObjectOf, uintptr(owner), uintptr(len(s.children)), uintptr(unsafe.Pointer(&s.children[0])), uintptr(unsafe.Pointer(iid)), 0, uintptr(unsafe.Pointer(&out)))
	if err := Failed(r); err != nil {
		return nil, err
	}
	return out, nil
}

// DataObject is the selected items as the shell hands files to other
// programs, on the clipboard or in a drag. The caller releases it.
func (s *Selection) DataObject() (unsafe.Pointer, error) {
	var out unsafe.Pointer
	r, _, _ := procCreateDataObject.Call(uintptr(s.parent), uintptr(len(s.children)), uintptr(unsafe.Pointer(&s.children[0])), 0, uintptr(unsafe.Pointer(&IIDDataObject)), uintptr(unsafe.Pointer(&out)))
	if err := Failed(r); err != nil {
		return nil, err
	}
	return out, nil
}
