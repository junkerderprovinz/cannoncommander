package fileops

import (
	"testing"
	"time"

	"github.com/junkerderprovinz/cannoncommander/backend/internal/jobs"
)

func TestNoticesReadLikeThePrototype(t *testing.T) {
	for _, tc := range []struct {
		job  jobs.Job
		took time.Duration
		want string
	}{
		{jobs.Job{Kind: KindCopy, State: jobs.Done, Name: "Urlaub", Count: 1, Dest: "/mnt/user/backups"}, 12 * time.Minute, "Kopieren: Urlaub nach backups, nach 12 Min."},
		{jobs.Job{Kind: KindMove, State: jobs.Failed, Name: "a", Count: 3, Dest: "/mnt/user/filme"}, 90 * time.Minute, "Verschieben: 3 Objekte nach filme, nach 1,5 Std."},
		{jobs.Job{Kind: "trash", State: jobs.Done, Name: "alt.txt", Count: 1}, 42 * time.Second, "In den Papierkorb: alt.txt, nach 42 s"},
	} {
		n := notice(jobs.Notice{Job: tc.job, Took: tc.took})
		if n.Description != tc.want {
			t.Errorf("%q, want %q", n.Description, tc.want)
		}
		if (tc.job.State == jobs.Failed) != (n.Importance == "warning") {
			t.Errorf("%s: importance %q", tc.job.State, n.Importance)
		}
	}
}
