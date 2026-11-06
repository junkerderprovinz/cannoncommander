package ccfs

import (
	"cmp"
	"path"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"unicode"
	"unicode/utf8"
)

// WithDevices returns a copy of r that shows the named pools and disks below
// mnt to admins, each as a source of its own at /mnt/<name>. They are never
// indexed, since a walk over a disk wakes it, and their files show up in the
// user shares anyway. A configured source at or around one of them wins.
func (r *Resolver) WithDevices(mnt string, names ...string) *Resolver {
	cur := r.current()
	next := *cur
	next.configured = nil
	for _, s := range cur.sources {
		if !s.Device {
			next.configured = append(next.configured, s)
		}
	}
	next.mnt = mnt
	next.named = slices.Clone(cur.named)
	return r.withSet(next.withDevices(names))
}

// SetDevices lays out the pools and disks of WithDevices again, when Unraid
// adds or takes one away. Requests that run meanwhile see either layout.
func (r *Resolver) SetDevices(names ...string) {
	r.set.Store(r.current().withDevices(names))
}

// withDevices returns s with the pools and disks in names below its mnt.
func (s *sourceSet) withDevices(names []string) *sourceSet {
	next := &sourceSet{
		configured: s.configured, mnt: s.mnt, marked: s.marked,
		devices: slices.Clone(names), named: slices.Clone(s.named),
	}
	for _, name := range names {
		if !slices.Contains(next.named, name) {
			next.named = append(next.named, name)
		}
	}
	taken := map[string]bool{}
	for _, src := range s.configured {
		c := *src
		c.AdminOnly = c.AdminOnly || next.adminOnly(c.Mount)
		next.sources = append(next.sources, &c)
		taken[strings.ToLower(c.Name)] = true
	}
	configured := slices.Clone(next.sources)
	for _, name := range names {
		mount := "/mnt/" + name
		if strings.Contains(name, "/") || slices.ContainsFunc(configured, func(c *Source) bool {
			return within(mount, c.Mount) || within(c.Mount, mount)
		}) {
			continue
		}
		label := deviceLabel(name)
		if taken[strings.ToLower(label)] {
			label = name
		}
		if taken[strings.ToLower(label)] {
			continue
		}
		taken[strings.ToLower(label)] = true
		next.sources = append(next.sources, &Source{
			Name: label, Mount: mount, Root: filepath.Join(s.mnt, name),
			AdminOnly: true, Device: true,
		})
	}
	next.close()
	return next
}

// deviceLabel names a disk the way the Unraid web interface does, Disk 1 for
// disk1, and a pool by its name with a capital letter.
func deviceLabel(name string) string {
	if n, ok := strings.CutPrefix(name, "disk"); ok && diskName.MatchString(name) {
		return "Disk " + n
	}
	first, size := utf8.DecodeRuneInString(name)
	return string(unicode.ToUpper(first)) + name[size:]
}

// childOrder puts the user shares first and the pools and disks after them,
// in the order Unraid lists them, so /mnt reads like the Main page.
func (r *Resolver) childOrder(dir string, names []string) {
	devices := r.current().devices
	rank := func(name string) int {
		p := path.Join(dir, name)
		switch {
		case p == "/mnt/user":
			return 0
		case slices.Contains(devices, name) && dir == "/mnt":
			return 1
		}
		return 2
	}
	slices.SortStableFunc(names, func(a, b string) int {
		if c := cmp.Compare(rank(a), rank(b)); c != 0 || rank(a) == 1 {
			return c
		}
		return naturalCompare(a, b)
	})
}

// naturalCompare orders disk2 before disk10.
func naturalCompare(a, b string) int {
	ta, na := splitNumber(a)
	tb, nb := splitNumber(b)
	if ta == tb && na >= 0 && nb >= 0 {
		return cmp.Compare(na, nb)
	}
	return strings.Compare(a, b)
}

func splitNumber(s string) (string, int) {
	i := len(s)
	for i > 0 && s[i-1] >= '0' && s[i-1] <= '9' {
		i--
	}
	n, err := strconv.Atoi(s[i:])
	if err != nil {
		return s, -1
	}
	return s[:i], n
}

// ExclusiveShares knows the user shares Unraid keeps on a single pool. Unraid
// makes such a share a link from /mnt/user/<share> to /mnt/<pool>/<share>, so
// that its files skip the user share file system.
type ExclusiveShares interface {
	// UserShares is the folder on disk that holds the user shares.
	UserShares() string
	// ExclusivePool returns the pool behind share when it is exclusive and
	// its link leads there.
	ExclusivePool(share string) (pool string, ok bool)
}

// WithExclusive returns a copy of r that treats the exclusive shares in x as
// the folders they stand for. Their files are reached on the pool, but the
// path, the rules and every answer stay with /mnt/user/<share>.
func (r *Resolver) WithExclusive(x ExclusiveShares) *Resolver {
	out := *r
	out.exclusive = x
	return &out
}

// exclusiveBase returns the folder on the pool behind the exclusive share that
// index, a path inside s, lies in, and the API path of that share.
func (r *Resolver) exclusiveBase(s *Source, index string) (base, mount string, ok bool) {
	if r.exclusive == nil || index == "/" || s.Root != filepath.Clean(r.exclusive.UserShares()) {
		return "", "", false
	}
	share, _, _ := strings.Cut(strings.TrimPrefix(index, "/"), "/")
	pool, ok := r.exclusive.ExclusivePool(share)
	if !ok {
		return "", "", false
	}
	return filepath.Join(filepath.Dir(s.Root), pool, share), path.Join(s.Mount, share), true
}

// RealIn returns the file on disk for the entry name in the folder dir. In
// the user shares an exclusive share is the folder on its pool, not the link.
func (r *Resolver) RealIn(dir Location, name string) string {
	if dir.Source != nil && dir.Index == "/" {
		if base, _, ok := r.exclusiveBase(dir.Source, "/"+name); ok {
			return base
		}
	}
	return filepath.Join(dir.Real, name)
}
