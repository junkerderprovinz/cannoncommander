package office

import (
	"context"
	"crypto/rand"
	"database/sql"
	"errors"
	"fmt"
	"sync"
	"time"

	"github.com/golang-jwt/jwt/v4"
	"github.com/junkerderprovinz/cannoncommander/backend/internal/database/migrate"
)

func init() {
	migrate.Register(migrate.Main,
		// The key signs the links the document server calls back on. It
		// stays across restarts, so open documents can still be saved.
		migrate.SQL("office/001_create",
			`CREATE TABLE office_key (
				id INTEGER PRIMARY KEY CHECK (id = 1),
				key BLOB NOT NULL
			)`,
		),
	)
}

// tokenTTL is how long a document stays open. The document server saves a
// while after the last editor closed, so the link has to outlive the tab.
const tokenTTL = 24 * time.Hour

// grant is what a link lets the document server do: one file, for one
// user, reading or also writing. The user's rights are checked again on
// every call, so a right taken away also closes the open document.
type grant struct {
	User  uint64 `json:"u"`
	Path  string `json:"p"`
	Write bool   `json:"w"`
	jwt.RegisteredClaims
}

var errBadToken = errors.New("the link is invalid or has expired")

type keys struct {
	db *sql.DB

	once sync.Once
	key  []byte
	err  error
}

func (k *keys) get(ctx context.Context) ([]byte, error) {
	k.once.Do(func() { k.key, k.err = loadKey(ctx, k.db) })
	return k.key, k.err
}

func loadKey(ctx context.Context, db *sql.DB) ([]byte, error) {
	var key []byte
	err := db.QueryRowContext(ctx, `SELECT key FROM office_key WHERE id = 1`).Scan(&key)
	if err == nil {
		return key, nil
	}
	if !errors.Is(err, sql.ErrNoRows) {
		return nil, err
	}
	key = make([]byte, 32)
	if _, err := rand.Read(key); err != nil {
		return nil, err
	}
	if _, err := db.ExecContext(ctx, `INSERT OR IGNORE INTO office_key (id, key) VALUES (1, ?)`, key); err != nil {
		return nil, err
	}
	// Another server process may have won the insert.
	err = db.QueryRowContext(ctx, `SELECT key FROM office_key WHERE id = 1`).Scan(&key)
	return key, err
}

func (k *keys) sign(ctx context.Context, g grant) (string, time.Time, error) {
	key, err := k.get(ctx)
	if err != nil {
		return "", time.Time{}, err
	}
	expires := time.Now().Add(tokenTTL)
	g.ExpiresAt = jwt.NewNumericDate(expires)
	token, err := jwt.NewWithClaims(jwt.SigningMethodHS256, g).SignedString(key)
	return token, expires, err
}

func (k *keys) verify(ctx context.Context, token string) (grant, error) {
	key, err := k.get(ctx)
	if err != nil {
		return grant{}, err
	}
	var g grant
	_, err = jwt.ParseWithClaims(token, &g, func(t *jwt.Token) (any, error) {
		if t.Method != jwt.SigningMethodHS256 {
			return nil, fmt.Errorf("unexpected signing method %v", t.Header["alg"])
		}
		return key, nil
	})
	if err != nil || g.Path == "" || g.User == 0 {
		return grant{}, errBadToken
	}
	return g, nil
}
