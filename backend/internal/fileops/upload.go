package fileops

import (
	"context"
	"crypto/rand"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"net/http"
	"os"
	"path"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/gtsteffaniak/go-logger/logger"

	v1 "github.com/junkerderprovinz/cannoncommander/backend/internal/api/v1"
	"github.com/junkerderprovinz/cannoncommander/backend/internal/ccfs"
	"github.com/junkerderprovinz/cannoncommander/backend/internal/database/migrate"
	"github.com/junkerderprovinz/cannoncommander/backend/internal/database/users"
	"github.com/junkerderprovinz/cannoncommander/backend/internal/fschange"
	"github.com/junkerderprovinz/cannoncommander/backend/internal/jobs"
)

func init() {
	migrate.Register(migrate.Main, migrate.SQL("fileops/001_uploads",
		// One upload per target at a time, so two never write the same
		// part file.
		`CREATE TABLE uploads (
			id TEXT PRIMARY KEY,
			owner TEXT NOT NULL,
			share TEXT NOT NULL,
			path TEXT NOT NULL UNIQUE,
			size INTEGER NOT NULL,
			received INTEGER NOT NULL,
			mtime INTEGER NOT NULL,
			rule TEXT NOT NULL,
			created_at INTEGER NOT NULL,
			updated_at INTEGER NOT NULL
		)`,
		`CREATE INDEX uploads_share ON uploads (share)`,
	))
}

// Upload states.
const (
	UploadOpen    = "open"
	UploadDone    = "done"
	UploadSkipped = "skipped"
)

const (
	// ChunkSize is what clients are asked to send per request. Losing the
	// connection costs at most one chunk.
	ChunkSize = 10 << 20
	maxChunk  = 64 << 20
	// An upload nobody continued for this long is given up together with
	// its part file. Visitors of a share get less time than users.
	userUploadTTL  = 7 * 24 * time.Hour
	shareUploadTTL = 24 * time.Hour
)

var errUploadBusy = v1.Errorf(http.StatusConflict, "another chunk of this upload is still being written")

// UploadRequest starts an upload or picks up the one already running for
// the same target.
type UploadRequest struct {
	// Path is the file to create. Missing folders on the way are created.
	Path  string     `json:"path"`
	Size  int64      `json:"size"`
	MTime *time.Time `json:"mtime,omitempty"`
	// Rule answers a name that exists already: ask, skip, replace, both or
	// newer. With ask the start or the last chunk answers 409, and the
	// client starts the upload again with its decision.
	Rule string `json:"rule,omitempty"`
}

// Upload is the state of a resumable upload.
type Upload struct {
	ID string `json:"id"`
	// Path is the target, and once the upload is done the file it became,
	// which differs from the target when both were kept.
	Path   string     `json:"path"`
	Size   int64      `json:"size"`
	Offset int64      `json:"offset"`
	State  string     `json:"state"`
	Rule   string     `json:"rule"`
	MTime  *time.Time `json:"mtime,omitempty"`
	Chunk  int64      `json:"chunk"`
}

// UploadConflict is returned while the target exists and the rule is ask.
type UploadConflict struct {
	Upload   Upload
	Conflict jobs.Conflict
}

func (c *UploadConflict) Error() string { return "a file of this name exists already" }

type uploadRow struct {
	id, owner, share, path, rule string
	size, received, mtime        int64
}

func (r uploadRow) view() Upload {
	u := Upload{ID: r.id, Path: r.path, Size: r.size, Offset: r.received, State: UploadOpen, Rule: r.rule, Chunk: ChunkSize}
	if r.mtime != 0 {
		t := time.UnixMilli(r.mtime).UTC()
		u.MTime = &t
	}
	return u
}

// busy keeps two requests from writing the same part file at once. Upload
// ids are random, so one map serves every database.
var busy sync.Map

func lockUpload(id string) (unlock func(), ok bool) {
	if _, taken := busy.LoadOrStore(id, struct{}{}); taken {
		return nil, false
	}
	return func() { busy.Delete(id) }, true
}

func partOf(real string) string {
	return filepath.Join(filepath.Dir(real), "."+filepath.Base(real)+PartSuffix)
}

// StartUpload begins an upload for u, or for a share when share is set. A
// second start for the same target and size continues the first one, which
// is how a client resumes after a reload and how it answers a conflict.
func (s *Service) StartUpload(ctx context.Context, u *users.User, share string, req UploadRequest) (Upload, bool, error) {
	if req.Size < 0 {
		return Upload{}, false, v1.Errorf(http.StatusBadRequest, "size cannot be negative")
	}
	if req.Rule == "" {
		req.Rule = jobs.Ask
	}
	if !jobs.ValidRule(req.Rule) {
		return Upload{}, false, v1.Errorf(http.StatusBadRequest, "rule must be ask, skip, replace, both or newer")
	}
	s.expireUploads(ctx)
	loc, err := s.uploadTarget(u, req.Path)
	if err != nil {
		return Upload{}, false, err
	}
	var mtime int64
	if req.MTime != nil {
		mtime = req.MTime.UnixMilli()
	}

	row, err := s.uploadByPath(ctx, loc.Path)
	if err == nil && share != "" {
		// Visitors of a share cannot be told apart, so one never picks up
		// the upload of another. They resume by id, and a second file of
		// the same name gets a name of its own.
		if req.Rule != jobs.Both {
			return Upload{}, false, v1.Errorf(http.StatusConflict, "someone is uploading a file of this name right now")
		}
		if loc, err = s.unclaimed(ctx, loc); err != nil {
			return Upload{}, false, err
		}
		err = sql.ErrNoRows
	}
	switch {
	case err == nil && row.owner == u.Username && row.share == share && row.size == req.Size:
		unlock, ok := lockUpload(row.id)
		if !ok {
			return Upload{}, false, errUploadBusy
		}
		defer unlock()
		row.rule, row.mtime = req.Rule, mtime
		if _, err := s.deps.DB.ExecContext(ctx, `UPDATE uploads SET rule = ?, mtime = ?, updated_at = ? WHERE id = ?`,
			row.rule, row.mtime, time.Now().Unix(), row.id); err != nil {
			return Upload{}, false, err
		}
		if err := s.precheck(u, loc, row); err != nil {
			return Upload{}, false, err
		}
		if row.received == row.size || row.rule == jobs.Skip && exists(loc.Real) {
			up, err := s.finish(ctx, u, row, nil)
			return up, false, err
		}
		return row.view(), false, nil
	case err == nil && row.owner == u.Username && row.share == share:
		// The same name with another size is another file. The old part
		// is of no use for it.
		if err := s.dropUpload(ctx, row); err != nil {
			return Upload{}, false, err
		}
	case err == nil:
		return Upload{}, false, v1.Errorf(http.StatusConflict, "someone else is uploading a file of this name right now")
	case !errors.Is(err, sql.ErrNoRows):
		return Upload{}, false, err
	}

	row = uploadRow{id: newUploadID(), owner: u.Username, share: share, path: loc.Path, rule: req.Rule, size: req.Size, mtime: mtime}
	if err := s.precheck(u, loc, row); err != nil {
		return Upload{}, false, err
	}
	if row.rule == jobs.Skip && exists(loc.Real) {
		out := row.view()
		out.State = UploadSkipped
		return out, false, nil
	}
	if free, ok := freeSpace(filepath.Dir(loc.Real)); ok && free < req.Size {
		return Upload{}, false, v1.Errorf(http.StatusInsufficientStorage, "there is not enough free space for this file")
	}
	part := partOf(loc.Real)
	if _, err := os.Lstat(part); err == nil {
		return Upload{}, false, v1.Errorf(http.StatusConflict, "this file is being written by a copy right now")
	}
	f, err := ccfs.OpenPart(part, os.O_WRONLY|os.O_CREATE|os.O_EXCL)
	if err != nil {
		return Upload{}, false, err
	}
	if err := f.Close(); err != nil {
		return Upload{}, false, err
	}
	now := time.Now().Unix()
	if _, err := s.deps.DB.ExecContext(ctx, `INSERT INTO uploads (id, owner, share, path, size, received, mtime, rule, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, 0, ?, ?, ?, ?)`, row.id, row.owner, row.share, row.path, row.size, row.mtime, row.rule, now, now); err != nil {
		_ = os.Remove(part)
		return Upload{}, false, err
	}
	if row.size == 0 {
		up, err := s.finish(ctx, u, row, nil)
		return up, true, err
	}
	return row.view(), true, nil
}

// Landing lets the caller of UploadChunk take part in the step that gives a
// complete part file its name.
type Landing struct {
	// Allow can still refuse, for a caller whose permission may have ended
	// while the body came in. The part file stays and the upload can go on
	// once it answers yes again.
	Allow func() error
	// Count runs in the transaction that closes the upload, so an upload is
	// always either open or counted.
	Count func(ctx context.Context, tx *sql.Tx, size int64) error
}

// UploadChunk writes body at offset, which has to be where the upload
// stands. The chunk that completes the file gives it its name, with landing
// taking part in that when it is set.
func (s *Service) UploadChunk(ctx context.Context, u *users.User, share, id string, offset int64, body io.Reader, landing *Landing) (Upload, error) {
	unlock, ok := lockUpload(id)
	if !ok {
		return Upload{}, errUploadBusy
	}
	defer unlock()
	row, err := s.uploadFor(ctx, u, share, id)
	if err != nil {
		return Upload{}, err
	}
	loc, err := s.fs.Reach(u, row.path, ccfs.Create)
	if err != nil {
		return Upload{}, err
	}
	part := partOf(loc.Real)
	f, err := ccfs.OpenPart(part, os.O_WRONLY)
	if errors.Is(err, fs.ErrNotExist) {
		_ = s.deleteUpload(ctx, row.id)
		return Upload{}, v1.Errorf(http.StatusGone, "the part file of this upload is gone, please start again")
	}
	if err != nil {
		return Upload{}, err
	}
	defer f.Close()
	if err := s.alignPart(ctx, f, &row); err != nil {
		return Upload{}, err
	}
	if offset != row.received {
		return Upload{}, &offsetMismatch{Upload: row.view()}
	}
	left := row.size - row.received
	if _, err := f.Seek(row.received, io.SeekStart); err != nil {
		return Upload{}, err
	}
	n, copyErr := io.CopyN(f, body, min(left, maxChunk))
	row.received += n
	if copyErr == nil && n == left {
		// The client may only send what the file still needs.
		var extra [1]byte
		if m, _ := body.Read(extra[:]); m > 0 {
			row.received -= n
			_ = f.Truncate(row.received)
			return Upload{}, v1.Errorf(http.StatusRequestEntityTooLarge, "this chunk goes past the end of the file")
		}
	}
	if _, err := s.deps.DB.ExecContext(ctx, `UPDATE uploads SET received = ?, updated_at = ? WHERE id = ?`,
		row.received, time.Now().Unix(), row.id); err != nil {
		return Upload{}, err
	}
	if copyErr != nil && !errors.Is(copyErr, io.EOF) {
		// What arrived stays, the client goes on from the new offset.
		return Upload{}, copyErr
	}
	if row.received < row.size {
		return row.view(), nil
	}
	if err := f.Close(); err != nil {
		return Upload{}, err
	}
	return s.finish(ctx, u, row, landing)
}

// UploadState returns where an upload stands.
func (s *Service) UploadState(ctx context.Context, u *users.User, share, id string) (Upload, error) {
	row, err := s.uploadFor(ctx, u, share, id)
	if err != nil {
		return Upload{}, err
	}
	return row.view(), nil
}

// CancelUpload stops an upload and removes what it wrote.
func (s *Service) CancelUpload(ctx context.Context, u *users.User, share, id string) error {
	unlock, ok := lockUpload(id)
	if !ok {
		return errUploadBusy
	}
	defer unlock()
	row, err := s.uploadFor(ctx, u, share, id)
	if err != nil {
		return err
	}
	return s.dropUpload(ctx, row)
}

// PendingUploads counts the open uploads of a share and the bytes they
// announced, so a share can keep its upload limit before anything lands.
func (s *Service) PendingUploads(ctx context.Context, share string) (n int, bytes int64, err error) {
	err = s.deps.DB.QueryRowContext(ctx, `SELECT COUNT(*), COALESCE(SUM(size), 0) FROM uploads WHERE share = ?`, share).Scan(&n, &bytes)
	return n, bytes, err
}

// CancelShareUploads drops every open upload of a share, for when the
// share is deleted.
func (s *Service) CancelShareUploads(ctx context.Context, share string) error {
	rows, err := s.uploadRows(ctx, `WHERE share = ?`, share)
	if err != nil {
		return err
	}
	for _, row := range rows {
		if err := s.dropUpload(ctx, row); err != nil {
			return err
		}
	}
	return nil
}

// uploadTarget checks that u may create p and makes the folders that lead
// to it, so a dropped folder arrives with its structure.
func (s *Service) uploadTarget(u *users.User, p string) (ccfs.Location, error) {
	loc, err := s.fs.Check(u, p, ccfs.Create)
	if err != nil {
		return loc, err
	}
	if loc.Virtual() || loc.InArchive() || loc.Index == "/" || !ccfs.ValidName(path.Base(loc.Path)) || IsPart(path.Base(loc.Path)) {
		return loc, v1.Errorf(http.StatusBadRequest, "invalid path")
	}
	if err := s.fs.Follows(u, loc, ccfs.Create); err != nil {
		return loc, err
	}
	dir := path.Dir(loc.Path)
	var missing []string
	for {
		parent, err := s.fs.Resolve(dir)
		if err != nil {
			return loc, err
		}
		info, err := os.Stat(parent.Real)
		if err == nil {
			if !info.IsDir() {
				return loc, v1.Errorf(http.StatusConflict, "a file stands where a folder of this path should be")
			}
			break
		}
		if !errors.Is(err, fs.ErrNotExist) || parent.Virtual() || parent.Index == "/" {
			return loc, err
		}
		missing = append(missing, dir)
		dir = path.Dir(dir)
	}
	for i := len(missing) - 1; i >= 0; i-- {
		d, err := s.fs.Check(u, missing[i], ccfs.Create)
		if err != nil {
			return loc, err
		}
		if !ccfs.ValidName(path.Base(d.Path)) {
			return loc, v1.Errorf(http.StatusBadRequest, "invalid path")
		}
		if err := os.Mkdir(d.Real, 0o777); err != nil && !errors.Is(err, fs.ErrExist) {
			return loc, err
		}
		fschange.Created(d.Path)
	}
	return loc, nil
}

// precheck answers the rule ask before any byte is sent, and refuses a
// replacement the user may not make.
func (s *Service) precheck(u *users.User, loc ccfs.Location, row uploadRow) error {
	target, err := os.Lstat(loc.Real)
	if errors.Is(err, fs.ErrNotExist) {
		return nil
	}
	if err != nil {
		return err
	}
	switch row.rule {
	case jobs.Ask:
		return s.conflict(loc, row, target)
	case jobs.Replace, jobs.Newer:
		if target.IsDir() {
			return v1.Errorf(http.StatusConflict, "a folder of this name is in the way")
		}
		if _, err := s.fs.Check(u, loc.Path, ccfs.Modify); err != nil {
			return err
		}
	}
	return nil
}

func (s *Service) conflict(loc ccfs.Location, row uploadRow, target fs.FileInfo) error {
	up := row.view()
	src := jobs.Side{Path: loc.Path, Size: row.size}
	if up.MTime != nil {
		src.MTime = *up.MTime
	}
	return &UploadConflict{Upload: up, Conflict: jobs.NewConflict(path.Base(loc.Path), src, side(node{loc.Path, loc.Real}, target))}
}

// finish gives a complete part file its name, by the rule of the upload.
func (s *Service) finish(ctx context.Context, u *users.User, row uploadRow, landing *Landing) (Upload, error) {
	if landing != nil {
		if err := landing.Allow(); err != nil {
			return Upload{}, err
		}
	}
	loc, err := s.fs.Check(u, row.path, ccfs.Create)
	if err != nil {
		return Upload{}, err
	}
	if err := s.fs.Follows(u, loc, ccfs.Create); err != nil {
		return Upload{}, err
	}
	part := partOf(loc.Real)
	out := row.view()
	final := loc
	if target, err := os.Lstat(loc.Real); err == nil {
		choice := row.rule
		if choice == jobs.Newer {
			choice = jobs.Skip
			if out.MTime != nil && out.MTime.Sub(target.ModTime()) > time.Second {
				choice = jobs.Replace
			}
		}
		switch choice {
		case jobs.Ask:
			return Upload{}, s.conflict(loc, row, target)
		case jobs.Skip:
			if err := s.dropUpload(ctx, row); err != nil {
				return Upload{}, err
			}
			out.State = UploadSkipped
			return out, nil
		case jobs.Both:
			name := ccfs.FreeName(filepath.Dir(loc.Real), filepath.Base(loc.Real))
			if final, err = s.fs.Resolve(path.Join(path.Dir(loc.Path), name)); err != nil {
				return Upload{}, err
			}
		case jobs.Replace:
			if target.IsDir() {
				return Upload{}, v1.Errorf(http.StatusConflict, "a folder of this name is in the way")
			}
			if _, err := s.fs.Check(u, loc.Path, ccfs.Modify); err != nil {
				return Upload{}, err
			}
			s.retire(ctx, u, loc)
		}
	} else if !errors.Is(err, fs.ErrNotExist) {
		return Upload{}, err
	}
	if out.MTime != nil {
		if err := os.Chtimes(part, *out.MTime, *out.MTime); err != nil {
			return Upload{}, err
		}
	}
	if err := s.land(ctx, part, final.Real, row, landing); err != nil {
		return Upload{}, err
	}
	fschange.Created(final.Path)
	out.Path, out.Offset, out.State = final.Path, row.size, UploadDone
	return out, nil
}

// land names the part file and closes the upload as one step. When the
// database refuses, the file goes back to being a part and the client can
// send the last request again.
func (s *Service) land(ctx context.Context, part, final string, row uploadRow, landing *Landing) error {
	tx, err := s.deps.DB.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	if _, err := tx.ExecContext(ctx, `DELETE FROM uploads WHERE id = ?`, row.id); err != nil {
		return err
	}
	if landing != nil {
		if err := landing.Count(ctx, tx, row.size); err != nil {
			return err
		}
	}
	if err := os.Rename(part, final); err != nil {
		return err
	}
	if err := tx.Commit(); err != nil {
		if rerr := os.Rename(final, part); rerr != nil {
			logger.Errorf("upload %s: %v", row.id, rerr)
		}
		return err
	}
	return nil
}

// retire moves a file that an upload replaces to the trash, so it can be
// brought back. Without a trash the rename replaces it in one step.
func (s *Service) retire(ctx context.Context, u *users.User, loc ccfs.Location) {
	if s.trash == nil {
		return
	}
	if _, err := s.trash.Put(ctx, u.Username, loc); err != nil {
		logger.Errorf("upload: keeping the replaced %s in the trash: %v", loc.Path, err)
	}
}

// alignPart trusts the part file over the database: after a crash it may
// hold less than recorded, or the bytes of a chunk whose count was never
// saved.
func (s *Service) alignPart(ctx context.Context, f *os.File, row *uploadRow) error {
	info, err := f.Stat()
	if err != nil {
		return err
	}
	switch {
	case info.Size() > row.received:
		return f.Truncate(row.received)
	case info.Size() < row.received:
		row.received = info.Size()
		_, err := s.deps.DB.ExecContext(ctx, `UPDATE uploads SET received = ? WHERE id = ?`, row.received, row.id)
		return err
	}
	return nil
}

func (s *Service) uploadFor(ctx context.Context, u *users.User, share, id string) (uploadRow, error) {
	rows, err := s.uploadRows(ctx, `WHERE id = ?`, id)
	if err != nil {
		return uploadRow{}, err
	}
	if len(rows) == 0 || rows[0].owner != u.Username || rows[0].share != share {
		return uploadRow{}, v1.Errorf(http.StatusNotFound, "no such upload")
	}
	return rows[0], nil
}

func (s *Service) uploadByPath(ctx context.Context, p string) (uploadRow, error) {
	rows, err := s.uploadRows(ctx, `WHERE path = ?`, p)
	if err != nil {
		return uploadRow{}, err
	}
	if len(rows) == 0 {
		return uploadRow{}, sql.ErrNoRows
	}
	return rows[0], nil
}

func (s *Service) uploadRows(ctx context.Context, where string, args ...any) ([]uploadRow, error) {
	rows, err := s.deps.DB.QueryContext(ctx, `SELECT id, owner, share, path, size, received, mtime, rule FROM uploads `+where, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []uploadRow
	for rows.Next() {
		var r uploadRow
		if err := rows.Scan(&r.id, &r.owner, &r.share, &r.path, &r.size, &r.received, &r.mtime, &r.rule); err != nil {
			return nil, err
		}
		out = append(out, r)
	}
	return out, rows.Err()
}

func (s *Service) dropUpload(ctx context.Context, row uploadRow) error {
	if loc, err := s.fs.Resolve(row.path); err == nil && !loc.Virtual() {
		if err := os.Remove(partOf(loc.Real)); err != nil && !errors.Is(err, fs.ErrNotExist) {
			return err
		}
	}
	return s.deleteUpload(ctx, row.id)
}

func (s *Service) deleteUpload(ctx context.Context, id string) error {
	_, err := s.deps.DB.ExecContext(ctx, `DELETE FROM uploads WHERE id = ?`, id)
	return err
}

// expireUploads gives up the uploads nobody continued. It runs when an
// upload starts, which is often enough to keep part files from piling up.
func (s *Service) expireUploads(ctx context.Context) {
	now := time.Now()
	rows, err := s.uploadRows(ctx, `WHERE (share = '' AND updated_at < ?) OR (share != '' AND updated_at < ?)`,
		now.Add(-userUploadTTL).Unix(), now.Add(-shareUploadTTL).Unix())
	if err != nil {
		logger.Errorf("uploads: %v", err)
		return
	}
	for _, row := range rows {
		unlock, ok := lockUpload(row.id)
		if !ok {
			continue
		}
		if err := s.dropUpload(ctx, row); err != nil {
			logger.Errorf("upload %s: %v", row.id, err)
		}
		unlock()
	}
}

// unclaimed returns the first "name (n).ext" next to loc that neither
// exists nor is the target of another upload.
func (s *Service) unclaimed(ctx context.Context, loc ccfs.Location) (ccfs.Location, error) {
	dir, name := path.Dir(loc.Path), path.Base(loc.Path)
	base, ext := name, ""
	if dot := strings.LastIndex(name, "."); dot > 0 {
		base, ext = name[:dot], name[dot:]
	}
	for n := 1; ; n++ {
		next, err := s.fs.Resolve(path.Join(dir, base+" ("+strconv.Itoa(n)+")"+ext))
		if err != nil {
			return loc, err
		}
		if exists(next.Real) {
			continue
		}
		if _, err := s.uploadByPath(ctx, next.Path); errors.Is(err, sql.ErrNoRows) {
			return next, nil
		} else if err != nil {
			return loc, err
		}
	}
}

func exists(p string) bool {
	_, err := os.Lstat(p)
	return err == nil
}

func newUploadID() string {
	b := make([]byte, 12)
	_, _ = rand.Read(b)
	return hex.EncodeToString(b)
}

type offsetMismatch struct{ Upload Upload }

func (e *offsetMismatch) Error() string {
	return fmt.Sprintf("the upload stands at %d bytes", e.Upload.Offset)
}

// ChunkOffset reads where a chunk goes from Upload-Offset, or from the
// start of Content-Range.
func ChunkOffset(r *http.Request) (int64, error) {
	if v := r.Header.Get("Upload-Offset"); v != "" {
		n, err := strconv.ParseInt(v, 10, 64)
		if err != nil || n < 0 {
			return 0, v1.Errorf(http.StatusBadRequest, "Upload-Offset must be a byte count")
		}
		return n, nil
	}
	if v := r.Header.Get("Content-Range"); v != "" {
		var start, end, total int64
		if _, err := fmt.Sscanf(v, "bytes %d-%d/%d", &start, &end, &total); err == nil && start >= 0 {
			return start, nil
		}
		return 0, v1.Errorf(http.StatusBadRequest, "Content-Range must look like bytes 0-99/1000")
	}
	return 0, v1.Errorf(http.StatusBadRequest, "say where the chunk goes with Upload-Offset")
}

// WriteUpload answers with the state of an upload and puts its offset in a
// header as well, where resumable upload clients look for it.
func WriteUpload(w http.ResponseWriter, r *http.Request, status int, up Upload) error {
	w.Header().Set("Upload-Offset", strconv.FormatInt(up.Offset, 10))
	return v1.WriteJSON(w, r, status, up)
}

// WriteUploadError answers the two errors of an upload that carry more
// than a problem: the conflict to decide and the offset to go on from. It
// reports false for any other error. view, when set, changes how the
// upload is shown, as a share does to hide where its folder lies.
func WriteUploadError(w http.ResponseWriter, r *http.Request, err error, view func(Upload) Upload) bool {
	var c *UploadConflict
	var m *offsetMismatch
	var body map[string]any
	if view == nil {
		view = func(up Upload) Upload { return up }
	}
	switch {
	case errors.As(err, &c):
		body = map[string]any{"upload": view(c.Upload), "conflict": c.Conflict}
	case errors.As(err, &m):
		w.Header().Set("Upload-Offset", strconv.FormatInt(m.Upload.Offset, 10))
		body = map[string]any{"upload": view(m.Upload)}
	default:
		return false
	}
	body["type"] = "about:blank"
	body["title"] = http.StatusText(http.StatusConflict)
	body["status"] = http.StatusConflict
	body["detail"] = err.Error()
	body["instance"] = r.URL.Path
	w.Header().Set("Content-Type", "application/problem+json")
	w.Header().Set("Cache-Control", "no-store")
	w.WriteHeader(http.StatusConflict)
	_ = json.NewEncoder(w).Encode(body)
	return true
}

func (s *Service) uploadCreate(w http.ResponseWriter, r *http.Request) error {
	var req UploadRequest
	if err := v1.DecodeJSON(r, &req); err != nil {
		return err
	}
	up, created, err := s.StartUpload(r.Context(), v1.UserFrom(r), "", req)
	if WriteUploadError(w, r, err, nil) {
		return nil
	}
	if err != nil {
		return err
	}
	status := http.StatusOK
	if created && up.State == UploadOpen {
		status = http.StatusCreated
	}
	return WriteUpload(w, r, status, up)
}

func (s *Service) uploadGet(w http.ResponseWriter, r *http.Request) error {
	up, err := s.UploadState(r.Context(), v1.UserFrom(r), "", r.PathValue("id"))
	if err != nil {
		return err
	}
	return WriteUpload(w, r, http.StatusOK, up)
}

func (s *Service) uploadChunk(w http.ResponseWriter, r *http.Request) error {
	offset, err := ChunkOffset(r)
	if err != nil {
		return err
	}
	up, err := s.UploadChunk(r.Context(), v1.UserFrom(r), "", r.PathValue("id"), offset, r.Body, nil)
	if WriteUploadError(w, r, err, nil) {
		return nil
	}
	if err != nil {
		return err
	}
	return WriteUpload(w, r, http.StatusOK, up)
}

func (s *Service) uploadCancel(w http.ResponseWriter, r *http.Request) error {
	if err := s.CancelUpload(r.Context(), v1.UserFrom(r), "", r.PathValue("id")); err != nil {
		return err
	}
	w.WriteHeader(http.StatusNoContent)
	return nil
}
