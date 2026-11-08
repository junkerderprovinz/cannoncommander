package nativeshell

import (
	"cmp"
	"io/fs"
	"math/big"
	"path"
	"path/filepath"
	"slices"
	"strconv"
	"strings"

	"github.com/junkerderprovinz/cannoncommander/backend/internal/xdg"
)

// nemoConditions reports whether every condition of a Nemo action holds
// for sel, as Nemo asks. dir is the folder of the action file.
func (k *desk) nemoConditions(g xdg.Group, dir string, sel selection) bool {
	for _, c := range g.List("Conditions") {
		if !k.nemoCondition(c, g, dir, sel) {
			return false
		}
	}
	return true
}

// nemoCondition checks one condition the way Nemo does. The app is neither
// Nemo's desktop nor its sidebar, so desktop and sidebar-only never hold,
// and a condition Nemo does not know keeps the action hidden.
func (k *desk) nemoCondition(c string, g xdg.Group, dir string, sel selection) bool {
	kind, arg, _ := strings.Cut(c, " ")
	switch kind {
	case "sidebar-allow":
		return arg == ""
	case "removable":
		m, ok := k.mountOf(sel.items[0].path)
		return arg == "" && ok && k.removable(m.device)
	case "filesystem":
		for _, it := range sel.items {
			if m, ok := k.mountOf(it.path); !ok || m.fstype != arg && m.fstype != "fuse."+arg {
				return false
			}
		}
		return arg != ""
	case "dbus":
		return arg != "" && k.hasOwner(arg)
	case "gsettings":
		return k.gsettingHolds(arg)
	case "exec":
		args, err := nemoCommand(inActionDir(dir, arg), g, sel)
		if err != nil || k.Output == nil {
			return false
		}
		_, err = k.Output(args[0], args[1:]...)
		return err == nil
	}
	return false
}

// mount is a line of /proc/self/mountinfo.
type mount struct {
	dir, fstype string
	// device is the block device as major:minor.
	device string
}

// mountOf finds the mount that holds file: the deepest mount point above
// it, and of mounts on the same point the last, which hides the others.
func (k *desk) mountOf(file string) (mount, bool) {
	if real, err := filepath.EvalSymlinks(file); err == nil {
		file = real
	}
	b, err := fs.ReadFile(k.root, "proc/self/mountinfo")
	if err != nil {
		return mount{}, false
	}
	var best mount
	found := false
	for _, line := range strings.Split(string(b), "\n") {
		before, after, ok := strings.Cut(line, " - ")
		fields, rest := strings.Fields(before), strings.Fields(after)
		if !ok || len(fields) < 5 || len(rest) == 0 {
			continue
		}
		dir := unescapeMount(fields[4])
		if dir != "/" && file != dir && !strings.HasPrefix(file, dir+"/") || found && len(dir) < len(best.dir) {
			continue
		}
		best, found = mount{dir: dir, fstype: rest[0], device: fields[2]}, true
	}
	return best, found
}

// unescapeMount undoes the octal escapes the kernel writes for spaces and
// other blanks in a mount point.
func unescapeMount(s string) string {
	var b strings.Builder
	for i := 0; i < len(s); i++ {
		if s[i] == '\\' && i+4 <= len(s) {
			if n, err := strconv.ParseUint(s[i+1:i+4], 8, 8); err == nil {
				b.WriteByte(byte(n))
				i += 3
				continue
			}
		}
		b.WriteByte(s[i])
	}
	return b.String()
}

// removable reports whether the block device major:minor sits on a drive
// the kernel calls removable, which is what GIO tells Nemo.
func (k *desk) removable(device string) bool {
	b, err := fs.ReadFile(k.root, "sys/dev/block/"+device+"/uevent")
	if err != nil {
		return false
	}
	for _, line := range strings.Split(string(b), "\n") {
		if name, ok := strings.CutPrefix(line, "DEVNAME="); ok {
			return k.diskRemovable(name)
		}
	}
	return false
}

// diskRemovable looks from a partition to its disk, and from a device
// mapper layer such as LUKS to the devices below it.
func (k *desk) diskRemovable(name string) bool {
	if _, err := fs.Stat(k.root, path.Join("sys/block", name)); err != nil {
		disks, _ := fs.ReadDir(k.root, "sys/block")
		i := slices.IndexFunc(disks, func(d fs.DirEntry) bool {
			_, err := fs.Stat(k.root, path.Join("sys/block", d.Name(), name))
			return err == nil
		})
		if i < 0 {
			return false
		}
		name = disks[i].Name()
	}
	if b, err := fs.ReadFile(k.root, path.Join("sys/block", name, "removable")); err == nil && strings.TrimSpace(string(b)) == "1" {
		return true
	}
	below, _ := fs.ReadDir(k.root, path.Join("sys/block", name, "slaves"))
	return slices.ContainsFunc(below, func(d fs.DirEntry) bool { return k.diskRemovable(d.Name()) })
}

// gsettingHolds checks a gsettings condition: "schema key" holds when the
// key is true, and "schema key type op value" compares the key with a
// value of that GVariant type by eq, ne, lt or gt.
func (k *desk) gsettingHolds(arg string) bool {
	f := strings.SplitN(arg, " ", 5)
	if len(f) != 2 && len(f) != 5 {
		return false
	}
	got, err := k.GSetting(f[0], f[1])
	if err != nil {
		return false
	}
	if len(f) == 2 {
		return got == "true"
	}
	c, ok := compareGVariant(f[2], got, f[4])
	if !ok {
		return false
	}
	switch f[3] {
	case "eq":
		return c == 0
	case "ne":
		return c != 0
	case "lt":
		return c < 0
	case "gt":
		return c > 0
	}
	return false
}

// gvariantInts are GVariant's integer types by the word GLib prints before
// a value of that type. An int32 has none.
var gvariantInts = map[string]string{"y": "byte", "n": "int16", "q": "uint16", "i": "", "u": "uint32", "x": "int64", "t": "uint64", "h": "handle"}

// compareGVariant compares got, a value as gsettings prints it, with want
// as Nemo parses it, both of type typ. It fails where got is of another
// type, as Nemo's comparison does.
func compareGVariant(typ, got, want string) (int, bool) {
	fields := strings.Fields(want)
	if len(fields) == 0 {
		return 0, false
	}
	bare := fields[len(fields)-1]
	switch typ {
	case "b":
		if got != "true" && got != "false" || bare != "true" && bare != "false" {
			return 0, false
		}
		return strings.Compare(got, bare), true
	case "s":
		a, ok1 := xdg.UnquoteGVariant(got)
		b, ok2 := xdg.UnquoteGVariant(want)
		return strings.Compare(a, b), ok1 && ok2
	case "d":
		a, err1 := strconv.ParseFloat(got, 64)
		b, err2 := strconv.ParseFloat(bare, 64)
		if err1 != nil || err2 != nil {
			return 0, false
		}
		return cmp.Compare(a, b), true
	}
	word, ok := gvariantInts[typ]
	if !ok {
		return 0, false
	}
	if word != "" {
		w, n, ok := strings.Cut(got, " ")
		if !ok || w != word {
			return 0, false
		}
		got = n
	}
	a, ok1 := new(big.Int).SetString(got, 0)
	b, ok2 := new(big.Int).SetString(bare, 0)
	if !ok1 || !ok2 {
		return 0, false
	}
	return a.Cmp(b), true
}
