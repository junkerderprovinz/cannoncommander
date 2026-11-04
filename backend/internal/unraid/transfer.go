package unraid

import (
	"cmp"
	"context"
	"path"
	"slices"
)

// How many trapped files a report names, and how many broken links it
// looks up. The rest only counts, finding link names walks a whole disk.
const (
	reportFiles = 50
	reportLinks = 6
)

// Transfer is a planned copy, move or link of paths into the folder Dest.
type Transfer struct {
	Kind  string   `json:"kind"`
	Paths []string `json:"paths"`
	Dest  string   `json:"dest"`
}

// TransferReport is what Unraid adds to the check before a transfer.
type TransferReport struct {
	Trap  *Trap        `json:"trap,omitempty"`
	Links *LinkReport  `json:"links,omitempty"`
	Space *SpaceReport `json:"space,omitempty"`
	// Linkable says that hard links can stand in for the copy, because
	// every file can live at the target on the disk it already lies on.
	Linkable bool `json:"linkable"`
	// Wakes are the sleeping disks the transfer spins up.
	Wakes []string `json:"wakes"`
	// Lands is the disk or pool new files go to, when that is known.
	Lands   string `json:"lands,omitempty"`
	Partial bool   `json:"partial,omitempty"`
}

// Trap reports files that are the same file at both ends: one side is a
// disk share, the other the user share above it. Copying or moving
// between them truncates the files to 0 bytes.
type Trap struct {
	// Device is the disk that holds most of the trapped files.
	Device string     `json:"device"`
	Count  int        `json:"count"`
	Size   int64      `json:"size"`
	Files  []TrapFile `json:"files"`
	// Targets are the other array disks, most free space first, for a
	// safe copy from disk to disk.
	Targets []DiskRoom `json:"targets"`
}

// TrapFile is one file caught in the trap.
type TrapFile struct {
	Path   string `json:"path"`
	Device string `json:"device"`
	Size   int64  `json:"size"`
}

// DiskRoom is a disk with its free space.
type DiskRoom struct {
	Disk string `json:"disk"`
	Free int64  `json:"free"`
}

// LinkReport lists files whose hard links the transfer would break: a copy
// always makes new data, a move does once it crosses a share or a disk.
type LinkReport struct {
	Count int          `json:"count"`
	Extra int64        `json:"extra"`
	Files []LinkedFile `json:"files"`
}

// LinkedFile is a file with other names that keep the old data.
type LinkedFile struct {
	Path     string   `json:"path"`
	Size     int64    `json:"size"`
	Names    int      `json:"names"`
	Others   []string `json:"others"`
	Complete bool     `json:"complete"`
}

type picked struct {
	src   Place
	files []file
}

// CheckTransfer looks at a transfer with Unraid's eyes. Paths and Dest are
// clean API paths the caller already checked.
func (s *Service) CheckTransfer(ctx context.Context, t Transfer) (TransferReport, error) {
	rep := TransferReport{Wakes: []string{}}
	st, err := s.State()
	if err != nil {
		return rep, err
	}
	dest, destOK := placeOf(st, t.Dest)
	var sel []picked
	var all []file
	for _, p := range t.Paths {
		src, ok := placeOf(st, p)
		if !ok {
			continue
		}
		files, partial := s.survey(ctx, st, src)
		rep.Partial = rep.Partial || partial
		sel = append(sel, picked{src: src, files: files})
		all = append(all, files...)
	}
	if len(all) == 0 {
		return rep, nil
	}
	if destOK && t.Kind != "link" {
		rep.Trap = s.trap(st, sel, dest)
	}
	if destOK {
		rep.Linkable = linkable(st, sel, dest)
	}
	if t.Kind != "link" {
		rep.Links = s.brokenLinks(ctx, st, t.Kind, all, dest, destOK)
	}

	var lands string
	if destOK && !dest.User() {
		lands = dest.Vol
	}
	if destOK && dest.User() && dest.Share != "" && t.Kind != "link" && !(t.Kind == "move" && sameShare(sel, dest)) {
		rep.Space, lands = s.checkSpace(st, all, dest)
	}
	rep.Lands = lands
	wakes := map[string]bool{}
	for _, f := range all {
		wakes[f.dev] = true
	}
	if lands != "" {
		wakes[lands] = true
	}
	for dev := range wakes {
		if d, ok := st.Disk(dev); ok && d.Spundown {
			rep.Wakes = append(rep.Wakes, dev)
		}
	}
	slices.Sort(rep.Wakes)
	return rep, nil
}

// sameShare reports a move inside one user share, which is a rename on
// the disk the file already lies on.
func sameShare(sel []picked, dest Place) bool {
	for _, p := range sel {
		if !p.src.User() || p.src.Share != dest.Share {
			return false
		}
	}
	return true
}

func (s *Service) trap(st *State, sel []picked, dest Place) *Trap {
	var hits []file
	for _, p := range sel {
		if p.src.User() == dest.User() || path.Dir("/"+p.src.Rel) != "/"+dest.Rel {
			continue
		}
		disk := p.src.Vol
		if p.src.User() {
			disk = dest.Vol
		}
		for _, f := range p.files {
			if f.dev == disk {
				hits = append(hits, f)
			}
		}
	}
	if len(hits) == 0 {
		return nil
	}
	tr := &Trap{Count: len(hits), Files: []TrapFile{}, Targets: []DiskRoom{}}
	per := map[string]int{}
	for i, f := range hits {
		tr.Size += f.size
		per[f.dev]++
		if i < reportFiles {
			tr.Files = append(tr.Files, TrapFile{Path: shownPath(st, f.dev, f.rel), Device: f.dev, Size: f.size})
		}
	}
	for dev, n := range per {
		if n > per[tr.Device] || (n == per[tr.Device] && dev < tr.Device) {
			tr.Device = dev
		}
	}
	for _, d := range st.DataDisks() {
		if d != tr.Device {
			tr.Targets = append(tr.Targets, DiskRoom{Disk: d, Free: diskFree(st, d)})
		}
	}
	slices.SortStableFunc(tr.Targets, func(a, b DiskRoom) int { return cmp.Compare(b.Free, a.Free) })
	return tr
}

// allowed reports whether a file on dev may live in share, which is what
// a hard link there needs: it stays on the disk of its source.
func allowed(st *State, dev, share string) bool {
	sh, ok := st.Share(share)
	if !ok {
		return false
	}
	if st.isPool(dev) {
		return sh.Primary.Name == dev || (sh.Secondary != nil && sh.Secondary.Name == dev)
	}
	return slices.Contains(sh.Disks, dev)
}

func linkable(st *State, sel []picked, dest Place) bool {
	if dest.User() && dest.Share == "" {
		return false
	}
	any := false
	for _, p := range sel {
		if p.src.Vol == dest.Vol && path.Dir("/"+p.src.Rel) == "/"+dest.Rel {
			return false
		}
		for _, f := range p.files {
			any = true
			if dest.User() && !allowed(st, f.dev, dest.Share) || !dest.User() && f.dev != dest.Vol {
				return false
			}
		}
	}
	return any
}

func (s *Service) brokenLinks(ctx context.Context, st *State, kind string, files []file, dest Place, destOK bool) *LinkReport {
	rep := &LinkReport{Files: []LinkedFile{}}
	for _, f := range files {
		if f.ino.nlink < 2 {
			continue
		}
		keeps := kind == "move" && destOK && (dest.User() && dest.Share == f.share() || !dest.User() && dest.Vol == f.dev)
		if keeps {
			continue
		}
		rep.Count++
		rep.Extra += f.size
		if len(rep.Files) >= reportLinks {
			continue
		}
		lf := LinkedFile{Path: shownPath(st, f.dev, f.rel), Size: f.size, Names: int(f.ino.nlink), Others: []string{}}
		rels, complete := s.otherNames(ctx, f)
		lf.Complete = complete
		for _, r := range rels {
			lf.Others = append(lf.Others, shownPath(st, f.dev, r))
		}
		rep.Files = append(rep.Files, lf)
	}
	if rep.Count == 0 {
		return nil
	}
	return rep
}
