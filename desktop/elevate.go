package main

import (
	"flag"
	"os"

	"github.com/junkerderprovinz/cannoncommander/backend/pkg/elevate"
)

var elevated = flag.String("elevated", "", "do one job with administrator rights for the window that started this copy, then exit")

// serveElevated is this copy's whole life when the window started it through
// UAC or pkexec: no window, no server, one job.
func serveElevated() error { return elevate.Serve(*elevated, os.Stdin) }
