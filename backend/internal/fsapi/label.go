package fsapi

import (
	"strings"
	"unicode/utf16"
)

// localizedName reads LocalizedResourceName from the [.ShellClassInfo] of a
// desktop.ini, which Windows writes in UTF-16 or in the code page.
func localizedName(ini []byte) string {
	var text string
	if len(ini) >= 2 && ini[0] == 0xFF && ini[1] == 0xFE {
		u := make([]uint16, (len(ini)-2)/2)
		for i := range u {
			u[i] = uint16(ini[2+2*i]) | uint16(ini[3+2*i])<<8
		}
		text = string(utf16.Decode(u))
	} else {
		text = string(ini)
	}
	in := false
	for _, line := range strings.Split(text, "\n") {
		line = strings.TrimSpace(line)
		if strings.HasPrefix(line, "[") {
			in = strings.EqualFold(line, "[.ShellClassInfo]")
			continue
		}
		if k, v, ok := strings.Cut(line, "="); in && ok && strings.EqualFold(strings.TrimSpace(k), "LocalizedResourceName") {
			return strings.TrimSpace(v)
		}
	}
	return ""
}
