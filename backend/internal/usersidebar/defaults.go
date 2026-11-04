package usersidebar

import (
	"fmt"
	"strings"

	"github.com/junkerderprovinz/cannoncommander/backend/internal/database/users"
	"github.com/junkerderprovinz/cannoncommander/backend/pkg/settings"
)

// ErrEnforcedSidebarLinkRemoved is returned when a non-admin removes or alters an enforced link.
type ErrEnforcedSidebarLinkRemoved struct {
	Key string
}

func (e ErrEnforcedSidebarLinkRemoved) Error() string {
	return fmt.Sprintf("sidebar link %q is enforced by an administrator", e.Key)
}

// LinkKey returns a stable identity for merge, dedup, and enforcement checks (persisted/path form).
func LinkKey(link users.SidebarLink) string {
	if users.IsSourceSidebarCategory(link.Category) {
		if source, ok := resolveSourceLink(link); ok {
			target := normalizeSidebarTarget(link.Target)
			return fmt.Sprintf("source:%s:%s", source.Path, target)
		}
	}
	target := strings.TrimSpace(link.Target)
	if link.Category == "tool" && target == "/tools" {
		return "tool:/tools"
	}
	return fmt.Sprintf("%s:%s", strings.TrimSpace(link.Category), target)
}

// LinkKeyForDisplay returns a stable key using source display names for API responses.
func LinkKeyForDisplay(link users.SidebarLink) string {
	if users.IsSourceSidebarCategory(link.Category) {
		if source, ok := resolveSourceLink(link); ok {
			target := normalizeSidebarTarget(link.Target)
			return fmt.Sprintf("source:%s:%s", source.Name, target)
		}
	}
	return LinkKey(link)
}

// UserHasSourceAccess reports whether the user has a backend scope on the given source path.
func UserHasSourceAccess(scopes []users.BackendScope, sourcePath string) bool {
	sourcePath = strings.TrimSpace(sourcePath)
	if sourcePath == "" {
		return false
	}
	for _, scope := range scopes {
		if scope.Path == sourcePath {
			return true
		}
	}
	return false
}

func defaultItemAppliesToUser(item SidebarLinkDefaultItem, scopes []users.BackendScope) bool {
	if !users.IsSourceSidebarCategory(item.Link.Category) {
		return true
	}
	source, ok := resolveSourceLink(item.Link)
	if !ok {
		return false
	}
	return UserHasSourceAccess(scopes, source.Path)
}

func prepareDefaultLink(link users.SidebarLink) (users.SidebarLink, bool) {
	normalized, changed := NormalizeSidebarLinks([]users.SidebarLink{link})
	if len(normalized) == 0 {
		return users.SidebarLink{}, false
	}
	_ = changed
	return normalized[0], true
}

func linkKeysPresent(links []users.SidebarLink) map[string]users.SidebarLink {
	out := make(map[string]users.SidebarLink, len(links))
	for _, link := range links {
		out[LinkKey(link)] = link
	}
	return out
}

func linksEquivalent(a, b users.SidebarLink) bool {
	na, okA := prepareDefaultLink(a)
	nb, okB := prepareDefaultLink(b)
	if !okA || !okB {
		return false
	}
	return na.Name == nb.Name &&
		na.Category == nb.Category &&
		na.Target == nb.Target &&
		na.Icon == nb.Icon &&
		na.SourceName == nb.SourceName
}

// MergeDefaultLinks appends enabled default items missing from links. Returns updated links and whether changed.
func MergeDefaultLinks(links []users.SidebarLink, scopes []users.BackendScope, doc SidebarLinkDefaultsDocument) ([]users.SidebarLink, bool) {
	present := linkKeysPresent(links)
	out := append([]users.SidebarLink(nil), links...)
	changed := false

	for _, item := range doc.Items {
		if !item.Enabled {
			continue
		}
		if !defaultItemAppliesToUser(item, scopes) {
			continue
		}
		prepared, ok := prepareDefaultLink(item.Link)
		if !ok {
			continue
		}
		key := LinkKey(prepared)
		if _, exists := present[key]; exists {
			continue
		}
		out = append(out, prepared)
		present[key] = prepared
		changed = true
	}

	return out, changed
}

// MergeEnforcedLinks ensures enforced items are present and match the admin template for non-admins.
func MergeEnforcedLinks(links []users.SidebarLink, scopes []users.BackendScope, doc SidebarLinkDefaultsDocument, isAdmin bool) ([]users.SidebarLink, bool) {
	if isAdmin {
		return links, false
	}
	present := linkKeysPresent(links)
	out := append([]users.SidebarLink(nil), links...)
	changed := false

	for _, item := range doc.Items {
		if !item.Enforced {
			continue
		}
		if !defaultItemAppliesToUser(item, scopes) {
			continue
		}
		prepared, ok := prepareDefaultLink(item.Link)
		if !ok {
			continue
		}
		key := LinkKey(prepared)
		if existing, exists := present[key]; exists {
			if !linksEquivalent(existing, prepared) {
				for i := range out {
					if LinkKey(out[i]) == key {
						out[i] = prepared
						changed = true
						break
					}
				}
				present[key] = prepared
			}
			continue
		}
		out = append(out, prepared)
		present[key] = prepared
		changed = true
	}

	return out, changed
}

// ValidateEnforcedSidebarLinks returns an error if a non-admin removed or altered an enforced link.
func ValidateEnforcedSidebarLinks(links []users.SidebarLink, scopes []users.BackendScope, doc SidebarLinkDefaultsDocument, isAdmin bool) error {
	if isAdmin {
		return nil
	}
	present := linkKeysPresent(links)
	for _, item := range doc.Items {
		if !item.Enforced {
			continue
		}
		if !defaultItemAppliesToUser(item, scopes) {
			continue
		}
		prepared, ok := prepareDefaultLink(item.Link)
		if !ok {
			continue
		}
		key := LinkKey(prepared)
		existing, exists := present[key]
		if !exists {
			return ErrEnforcedSidebarLinkRemoved{Key: key}
		}
		if !linksEquivalent(existing, prepared) {
			return ErrEnforcedSidebarLinkRemoved{Key: key}
		}
	}
	return nil
}

// DocumentWithAllSources merges configured sources into the defaults document for admin UI.
func DocumentWithAllSources(doc SidebarLinkDefaultsDocument) SidebarLinkDefaultsDocument {
	if !users.SourceConfigLoaded() {
		return doc
	}
	bySourcePath := make(map[string][]SidebarLinkDefaultItem)
	custom := make([]SidebarLinkDefaultItem, 0, len(doc.Items))
	for _, item := range doc.Items {
		if users.IsSourceSidebarCategory(item.Link.Category) {
			if source, ok := resolveSourceLink(item.Link); ok {
				bySourcePath[source.Path] = append(bySourcePath[source.Path], item)
				continue
			}
		}
		custom = append(custom, item)
	}

	out := make([]SidebarLinkDefaultItem, 0, len(doc.Items)+len(settings.Config.Server.Sources))
	for _, src := range settings.Config.Server.Sources {
		if src == nil {
			continue
		}
		items := bySourcePath[src.Path]
		hasRoot := false
		for _, item := range items {
			if normalizeSidebarTarget(item.Link.Target) == "/" {
				hasRoot = true
				break
			}
		}
		if !hasRoot {
			out = append(out, SidebarLinkDefaultItem{
				Enabled: true,
				Link: users.SidebarLink{
					Name:       src.Name,
					Category:   string(users.SidebarLinkSource),
					Target:     "/",
					SourceName: src.Path,
				},
			})
		}
		out = append(out, items...)
		delete(bySourcePath, src.Path)
	}
	for _, items := range bySourcePath {
		out = append(out, items...)
	}
	out = append(out, custom...)
	return SidebarLinkDefaultsDocument{Items: out}
}

// EnsureAllSourcesInDefaults merges in any configured sources missing from doc (enabled by default).
func EnsureAllSourcesInDefaults(doc SidebarLinkDefaultsDocument) (SidebarLinkDefaultsDocument, bool) {
	merged := DocumentWithAllSources(doc)
	if len(merged.Items) == len(doc.Items) {
		return doc, false
	}
	return merged, true
}

// InitialSidebarLinkDefaultsDocument returns enabled defaults for every configured source.
func InitialSidebarLinkDefaultsDocument() SidebarLinkDefaultsDocument {
	return DocumentWithAllSources(SidebarLinkDefaultsDocument{Items: []SidebarLinkDefaultItem{}})
}
