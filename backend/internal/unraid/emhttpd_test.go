package unraid

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestWriteAccessIsOffByDefault(t *testing.T) {
	e := newEnv(t)
	if _, err := e.svc.Spindown(context.Background(), nil); !errors.Is(err, ErrWriteOff) {
		t.Errorf("spindown: %v", err)
	}
	if err := e.svc.Notify(Notice{Subject: "x"}); !errors.Is(err, ErrWriteOff) {
		t.Errorf("notify: %v", err)
	}
	if ConfigFromEnv().Write {
		t.Error("write access is on without CC_UNRAID_SOCKET")
	}
}

func TestNotifyWritesWhatTheWebInterfaceReads(t *testing.T) {
	e := newEnv(t)
	e.cfg.Write = true
	e.svc = New(e.cfg)
	err := e.svc.Notify(Notice{Subject: `Kopieren "fertig"`, Description: "Filme\nnach disk3", Importance: "warning", Link: "/Main"})
	if err != nil {
		t.Fatal(err)
	}
	for _, dir := range []string{"unread", "archive"} {
		files, _ := filepath.Glob(filepath.Join(e.cfg.Notifications, dir, "CannonCommander_*.notify"))
		if len(files) != 1 {
			t.Fatalf("%s holds %v", dir, files)
		}
		data, _ := os.ReadFile(files[0])
		secs := parseINI(data)
		if len(secs) != 1 || secs[0].str("event") != "CannonCommander" || secs[0].str("subject") != "Kopieren 'fertig'" ||
			secs[0].str("description") != "Filme nach disk3" || secs[0].str("importance") != "warning" || secs[0].int("timestamp") == 0 {
			t.Errorf("%s: %q", dir, data)
		}
		if strings.Contains(string(data), "link=") != (dir == "unread") {
			t.Errorf("%s: link field %q", dir, data)
		}
	}
	if err := e.svc.Notify(Notice{Importance: "loud"}); err == nil {
		t.Error("unknown importance accepted")
	}
}
