package ccfs

// WindowsNames reports whether names in dir follow the rules of Windows.
// On Windows they always do.
func WindowsNames(string) bool { return true }
