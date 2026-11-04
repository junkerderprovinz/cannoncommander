// Package watch reports changes inside the folders clients have open. It
// follows the hub's topics, so a folder is watched exactly while some
// client looks at it.
package watch

// EventChanged is published under hub.FolderTopic(folder) when something
// inside the folder appears, changes or goes away.
const EventChanged = "fs.changed"
