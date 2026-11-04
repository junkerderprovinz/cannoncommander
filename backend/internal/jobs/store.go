package jobs

import (
	"bytes"
	"context"
	"database/sql"
	"encoding/json"
	"strconv"
	"time"

	"github.com/junkerderprovinz/cannoncommander/backend/internal/database/migrate"
)

func init() {
	migrate.Register(migrate.Main, migrate.SQL("jobs/001_create",
		`CREATE TABLE jobs (
			id TEXT PRIMARY KEY,
			kind TEXT NOT NULL,
			owner TEXT NOT NULL,
			state TEXT NOT NULL,
			position INTEGER NOT NULL,
			name TEXT NOT NULL,
			dest TEXT NOT NULL,
			params TEXT NOT NULL,
			answers TEXT NOT NULL,
			cap INTEGER NOT NULL,
			wait_parity INTEGER NOT NULL,
			done_bytes INTEGER NOT NULL,
			done_files INTEGER NOT NULL,
			current TEXT NOT NULL,
			error TEXT NOT NULL,
			undo TEXT NOT NULL,
			result TEXT NOT NULL,
			created_at INTEGER NOT NULL,
			started_at INTEGER NOT NULL,
			finished_at INTEGER NOT NULL
		)`,
		`CREATE INDEX jobs_owner ON jobs (owner)`,
		`CREATE TABLE job_items (
			job_id TEXT NOT NULL,
			seq INTEGER NOT NULL,
			path TEXT NOT NULL,
			name TEXT NOT NULL,
			ref TEXT NOT NULL,
			state TEXT NOT NULL,
			resumed INTEGER NOT NULL,
			bytes INTEGER NOT NULL,
			files INTEGER NOT NULL,
			error TEXT NOT NULL,
			PRIMARY KEY (job_id, seq)
		)`,
		`CREATE TABLE job_settings (
			key TEXT PRIMARY KEY,
			value TEXT NOT NULL
		)`,
	))
}

// answers are the decisions a job collected, before and while it ran.
type answers struct {
	Paths map[string]string `json:"paths,omitempty"`
	All   string            `json:"all,omitempty"`
}

func strictUnmarshal(raw []byte, v any) error {
	dec := json.NewDecoder(bytes.NewReader(raw))
	dec.DisallowUnknownFields()
	return dec.Decode(v)
}

func ms(t time.Time) int64 {
	if t.IsZero() {
		return 0
	}
	return t.UnixMilli()
}

func fromMS(v int64) time.Time {
	if v == 0 {
		return time.Time{}
	}
	return time.UnixMilli(v)
}

func b2i(b bool) int {
	if b {
		return 1
	}
	return 0
}

func (m *Manager) insert(j *job) error {
	tx, err := m.db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()
	if _, err := tx.Exec(`INSERT INTO jobs (id, kind, owner, state, position, name, dest, params, answers, cap,
		wait_parity, done_bytes, done_files, current, error, undo, result, created_at, started_at, finished_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, 0, 0, '', '', '', '', ?, 0, 0)`,
		j.ID, j.Kind, j.Owner, j.State, j.Position, j.Name, j.Dest, string(j.params), j.answersJSON(), j.Cap,
		b2i(j.WaitForParity), ms(j.CreatedAt)); err != nil {
		return err
	}
	for _, it := range j.items {
		if _, err := tx.Exec(`INSERT INTO job_items (job_id, seq, path, name, ref, state, resumed, bytes, files, error)
			VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
			j.ID, it.Seq, it.Path, it.Name, it.Ref, it.State, b2i(it.Resumed), it.Bytes, it.Files, it.Error); err != nil {
			return err
		}
	}
	return tx.Commit()
}

func (j *job) answersJSON() string {
	b, _ := json.Marshal(j.answers)
	return string(b)
}

// save writes the changing columns of a job.
func (m *Manager) save(j *job) {
	result := ""
	if j.Result != nil {
		if b, err := json.Marshal(j.Result); err == nil {
			result = string(b)
		}
	}
	m.exec(`UPDATE jobs SET state = ?, position = ?, answers = ?, cap = ?, wait_parity = ?, done_bytes = ?,
		done_files = ?, current = ?, error = ?, undo = ?, result = ?, started_at = ?, finished_at = ? WHERE id = ?`,
		j.State, j.Position, j.answersJSON(), j.Cap, b2i(j.WaitForParity), j.DoneBytes, j.DoneFiles, j.Current,
		j.Error, j.Undo, result, ms(j.StartedAt), ms(j.FinishedAt), j.ID)
	j.savedAt = time.Now()
}

func (m *Manager) saveItem(j *job, it Item) {
	m.exec(`UPDATE job_items SET ref = ?, state = ?, resumed = ?, bytes = ?, files = ?, error = ? WHERE job_id = ? AND seq = ?`,
		it.Ref, it.State, b2i(it.Resumed), it.Bytes, it.Files, it.Error, j.ID, it.Seq)
}

func (m *Manager) delete(id string) {
	m.exec(`DELETE FROM job_items WHERE job_id = ?`, id)
	m.exec(`DELETE FROM jobs WHERE id = ?`, id)
}

// exec logs instead of failing. A lost progress update must not stop a
// copy that is going well, and the next save writes the whole row again.
func (m *Manager) exec(query string, args ...any) {
	if _, err := m.db.Exec(query, args...); err != nil {
		m.logf("jobs: %v", err)
	}
}

func (m *Manager) load(ctx context.Context) ([]*job, error) {
	rows, err := m.db.QueryContext(ctx, `SELECT id, kind, owner, state, position, name, dest, params, answers, cap,
		wait_parity, done_bytes, done_files, current, error, undo, result, created_at, started_at, finished_at FROM jobs`)
	if err != nil {
		return nil, err
	}
	byID := map[string]*job{}
	var list []*job
	for rows.Next() {
		j := &job{}
		var params, ans, result string
		var waitParity int
		var created, started, finished int64
		if err := rows.Scan(&j.ID, &j.Kind, &j.Owner, &j.State, &j.Position, &j.Name, &j.Dest, &params, &ans, &j.Cap,
			&waitParity, &j.DoneBytes, &j.DoneFiles, &j.Current, &j.Error, &j.Undo, &result, &created, &started, &finished); err != nil {
			rows.Close()
			return nil, err
		}
		j.params = json.RawMessage(params)
		_ = json.Unmarshal([]byte(ans), &j.answers)
		if result != "" {
			j.Result = json.RawMessage(result)
		}
		j.WaitForParity = waitParity != 0
		j.CreatedAt, j.StartedAt, j.FinishedAt = fromMS(created), fromMS(started), fromMS(finished)
		byID[j.ID] = j
		list = append(list, j)
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		return nil, err
	}

	rows, err = m.db.QueryContext(ctx, `SELECT job_id, seq, path, name, ref, state, resumed, bytes, files, error
		FROM job_items ORDER BY job_id, seq`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	for rows.Next() {
		var id string
		var it Item
		var resumed int
		if err := rows.Scan(&id, &it.Seq, &it.Path, &it.Name, &it.Ref, &it.State, &resumed, &it.Bytes, &it.Files, &it.Error); err != nil {
			return nil, err
		}
		it.Resumed = resumed != 0
		if j, ok := byID[id]; ok {
			j.items = append(j.items, it)
		}
	}
	return list, rows.Err()
}

const (
	settingMode       = "mode"
	settingCap        = "cap"
	settingParityWait = "parityWait"
	settingNotify     = "notifyUnraid"
	settingNotifyAt   = "notifyAfter"
)

// Setting reads one value from job_settings, which also holds the
// settings of the trash.
func Setting(db *sql.DB, key, fallback string) string {
	var v string
	if err := db.QueryRow(`SELECT value FROM job_settings WHERE key = ?`, key).Scan(&v); err != nil {
		return fallback
	}
	return v
}

// PutSetting writes one value to job_settings.
func PutSetting(db *sql.DB, key, value string) error {
	_, err := db.Exec(`INSERT INTO job_settings (key, value) VALUES (?, ?)
		ON CONFLICT (key) DO UPDATE SET value = excluded.value`, key, value)
	return err
}

func (m *Manager) loadSettings() Settings {
	s := Settings{Mode: Serial, ParityWait: true}
	if Setting(m.db, settingMode, Serial) == Parallel {
		s.Mode = Parallel
	}
	s.Cap, _ = strconv.ParseInt(Setting(m.db, settingCap, "0"), 10, 64)
	s.ParityWait = Setting(m.db, settingParityWait, "1") == "1"
	s.NotifyUnraid = Setting(m.db, settingNotify, "0") == "1"
	s.NotifyAfter, _ = strconv.ParseInt(Setting(m.db, settingNotifyAt, "60"), 10, 64)
	return s
}

func (m *Manager) saveSettings(s Settings) error {
	for k, v := range map[string]string{
		settingMode:       s.Mode,
		settingCap:        strconv.FormatInt(s.Cap, 10),
		settingParityWait: strconv.Itoa(b2i(s.ParityWait)),
		settingNotify:     strconv.Itoa(b2i(s.NotifyUnraid)),
		settingNotifyAt:   strconv.FormatInt(s.NotifyAfter, 10),
	} {
		if err := PutSetting(m.db, k, v); err != nil {
			return err
		}
	}
	return nil
}
