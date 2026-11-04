package fsapi

import (
	"errors"
	"io/fs"
	"time"

	"golang.org/x/sys/unix"
)

// statx is the one call that brings the birth time along with everything
// else, so a listing costs one system call per entry.
func lstatFile(p string) (fileStat, error) { return statx(p, unix.AT_SYMLINK_NOFOLLOW) }

func statFile(p string) (fileStat, error) { return statx(p, 0) }

func statx(p string, flags int) (fileStat, error) {
	var stx unix.Statx_t
	err := unix.Statx(unix.AT_FDCWD, p, flags, unix.STATX_BASIC_STATS|unix.STATX_BTIME, &stx)
	for errors.Is(err, unix.EINTR) {
		err = unix.Statx(unix.AT_FDCWD, p, flags, unix.STATX_BASIC_STATS|unix.STATX_BTIME, &stx)
	}
	// Seccomp profiles of older container runtimes reject statx.
	if errors.Is(err, unix.ENOSYS) || errors.Is(err, unix.EPERM) {
		return statPlain(p, flags)
	}
	if err != nil {
		return fileStat{}, &fs.PathError{Op: "statx", Path: p, Err: err}
	}
	st := fileStat{
		mode:  modeOf(uint32(stx.Mode)),
		size:  int64(stx.Size),
		mtime: time.Unix(stx.Mtime.Sec, int64(stx.Mtime.Nsec)),
		ino:   stx.Ino,
		nlink: uint64(stx.Nlink),
		ids:   true,
		uid:   stx.Uid,
		gid:   stx.Gid,
	}
	if stx.Mask&unix.STATX_BTIME != 0 && stx.Btime.Sec > 0 {
		st.btime = time.Unix(stx.Btime.Sec, int64(stx.Btime.Nsec))
	}
	return st, nil
}

func statPlain(p string, flags int) (fileStat, error) {
	var st unix.Stat_t
	var err error
	if flags&unix.AT_SYMLINK_NOFOLLOW != 0 {
		err = unix.Lstat(p, &st)
	} else {
		err = unix.Stat(p, &st)
	}
	if err != nil {
		return fileStat{}, &fs.PathError{Op: "stat", Path: p, Err: err}
	}
	return fileStat{
		mode:  modeOf(st.Mode),
		size:  st.Size,
		mtime: time.Unix(st.Mtim.Sec, st.Mtim.Nsec),
		ino:   st.Ino,
		nlink: uint64(st.Nlink),
		ids:   true,
		uid:   st.Uid,
		gid:   st.Gid,
	}, nil
}

func modeOf(m uint32) fs.FileMode {
	mode := fs.FileMode(m & 0o777)
	switch m & unix.S_IFMT {
	case unix.S_IFDIR:
		mode |= fs.ModeDir
	case unix.S_IFLNK:
		mode |= fs.ModeSymlink
	case unix.S_IFIFO:
		mode |= fs.ModeNamedPipe
	case unix.S_IFSOCK:
		mode |= fs.ModeSocket
	case unix.S_IFBLK:
		mode |= fs.ModeDevice
	case unix.S_IFCHR:
		mode |= fs.ModeDevice | fs.ModeCharDevice
	}
	if m&unix.S_ISUID != 0 {
		mode |= fs.ModeSetuid
	}
	if m&unix.S_ISGID != 0 {
		mode |= fs.ModeSetgid
	}
	if m&unix.S_ISVTX != 0 {
		mode |= fs.ModeSticky
	}
	return mode
}
