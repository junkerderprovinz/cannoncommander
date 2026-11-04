//go:build !linux && !windows

package meta

func timesOf(string) fileTimes { return fileTimes{} }
