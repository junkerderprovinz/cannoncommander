//go:build !windows

package update

// shellChanged has nothing to tell outside Windows, where the menus read the
// program's icon afresh.
var shellChanged = func(string) {}
