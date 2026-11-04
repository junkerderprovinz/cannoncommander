package users

import (
	"encoding/json"
	"fmt"
	"strings"

	jwt "github.com/golang-jwt/jwt/v4"
)

// ProfileStorageVersion is the user version after nested profile JSON in user_data.
// It marks a storage-shape boundary (used by the persist layer), not the newest version.
const ProfileStorageVersion = 5

// NewestUserVersion is the single reference for the latest user version.
// Per-migration watermarks below keep their historical values; only this moves forward.
const NewestUserVersion = 6

// PermMigrationVersion is the user version after the legacy perm-key to permissions migration.
const PermMigrationVersion = 1

// TokenMigrationVersion is the user version after the legacy apiKeys to tokens migration.
const TokenMigrationVersion = 2

// TokenHashBackfillVersion is the user version that guarantees every stored API
// token (Tokens and legacy ApiKeys) has a SHA256(raw) → owner row in
// hashed_tokens. Strict server-side auth ignores BelongsTo and rejects any
// bearer JWT without a hash mapping, so this migration must run on startup.
const TokenHashBackfillVersion = 6

// CurrentUserMigrationVersion is persisted for newly created accounts and after legacy migrations finish.
const CurrentUserMigrationVersion = NewestUserVersion

type LoginMethod string

const (
	LoginMethodPassword LoginMethod = "password"
	LoginMethodOidc     LoginMethod = "oidc"
)

// AnonymousUserName is the username for unauthenticated requests (user_id 0).
const AnonymousUserName = "anonymous"

var reservedUsernames = map[string]struct{}{
	strings.ToLower(AnonymousUserName): {},
}

// ValidateUsername rejects empty and reserved login names, and names that do not map to their
// own createUserDir home directory.
func ValidateUsername(username string) error {
	raw := username
	username = strings.TrimSpace(username)
	if username == "" {
		return fmt.Errorf("username is empty")
	}
	if _, reserved := reservedUsernames[strings.ToLower(username)]; reserved {
		return fmt.Errorf("username %q is reserved", username)
	}
	// The per-user home directory is derived from CleanUsername(username), so reject names
	// that it would rewrite, the names "." and "-", and names containing path separators.
	if CleanUsername(raw) != raw || raw == "." || raw == "-" || strings.ContainsAny(raw, `/\`) {
		return fmt.Errorf("username %q contains characters that are not allowed", raw)
	}
	return nil
}

// WebAuthnCredentialFlags mirrors the boolean flags from authenticator data.
type WebAuthnCredentialFlags struct {
	UserPresent    bool `json:"userPresent"`
	UserVerified   bool `json:"userVerified"`
	BackupEligible bool `json:"backupEligible"`
	BackupState    bool `json:"backupState"`
}

// WebAuthnCredential stores a single passkey credential in BoltDB-friendly format.
type WebAuthnCredential struct {
	ID                string                  `json:"id"`
	PublicKey         string                  `json:"publicKey"`
	AttestationType   string                  `json:"attestationType"`
	AttestationFormat string                  `json:"attestationFormat"`
	Transport         []string                `json:"transport"`
	Flags             WebAuthnCredentialFlags `json:"flags"`
	AAGUID            string                  `json:"aaguid"`
	SignCount         uint32                  `json:"signCount"`
	CloneWarning      bool                    `json:"cloneWarning"`
	Name              string                  `json:"name"`
	CreatedAt         int64                   `json:"createdAt"`
	LastUsedAt        int64                   `json:"lastUsedAt"`
	ClientDataJSON    string                  `json:"clientDataJSON"`
	ClientDataHash    string                  `json:"clientDataHash"`
	AuthenticatorData string                  `json:"authenticatorData"`
	PublicKeyAlg      int64                   `json:"publicKeyAlg"`
	AttestationObj    string                  `json:"attestationObj"`
	// Origin is the host the passkey was registered under, the only one it works for.
	Origin string `json:"origin,omitempty"`
}

type AuthToken struct {
	MinimalAuthToken
	Key       string `json:"key,omitempty"` // for backward compatibility
	Token     string `json:"token,omitempty"`
	Name      string `json:"name,omitempty"`
	Username  string `json:"username,omitempty"`
	BelongsTo uint64 `json:"belongsTo,omitempty"` // legacy stored metadata only, never trusted for auth; identity is always the hashed_tokens lookup
	IssuedAt  int64  `json:"issuedAt,omitempty"`
	ExpiresAt int64  `json:"expiresAt,omitempty"`
	// Minimal distinguishes owner-inheriting tokens from customized caps. Nil means legacy (infer from stored permissions).
	Minimal     *bool       `json:"minimal,omitempty"`
	Permissions Permissions `json:"Permissions,omitempty"`
	// ReadOnly limits the token to looking at and downloading files.
	ReadOnly bool `json:"readOnly,omitempty"`
}

// MinimalAuthToken is used for tokens that only include JWT standard claims
type MinimalAuthToken struct {
	jwt.RegisteredClaims `swaggerignore:"true"`
}

type Permissions struct {
	Api        bool `json:"api"`                // allow api access
	Admin      bool `json:"admin"`              // allow admin access
	Modify     bool `json:"modify,omitempty"`   // deprecated: legacy user migration only; not used for API tokens
	Share      bool `json:"share"`              // allow sharing files
	Realtime   bool `json:"realtime"`           // allow realtime updates
	Delete     bool `json:"delete,omitempty"`   // deprecated: legacy user migration only; not used for API tokens
	Create     bool `json:"create,omitempty"`   // deprecated: legacy user migration only; not used for API tokens
	Download   bool `json:"download,omitempty"` // deprecated: legacy user migration only; not used for API tokens
	View       bool `json:"view,omitempty"`     // deprecated: legacy user migration only; not used for API tokens
	Configured bool `json:"-"`                  // true when explicitly set on create (allows intentional deny-all); not persisted or API-visible

	PresentGlobalKeys GlobalPermissionsFieldMask `json:"-"` // JSON keys present on create-user permissions object
}

// MarkPermissionsConfigured marks account permissions as explicitly set (including all-false).
func MarkPermissionsConfigured(p Permissions) Permissions {
	p.Configured = true
	return p
}

// GlobalPermissionsFieldMaskFromPermissions returns keys recorded during Permissions JSON decode.
func (p Permissions) GlobalPermissionsFieldMaskFromPermissions() GlobalPermissionsFieldMask {
	return p.PresentGlobalKeys
}

// UnmarshalJSON sets Configured so an explicit all-false permissions object is distinct from omission.
// JSON null and absent permissions do not set Configured.
func (p *Permissions) UnmarshalJSON(data []byte) error {
	trimmed := strings.TrimSpace(string(data))
	if trimmed == "" || trimmed == "null" {
		*p = Permissions{}
		return nil
	}
	var obj map[string]json.RawMessage
	if err := json.Unmarshal(data, &obj); err != nil {
		return err
	}
	type plain Permissions
	var tmp plain
	if err := json.Unmarshal(data, &tmp); err != nil {
		return err
	}
	*p = Permissions(tmp)
	p.Configured = true
	p.PresentGlobalKeys = GlobalPermissionsFieldMaskFromJSON(data)
	return nil
}

// EffectiveGlobalPermissionsMask combines the caller-supplied field mask and the key
// presence recorded by Permissions.UnmarshalJSON.
func (u *User) EffectiveGlobalPermissionsMask() GlobalPermissionsFieldMask {
	fromPerms := u.Permissions.GlobalPermissionsFieldMaskFromPermissions()
	m := u.PermissionsFieldMask
	return GlobalPermissionsFieldMask{
		Admin:    m.Admin || fromPerms.Admin,
		Api:      m.Api || fromPerms.Api,
		Share:    m.Share || fromPerms.Share,
		Realtime: m.Realtime || fromPerms.Realtime,
	}
}

// SourceFilePermissions holds per-source file operation permissions (v4+).
type SourceFilePermissions struct {
	View       bool `json:"view"`
	Download   bool `json:"download"`
	Modify     bool `json:"modify"`
	Delete     bool `json:"delete"`
	Create     bool `json:"create"`
	Configured bool `json:"configured,omitempty"` // true when explicitly set (allows intentional deny-all)
}

// MarkSourceFilePermissionsConfigured marks permissions as explicitly set (including deny-all).
func MarkSourceFilePermissionsConfigured(p SourceFilePermissions) SourceFilePermissions {
	p.Configured = true
	return p
}

// DenyAllSourceFilePermissions returns a SourceFilePermissions with all flags false and Configured set.
func DenyAllSourceFilePermissions() SourceFilePermissions {
	return MarkSourceFilePermissionsConfigured(SourceFilePermissions{})
}

// IsUnset reports whether no permission bit is set and the value was never explicitly configured.
func (p SourceFilePermissions) IsUnset() bool {
	if p.Configured {
		return false
	}
	return !p.View && !p.Download && !p.Modify && !p.Delete && !p.Create
}

// HasAnyFilePermission reports whether any file-operation flag is set on token/global claims.
func (p Permissions) HasAnyFilePermission() bool {
	return p.View || p.Download || p.Modify || p.Delete || p.Create
}

// FrontendUser holds fields safe to return from user APIs (embedded on User).
type FrontendUser struct {
	NonAdminEditable
	DisableSettings       bool                             `json:"disableSettings"`
	Username              string                           `json:"username"`
	FrontendScopes        []FrontendScope                  `json:"scopes"`
	LockPassword          bool                             `json:"lockPassword"`
	RequirePasswordChange bool                             `json:"requirePasswordChange"`
	Permissions           Permissions                      `json:"permissions"`                 // global: admin, api, share, realtime
	SourcePermissions     map[string]SourceFilePermissions `json:"sourcePermissions,omitempty"` // deprecated: use scopes[].permissions
	LoginMethod           LoginMethod                      `json:"loginMethod"`
	OtpEnabled            bool                             `json:"otpEnabled"`
	ShowFirstLogin        bool                             `json:"showFirstLogin"`
	Perm                  Permissions                      `json:"perm,omitzero"`
}

// PinnedItems maps source filesystem path -> index directory path -> pinned item names.
type PinnedItems map[string]map[string][]string

// UserLegacyFields holds v1.x user JSON keys migrated on startup (Bolt/SQLite user_data).
type UserLegacyFields struct {
	ApiKeys map[string]AuthToken `json:"apiKeys,omitempty"`
}

// User is the persisted user: profile/settings in user_data, plus BackendScopes for source access.
type User struct {
	FrontendUser
	ID uint64 `json:"id,omitempty"`
	// PermissionsFieldMask marks global permission keys a programmatic create caller
	// explicitly supplied; not persisted. JSON create requests record key presence on
	// Permissions.PresentGlobalKeys during decode instead.
	PermissionsFieldMask GlobalPermissionsFieldMask `json:"-"`
	// BackendScopes is the authoritative, persisted access list (SourceScope.Name = backend source path).
	// SQLite stores this inside user_data JSON under the key "scopes" (see sqldb.UserData).
	BackendScopes            []BackendScope                   `json:"backendScopes,omitempty"`
	DeclinedDefaultSources   []string                         `json:"declinedDefaultSources,omitempty"`
	BackendSourcePermissions map[string]SourceFilePermissions `json:"backendSourcePermissions,omitempty"` // key = source path
	Tokens                   map[string]AuthToken             `json:"tokens,omitempty"`
	TOTPSecret               string                           `json:"totpSecret,omitempty"`
	TOTPNonce                string                           `json:"totpNonce,omitempty"`
	PasskeyCredentials       []WebAuthnCredential             `json:"passkeyCredentials,omitempty"`
	PinnedItems              PinnedItems                      `json:"pinnedItems,omitempty"`
	Version                  int                              `json:"version"`
	UserLegacyFields         `json:",inline"`
}

type FrontendScope struct {
	Name        string                 `json:"name"`  // Bolt: filesystem path; JSON API: display name after prepForFrontend
	Scope       string                 `json:"scope"` // index path within that source
	Permissions *SourceFilePermissions `json:"permissions,omitempty"`
}

type BackendScope struct {
	Path        string                `json:"path"`  // real path for the source
	Scope       string                `json:"scope"` // index path within that source
	Permissions SourceFilePermissions `json:"permissions,omitempty"`
}

// json tags must match variable name with smaller case first letter
type NonAdminEditable struct {
	Password           string               `json:"password,omitempty"`
	Locale             string               `json:"locale"` // language for the office editor: eg. de, en, or fr
	ShowFirstLogin     bool                 `json:"showFirstLogin"`
	PasskeyCredentials []WebAuthnCredential `json:"passkeyCredentials,omitempty"`
}

func CleanUsername(s string) string {
	// Remove any trailing space to avoid ending on -
	s = strings.Trim(s, " ")
	s = strings.Replace(s, "..", "", -1)
	return s
}

// SourceNameResolver is a function type that resolves source names to source paths
// This avoids circular dependencies by allowing the settings package to provide the resolver
type SourceNameResolver func(sourceName string) (sourcePath string, err error)

// SourceInfo represents basic information about a source needed for user operations
type SourceInfo struct {
	Path             string
	Name             string
	DefaultUserScope string
}

// SourceConfigProvider provides access to source configuration
type SourceConfigProvider struct {
	GetSourceByPath  func(path string) (SourceInfo, bool)
	GetSourceByName  func(name string) (SourceInfo, bool)
	GetAllSources    func() []SourceInfo
	GetDefaultScopes func() []BackendScope
}

// Global variables set by the settings package
var (
	sourceNameResolver SourceNameResolver
	sourceConfig       *SourceConfigProvider
)

// SetSourceNameResolver sets the global source name resolver
// This should be called once during initialization by the settings package
func SetSourceNameResolver(resolver SourceNameResolver) {
	sourceNameResolver = resolver
}

// SetSourceConfig sets the global source configuration provider
// This should be called once during initialization by the settings package
func SetSourceConfig(config *SourceConfigProvider) {
	sourceConfig = config
}

// GetSourceConfig returns the current source configuration provider, or nil if unset.
func GetSourceConfig() *SourceConfigProvider {
	return sourceConfig
}

// GetSourceNameResolver returns the current source name resolver, or nil if unset.
func GetSourceNameResolver() SourceNameResolver {
	return sourceNameResolver
}

// SourceConfigLoaded reports whether SetSourceConfig has been called (needed for ResolveSourceKey).
func SourceConfigLoaded() bool {
	return sourceConfig != nil
}

// GetScopeForSourcePath returns the scope for a given source path (backend-style)
// This method works with backend-style scopes where Name is the source path
func (u *User) GetScopeForSourcePath(sourcePath string) (string, error) {
	for _, scope := range u.BackendScopes {
		if scope.Path == sourcePath {
			return scope.Scope, nil
		}
	}
	return "", fmt.Errorf("scope not found for source %v", sourcePath)
}

// GetScopeForSourceName returns the scope for a given source name (frontend-style)
// Uses the global SourceNameResolver to convert source names to paths
func (u *User) GetScopeForSourceName(sourceName string) (string, error) {
	if sourceNameResolver == nil {
		return "", fmt.Errorf("source name resolver not initialized")
	}

	sourcePath, err := sourceNameResolver(sourceName)
	if err != nil {
		return "", err
	}

	return u.GetScopeForSourcePath(sourcePath)
}

// HasSourceByPath checks if the user has access to a given source path
func (u *User) HasSourceByPath(sourcePath string) bool {
	for _, scope := range u.BackendScopes {
		if scope.Path == sourcePath {
			return true
		}
	}
	return false
}

// GetSourcePaths returns all source paths the user has access to
func (u *User) GetSourcePaths() []string {
	paths := make([]string, 0, len(u.BackendScopes))
	for _, scope := range u.BackendScopes {
		paths = append(paths, scope.Path)
	}
	return paths
}
