package meta

import (
	"os"
	"syscall"
	"time"
)

func timesOf(real string) fileTimes {
	info, err := os.Stat(real)
	if err != nil {
		return fileTimes{}
	}
	st, ok := info.Sys().(*syscall.Stat_t)
	if !ok {
		return fileTimes{}
	}
	return fileTimes{
		accessed: time.Unix(st.Atim.Sec, st.Atim.Nsec),
		changed:  time.Unix(st.Ctim.Sec, st.Ctim.Nsec),
	}
}
