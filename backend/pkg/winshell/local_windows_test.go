package winshell

import "testing"

func TestAPathOnAnotherComputerIsNotLocal(t *testing.T) {
	for p, want := range map[string]bool{
		`C:\Windows\system32\shell32.dll`:    true,
		`%SystemRoot%\system32\imageres.dll`: true,
		`imageres.dll`:                       true,
		``:                                   true,
		`\\server\share\x.ico`:               false,
		`//server/share/x.ico`:               false,
		`\\?\UNC\server\share\x.ico`:         false,
		`file://server/share/x.ico`:          false,
		`https://example.org/x.ico`:          false,
	} {
		if got := LocalPath(p); got != want {
			t.Errorf("%q: local %v, want %v", p, got, want)
		}
	}
}
