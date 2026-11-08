package system

import "testing"

func TestFFmpegReadsAsCheckingWhileTheDesktopLooksForIt(t *testing.T) {
	looking := true
	tb := toolbox{checking: func() bool { return looking }}
	got := tb.get()
	for _, name := range []string{"ffmpeg", "ffprobe"} {
		if !got[name].Checking || got[name].Available {
			t.Errorf("%s while looking: %+v", name, got[name])
		}
	}
	if got["rclone"].Checking {
		t.Error("rclone does not wait for ffmpeg")
	}

	looking = false
	got = tb.get()
	for _, name := range []string{"ffmpeg", "ffprobe"} {
		if got[name].Checking {
			t.Errorf("%s still checking after the look: %+v", name, got[name])
		}
	}
}
