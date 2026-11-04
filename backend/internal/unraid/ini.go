package unraid

import (
	"bufio"
	"bytes"
	"strconv"
	"strings"
)

// section is one block of an emhttp ini file. Files without headers come
// back as a single section with an empty name.
type section struct {
	name   string
	values map[string]string
}

func (s section) str(key string) string { return s.values[key] }

func (s section) int(key string) int64 {
	n, _ := strconv.ParseInt(s.values[key], 10, 64)
	return n
}

// kib reads a size that emhttp writes in 1024 byte blocks.
func (s section) kib(key string) int64 { return s.int(key) * 1024 }

func (s section) yes(key string) bool { return s.values[key] == "yes" }

// parseINI reads the ini dialect emhttp writes: ["name"] headers and
// key="value" lines, with /proc/mdstat style key=value lines and # comments
// accepted too. Later keys win, as they do for PHP's parse_ini_file.
func parseINI(data []byte) []section {
	var out []section
	cur := section{values: map[string]string{}}
	started := false
	sc := bufio.NewScanner(bytes.NewReader(data))
	sc.Buffer(make([]byte, 64*1024), 1<<20)
	for sc.Scan() {
		line := strings.TrimSpace(sc.Text())
		if line == "" || line[0] == '#' || line[0] == ';' {
			continue
		}
		if line[0] == '[' && line[len(line)-1] == ']' {
			if started || len(cur.values) > 0 {
				out = append(out, cur)
			}
			cur = section{name: unquote(line[1 : len(line)-1]), values: map[string]string{}}
			started = true
			continue
		}
		key, val, ok := strings.Cut(line, "=")
		if !ok {
			continue
		}
		cur.values[strings.TrimSpace(key)] = unquote(strings.TrimSpace(val))
	}
	if started || len(cur.values) > 0 {
		out = append(out, cur)
	}
	return out
}

func unquote(s string) string {
	if len(s) >= 2 && s[0] == '"' && s[len(s)-1] == '"' {
		return s[1 : len(s)-1]
	}
	return s
}

// list splits a comma separated value such as include="disk1,disk2".
func list(v string) []string {
	var out []string
	for _, p := range strings.Split(v, ",") {
		if p = strings.TrimSpace(p); p != "" {
			out = append(out, p)
		}
	}
	return out
}
