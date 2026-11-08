package ccfs

import "testing"

func TestWindowsRefusesTheNamesExplorerRefuses(t *testing.T) {
	cases := map[string]string{
		"Bericht.txt": "", "CONTRA": "", "com0": "", "LPT10.txt": "", ".gitignore": "", "Grüße": "",
		"CON": "nameReserved", "nul.txt": "nameReserved", "Com1": "nameReserved", "lpt9.tar.gz": "nameReserved", "AUX .txt": "nameReserved",
		"a<b": "nameChars", "Was?": "nameChars", `a\b`: "nameChars", "tab\there": "nameChars", "x:y": "nameChars",
		"Bericht.": "nameEnd", "Bericht ": "nameEnd",
	}
	for name, want := range cases {
		if got := WindowsNameProblem(name); got != want {
			t.Errorf("WindowsNameProblem(%q) = %q, want %q", name, got, want)
		}
	}
}
