package nativeshell

import (
	"context"
	"io/fs"
	"os/exec"
	"strings"
	"sync"
	"time"
)

// sandboxEnv is all of an environment a thumbnailer gets, as names and
// values. HOME points into its private folder, where a program may keep a
// cache for the one run.
var sandboxEnv = []string{"PATH", "/usr/local/bin:/usr/bin:/bin", "HOME", "/tmp", "GIO_USE_VFS", "local"}

// bwrapArgs wraps cmd in bubblewrap the way gnome-desktop runs its
// thumbnailers: the system's programs and libraries read-only, outDir as a
// private /tmp with the input read-only inside it at inside, no network and
// none of this process's environment. root is the file system at /, read
// to tell which of /bin, /lib and the others are links into /usr.
func bwrapArgs(bwrap string, root fs.FS, outDir, input, inside string, cmd []string) []string {
	args := []string{bwrap, "--ro-bind", "/usr", "/usr", "--ro-bind-try", "/etc/ld.so.cache", "/etc/ld.so.cache"}
	for _, dir := range []string{"bin", "lib64", "lib", "lib32", "sbin"} {
		if target, err := fs.ReadLink(root, dir); err == nil && strings.TrimPrefix(target, "/") == "usr/"+dir {
			args = append(args, "--symlink", "/usr/"+dir, "/"+dir)
		} else if info, err := fs.Stat(root, dir); err == nil && info.IsDir() {
			args = append(args, "--ro-bind", "/"+dir, "/"+dir)
		}
	}
	// Debian's alternatives complete /usr/bin, and fontconfig needs its
	// settings and cache to draw text.
	for _, dir := range []string{"/etc/alternatives", "/etc/fonts", "/var/cache/fontconfig"} {
		args = append(args, "--ro-bind-try", dir, dir)
	}
	args = append(args, "--proc", "/proc", "--dev", "/dev", "--chdir", "/", "--unshare-all", "--die-with-parent", "--new-session", "--clearenv")
	for i := 0; i < len(sandboxEnv); i += 2 {
		args = append(args, "--setenv", sandboxEnv[i], sandboxEnv[i+1])
	}
	if outDir != "" {
		args = append(args, "--bind", outDir, "/tmp", "--ro-bind", input, inside)
	}
	return append(append(args, "--"), cmd...)
}

var sandboxes = struct {
	sync.Mutex
	works map[string]bool
}{works: map[string]bool{}}

// sandboxProgram is bwrap where it can set up a sandbox here, or "". Where
// user namespaces are off, as in many containers, bwrap is there and still
// fails, so it is tried once with a program that does nothing.
func sandboxProgram(look func(string) (string, error), root fs.FS) string {
	bwrap, err := look("bwrap")
	if err != nil {
		return ""
	}
	sandboxes.Lock()
	defer sandboxes.Unlock()
	works, tried := sandboxes.works[bwrap]
	if !tried {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		args := bwrapArgs(bwrap, root, "", "", "", []string{"true"})
		works = exec.CommandContext(ctx, args[0], args[1:]...).Run() == nil
		sandboxes.works[bwrap] = works
	}
	if !works {
		return ""
	}
	return bwrap
}
