//go:build !linux

package fsapi

import (
	"io/fs"
	"os"
)

func lstatFile(p string) (fileStat, error) {
	info, err := os.Lstat(p)
	if err != nil {
		return fileStat{}, err
	}
	return fromInfo(info), nil
}

func statFile(p string) (fileStat, error) {
	info, err := os.Stat(p)
	if err != nil {
		return fileStat{}, err
	}
	return fromInfo(info), nil
}

func fromInfo(info fs.FileInfo) fileStat {
	st := fileStat{mode: info.Mode(), size: info.Size(), mtime: info.ModTime()}
	sysDetails(info, &st)
	return st
}
