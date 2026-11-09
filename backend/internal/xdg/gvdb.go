package xdg

import (
	"bytes"
	"encoding/binary"
	"strings"
)

// gvdbString finds key in a GVDB file, the format dconf keeps its settings
// in, and returns its value when that is a string. Each item of the file's
// table names its parent folder, so a key such as
// /org/gnome/desktop/interface/icon-theme is the names along that chain.
func gvdbString(b []byte, key string) (string, bool) {
	if len(b) < 24 {
		return "", false
	}
	var order binary.ByteOrder = binary.LittleEndian
	switch string(b[:8]) {
	case "GVariant":
	case "raVGtnai":
		order = binary.BigEndian
	default:
		return "", false
	}
	span := func(at []byte) ([]byte, bool) {
		start, end := order.Uint32(at), order.Uint32(at[4:])
		if start > end || uint64(end) > uint64(len(b)) {
			return nil, false
		}
		return b[start:end], true
	}
	table, ok := span(b[16:])
	if !ok || len(table) < 8 {
		return "", false
	}
	bloom, buckets := uint64(order.Uint32(table)&(1<<27-1)), uint64(order.Uint32(table[4:]))
	if 8+4*(bloom+buckets) > uint64(len(table)) {
		return "", false
	}
	items := table[8+4*(bloom+buckets):]
	n := len(items) / 24
	item := func(i int) []byte { return items[i*24 : i*24+24] }
	name := func(i int) (string, bool) {
		var parts []string
		// A parent chain longer than the table is a broken file.
		for range n {
			it := item(i)
			start, size := order.Uint32(it[8:]), order.Uint16(it[12:])
			if uint64(start)+uint64(size) > uint64(len(b)) {
				return "", false
			}
			parts = append(parts, string(b[start:start+uint32(size)]))
			parent := order.Uint32(it[4:])
			if parent == 0xffffffff {
				var full strings.Builder
				for j := len(parts) - 1; j >= 0; j-- {
					full.WriteString(parts[j])
				}
				return full.String(), true
			}
			if parent >= uint32(n) {
				return "", false
			}
			i = int(parent)
		}
		return "", false
	}
	for i := range n {
		it := item(i)
		if it[14] != 'v' {
			continue
		}
		if full, ok := name(i); !ok || full != key {
			continue
		}
		// A variant is its value, a zero byte and its type.
		v, ok := span(it[16:])
		if !ok {
			return "", false
		}
		sep := bytes.LastIndexByte(v, 0)
		if sep < 0 || string(v[sep+1:]) != "s" {
			return "", false
		}
		return strings.TrimSuffix(string(v[:sep]), "\x00"), true
	}
	return "", false
}
