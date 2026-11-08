package system

import (
	"context"
	"os"
	"os/exec"
	"regexp"
	"runtime"
	"sync"
	"time"

	"github.com/junkerderprovinz/cannoncommander/backend/internal/archives"
	"github.com/junkerderprovinz/cannoncommander/backend/internal/ffmpeg"
	"github.com/junkerderprovinz/cannoncommander/backend/internal/unraid"
	"github.com/junkerderprovinz/cannoncommander/backend/internal/version"
	"github.com/junkerderprovinz/cannoncommander/backend/pkg/desktop"
	"github.com/junkerderprovinz/cannoncommander/backend/pkg/settings"
)

var started = time.Now()

// Info is what GET /system answers.
type Info struct {
	Version string `json:"version"`
	Commit  string `json:"commit,omitempty"`
	// Deployment is "desktop" inside the desktop app and "container"
	// everywhere else.
	Deployment string          `json:"deployment"`
	Release    bool            `json:"release"`
	Go         string          `json:"go"`
	OS         string          `json:"os"`
	Arch       string          `json:"arch"`
	Started    time.Time       `json:"started"`
	Uptime     int64           `json:"uptime"`
	Tools      map[string]Tool `json:"tools"`
	Unraid     UnraidInfo      `json:"unraid"`
	Feature    Features        `json:"features"`
}

// Tool is a helper program the server runs.
type Tool struct {
	Available bool   `json:"available"`
	Path      string `json:"path,omitempty"`
	Version   string `json:"version,omitempty"`
	// Checking is set while the desktop app still looks for ffmpeg.
	Checking bool `json:"checking,omitempty"`
}

// UnraidInfo says whether the server runs on Unraid and whether the switch
// for writing to Unraid is on.
type UnraidInfo struct {
	Available bool `json:"available"`
	Socket    bool `json:"socket"`
}

// Features tells the clients what the server does, so the viewers know
// where office files open.
type Features struct {
	Office OfficeInfo `json:"office"`
	Thumbs bool       `json:"thumbs"`
	Video  bool       `json:"videoPreview"`
	WebDAV bool       `json:"webdav"`
	MuPDF  bool       `json:"mupdf"`
	// Terminal says whether admins may open a terminal. The desktop app
	// opens the computer's own terminal and needs no switch for it.
	Terminal bool `json:"terminal"`
}

// OfficeInfo is Office without the secret.
type OfficeInfo struct {
	Kind string `json:"kind"`
	URL  string `json:"url,omitempty"`
}

type probe struct {
	name string
	args []string
	path func() string
	// ffmpeg marks the programs whose path comes from the ffmpeg service.
	ffmpeg bool
}

// probes are the helper programs and how they print their version.
var probes = []probe{
	{"ffmpeg", []string{"-version"}, func() string { return ffmpeg.Get().FFmpegPath() }, true},
	{"ffprobe", []string{"-version"}, func() string { return ffmpeg.Get().FFprobePath() }, true},
	{"mutool", []string{"-v"}, nil, false},
	{"7zz", nil, func() string { return archives.Find("7zz", "7z") }, false},
	{"rclone", []string{"version"}, RcloneBinary, false},
}

// RcloneBinary returns the rclone program, CC_RCLONE_BINARY or the one on
// the PATH, or "" when there is none.
func RcloneBinary() string {
	if p := os.Getenv("CC_RCLONE_BINARY"); p != "" {
		return p
	}
	p, _ := exec.LookPath("rclone")
	return p
}

var versionNumber = regexp.MustCompile(`\d+\.\d+(?:\.\d+)*`)

// toolbox finds the helper programs once. They come with the image and do
// not change while the server runs. The desktop app looks for ffmpeg after
// its window opens, and until it has, the answer is not kept.
type toolbox struct {
	mu    sync.Mutex
	tools map[string]Tool
	// checking reports whether that look still runs; nil means
	// ffmpeg.Checking.
	checking func() bool
}

func (t *toolbox) get() map[string]Tool {
	t.mu.Lock()
	defer t.mu.Unlock()
	if t.tools != nil {
		return t.tools
	}
	checking := ffmpeg.Checking
	if t.checking != nil {
		checking = t.checking
	}
	pending := checking()
	tools := map[string]Tool{}
	var mu sync.Mutex
	var wg sync.WaitGroup
	for _, p := range probes {
		wg.Go(func() {
			tool := Tool{Checking: true}
			if !pending || !p.ffmpeg {
				tool = find(p)
			}
			mu.Lock()
			tools[p.name] = tool
			mu.Unlock()
		})
	}
	wg.Wait()
	if !pending {
		t.tools = tools
	}
	return tools
}

func find(p probe) Tool {
	path := ""
	if p.path != nil {
		path = p.path()
	}
	if path == "" {
		path, _ = exec.LookPath(p.name)
	}
	if path == "" {
		return Tool{}
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	// Some print their version with an exit code that means usage, so only
	// the output counts.
	out, _ := exec.CommandContext(ctx, path, p.args...).CombinedOutput()
	if len(out) > 4096 {
		out = out[:4096]
	}
	return Tool{Available: true, Path: path, Version: versionNumber.FindString(string(out))}
}

func (s *service) info(ctx context.Context) (Info, error) {
	app, err := s.store.get(ctx)
	if err != nil {
		return Info{}, err
	}
	_, released := version.Release()
	in := Info{
		Version:    version.Version,
		Deployment: "container",
		Commit:     version.CommitSHA,
		Release:    released,
		Go:         runtime.Version(),
		OS:         runtime.GOOS,
		Arch:       runtime.GOARCH,
		Started:    started.UTC().Truncate(time.Second),
		Uptime:     int64(time.Since(started) / time.Second),
		Tools:      s.tools.get(),
		Unraid:     UnraidInfo{Socket: unraid.ConfigFromEnv().Write},
		Feature: Features{
			Office:   OfficeInfo{Kind: app.Office.Kind, URL: app.Office.URL},
			Thumbs:   app.Previews.Thumbs,
			Video:    app.Previews.Video,
			WebDAV:   app.WebDAV,
			MuPDF:    settings.Env.MuPdfAvailable,
			Terminal: app.Terminal,
		},
	}
	if desktop.Active() {
		in.Deployment = "desktop"
		in.Feature.Terminal = true
	}
	if app.Office.Kind == OfficeNone {
		in.Feature.Office.URL = ""
	}
	if st, err := unraid.Default().State(); err == nil {
		in.Unraid.Available = st.Available
	}
	return in, nil
}
