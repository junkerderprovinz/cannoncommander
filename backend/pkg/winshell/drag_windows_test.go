package winshell

import (
	"slices"
	"strings"
	"testing"
	"unsafe"

	"golang.org/x/sys/windows"
)

var (
	procReleaseStgMedium = ole32.NewProc("ReleaseStgMedium")
	procDragQueryFile    = shell32.NewProc("DragQueryFileW")
)

type formatEtc struct {
	format uint16
	device uintptr
	aspect uint32
	index  int32
	tymed  uint32
}

type stgMedium struct {
	tymed   uint32
	handle  uintptr
	release unsafe.Pointer
}

// droppedFiles reads the file list a program gets when it takes obj as a
// drop.
func droppedFiles(obj unsafe.Pointer) ([]string, error) {
	const cfHDrop, aspectContent, tymedHGlobal, slotGetData = 15, 1, 1, 3
	f := formatEtc{format: cfHDrop, aspect: aspectContent, index: -1, tymed: tymedHGlobal}
	var m stgMedium
	if err := Failed(Call(obj, slotGetData, uintptr(unsafe.Pointer(&f)), uintptr(unsafe.Pointer(&m)))); err != nil {
		return nil, err
	}
	defer procReleaseStgMedium.Call(uintptr(unsafe.Pointer(&m)))
	n, _, _ := procDragQueryFile.Call(m.handle, 0xFFFFFFFF, 0, 0)
	var out []string
	for i := range n {
		buf := make([]uint16, windows.MAX_LONG_PATH)
		procDragQueryFile.Call(m.handle, i, uintptr(unsafe.Pointer(&buf[0])), uintptr(len(buf)))
		out = append(out, strings.ToLower(windows.UTF16ToString(buf)))
	}
	return out, nil
}

func longLower(t *testing.T, paths []string) []string {
	t.Helper()
	out := make([]string, len(paths))
	for i, p := range paths {
		long, err := windows.GetLongPathName(windows.StringToUTF16Ptr(p), nil, 0)
		buf := make([]uint16, long)
		if _, err2 := windows.GetLongPathName(windows.StringToUTF16Ptr(p), &buf[0], long); err != nil || err2 != nil {
			t.Fatal(err, err2)
		}
		out[i] = strings.ToLower(windows.UTF16ToString(buf))
	}
	return out
}

func TestFilesOfOneOrSeveralFoldersGoOutWithTheirPaths(t *testing.T) {
	one := files(t, t.TempDir(), "a.txt", "b.txt")
	several := append(files(t, t.TempDir(), "c.txt"), files(t, t.TempDir(), "d.txt")...)
	cases := [][]string{one, several}
	wants := [][]string{longLower(t, one), longLower(t, several)}
	err := Apartment(func() error {
		for i, paths := range cases {
			obj, err := FilesObject(paths)
			if err != nil {
				return err
			}
			got, err := droppedFiles(obj)
			Release(obj)
			if err != nil {
				return err
			}
			if !slices.Equal(got, wants[i]) {
				t.Errorf("got %v, want %v", got, wants[i])
			}
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
}
