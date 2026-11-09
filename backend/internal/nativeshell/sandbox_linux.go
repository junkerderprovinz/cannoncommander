package nativeshell

import (
	"context"
	"io/fs"
	"os"
	"os/exec"
	"strings"
	"sync"
	"time"

	"github.com/gtsteffaniak/go-logger/logger"
	"github.com/junkerderprovinz/cannoncommander/backend/pkg/elevate"
)

// sandboxEnv is all of an environment a thumbnailer gets, as names and
// values. HOME points into its private folder, where a program may keep a
// cache for the one run.
var sandboxEnv = []string{"PATH", "/usr/local/bin:/usr/bin:/bin", "HOME", "/tmp", "GIO_USE_VFS", "local"}

// isolations are the namespaces bwrap shuts a thumbnailer into, strongest
// first. Some kernels refuse a process the loopback of a network namespace
// but allow the others, so the second leaves the network out.
var isolations = [][]string{
	{"--unshare-all"},
	{"--unshare-user-try", "--unshare-ipc", "--unshare-pid", "--unshare-uts", "--unshare-cgroup-try"},
}

// bwrapArgs wraps cmd in bubblewrap the way gnome-desktop runs its
// thumbnailers: the system's programs and libraries read-only, outDir as a
// private /tmp with the input read-only inside it at inside, the namespaces
// of isolate and none of this process's environment. root is the file
// system at /, read to tell which of /bin, /lib and the others are links
// into /usr.
func bwrapArgs(bwrap string, isolate []string, root fs.FS, outDir, input, inside string, cmd []string) []string {
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
	args = append(args, "--proc", "/proc", "--dev", "/dev", "--chdir", "/")
	args = append(args, isolate...)
	args = append(args, "--die-with-parent", "--new-session", "--clearenv")
	for i := 0; i < len(sandboxEnv); i += 2 {
		args = append(args, "--setenv", sandboxEnv[i], sandboxEnv[i+1])
	}
	if outDir != "" {
		args = append(args, "--bind", outDir, "/tmp", "--ro-bind", input, inside)
	}
	return append(append(args, "--"), cmd...)
}

// sandbox is bwrap and the namespaces it can make here, or no program
// where it cannot shut a thumbnailer in.
type sandbox struct {
	program string
	isolate []string
}

var (
	sandboxes = struct {
		sync.Mutex
		found map[string]sandbox
	}{found: map[string]sandbox{}}
	warn = logger.Warningf
)

// findSandbox is the strongest sandbox bwrap can set up here. Where the
// kernel restricts user namespaces, as Ubuntu does since 24.04, or a
// container keeps them off, bwrap is there and still fails, so each way is
// tried once with a program that does nothing, and why one failed is
// logged that once.
func findSandbox(look func(string) (string, error), root fs.FS) sandbox {
	bwrap, err := look("bwrap")
	if err != nil {
		return sandbox{}
	}
	sandboxes.Lock()
	defer sandboxes.Unlock()
	if s, tried := sandboxes.found[bwrap]; tried {
		return s
	}
	var s sandbox
	for _, isolate := range isolations {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		args := bwrapArgs(bwrap, isolate, root, "", "", "", []string{"true"})
		out, err := exec.CommandContext(ctx, args[0], args[1:]...).CombinedOutput()
		cancel()
		if err == nil {
			s = sandbox{program: bwrap, isolate: isolate}
			break
		}
		warn("bwrap cannot shut thumbnailers in with %s here: %v %s", strings.Join(isolate, " "), err, strings.TrimSpace(string(out)))
	}
	if s.program == "" {
		warn("no thumbnailer of the system runs without a sandbox, so only the shared cache and a thumbnail service give pictures")
	}
	sandboxes.found[bwrap] = s
	return s
}

// Sandboxing says how the system's thumbnails come about here, and whether
// the kernel's restriction on user namespaces is what keeps the sandbox out.
func Sandboxing() SandboxState {
	k := current()
	restrict, _ := fs.ReadFile(k.root, "proc/sys/kernel/apparmor_restrict_unprivileged_userns")
	_, err := os.Stat(elevate.ProfileFile)
	return SandboxState{
		Sandboxed: k.sandbox.program != "", Service: k.service != nil,
		Restricted: strings.TrimSpace(string(restrict)) == "1", Allowed: err == nil,
	}
}

// Resandbox forgets what the probe found, so the next thumbnail tries bwrap
// again, as after the app's AppArmor profile came or went.
func Resandbox() {
	sandboxes.Lock()
	clear(sandboxes.found)
	sandboxes.Unlock()
	deskMu.Lock()
	shared = nil
	deskMu.Unlock()
}
