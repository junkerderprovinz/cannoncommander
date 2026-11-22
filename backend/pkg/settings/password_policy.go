package settings

import "unicode/utf8"

// PasswordMinLength is the length below which the sign-in points out that a
// password is short. It never refuses one.
func PasswordMinLength() int {
	if n := Config.Auth.Methods.PasswordAuth.MinLength; n > 0 {
		return n
	}
	return 12
}

// ShortPassword reports whether pw is under PasswordMinLength, for an answer
// that takes it to say so.
func ShortPassword(pw string) bool {
	return pw != "" && utf8.RuneCountInString(pw) < PasswordMinLength()
}
