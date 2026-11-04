package unraid

import (
	"context"
	"errors"
	"testing"
)

func gatherFixture(t *testing.T) *env {
	t.Helper()
	e := newEnv(t)
	e.file("disk1", "filme/Sintel/Sintel.mkv", 30)
	e.file("disk2", "filme/Big Buck Bunny/bbb.mkv", 20)
	e.file("disk2", "filme/Big Buck Bunny/poster.jpg", 1)
	e.file("disk3", "filme/Tears of Steel/tos.mkv", 30)
	e.file("cache", "filme/neu.mkv", 2)
	return e
}

func device(p GatherPlan, name string) GatherDevice {
	for _, d := range p.Devices {
		if d.Device == name {
			return d
		}
	}
	return GatherDevice{}
}

func TestGatherSuggestsTheDiskThatMovesLeast(t *testing.T) {
	e := gatherFixture(t)
	plan, err := e.svc.PlanGather(context.Background(), GatherRequest{Path: "/mnt/user/filme", Mode: ModeGather})
	if err != nil {
		t.Fatal(err)
	}
	// disk1 would take as much as disk3 but has only 48 KiB free.
	if plan.Disk != "disk3" || plan.Moves != 3 || plan.Move != 51*1024 || plan.Short != 0 || plan.Cached != 1 || plan.Files != 5 {
		t.Fatalf("plan = %+v", plan)
	}
	if d := device(plan, "disk3"); d.Now != 30*1024 || d.After != 81*1024 {
		t.Errorf("disk3 = %+v", d)
	}
	if d := device(plan, "disk1"); d.After != 0 || !d.Asleep {
		t.Errorf("disk1 = %+v", d)
	}
	if d := device(plan, "cache"); d.Now != 2*1024 || d.After != 2*1024 {
		t.Errorf("the cache stays for the mover: %+v", d)
	}
	if len(plan.MoveList()) != 3 || plan.MoveList()[0].To != "disk3" {
		t.Errorf("moves = %+v", plan.MoveList())
	}
}

func TestGatherOntoAChosenDiskReportsWhatIsShort(t *testing.T) {
	e := gatherFixture(t)
	e.file("disk3", "filme/Elephants Dream/ed.mkv", 40)
	plan, _ := e.svc.PlanGather(context.Background(), GatherRequest{Path: "/mnt/user/filme", Mode: ModeGather, Disk: "disk1"})
	if plan.Disk != "disk1" || plan.Short != (91-48)*1024 {
		t.Errorf("plan = %+v", plan)
	}
}

func TestScatterKeepsSubfoldersTogether(t *testing.T) {
	e := gatherFixture(t)
	plan, err := e.svc.PlanGather(context.Background(), GatherRequest{Path: "/mnt/user/filme", Mode: ModeScatter, Disk: "disk2"})
	if err != nil {
		t.Fatal(err)
	}
	if plan.Moves != 2 || plan.Move != 21*1024 {
		t.Fatalf("plan = %+v", plan)
	}
	for _, m := range plan.MoveList() {
		if m.To != "disk3" {
			t.Errorf("%s goes to %s, the folder should stay whole on disk3", m.Rel, m.To)
		}
	}
	if len(plan.Options) != 3 || plan.Options[0].Disk != "disk1" || plan.Options[1].Disk != "disk2" {
		t.Errorf("options = %+v", plan.Options)
	}
}

func TestGatherNeedsAUserShareFolder(t *testing.T) {
	e := gatherFixture(t)
	for _, p := range []string{"/mnt/disk1/filme", "/mnt/user", "/mnt/remotes/x"} {
		if _, err := e.svc.PlanGather(context.Background(), GatherRequest{Path: p, Mode: ModeGather}); !errors.Is(err, ErrNotUserFolder) {
			t.Errorf("%s: %v", p, err)
		}
	}
	if _, err := e.svc.PlanGather(context.Background(), GatherRequest{Path: "/mnt/user/filme", Mode: "spread"}); !errors.Is(err, errBadMode) {
		t.Errorf("bad mode: %v", err)
	}
}
