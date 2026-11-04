package meta

import (
	"os"
	"syscall"
	"time"
)

// Windows keeps no status change time, only the last access.
func timesOf(real string) fileTimes {
	info, err := os.Stat(real)
	if err != nil {
		return fileTimes{}
	}
	d, ok := info.Sys().(*syscall.Win32FileAttributeData)
	if !ok {
		return fileTimes{}
	}
	return fileTimes{accessed: time.Unix(0, d.LastAccessTime.Nanoseconds())}
}
