package shares

import (
	"context"
	"crypto/rand"
	"database/sql"
	"encoding/hex"
	"errors"
	"math/big"
	"net/http"
	"os"
	"path"
	"regexp"
	"strings"
	"sync"
	"time"
	"unicode"

	"github.com/gtsteffaniak/go-logger/logger"
	"golang.org/x/text/unicode/norm"

	v1 "github.com/junkerderprovinz/cannoncommander/backend/internal/api/v1"
	"github.com/junkerderprovinz/cannoncommander/backend/internal/ccfs"
	"github.com/junkerderprovinz/cannoncommander/backend/internal/database/users"
	"github.com/junkerderprovinz/cannoncommander/backend/internal/fileops"
	"github.com/junkerderprovinz/cannoncommander/backend/internal/fsapi"
	"github.com/junkerderprovinz/cannoncommander/backend/internal/fschange"
	"github.com/junkerderprovinz/cannoncommander/backend/internal/hub"
	"github.com/junkerderprovinz/cannoncommander/backend/internal/state"
	"github.com/junkerderprovinz/cannoncommander/backend/internal/unraid"
)

// Kinds of share links.
const (
	KindFile       = "file"
	KindFolder     = "folder"
	KindCollection = "collection"
)

// States of a link as its owner sees it.
const (
	StateActive    = "active"
	StateExpired   = "expired"
	StateExhausted = "exhausted"
)

const (
	minPassword = 6
	maxPaths    = 1000
)

// slugPattern is the address rule of the share dialog.
var slugPattern = regexp.MustCompile(`^[a-z0-9][a-z0-9-]{2,39}$`)

// maxMessage caps the note a link shows its visitors.
const maxMessage = 2000

// A link that leads nowhere says why in the problem type, so the page for
// visitors can explain it.
var (
	errExpired   = &v1.Problem{Type: "share-expired", Title: "Gone", Status: http.StatusGone, Detail: "this link has expired"}
	errUsedUp    = &v1.Problem{Type: "share-used-up", Title: "Gone", Status: http.StatusGone, Detail: "this link has used up its downloads"}
	errWithdrawn = &v1.Problem{Type: "share-withdrawn", Title: "Gone", Status: http.StatusGone, Detail: "the owner of this link may no longer share"}
)

// Share is a link as its owner and the admins see it.
type Share struct {
	ID   string `json:"id"`
	Slug string `json:"slug"`
	Kind string `json:"kind"`
	// Name is the folder name a collection shows, and the file or folder
	// name otherwise.
	Name string `json:"name"`
	// Message is a note for the visitors, shown above the files.
	Message string   `json:"message"`
	Paths   []string `json:"paths"`
	Owner   string   `json:"owner"`
	// OwnerID is the account behind the link, whose rights it uses.
	OwnerID     uint64     `json:"-"`
	Created     time.Time  `json:"created"`
	Expires     *time.Time `json:"expires"`
	Password    bool       `json:"password"`
	Limit       int        `json:"limit"`
	Downloads   int        `json:"downloads"`
	Views       int        `json:"views"`
	Uploads     bool       `json:"uploads"`
	UploadLimit int64      `json:"uploadLimit"`
	Uploaded    int64      `json:"uploaded"`
	Notify      bool       `json:"notify"`
	State       string     `json:"state"`

	passwordHash string
}

func (s *Share) settle(now time.Time) {
	s.State = StateActive
	switch {
	case s.Expires != nil && !now.Before(*s.Expires):
		s.State = StateExpired
	case s.Limit > 0 && s.Downloads >= s.Limit:
		s.State = StateExhausted
	}
}

// Service keeps the links of one database.
type Service struct {
	db     *sql.DB
	fs     *ccfs.Resolver
	hub    *hub.Hub
	files  *fileops.Service
	key    []byte
	guard  *guard
	opened *opens
	stop   func()
	// uploadMu keeps two uploads from both fitting into the last room of
	// an upload limit.
	uploadMu sync.Mutex
}

// UserByID finds the account a link belongs to. The server answers it from
// the user store, tests set their own.
var UserByID = func(id uint64) (*users.User, error) {
	u, err := state.GetUserByID(id)
	if err != nil {
		return nil, err
	}
	return &u, nil
}

// Notify passes the notice about an opened link on to Unraid. A server
// without the Unraid socket switched on answers with an error, which is
// fine to ignore.
var Notify = func(n unraid.Notice) error { return unraid.Default().Notify(n) }

// For returns the service for the database in d, or nil without a database
// or resolver.
func For(d v1.Deps) *Service {
	if d.DB == nil || d.FS == nil {
		return nil
	}
	s, err := v1.Shared(d, "shares", func() (*Service, error) {
		s := &Service{db: d.DB, fs: d.FS, hub: d.Hub, files: fileops.For(d), guard: newGuard(), opened: newOpens()}
		if err := d.DB.QueryRow(`SELECT key FROM share_secret WHERE id = 1`).Scan(&s.key); err != nil {
			return nil, err
		}
		s.stop = fschange.Observe(fschange.Observer{Moved: s.follow})
		return s, nil
	})
	if err != nil {
		logger.Errorf("shares: %v", err)
		return nil
	}
	return s
}

// Close stops following renames, for a server that shuts down.
func (s *Service) Close() {
	if s.stop != nil {
		s.stop()
	}
}

// follow carries the links along when something they share is renamed or
// moved inside the app.
func (s *Service) follow(from, to string) {
	_, err := s.db.Exec(`UPDATE share_link_paths SET path = ? || substr(path, ?) WHERE path = ? OR substr(path, 1, ?) = ?`,
		to, len(from)+1, from, len(from)+1, from+"/")
	if err != nil {
		logger.Errorf("shares: following %s to %s: %v", from, to, err)
	}
}

// ownerKey is how a user id goes into SQLite, which takes no uint64 above
// the int64 range. Ids are random, so half of them are up there.
func ownerKey(id uint64) int64 { return int64(id) }

// RemoveUser deletes the links of a user, for when the account goes away.
func RemoveUser(ctx context.Context, d v1.Deps, userID uint64) error {
	s := For(d)
	if s == nil {
		return nil
	}
	list, err := s.list(ctx, `WHERE owner_id = ?`, ownerKey(userID))
	if err != nil {
		return err
	}
	for _, sh := range list {
		if err := s.remove(ctx, sh.ID); err != nil {
			return err
		}
	}
	return nil
}

const selectShare = `SELECT id, slug, owner_id, owner, kind, name, message, password_hash, expires_at, download_limit, downloads,
	views, uploads, upload_limit, uploaded, notify, created_at FROM share_links `

func (s *Service) list(ctx context.Context, where string, args ...any) ([]*Share, error) {
	rows, err := s.db.QueryContext(ctx, selectShare+where+` ORDER BY created_at DESC, id`, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []*Share{}
	byID := map[string]*Share{}
	now := time.Now()
	for rows.Next() {
		var sh Share
		var expires sql.NullInt64
		var created, owner int64
		if err := rows.Scan(&sh.ID, &sh.Slug, &owner, &sh.Owner, &sh.Kind, &sh.Name, &sh.Message, &sh.passwordHash, &expires,
			&sh.Limit, &sh.Downloads, &sh.Views, &sh.Uploads, &sh.UploadLimit, &sh.Uploaded, &sh.Notify, &created); err != nil {
			return nil, err
		}
		sh.OwnerID = uint64(owner)
		sh.Created = time.Unix(created, 0).UTC()
		if expires.Valid {
			t := time.Unix(expires.Int64, 0).UTC()
			sh.Expires = &t
		}
		sh.Password = sh.passwordHash != ""
		sh.Paths = []string{}
		sh.settle(now)
		out = append(out, &sh)
		byID[sh.ID] = &sh
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	if len(out) == 0 {
		return out, nil
	}
	paths, err := s.db.QueryContext(ctx, `SELECT share_id, path FROM share_link_paths ORDER BY share_id, seq`)
	if err != nil {
		return nil, err
	}
	defer paths.Close()
	for paths.Next() {
		var id, p string
		if err := paths.Scan(&id, &p); err != nil {
			return nil, err
		}
		if sh := byID[id]; sh != nil {
			sh.Paths = append(sh.Paths, p)
		}
	}
	if err := paths.Err(); err != nil {
		return nil, err
	}
	for _, sh := range out {
		if sh.Kind != KindCollection && len(sh.Paths) > 0 {
			sh.Name = path.Base(sh.Paths[0])
		}
	}
	return out, nil
}

func (s *Service) one(ctx context.Context, where string, args ...any) (*Share, error) {
	list, err := s.list(ctx, where, args...)
	if err != nil {
		return nil, err
	}
	if len(list) == 0 {
		return nil, v1.Coded(http.StatusNotFound, "noLink", "there is no such link")
	}
	return list[0], nil
}

func (s *Service) insert(ctx context.Context, sh *Share) error {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	now := time.Now().Unix()
	if _, err := tx.ExecContext(ctx, `INSERT INTO share_links (id, slug, owner_id, owner, kind, name, message, password_hash, expires_at,
		download_limit, downloads, views, uploads, upload_limit, uploaded, notify, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, 0, 0, ?, ?, 0, ?, ?, ?)`,
		sh.ID, sh.Slug, ownerKey(sh.OwnerID), sh.Owner, sh.Kind, storedName(sh), sh.Message, sh.passwordHash, unixOrNil(sh.Expires),
		sh.Limit, sh.Uploads, sh.UploadLimit, sh.Notify, now, now); err != nil {
		if isUnique(err) {
			return errSlugTaken
		}
		return err
	}
	for i, p := range sh.Paths {
		if _, err := tx.ExecContext(ctx, `INSERT INTO share_link_paths (share_id, seq, path) VALUES (?, ?, ?)`, sh.ID, i, p); err != nil {
			return err
		}
	}
	return tx.Commit()
}

func (s *Service) save(ctx context.Context, sh *Share) error {
	_, err := s.db.ExecContext(ctx, `UPDATE share_links SET slug = ?, name = ?, message = ?, password_hash = ?, expires_at = ?, download_limit = ?,
		uploads = ?, upload_limit = ?, notify = ?, updated_at = ? WHERE id = ?`,
		sh.Slug, storedName(sh), sh.Message, sh.passwordHash, unixOrNil(sh.Expires), sh.Limit, sh.Uploads, sh.UploadLimit, sh.Notify, time.Now().Unix(), sh.ID)
	if isUnique(err) {
		return errSlugTaken
	}
	return err
}

// storedName keeps a name only for collections. A file or folder is called
// what it is called on disk, also after a rename.
func storedName(sh *Share) string {
	if sh.Kind == KindCollection {
		return sh.Name
	}
	return ""
}

func (s *Service) remove(ctx context.Context, id string) error {
	if s.files != nil {
		if err := s.files.CancelShareUploads(ctx, id); err != nil {
			return err
		}
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	if _, err := tx.ExecContext(ctx, `DELETE FROM share_link_paths WHERE share_id = ?`, id); err != nil {
		return err
	}
	if _, err := tx.ExecContext(ctx, `DELETE FROM share_links WHERE id = ?`, id); err != nil {
		return err
	}
	return tx.Commit()
}

// countDownload takes one download off the limit. It fails once the limit
// is used up, also when several downloads race for the last one.
func (s *Service) countDownload(ctx context.Context, id string) error {
	res, err := s.db.ExecContext(ctx, `UPDATE share_links SET downloads = downloads + 1
		WHERE id = ? AND (download_limit = 0 OR downloads < download_limit)`, id)
	if err != nil {
		return err
	}
	if n, err := res.RowsAffected(); err != nil || n == 0 {
		return errUsedUp
	}
	return nil
}

var errSlugTaken = v1.Coded(http.StatusConflict, "slugTaken", "this address is taken already, the dice finds a free one")

func isUnique(err error) bool {
	return err != nil && strings.Contains(err.Error(), "UNIQUE")
}

func unixOrNil(t *time.Time) any {
	if t == nil {
		return nil
	}
	return t.Unix()
}

// slugAlphabet leaves out letters and digits that are easy to mix up when
// a link is read out or typed from paper.
const slugAlphabet = "abcdefghjkmnpqrstuvwxyz23456789"

// freshSlug makes an address like "k3x9qa-urlaub" that is not in use. The
// random part makes it hard to guess, the words help people recognise
// their link.
func (s *Service) freshSlug(ctx context.Context, name string) (string, error) {
	for range 20 {
		id := make([]byte, 6)
		for i := range id {
			n, err := rand.Int(rand.Reader, big.NewInt(int64(len(slugAlphabet))))
			if err != nil {
				return "", err
			}
			id[i] = slugAlphabet[n.Int64()]
		}
		slug := string(id) + "-" + slugWords(name)
		var taken int
		if err := s.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM share_links WHERE slug = ?`, slug).Scan(&taken); err != nil {
			return "", err
		}
		if taken == 0 {
			return slug, nil
		}
	}
	return "", errors.New("no free address found")
}

// slugWords turns a file name into the readable end of an address:
// "Über Uns.pdf" becomes "uber-uns".
func slugWords(name string) string {
	if dot := strings.LastIndex(name, "."); dot > 0 {
		name = name[:dot]
	}
	var b strings.Builder
	dash := false
	for _, r := range norm.NFD.String(strings.ToLower(name)) {
		switch {
		case unicode.Is(unicode.Mn, r):
		case r >= 'a' && r <= 'z' || r >= '0' && r <= '9':
			b.WriteRune(r)
			dash = false
		case !dash && b.Len() > 0:
			b.WriteByte('-')
			dash = true
		}
	}
	out := b.String()
	if len(out) > 18 {
		out = out[:18]
	}
	if out = strings.Trim(out, "-"); out == "" {
		return "link"
	}
	return out
}

func newID() string {
	b := make([]byte, 8)
	_, _ = rand.Read(b)
	return hex.EncodeToString(b)
}

// shownNames are the names the entries of a collection have on the other
// side, where they sit next to each other in one folder.
func (s *Service) shownNames(paths []string) []string {
	names := make([]string, len(paths))
	dirs := make([]bool, len(paths))
	for i, p := range paths {
		names[i] = path.Base(p)
		if loc, err := s.fs.Resolve(p); err == nil && !loc.Virtual() {
			if info, err := os.Stat(loc.Real); err == nil {
				dirs[i] = info.IsDir()
			}
		}
	}
	return fsapi.NumberDuplicates(names, dirs)
}
