package accounts

import (
	"strings"
	"testing"
	"time"
)

func TestALimiterForgetsKeysNobodyAsksForAgain(t *testing.T) {
	l := newLimiter(5, time.Millisecond)
	l.fail("einmal")
	time.Sleep(5 * time.Millisecond)
	l.fail("ein anderer")
	if _, kept := l.fails["einmal"]; kept {
		t.Error("the old key is still kept")
	}
}

func TestAnEmptyKeyIsNeverBlocked(t *testing.T) {
	l := newLimiter(1, time.Hour)
	l.fail("")
	if l.blocked("") || len(l.fails) != 0 {
		t.Error("an unknown address was counted")
	}
}

func TestTheKeyOfAPairHasAFixedSize(t *testing.T) {
	if long := pairKey(strings.Repeat("a", 1<<20), "192.0.2.1"); len(long) > 64 {
		t.Errorf("%d bytes", len(long))
	}
	if pairKey("Anna", "192.0.2.1") != pairKey("anna", "192.0.2.1") {
		t.Error("case tells names apart")
	}
	if pairKey("anna", "192.0.2.1") == pairKey("anna", "192.0.2.2") {
		t.Error("two addresses share a key")
	}
}

func TestOneIPv6NetworkCountsAsOneAddress(t *testing.T) {
	if clientNet("2001:db8:1:2:3:4:5:6") != clientNet("2001:db8:1:2:ffff::1") {
		t.Error("a client can hop through its own network")
	}
	if clientNet("2001:db8:1:2::1") == clientNet("2001:db8:1:3::1") {
		t.Error("two networks share a key")
	}
	if clientNet("192.0.2.1") == clientNet("192.0.2.2") {
		t.Error("two IPv4 addresses share a key")
	}
	if clientNet("@") != "@" {
		t.Error("what is no address has to stay as it is")
	}
}

func TestALiveResetCodeStaysUntilItRunsOut(t *testing.T) {
	s := newResets()
	first := s.issue(1)
	if first == "" {
		t.Fatal("no code")
	}
	if again := s.issue(1); again != "" {
		t.Errorf("a second code while the first is live: %q", again)
	}
	c := s.codes[first]
	c.until = time.Now().Add(-time.Second)
	s.codes[first] = c
	if s.issue(1) == "" {
		t.Error("no new code after the old one ran out")
	}
	if _, ok := s.redeem(first); ok {
		t.Error("the old code still works")
	}
}
