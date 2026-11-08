package sysbin

import (
	"bufio"
	"bytes"
	"encoding/binary"
	"errors"
	"net/url"
	"strings"
	"time"
	"unicode/utf16"
)

var errBadInfo = errors.New("unreadable trash record")

// recycleInfo is what a $I file in the Windows Recycle Bin holds about the $R
// file beside it.
type recycleInfo struct {
	Size    int64
	Deleted time.Time
	Origin  string
}

// filetimeEpoch is 1970-01-01 counted in the 100 ns steps of a FILETIME,
// which starts in 1601.
const filetimeEpoch = 116444736000000000

func toFiletime(t time.Time) int64 { return t.UnixNano()/100 + filetimeEpoch }

func fromFiletime(ft int64) time.Time { return time.Unix(0, (ft-filetimeEpoch)*100) }

// parseRecycleInfo reads both layouts: version 1 up to Windows 8.1 with a
// fixed field of 260 characters, version 2 since Windows 10 with the length
// in front of the path.
func parseRecycleInfo(b []byte) (recycleInfo, error) {
	if len(b) < 24 {
		return recycleInfo{}, errBadInfo
	}
	le := binary.LittleEndian
	out := recycleInfo{Size: int64(le.Uint64(b[8:])), Deleted: fromFiletime(int64(le.Uint64(b[16:])))}
	var name []byte
	switch le.Uint64(b) {
	case 1:
		if len(b) < 24+520 {
			return recycleInfo{}, errBadInfo
		}
		name = b[24 : 24+520]
	case 2:
		if len(b) < 28 {
			return recycleInfo{}, errBadInfo
		}
		n := int(le.Uint32(b[24:]))
		if n == 0 || len(b) < 28+2*n {
			return recycleInfo{}, errBadInfo
		}
		name = b[28 : 28+2*n]
	default:
		return recycleInfo{}, errBadInfo
	}
	units := make([]uint16, 0, len(name)/2)
	for i := 0; i+1 < len(name); i += 2 {
		u := le.Uint16(name[i:])
		if u == 0 {
			break
		}
		units = append(units, u)
	}
	out.Origin = string(utf16.Decode(units))
	if out.Origin == "" {
		return recycleInfo{}, errBadInfo
	}
	return out, nil
}

// bytes writes version 2, which every Windows since 10 reads.
func (r recycleInfo) bytes() []byte {
	name := append(utf16.Encode([]rune(r.Origin)), 0)
	b := make([]byte, 28+2*len(name))
	le := binary.LittleEndian
	le.PutUint64(b, 2)
	le.PutUint64(b[8:], uint64(r.Size))
	le.PutUint64(b[16:], uint64(toFiletime(r.Deleted)))
	le.PutUint32(b[24:], uint32(len(name)))
	for i, u := range name {
		le.PutUint16(b[28+2*i:], u)
	}
	return b
}

// trashInfo is a .trashinfo file of the freedesktop.org trash. Path is
// absolute in the trash of the home folder and relative to the top of the
// volume in the trash of another volume.
type trashInfo struct {
	Path    string
	Deleted time.Time
}

// infoDate is how the specification writes the time, in local time.
const infoDate = "2006-01-02T15:04:05"

func formatTrashInfo(p string, deleted time.Time) []byte {
	parts := strings.Split(p, "/")
	for i, s := range parts {
		parts[i] = url.PathEscape(s)
	}
	return []byte("[Trash Info]\nPath=" + strings.Join(parts, "/") + "\nDeletionDate=" + deleted.Local().Format(infoDate) + "\n")
}

// parseTrashInfo needs a path. A missing or broken date leaves the time
// empty, since the entry can still be restored without it.
func parseTrashInfo(b []byte) (trashInfo, error) {
	var out trashInfo
	inGroup, found := false, false
	sc := bufio.NewScanner(bytes.NewReader(b))
	for sc.Scan() {
		line := strings.TrimSpace(sc.Text())
		if strings.HasPrefix(line, "[") {
			inGroup = line == "[Trash Info]"
			continue
		}
		key, value, ok := strings.Cut(line, "=")
		if !inGroup || !ok {
			continue
		}
		switch key {
		case "Path":
			p, err := url.PathUnescape(value)
			if err != nil || p == "" {
				return trashInfo{}, errBadInfo
			}
			out.Path, found = p, true
		case "DeletionDate":
			if t, err := time.ParseInLocation(infoDate, value, time.Local); err == nil {
				out.Deleted = t
			}
		}
	}
	if !found {
		return trashInfo{}, errBadInfo
	}
	return out, nil
}
