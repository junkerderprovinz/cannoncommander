package office

import (
	"context"
	"database/sql"
	"errors"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/golang-jwt/jwt/v5"

	"github.com/junkerderprovinz/cannoncommander/backend/internal/database/migrate"
)

func newKeys(t *testing.T) *keys {
	t.Helper()
	db, err := sql.Open("sqlite", filepath.Join(t.TempDir(), "main.sqlite"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { db.Close() })
	if _, err := migrate.Apply(context.Background(), db, migrate.Main); err != nil {
		t.Fatal(err)
	}
	return &keys{db: db}
}

func TestALinkOnlyOpensWhileItIsFreshAndSignedByUs(t *testing.T) {
	ctx := context.Background()
	k := newKeys(t)
	key, err := k.get(ctx)
	if err != nil {
		t.Fatal(err)
	}
	now := time.Now()
	link := func(method jwt.SigningMethod, claims jwt.RegisteredClaims) string {
		s, err := jwt.NewWithClaims(method, grant{User: 1, Path: "/Brief.docx", RegisteredClaims: claims}).SignedString(key)
		if err != nil {
			t.Fatal(err)
		}
		return s
	}

	fresh, _, err := k.sign(ctx, grant{User: 1, Path: "/Brief.docx"})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := k.verify(ctx, fresh); err != nil {
		t.Fatalf("a fresh link should open: %v", err)
	}

	cases := map[string]string{
		"expired": link(jwt.SigningMethodHS256, jwt.RegisteredClaims{ExpiresAt: jwt.NewNumericDate(now.Add(-time.Minute))}),
		"issued in the future": link(jwt.SigningMethodHS256, jwt.RegisteredClaims{
			IssuedAt:  jwt.NewNumericDate(now.Add(time.Hour)),
			ExpiresAt: jwt.NewNumericDate(now.Add(2 * time.Hour)),
		}),
		"signed with HS512": link(jwt.SigningMethodHS512, jwt.RegisteredClaims{ExpiresAt: jwt.NewNumericDate(now.Add(time.Hour))}),
		"malformed":         "not.a.link",
	}
	for name, raw := range cases {
		if _, err := k.verify(ctx, raw); !errors.Is(err, errBadToken) {
			t.Errorf("%s: got %v, want %v", name, err, errBadToken)
		}
	}
}

func TestACallbackOnlyCountsWhileItsTokenIsFresh(t *testing.T) {
	const secret = "geheim"
	now := time.Now().Unix()
	call := func(claims jwt.MapClaims) error {
		claims["key"], claims["status"] = "doc", 2
		body := `{"key":"doc","status":2,"token":"` + sign(t, secret, claims) + `"}`
		_, err := readCallback(httptest.NewRequest("POST", "/callback", strings.NewReader(body)), secret)
		return err
	}

	if err := call(jwt.MapClaims{"iat": now, "exp": now + 3600}); err != nil {
		t.Fatalf("a fresh callback should count: %v", err)
	}
	if err := call(jwt.MapClaims{"iat": now - 7200, "exp": now - 60}); !errors.Is(err, jwt.ErrTokenExpired) {
		t.Errorf("expired callback: got %v, want %v", err, jwt.ErrTokenExpired)
	}
	if err := call(jwt.MapClaims{"iat": now + 3600}); !errors.Is(err, jwt.ErrTokenUsedBeforeIssued) {
		t.Errorf("callback issued in the future: got %v, want %v", err, jwt.ErrTokenUsedBeforeIssued)
	}
}
