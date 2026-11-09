package ccfs

import (
	"path"
	"path/filepath"
	"slices"
	"strings"
)

// Drive is what the computer tells about a volume.
type Drive struct {
	// Label is the name the volume was given, "" when it has none.
	Label string
	// Kind is fixed, removable, optical, network or virtual.
	Kind string
	// Remote is the share a network drive stands for, as \\nas\daten.
	Remote string
	// Eject says whether the volume can be ejected, as a stick, a disc or
	// an attached disk image can.
	Eject bool
}

// Volume is a drive the computer reports, such as E: for E:\.
type Volume struct {
	Name string
	Root string
	Drive
}

// SetVolumes shows the drives the computer reports, each as a source of its
// own unless another source covers it. A source from the settings at the
// root of a drive the computer no longer reports is left out until the
// drive is back, as Explorer leaves out a stick that was pulled.
func (r *Resolver) SetVolumes(vols []Volume) {
	list := make([]Volume, len(vols))
	for i, v := range vols {
		v.Root = cleanRoot(v.Root)
		list[i] = v
	}
	r.update(func(cur *sourceSet) *sourceSet {
		next := *cur
		next.volumes = list
		return next.laidOut()
	})
}

// Volumes returns the sources that are drives, a configured source at the
// root of one included.
func (r *Resolver) Volumes() []Source {
	var out []Source
	for _, s := range r.current().sources {
		if s.Drive != nil {
			out = append(out, *s)
		}
	}
	return out
}

// VolumeAt returns the drive the computer reports that holds p, the deepest
// one, and the API path it starts at. That also finds a drive inside another
// source, such as a stick or a share below / on Linux. ok is false for a path
// on no drive of its own, as everywhere in the container.
func (r *Resolver) VolumeAt(p string) (mount string, v Volume, ok bool) {
	cur := r.current()
	consider := func(m string, cand Volume) {
		if within(p, m) && len(m) > len(mount) {
			mount, v, ok = m, cand, true
		}
	}
	for _, vol := range cur.volumes {
		consider(cur.apiPath(vol.Root), vol)
	}
	for _, sh := range cur.shares {
		consider(sh.Mount, Volume{Name: sh.Name, Root: sh.Root, Drive: *sh.Drive})
	}
	return mount, v, ok
}

// apiPath is the API path of the folder dir on disk: below the deepest
// source that holds it, or as MountFor writes it when none does.
func (s *sourceSet) apiPath(dir string) string {
	best, out := "", path.Clean(MountFor(dir))
	for _, src := range s.sources {
		if inside(src.Root, dir) && len(src.Root) > len(best) {
			rel, err := filepath.Rel(src.Root, dir)
			if err != nil {
				continue
			}
			best, out = src.Root, path.Join(src.Mount, filepath.ToSlash(rel))
		}
	}
	return out
}

// laidOut returns s with its sources built from the rest: the configured ones
// first, then the pools and disks in the order Unraid lists them, the drives
// and the network shares that none of them covers. Only a configured source
// at / holds others, and it comes first.
func (s *sourceSet) laidOut() *sourceSet {
	next := *s
	next.sources = nil
	drives := map[string]Drive{}
	for _, v := range s.volumes {
		drives[v.Root] = v.Drive
	}
	names := map[string]bool{}
	for _, src := range s.configured {
		c := *src
		c.AdminOnly = c.AdminOnly || next.adminOnly(c.Mount)
		if d, ok := drives[c.Root]; ok {
			c.Drive = &d
		} else if len(s.volumes) > 0 && isDriveRoot(c.Root) {
			continue
		}
		next.sources = append(next.sources, &c)
		names[strings.ToLower(c.Name)] = true
	}
	configured := slices.Clone(next.sources)
	covered := func(mount string) bool {
		return slices.ContainsFunc(configured, func(c *Source) bool {
			return within(mount, c.Mount) || within(c.Mount, mount)
		})
	}
	next.sources = append(next.sources, next.deviceSources(covered, names)...)
	for _, v := range s.volumes {
		mount := next.apiPath(v.Root)
		if covered(mount) {
			continue
		}
		d := v.Drive
		next.sources = append(next.sources, &Source{Name: v.Name, Mount: mount, Root: v.Root, Drive: &d})
	}
	for _, sh := range s.shares {
		if !covered(sh.Mount) {
			next.sources = append(next.sources, sh)
		}
	}
	next.close()
	return &next
}

// isDriveRoot reports whether p is the top of a Windows drive or share, as
// C:\ or \\nas\daten\ are.
func isDriveRoot(p string) bool {
	vol := filepath.VolumeName(p)
	return vol != "" && strings.TrimRight(p[len(vol):], `\/`) == ""
}

// cleanRoot cleans the folder on disk a source starts at. The top of a share
// keeps its closing backslash, as the top of a drive does, because
// filepath.Rel never returns for \\nas\daten against \\nas\daten\.
func cleanRoot(p string) string {
	p = filepath.Clean(p)
	if strings.HasPrefix(p, `\\`) && p == filepath.VolumeName(p) {
		return p + `\`
	}
	return p
}

// netRoot is the API path of the network, where \\nas\daten shows up as
// /net/nas/daten.
const netRoot = "/net"

// Network reaches the folders other computers share, by the names they have
// in \\server\share.
type Network interface {
	// Share returns the folder on disk for share on server, if it is there.
	Share(server, share string) (root string, ok bool)
	// Shares lists what server shares.
	Shares(server string) []string
}

// WithNetwork returns a copy of r that reaches the shares in n below /net.
// A share becomes a source of its own the first time a path asks for it.
func (r *Resolver) WithNetwork(n Network) *Resolver {
	out := *r
	out.network = n
	return &out
}

// netParts splits a path below /net into its server and share, which are ""
// for /net itself and for a server. ok is false outside /net or without a
// network.
func (r *Resolver) netParts(p string) (server, share string, ok bool) {
	if r.network == nil || !within(p, netRoot) {
		return "", "", false
	}
	parts := strings.SplitN(strings.TrimPrefix(p, netRoot+"/"), "/", 3)
	if p == netRoot {
		return "", "", true
	}
	if len(parts) == 1 {
		return parts[0], "", true
	}
	return parts[0], parts[1], true
}

// openShare makes share on server a source, and reports whether it is one.
func (r *Resolver) openShare(server, share string) bool {
	mount := path.Join(netRoot, server, share)
	root, ok := r.network.Share(server, share)
	if !ok {
		return false
	}
	src := &Source{Name: share, Mount: mount, Root: cleanRoot(root), Drive: &Drive{Kind: "network", Remote: root}}
	r.update(func(cur *sourceSet) *sourceSet {
		if slices.ContainsFunc(cur.shares, func(s *Source) bool { return s.Mount == mount }) {
			return cur
		}
		next := *cur
		next.shares = append(slices.Clone(cur.shares), src)
		return next.laidOut()
	})
	return slices.ContainsFunc(r.current().sources, func(s *Source) bool { return s.Mount == mount })
}
