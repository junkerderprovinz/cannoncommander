package settings

import "testing"

func TestThePasswordHintStartsAtTwelveUnlessTheConfigSaysOtherwise(t *testing.T) {
	saved := Config.Auth.Methods.PasswordAuth.MinLength
	t.Cleanup(func() { Config.Auth.Methods.PasswordAuth.MinLength = saved })
	Config.Auth.Methods.PasswordAuth.MinLength = 0
	if PasswordMinLength() != 12 {
		t.Fatalf("the hint starts at %d", PasswordMinLength())
	}
	Config.Auth.Methods.PasswordAuth.MinLength = 8
	if PasswordMinLength() != 8 {
		t.Fatalf("the configured length gave %d", PasswordMinLength())
	}
}
