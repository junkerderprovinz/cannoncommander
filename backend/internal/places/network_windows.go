//go:build desktop

package places

import (
	"os"
	"time"
	"unsafe"

	"golang.org/x/sys/windows"

	"github.com/junkerderprovinz/cannoncommander/backend/internal/ccfs"
)

var procNetShareEnum = windows.NewLazySystemDLL("netapi32.dll").NewProc("NetShareEnum")

// reachWithin bounds how long a request waits for a computer on the network,
// which takes half a minute to give up when it is switched off.
const reachWithin = 5 * time.Second

func network() ccfs.Network { return lan{} }

// lan reaches the shares of other computers as \\server\share, with the
// credentials Windows keeps for them.
type lan struct{}

func (lan) Share(server, share string) (string, bool) {
	root := `\\` + server + `\` + share
	ok, _ := within(reachWithin, func() bool {
		info, err := os.Stat(root)
		return err == nil && info.IsDir()
	})
	return root, ok
}

// Shares lists the folders server shares, leaving out the hidden ones such
// as C$ and the printers.
func (lan) Shares(server string) []string {
	names, _ := within(reachWithin, func() []string { return diskShares(server) })
	return names
}

// shareInfo1 is SHARE_INFO_1.
type shareInfo1 struct {
	name   *uint16
	typ    uint32
	remark *uint16
}

func diskShares(server string) []string {
	const (
		level        = 1
		preferredMax = 0xFFFFFFFF
		typeMask     = 0xFF
		diskTree     = 0
		special      = 0x80000000
	)
	srv, err := windows.UTF16PtrFromString(`\\` + server)
	if err != nil {
		return nil
	}
	var buf *byte
	var read, total uint32
	r, _, _ := procNetShareEnum.Call(uintptr(unsafe.Pointer(srv)), level, uintptr(unsafe.Pointer(&buf)), preferredMax,
		uintptr(unsafe.Pointer(&read)), uintptr(unsafe.Pointer(&total)), 0)
	if buf != nil {
		defer windows.NetApiBufferFree(buf)
	}
	if r != 0 || buf == nil {
		return nil
	}
	var names []string
	for _, s := range unsafe.Slice((*shareInfo1)(unsafe.Pointer(buf)), read) {
		if s.typ&typeMask == diskTree && s.typ&special == 0 {
			names = append(names, windows.UTF16PtrToString(s.name))
		}
	}
	return names
}

// within runs ask and gives up on its answer after d. The call it waits for
// goes on in the background until the system lets go of it.
func within[T any](d time.Duration, ask func() T) (T, bool) {
	done := make(chan T, 1)
	go func() { done <- ask() }()
	select {
	case v := <-done:
		return v, true
	case <-time.After(d):
		var zero T
		return zero, false
	}
}
