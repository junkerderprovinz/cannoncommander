package modules

import (
	"fmt"
	"io"
	"strconv"
)

// PrintProgress returns a Report for Installer that writes "progress
// <fraction>" and "status <text>" lines to w, which an installer reads to
// show the install of the modules as a step of its own.
func PrintProgress(w io.Writer) func(fraction float64, status string) {
	return func(fraction float64, status string) {
		fmt.Fprintln(w, "progress", strconv.FormatFloat(fraction, 'f', 2, 64))
		if status != "" {
			fmt.Fprintln(w, "status", status)
		}
	}
}
