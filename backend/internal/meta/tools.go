package meta

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"os/exec"
	"strings"
	"sync"
	"time"

	"github.com/junkerderprovinz/cannoncommander/backend/internal/archives"
)

// toolTimeout bounds one run of a helper program. A file on a disk that has
// to spin up takes a few seconds, a broken one should not hold a slot.
const toolTimeout = 20 * time.Second

// tools finds the helper programs on the PATH. A program that shows up
// later, after an image update, is found on the next look.
type tools struct {
	mu    sync.Mutex
	paths map[string]string
	at    map[string]time.Time
}

const lookAgain = time.Minute

func (t *tools) find(names ...string) (string, bool) {
	t.mu.Lock()
	defer t.mu.Unlock()
	if t.paths == nil {
		t.paths, t.at = map[string]string{}, map[string]time.Time{}
	}
	key := strings.Join(names, " ")
	if p, ok := t.paths[key]; ok && (p != "" || time.Since(t.at[key]) < lookAgain) {
		return p, p != ""
	}
	p := archives.Find(names...)
	t.paths[key], t.at[key] = p, time.Now()
	return p, p != ""
}

// run starts a helper and returns what it wrote. The exit status is
// returned as an error together with the output, since some tools report a
// broken file with a status but still print what they could read.
func run(ctx context.Context, prog string, args ...string) ([]byte, error) {
	ctx, cancel := context.WithTimeout(ctx, toolTimeout)
	defer cancel()
	cmd := exec.CommandContext(ctx, prog, args...)
	var out, errOut bytes.Buffer
	cmd.Stdout, cmd.Stderr = &out, &errOut
	err := cmd.Run()
	if ctx.Err() != nil {
		return nil, ctx.Err()
	}
	var exit *exec.ExitError
	if errors.As(err, &exit) {
		msg := strings.TrimSpace(errOut.String())
		if len(msg) > 300 {
			msg = msg[:300]
		}
		return out.Bytes(), fmt.Errorf("%s: %w: %s", prog, err, msg)
	}
	return out.Bytes(), err
}
