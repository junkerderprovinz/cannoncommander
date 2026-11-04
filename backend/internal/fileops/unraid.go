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

// kindNames name the jobs in the notices, which Unraid shows as they are.
var kindNames = map[string]string{
	KindCopy: "Kopieren", KindMove: "Verschieben", KindDelete: "Löschen",
	"trash": "In den Papierkorb", "restore": "Wiederherstellen", "emptyTrash": "Papierkorb leeren",
	unraid.FixPermsKind: "Rechte setzen", "undo": "Rückgängig",
}

// notice words a finished job like the prototype does: "Kopieren: Urlaub
// nach backups, nach 12 Min."
func notice(n jobs.Notice) unraid.Notice {
	j := n.Job
	what := kindNames[j.Kind]
	if what == "" {
		what = j.Kind
	}
	label := what + ": " + j.Name
	if j.Count > 1 {
		label = fmt.Sprintf("%s: %d Objekte", what, j.Count)
	}
	if j.Dest != "" {
		label += " nach " + path.Base(j.Dest)
	}
	out := unraid.Notice{Subject: "Aufgabe fertig", Description: label + ", nach " + took(n.Took)}
	if j.State != jobs.Done {
		out.Subject, out.Importance = "Aufgabe fehlgeschlagen", "warning"
		if j.State == jobs.Cancelled {
			out.Subject, out.Importance = "Aufgabe abgebrochen", "normal"
		}
	}
	return out
}

func took(d time.Duration) string {
	switch s := d.Seconds(); {
	case s < 60:
		return fmt.Sprintf("%d s", int(s+0.5))
	case s < 3600:
		return fmt.Sprintf("%d Min.", int(s/60+0.5))
	default:
		return strings.Replace(fmt.Sprintf("%.1f Std.", s/3600), ".", ",", 1)
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
	task, err := unraid.Default().PreparePerms(d.FS, u, p.Path)
	if err != nil {
		return nil, err
	}
	return &jobs.Plan{Params: permsParams{Path: task.Path}, Items: []jobs.Item{{Path: task.Path}}}, nil
}

func runFixPerms(ctx context.Context, run *jobs.Run) error {
	var p permsParams
	if err := run.Params(&p); err != nil {
		return err
	}
	task, err := unraid.Default().ResumePerms(run.Deps().FS, p.Path)
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
