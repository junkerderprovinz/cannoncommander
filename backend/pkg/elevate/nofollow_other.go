//go:build !unix

package elevate

// noFollow is nothing where opening through a handle already refuses what
// leads out of it.
const noFollow = 0
