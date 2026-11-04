package unraid

import (
	"context"
	"slices"
)

// Where says on which disks a path really lies.
type Where struct {
	Path string `json:"path"`
	// Volume is user, user0 or the disk or pool in the path. It is empty
	// when the path is not on the array or a pool.
	Volume  string      `json:"volume,omitempty"`
	Share   string      `json:"share,omitempty"`
	Dir     bool        `json:"dir"`
	On      []Placement `json:"on"`
	Files   int         `json:"files"`
	Size    int64       `json:"size"`
	Partial bool        `json:"partial,omitempty"`
	Links   *Links      `json:"links,omitempty"`
}

// Placement is the part of a path that lies on one disk or pool.
type Placement struct {
	Device string `json:"device"`
	Files  int    `json:"files"`
	Size   int64  `json:"size"`
	Asleep bool   `json:"asleep"`
}

// Links are the names of a file with more than one hard link.
type Links struct {
	// Count is the link count of the file, this path included.
	Count int      `json:"count"`
	Paths []string `json:"paths"`
	// Complete is false when the search stopped before it found them all.
	Complete bool `json:"complete"`
}

// Where looks up apiPath, which must be clean. dir says whether the path
// is a folder.
func (s *Service) Where(ctx context.Context, apiPath string, dir bool) (Where, error) {
	out := Where{Path: apiPath, Dir: dir, On: []Placement{}}
	st, err := s.State()
	if err != nil {
		return out, err
	}
	p, ok := placeOf(st, apiPath)
	if !ok {
		return out, nil
	}
	out.Volume, out.Share = p.Vol, p.Share
	files, partial := s.survey(ctx, st, p)
	out.Partial = partial
	on := map[string]*Placement{}
	for _, f := range files {
		pl := on[f.dev]
		if pl == nil {
			d, _ := st.Disk(f.dev)
			pl = &Placement{Device: f.dev, Asleep: d.Spundown}
			on[f.dev] = pl
		}
		pl.Files++
		pl.Size += f.size
		out.Files++
		out.Size += f.size
	}
	for _, dev := range devicesFor(st, p) {
		if pl := on[dev]; pl != nil {
			out.On = append(out.On, *pl)
		}
	}
	if dir && len(out.On) == 0 {
		// An empty folder still lies somewhere.
		for _, dev := range s.DevicesOf(apiPath) {
			d, _ := st.Disk(dev)
			out.On = append(out.On, Placement{Device: dev, Asleep: d.Spundown})
		}
	}
	if !dir && len(files) == 1 && files[0].ino.nlink > 1 {
		f := files[0]
		rels, complete := s.otherNames(ctx, f)
		out.Links = &Links{Count: int(f.ino.nlink), Paths: []string{}, Complete: complete}
		for _, r := range rels {
			out.Links.Paths = append(out.Links.Paths, shownPath(st, f.dev, r))
		}
		slices.Sort(out.Links.Paths)
	}
	return out, nil
}
