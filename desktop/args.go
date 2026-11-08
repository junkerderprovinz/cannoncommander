package main

import (
	"encoding/json"
	"log"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"runtime"
	"strings"

	"github.com/wailsapp/wails/v3/pkg/application"

	"github.com/junkerderprovinz/cannoncommander/backend/pkg/desktop"
)

// opening is a folder the window opens in a new tab, with the file to put
// the cursor on when a file was handed over. An empty one is the home
// folder.
type opening struct {
	Folder string `json:"folder,omitempty"`
	Select string `json:"select,omitempty"`
}

// homeFlag asks for the home folder in a new tab, as Win+E does.
const homeFlag = "--home"

var driveURI = regexp.MustCompile(`^/[A-Za-z]:`)

// handedOver turns the arguments of a start, as the command line, a second
// launch or another program hands them over, into what the window opens.
// Relative paths count from dir. Places of the Windows shell that are no
// folder on disk, such as This PC or the control panel, come back apart
// for Explorer to show.
func handedOver(args []string, dir string) (open []opening, places []string) {
	for _, a := range args {
		switch {
		case a == homeFlag || a == "-home":
			open = append(open, opening{})
			continue
		case strings.HasPrefix(a, "-"):
			continue
		case runtime.GOOS == "windows" && (strings.HasPrefix(a, "::") || strings.HasPrefix(strings.ToLower(a), "shell:")):
			places = append(places, a)
			continue
		}
		p := a
		if u, err := url.Parse(a); err == nil && u.Scheme == "file" {
			p = u.Path
			if driveURI.MatchString(p) {
				p = p[1:]
			}
			p = filepath.FromSlash(p)
		}
		if !filepath.IsAbs(p) {
			p = filepath.Join(dir, p)
		}
		info, err := os.Stat(p)
		switch {
		case err != nil:
		case info.IsDir():
			open = append(open, opening{Folder: filepath.Clean(p)})
		default:
			open = append(open, opening{Folder: filepath.Dir(p), Select: filepath.Base(p)})
		}
	}
	return open, places
}

// enterURL is where the window starts, with the folders it opens first.
func enterURL(base, key string, open []opening) string {
	q := url.Values{"key": {key}}
	if len(open) > 0 {
		if b, err := json.Marshal(open); err == nil {
			q.Set("open", string(b))
		}
	}
	return base + desktop.EnterPath + "?" + q.Encode()
}

// secondLaunch shows what a second start of the app hands over in the window
// that is open already, and brings that window to the front.
func (s *shell) secondLaunch(d application.SecondInstanceData) {
	var open []opening
	if len(d.Args) > 1 {
		var places []string
		open, places = handedOver(d.Args[1:], d.WorkingDir)
		passOn(places)
	}
	s.show(open)
}

// passOn hands places of the Windows shell to Explorer, which alone can show
// them.
func passOn(places []string) {
	for _, p := range places {
		if err := exec.Command("explorer.exe", p).Start(); err != nil {
			log.Printf("could not hand %s to Explorer: %v", p, err)
		}
	}
}
