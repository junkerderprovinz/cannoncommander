package state

import (
	"encoding/json"
	"fmt"
	"sync"

	"github.com/junkerderprovinz/cannoncommander/backend/pkg/settings"
)

const (
	shareDefaultsDefaultSettingKey  = "shareDefaults.default"
	shareDefaultsEnforcedDefaultKey = "shareDefaults.enforced.default"
)

var (
	shareDefaultsMu sync.RWMutex
)

// InitShareDefaultsSettings loads persisted share defaults from SQLite and seeds from config when missing.
func InitShareDefaultsSettings() error {
	seed := settings.Config.ShareDefaults
	if _, err := sqlDb.GetSetting(shareDefaultsDefaultSettingKey); err != nil {
		if saveErr := sqlDb.SaveSetting(shareDefaultsDefaultSettingKey, seed); saveErr != nil {
			return fmt.Errorf("seed share defaults: %w", saveErr)
		}
	}
	if _, err := sqlDb.GetSetting(shareDefaultsEnforcedDefaultKey); err != nil {
		if saveErr := sqlDb.SaveSetting(shareDefaultsEnforcedDefaultKey, settings.ShareDefaultsEnforcement{}); saveErr != nil {
			return fmt.Errorf("seed enforced share defaults: %w", saveErr)
		}
	}

	defaults, err := loadShareDefaultsSetting(shareDefaultsDefaultSettingKey)
	if err != nil {
		return fmt.Errorf("load share defaults: %w", err)
	}
	if _, err := loadShareDefaultsEnforcedSetting(shareDefaultsEnforcedDefaultKey); err != nil {
		return fmt.Errorf("load enforced share defaults: %w", err)
	}

	shareDefaultsMu.Lock()
	settings.Config.ShareDefaults = defaults
	shareDefaultsMu.Unlock()
	return nil
}

func loadShareDefaultsSetting(key string) (settings.ShareDefaults, error) {
	raw, err := sqlDb.GetSetting(key)
	if err != nil {
		return settings.ShareDefaults{}, err
	}
	var sd settings.ShareDefaults
	if err := json.Unmarshal(raw, &sd); err != nil {
		return settings.ShareDefaults{}, fmt.Errorf("parse %s: %w", key, err)
	}
	return sd, nil
}

func loadShareDefaultsEnforcedSetting(key string) (settings.ShareDefaultsEnforcement, error) {
	raw, err := sqlDb.GetSetting(key)
	if err != nil {
		return settings.ShareDefaultsEnforcement{}, err
	}
	var enforced settings.ShareDefaultsEnforcement
	if err := json.Unmarshal(raw, &enforced); err != nil {
		return settings.ShareDefaultsEnforcement{}, fmt.Errorf("parse %s: %w", key, err)
	}
	return enforced, nil
}
