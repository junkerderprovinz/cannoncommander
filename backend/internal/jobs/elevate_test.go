//go:build desktop

package jobs

import (
	"context"
	"testing"

	v1 "github.com/junkerderprovinz/cannoncommander/backend/internal/api/v1"
	"github.com/junkerderprovinz/cannoncommander/backend/pkg/desktop"
)

func init() {
	RegisterKind("elevatable", Kind{Run: func(ctx context.Context, run *Run) error {
		for _, it := range run.Items() {
			if it.State == ItemPending {
				run.Finish(it.Seq, ItemDone, v1.Coded(403, "denied", "access denied"))
			}
		}
		return nil
	}})
	Elevates("elevatable", func(ctx context.Context, run *Run) error {
		for _, it := range run.Items() {
			if it.State == ItemPending {
				run.Finish(it.Seq, ItemDone, nil)
			}
		}
		return nil
	})
}

type inWindow struct{ desktop.App }

func inDesktop(t *testing.T) {
	desktop.Use(inWindow{}, desktop.Options{})
	t.Cleanup(func() { desktop.Use(nil, desktop.Options{}) })
}

func TestADeniedJobGoesOnWithAdministratorRights(t *testing.T) {
	inDesktop(t)
	e := newEnv(t)
	j := e.submit(t, "elevatable", "", 2)
	failed := e.waitState(t, j.ID, Failed)
	if failed.Items[0].ErrorCode != "denied" {
		t.Fatalf("failed job: %+v", failed)
	}
	if _, err := e.m.Elevate(j.ID); err != nil {
		t.Fatal(err)
	}
	if done := e.waitState(t, j.ID, Done); done.FailedItems != 0 {
		t.Fatalf("elevated job: %+v", done)
	}
}

func TestOnlyKindsThatKnowHowGoOnWithAdministratorRights(t *testing.T) {
	inDesktop(t)
	e := newEnv(t)
	name := addScript(t, func(ctx context.Context, run *Run) error {
		run.Finish(0, ItemDone, v1.Coded(403, "denied", "access denied"))
		return nil
	})
	j := e.submit(t, "test", name, 1)
	e.waitState(t, j.ID, Failed)
	if _, err := e.m.Elevate(j.ID); err == nil {
		t.Fatal("a kind without an elevation went on")
	}
}

func TestTheServerOnItsOwnNeverElevates(t *testing.T) {
	e := newEnv(t)
	j := e.submit(t, "elevatable", "", 1)
	e.waitState(t, j.ID, Failed)
	if _, err := e.m.Elevate(j.ID); err == nil {
		t.Fatal("the container went on with administrator rights")
	}
}
