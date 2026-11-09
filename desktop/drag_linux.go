//go:build gtk3

package main

/*
#cgo pkg-config: gtk+-3.0
#include <stdlib.h>

int drag_files(void *window, char **uris);
*/
import "C"

import (
	"errors"
	"unsafe"

	"github.com/wailsapp/wails/v3/pkg/application"
)

var dragEnd = make(chan struct{}, 1)

//export dragEnded
func dragEnded() {
	select {
	case dragEnd <- struct{}{}:
	default:
	}
}

// DragFiles hands the files to GTK's drag and drop. The page's own drag
// cannot carry them to GTK 4 programs such as Nautilus: WebKitGTK answers
// the file transfer portal's format with a bare URI list, which they prefer
// and then fail to read, where GTK answers with a key the portal knows.
func (s *shell) DragFiles(paths []string) error {
	w := s.window.Load()
	if w == nil {
		return errors.New("no window to drag from")
	}
	uris := fileURIs(paths)
	list := unsafe.Slice((**C.char)(C.malloc(C.size_t(len(uris)+1)*C.size_t(unsafe.Sizeof((*C.char)(nil))))), len(uris)+1)
	for i, u := range uris {
		list[i] = C.CString(u)
	}
	list[len(uris)] = nil
	defer func() {
		for _, c := range list {
			C.free(unsafe.Pointer(c))
		}
		C.free(unsafe.Pointer(&list[0]))
	}()
	var started C.int
	application.InvokeSync(func() { started = C.drag_files(w.NativeWindow(), &list[0]) })
	if started == 0 {
		return errors.New("the mouse button was let go before the drag began")
	}
	<-dragEnd
	return nil
}
