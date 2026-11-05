package auth

import (
	"strconv"
	"testing"

	"github.com/go-webauthn/webauthn/webauthn"
)

func TestTheListOfPasskeyOriginsDoesNotGrowWithForgedHosts(t *testing.T) {
	s := &WebAuthnService{wa: &webauthn.WebAuthn{Config: &webauthn.Config{}}}
	s.EnsureOrigin("https://echt.example")
	for i := range 10000 {
		s.EnsureOrigin("https://forged" + strconv.Itoa(i) + ".example")
	}
	if n := len(s.wa.Config.RPOrigins); n > maxPasskeyOrigins {
		t.Errorf("%d origins", n)
	}
	if s.wa.Config.RPOrigins[len(s.wa.Config.RPOrigins)-1] != "https://forged9999.example" {
		t.Error("the newest origin has to work")
	}
}
