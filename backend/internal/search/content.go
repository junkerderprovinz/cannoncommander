package search

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path"
	"strings"
	"sync"
	"time"
	"unicode/utf8"

	"github.com/gtsteffaniak/go-logger/logger"

	v1 "github.com/junkerderprovinz/cannoncommander/backend/internal/api/v1"
	"github.com/junkerderprovinz/cannoncommander/backend/internal/database/users"
	"github.com/junkerderprovinz/cannoncommander/backend/internal/jobs"
)

// KindContent is the job that looks for a text inside files.
const KindContent = "contentSearch"

func init() {
	jobs.RegisterKind(KindContent, jobs.Kind{Prepare: prepareContent, Run: runContent, BigResult: true})
}

// ContentParams are the criteria of the search window together with the
// text to find. Upper and lower case do not count.
type ContentParams struct {
	Params
	Text string `json:"text"`
}

// ContentResult is what a finished content search keeps: the first hits
// in the window's order and how many there were.
type ContentResult struct {
	Items   []Hit `json:"items"`
	Total   int   `json:"total"`
	Scanned int   `json:"scanned"`
}

// hitsEvent carries new hits of a running content search.
type hitsEvent struct {
	Job     string `json:"job"`
	Items   []Hit  `json:"items"`
	Total   int    `json:"total"`
	Scanned int    `json:"scanned"`
	Done    bool   `json:"done,omitempty"`
}

func prepareContent(_ context.Context, d v1.Deps, u *users.User, raw json.RawMessage) (*jobs.Plan, error) {
	var p ContentParams
	if err := jobs.Decode(raw, &p); err != nil {
		return nil, err
	}
	if strings.TrimSpace(p.Text) == "" {
		return nil, v1.Errorf(http.StatusBadRequest, "text is required")
	}
	loc, err := checkParams(d.FS, u, p.Params)
	if err != nil {
		return nil, err
	}
	if unknown := Parse(p.Q, time.Now()).Unknown; len(unknown) > 0 {
		return nil, v1.Errorf(http.StatusBadRequest, "these fields are unknown: "+strings.Join(unknown, ", "))
	}
	p.Path = loc.Path
	return &jobs.Plan{
		Params: p,
		Name:   "„" + p.Text + "“",
		Dest:   loc.Path,
		Items:  []jobs.Item{{Path: loc.Path, Name: path.Base(loc.Path)}},
	}, nil
}

func runContent(ctx context.Context, run *jobs.Run) error {
	var p ContentParams
	if err := run.Params(&p); err != nil {
		return err
	}
	it := run.Items()[0]
	if it.State != jobs.ItemPending {
		return nil
	}
	u, err := run.User()
	if err != nil {
		return fmt.Errorf("the user who started this search is gone: %w", err)
	}
	d := run.Deps()
	loc, err := checkParams(d.FS, u, p.Params)
	if err != nil {
		run.Finish(it.Seq, jobs.ItemFailed, err)
		return nil
	}
	run.Begin(it.Seq)

	// A first walk counts what the second one opens, for the progress bar.
	// It reads the index, so it is quick next to opening the files.
	count := newFinder(d.FS, u, Params{Path: p.Path, Q: nameOnly(p.Q), Hidden: p.Hidden, Type: p.Type, Min: p.Min, Max: p.Max, When: p.When}, time.Now())
	var files, size int64
	if err := count.run(ctx, loc, func(r *row) error {
		if !r.Dir {
			files++
			size += r.Size
		}
		return nil
	}); err != nil {
		return err
	}
	run.Measure(it.Seq, size, files)

	needle := []byte(strings.ToLower(p.Text))
	f := newFinder(d.FS, u, p.Params, time.Now())
	if tags := f.q.Tags(); len(tags) > 0 {
		f.tags = taggedPaths(ctx, d.DB, tags)
	}
	f.readUntil = time.Now().Add(time.Hour)
	f.extra = func(ctx context.Context, r *row) (bool, error) {
		if r.Dir {
			return false, nil
		}
		run.Current(strings.TrimPrefix(strings.TrimPrefix(r.Path, loc.Path), "/"))
		if err := run.Wait(ctx); err != nil {
			return false, err
		}
		line, found, err := findIn(ctx, r.Real, needle)
		if err != nil && ctx.Err() != nil {
			return false, ctx.Err()
		}
		if err := run.Advance(ctx, r.Size, 1); err != nil {
			return false, err
		}
		r.line = line
		return found, nil
	}
	pub := &publisher{run: run}
	c := newCollector(DefaultLimit)
	err = f.run(ctx, loc, func(r *row) error {
		c.add(r)
		h := hitOf(r)
		h.Line = r.line
		pub.add(h, c.total, f.scanned)
		return nil
	})
	if err != nil {
		return err
	}
	c.trim()
	res := ContentResult{Items: []Hit{}, Total: c.total, Scanned: f.scanned}
	for _, r := range c.rows {
		h := hitOf(r)
		h.Line = r.line
		res.Items = append(res.Items, h)
	}
	pub.done(c.total, f.scanned)
	run.SetResult(res)
	run.Finish(it.Seq, jobs.ItemDone, nil)
	return nil
}

// nameOnly drops the field tests from a search line, for the counting
// walk that must not read metadata.
func nameOnly(line string) string {
	var keep []string
	for _, m := range token.FindAllStringSubmatch(line, -1) {
		if m[1] == "" {
			keep = append(keep, m[0])
		}
	}
	return strings.Join(keep, " ")
}

// sniff is how much of a file decides whether it holds text. A zero byte
// in it marks a binary file, which a text search passes over.
const sniff = 8 << 10

// findIn looks for needle, in lower case, in the file and returns the line
// of the first match.
func findIn(ctx context.Context, real string, needle []byte) (line int, found bool, err error) {
	fh, err := os.Open(real)
	if err != nil {
		return 0, false, err
	}
	defer fh.Close()
	buf := make([]byte, 1<<20)
	var carry, tail []byte
	lines := 0
	first := true
	for {
		if err := ctx.Err(); err != nil {
			return 0, false, err
		}
		n, rerr := fh.Read(buf)
		chunk := append(carry, buf[:n]...)
		if first && n > 0 {
			first = false
			if bytes.IndexByte(chunk[:min(len(chunk), sniff)], 0) >= 0 {
				return 0, false, nil
			}
		}
		carry = nil
		if rerr == nil {
			// Keep a character cut off at the end for the next round, so
			// lower casing sees it whole.
			if i := lastRuneStart(chunk); i < len(chunk) && !utf8.FullRune(chunk[i:]) {
				carry = append([]byte(nil), chunk[i:]...)
				chunk = chunk[:i]
			}
		}
		window := append(tail, bytes.ToLower(chunk)...)
		if i := bytes.Index(window, needle); i >= 0 {
			return lines + bytes.Count(window[:i], []byte{'\n'}) + 1, true, nil
		}
		keep := min(len(window), len(needle)-1)
		lines += bytes.Count(window[:len(window)-keep], []byte{'\n'})
		tail = append([]byte(nil), window[len(window)-keep:]...)
		if rerr == io.EOF {
			return 0, false, nil
		}
		if rerr != nil {
			return 0, false, rerr
		}
	}
}

func lastRuneStart(b []byte) int {
	for i := len(b) - 1; i >= 0 && i >= len(b)-utf8.UTFMax; i-- {
		if utf8.RuneStart(b[i]) {
			return i
		}
	}
	return len(b)
}

// publisher sends new hits to the user who started the search, a batch at
// a time, as the progress of a job goes out.
type publisher struct {
	run  *jobs.Run
	mu   sync.Mutex
	hits []Hit
	at   time.Time
}

const publishEvery = 250 * time.Millisecond

func (p *publisher) add(h Hit, total, scanned int) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.hits = append(p.hits, h)
	if len(p.hits) >= 100 || time.Since(p.at) >= publishEvery {
		p.send(total, scanned, false)
	}
}

func (p *publisher) done(total, scanned int) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.send(total, scanned, true)
}

func (p *publisher) send(total, scanned int, done bool) {
	p.at = time.Now()
	h := p.run.Deps().Hub
	if h == nil {
		p.hits = nil
		return
	}
	ev := hitsEvent{Job: p.run.ID(), Items: p.hits, Total: total, Scanned: scanned, Done: done}
	if ev.Items == nil {
		ev.Items = []Hit{}
	}
	if _, err := h.PublishJSON(EventHits, "", ev, p.run.Owner()); err != nil {
		logger.Errorf("search: %v", err)
	}
	p.hits = nil
}
