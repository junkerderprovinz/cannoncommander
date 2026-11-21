//go:build !windows

package update

// ShellChanged has nothing to tell outside Windows, where the menus read the
// program's icon afresh.
var ShellChanged = func(string) {}
