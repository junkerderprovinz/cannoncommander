package tools

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"regexp"
	"strconv"
	"strings"
	"time"

	v1 "github.com/junkerderprovinz/cannoncommander/backend/internal/api/v1"
	"github.com/junkerderprovinz/cannoncommander/backend/internal/ccfs"
	"github.com/junkerderprovinz/cannoncommander/backend/internal/system"
)

// ArrowLoopRequest hands a pair of folders to ArrowLoop as a job that
// runs on a schedule. Direction, Mirror and the folders mean what they
// mean for a sync here.
type ArrowLoopRequest struct {
	Name      string `json:"name"`
	Left      string `json:"left"`
	Right     string `json:"right"`
	Direction string `json:"direction"`
	Mirror    bool   `json:"mirror,omitempty"`
	// Every is hourly, daily or weekly, At the time of day as 03:00.
	Every string `json:"every"`
	At    string `json:"at,omitempty"`
}

// ArrowLoopConfig is where ArrowLoop is and how to reach it.
type ArrowLoopConfig struct {
	URL   string
	Token string
	// Paths maps the start of a path here to the same folder as
	// ArrowLoop's container names it.
	Paths map[string]string
}

func (h handlers) savedArrowLoop(ctx context.Context) (ArrowLoopConfig, error) {
	app, err := system.Settings(ctx, h.deps)
	if err != nil {
		return ArrowLoopConfig{}, err
	}
	c := app.ArrowLoop
	return ArrowLoopConfig{URL: c.URL, Token: c.Token, Paths: c.Paths}, nil
}

// mapPath names p as ArrowLoop sees it, by the longest matching prefix.
func (c ArrowLoopConfig) mapPath(p string) string {
	best, to := "", ""
	for from, there := range c.Paths {
		from = strings.TrimSuffix(from, "/")
		if (p == from || strings.HasPrefix(p, from+"/")) && len(from) > len(best) {
			best, to = from, strings.TrimSuffix(there, "/")
		}
	}
	if best == "" {
		return p
	}
	return to + strings.TrimPrefix(p, best)
}

var clock = regexp.MustCompile(`^([01]?\d|2[0-3]):([0-5]\d)$`)

// Cron turns the choice of the dialog into the schedule ArrowLoop reads.
// A weekly run goes on Mondays.
func Cron(every, at string) (string, error) {
	if at == "" {
		at = "03:00"
	}
	m := clock.FindStringSubmatch(at)
	if m == nil {
		return "", badRequest("the time has to be written as 03:00")
	}
	hour, _ := strconv.Atoi(m[1])
	minute, _ := strconv.Atoi(m[2])
	switch every {
	case "hourly":
		return fmt.Sprintf("%d * * * *", minute), nil
	case "daily":
		return fmt.Sprintf("%d %d * * *", minute, hour), nil
	case "weekly":
		return fmt.Sprintf("%d %d * * 1", minute, hour), nil
	}
	return "", badRequest("every must be hourly, daily or weekly")
}

// arrowLoopJob is the job as ArrowLoop's POST /api/jobs takes it.
func (c ArrowLoopConfig) job(req ArrowLoopRequest, left, right string) (map[string]any, error) {
	schedule, err := Cron(req.Every, req.At)
	if err != nil {
		return nil, err
	}
	direction, ok := map[string]string{"right": "leftToRight", "left": "rightToLeft", "both": "both"}[req.Direction]
	if !ok {
		return nil, badRequest("direction must be right, left or both")
	}
	mode := "sync"
	if req.Mirror {
		if req.Direction == "both" {
			return nil, badRequest("mirroring needs one direction")
		}
		mode = "mirror"
	}
	return map[string]any{
		"name": req.Name, "left": c.mapPath(left), "right": c.mapPath(right),
		"direction": direction, "mode": mode, "schedule": schedule,
	}, nil
}

var arrowLoopClient = &http.Client{Timeout: 20 * time.Second}

// Create sends the job and returns it as ArrowLoop stored it.
func (c ArrowLoopConfig) Create(ctx context.Context, job map[string]any) (map[string]any, error) {
	if c.URL == "" {
		return nil, v1.Errorf(http.StatusServiceUnavailable, "ArrowLoop is not set up under Integrations")
	}
	body, err := json.Marshal(job)
	if err != nil {
		return nil, err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, strings.TrimSuffix(c.URL, "/")+"/api/jobs", bytes.NewReader(body))
	if err != nil {
		return nil, v1.Errorf(http.StatusServiceUnavailable, "the ArrowLoop address is not valid")
	}
	req.Header.Set("Content-Type", "application/json")
	if c.Token != "" {
		req.Header.Set("X-ArrowLoop-Token", c.Token)
	}
	resp, err := arrowLoopClient.Do(req)
	if err != nil {
		return nil, v1.Errorf(http.StatusBadGateway, "ArrowLoop cannot be reached")
	}
	defer resp.Body.Close()
	raw, _ := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	var out map[string]any
	_ = json.Unmarshal(raw, &out)
	switch {
	case resp.StatusCode == http.StatusCreated:
		return out, nil
	case resp.StatusCode == http.StatusUnauthorized:
		return nil, v1.Errorf(http.StatusBadGateway, "ArrowLoop does not accept the token")
	case resp.StatusCode == http.StatusConflict:
		return nil, v1.Errorf(http.StatusConflict, "ArrowLoop has a job of this name already")
	case resp.StatusCode == http.StatusNotFound || resp.StatusCode == http.StatusMethodNotAllowed:
		return nil, v1.Errorf(http.StatusBadGateway, "this ArrowLoop is too old to take jobs, it needs an update")
	}
	msg, _ := out["error"].(string)
	if msg == "" {
		msg = resp.Status
	}
	return nil, v1.Errorf(http.StatusBadGateway, "ArrowLoop refused the job: "+msg)
}

func (h handlers) arrowLoop(w http.ResponseWriter, r *http.Request) error {
	u := v1.UserFrom(r)
	var req ArrowLoopRequest
	if err := v1.DecodeJSON(r, &req); err != nil {
		return err
	}
	req.Name = strings.TrimSpace(req.Name)
	if req.Name == "" {
		return badRequest("the job needs a name")
	}
	// ArrowLoop writes with its own rights, so the user has to hold here
	// what the job will do there.
	write := ccfs.View | ccfs.Create | ccfs.Modify
	if req.Mirror {
		write |= ccfs.Delete
	}
	read := ccfs.View | ccfs.Download
	lperm, rperm := read, write
	switch req.Direction {
	case "left":
		lperm, rperm = write, read
	case "both":
		lperm, rperm = read|write, read|write
	}
	l, err := folder(h.fs, u, req.Left, lperm)
	if err != nil {
		return err
	}
	rr, err := folder(h.fs, u, req.Right, rperm)
	if err != nil {
		return err
	}
	if l.Path == rr.Path {
		return badRequest("both sides are the same folder")
	}
	// ArrowLoop knows nothing about the rules, so it gets no folder a rule
	// names something in.
	if err := h.fs.Whole(r.Context(), u, l, lperm); err != nil {
		return err
	}
	if err := h.fs.Whole(r.Context(), u, rr, rperm); err != nil {
		return err
	}
	c, err := h.arrowLoopConfig(r.Context())
	if err != nil {
		return err
	}
	job, err := c.job(req, l.Path, rr.Path)
	if err != nil {
		return err
	}
	stored, err := c.Create(r.Context(), job)
	if err != nil {
		return err
	}
	return v1.WriteJSON(w, r, http.StatusCreated, map[string]any{"job": stored, "url": c.URL})
}
