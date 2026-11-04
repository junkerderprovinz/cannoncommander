package web

import (
	"net/http"
	"testing"
)

func TestTheServerGivesUpOnSlowHeadersAndIdleConnectionsOnly(t *testing.T) {
	srv := &http.Server{}
	limitSlowClients(srv)
	if srv.ReadHeaderTimeout <= 0 || srv.IdleTimeout <= 0 {
		t.Errorf("slow headers %v, idle %v: both need a limit", srv.ReadHeaderTimeout, srv.IdleTimeout)
	}
	if srv.ReadTimeout != 0 || srv.WriteTimeout != 0 {
		t.Errorf("read %v, write %v: uploads, downloads and event streams run as long as they need", srv.ReadTimeout, srv.WriteTimeout)
	}
}
