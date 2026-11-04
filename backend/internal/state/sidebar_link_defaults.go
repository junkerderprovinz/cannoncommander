package state

import (
	"encoding/json"
	"fmt"
	"sync"

	"github.com/junkerderprovinz/cannoncommander/backend/internal/usersidebar"
)

const sidebarLinkDefaultsSettingKey = "sidebarLinkDefaults"

var (
	sidebarLinkDefaultsMu         sync.RWMutex
	sidebarLinkDefaults           usersidebar.SidebarLinkDefaultsDocument
	sidebarLinkDefaultsNeedResync bool
	// errInjectResyncSidebarDefaults is set by tests in this package only.
	errInjectResyncSidebarDefaults error
)

// InitSidebarLinkDefaults loads persisted sidebar link defaults into memory.
func InitSidebarLinkDefaults() error {
	if sqlDb == nil {
		return fmt.Errorf("sqlDb not initialized")
	}
	doc, found, err := loadSidebarLinkDefaultsDocument()
	if err != nil {
		return err
	}
	if !found {
		doc = usersidebar.InitialSidebarLinkDefaultsDocument()
		if saveErr := saveSidebarLinkDefaultsDocument(doc); saveErr != nil {
			return saveErr
		}
	} else {
		ensured, changed := usersidebar.EnsureAllSourcesInDefaults(doc)
		if changed {
			if saveErr := saveSidebarLinkDefaultsDocument(ensured); saveErr != nil {
				return saveErr
			}
			doc = ensured
			sidebarLinkDefaultsNeedResync = true
		}
	}
	sidebarLinkDefaultsMu.Lock()
	sidebarLinkDefaults = doc
	sidebarLinkDefaultsMu.Unlock()
	if !found {
		sidebarLinkDefaultsNeedResync = true
	}
	return nil
}

func loadSidebarLinkDefaultsDocument() (usersidebar.SidebarLinkDefaultsDocument, bool, error) {
	raw, err := sqlDb.GetSetting(sidebarLinkDefaultsSettingKey)
	if err != nil {
		if err.Error() == fmt.Sprintf("setting not found: %s", sidebarLinkDefaultsSettingKey) {
			return usersidebar.SidebarLinkDefaultsDocument{}, false, nil
		}
		return usersidebar.SidebarLinkDefaultsDocument{}, false, err
	}
	var doc usersidebar.SidebarLinkDefaultsDocument
	if err := json.Unmarshal(raw, &doc); err != nil {
		return usersidebar.SidebarLinkDefaultsDocument{}, false, fmt.Errorf("parse %s: %w", sidebarLinkDefaultsSettingKey, err)
	}
	if doc.Items == nil {
		doc.Items = []usersidebar.SidebarLinkDefaultItem{}
	}
	return doc, true, nil
}

func saveSidebarLinkDefaultsDocument(doc usersidebar.SidebarLinkDefaultsDocument) error {
	if doc.Items == nil {
		doc.Items = []usersidebar.SidebarLinkDefaultItem{}
	}
	return sqlDb.SaveSetting(sidebarLinkDefaultsSettingKey, doc)
}

// EffectiveSidebarLinkDefaults returns the in-memory sidebar link defaults document.
func EffectiveSidebarLinkDefaults() usersidebar.SidebarLinkDefaultsDocument {
	sidebarLinkDefaultsMu.RLock()
	defer sidebarLinkDefaultsMu.RUnlock()
	return sidebarLinkDefaults
}

