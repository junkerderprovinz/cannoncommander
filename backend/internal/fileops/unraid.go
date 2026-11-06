package fileops

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"path"
	"strings"
	"time"

	"github.com/gtsteffaniak/go-logger/logger"

	v1 "github.com/junkerderprovinz/cannoncommander/backend/internal/api/v1"
	"github.com/junkerderprovinz/cannoncommander/backend/internal/database/users"
	"github.com/junkerderprovinz/cannoncommander/backend/internal/jobs"
	"github.com/junkerderprovinz/cannoncommander/backend/internal/unraid"
)

func init() {
	jobs.RegisterKind(unraid.FixPermsKind, jobs.Kind{Prepare: prepareFixPerms, Run: runFixPerms})
}

// connectUnraid lets big jobs wait for a parity check and reports long
// jobs to the Unraid notifications.
func connectUnraid(m *jobs.Manager, u *unraid.Service) {
	m.SetParityCheck(u.ParityBusy)
	m.OnFinish(func(n jobs.Notice) {
		s := m.Settings()
		if !s.NotifyUnraid || n.Took < time.Duration(s.NotifyAfter)*time.Second {
			return
		}
		if err := u.Notify(notice(n)); err != nil && !errors.Is(err, unraid.ErrWriteOff) {
			logger.Errorf("jobs: notice to Unraid: %v", err)
		}
	})
}

// kindNames name the jobs in the notices. Unraid shows them as they are and
// knows nothing of the app's languages, so they are in English, as in the
// English interface.
var kindNames = map[string]string{
	KindCopy: "Copy", KindMove: "Move", KindDelete: "Delete",
	"trash": "To the trash", "restore": "Restore", "emptyTrash": "Delete for good",
	unraid.FixPermsKind: "Repair permissions", "undo": "Take back",
	unraid.ModeGather: "Gather", unraid.ModeScatter: "Empty",
}

// notice words a finished job like the interface does: "Copy: Urlaub to
// backups, after 12 min".
func notice(n jobs.Notice) unraid.Notice {
	j := n.Job
	what := kindNames[j.Kind]
	if what == "" {
		what = j.Kind
	}
	label := what + ": " + j.Name
	if j.Count > 1 {
		label = fmt.Sprintf("%s: %d items", what, j.Count)
	}
	if j.Dest != "" {
		to := " to "
		switch j.Kind {
		case unraid.ModeGather:
			to = " onto "
		case unraid.ModeScatter:
			to = " from "
		}
		label += to + path.Base(j.Dest)
	}
	out := unraid.Notice{Subject: "Job done", Description: label + ", after " + took(n.Took)}
	if j.State != jobs.Done {
		out.Subject, out.Importance = "Job failed", "warning"
		if j.State == jobs.Cancelled {
			out.Subject, out.Importance = "Job cancelled", "normal"
		}
	}
	return out
}

func took(d time.Duration) string {
	switch s := d.Seconds(); {
	case s < 60:
		return fmt.Sprintf("%d s", int(s+0.5))
	case s < 3600:
		return fmt.Sprintf("%d min", int(s/60+0.5))
	default:
		return fmt.Sprintf("%.1f h", s/3600)
	}
}

type permsParams struct {
	Path string `json:"path"`
}

func prepareFixPerms(_ context.Context, d v1.Deps, u *users.User, raw json.RawMessage) (*jobs.Plan, error) {
	var p permsParams
	if err := jobs.Decode(raw, &p); err != nil {
		return nil, err
	}
	task, err := unraidService().PreparePerms(d.FS, u, p.Path)
	if err != nil {
		return nil, err
	}
	return &jobs.Plan{Params: permsParams{Path: task.Path}, Items: []jobs.Item{{Path: task.Path}}}, nil
}

func runFixPerms(ctx context.Context, run *jobs.Run) error {
	if err := run.RequireAdmin(); err != nil {
		return err
	}
	var p permsParams
	if err := run.Params(&p); err != nil {
		return err
	}
	task, err := unraidService().ResumePerms(run.Deps().FS, p.Path)
	if err != nil {
		return err
	}
	it := run.Items()[0]
	if loc, err := run.Deps().FS.Resolve(task.Path); err == nil {
		if _, files, err := jobs.TreeSize(ctx, loc.Real); err == nil {
			run.Measure(it.Seq, 0, files)
		}
	}
	run.Begin(it.Seq)
	res, err := task.Run(ctx, func(apiPath string) error {
		run.Current(strings.TrimPrefix(strings.TrimPrefix(apiPath, task.Path), "/"))
		return run.Advance(ctx, 0, 1)
	})
	if ctx.Err() != nil {
		return ctx.Err()
	}
	run.SetResult(res)
	if err == nil && res.Failed > 0 {
		err = fmt.Errorf("%d entries could not be changed: %s", res.Failed, res.FirstError)
	}
	run.Finish(it.Seq, jobs.ItemDone, err)
	return nil
}
