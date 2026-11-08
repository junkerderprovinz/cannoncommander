//go:build !desktop

package trash

// systemTrash is nil: the container keeps every deleted file in the app's own
// trash.
func systemTrash(*Store) systemBin { return nil }
