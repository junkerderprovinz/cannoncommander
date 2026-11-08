package openwith

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os/exec"
	"strings"
	"time"
)

// Launch Services knows which programs open a file. The server asks it
// through NSWorkspace in JavaScript for Automation, which every Mac runs
// with osascript.
type launchServices struct{}

func newSystem() system { return launchServices{} }

func (launchServices) dialog() bool { return true }

func (launchServices) every() ([]app, error) { return nil, errNoDialog }

// appsScript prints the programs for the file in argv[0] as JSON, the
// default one marked.
const appsScript = `
ObjC.import('AppKit');
function run(argv) {
  const ws = $.NSWorkspace.sharedWorkspace;
  const fm = $.NSFileManager.defaultManager;
  const url = $.NSURL.fileURLWithPath(argv[0]);
  const def = ws.URLForApplicationToOpenURL(url);
  const urls = ws.URLsForApplicationsToOpenURL(url);
  const out = [];
  for (let i = 0; i < urls.count; i++) {
    const path = urls.objectAtIndex(i).path.js;
    out.push({ path: path, name: fm.displayNameAtPath(path).js.replace(/\.app$/, ''), def: !def.isNil() && def.path.js === path });
  }
  return JSON.stringify(out);
}`

// iconScript prints the icon of the program in argv[0] as a PNG in base64,
// drawn at the size of a menu icon.
const iconScript = `
ObjC.import('AppKit');
function run(argv) {
  const size = Number(argv[1]);
  const img = $.NSWorkspace.sharedWorkspace.iconForFile(argv[0]);
  img.setSize($.NSMakeSize(size, size));
  const cg = img.CGImageForProposedRectContextHints(null, $(), $());
  const rep = $.NSBitmapImageRep.alloc.initWithCGImage(cg);
  const data = rep.representationUsingTypeProperties($.NSBitmapImageFileTypePNG, $());
  return data.base64EncodedStringWithOptions(0).js;
}`

func script(src string, args ...string) (string, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()
	out, err := exec.CommandContext(ctx, "osascript", append([]string{"-l", "JavaScript", "-e", src}, args...)...).Output()
	if err != nil {
		var exit *exec.ExitError
		if errors.As(err, &exit) {
			return "", fmt.Errorf("%w: %s", err, strings.TrimSpace(string(exit.Stderr)))
		}
		return "", err
	}
	return strings.TrimSpace(string(out)), nil
}

func (launchServices) apps(file string) ([]app, error) {
	out, err := script(appsScript, file)
	if err != nil {
		return nil, err
	}
	var found []struct {
		Path string `json:"path"`
		Name string `json:"name"`
		Def  bool   `json:"def"`
	}
	if err := json.Unmarshal([]byte(out), &found); err != nil {
		return nil, err
	}
	list := make([]app, 0, len(found))
	for _, f := range found {
		list = append(list, app{id: f.Path, name: f.Name, icon: f.Path, def: f.Def})
	}
	return defaultFirst(list), nil
}

func (launchServices) icon(where string) (string, error) {
	out, err := script(iconScript, where, fmt.Sprint(iconSize))
	if err != nil {
		return "", err
	}
	if out == "" {
		return "", errNoIcon
	}
	return "data:image/png;base64," + out, nil
}

func (launchServices) open(file, id string) error {
	return exec.Command("open", "-a", id, file).Run()
}

// chooseScript asks for a program in the Mac's application chooser and
// prints its path. Cancel ends it with error -128.
const chooseScript = `
function run(argv) {
  const app = Application.currentApplication();
  app.includeStandardAdditions = true;
  return app.chooseApplication({ as: 'alias' }).toString();
}`

func (launchServices) choose(file string) error {
	ctx, cancel := context.WithTimeout(context.Background(), time.Hour)
	defer cancel()
	out, err := exec.CommandContext(ctx, "osascript", "-l", "JavaScript", "-e", chooseScript).Output()
	if err != nil {
		var exit *exec.ExitError
		if errors.As(err, &exit) && strings.Contains(string(exit.Stderr), "-128") {
			return nil
		}
		return err
	}
	return exec.Command("open", "-a", strings.TrimSpace(string(out)), file).Run()
}
