package state

import (
	"encoding/json"
	"fmt"
	"reflect"
	"sync"

	"github.com/junkerderprovinz/cannoncommander/backend/internal/database/users"
	"github.com/junkerderprovinz/cannoncommander/backend/pkg/settings"
)

const sourceAccessDefaultsSettingKey = "sourceAccessDefaults"

var (
	sourceAccessMu              sync.RWMutex
	sourceAccessEnforcedDefault settings.SourceFilePermissionsEnforcement
)

type sourceAccessSettingsDocument struct {
	DefaultPermissions  users.SourceFilePermissions               `json:"defaultPermissions"`
	EnforcedPermissions settings.SourceFilePermissionsEnforcement `json:"enforcedPermissions,omitempty"`
}

// InitSourceAccessDefaults loads persisted source access defaults and applies them to every source.
func InitSourceAccessDefaults() error {
	if sqlDb == nil {
		return fmt.Errorf("sqlDb not initialized")
	}

	doc, found, err := loadSourceAccessSettingsDocument()
	if err != nil {
		return err
	}
	if !found {
		perms := deriveInitialSourceAccessDefaults()
		doc = sourceAccessSettingsDocument{DefaultPermissions: perms}
		if saveErr := saveSourceAccessSettingsDocument(doc); saveErr != nil {
			return saveErr
		}
	}

	if settings.Env.ConfigSourceDefaultPermissions != nil {
		overlaid := settings.OverlayConfigSourceDefaults(doc.DefaultPermissions)
		if !reflect.DeepEqual(doc.DefaultPermissions, overlaid) {
			doc.DefaultPermissions = overlaid
			if saveErr := saveSourceAccessSettingsDocument(doc); saveErr != nil {
				return fmt.Errorf("save config-synced source defaults: %w", saveErr)
			}
		}
	}

	settings.ApplySourceAccessDefaultsToAllSources(doc.DefaultPermissions)
	sourceAccessMu.Lock()
	sourceAccessEnforcedDefault = doc.EnforcedPermissions
	sourceAccessMu.Unlock()

	if !found {
		return stripLegacyFilePermissionsFromUserDefaults()
	}
	return nil
}

func deriveInitialSourceAccessDefaults() users.SourceFilePermissions {
	for _, src := range settings.Config.Server.Sources {
		if src != nil && !src.Config.DefaultPermissions.IsUnset() {
			return settings.NormalizeSourceFilePermissions(src.Config.DefaultPermissions)
		}
	}
	return settings.BuiltinDefaultSourceFilePermissions()
}

func loadSourceAccessSettingsDocument() (sourceAccessSettingsDocument, bool, error) {
	raw, err := sqlDb.GetSetting(sourceAccessDefaultsSettingKey)
	if err != nil {
		if err.Error() == fmt.Sprintf("setting not found: %s", sourceAccessDefaultsSettingKey) {
			return sourceAccessSettingsDocument{}, false, nil
		}
		return sourceAccessSettingsDocument{}, false, err
	}

	var probe map[string]json.RawMessage
	if err := json.Unmarshal(raw, &probe); err != nil {
		return sourceAccessSettingsDocument{}, false, fmt.Errorf("parse %s: %w", sourceAccessDefaultsSettingKey, err)
	}
	if _, wrapped := probe["defaultPermissions"]; wrapped {
		var doc sourceAccessSettingsDocument
		if err := json.Unmarshal(raw, &doc); err != nil {
			return sourceAccessSettingsDocument{}, false, fmt.Errorf("parse %s: %w", sourceAccessDefaultsSettingKey, err)
		}
		doc.DefaultPermissions = settings.NormalizeSourceFilePermissions(doc.DefaultPermissions)
		return doc, true, nil
	}

	var legacy users.SourceFilePermissions
	if err := json.Unmarshal(raw, &legacy); err != nil {
		return sourceAccessSettingsDocument{}, false, fmt.Errorf("parse legacy %s: %w", sourceAccessDefaultsSettingKey, err)
	}
	return sourceAccessSettingsDocument{
		DefaultPermissions: settings.NormalizeSourceFilePermissions(legacy),
	}, true, nil
}

func saveSourceAccessSettingsDocument(doc sourceAccessSettingsDocument) error {
	doc.DefaultPermissions = settings.NormalizeSourceFilePermissions(
		users.MarkSourceFilePermissionsConfigured(doc.DefaultPermissions),
	)
	return sqlDb.SaveSetting(sourceAccessDefaultsSettingKey, doc)
}

// GetSourceAccessDefaults returns the global default file permissions for sources.
func GetSourceAccessDefaults() users.SourceFilePermissions {
	return settings.DefaultSourceFilePermissions()
}


// GetEnforcedSourcePermissions returns universal enforced source permission flags.
func GetEnforcedSourcePermissions() settings.SourceFilePermissionsEnforcement {
	sourceAccessMu.RLock()
	defer sourceAccessMu.RUnlock()
	return sourceAccessEnforcedDefault
}

func stripLegacyFilePermissionsFromUserDefaults() error {
	patch := map[string]interface{}{
		"account": map[string]interface{}{
			"permissions": map[string]interface{}{
				"modify":   nil,
				"create":   nil,
				"delete":   nil,
				"download": nil,
				"view":     nil,
			},
		},
	}
	patchJSON, err := json.Marshal(patch)
	if err != nil {
		return err
	}
	merged, mergeErr := settings.MergeUserDefaultsPatchJSON(userDefaultsDefault, patchJSON)
	if mergeErr != nil {
		return mergeErr
	}
	userDefaultsDefault = merged
	settings.Config.UserDefaults = merged
	if saveErr := sqlDb.SaveSetting(userDefaultsDefaultSettingKey, merged); saveErr != nil {
		return saveErr
	}
	enfPatch := map[string]interface{}{
		"account": map[string]interface{}{
			"permissions": map[string]interface{}{
				"modify":   false,
				"create":   false,
				"delete":   false,
				"download": false,
				"view":     false,
			},
		},
	}
	enfJSON, err := json.Marshal(enfPatch)
	if err != nil {
		return err
	}
	mergedEnf, mergeErr := settings.MergeEnforcedPatchJSON(userDefaultsEnforcedDefault, enfJSON)
	if mergeErr != nil {
		return mergeErr
	}
	userDefaultsEnforcedDefault = mergedEnf
	if saveErr := sqlDb.SaveSetting(userDefaultsEnforcedDefaultKey, mergedEnf); saveErr != nil {
		return saveErr
	}
	return nil
}
