package fsapi

import (
	"testing"
	"unicode/utf16"
)

func TestTheLocalizedNameComesFromDesktopIni(t *testing.T) {
	plain := "[.ShellClassInfo]\r\nLocalizedResourceName=@%SystemRoot%\\system32\\shell32.dll,-21770\r\nIconResource=x\r\n"
	if got := localizedName([]byte(plain)); got != `@%SystemRoot%\system32\shell32.dll,-21770` {
		t.Errorf("plain = %q", got)
	}
	wide := []byte{0xFF, 0xFE}
	for _, c := range utf16.Encode([]rune("\r\n[ViewState]\r\nLocalizedResourceName=nein\r\n[.ShellClassInfo]\r\nLocalizedResourceName=Meine Fotos\r\n")) {
		wide = append(wide, byte(c), byte(c>>8))
	}
	if got := localizedName(wide); got != "Meine Fotos" {
		t.Errorf("UTF-16 = %q", got)
	}
	if got := localizedName([]byte("[.ShellClassInfo]\r\nIconResource=x\r\n")); got != "" {
		t.Errorf("without a name = %q", got)
	}
}
