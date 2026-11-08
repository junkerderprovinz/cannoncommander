//go:build !windows && !linux

package launch

import "os/exec"

type openSystem struct{}

func newSystem() system { return openSystem{} }

// asks is false: the Finder starts a program on a double click.
func (openSystem) asks(string) bool { return false }

func (openSystem) open(file string) error { return exec.Command("open", file).Run() }

func (openSystem) run(file string) error { return exec.Command("open", file).Run() }
