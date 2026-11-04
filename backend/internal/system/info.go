package system

import (
	"context"
	"os"
	"os/exec"
	"regexp"
	"runtime"
	"sync"
	"time"

	"github.com/junkerderprovinz/cannoncommander/backend/internal/unraid"
	"github.com/junkerderprovinz/cannoncommander/backend/internal/version"
	"github.com/junkerderprovinz/cannoncommander/backend/pkg/settings"
)

var started = time.Now()

// Info is what GET /system answers.
type Info struct {
	Version string          `json:"version"`
	Commit  string          `json:"commit,omitempty"`
	Go      string          `json:"go"`
	OS      string          `json:"os"`
	Arch    string          `json:"arch"`
	Started time.Time       `json:"started"`
	Uptime  int64           `json:"uptime"`
	Tools   map[string]Tool `json:"tools"`
	Unraid  UnraidInfo      `json:"unraid"`
	Feature Features        `json:"features"`
}

// Tool is a helper program the server runs.
type Tool struct {
	Available bool   `json:"available"`
	Path      string `json:"path,omitempty"`
	Version   string `json:"version,omitempty"`
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
}

// probes are the helper programs and how they print their version.
var probes = []probe{
	{"ffmpeg", []string{"-version"}, func() string { return settings.Env.FFmpegPath }},
	{"ffprobe", []string{"-version"}, func() string { return settings.Env.FFprobePath }},
	{"mutool", []string{"-v"}, nil},
	{"7zz", nil, nil},
	{"rclone", []string{"version"}, RcloneBinary},
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
// not change while the server runs.
type toolbox struct {
	once  sync.Once
	tools map[string]Tool
}

func (t *toolbox) get() map[string]Tool {
	t.once.Do(func() {
		t.tools = map[string]Tool{}
		var mu sync.Mutex
		var wg sync.WaitGroup
		for _, p := range probes {
			wg.Go(func() {
				tool := find(p)
				mu.Lock()
				t.tools[p.name] = tool
				mu.Unlock()
			})
		}
		wg.Wait()
	})
	return t.tools
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
	in := Info{
		Version: version.Version,
		Commit:  version.CommitSHA,
		Go:      runtime.Version(),
		OS:      runtime.GOOS,
		Arch:    runtime.GOARCH,
		Started: started.UTC().Truncate(time.Second),
		Uptime:  int64(time.Since(started) / time.Second),
		Tools:   s.tools.get(),
		Unraid:  UnraidInfo{Socket: unraid.ConfigFromEnv().Write},
		Feature: Features{
			Office: OfficeInfo{Kind: app.Office.Kind, URL: app.Office.URL},
			Thumbs: app.Previews.Thumbs,
			Video:  app.Previews.Video,
			WebDAV: app.WebDAV,
			MuPDF:  settings.Env.MuPdfAvailable,
		},
	}
	if app.Office.Kind == OfficeNone {
		in.Feature.Office.URL = ""
	}
	if st, err := unraid.Default().State(); err == nil {
		in.Unraid.Available = st.Available
	}
	return in, nil
}
