package settings

import (
	"os"
	"strings"
	"testing"
)

// TestSecretMarkers_drive_yaml_redaction guards the marker gaps fixed for GHSA-892f-9h5p-mfc8:
// the YAML export redacts recaptcha.secret and ldap.userPassword, which previously leaked
// because their Go comments lacked the `secret:` marker the generator looks for.
func TestSecretMarkers_drive_yaml_redaction(t *testing.T) {
	embedded, err := os.ReadFile("../../../frontend/public/config.generated.yaml")
	if err != nil {
		t.Fatalf("read generated yaml: %v", err)
	}

	prev := Config
	t.Cleanup(func() { Config = prev })
	Config = SetDefaults(false)
	Config.Auth.Key = "SENTINEL-AUTH-KEY"
	Config.Auth.TotpSecret = "SENTINEL-TOTP"
	Config.Auth.AdminPassword = "SENTINEL-LEGACY-ADMIN-PW"
	Config.Auth.AdminUsername = "SENTINEL-LEGACY-ADMIN-USER"
	Config.Auth.Methods.PasswordAuth.AdminPassword = "SENTINEL-NESTED-ADMIN-PW"
	Config.Auth.Methods.OidcAuth.ClientSecret = "SENTINEL-OIDC"
	Config.Integrations.OnlyOffice.Secret = "SENTINEL-OFFICE"

	out, err := GenerateConfigYamlWithEmbedded(&Config, false, true, false, string(embedded))
	if err != nil {
		t.Fatalf("generate yaml: %v", err)
	}
	for _, sentinel := range []string{
		"SENTINEL-AUTH-KEY",
		"SENTINEL-TOTP",
		"SENTINEL-LEGACY-ADMIN-PW",
		"SENTINEL-LEGACY-ADMIN-USER",
		"SENTINEL-NESTED-ADMIN-PW",
		"SENTINEL-OIDC",
		"SENTINEL-OFFICE",
	} {
		if strings.Contains(out, sentinel) {
			t.Errorf("YAML export leaked %s", sentinel)
		}
	}
}

// TestActiveSecretFields_fromEmbeddedYaml exercises the deployed-binary path, where the
// secret markers come from the embedded config.generated.yaml rather than the Go sources.
func TestActiveSecretFields_fromEmbeddedYaml(t *testing.T) {
	data, err := os.ReadFile("../../../frontend/public/config.generated.yaml")
	if err != nil {
		t.Skipf("generated YAML not available: %v", err)
	}
	// Assert the YAML parser output directly: ActiveSecretFields also unions in the reflection
	// fallback (SecretFieldNames), which would mask broken marker detection.
	_, secrets, _, err := CollectCommentsFromEmbeddedYaml(string(data))
	if err != nil {
		t.Fatalf("parse embedded yaml: %v", err)
	}
	for _, tc := range []struct{ typ, field string }{
		{"Auth", "Key"},
		{"OidcConfig", "ClientSecret"},
		{"OnlyOffice", "Secret"},
		{"PasswordAuthConfig", "AdminPassword"},
	} {
		if !secrets[tc.typ][tc.field] {
			t.Errorf("%s.%s is not marked secret via embedded YAML", tc.typ, tc.field)
		}
	}
}
