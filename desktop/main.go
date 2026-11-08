// Command desktop is CannonCommander in a window of its own. The server runs
// in this process on a loopback port that only the window may use, and the
// window shows its interface, so the desktop app and the container run the
// same program.
//
// It is a separate Go module so the Wails toolchain never touches the server
// build; .github/workflows/desktop.yml builds it per platform.
package main

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	_ "embed"
	"encoding/hex"
	"flag"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"sync/atomic"
	"time"

	"github.com/wailsapp/wails/v3/pkg/application"
	"github.com/wailsapp/wails/v3/pkg/events"

	"github.com/junkerderprovinz/cannoncommander/backend/cmd"
	"github.com/junkerderprovinz/cannoncommander/backend/pkg/desktop"
)

// The window, the taskbar and the installer show one mark.
//
//go:embed build/appicon.png
var appIcon []byte

func main() {
	if err := run(); err != nil {
		log.Fatal(err)
	}
}

func run() error {
	scheduled := flag.Bool("update", false, "update the copy installed for all users and exit, as the installer's scheduled task does")
	after := flag.Int("after", 0, "wait for the process with this id to end first, as a restart does")
	flag.Bool("home", false, "open the home folder in a new tab, as Win+E does")
	fileManager := flag.String("file-manager", "", "on to open the computer's folders here, off to give them back, and exit")
	flag.Parse()
	if *elevated != "" {
		return serveElevated()
	}
	if *scheduled {
		return updateInstalled()
	}
	if *fileManager != "" {
		return switchFileManager(*fileManager == "on")
	}
	if *after > 0 {
		waitGone(*after, 30*time.Second)
	}

	dir, err := dataDir()
	if err != nil {
		return err
	}
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return fmt.Errorf("data folder: %w", err)
	}
	state := openState(dir)
	cwd, _ := os.Getwd()
	start, places := handedOver(flag.Args(), cwd)
	sh := &shell{state: state}
	sh.folders = newFolderHandler()

	// Filled in below, once this is known to be the only copy.
	var window *application.WebviewWindow
	app := application.New(application.Options{
		Name:        "CannonCommander",
		Description: "A two-pane file manager",
		Icon:        appIcon,
		// Shown while the server starts; the window then goes over to it.
		Assets: application.AssetOptions{Handler: http.HandlerFunc(splash)},
		// A second launch brings the first window back, since two copies would
		// share one database and one port.
		SingleInstance: &application.SingleInstanceOptions{
			UniqueID: instanceID(dir),
			OnSecondInstanceLaunch: sh.secondLaunch,
		},
		Windows: application.WindowsOptions{WebviewUserDataPath: filepath.Join(dir, "webview")},
		Linux:   application.LinuxOptions{ProgramName: "CannonCommander"},
	})
	sh.app = app
	passOn(places)

	port, err := pickPort(state.get().Port)
	if err != nil {
		return err
	}
	if port != state.get().Port {
		if err := state.update(func(s *State) { s.Port = port }); err != nil {
			log.Printf("could not keep the port: %v", err)
		}
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return err
	}
	config, err := writeServerConfig(dir, port, defaultSources(home))
	if err != nil {
		return fmt.Errorf("write the configuration: %w", err)
	}
	trimLog(filepath.Join(dir, logFile))
	// rclone mounts the remotes below the data folder, as the container does
	// below /remotes.
	os.Setenv("CC_REMOTES_ROOT", filepath.Join(dir, "remotes"))

	installed := isInstalled()
	if installed {
		if machine, err := machineData(); err == nil {
			sh.machineFile = filepath.Join(machine, machineSettingsFile)
		}
	}
	key, err := newKey()
	if err != nil {
		return err
	}
	desktop.Use(sh, desktop.Options{Key: key, Port: port, Home: home})

	ctx, stop := context.WithCancel(context.Background())
	defer stop()
	served := make(chan struct{})
	go func() {
		defer close(served)
		cmd.Desktop(ctx, config)
	}()

	// A newer release is downloaded in the background and starts next time;
	// see updates.go. The copy installed for all users leaves that to the
	// installer's scheduled task and only passes on the news.
	sh.up = newUpdater(openUpdateLog(filepath.Join(dir, "update.log")))
	if installed {
		go sh.up.follow(ctx)
	} else {
		go sh.up.run(ctx, sh.AutoUpdate)
	}

	window = openWindow(app, state)
	sh.followWindow(window)
	base := "http://127.0.0.1:" + strconv.Itoa(port)
	go func() {
		if waitServing(ctx, base, 2*time.Minute) {
			window.SetURL(enterURL(base, key, start))
		}
	}()

	app.OnShutdown(func() {
		stop()
		sh.up.stop()
	})
	err = app.Run()
	stop()
	// The server closes its database and ends its rclone; a shutdown that
	// hangs is cut short so the program still ends.
	select {
	case <-served:
	case <-time.After(30 * time.Second):
		log.Printf("the server did not stop within 30 seconds")
	}
	if sh.restart.Load() {
		return startAgain()
	}
	return err
}

// shell is what the server asks of the desktop app; see desktop.App.
type shell struct {
	app   *application.App
	state *stateStore
	up    *updater
	// machineFile holds the switch of the copy installed for all users,
	// which the scheduled task reads. "" for every other copy.
	machineFile string
	restart     atomic.Bool
	volumes     volumeWatch
	standby
}

func (s *shell) AutoUpdate() bool {
	if s.machineFile != "" {
		return readAutoUpdate(s.machineFile)
	}
	return s.state.get().autoUpdate()
}

func (s *shell) SetAutoUpdate(on bool) error {
	if s.machineFile != "" {
		return writeAutoUpdate(s.machineFile, on)
	}
	return s.state.update(func(st *State) { st.AutoUpdate = &on })
}

func (s *shell) UpdateReady() string { return s.up.readyVersion() }

func (s *shell) Restart() {
	s.restart.Store(true)
	s.app.Quit()
}

// startAgain starts this program anew, which waits for this process to end
// before it takes the lock, the database and the port.
func startAgain() error {
	exe, err := os.Executable()
	if err != nil {
		return err
	}
	c := exec.Command(exe, "--after", strconv.Itoa(os.Getpid()))
	c.Env = os.Environ()
	return c.Start()
}

// openWindow opens the main window where it was when it last closed, as long
// as that place is still on a screen.
func openWindow(app *application.App, state *stateStore) *application.WebviewWindow {
	b := state.get().Window
	opts := application.WebviewWindowOptions{
		Name:             "main",
		Title:            "CannonCommander",
		Width:            1280,
		Height:           820,
		MinWidth:         800,
		MinHeight:        520,
		BackgroundColour: application.NewRGB(0x16, 0x16, 0x16),
		URL:              "/",
	}
	if b.Width > 0 && b.Height > 0 {
		opts.Width, opts.Height = b.Width, b.Height
	}
	if b.Maximised {
		opts.StartState = application.WindowStateMaximised
	}
	w := app.Window.NewWithOptions(opts)
	app.Event.OnApplicationEvent(events.Common.ApplicationStarted, func(*application.ApplicationEvent) {
		if b.Width > 0 && !b.Maximised && onScreen(app.Screen.GetAll(), b) {
			w.SetBounds(application.Rect{X: b.X, Y: b.Y, Width: b.Width, Height: b.Height})
		}
	})
	// The place is kept as the window moves, since a restart or a shutdown of
	// the computer ends the program without closing the window first.
	var later *time.Timer
	keep := func(*application.WindowEvent) {
		if later != nil {
			later.Stop()
		}
		later = time.AfterFunc(500*time.Millisecond, func() { keepPlace(w, state) })
	}
	w.OnWindowEvent(events.Common.WindowDidMove, keep)
	w.OnWindowEvent(events.Common.WindowDidResize, keep)
	w.RegisterHook(events.Common.WindowClosing, func(*application.WindowEvent) { keepPlace(w, state) })
	return w
}

// keepPlace saves where the window is. A maximised window keeps the place it
// goes back to.
func keepPlace(w *application.WebviewWindow, state *stateStore) {
	maximised, r := w.IsMaximised(), w.Bounds()
	err := state.update(func(s *State) {
		s.Window.Maximised = maximised
		if !maximised && !w.IsMinimised() {
			s.Window.X, s.Window.Y, s.Window.Width, s.Window.Height = r.X, r.Y, r.Width, r.Height
		}
	})
	if err != nil {
		log.Printf("could not keep the window's place: %v", err)
	}
}

// onScreen reports whether the title bar of a window at b would show on one
// of the screens, so a window last seen on a screen since unplugged opens in
// the middle instead.
func onScreen(screens []*application.Screen, b Bounds) bool {
	for _, s := range screens {
		a := s.WorkArea
		if b.X+100 > a.X && b.X+100 < a.X+a.Width && b.Y >= a.Y && b.Y+30 < a.Y+a.Height {
			return true
		}
	}
	return false
}

// waitServing waits for the server to answer at base. Any answer counts: the
// guard turns away a request without the key, and so says it listens.
func waitServing(ctx context.Context, base string, within time.Duration) bool {
	client := &http.Client{Timeout: 2 * time.Second}
	deadline := time.Now().Add(within)
	for time.Now().Before(deadline) {
		req, err := http.NewRequestWithContext(ctx, http.MethodGet, base+"/health", nil)
		if err != nil {
			return false
		}
		if resp, err := client.Do(req); err == nil {
			resp.Body.Close()
			return true
		}
		select {
		case <-ctx.Done():
			return false
		case <-time.After(100 * time.Millisecond):
		}
	}
	log.Printf("the server did not answer within %s", within)
	return false
}

// newKey is the secret the window proves itself with, new on every start.
func newKey() (string, error) {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return hex.EncodeToString(b), nil
}

// dataDir is the folder for the configuration, the database and the logs:
// CC_DATA where it is set, as for a copy that keeps everything apart, and
// CannonCommander in the user's configuration folder otherwise. That is
// AppData\Roaming on Windows, Library/Application Support on a Mac and
// ~/.config on Linux.
func dataDir() (string, error) {
	if d := os.Getenv("CC_DATA"); d != "" {
		return filepath.Abs(d)
	}
	base, err := os.UserConfigDir()
	if err != nil {
		return "", fmt.Errorf("find the configuration folder: %w", err)
	}
	return filepath.Join(base, product), nil
}

// instanceID is the single-instance lock for dir. A copy on its own CC_DATA
// folder gets a lock of its own, so it runs beside the usual one.
func instanceID(dir string) string {
	if os.Getenv("CC_DATA") == "" {
		return appID
	}
	sum := sha256.Sum256([]byte(dir))
	return appID + "." + hex.EncodeToString(sum[:4])
}

// splash is the page the window shows until the server answers: the mark on
// the background the interface starts with.
func splash(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path == "/icon.png" {
		w.Header().Set("Content-Type", "image/png")
		_, _ = w.Write(appIcon)
		return
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	_, _ = w.Write([]byte(splashPage))
}

const splashPage = `<!doctype html>
<html><head><meta charset="utf-8"><title>CannonCommander</title>
<style>
html,body{margin:0;height:100%;background:#161616}
@media (prefers-color-scheme: light){html,body{background:#f4f4f4}}
body{display:grid;place-items:center}
img{width:96px;height:96px;opacity:.9}
</style></head>
<body><img src="/icon.png" alt=""></body></html>
`
