package prefs

import (
	"bytes"
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"net/http"
	"regexp"
	"time"

	v1 "github.com/junkerderprovinz/cannoncommander/backend/internal/api/v1"
)

// deviceKeys follow the screen rather than the person: the phone wants one
// panel without the F-key bar while the desktop keeps two panels and its
// column widths.
var deviceKeys = map[string]bool{"dual": true, "side": true, "fkeys": true, "colWidths": true}

type kind byte

const (
	kindNull kind = iota
	kindBool
	kindString
	kindNumber
	kindArray
	kindObject
)

// kinds holds the JSON type of every setting the interface ships with, so a
// broken client cannot store a value that breaks the others. Keys added by
// later features pass unchecked.
var kinds = map[string]kind{
	"theme": kindString, "accent": kindString, "shape": kindString, "motion": kindString, "quietToasts": kindBool,
	"labelsButtons": kindString, "labelsSidebar": kindString, "labelsTabs": kindString, "labelsBottombar": kindString,
	"rainbow": kindBool, "reactive": kindBool, "rotate": kindBool, "tabOrder": kindArray,
	"columns": kindArray, "colWidths": kindObject,
	"density": kindString, "showHidden": kindBool, "showExt": kindBool, "syncBrowse": kindBool, "folderSizes": kindBool,
	"foldersFirst": kindBool, "dblclick": kindString, "fkeys": kindBool, "side": kindBool, "dual": kindBool,
	"quickview": kindBool, "confirmDelete": kindBool, "trash": kindBool, "trashDays": kindNumber, "stayDays": kindNumber,
	"language": kindString, "dateFormat": kindString, "startFolder": kindString,
	"folderViews": kindObject, "folderGroups": kindObject,
}

const (
	maxSettingsBytes = 256 << 10
	maxSettingKeys   = 500
	maxKeyLen        = 64
)

var deviceID = regexp.MustCompile(`^[A-Za-z0-9_-]{1,64}$`)

// Settings is a flat set of settings as the interface keeps them.
type Settings map[string]json.RawMessage

func kindOf(raw json.RawMessage) kind {
	raw = bytes.TrimSpace(raw)
	if len(raw) == 0 {
		return kindNull
	}
	switch raw[0] {
	case 'n':
		return kindNull
	case 't', 'f':
		return kindBool
	case '"':
		return kindString
	case '[':
		return kindArray
	case '{':
		return kindObject
	}
	return kindNumber
}

func (s Settings) validate() error {
	if len(s) > maxSettingKeys {
		return v1.Errorf(http.StatusBadRequest, "too many settings")
	}
	size := 0
	for k, v := range s {
		if k == "" || len(k) > maxKeyLen {
			return v1.Errorf(http.StatusBadRequest, "invalid setting name "+k)
		}
		if want, known := kinds[k]; known && kindOf(v) != want {
			return v1.Errorf(http.StatusBadRequest, "setting "+k+" has the wrong type")
		}
		size += len(k) + len(v)
	}
	if size > maxSettingsBytes {
		return v1.Errorf(http.StatusRequestEntityTooLarge, "settings are too large")
	}
	return nil
}

// device returns the device the request speaks for, or "" when the client
// sends none. Without a device the device settings are kept with the user.
func device(r *http.Request) (string, error) {
	d := r.URL.Query().Get("device")
	if d != "" && !deviceID.MatchString(d) {
		return "", v1.Errorf(http.StatusBadRequest, "invalid device id")
	}
	return d, nil
}

func loadDoc(ctx context.Context, db *sql.DB, query string, args ...any) (Settings, error) {
	var data string
	err := db.QueryRowContext(ctx, query, args...).Scan(&data)
	if errors.Is(err, sql.ErrNoRows) {
		return Settings{}, nil
	}
	if err != nil {
		return nil, err
	}
	out := Settings{}
	return out, json.Unmarshal([]byte(data), &out)
}

// loadSettings returns the settings of a user as seen from one device: the
// user's settings with the device's own values on top.
func loadSettings(ctx context.Context, db *sql.DB, userID, dev string) (Settings, error) {
	out, err := loadDoc(ctx, db, `SELECT data FROM user_prefs WHERE user_id = ?`, userID)
	if err != nil || dev == "" {
		return out, err
	}
	own, err := loadDoc(ctx, db, `SELECT data FROM device_prefs WHERE user_id = ? AND device = ?`, userID, dev)
	if err != nil {
		return nil, err
	}
	for k, v := range own {
		out[k] = v
	}
	return out, nil
}

// saveSettings replaces the settings of a user. With a device the device
// settings go to that device and the rest to the user.
func saveSettings(ctx context.Context, db *sql.DB, userID, dev string, s Settings) error {
	shared, own := s, Settings{}
	if dev != "" {
		shared = Settings{}
		for k, v := range s {
			if deviceKeys[k] {
				own[k] = v
			} else {
				shared[k] = v
			}
		}
	}
	now := time.Now().Unix()
	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	data, err := json.Marshal(shared)
	if err != nil {
		return err
	}
	if _, err := tx.ExecContext(ctx, `INSERT INTO user_prefs (user_id, data, updated_at) VALUES (?, ?, ?)
		ON CONFLICT (user_id) DO UPDATE SET data = excluded.data, updated_at = excluded.updated_at`, userID, string(data), now); err != nil {
		return err
	}
	if dev != "" {
		if data, err = json.Marshal(own); err != nil {
			return err
		}
		if _, err := tx.ExecContext(ctx, `INSERT INTO device_prefs (user_id, device, data, updated_at) VALUES (?, ?, ?, ?)
			ON CONFLICT (user_id, device) DO UPDATE SET data = excluded.data, updated_at = excluded.updated_at`, userID, dev, string(data), now); err != nil {
			return err
		}
	}
	return tx.Commit()
}

func (h handlers) getSettings(w http.ResponseWriter, r *http.Request) error {
	dev, err := device(r)
	if err != nil {
		return err
	}
	s, err := loadSettings(r.Context(), h.db, userID(r), dev)
	if err != nil {
		return err
	}
	return v1.WriteJSON(w, r, http.StatusOK, s)
}

func (h handlers) putSettings(w http.ResponseWriter, r *http.Request) error {
	dev, err := device(r)
	if err != nil {
		return err
	}
	var s Settings
	if err := v1.DecodeJSON(r, &s); err != nil {
		return err
	}
	if s == nil {
		return v1.Errorf(http.StatusBadRequest, "settings must be an object")
	}
	if err := s.validate(); err != nil {
		return err
	}
	if err := saveSettings(r.Context(), h.db, userID(r), dev, s); err != nil {
		return err
	}
	h.changed(r, Change{Kind: "settings", Device: dev})
	return noContent(w)
}
