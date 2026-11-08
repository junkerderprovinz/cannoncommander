package fsapi

import (
	"runtime"
	"testing"
)

func TestHiddenFollowsTheRuleOfTheSystem(t *testing.T) {
	cases := []struct {
		name           string
		attrs          uint32
		hidden, system bool
	}{
		{"notes.txt", 0, false, false},
		{".bashrc", 0, runtime.GOOS != "windows", false},
		{"Thumbs.db", attrHidden, true, false},
		{"desktop.ini", attrHidden | attrSystem, true, true},
		{"boot.ini", attrSystem, false, false},
	}
	for _, c := range cases {
		hidden, system := hiddenOf(c.name, c.attrs)
		if hidden != c.hidden || system != c.system {
			t.Errorf("%s with %#x: hidden %v system %v, want %v %v", c.name, c.attrs, hidden, system, c.hidden, c.system)
		}
	}
}
