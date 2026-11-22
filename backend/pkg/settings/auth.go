package settings

import (
	"strings"

	"github.com/gtsteffaniak/go-logger/logger"
)

// AuthCommon contains fields shared across multiple authentication methods
type AuthCommon struct {
	Enabled           bool     `json:"enabled"`           // whether to enable this authentication method.
	AdminGroup        string   `json:"adminGroup"`        // if set, users in this group will be granted admin privileges
	UserGroups        []string `json:"userGroups"`        // if set, only users in these groups are allowed to log in. Blocks all other users even with valid credentials.
	GroupsClaim       string   `json:"groupsClaim"`       // the JSON field name to read groups from. Empty takes the provider's, usually "groups"
	UserIdentifier    string   `json:"userIdentifier"`    // the field value to use as the username. Empty takes the provider's, usually "preferred_username". Other common values are "email" or "username"
	DisableVerifyTLS  bool     `json:"disableVerifyTLS"`  // disable TLS verification (insecure, for testing only)
	LogoutRedirectUrl string   `json:"logoutRedirectUrl"` // if provider logout url is provided, the server will also redirect to logout url. Custom logout query params are respected.
	CreateUser        bool     `json:"createUser"`        // deprecated: always true for supported authentication methods
}

type Auth struct {
	TokenExpirationHours int          `json:"tokenExpirationHours"` // time in hours each web UI session token is valid for. Default is 2 hours.
	Methods              LoginMethods `json:"methods"`
	Key                  string       `json:"key"`           // secret: HMAC key for JWT tokens. If unset in config/env, one is generated and stored in the application database.
	TotpSecret           string       `json:"totpSecret"`    // secret: secret used to encrypt TOTP secrets
	AdminUsername        string       `json:"adminUsername"` // deprecated: use auth.methods.password.adminUsername. secret: password-auth admin username.
	AdminPassword        string       `json:"adminPassword"` // deprecated: use auth.methods.password.adminPassword. secret: password-auth admin password.
	AuthMethods          []string     `json:"-"`
}

// ConfigTriggersAdminPasswordReset reports whether startup should sync auth.adminPassword from config into the admin user record.
// Blank or "admin" leaves the stored password unchanged across restarts.
func ConfigTriggersAdminPasswordReset() bool {
	p := PasswordAdminPassword()
	return p != "" && p != "admin"
}

type LoginMethods struct {
	NoAuth       bool               `json:"noauth"` // if set to true, overrides all other auth methods and disables authentication
	PasswordAuth PasswordAuthConfig `json:"password" validate:"omitempty"`
	OidcAuth     OidcConfig         `json:"oidc" validate:"omitempty"`
	PasskeyAuth  PasskeyAuthConfig  `json:"passkey" validate:"omitempty"`
}

type PasswordAuthConfig struct {
	Enabled       bool   `json:"enabled"`
	AdminUsername string `json:"adminUsername"`                  // secret: admin username auto-assigned. If not set, the default is "admin".
	AdminPassword string `json:"adminPassword"`                  // secret: password for admin auto-assigned admin account. If set, reset on startup.
	MinLength     int    `json:"minLength" validate:"omitempty"` // below this length the sign-in points out that a password is short, default is 12. It never refuses one.
	EnforcedOtp   bool   `json:"enforcedOtp"`                    // has no effect: each account turns on two-factor sign-in for itself.
}

// PasswordAdminUsername returns the password-auth admin username.
// auth.methods.password.adminUsername takes priority; otherwise auth.adminUsername;
// otherwise "admin".
func PasswordAdminUsername() string {
	if u := strings.TrimSpace(Config.Auth.Methods.PasswordAuth.AdminUsername); u != "" {
		return u
	}
	if u := strings.TrimSpace(Config.Auth.AdminUsername); u != "" {
		return u
	}
	return "admin"
}

// PasswordAdminPassword returns the password-auth admin password override.
// auth.methods.password.adminPassword takes priority; otherwise auth.adminPassword.
func PasswordAdminPassword() string {
	if p := Config.Auth.Methods.PasswordAuth.AdminPassword; p != "" {
		return p
	}
	return Config.Auth.AdminPassword
}

// MigrateLegacyPasswordAdminFromAuth copies deprecated top-level auth.adminUsername and
// auth.adminPassword into auth.methods.password in memory when the nested keys are
// unset (or still the generated "admin" placeholder). It does not rewrite the config file.
func MigrateLegacyPasswordAdminFromAuth() {
	pwd := &Config.Auth.Methods.PasswordAuth
	legacyUser := strings.TrimSpace(Config.Auth.AdminUsername)
	if legacyUser != "" {
		nestedUser := strings.TrimSpace(pwd.AdminUsername)
		if nestedUser == "" || (nestedUser == "admin" && legacyUser != "admin") {
			pwd.AdminUsername = legacyUser
			logger.Warning("auth.adminUsername is deprecated; use auth.methods.password.adminUsername")
		}
	}
	legacyPass := Config.Auth.AdminPassword
	if legacyPass != "" {
		nestedPass := pwd.AdminPassword
		if nestedPass == "" || (nestedPass == "admin" && legacyPass != "admin") {
			pwd.AdminPassword = legacyPass
			logger.Warning("auth.adminPassword is deprecated; use auth.methods.password.adminPassword")
		}
	}
}

// OpenID OAuth2.0
type OidcConfig struct {
	AuthCommon   `json:",inline"`
	ClientID     string `json:"clientId"`     // secret: client id of the OIDC application
	ClientSecret string `json:"clientSecret"` // secret: client secret of the OIDC application
	IssuerUrl    string `json:"issuerUrl"`    // authorization URL of the OIDC provider
	Scopes       string `json:"scopes"`       // scopes to request from the OIDC provider. Empty takes the provider's
	Name         string `json:"name"`         // the name the sign-in button shows. Empty is the provider's own name
	Provider     string `json:"provider"`     // the kind of provider: pocket-id, authentik, authelia, keycloak, kanidm, zitadel or custom. Empty recognises it
	CaFile       string `json:"caFile"`       // PEM file of a certificate authority to trust for this provider, such as your own
}

// PasskeyAuthConfig configures passkeys. They are always on, so Enabled is
// only read for config files that still carry it.
type PasskeyAuthConfig struct {
	Enabled         bool     `json:"enabled"`         // has no effect, passkeys are always on
	RPDisplayName   string   `json:"rpDisplayName"`   // the Relying Party display name (defaults to frontend name)
	RPID            string   `json:"rpId"`            // the Relying Party ID until a request names its host, which every passkey request does
	RPOrigins       []string `json:"rpOrigins"`       // allowed origins; if empty, auto-derived from base URL
	LoginButtonText string   `json:"loginButtonText"` // custom text for the passkey login button
}
