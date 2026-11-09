package elevate

import (
	"errors"
	"testing"
)

func TestTheUserNamespaceOperationsTakeNoPaths(t *testing.T) {
	for _, kind := range []string{AllowUserNamespaces, DisallowUserNamespaces} {
		if err := (Op{Kind: kind}).check(); err != nil {
			t.Errorf("%s: %v", kind, err)
		}
		for _, o := range []Op{{Kind: kind, From: "/etc/passwd"}, {Kind: kind, To: "/etc/apparmor.d/x"}, {Kind: kind, Skip: []string{"/a"}}, {Kind: kind, Replace: true}, {Kind: kind, Parents: true}} {
			if err := o.check(); !errors.Is(err, ErrBadOp) {
				t.Errorf("%+v: got %v", o, err)
			}
		}
	}
}
