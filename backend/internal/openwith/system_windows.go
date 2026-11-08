package openwith

import (
	"errors"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"unsafe"

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
	err := apartment(func() error {
		friendly, exe := assocString(ext, assocFriendlyName), assocString(ext, assocExecutable)
		return eachHandler(ext, func(h unsafe.Pointer) bool {
			id, err := taskString(h, handlerName)
			if err != nil || id == "" {
				return false
			}
			name, err := taskString(h, handlerUIName)
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
	if failed(method(h, handlerIconLocation, uintptr(unsafe.Pointer(&p)), uintptr(unsafe.Pointer(&index)))) != nil {
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
func (shell) open(file, id string) error {
	return apartment(func() error {
		obj, err := dataObject(file)
		if err != nil {
			return err
		}
		defer releaseObject(obj)
		invoked := errGone
		err = eachHandler(filepath.Ext(file), func(h unsafe.Pointer) bool {
			if name, _ := taskString(h, handlerName); name != id {
				return false
			}
			invoked = failed(method(h, handlerInvoke, uintptr(obj)))
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
	return apartment(func() error {
		info := openAsInfo{file: p, flags: oaifAllowRegistration | oaifRegisterExt | oaifExec}
		r, _, _ := procOpenWithDialog.Call(uintptr(owner()), uintptr(unsafe.Pointer(&info)))
		err := failed(r)
		// Closing the dialog without a pick is no failure.
		if err != nil && uint32(r)&0xffff == uint32(errCancelled) {
			return nil
		}
		return err
	})
}

// owner is the app's window, which the person just clicked, so the dialog
// opens in front of it.
func owner() windows.HWND {
	w := windows.GetForegroundWindow()
	var pid uint32
	if _, err := windows.GetWindowThreadProcessId(w, &pid); err != nil || int(pid) != os.Getpid() {
		return 0
	}
	return w
}
