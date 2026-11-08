//go:build !windows

package fsapi

// labelOf has nothing to add: only Explorer names folders other than they
// are called.
func labelOf(string, uint32) string { return "" }
