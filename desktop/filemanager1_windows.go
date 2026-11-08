package main

// offerFolders has nothing to do on Windows, where the registry alone sends
// the folders here.
func offerFolders(*shell, bool) {}
