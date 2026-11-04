package settings

// userJSONFieldEnforcementPaths maps flat user JSON field names (PATCH which / data keys)
// to user-defaults enforcement dot-paths (see UserDefaultsEnforcement).
var userJSONFieldEnforcementPaths = map[string]string{
	"disableQuickToggles":        "sidebar.disableQuickToggles",
	"hideSidebarFileActions":     "sidebar.hideFileActions",
	"stickySidebar":              "sidebar.sticky",
	"hideFilesInTree":            "sidebar.hideFiles",
	"showToolsInSidebar":         "sidebar.showTools",
	"deleteWithoutConfirming":    "listing.deleteWithoutConfirming",
	"promptRightCloseButton":     "listing.promptRightCloseButton",
	"dateFormat":                 "listing.dateFormat",
	"showHidden":                 "listing.showHidden",
	"quickDownload":              "listing.quickDownload",
	"showSelectMultiple":         "listing.showSelectMultiple",
	"singleClick":                "listing.singleClick",
	"hideFileExt":                "listing.hideFileExt",
	"showCopyPath":               "listing.showCopyPath",
	"showTypeColumn":             "listing.showTypeColumn",
	"showCreationDateColumn":     "listing.showCreationDateColumn",
	"deleteAfterArchive":         "listing.deleteAfterArchive",
	"viewMode":                   "listing.viewMode",
	"gallerySize":                "listing.gallerySize",
	"newFileTemplate":            "listing.newFileTemplate",
	"disablePreviewExt":          "preview.disablePreviewExt",
	"disableSearchOptions":       "search.disableOptions",
	"preferEditorForMarkdown":    "fileViewer.preferEditorForMarkdown",
	"debugOffice":                "fileViewer.debugOffice",
	"disableViewingExt":          "fileViewer.disableViewingExt",
	"disableOnlyOfficeExt":       "fileViewer.disableOnlyOfficeExt",
	"disableOfficePreviewExt":    "fileViewer.disableOnlyOfficeExt",
	"darkMode":                   "ui.darkMode",
	"themeColor":                 "ui.themeColor",
	"customTheme":                "ui.customTheme",
	"locale":                     "ui.locale",
	"lockPassword":               "account.lockPassword",
	"disableSettings":            "account.disableSettings",
	"disableUpdateNotifications": "account.disableUpdateNotifications",
	"showAdvancedProfile":        "account.showAdvancedProfile",
}

// ErrEnforcedUserField is returned when a self-service user update touches an enforced default.
type ErrEnforcedUserField struct {
	Field string
	Path  string
}

// UserJSONFieldsForEnforcedSync lists top-level user JSON keys that enforced-default sync may modify.
func UserJSONFieldsForEnforcedSync() []string {
	out := make([]string, 0, len(userJSONFieldEnforcementPaths)+3)
	for k := range userJSONFieldEnforcementPaths {
		out = append(out, k)
	}
	out = append(out, "preview", "fileLoading", "permissions")
	return out
}
