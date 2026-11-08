//go:build unix

package elevate

import "syscall"

// noFollow keeps an open from following a link that took the place of a file.
const noFollow = syscall.O_NOFOLLOW
