package nativeshell

import (
	"errors"
	"strings"
)

// shellQuote quotes s the way g_shell_quote does.
func shellQuote(s string) string {
	return "'" + strings.ReplaceAll(s, "'", `'\''`) + "'"
}

// doubleQuote quotes s in double quotes, which Nemo uses for Quote=double.
func doubleQuote(s string) string {
	var b strings.Builder
	b.WriteByte('"')
	for i := 0; i < len(s); i++ {
		if strings.IndexByte("\"\\$`", s[i]) >= 0 {
			b.WriteByte('\\')
		}
		b.WriteByte(s[i])
	}
	b.WriteByte('"')
	return b.String()
}

// fill puts codes into command where it says % and the code's letter, and a
// percent sign where it says %%. Other letters stay as they are.
func fill(command string, codes map[byte]string) string {
	var b strings.Builder
	for i := 0; i < len(command); i++ {
		if command[i] == '%' && i+1 < len(command) {
			if v, ok := codes[command[i+1]]; ok {
				b.WriteString(v)
				i++
				continue
			}
			if command[i+1] == '%' {
				b.WriteByte('%')
				i++
				continue
			}
		}
		b.WriteByte(command[i])
	}
	return b.String()
}

// generic is a label with what its field codes fill in, such as the names
// of the files, given as an ellipsis, the way the settings keep it for
// every file.
func generic(label string) string {
	codes := map[byte]string{}
	for c := byte('A'); c <= 'z'; c++ {
		if c <= 'Z' || c >= 'a' {
			codes[c] = "…"
		}
	}
	return fill(label, codes)
}

// splitArgv splits a command line the way g_shell_parse_argv does for
// Thunar and Nemo instead of running a shell, so a quoted file name stays
// one argument. Quotes, backslashes and comments work as in sh, and nothing
// is expanded.
func splitArgv(s string) ([]string, error) {
	var out []string
	var b strings.Builder
	inWord := false
	for i := 0; i < len(s); i++ {
		switch c := s[i]; {
		case c == ' ' || c == '\t' || c == '\n':
			if inWord {
				out = append(out, b.String())
				b.Reset()
				inWord = false
			}
		case c == '#' && !inWord:
			for i+1 < len(s) && s[i+1] != '\n' {
				i++
			}
		case c == '\\':
			i++
			if i == len(s) {
				return nil, errors.New("the command ends in a backslash")
			}
			if s[i] != '\n' {
				b.WriteByte(s[i])
				inWord = true
			}
		case c == '\'':
			end := strings.IndexByte(s[i+1:], '\'')
			if end < 0 {
				return nil, errors.New("the command leaves a single quote open")
			}
			b.WriteString(s[i+1 : i+1+end])
			i += end + 1
			inWord = true
		case c == '"':
			inWord = true
			for i++; ; i++ {
				if i == len(s) {
					return nil, errors.New("the command leaves a double quote open")
				}
				if s[i] == '"' {
					break
				}
				if s[i] == '\\' && i+1 < len(s) && strings.IndexByte("$`\"\\\n", s[i+1]) >= 0 {
					i++
					if s[i] == '\n' {
						continue
					}
				}
				b.WriteByte(s[i])
			}
		default:
			b.WriteByte(c)
			inWord = true
		}
	}
	if inWord {
		out = append(out, b.String())
	}
	if len(out) == 0 {
		return nil, errors.New("the command is empty")
	}
	return out, nil
}
