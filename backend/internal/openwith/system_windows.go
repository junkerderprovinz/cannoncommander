//go:build desktop

package openwith

import (
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"unicode/utf16"
	"unsafe"

	"github.com/junkerderprovinz/cannoncommander/backend/pkg/winshell"
	"golang.org/x/sys/windows"
)

// What AssocQueryString tells about the program that opens a type of file.
const (
	assocExecutable   = 2
	assocFriendlyName = 4
)

type shell struct{}

func newSystem() system { return shell{} }

func (shell) dialog() bool { return true }

func (shell) every() ([]app, error) { return nil, errNoDialog }

// apps lists what Explorer lists under Open with. Its order puts the
// default anywhere, so the default is the one Windows names for the type.
func (shell) apps(file string) ([]app, error) {
	ext := filepath.Ext(file)
	if ext == "" {
		return nil, nil
	}
	var list []app
	err := winshell.Apartment(func() error {
		friendly, exe := assocString(ext, assocFriendlyName), assocString(ext, assocExecutable)
		return eachHandler(ext, func(h unsafe.Pointer) bool {
			id, err := winshell.TaskString(h, handlerName)
			if err != nil || id == "" {
				return false
			}
			name, err := winshell.TaskString(h, handlerUIName)
			if err != nil || name == "" {
				name = strings.TrimSuffix(filepath.Base(id), filepath.Ext(id))
			}
			list = append(list, app{
				id: id, name: name, icon: iconLocation(h),
				def: strings.EqualFold(name, friendly) || strings.EqualFold(id, exe),
			})
			return false
		})
	})
	return defaultFirst(list), err
}

func iconLocation(h unsafe.Pointer) string {
	var p *uint16
	var index int32
	if winshell.Failed(winshell.Call(h, handlerIconLocation, uintptr(unsafe.Pointer(&p)), uintptr(unsafe.Pointer(&index)))) != nil {
		return ""
	}
	defer windows.CoTaskMemFree(unsafe.Pointer(p))
	where := windows.UTF16PtrToString(p)
	if where == "" || strings.HasPrefix(where, "@{") {
		return where
	}
	return where + "," + strconv.Itoa(int(index))
}

func (shell) icon(where string) (string, error) {
	img, err := iconImage(where)
	if err != nil {
		return "", err
	}
	return pngIcon(img)
}

var errGone = errors.New("the program is no longer offered for this file")

// open hands the file to the program the way Explorer does, which also
// starts a packaged app that has no program file to run.
func (shell) open(file, id, ext string) error {
	return winshell.Apartment(func() error {
		obj, err := dataObject(file)
		if err != nil {
			return err
		}
		defer winshell.Release(obj)
		invoked := errGone
		err = eachHandler(ext, func(h unsafe.Pointer) bool {
			if name, _ := winshell.TaskString(h, handlerName); name != id {
				return false
			}
			invoked = winshell.Failed(winshell.Call(h, handlerInvoke, uintptr(obj)))
			return true
		})
		if err != nil {
			return err
		}
		return invoked
	})
}

// openAsInfo is OPENASINFO.
type openAsInfo struct {
	file  *uint16
	class *uint16
	flags uint32
}

// The flags Explorer's Choose another app passes: the box that keeps the
// choice for the type, keeping it once ticked, and opening the file.
const (
	oaifAllowRegistration = 0x1
	oaifRegisterExt       = 0x2
	oaifExec              = 0x4
)

var errCancelled = windows.Errno(windows.ERROR_CANCELLED)

func (shell) choose(file string) error {
	p, err := windows.UTF16PtrFromString(file)
	if err != nil {
		return err
	}
	return winshell.Apartment(func() error {
		owner := winshell.Owner()
		defer shareInput(owner)()
		info := openAsInfo{file: p, flags: oaifAllowRegistration | oaifRegisterExt | oaifExec}
		r, _, _ := procOpenWithDialog.Call(uintptr(owner), uintptr(unsafe.Pointer(&info)))
		err := winshell.Failed(r)
		// Closing the dialog without a pick is no failure.
		if err != nil && uint32(r)&0xffff == uint32(errCancelled) {
			return nil
		}
		return err
	})
}

// openFileName is OPENFILENAMEW.
type openFileName struct {
	structSize    uint32
	owner         windows.HWND
	instance      windows.Handle
	filter        *uint16
	customFilter  *uint16
	maxCustFilter uint32
	filterIndex   uint32
	file          *uint16
	maxFile       uint32
	fileTitle     *uint16
	maxFileTitle  uint32
	initialDir    *uint16
	title         *uint16
	flags         uint32
	fileOffset    uint16
	fileExt       uint16
	defExt        *uint16
	custData      uintptr
	hook          uintptr
	template      *uint16
	reserved      uintptr
	reserved2     uint32
	flagsEx       uint32
}

// The flags of a dialog that opens one program file that exists, and leaves
// the recent files and the working folder as they were.
const (
	ofnHideReadOnly    = 0x4
	ofnNoChangeDir     = 0x8
	ofnPathMustExist   = 0x800
	ofnFileMustExist   = 0x1000
	ofnExplorer        = 0x80000
	ofnDontAddToRecent = 0x2000000
)

// pick shows the system's dialog for opening a file, kept to programs and
// starting in Program Files, where they live.
func (shell) pick(title, programs string) (app, error) {
	if programs == "" {
		programs = "Programs"
	}
	// The filter is pairs of a name and a pattern, each ending in a zero,
	// which a Go string cannot hold, so it is made from UTF-16 by hand.
	filter := append(utf16.Encode([]rune(programs+"\x00*.exe\x00")), 0)
	buf := make([]uint16, 4096)
	var dir, head *uint16
	if pf := os.Getenv("ProgramFiles"); pf != "" {
		dir, _ = windows.UTF16PtrFromString(pf)
	}
	if title != "" {
		head, _ = windows.UTF16PtrFromString(title)
	}
	var chosen string
	err := winshell.Apartment(func() error {
		owner := winshell.Owner()
		defer shareInput(owner)()
		o := openFileName{
			owner: owner, filter: &filter[0], filterIndex: 1, file: &buf[0], maxFile: uint32(len(buf)),
			initialDir: dir, title: head, flags: ofnHideReadOnly | ofnNoChangeDir | ofnPathMustExist | ofnFileMustExist | ofnExplorer | ofnDontAddToRecent,
		}
		o.structSize = uint32(unsafe.Sizeof(o))
		if r, _, _ := procGetOpenFileName.Call(uintptr(unsafe.Pointer(&o))); r != 0 {
			chosen = windows.UTF16ToString(buf)
			return nil
		}
		// No extended error means the person closed the dialog.
		if code, _, _ := procCommDlgExtendedError.Call(); code != 0 {
			return fmt.Errorf("the dialog failed with %#x", code)
		}
		return nil
	})
	if err != nil || chosen == "" {
		return app{}, err
	}
	return app{id: chosen, name: strings.TrimSuffix(filepath.Base(chosen), filepath.Ext(chosen)), icon: chosen + ",0"}, nil
}

func (shell) runs(program string, info os.FileInfo) bool {
	return info.Mode().IsRegular() && strings.EqualFold(filepath.Ext(program), ".exe")
}

// run starts the program itself, so the file goes to it as its one
// argument and no shell reads the command line. A console program gets a
// window of its own rather than the desktop app's hidden console.
func (shell) run(program, file string) error {
	cmd := exec.Command(program, file)
	cmd.Dir = filepath.Dir(file)
	cmd.SysProcAttr = &syscall.SysProcAttr{CreationFlags: windows.CREATE_NEW_CONSOLE}
	if err := cmd.Start(); err != nil {
		return err
	}
	go func() { _ = cmd.Wait() }()
	return nil
}

// shareInput lets the calling thread share the input of the thread that owns
// the app's window, so a dialog it opens comes up in front and takes the
// keys, as one the window's own thread opened would. It returns the undo.
func shareInput(owner windows.HWND) func() {
	if owner == 0 {
		return func() {}
	}
	var pid uint32
	to, err := windows.GetWindowThreadProcessId(owner, &pid)
	if err != nil {
		return func() {}
	}
	me := windows.GetCurrentThreadId()
	if r, _, _ := procAttachThreadInput.Call(uintptr(me), uintptr(to), 1); r == 0 {
		return func() {}
	}
	return func() { procAttachThreadInput.Call(uintptr(me), uintptr(to), 0) }
}
