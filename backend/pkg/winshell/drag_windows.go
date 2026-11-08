package winshell

import (
	"errors"
	"sync"
	"unsafe"

	"golang.org/x/sys/windows"
)

var (
	ole32 = windows.NewLazySystemDLL("ole32.dll")

	procOleInitialize = ole32.NewProc("OleInitialize")
	procDoDragDrop    = shell32.NewProc("SHDoDragDrop")
)

// What a drop may do with the files, as DROPEFFECT says it.
const (
	DropCopy = 1
	DropMove = 2
	DropLink = 4
)

// FilesObject is the files at paths as the shell hands them to other
// programs. Files from several folders go under the desktop with their whole
// names, as Explorer hands over the hits of a search. The caller releases it.
func FilesObject(paths []string) (unsafe.Pointer, error) {
	sel, err := Select(paths)
	if err == nil {
		defer sel.Close()
		return sel.DataObject()
	}
	if !errors.Is(err, ErrSeveralFolders) {
		return nil, err
	}
	items := make([]IDList, 0, len(paths))
	defer func() {
		for _, l := range items {
			l.Free()
		}
	}()
	for _, p := range paths {
		l, err := ParseName(p)
		if err != nil {
			return nil, err
		}
		items = append(items, l)
	}
	// The desktop, the root every whole name starts from, is the empty list.
	var desktop [2]byte
	var out unsafe.Pointer
	r, _, _ := procCreateDataObject.Call(uintptr(unsafe.Pointer(&desktop)), uintptr(len(items)), uintptr(unsafe.Pointer(&items[0])), 0, uintptr(unsafe.Pointer(&IIDDataObject)), uintptr(unsafe.Pointer(&out)))
	if err := Failed(r); err != nil {
		return nil, err
	}
	return out, nil
}

// oleOnce sets up OLE on the thread that drags, which keeps it for as long
// as the program runs.
var oleOnce sync.Once

// DragFiles runs the shell's drag and drop with the files at paths from
// where the pointer is, and returns once they are dropped or the drag is
// given up. It runs on the thread of owner's window while the mouse button
// is still down.
func DragFiles(owner windows.HWND, paths []string) error {
	oleOnce.Do(func() { procOleInitialize.Call(0) })
	data, err := FilesObject(paths)
	if err != nil {
		return err
	}
	defer Release(data)
	var effect uint32
	r, _, _ := procDoDragDrop.Call(uintptr(owner), uintptr(data), 0, DropCopy|DropMove|DropLink, uintptr(unsafe.Pointer(&effect)))
	return Failed(r)
}
