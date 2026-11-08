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

func TestOnlyATextInAProgramOfWindowsIsRead(t *testing.T) {
	root := `C:\Windows`
	cases := []struct {
		ref, want string
		ok        bool
	}{
		{`@%SystemRoot%\system32\shell32.dll,-21770`, `@C:\Windows\system32\shell32.dll,-21770`, true},
		{`@%windir%\System32\imageres.dll,-3`, `@C:\Windows\System32\imageres.dll,-3`, true},
		{`@shell32.dll,-21813`, `@C:\Windows\System32\shell32.dll,-21813`, true},
		{`Meine Fotos`, `Meine Fotos`, true},
		{`@\host\share\x.dll,-1`, "", false},
		{`@//host/share/x.dll,-1`, "", false},
		{`@\?\UNC\host\share\x.dll,-1`, "", false},
		{`@C:\Users\anna\x.dll,-1`, "", false},
		{`@D:\Windows\shell32.dll,-1`, "", false},
		{`@%SystemRoot%\..\Users\anna\x.dll,-1`, "", false},
		{`@%TEMP%\x.dll,-1`, "", false},
		{`@%SystemRoot%\system32\shell32.dll,1`, "", false},
		{`@{Microsoft.Windows.Photos_8wekyb3d8bbwe?ms-resource://Photos/Files/x}`, "", false},
	}
	for _, c := range cases {
		got, ok := localResource(c.ref, root)
		if ok != c.ok || got != c.want {
			t.Errorf("localResource(%q) = %q, %v, want %q, %v", c.ref, got, ok, c.want, c.ok)
		}
	}
}
