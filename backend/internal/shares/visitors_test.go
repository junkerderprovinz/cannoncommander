package shares_test

import (
	"encoding/json"
	"net/http"
	"strings"
	"testing"
	"time"

	v1 "github.com/junkerderprovinz/cannoncommander/backend/internal/api/v1"
	"github.com/junkerderprovinz/cannoncommander/backend/internal/shares"
)

func TestTheMessageWaitsBehindThePassword(t *testing.T) {
	e := newEnv(t)
	sh := e.create("anna", map[string]any{"paths": []string{"/mnt/user/fotos/Urlaub"}, "password": "sonnenschein", "message": "  Hallo Oma, hier die Fotos vom Strand!  "})
	if sh.Message != "Hallo Oma, hier die Fotos vom Strand!" {
		t.Fatalf("message %q", sh.Message)
	}
	var pub shares.Public
	e.do("GET", "/s/"+sh.Slug, nil).into(t, &pub)
	if !pub.Locked || pub.Message != "" {
		t.Fatalf("a locked link tells %+v", pub)
	}
	var un shares.Unlocked
	e.do("POST", "/s/"+sh.Slug+"/unlock", map[string]string{"password": "sonnenschein"}).into(t, &un)
	e.do("GET", "/s/"+sh.Slug, nil, "X-Share-Token", un.Token).into(t, &pub)
	if pub.Locked || pub.Message != sh.Message {
		t.Fatalf("unlocked %+v", pub)
	}

	var got shares.Share
	e.do("PATCH", "/shares/"+sh.ID, map[string]any{"message": ""}, "X-User", "anna").into(t, &got)
	if got.Message != "" {
		t.Fatalf("message after clearing %q", got.Message)
	}
	if a := e.do("PATCH", "/shares/"+sh.ID, map[string]any{"message": strings.Repeat("ä", 2001)}, "X-User", "anna"); a.code != http.StatusBadRequest {
		t.Fatalf("a long message: %d", a.code)
	}
}

func goneType(t *testing.T, a answer) string {
	t.Helper()
	if a.code != http.StatusGone {
		t.Fatalf("got %d, want 410: %s", a.code, a.body)
	}
	var p v1.Problem
	if err := json.Unmarshal(a.body, &p); err != nil {
		t.Fatal(err)
	}
	return p.Type
}

func TestALinkThatLeadsNowhereSaysWhy(t *testing.T) {
	e := newEnv(t)
	soon := e.create("anna", map[string]any{"paths": []string{"/mnt/user/fotos/a"}, "expires": time.Now().Add(1100 * time.Millisecond)})
	once := e.create("anna", map[string]any{"paths": []string{"/mnt/user/dokumente/rechnung.pdf"}, "limit": 1})
	mine := e.create("bob", map[string]any{"paths": []string{"/mnt/user/fotos/b"}})

	e.do("GET", "/s/"+once.Slug+"/raw?download=1", nil)
	if got := goneType(t, e.do("GET", "/s/"+once.Slug+"/raw?download=1", nil)); got != "share-used-up" {
		t.Fatalf("the second download: %q", got)
	}
	if got := goneType(t, e.do("GET", "/s/"+once.Slug, nil)); got != "share-used-up" {
		t.Fatalf("a used up link: %q", got)
	}
	e.people["bob"].Permissions.Share = false
	if got := goneType(t, e.do("GET", "/s/"+mine.Slug, nil)); got != "share-withdrawn" {
		t.Fatalf("a link of someone who may no longer share: %q", got)
	}
	time.Sleep(2 * time.Second)
	if got := goneType(t, e.do("GET", "/s/"+soon.Slug, nil)); got != "share-expired" {
		t.Fatalf("an expired link: %q", got)
	}
}
