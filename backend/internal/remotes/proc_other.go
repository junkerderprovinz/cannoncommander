//go:build !linux

package remotes

import (
	"os"
	"syscall"
)

func childAttr() *syscall.SysProcAttr { return nil }

func stopChild(p *os.Process) error { return p.Kill() }

func clearStale(string) {}
