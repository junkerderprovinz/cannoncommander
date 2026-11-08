package nativeshell

import (
	"errors"
	"path/filepath"
	"testing"
	"time"
)

func TestWithoutASessionBusThereIsNoPropertiesDialog(t *testing.T) {
	t.Setenv("DBUS_SESSION_BUS_ADDRESS", "unix:path="+filepath.Join(t.TempDir(), "no-bus"))
	session.Lock()
	session.conn, session.checked = nil, time.Time{}
	session.Unlock()
	if HasProperties() {
		t.Error("properties are offered")
	}
	if err := Properties([]string{"/tmp"}); !errors.Is(err, ErrUnsupported) {
		t.Errorf("got %v", err)
	}
	if CanRunAsAdmin("/usr/bin/true") || !errors.Is(RunAsAdmin("/usr/bin/true"), ErrUnsupported) {
		t.Error("running as administrator is offered")
	}
}
