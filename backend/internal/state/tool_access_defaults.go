package state

import (
	"encoding/json"
	"fmt"
	"sync"

	"github.com/junkerderprovinz/cannoncommander/backend/internal/toolaccess"
)

const toolAccessDefaultsSettingKey = "toolAccessDefaults"

var (
	toolAccessDefaultsMu         sync.RWMutex
	toolAccessDefaults           toolaccess.ToolAccessDefaultsDocument
	toolAccessDefaultsNeedResync bool
	// errInjectResyncToolAccessDefaults is set by tests in this package only.
	errInjectResyncToolAccessDefaults error
)

// InitToolAccessDefaults loads persisted tool access defaults into memory.
func InitToolAccessDefaults() error {
	if sqlDb == nil {
		return fmt.Errorf("sqlDb not initialized")
	}
	doc, found, err := loadToolAccessDefaultsDocument()
	if err != nil {
		return err
	}
	if !found {
		doc = toolaccess.InitialToolAccessDefaultsDocument()
		if saveErr := saveToolAccessDefaultsDocument(doc); saveErr != nil {
			return saveErr
		}
	} else {
		ensured, changed := toolaccess.EnsureCatalogInDefaults(doc)
		if changed {
			if saveErr := saveToolAccessDefaultsDocument(ensured); saveErr != nil {
				return saveErr
			}
			doc = ensured
			toolAccessDefaultsNeedResync = true
		}
	}
	toolAccessDefaultsMu.Lock()
	toolAccessDefaults = doc
	toolAccessDefaultsMu.Unlock()
	if !found {
		toolAccessDefaultsNeedResync = true
	}
	return nil
}

func loadToolAccessDefaultsDocument() (toolaccess.ToolAccessDefaultsDocument, bool, error) {
	raw, err := sqlDb.GetSetting(toolAccessDefaultsSettingKey)
	if err != nil {
		if err.Error() == fmt.Sprintf("setting not found: %s", toolAccessDefaultsSettingKey) {
			return toolaccess.ToolAccessDefaultsDocument{}, false, nil
		}
		return toolaccess.ToolAccessDefaultsDocument{}, false, err
	}
	var doc toolaccess.ToolAccessDefaultsDocument
	if err := json.Unmarshal(raw, &doc); err != nil {
		return toolaccess.ToolAccessDefaultsDocument{}, false, fmt.Errorf("parse %s: %w", toolAccessDefaultsSettingKey, err)
	}
	if doc.Items == nil {
		doc.Items = []toolaccess.ToolAccessDefaultItem{}
	}
	return doc, true, nil
}

func saveToolAccessDefaultsDocument(doc toolaccess.ToolAccessDefaultsDocument) error {
	if doc.Items == nil {
		doc.Items = []toolaccess.ToolAccessDefaultItem{}
	}
	return sqlDb.SaveSetting(toolAccessDefaultsSettingKey, doc)
}

// EffectiveToolAccessDefaults returns the in-memory tool access defaults document.
func EffectiveToolAccessDefaults() toolaccess.ToolAccessDefaultsDocument {
	toolAccessDefaultsMu.RLock()
	defer toolAccessDefaultsMu.RUnlock()
	doc := toolAccessDefaults
	if doc.Items != nil {
		doc.Items = append([]toolaccess.ToolAccessDefaultItem(nil), doc.Items...)
	}
	return doc
}
