package archives

import (
	"errors"
	"fmt"
	"io"
)

// Damage describes the first member of an archive that does not read back.
type Damage struct {
	Entry  string `json:"entry,omitempty"`
	Reason string `json:"reason"`
}

func (d *Damage) Error() string {
	if d.Entry == "" {
		return d.Reason
	}
	return d.Entry + ": " + d.Reason
}

// Test reads every member of the archive at file to its end, which makes
// every format check its checksums, and unpacks nothing. It returns the
// number of members read. A damaged archive comes back as a *Damage, a
// missing password as ErrPassword.
func Test(file string, o Options, wrote func(n int) error) (int, error) {
	a, err := Open(file, o)
	if err != nil {
		if errors.Is(err, ErrPassword) {
			return 0, err
		}
		return 0, &Damage{Reason: err.Error()}
	}
	defer a.Close()
	members := 0
	var stop error
	buf := make([]byte, 1<<20)
	err = a.Walk(func(e Entry, open Opener) error {
		members++
		if e.Dir || e.Symlink() || e.HardLink != "" {
			return nil
		}
		rc, err := open()
		if err != nil {
			return memberError(e, err)
		}
		defer rc.Close()
		var n int64
		for {
			k, rerr := rc.Read(buf)
			n += int64(k)
			if k > 0 && wrote != nil {
				if err := wrote(k); err != nil {
					stop = err
					return err
				}
			}
			if errors.Is(rerr, io.EOF) {
				break
			}
			if rerr != nil {
				return memberError(e, rerr)
			}
		}
		if n != e.Size && e.Size > 0 {
			return &Damage{Entry: e.Name, Reason: fmt.Sprintf("%d of %d bytes", n, e.Size)}
		}
		return nil
	})
	if stop != nil {
		return members, stop
	}
	if err != nil {
		var d *Damage
		if errors.As(err, &d) || errors.Is(err, ErrPassword) {
			return members, err
		}
		return members, &Damage{Reason: err.Error()}
	}
	return members, nil
}

func memberError(e Entry, err error) error {
	if errors.Is(err, ErrPassword) {
		return err
	}
	return &Damage{Entry: e.Name, Reason: err.Error()}
}
