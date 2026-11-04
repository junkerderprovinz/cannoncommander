package accounts

// CatchResetCodes sends reset codes to f instead of the log until the
// returned func puts the log back.
func CatchResetCodes(f func(username, code string)) func() {
	old := announce
	announce = f
	return func() { announce = old }
}
