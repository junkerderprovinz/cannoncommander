package nativeshell

import (
	"errors"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"time"
	"unicode"
	"unsafe"

	"github.com/junkerderprovinz/cannoncommander/backend/pkg/winshell"
	"golang.org/x/sys/windows"
)

var (
	user32 = windows.NewLazySystemDLL("user32.dll")

	procCreatePopupMenu   = user32.NewProc("CreatePopupMenu")
	procDestroyMenu       = user32.NewProc("DestroyMenu")
	procGetMenuItemCount  = user32.NewProc("GetMenuItemCount")
	procGetMenuItemInfo   = user32.NewProc("GetMenuItemInfoW")
	procDeleteMenu        = user32.NewProc("DeleteMenu")
	procTrackPopupMenuEx  = user32.NewProc("TrackPopupMenuEx")
	procSetForeground     = user32.NewProc("SetForegroundWindow")
	procPostMessage       = user32.NewProc("PostMessageW")
	procRegisterClassEx   = user32.NewProc("RegisterClassExW")
	procCreateWindowEx    = user32.NewProc("CreateWindowExW")
	procDestroyWindow     = user32.NewProc("DestroyWindow")
	procDefWindowProc     = user32.NewProc("DefWindowProcW")
	procClientToScreen    = user32.NewProc("ClientToScreen")
	procGetKeyState       = user32.NewProc("GetKeyState")
	procPeekMessage       = user32.NewProc("PeekMessageW")
	procTranslateMessage  = user32.NewProc("TranslateMessage")
	procDispatchMessage   = user32.NewProc("DispatchMessageW")
	procMsgWaitForObjects = user32.NewProc("MsgWaitForMultipleObjectsEx")
)

var (
	iidContextMenu  = windows.GUID{Data1: 0x000214e4, Data2: 0x0000, Data3: 0x0000, Data4: [8]byte{0xc0, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x46}}
	iidContextMenu2 = windows.GUID{Data1: 0x000214f4, Data2: 0x0000, Data3: 0x0000, Data4: [8]byte{0xc0, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x46}}
	iidContextMenu3 = windows.GUID{Data1: 0xbcfce0a0, Data2: 0xec17, Data3: 0x11d0, Data4: [8]byte{0x8d, 0x10, 0x00, 0xa0, 0xc9, 0x0f, 0x27, 0x19}}
)

const (
	cmQueryContextMenu = 3
	cmInvokeCommand    = 4
	cmGetCommandString = 5
	cm2HandleMenuMsg   = 6
	cm3HandleMenuMsg2  = 7

	cmfExtendedVerbs = 0x100
	cmfItemMenu      = 0x80
	gcsVerbW         = 4

	cmicMaskUnicode     = 0x4000
	cmicMaskShiftDown   = 0x10000000
	cmicMaskPtInvoke    = 0x20000000
	cmicMaskControlDown = 0x40000000

	// The commands the handlers may number their rows with.
	firstCommand = 1
	lastCommand  = 0x7fff

	miimID      = 0x2
	miimSubmenu = 0x4
	miimString  = 0x40
	miimBitmap  = 0x80
	miimFType   = 0x100
	mftSep      = 0x800
	mfByPos     = 0x400

	tpmRightButton = 0x2
	tpmReturnCmd   = 0x100

	wmNull          = 0x0
	wmDrawItem      = 0x2b
	wmMeasureItem   = 0x2c
	wmInitMenuPopup = 0x117
	wmMenuChar      = 0x120

	wsPopup        = 0x80000000
	wsExToolWindow = 0x80

	vkShift   = 0x10
	vkControl = 0x11

	// How long a shell extension at least gets to finish what a picked row
	// started on this thread, such as the copy that Send to starts.
	afterInvoke = 10 * time.Second
)

var errNotInFront = errors.New("the app's window is not in front")

// errCancelledByUser is what a row returns when the person closes the
// dialog it opened, such as the prompt for administrator rights.
var errCancelledByUser = winshell.HRESULT(0x800704c7)

// Actions is for desktops without a menu of their own.
func Actions([]string) ([]Action, error) { return nil, nil }

// RunAction is for desktops without a menu of their own.
func RunAction([]string, string) error { return ErrUnsupported }

type menuItemInfo struct {
	size, mask, ftype, state, id uint32
	submenu                      windows.Handle
	checked, unchecked           windows.Handle
	data                         uintptr
	typeData                     *uint16
	cch                          uint32
	bmpItem                      windows.Handle
}

type invokeInfo struct {
	size, mask                uint32
	hwnd                      windows.HWND
	verb, params, dir         uintptr
	show                      int32
	hotKey                    uint32
	icon                      windows.Handle
	title                     uintptr
	verbW, paramsW, dirW, tlW uintptr
	x, y                      int32
}

type msg struct {
	hwnd    windows.HWND
	message uint32
	wParam  uintptr
	lParam  uintptr
	time    uint32
	x, y    int32
	private uint32
}

// shown is the menu on screen. Its window procedure runs on the thread
// that shows it, and menuMu keeps it to one at a time.
type shown struct {
	cm, cm2, cm3 unsafe.Pointer
	hidden       func(string) bool
	// paths names each submenu by the rows that lead to it, for the keys
	// of rows that arrive only when it opens.
	paths map[windows.Handle]string
	seen  []Entry
	known map[string]bool
}

var (
	menuMu  sync.Mutex
	current *shown

	classOnce sync.Once
	className *uint16
	classErr  error
)

// NativeMenu reports that Windows draws a menu of its own.
func NativeMenu() bool { return true }

// ShowMenu shows Explorer's menu for files, which have to share a folder,
// over the app's window, and runs the row the person picks. It returns once
// the menu closes, with the rows it held, the hidden ones included, so the
// settings can offer every row it ever showed.
func ShowMenu(files []string, o MenuOptions) ([]Entry, error) {
	owner := winshell.Owner()
	if owner == 0 {
		return nil, errNotInFront
	}
	menuMu.Lock()
	defer menuMu.Unlock()
	done := make(chan error, 1)
	var seen []Entry
	go func() {
		runtime.LockOSThread()
		defer runtime.UnlockOSThread()
		if err := windows.CoInitializeEx(0, windows.COINIT_APARTMENTTHREADED|windows.COINIT_DISABLE_OLE1DDE); err != nil {
			done <- err
			return
		}
		defer windows.CoUninitialize()
		m, err := showMenu(owner, files, o)
		if m != nil {
			seen = m.seen
			defer m.release()
		}
		done <- err
		if err == nil {
			pump(afterInvoke)
		}
	}()
	err := <-done
	return seen, err
}

func showMenu(owner windows.HWND, files []string, o MenuOptions) (*shown, error) {
	sel, err := winshell.Select(files)
	if err != nil {
		return nil, err
	}
	defer sel.Close()
	cm, err := sel.UIObject(owner, &iidContextMenu)
	if err != nil {
		return nil, err
	}
	m := &shown{cm: cm, hidden: o.Hidden, paths: map[windows.Handle]string{}, known: map[string]bool{}}
	if m.hidden == nil {
		m.hidden = func(string) bool { return false }
	}
	m.cm3, _ = winshell.Query(cm, &iidContextMenu3)
	if m.cm3 == nil {
		m.cm2, _ = winshell.Query(cm, &iidContextMenu2)
	}
	hmenu, _, _ := procCreatePopupMenu.Call()
	if hmenu == 0 {
		return m, errors.New("no menu")
	}
	defer procDestroyMenu.Call(hmenu)
	flags := uintptr(cmfItemMenu)
	if o.Extended {
		flags |= cmfExtendedVerbs
	}
	if err := winshell.Failed(winshell.Call(cm, cmQueryContextMenu, hmenu, 0, firstCommand, lastCommand, flags)); err != nil {
		return m, err
	}
	m.prune(windows.Handle(hmenu), "")

	window, err := helperWindow(owner)
	if err != nil {
		return m, err
	}
	defer procDestroyWindow.Call(uintptr(window))
	pt := struct{ x, y int32 }{int32(o.X), int32(o.Y)}
	procClientToScreen.Call(uintptr(owner), uintptr(unsafe.Pointer(&pt)))

	current = m
	procSetForeground.Call(uintptr(window))
	cmd, _, _ := procTrackPopupMenuEx.Call(hmenu, tpmRightButton|tpmReturnCmd, uintptr(pt.x), uintptr(pt.y), uintptr(window), 0)
	// The menu closes properly only after its window has had a message.
	procPostMessage.Call(uintptr(window), wmNull, 0, 0)
	current = nil
	procSetForeground.Call(uintptr(owner))
	if cmd == 0 {
		return m, nil
	}
	return m, m.invoke(owner, int(cmd)-firstCommand, filepath.Dir(files[0]), pt.x, pt.y)
}

func (m *shown) invoke(owner windows.HWND, offset int, dir string, x, y int32) error {
	d, err := windows.UTF16PtrFromString(dir)
	if err != nil {
		return err
	}
	info := invokeInfo{
		mask: cmicMaskUnicode | cmicMaskPtInvoke,
		hwnd: owner,
		// A row is picked by its offset from the first command, passed where
		// a verb's text would go.
		verb: uintptr(offset), verbW: uintptr(offset),
		dirW: uintptr(unsafe.Pointer(d)),
		show: windows.SW_SHOWNORMAL,
		x:    x, y: y,
	}
	info.size = uint32(unsafe.Sizeof(info))
	if down(vkShift) {
		info.mask |= cmicMaskShiftDown
	}
	if down(vkControl) {
		info.mask |= cmicMaskControlDown
	}
	err = winshell.Failed(winshell.Call(m.cm, cmInvokeCommand, uintptr(unsafe.Pointer(&info))))
	if errors.Is(err, errCancelledByUser) {
		return nil
	}
	return err
}

func (m *shown) release() {
	winshell.Release(m.cm3)
	winshell.Release(m.cm2)
	winshell.Release(m.cm)
}

func down(key uintptr) bool {
	s, _, _ := procGetKeyState.Call(key)
	return int16(s) < 0
}

// prune notes every row of menu, which sits below the rows named by path,
// and takes out the ones the person hid.
func (m *shown) prune(menu windows.Handle, path string) {
	n, _, _ := procGetMenuItemCount.Call(uintptr(menu))
	for i := int(int32(n)) - 1; i >= 0; i-- {
		var it menuItemInfo
		it.size = uint32(unsafe.Sizeof(it))
		it.mask = miimID | miimSubmenu | miimFType | miimString | miimBitmap
		if ok, _, _ := procGetMenuItemInfo.Call(uintptr(menu), uintptr(i), 1, uintptr(unsafe.Pointer(&it))); ok == 0 || it.ftype&mftSep != 0 {
			continue
		}
		label := m.text(menu, i, it.cch)
		if label == "" {
			continue
		}
		key := "text:" + path + label
		if it.submenu == 0 && it.id >= firstCommand && it.id <= lastCommand {
			if verb := m.verb(int(it.id) - firstCommand); verb != "" {
				key = "verb:" + verb
			}
		}
		if !m.known[key] {
			m.known[key] = true
			shownLabel := label
			if path != "" {
				shownLabel = strings.ReplaceAll(path, "/", " › ") + label
			}
			m.seen = append(m.seen, Entry{Key: key, Label: shownLabel, Icon: menuPicture(it.bmpItem)})
		}
		if m.hidden(key) {
			procDeleteMenu.Call(uintptr(menu), uintptr(i), mfByPos)
			continue
		}
		if it.submenu != 0 {
			m.paths[it.submenu] = path + label + "/"
			m.prune(it.submenu, path+label+"/")
		}
	}
	tidy(menu)
}

// text is the row's label without its access key and shortcut.
func (m *shown) text(menu windows.Handle, i int, cch uint32) string {
	if cch == 0 {
		return ""
	}
	buf := make([]uint16, cch+1)
	it := menuItemInfo{mask: miimString, typeData: &buf[0], cch: cch + 1}
	it.size = uint32(unsafe.Sizeof(it))
	if ok, _, _ := procGetMenuItemInfo.Call(uintptr(menu), uintptr(i), 1, uintptr(unsafe.Pointer(&it))); ok == 0 {
		return ""
	}
	return cleanLabel(windows.UTF16ToString(buf))
}

// cleanLabel drops what a row shows besides its name: the & before its
// access key, the key in brackets some languages append, and the shortcut
// after a tab.
func cleanLabel(s string) string {
	s, _, _ = strings.Cut(s, "\t")
	if open := strings.LastIndex(s, "(&"); open >= 0 && strings.HasSuffix(s, ")") {
		s = s[:open]
	}
	s = strings.ReplaceAll(strings.ReplaceAll(s, "&&", "\x00"), "&", "")
	return strings.TrimSpace(strings.ReplaceAll(s, "\x00", "&"))
}

// verb is the language-neutral name a handler gives the row at offset, or
// "" for a row without one.
func (m *shown) verb(offset int) string {
	buf := make([]uint16, 256)
	if winshell.Failed(winshell.Call(m.cm, cmGetCommandString, uintptr(offset), gcsVerbW, 0, uintptr(unsafe.Pointer(&buf[0])), uintptr(len(buf)))) != nil {
		return ""
	}
	v := windows.UTF16ToString(buf)
	// Some handlers answer with a verb in ANSI or with whatever was in the
	// buffer, which is no name to keep.
	for _, r := range v {
		if !unicode.IsPrint(r) {
			return ""
		}
	}
	return strings.ToLower(v)
}

// tidy takes out the separators that the hidden rows left at the ends or
// next to each other.
func tidy(menu windows.Handle) {
	n, _, _ := procGetMenuItemCount.Call(uintptr(menu))
	lastSep := true
	for i := 0; i < int(int32(n)); {
		var it menuItemInfo
		it.size = uint32(unsafe.Sizeof(it))
		it.mask = miimFType
		procGetMenuItemInfo.Call(uintptr(menu), uintptr(i), 1, uintptr(unsafe.Pointer(&it)))
		sep := it.ftype&mftSep != 0
		if sep && lastSep {
			procDeleteMenu.Call(uintptr(menu), uintptr(i), mfByPos)
			n--
			continue
		}
		lastSep = sep
		i++
	}
	if n > 0 && lastSep {
		procDeleteMenu.Call(uintptr(menu), n-1, mfByPos)
	}
}

// menuPicture is the picture a row carries as a data URL. Rows that draw
// their own, or show one of the system's stock pictures, have none to keep.
func menuPicture(bm windows.Handle) string {
	const lastStock = 11
	if bm <= lastStock || bm == ^windows.Handle(0) {
		return ""
	}
	img, err := winshell.BitmapImage(bm)
	if err != nil {
		return ""
	}
	url, err := pngDataURL(img)
	if err != nil {
		return ""
	}
	return url
}

// helperWindow is a hidden window of this thread that owns the menu, since
// a menu sends what it needs for drawing to its owner, which has to pass it
// on to the shell's handlers.
func helperWindow(owner windows.HWND) (windows.HWND, error) {
	classOnce.Do(func() {
		className, classErr = windows.UTF16PtrFromString("CannonCommanderShellMenu")
		if classErr != nil {
			return
		}
		var instance windows.Handle
		if classErr = windows.GetModuleHandleEx(0, nil, &instance); classErr != nil {
			return
		}
		wc := struct {
			size, style        uint32
			proc               uintptr
			clsExtra, wndExtra int32
			instance           windows.Handle
			icon, cursor, bg   windows.Handle
			menuName, name     *uint16
			smallIcon          windows.Handle
		}{style: 0, proc: windows.NewCallback(menuProc), instance: instance, name: className}
		wc.size = uint32(unsafe.Sizeof(wc))
		if atom, _, err := procRegisterClassEx.Call(uintptr(unsafe.Pointer(&wc))); atom == 0 {
			classErr = err
		}
	})
	if classErr != nil {
		return 0, classErr
	}
	w, _, err := procCreateWindowEx.Call(wsExToolWindow, uintptr(unsafe.Pointer(className)), 0, wsPopup, 0, 0, 0, 0, uintptr(owner), 0, 0, 0)
	if w == 0 {
		return 0, err
	}
	return windows.HWND(w), nil
}

// menuProc hands the messages a menu sends while it opens and draws its
// submenus to the shell, so Send to and the rows of extensions fill in.
func menuProc(hwnd windows.HWND, message uint32, wParam, lParam uintptr) uintptr {
	if m := current; m != nil {
		switch message {
		case wmInitMenuPopup, wmDrawItem, wmMeasureItem, wmMenuChar:
			handled := false
			var result uintptr
			if m.cm3 != nil {
				handled = winshell.Failed(winshell.Call(m.cm3, cm3HandleMenuMsg2, uintptr(message), wParam, lParam, uintptr(unsafe.Pointer(&result)))) == nil
			} else if m.cm2 != nil && message != wmMenuChar {
				handled = winshell.Failed(winshell.Call(m.cm2, cm2HandleMenuMsg, uintptr(message), wParam, lParam)) == nil
			}
			if message == wmInitMenuPopup {
				if path, ok := m.paths[windows.Handle(wParam)]; ok {
					m.prune(windows.Handle(wParam), path)
				}
			}
			if handled {
				return result
			}
		}
	}
	r, _, _ := procDefWindowProc.Call(uintptr(hwnd), uintptr(message), wParam, lParam)
	return r
}

// pump keeps this thread answering its messages for at least atLeast, which a
// shell extension may need for work a picked row started, and for as long
// as a dialog it opened on this thread is up.
func pump(atLeast time.Duration) {
	const qsAllInput, inputAvailable, pmRemove = 0x4ff, 0x4, 0x1
	deadline := time.Now().Add(atLeast)
	for time.Now().Before(deadline) || showsWindows() {
		procMsgWaitForObjects.Call(0, 0, 250, qsAllInput, inputAvailable)
		var m msg
		for {
			ok, _, _ := procPeekMessage.Call(uintptr(unsafe.Pointer(&m)), 0, 0, 0, pmRemove)
			if ok == 0 {
				break
			}
			procTranslateMessage.Call(uintptr(unsafe.Pointer(&m)))
			procDispatchMessage.Call(uintptr(unsafe.Pointer(&m)))
		}
	}
}
