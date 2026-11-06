package fileops

import (
	"testing"
	"time"

	"github.com/junkerderprovinz/cannoncommander/backend/internal/jobs"
)

func TestNoticesReadLikeTheInterface(t *testing.T) {
	for _, tc := range []struct {
		job  jobs.Job
		took time.Duration
		want string
	}{
		{jobs.Job{Kind: KindCopy, State: jobs.Done, Name: "Urlaub", Count: 1, Dest: "/mnt/user/backups"}, 12 * time.Minute, "Copy: Urlaub to backups, after 12 min"},
		{jobs.Job{Kind: KindMove, State: jobs.Failed, Name: "a", Count: 3, Dest: "/mnt/user/filme"}, 90 * time.Minute, "Move: 3 items to filme, after 1.5 h"},
		{jobs.Job{Kind: "trash", State: jobs.Done, Name: "alt.txt", Count: 1}, 42 * time.Second, "To the trash: alt.txt, after 42 s"},
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
