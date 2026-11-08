package fsapi

import (
	"strings"
	"unicode/utf16"
)

// FolderLabel is the name Explorer shows for the folder at real, such as
// Dokumente for Documents, or "" where it shows the folder's own name.
func FolderLabel(real string) string {
	st, err := statFile(real)
	if err != nil {
		return ""
	}
	return labelOf(real, st.attrs)
}

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

// localResource lets through a reference only to a text in a program of
// Windows itself, rewritten to its full path. Any other file the folder
// names would be loaded just because the folder is listed, and a path to
// another computer would hand it the login of whoever looks at the folder.
func localResource(ref, systemRoot string) (string, bool) {
	if !strings.HasPrefix(ref, "@") {
		return ref, true
	}
	comma := strings.LastIndexByte(ref, ',')
	if comma < 0 || systemRoot == "" {
		return "", false
	}
	file, id := strings.ReplaceAll(ref[1:comma], "/", `\`), ref[comma+1:]
	if len(id) < 2 || id[0] != '-' || strings.Trim(id[1:], "0123456789") != "" {
		return "", false
	}
	for _, v := range []string{"%SystemRoot%", "%windir%"} {
		if len(file) >= len(v) && strings.EqualFold(file[:len(v)], v) {
			file = systemRoot + file[len(v):]
		}
	}
	if !strings.ContainsAny(file, `\:`) {
		file = systemRoot + `\System32\` + file
	}
	inside := len(file) > len(systemRoot)+1 && strings.EqualFold(file[:len(systemRoot)+1], systemRoot+`\`)
	if !inside || strings.Contains(file, `\..\`) || strings.ContainsAny(file, "%?*") {
		return "", false
	}
	return "@" + file + "," + id, true
}
