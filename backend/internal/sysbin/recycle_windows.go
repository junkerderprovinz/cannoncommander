package sysbin

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"time"
	"unsafe"

	"golang.org/x/sys/windows"

	"github.com/junkerderprovinz/cannoncommander/backend/pkg/winshell"
)

// recycleBin is the Windows Recycle Bin. Every fixed drive keeps one folder
// per account, $Recycle.Bin\<SID>, where each deleted item becomes a $R file
// with its data and a $I file that records where it came from.
type recycleBin struct {
	sid string
}

// Open returns the Recycle Bin of the account running the app.
func Open() (Bin, error) {
	u, err := windows.GetCurrentProcessToken().GetTokenUser()
	if err != nil {
		return nil, err
	}
	return &recycleBin{sid: u.User.Sid.String()}, nil
}

func (b *recycleBin) folder(drive string) string {
	return drive + `:\$Recycle.Bin\` + b.sid
}

// hasBin reports whether Windows recycles on the drive of path. Network
// drives, USB sticks and card readers have no Recycle Bin.
func hasBin(path string) (string, bool) {
	vol := filepath.VolumeName(path)
	if len(vol) != 2 || vol[1] != ':' {
		return "", false
	}
	root, err := windows.UTF16PtrFromString(vol + `\`)
	if err != nil {
		return "", false
	}
	return strings.ToUpper(vol[:1]), windows.GetDriveType(root) == windows.DRIVE_FIXED
}

func (b *recycleBin) Put(path string) (Entry, error) {
	drive, ok := hasBin(path)
	if !ok {
		return Entry{}, ErrNoBin
	}
	since := time.Now().Add(-2 * time.Second)
	if err := shellRecycle(path); err != nil {
		return Entry{}, err
	}
	if e, ok := b.newest(drive, path, since); ok {
		return e, nil
	}
	if _, err := os.Lstat(path); err == nil {
		return Entry{}, fmt.Errorf("%s did not go to the Recycle Bin", path)
	}
	// Windows asked whether to delete it for good, as it does when the
	// Recycle Bin is too small, and the answer was yes.
	return Entry{}, ErrNoBin
}

// newest finds the entry that came from origin last, at the earliest at
// since, for the item the shell just recycled.
func (b *recycleBin) newest(drive, origin string, since time.Time) (Entry, bool) {
	var found Entry
	for _, e := range b.listDrive(drive) {
		if strings.EqualFold(e.Origin, origin) && !e.Deleted.Before(since) && e.Deleted.After(found.Deleted) {
			found = e
		}
	}
	return found, found.ID != ""
}

// Adopt recycles path through the shell, which sets up the folders with the
// rights Windows expects, and then writes the old place and time into the
// record.
func (b *recycleBin) Adopt(path, origin string, deleted time.Time) (Entry, error) {
	e, err := b.Put(path)
	if err != nil {
		return Entry{}, err
	}
	drive, suffix, _ := splitID(e.ID)
	info := recycleInfo{Size: e.Size, Deleted: deleted, Origin: origin}
	if err := os.WriteFile(filepath.Join(b.folder(drive), "$I"+suffix), info.bytes(), 0o600); err != nil {
		return Entry{}, err
	}
	e.Name, e.Origin, e.Deleted = filepath.Base(origin), origin, deleted
	return e, nil
}

func (b *recycleBin) List() ([]Entry, error) {
	var out []Entry
	for _, d := range fixedDrives() {
		out = append(out, b.listDrive(d)...)
	}
	return out, nil
}

func (b *recycleBin) listDrive(drive string) []Entry {
	names, err := os.ReadDir(b.folder(drive))
	if err != nil {
		return nil
	}
	var out []Entry
	for _, n := range names {
		if suffix, ok := strings.CutPrefix(n.Name(), "$I"); ok {
			if e, err := b.entry(drive, suffix); err == nil {
				out = append(out, e)
			}
		}
	}
	return out
}

func (b *recycleBin) entry(drive, suffix string) (Entry, error) {
	raw, err := os.ReadFile(filepath.Join(b.folder(drive), "$I"+suffix))
	if err != nil {
		return Entry{}, ErrNotFound
	}
	info, err := parseRecycleInfo(raw)
	if err != nil {
		return Entry{}, ErrNotFound
	}
	data := filepath.Join(b.folder(drive), "$R"+suffix)
	st, err := os.Lstat(data)
	if err != nil {
		return Entry{}, ErrNotFound
	}
	return Entry{
		ID: makeID(drive, suffix), Name: filepath.Base(info.Origin), Origin: info.Origin, Path: data,
		Dir: st.IsDir(), Size: info.Size, Deleted: info.Deleted,
	}, nil
}

func (b *recycleBin) Get(id string) (Entry, error) {
	drive, suffix, err := b.split(id)
	if err != nil {
		return Entry{}, err
	}
	return b.entry(drive, suffix)
}

func (b *recycleBin) split(id string) (string, string, error) {
	drive, suffix, err := splitID(id)
	if err != nil || len(drive) != 1 || drive[0] < 'A' || drive[0] > 'Z' {
		return "", "", ErrNotFound
	}
	return drive, suffix, nil
}

func (b *recycleBin) Restore(id, to string) error {
	e, err := b.Get(id)
	if err != nil {
		return err
	}
	if _, err := os.Lstat(to); err == nil {
		return &fs.PathError{Op: "restore", Path: to, Err: fs.ErrExist}
	}
	if err := os.Rename(e.Path, to); err != nil {
		return err
	}
	return b.Forget(id)
}

func (b *recycleBin) Forget(id string) error {
	drive, suffix, err := b.split(id)
	if err != nil {
		return err
	}
	dir := b.folder(drive)
	if err := os.Remove(filepath.Join(dir, "$I"+suffix)); err != nil && !errors.Is(err, fs.ErrNotExist) {
		return err
	}
	shellChanged(dir)
	return nil
}

func fixedDrives() []string {
	mask, err := windows.GetLogicalDrives()
	if err != nil {
		return nil
	}
	var out []string
	for i := range 26 {
		if mask&(1<<i) == 0 {
			continue
		}
		letter := string(rune('A' + i))
		if _, ok := hasBin(letter + `:\`); ok {
			out = append(out, letter)
		}
	}
	return out
}

var (
	shell32              = windows.NewLazySystemDLL("shell32.dll")
	procSHFileOperationW = shell32.NewProc("SHFileOperationW")
	procSHChangeNotify   = shell32.NewProc("SHChangeNotify")
	procSHUpdateRecycle  = shell32.NewProc("SHUpdateRecycleBinIcon")
)

// shFileOpStruct is SHFILEOPSTRUCTW in its 64 bit layout.
type shFileOpStruct struct {
	hwnd         uintptr
	wFunc        uint32
	from         *uint16
	to           *uint16
	flags        uint16
	aborted      int32
	nameMappings uintptr
	title        *uint16
}

const (
	foDelete           = 3
	fofSilent          = 0x4
	fofNoConfirmation  = 0x10
	fofAllowUndo       = 0x40
	fofNoErrorUI       = 0x400
	fofWantNukeWarning = 0x4000

	// Codes SHFileOperation returns besides the usual Win32 errors.
	deCancelled    = 0x75
	deAccessDenied = 0x78
	dePathTooDeep  = 0x79
	deInvalidFiles = 0x7C
	deNameTooLong  = 0x81

	shcneUpdateDir = 0x1000
	shcnfPathW     = 0x5
)

// shellRecycle hands path to the shell with undo allowed, which is what puts
// it in the Recycle Bin. The shell shows nothing except its warning when an
// item is too big for the Recycle Bin and would be gone for good.
func shellRecycle(path string) error {
	from, err := windows.UTF16FromString(path)
	if err != nil {
		return err
	}
	from = append(from, 0)
	op := shFileOpStruct{
		wFunc: foDelete,
		from:  &from[0],
		flags: fofAllowUndo | fofNoConfirmation | fofSilent | fofNoErrorUI | fofWantNukeWarning,
	}
	var r uintptr
	winshell.Apartment(func() error {
		op.hwnd = uintptr(winshell.Owner())
		r, _, _ = procSHFileOperationW.Call(uintptr(unsafe.Pointer(&op)))
		return nil
	})
	if op.aborted != 0 {
		return ErrCancelled
	}
	switch r {
	case 0:
		return nil
	case deCancelled, uintptr(windows.ERROR_CANCELLED):
		return ErrCancelled
	case uintptr(windows.ERROR_ACCESS_DENIED), deAccessDenied:
		return &fs.PathError{Op: "recycle", Path: path, Err: windows.ERROR_ACCESS_DENIED}
	case uintptr(windows.ERROR_FILE_NOT_FOUND), uintptr(windows.ERROR_PATH_NOT_FOUND), deInvalidFiles:
		return &fs.PathError{Op: "recycle", Path: path, Err: fs.ErrNotExist}
	case dePathTooDeep, deNameTooLong, uintptr(windows.ERROR_FILENAME_EXCED_RANGE):
		return &fs.PathError{Op: "recycle", Path: path, Err: windows.ERROR_FILENAME_EXCED_RANGE}
	case uintptr(windows.ERROR_SHARING_VIOLATION), uintptr(windows.ERROR_LOCK_VIOLATION):
		return &fs.PathError{Op: "recycle", Path: path, Err: windows.ERROR_SHARING_VIOLATION}
	}
	return fmt.Errorf("the shell could not recycle %s (code %#x)", path, r)
}

// shellChanged tells the Explorer that a Recycle Bin folder changed behind
// the shell's back, so an open Recycle Bin window and the icon on the desktop
// show it.
func shellChanged(dir string) {
	p, err := windows.UTF16PtrFromString(dir)
	if err != nil {
		return
	}
	procSHChangeNotify.Call(shcneUpdateDir, shcnfPathW, uintptr(unsafe.Pointer(p)), 0)
	procSHUpdateRecycle.Call()
}
