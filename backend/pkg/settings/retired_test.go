package settings

import (
	"os"
	"path/filepath"
	"testing"
)

func TestAnOlderConfigWithRetiredKeysStillLoads(t *testing.T) {
	dir := t.TempDir()
	file := filepath.Join(dir, "config.yaml")
	content := []byte(`
auth:
  methods:
    password:
      enabled: true
      signup: true
      recaptcha:
        host: "https://www.google.com/recaptcha/api.js"
    ldap:
      enabled: true
      server: "ldap://ldap.example.com:389"
    jwt:
      enabled: true
      secret: "geheim"
    proxy:
      enabled: true
      header: "X-Remote-User"
server:
  database:
    quotas:
      flushIntervalSeconds: 10
  sources:
    - path: "."
frontend:
  name: "Turm"
  styling:
    disableEventThemes: true
  externalLinks:
    - text: "Hilfe"
      url: "https://example.org"
  favicon: "favicon.png"
  description: "Dateien"
  loginButtonText: "Los"
shareDefaults:
  disableAnonymous: true
`)
	if err := os.WriteFile(file, content, 0o644); err != nil {
		t.Fatal(err)
	}
	if err := loadConfigWithDefaults(file, false); err != nil {
		t.Fatalf("load: %v", err)
	}
	if !Config.Auth.Methods.PasswordAuth.Enabled || Config.Frontend.Name != "Turm" {
		t.Fatal("the rest of the config was not read")
	}
}

func TestAnUnknownKeyIsStillAnError(t *testing.T) {
	file := filepath.Join(t.TempDir(), "config.yaml")
	if err := os.WriteFile(file, []byte("auth:\n  methods:\n    smoke:\n      enabled: true\nserver:\n  sources:\n    - path: \".\"\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := loadConfigWithDefaults(file, false); err == nil {
		t.Fatal("a mistyped key loaded without complaint")
	}
}

func TestAConfigFromBeforeTheRuleCarryOverStillLoads(t *testing.T) {
	file := filepath.Join(t.TempDir(), "config.yaml")
	content := []byte(`
http:
  disableWebDAV: true
server:
  database:
    migrateFrom: "database.db.old"
  sources:
    - path: "."
      config:
        denyByDefault: true
`)
	if err := os.WriteFile(file, content, 0o644); err != nil {
		t.Fatal(err)
	}
	if err := loadConfigWithDefaults(file, false); err != nil {
		t.Fatalf("load: %v", err)
	}
	if !Config.Server.Sources[0].Config.DenyByDefault {
		t.Fatal("denyByDefault has to stay readable until the access rules took it over")
	}
}
