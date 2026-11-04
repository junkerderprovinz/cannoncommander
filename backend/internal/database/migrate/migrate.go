// Package migrate keeps a registry of named schema migrations. Every package
// registers its own migrations, and the names already applied are stored in
// cc_migrations, so packages never fight over one version number.
package migrate

import (
	"context"
	"database/sql"
	"fmt"
	"slices"
	"strings"
	"sync"
	"time"
)

// Target names the database a migration belongs to.
type Target string

const (
	// Main is the database with users, shares, jobs and settings.
	Main Target = "main"
	// Index is the per source index database, which can be rebuilt any time.
	Index Target = "index"
)

// Migration is one schema change. Name must be unique per target and is
// written as "<package>/<NNN>_<what>", for example "jobs/001_create". Up runs
// inside a transaction together with the bookkeeping row.
type Migration struct {
	Name string
	Up   func(ctx context.Context, tx *sql.Tx) error
}

// SQL returns a migration that runs the given statements in order.
func SQL(name string, statements ...string) Migration {
	return Migration{Name: name, Up: func(ctx context.Context, tx *sql.Tx) error {
		for _, s := range statements {
			if _, err := tx.ExecContext(ctx, s); err != nil {
				return err
			}
		}
		return nil
	}}
}

var (
	mu       sync.Mutex
	registry = map[Target][]Migration{}
)

// Register adds migrations for target. It is meant for init functions and
// panics on a bad or duplicate name, because both are programming errors.
func Register(target Target, ms ...Migration) {
	mu.Lock()
	defer mu.Unlock()
	for _, m := range ms {
		pkg, rest, ok := strings.Cut(m.Name, "/")
		if !ok || pkg == "" || rest == "" || m.Up == nil {
			panic(fmt.Sprintf("migrate: bad migration %q", m.Name))
		}
		if slices.ContainsFunc(registry[target], func(o Migration) bool { return o.Name == m.Name }) {
			panic(fmt.Sprintf("migrate: %q registered twice", m.Name))
		}
		registry[target] = append(registry[target], m)
	}
}

// Registered returns the migrations for target sorted by name, which is
// also the order Apply runs them in.
func Registered(target Target) []Migration {
	mu.Lock()
	defer mu.Unlock()
	ms := slices.Clone(registry[target])
	slices.SortFunc(ms, func(a, b Migration) int { return strings.Compare(a.Name, b.Name) })
	return ms
}

// Apply runs every registered migration for target that db has not seen
// yet and returns the names it applied.
func Apply(ctx context.Context, db *sql.DB, target Target) ([]string, error) {
	return ApplyList(ctx, db, Registered(target))
}

// ApplyList is Apply for an explicit list, used by tests.
func ApplyList(ctx context.Context, db *sql.DB, ms []Migration) ([]string, error) {
	if _, err := db.ExecContext(ctx, `CREATE TABLE IF NOT EXISTS cc_migrations (
		name TEXT PRIMARY KEY,
		applied_at INTEGER NOT NULL
	)`); err != nil {
		return nil, fmt.Errorf("create cc_migrations: %w", err)
	}
	done, err := appliedNames(ctx, db)
	if err != nil {
		return nil, err
	}
	var applied []string
	for _, m := range ms {
		if _, ok := done[m.Name]; ok {
			continue
		}
		if err := applyOne(ctx, db, m); err != nil {
			return applied, fmt.Errorf("migration %s: %w", m.Name, err)
		}
		applied = append(applied, m.Name)
	}
	return applied, nil
}

func appliedNames(ctx context.Context, db *sql.DB) (map[string]struct{}, error) {
	rows, err := db.QueryContext(ctx, `SELECT name FROM cc_migrations`)
	if err != nil {
		return nil, fmt.Errorf("read cc_migrations: %w", err)
	}
	defer rows.Close()
	done := map[string]struct{}{}
	for rows.Next() {
		var name string
		if err := rows.Scan(&name); err != nil {
			return nil, err
		}
		done[name] = struct{}{}
	}
	return done, rows.Err()
}

func applyOne(ctx context.Context, db *sql.DB, m Migration) error {
	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	if err := m.Up(ctx, tx); err != nil {
		return err
	}
	if _, err := tx.ExecContext(ctx, `INSERT INTO cc_migrations (name, applied_at) VALUES (?, ?)`, m.Name, time.Now().Unix()); err != nil {
		return err
	}
	return tx.Commit()
}
