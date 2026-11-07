package terminal

import "errors"

var errNoShell = errors.New("a terminal needs a Linux server")

func start(string, size) (tty, error) { return nil, errNoShell }
