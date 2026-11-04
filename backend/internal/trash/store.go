package trash

import (
	"context"
	"crypto/rand"
	"database/sql"
	"errors"
	"io/fs"
	"net/http"
	"os"
	"path"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/gtsteffaniak/go-logger/logger"

	v1 "github.com/junkerderprovinz/cannoncommander/backend/internal/api/v1"
	"github.com/junkerderprovinz/cannoncommander/backend/internal/ccfs"
	"github.com/junkerderprovinz/cannoncommander/backend/internal/database/migrate"
	"github.com/junkerderprovinz/cannoncommander/backend/internal/fschange"
	"github.com/junkerderprovinz/cannoncommander/backend/internal/hub"
	"github.com/junkerderprovinz/cannoncommander/backend/internal/jobs"
	"github.com/junkerderprovinz/cannoncommander/backend/internal/labels"
)

// DirName is the bin folder at the top of every share or volume. Moving
// into it is a rename on the same disk, so nothing gets copied.
const DirName = ".cc-trash"

// DefaultDays is how long entries stay before they are deleted for good.
const DefaultDays = 30

const (
	settingDays = "trashDays"
	expireEvery = time.Hour
	tellEvery   = time.Second
	// idChunk stays below what SQLite allows for bound values in one query.
	idChunk = 500
)

func init() {
	migrate.Register(migrate.Main, migrate.SQL("trash/001_create",
		`CREATE TABLE trash_items (
			id TEXT PRIMARY KEY,
			owner TEXT NOT NULL,
			name TEXT NOT NULL,
			origin TEXT NOT NULL,
			bin TEXT NOT NULL,
			dir INTEGER NOT NULL,
			size INTEGER NOT NULL,
			files INTEGER NOT NULL,
			deleted_at INTEGER NOT NULL
		)`,
		`CREATE INDEX trash_items_owner ON trash_items (owner)`,
	))
}

var (
	ErrShareRoot = v1.Errorf(http.StatusBadRequest, "a whole share or volume cannot go to the trash")
	ErrInBin     = v1.Errorf(http.StatusBadRequest, "this is in the trash already")
	ErrCrossFS   = v1.Errorf(http.StatusConflict, "this lies on another file system than its trash, delete it instead")
	ErrNotFound  = v1.Errorf(http.StatusNotFound, "no such entry in the trash")
)

// Item is one entry in the trash.
type Item struct {
	ID        string    `json:"id"`
	Name      string    `json:"name"`
	Origin    string    `json:"origin"`
	From      string    `json:"from"`
	Dir       bool      `json:"dir"`
	Size      int64     `json:"size"`
	Files     int64     `json:"files"`
	By        string    `json:"by"`
	DeletedAt time.Time `json:"deletedAt"`
	ExpiresAt time.Time `json:"expiresAt"`
	// Bin is the API path of the entry inside the trash.
	Bin string `json:"-"`
}

// Store keeps the trash of every share and volume.
type Store struct {
	deps v1.Deps
	db   *sql.DB
	hub  *hub.Hub
	fs   *ccfs.Resolver

	ctx  context.Context
	stop context.CancelFunc
	done chan struct{}

	// mu keeps two restores from picking the same free name.
	mu sync.Mutex

	// tellMu guards holding and tellAgain, see changed.
	tellMu    sync.Mutex
	holding   bool
	tellAgain bool
}

// For returns the trash for the database in d and starts its clean up,
// or nil without a database or resolver.
func For(d v1.Deps) *Store {
	if d.DB == nil || d.FS == nil {
		return nil
	}
	s, _ := v1.Shared(d, "trash", func() (*Store, error) { return New(d), nil })
	return s
}

// New returns a store that deletes expired entries once an hour.
func New(d v1.Deps) *Store {
	s := &Store{deps: d, db: d.DB, hub: d.Hub, fs: d.FS, done: make(chan struct{})}
	s.ctx, s.stop = context.WithCancel(context.Background())
	go s.expireLoop()
	return s
}

// Close stops the clean up.
func (s *Store) Close() {
	s.stop()
	<-s.done
}

// BinFor returns the API path of the bin folder that takes loc. On a
// source mounted right below /mnt, such as /mnt/user or /mnt/disk1, every
// share gets its own, elsewhere the source has one at its top.
func BinFor(loc ccfs.Location) (string, error) {
	if loc.Virtual() || loc.InArchive() {
		return "", ErrShareRoot
	}
	if InBin(loc.Path) {
		return "", ErrInBin
	}
	src := loc.Source
	if path.Dir(src.Mount) == "/mnt" {
		share, rest, _ := strings.Cut(strings.TrimPrefix(loc.Index, "/"), "/")
		if share == "" || rest == "" {
			return "", ErrShareRoot
		}
		return path.Join(src.Mount, share, DirName), nil
	}
	if loc.Index == "/" {
		return "", ErrShareRoot
	}
	return path.Join(src.Mount, DirName), nil
}

// InBin reports whether an API path lies in a bin folder.
func InBin(p string) bool {
	return strings.Contains(p+"/", "/"+DirName+"/")
}

// Put moves the entry at loc into its bin and records it for owner. It
// counts the entry first, which can take a moment for a big folder.
func (s *Store) Put(ctx context.Context, owner string, loc ccfs.Location) (Item, error) {
	bin, err := BinFor(loc)
	if err != nil {
		return Item{}, err
	}
	info, err := os.Lstat(loc.Real)
	if err != nil {
		return Item{}, err
	}
	size, files, err := jobs.TreeSize(ctx, loc.Real)
	if err != nil {
		return Item{}, err
	}
	it := Item{
		ID:        strings.ToLower(rand.Text()[:16]),
		Name:      path.Base(loc.Path),
		Origin:    loc.Path,
		From:      path.Dir(loc.Path),
		Dir:       info.IsDir(),
		Size:      size,
		Files:     files,
		By:        owner,
		DeletedAt: time.Now(),
	}
	holder := path.Join(bin, it.ID)
	it.Bin = path.Join(holder, it.Name)
	holderLoc, err := s.fs.Resolve(holder)
	if err != nil {
		return Item{}, err
	}
	if err := os.MkdirAll(holderLoc.Real, 0o777); err != nil {
		return Item{}, err
	}
	if err := os.Rename(loc.Real, filepath.Join(holderLoc.Real, it.Name)); err != nil {
		_ = os.Remove(holderLoc.Real)
		if ccfs.CrossDevice(err) {
			return Item{}, ErrCrossFS
		}
		return Item{}, err
	}
	if _, err := s.db.Exec(`INSERT INTO trash_items (id, owner, name, origin, bin, dir, size, files, deleted_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)`, it.ID, owner, it.Name, it.Origin, it.Bin, it.Dir, it.Size, it.Files,
		it.DeletedAt.UnixMilli()); err != nil {
		// Without its row the entry could never be found again.
		_ = os.Rename(filepath.Join(holderLoc.Real, it.Name), loc.Real)
		_ = os.Remove(holderLoc.Real)
		return Item{}, err
	}
	it.ExpiresAt = it.DeletedAt.Add(time.Duration(s.Days()) * 24 * time.Hour)
	fschange.Moved(it.Origin, it.Bin)
	s.follow(labels.Moved(ctx, s.deps, labels.Move{From: it.Origin, To: it.Bin}))
	s.changed()
	return it, nil
}

// Restore moves an entry back to where it came from and returns where it
// landed. Missing folders on the way are created again, and a name that is
// taken meanwhile gets a number, as in the prototype.
func (s *Store) Restore(id string) (string, error) {
	it, err := s.Get(id)
	if err != nil {
		return "", err
	}
	from, err := s.fs.Resolve(it.Bin)
	if err != nil {
		return "", err
	}
	parent, err := s.fs.Resolve(it.From)
	if err != nil {
		return "", err
	}
	if parent.Virtual() || parent.InArchive() {
		return "", ccfs.ErrBadPath
	}
	if err := ccfs.Contained(parent); err != nil {
		return "", err
	}
	if err := os.MkdirAll(parent.Real, 0o777); err != nil {
		return "", err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	name := ccfs.FreeName(parent.Real, it.Name)
	if err := os.Rename(from.Real, filepath.Join(parent.Real, name)); err != nil {
		if ccfs.CrossDevice(err) {
			return "", ErrCrossFS
		}
		return "", err
	}
	_ = os.Remove(filepath.Dir(from.Real))
	s.forget(it.ID)
	to := path.Join(it.From, name)
	fschange.Moved(it.Bin, to)
	s.follow(labels.Moved(context.Background(), s.deps, labels.Move{From: it.Bin, To: to}))
	s.changed()
	return to, nil
}

// Drop deletes an entry for good. With a run it goes file by file, so the
// job can count and be paused.
func (s *Store) Drop(ctx context.Context, run *jobs.Run, id string) error {
	it, err := s.Get(id)
	if err != nil {
		return err
	}
	loc, err := s.fs.Resolve(path.Dir(it.Bin))
	if err != nil {
		return err
	}
	if run != nil {
		err = jobs.RemoveTree(ctx, run, loc.Real)
	} else {
		err = os.RemoveAll(loc.Real)
	}
	if err != nil {
		return err
	}
	s.forget(it.ID)
	fschange.Removed(it.Bin)
	s.follow(labels.Removed(ctx, s.deps, it.Bin))
	s.changed()
	return nil
}

// Get returns one entry.
func (s *Store) Get(id string) (Item, error) {
	items, err := s.query(`WHERE id = ?`, id)
	if err != nil {
		return Item{}, err
	}
	if len(items) == 0 {
		return Item{}, ErrNotFound
	}
	return items[0], nil
}

// getAll returns the entries with the given ids, as far as they exist.
func (s *Store) getAll(ids []string) (map[string]Item, error) {
	out := make(map[string]Item, len(ids))
	for chunk := range slices.Chunk(ids, idChunk) {
		args := make([]any, len(chunk))
		for i, id := range chunk {
			args[i] = id
		}
		items, err := s.query(`WHERE id IN (?`+strings.Repeat(`, ?`, len(chunk)-1)+`)`, args...)
		if err != nil {
			return nil, err
		}
		for _, it := range items {
			out[it.ID] = it
		}
	}
	return out, nil
}

// List returns the entries of owner, or of everybody when owner is empty,
// newest first. Entries whose files vanished from the bin are dropped.
func (s *Store) List(owner string) ([]Item, error) {
	where, args := `WHERE 1 = 1`, []any{}
	if owner != "" {
		where, args = `WHERE owner = ?`, []any{owner}
	}
	items, err := s.query(where+` ORDER BY deleted_at DESC`, args...)
	if err != nil {
		return nil, err
	}
	out := items[:0]
	for _, it := range items {
		if loc, err := s.fs.Resolve(it.Bin); err == nil {
			if _, err := os.Lstat(loc.Real); errors.Is(err, fs.ErrNotExist) {
				s.forget(it.ID)
				continue
			}
		}
		out = append(out, it)
	}
	return out, nil
}

func (s *Store) query(tail string, args ...any) ([]Item, error) {
	rows, err := s.db.Query(`SELECT id, owner, name, origin, bin, dir, size, files, deleted_at FROM trash_items `+tail, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	keep := time.Duration(s.Days()) * 24 * time.Hour
	items := []Item{}
	for rows.Next() {
		var it Item
		var at int64
		if err := rows.Scan(&it.ID, &it.By, &it.Name, &it.Origin, &it.Bin, &it.Dir, &it.Size, &it.Files, &at); err != nil {
			return nil, err
		}
		it.From = path.Dir(it.Origin)
		it.DeletedAt = time.UnixMilli(at)
		it.ExpiresAt = it.DeletedAt.Add(keep)
		items = append(items, it)
	}
	return items, rows.Err()
}

func (s *Store) forget(id string) {
	if _, err := s.db.Exec(`DELETE FROM trash_items WHERE id = ?`, id); err != nil {
		logger.Errorf("trash: %v", err)
	}
}

// Disown takes the entries of an account that is being deleted away from its
// login name, which a new account could get. Admins still see them and the
// clean up deletes them when their days are over.
func (s *Store) Disown(owner string) error {
	return s.Rename(owner, "")
}

// Rename moves the entries of a user to a new login name.
func (s *Store) Rename(from, to string) error {
	if _, err := s.db.Exec(`UPDATE trash_items SET owner = ? WHERE owner = ?`, to, from); err != nil {
		return err
	}
	s.changed()
	return nil
}

// Days returns how long entries stay.
func (s *Store) Days() int {
	n, err := strconv.Atoi(jobs.Setting(s.db, settingDays, ""))
	if err != nil || n < 1 {
		return DefaultDays
	}
	return n
}

// SetDays changes how long entries stay and deletes the ones past the new
// limit right away.
func (s *Store) SetDays(n int) error {
	if n < 1 || n > 3650 {
		return v1.Errorf(http.StatusBadRequest, "days must be between 1 and 3650")
	}
	if err := jobs.PutSetting(s.db, settingDays, strconv.Itoa(n)); err != nil {
		return err
	}
	s.expire()
	return nil
}

func (s *Store) expireLoop() {
	defer close(s.done)
	s.expire()
	t := time.NewTicker(expireEvery)
	defer t.Stop()
	for {
		select {
		case <-s.ctx.Done():
			return
		case <-t.C:
			s.expire()
		}
	}
}

// expire deletes the entries that stayed their days.
func (s *Store) expire() {
	cutoff := time.Now().Add(-time.Duration(s.Days()) * 24 * time.Hour).UnixMilli()
	items, err := s.query(`WHERE deleted_at < ?`, cutoff)
	if err != nil {
		logger.Errorf("trash: %v", err)
		return
	}
	for _, it := range items {
		if s.ctx.Err() != nil {
			return
		}
		if err := s.Drop(s.ctx, nil, it.ID); err != nil && !errors.Is(err, ErrNotFound) {
			logger.Errorf("trash: removing %s: %v", it.Bin, err)
		}
	}
}

// follow logs a failure to carry labels along. The entry itself is where
// it should be.
func (s *Store) follow(err error) {
	if err != nil {
		logger.Errorf("labels: %v", err)
	}
}

// changed tells every client to reload its trash, at most once per tellEvery
// because each of them reads the whole list again and a job can put thousands
// of entries in a row. The event carries no names, because the trash of one
// user is none of another's business.
func (s *Store) changed() {
	if s.hub == nil {
		return
	}
	s.tellMu.Lock()
	defer s.tellMu.Unlock()
	if s.holding {
		s.tellAgain = true
		return
	}
	s.holding = true
	s.tell()
	time.AfterFunc(tellEvery, s.release)
}

// release ends the wait after a change and tells what happened during it.
func (s *Store) release() {
	s.tellMu.Lock()
	defer s.tellMu.Unlock()
	if !s.tellAgain {
		s.holding = false
		return
	}
	s.tellAgain = false
	s.tell()
	time.AfterFunc(tellEvery, s.release)
}

func (s *Store) tell() {
	if _, err := s.hub.PublishJSON(EventChanged, "", struct{}{}); err != nil {
		logger.Errorf("trash: %v", err)
	}
}
