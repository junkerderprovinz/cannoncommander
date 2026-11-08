package ccfs

import "strings"

// WindowsNameProblem says why Windows refuses a name, the way Explorer
// does before it tries: nameChars for a character it forbids, nameEnd for a
// dot or a space at the end, which Windows would drop without a word, and
// nameReserved for the names of devices, which stay reserved with any
// ending. It returns "" for a good name.
func WindowsNameProblem(name string) string {
	if strings.ContainsAny(name, `<>:"/\|?*`) || strings.ContainsFunc(name, func(r rune) bool { return r < 32 }) {
		return "nameChars"
	}
	if strings.HasSuffix(name, ".") || strings.HasSuffix(name, " ") {
		return "nameEnd"
	}
	stem, _, _ := strings.Cut(name, ".")
	switch u := strings.ToUpper(strings.TrimRight(stem, " ")); {
	case u == "CON" || u == "PRN" || u == "AUX" || u == "NUL":
		return "nameReserved"
	case len(u) == 4 && (strings.HasPrefix(u, "COM") || strings.HasPrefix(u, "LPT")) && u[3] >= '1' && u[3] <= '9':
		return "nameReserved"
	}
	return ""
}
