package main

import (
	"errors"
	"fmt"
	"log"
	"runtime"
	"strings"
	"sync"
	"syscall"
	"time"
	"unsafe"

	"golang.org/x/sys/windows"

	"github.com/junkerderprovinz/cannoncommander/backend/pkg/desktop"
	"github.com/junkerderprovinz/cannoncommander/backend/pkg/winshell"
)

var (
	mpr = windows.NewLazySystemDLL("mpr.dll")

	procRegisterClassEx   = user32.NewProc("RegisterClassExW")
	procCreateWindowEx    = user32.NewProc("CreateWindowExW")
	procDefWindowProc     = user32.NewProc("DefWindowProcW")
	procGetMessage        = user32.NewProc("GetMessageW")
	procPeekMessage       = user32.NewProc("PeekMessageW")
	procDispatchMessage   = user32.NewProc("DispatchMessageW")
	procCreatePopupMenu   = user32.NewProc("CreatePopupMenu")
	procDestroyMenu       = user32.NewProc("DestroyMenu")
	procParseDisplayName  = shell32.NewProc("SHParseDisplayName")
	procBindToParent      = shell32.NewProc("SHBindToParent")
	procWNetGetConnection = mpr.NewProc("WNetGetConnectionW")

	iidShellFolder = windows.GUID{Data1: 0x000214e6, Data4: [8]byte{0xc0, 0, 0, 0, 0, 0, 0, 0x46}}
	iidContextMenu = windows.GUID{Data1: 0x000214e4, Data4: [8]byte{0xc0, 0, 0, 0, 0, 0, 0, 0x46}}
)

const (
	// A network drive whose server is away takes half a minute to answer.
	driveAnswersWithin = 2 * time.Second
	drivePollInterval  = 2 * time.Second
	// Windows sends several messages for one stick: its disk, its volume and
	// its letter.
	driveSettleAfterSignal = 300 * time.Millisecond
	// A disc drive keeps its letter when its tray opens, so the eject waits
	// for the drive to go no longer than this.
	ejectSettlesWithin = 5 * time.Second
)

// volumeWatch tells the server when a drive comes or goes. Windows sends a
// stick, a disc or an attached disk image to every window as it arrives,
// and a new network drive only changes the letters in use, which are asked
// for every two seconds.
type volumeWatch struct {
	once    sync.Once
	mu      sync.Mutex
	changed func()
	later   *time.Timer
}

func (s *shell) Volumes() []desktop.Volume { return listDrives() }

func (s *shell) OnVolumes(changed func()) {
	w := &s.volumes
	w.mu.Lock()
	w.changed = changed
	w.mu.Unlock()
	w.once.Do(func() {
		go func() {
			if err := listenForDevices(w.signal); err != nil {
				log.Printf("drives: %v, the letters in use are all that is watched", err)
			}
		}()
		go pollDrives(w.signal)
	})
}

// signal tells the server once a burst of messages about one device is over.
func (w *volumeWatch) signal() {
	w.mu.Lock()
	defer w.mu.Unlock()
	if w.later != nil {
		w.later.Stop()
	}
	w.later = time.AfterFunc(driveSettleAfterSignal, w.changed)
}

func (s *shell) Eject(id string) error { return ejectDrive(strings.TrimSuffix(id, ":")) }

func (s *shell) Mount(string) (string, error) {
	return "", errors.New("Windows mounts its drives by itself")
}

// listDrives describes every drive with a letter that has something in it.
// The drives are asked side by side, and one that does not answer in time is
// left out until it does.
func listDrives() []desktop.Volume {
	mask, err := windows.GetLogicalDrives()
	if err != nil {
		return nil
	}
	found := make([]*desktop.Volume, 26)
	var wg sync.WaitGroup
	for i := range 26 {
		if mask&(1<<i) == 0 {
			continue
		}
		wg.Go(func() {
			done := make(chan *desktop.Volume, 1)
			go func() { done <- describeDrive(string(rune('A' + i))) }()
			select {
			case found[i] = <-done:
			case <-time.After(driveAnswersWithin):
			}
		})
	}
	wg.Wait()
	var out []desktop.Volume
	for _, v := range found {
		if v != nil {
			out = append(out, *v)
		}
	}
	return out
}

// Bus types of STORAGE_DEVICE_DESCRIPTOR that Explorer shows as drives one
// can pull, and the one of an attached disk image.
const (
	bus1394        = 0x4
	busUSB         = 0x7
	busSD          = 0xC
	busMMC         = 0xD
	busVirtualFile = 0xF
)

// describeDrive tells what the drive at letter is, as Explorer shows it
// under This PC, or nil for a card reader or a disc drive left empty.
func describeDrive(letter string) *desktop.Volume {
	root := letter + `:\`
	p, _ := windows.UTF16PtrFromString(root)
	typ := windows.GetDriveType(p)
	var label [windows.MAX_PATH + 1]uint16
	if err := windows.GetVolumeInformation(p, &label[0], uint32(len(label)), nil, nil, nil, nil, 0); err != nil {
		return nil
	}
	v := &desktop.Volume{ID: letter + ":", Root: root, Label: windows.UTF16ToString(label[:]), Kind: "fixed"}
	switch typ {
	case windows.DRIVE_REMOVABLE:
		v.Kind, v.Eject = "removable", true
	case windows.DRIVE_CDROM:
		v.Kind, v.Eject = "optical", true
	case windows.DRIVE_REMOTE:
		v.Kind, v.Remote = "network", remoteName(letter+":")
	case windows.DRIVE_FIXED:
		switch busType(letter) {
		case busUSB, bus1394, busSD, busMMC:
			v.Kind, v.Eject = "removable", true
		case busVirtualFile:
			v.Kind, v.Eject = "virtual", true
		}
	}
	return v
}

// busType asks the drive what connects it, which tells a USB disk from a
// built-in one although both are fixed drives to Windows.
func busType(letter string) uint32 {
	const (
		ioctlStorageQueryProperty = 0x2D1400
		busTypeOffset             = 28
	)
	p, _ := windows.UTF16PtrFromString(`\\.\` + letter + ":")
	h, err := windows.CreateFile(p, 0, windows.FILE_SHARE_READ|windows.FILE_SHARE_WRITE, nil, windows.OPEN_EXISTING, 0, 0)
	if err != nil {
		return 0
	}
	defer windows.CloseHandle(h)
	// StorageDeviceProperty asked as PropertyStandardQuery: both are zero.
	var query [12]byte
	var out [1024]byte
	var n uint32
	if err := windows.DeviceIoControl(h, ioctlStorageQueryProperty, &query[0], uint32(len(query)), &out[0], uint32(len(out)), &n, nil); err != nil || n < busTypeOffset+4 {
		return 0
	}
	return *(*uint32)(unsafe.Pointer(&out[busTypeOffset]))
}

// remoteName is the share a network drive stands for, as \\nas\daten.
func remoteName(drive string) string {
	local, _ := windows.UTF16PtrFromString(drive)
	buf := make([]uint16, 1024)
	n := uint32(len(buf))
	if r, _, _ := procWNetGetConnection.Call(uintptr(unsafe.Pointer(local)), uintptr(unsafe.Pointer(&buf[0])), uintptr(unsafe.Pointer(&n))); r != 0 {
		return ""
	}
	return windows.UTF16ToString(buf)
}

// pollDrives signals whenever the drive letters in use change.
func pollDrives(signal func()) {
	last, _ := windows.GetLogicalDrives()
	for range time.Tick(drivePollInterval) {
		if now, err := windows.GetLogicalDrives(); err == nil && now != last {
			last = now
			signal()
		}
	}
}

type wndClassEx struct {
	size       uint32
	style      uint32
	wndProc    uintptr
	clsExtra   int32
	wndExtra   int32
	instance   windows.Handle
	icon       windows.Handle
	cursor     windows.Handle
	background windows.Handle
	menuName   *uint16
	className  *uint16
	iconSm     windows.Handle
}

type msg struct {
	hwnd    windows.HWND
	message uint32
	wParam  uintptr
	lParam  uintptr
	time    uint32
	pt      struct{ x, y int32 }
	private uint32
}

// listenForDevices keeps a hidden window that hears of every device that
// arrives or leaves. Windows tells only top-level windows of a new volume,
// so it cannot be a message-only one.
func listenForDevices(signal func()) error {
	const (
		wmDeviceChange      = 0x0219
		dbtDeviceArrival    = 0x8000
		dbtDeviceRemoveDone = 0x8004
		className           = "CannonCommanderDrives"
	)
	runtime.LockOSThread()
	var instance windows.Handle
	if err := windows.GetModuleHandleEx(0, nil, &instance); err != nil {
		return err
	}
	name, _ := windows.UTF16PtrFromString(className)
	proc := syscall.NewCallback(func(hwnd windows.HWND, message uint32, wParam, lParam uintptr) uintptr {
		if message == wmDeviceChange && (wParam == dbtDeviceArrival || wParam == dbtDeviceRemoveDone) {
			signal()
		}
		r, _, _ := procDefWindowProc.Call(uintptr(hwnd), uintptr(message), wParam, lParam)
		return r
	})
	wc := wndClassEx{wndProc: proc, instance: instance, className: name}
	wc.size = uint32(unsafe.Sizeof(wc))
	if r, _, err := procRegisterClassEx.Call(uintptr(unsafe.Pointer(&wc))); r == 0 {
		return fmt.Errorf("register the window class: %w", err)
	}
	if hwnd, _, err := procCreateWindowEx.Call(0, uintptr(unsafe.Pointer(name)), uintptr(unsafe.Pointer(name)), 0, 0, 0, 0, 0, 0, 0, uintptr(instance), 0); hwnd == 0 {
		return fmt.Errorf("create the window: %w", err)
	}
	var m msg
	for {
		if r, _, _ := procGetMessage.Call(uintptr(unsafe.Pointer(&m)), 0, 0, 0); int32(r) <= 0 {
			return nil
		}
		procDispatchMessage.Call(uintptr(unsafe.Pointer(&m)))
	}
}

// cmInvokeCommandInfo is CMINVOKECOMMANDINFO.
type cmInvokeCommandInfo struct {
	size   uint32
	mask   uint32
	hwnd   uintptr
	verb   *byte
	params *byte
	dir    *byte
	show   int32
	hotKey uint32
	icon   uintptr
}

// ejectDrive runs the Eject of the drive at letter from its menu in
// Explorer, which releases a stick, opens a disc drive and detaches a disk
// image, and shows the system's own note when a program still holds a file
// on it.
func ejectDrive(letter string) error {
	const (
		shellFolderGetUIObjectOf    = 10
		contextMenuQueryContextMenu = 3
		contextMenuInvokeCommand    = 4
		swShowNormal                = 1
		firstCommand, lastCommand   = 1, 0x7FFF
	)
	root := letter + `:\`
	return winshell.Apartment(func() error {
		p, _ := windows.UTF16PtrFromString(root)
		var pidl unsafe.Pointer
		r, _, _ := procParseDisplayName.Call(uintptr(unsafe.Pointer(p)), 0, uintptr(unsafe.Pointer(&pidl)), 0, 0)
		if err := winshell.Failed(r); err != nil {
			return fmt.Errorf("find %s: %w", root, err)
		}
		defer windows.CoTaskMemFree(pidl)
		var folder unsafe.Pointer
		var child uintptr
		r, _, _ = procBindToParent.Call(uintptr(pidl), uintptr(unsafe.Pointer(&iidShellFolder)), uintptr(unsafe.Pointer(&folder)), uintptr(unsafe.Pointer(&child)))
		if err := winshell.Failed(r); err != nil {
			return err
		}
		defer winshell.Release(folder)
		var menu unsafe.Pointer
		if err := winshell.Failed(winshell.Call(folder, shellFolderGetUIObjectOf, 0, 1, uintptr(unsafe.Pointer(&child)), uintptr(unsafe.Pointer(&iidContextMenu)), 0, uintptr(unsafe.Pointer(&menu)))); err != nil {
			return err
		}
		defer winshell.Release(menu)
		hmenu, _, _ := procCreatePopupMenu.Call()
		defer procDestroyMenu.Call(hmenu)
		if err := winshell.Failed(winshell.Call(menu, contextMenuQueryContextMenu, hmenu, 0, firstCommand, lastCommand, 0)); err != nil {
			return err
		}
		verb, _ := windows.BytePtrFromString("eject")
		info := cmInvokeCommandInfo{hwnd: uintptr(winshell.Owner()), verb: verb, show: swShowNormal}
		info.size = uint32(unsafe.Sizeof(info))
		if err := winshell.Failed(winshell.Call(menu, contextMenuInvokeCommand, uintptr(unsafe.Pointer(&info)))); err != nil {
			return fmt.Errorf("eject %s: %w", root, err)
		}
		// The shell finishes the eject through this thread's messages, so they
		// are handled until the drive is gone or it is clear it stays.
		pumpUntil(time.Now().Add(ejectSettlesWithin), func() bool { return describeDrive(letter) == nil })
		return nil
	})
}

// pumpUntil handles the messages of this thread until done reports true or
// the deadline passes.
func pumpUntil(deadline time.Time, done func() bool) {
	const removeMessage = 1
	var m msg
	for !done() && time.Now().Before(deadline) {
		for {
			if r, _, _ := procPeekMessage.Call(uintptr(unsafe.Pointer(&m)), 0, 0, 0, removeMessage); r == 0 {
				break
			}
			procDispatchMessage.Call(uintptr(unsafe.Pointer(&m)))
		}
		time.Sleep(100 * time.Millisecond)
	}
}
