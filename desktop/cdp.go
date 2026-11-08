//go:build cdp

package main

import (
	"os"
	"strconv"
)

// debugArgs opens WebView2's DevTools protocol on the loopback port in
// CC_CDP_PORT, so a live test can drive the window. Only a build with the cdp
// tag has it; a release build never opens the port.
func debugArgs() []string {
	port, err := strconv.Atoi(os.Getenv("CC_CDP_PORT"))
	if err != nil || port < 1 || port > 65535 {
		return nil
	}
	return []string{"--remote-debugging-port=" + strconv.Itoa(port)}
}
