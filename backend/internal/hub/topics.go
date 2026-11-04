package hub

// FolderTopic is the topic for changes inside one folder. The stream
// subscribes to it for every folder a client has open, and the watcher
// publishes fs.changed under it.
func FolderTopic(path string) string { return "fs:" + path }
