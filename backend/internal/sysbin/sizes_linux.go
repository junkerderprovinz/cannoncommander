package sysbin

import (
	"bufio"
	"fmt"
	"io/fs"
	"net/url"
	"os"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
)

// sizeCache is the directorysizes file of a trash. It keeps the size of each
// folder in it, together with the time of the folder's info file, so a
// listing need not walk every folder again. Each line is the size in bytes,
// that time in seconds and the folder's name in percent encoding.
type sizeCache struct {
	file    string
	rows    map[string]sizeRow
	changed bool
}

type sizeRow struct{ size, stamp int64 }

func readSizes(pl place) *sizeCache {
	c := &sizeCache{file: filepath.Join(pl.dir, "directorysizes"), rows: map[string]sizeRow{}}
	f, err := os.Open(c.file)
	if err != nil {
		return c
	}
	defer f.Close()
	sc := bufio.NewScanner(f)
	for sc.Scan() {
		parts := strings.SplitN(sc.Text(), " ", 3)
		if len(parts) != 3 {
			continue
		}
		size, err1 := strconv.ParseInt(parts[0], 10, 64)
		stamp, err2 := strconv.ParseInt(parts[1], 10, 64)
		name, err3 := url.PathUnescape(parts[2])
		if err1 == nil && err2 == nil && err3 == nil {
			c.rows[name] = sizeRow{size, stamp}
		}
	}
	return c
}

// of returns the size of the folder data, from the cache while its info file
// is unchanged.
func (c *sizeCache) of(name, data, info string) int64 {
	st, err := os.Stat(info)
	if err != nil {
		return 0
	}
	stamp := st.ModTime().Unix()
	if r, ok := c.rows[name]; ok && r.stamp == stamp {
		return r.size
	}
	var size int64
	filepath.WalkDir(data, func(_ string, d fs.DirEntry, err error) error {
		if err == nil && d.Type().IsRegular() {
			if fi, err := d.Info(); err == nil {
				size += fi.Size()
			}
		}
		return nil
	})
	c.rows[name], c.changed = sizeRow{size, stamp}, true
	return size
}

// save writes the cache when it changed. With folders, the folders a listing
// found, lines for anything else go.
func (c *sizeCache) save(folders map[string]bool) {
	if folders != nil {
		for name := range c.rows {
			if !folders[name] {
				delete(c.rows, name)
				c.changed = true
			}
		}
	}
	if !c.changed {
		return
	}
	names := make([]string, 0, len(c.rows))
	for name := range c.rows {
		names = append(names, name)
	}
	slices.Sort(names)
	var b strings.Builder
	for _, name := range names {
		r := c.rows[name]
		fmt.Fprintf(&b, "%d %d %s\n", r.size, r.stamp, url.PathEscape(name))
	}
	// Others read the file at any time, so it is replaced, never rewritten.
	tmp, err := os.CreateTemp(filepath.Dir(c.file), ".directorysizes-")
	if err != nil {
		return
	}
	_, err = tmp.WriteString(b.String())
	if cerr := tmp.Close(); err == nil {
		err = cerr
	}
	if err == nil {
		err = os.Rename(tmp.Name(), c.file)
	}
	if err != nil {
		os.Remove(tmp.Name())
	}
}
