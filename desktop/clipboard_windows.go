package main

import (
	"encoding/binary"
	"errors"
	"runtime"
	"strings"
	"sync"
	"time"
	"unicode/utf16"
	"unsafe"

	"golang.org/x/sys/windows"

	"github.com/junkerderprovinz/cannoncommander/backend/pkg/desktop"
	"github.com/junkerderprovinz/cannoncommander/backend/pkg/winshell"
)

var (
	_ desktop.Clipboard = (*shell)(nil)
	_ desktop.Dragger   = (*shell)(nil)
)

var (
	user32   = windows.NewLazySystemDLL("user32.dll")
	kernel32 = windows.NewLazySystemDLL("kernel32.dll")
	shell32  = windows.NewLazySystemDLL("shell32.dll")

	procOpenClipboard     = user32.NewProc("OpenClipboard")
	procCloseClipboard    = user32.NewProc("CloseClipboard")
	procEmptyClipboard    = user32.NewProc("EmptyClipboard")
	procGetClipboardData  = user32.NewProc("GetClipboardData")
	procSetClipboardData  = user32.NewProc("SetClipboardData")
	procRegisterClipboard = user32.NewProc("RegisterClipboardFormatW")
	procGlobalAlloc       = kernel32.NewProc("GlobalAlloc")
	procGlobalFree        = kernel32.NewProc("GlobalFree")
	procGlobalLock        = kernel32.NewProc("GlobalLock")
	procGlobalUnlock      = kernel32.NewProc("GlobalUnlock")
	procGlobalSize        = kernel32.NewProc("GlobalSize")
	procDragQueryFile     = shell32.NewProc("DragQueryFileW")
)

const (
	cfUnicodeText = 13
	cfHDrop       = 15
	gmemMoveable  = 2

	dropCopy = 1
	dropMove = 2
	dropLink = 4
)

// dropEffectFormat is where Explorer says whether files on the clipboard are
// to be copied or moved.
var dropEffectFormat = sync.OnceValue(func() uintptr {
	name, _ := windows.UTF16PtrFromString("Preferred DropEffect")
	f, _, _ := procRegisterClipboard.Call(uintptr(unsafe.Pointer(name)))
	return f
})

var errClipboardBusy = errors.New("another program holds the clipboard")

// openClipboard opens the clipboard for the calling thread, waiting a moment
// for a program that holds it, as clipboard managers do right after a copy.
func openClipboard() error {
	for range 20 {
		if ok, _, _ := procOpenClipboard.Call(uintptr(winshell.Owner())); ok != 0 {
			return nil
		}
		time.Sleep(15 * time.Millisecond)
	}
	return errClipboardBusy
}

func (*shell) ClipboardFiles() ([]string, bool, error) {
	runtime.LockOSThread()
	defer runtime.UnlockOSThread()
	if err := openClipboard(); err != nil {
		return nil, false, err
	}
	defer procCloseClipboard.Call()
	drop, _, _ := procGetClipboardData.Call(cfHDrop)
	if drop == 0 {
		return nil, false, nil
	}
	paths := droppedFiles(drop)
	cut := false
	if h, _, _ := procGetClipboardData.Call(dropEffectFormat()); h != 0 {
		if b := globalBytes(h); len(b) >= 4 {
			effect := binary.LittleEndian.Uint32(b)
			cut = effect&dropMove != 0 && effect&dropCopy == 0
		}
	}
	return paths, cut, nil
}

// CopyFiles puts the files on the clipboard as Explorer does, and their
// paths as text beside them for a program that only takes text.
func (*shell) CopyFiles(paths []string, cut bool) error {
	effect := make([]byte, 4)
	binary.LittleEndian.PutUint32(effect, dropCopy|dropLink)
	if cut {
		binary.LittleEndian.PutUint32(effect, dropMove)
	}
	text := utf16.Encode([]rune(strings.Join(paths, "\r\n") + "\x00"))
	formats := []struct {
		id   uintptr
		data []byte
	}{
		{cfHDrop, dropFiles(paths)},
		{dropEffectFormat(), effect},
		{cfUnicodeText, unsafe.Slice((*byte)(unsafe.Pointer(&text[0])), len(text)*2)},
	}
	runtime.LockOSThread()
	defer runtime.UnlockOSThread()
	if err := openClipboard(); err != nil {
		return err
	}
	defer procCloseClipboard.Call()
	if ok, _, err := procEmptyClipboard.Call(); ok == 0 {
		return err
	}
	for _, f := range formats {
		h, err := globalCopy(f.data)
		if err != nil {
			return err
		}
		// The clipboard owns the memory once it took it.
		if r, _, err := procSetClipboardData.Call(f.id, h); r == 0 {
			procGlobalFree.Call(h)
			return err
		}
	}
	return nil
}

func (*shell) ClearClipboard() error {
	runtime.LockOSThread()
	defer runtime.UnlockOSThread()
	if err := openClipboard(); err != nil {
		return err
	}
	defer procCloseClipboard.Call()
	if ok, _, err := procEmptyClipboard.Call(); ok == 0 {
		return err
	}
	return nil
}

// dropFiles is the DROPFILES structure CF_HDROP holds: a header of 20 bytes
// that says where the names start and that they are UTF-16, then each name
// ending in a zero, then one more zero.
func dropFiles(paths []string) []byte {
	const header = 20
	var names []uint16
	for _, p := range paths {
		names = append(names, utf16.Encode([]rune(p))...)
		names = append(names, 0)
	}
	names = append(names, 0)
	b := make([]byte, header+2*len(names))
	binary.LittleEndian.PutUint32(b[0:], header)
	binary.LittleEndian.PutUint32(b[16:], 1)
	for i, c := range names {
		binary.LittleEndian.PutUint16(b[header+2*i:], c)
	}
	return b
}

// droppedFiles reads the names in a CF_HDROP handle, wide or not.
func droppedFiles(drop uintptr) []string {
	n, _, _ := procDragQueryFile.Call(drop, 0xFFFFFFFF, 0, 0)
	paths := make([]string, 0, n)
	for i := range n {
		size, _, _ := procDragQueryFile.Call(drop, i, 0, 0)
		buf := make([]uint16, size+1)
		procDragQueryFile.Call(drop, i, uintptr(unsafe.Pointer(&buf[0])), uintptr(len(buf)))
		paths = append(paths, windows.UTF16ToString(buf))
	}
	return paths
}

func globalCopy(b []byte) (uintptr, error) {
	h, _, err := procGlobalAlloc.Call(gmemMoveable, uintptr(len(b)))
	if h == 0 {
		return 0, err
	}
	p, _, err := procGlobalLock.Call(h)
	if p == 0 {
		procGlobalFree.Call(h)
		return 0, err
	}
	copy(memAt(p, len(b)), b)
	procGlobalUnlock.Call(h)
	return h, nil
}

func globalBytes(h uintptr) []byte {
	p, _, _ := procGlobalLock.Call(h)
	if p == 0 {
		return nil
	}
	defer procGlobalUnlock.Call(h)
	n, _, _ := procGlobalSize.Call(h)
	return append([]byte(nil), memAt(p, int(n))...)
}

// memAt is the locked global memory at p, which the Go heap does not move.
func memAt(p uintptr, n int) []byte {
	return unsafe.Slice((*byte)(*(*unsafe.Pointer)(unsafe.Pointer(&p))), n)
}
