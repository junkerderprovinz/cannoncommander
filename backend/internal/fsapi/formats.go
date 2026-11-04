package fsapi

import (
	_ "embed"
	"encoding/json"
	"path"
	"regexp"
	"strings"
)

// formats.json is the prototype's list of format groups, shared with the
// interface so both sort a file into the same group.
//
//go:embed formats.json
var formatsJSON []byte

var formatByExt = func() map[string]string {
	var groups []struct {
		ID  string   `json:"id"`
		Ext []string `json:"ext"`
	}
	if err := json.Unmarshal(formatsJSON, &groups); err != nil {
		panic("fsapi: formats.json: " + err.Error())
	}
	m := map[string]string{}
	for _, g := range groups {
		for _, e := range g.Ext {
			m[e] = g.ID
		}
	}
	return m
}()

// Rotated logs such as syslog.log.1 keep their real type in front of the number.
var rotatedLog = regexp.MustCompile(`\.log\.\d+$`)

// FormatOf returns the format group of a file name, an id from
// formats.json such as video, or "" when none fits.
func FormatOf(name string) string {
	if rotatedLog.MatchString(name) {
		return formatByExt["log"]
	}
	ext := strings.TrimPrefix(strings.ToLower(path.Ext(name)), ".")
	return formatByExt[ext]
}
