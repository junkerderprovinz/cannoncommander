package unraid

import (
	"cmp"
	"context"
	"errors"
	"slices"
	"strings"
)

// Gather modes, after the unbalanced plugin.
const (
	// ModeGather brings a folder onto one disk, so playing it wakes only
	// that one.
	ModeGather = "gather"
	// ModeScatter spreads the part of a folder on one disk over the
	// others, for example before the disk is replaced.
	ModeScatter = "scatter"
)

var (
	// ErrNotUserFolder is returned for a gather path outside /mnt/user.
	ErrNotUserFolder = errors.New("unraid: gather needs a folder in a user share")
	errBadMode       = errors.New("unraid: mode must be gather or scatter")
)

// GatherRequest asks for a plan. Disk is the target for gather and the
// source for scatter, empty to let the plan suggest one.
type GatherRequest struct {
	Path string `json:"path"`
	Mode string `json:"mode"`
	Disk string `json:"disk,omitempty"`
}

// GatherPlan is how the folder lies now and after the moves.
type GatherPlan struct {
	Path    string         `json:"path"`
	Mode    string         `json:"mode"`
	Disk    string         `json:"disk,omitempty"`
	Files   int            `json:"files"`
	Size    int64          `json:"size"`
	Devices []GatherDevice `json:"devices"`
	// Options are the disks to choose from: every array disk with what it
	// would have to take for gather, the disks holding part of the folder
	// for scatter.
	Options []GatherOption `json:"options"`
	Moves   int            `json:"moves"`
	Move    int64          `json:"move"`
	// Short is what does not fit anywhere and stays where it is.
	Short int64 `json:"short"`
	// Cached counts files still on a pool. The mover takes care of them.
	Cached  int  `json:"cached"`
	Partial bool `json:"partial,omitempty"`

	moves []Move
}

// GatherDevice is one disk or pool with its share of the folder.
type GatherDevice struct {
	Device string `json:"device"`
	Now    int64  `json:"now"`
	Files  int    `json:"files"`
	After  int64  `json:"after"`
	Free   int64  `json:"free"`
	Asleep bool   `json:"asleep"`
}

// GatherOption is a disk the plan can use, with its cost.
type GatherOption struct {
	Disk  string `json:"disk"`
	Move  int64  `json:"move"`
	Short int64  `json:"short"`
}

// Move is one file going from one disk to another. Rel is below /mnt/<disk>.
type Move struct {
	Rel  string
	From string
	To   string
	Size int64
}

// MoveList returns the moves the plan makes, for the job that runs it.
func (p GatherPlan) MoveList() []Move { return p.moves }

// PlanGather works out a gather or scatter for a folder in a user share.
func (s *Service) PlanGather(ctx context.Context, req GatherRequest) (GatherPlan, error) {
	plan := GatherPlan{Path: req.Path, Mode: req.Mode, Devices: []GatherDevice{}, Options: []GatherOption{}}
	if req.Mode != ModeGather && req.Mode != ModeScatter {
		return plan, errBadMode
	}
	st, err := s.State()
	if err != nil {
		return plan, err
	}
	p, ok := placeOf(st, req.Path)
	if !ok || p.Vol != "user" || p.Share == "" {
		return plan, ErrNotUserFolder
	}
	files, partial := s.survey(ctx, st, p)
	plan.Partial = partial
	disks := st.DataDisks()
	var array []file
	on := map[string]*GatherDevice{}
	for _, f := range files {
		plan.Files++
		plan.Size += f.size
		g := on[f.dev]
		if g == nil {
			g = &GatherDevice{Device: f.dev}
			on[f.dev] = g
		}
		g.Now += f.size
		g.Files++
		if st.isPool(f.dev) {
			plan.Cached++
		} else {
			array = append(array, f)
		}
	}

	if req.Mode == ModeGather {
		for _, d := range disks {
			_, size, short := gatherMoves(st, array, d)
			plan.Options = append(plan.Options, GatherOption{Disk: d, Move: size, Short: short})
		}
		plan.Disk = choose(plan.Options, req.Disk, cheapest)
		if plan.Disk != "" {
			plan.moves, plan.Move, plan.Short = gatherMoves(st, array, plan.Disk)
		}
	} else {
		for _, d := range disks {
			if g := on[d]; g != nil {
				plan.Options = append(plan.Options, GatherOption{Disk: d, Move: g.Now})
			}
		}
		plan.Disk = choose(plan.Options, req.Disk, largest)
		if plan.Disk != "" {
			plan.moves, plan.Move, plan.Short = scatterMoves(st, array, plan.Disk, p.Rel)
		}
	}
	plan.Moves = len(plan.moves)

	after := map[string]int64{}
	for dev, g := range on {
		after[dev] = g.Now
	}
	for _, m := range plan.moves {
		after[m.From] -= m.Size
		after[m.To] += m.Size
	}
	for _, dev := range st.Devices() {
		g := on[dev]
		if g == nil {
			if st.isPool(dev) {
				continue
			}
			g = &GatherDevice{Device: dev}
		}
		d, _ := st.Disk(dev)
		g.After, g.Free, g.Asleep = after[dev], d.Free, d.Spundown
		plan.Devices = append(plan.Devices, *g)
	}
	return plan, nil
}

// choose returns the wanted disk when it is an option, else the one the
// rule suggests.
func choose(opts []GatherOption, want string, rule func([]GatherOption) string) string {
	for _, o := range opts {
		if o.Disk == want {
			return want
		}
	}
	return rule(opts)
}

// cheapest is the gather target that has to take the least and has room
// for it, or the first disk when none has.
func cheapest(opts []GatherOption) string {
	best := -1
	for i, o := range opts {
		if o.Short == 0 && (best < 0 || o.Move < opts[best].Move) {
			best = i
		}
	}
	switch {
	case best >= 0:
		return opts[best].Disk
	case len(opts) > 0:
		return opts[0].Disk
	}
	return ""
}

// largest is the scatter source holding the most of the folder.
func largest(opts []GatherOption) string {
	best := ""
	var most int64 = -1
	for _, o := range opts {
		if o.Move > most {
			best, most = o.Disk, o.Move
		}
	}
	return best
}

func gatherMoves(st *State, array []file, to string) (moves []Move, size, short int64) {
	for _, f := range array {
		if f.dev != to {
			moves = append(moves, Move{Rel: f.rel, From: f.dev, To: to, Size: f.size})
			size += f.size
		}
	}
	return moves, size, max(0, size-diskFree(st, to))
}

// scatterMoves sends whole subfolders of the folder away from one disk,
// largest first, each to the disk with the most room left at that point.
func scatterMoves(st *State, array []file, from, folder string) (moves []Move, size, short int64) {
	type unit struct {
		files []file
		size  int64
	}
	byName := map[string]*unit{}
	var units []*unit
	for _, f := range array {
		if f.dev != from {
			continue
		}
		name, _, _ := strings.Cut(strings.TrimPrefix(f.rel, folder+"/"), "/")
		u := byName[name]
		if u == nil {
			u = &unit{}
			byName[name] = u
			units = append(units, u)
		}
		u.files = append(u.files, f)
		u.size += f.size
	}
	slices.SortStableFunc(units, func(a, b *unit) int { return cmp.Compare(b.size, a.size) })
	room := map[string]int64{}
	var targets []string
	for _, d := range st.DataDisks() {
		if d != from {
			room[d] = diskFree(st, d)
			targets = append(targets, d)
		}
	}
	for _, u := range units {
		to := ""
		for _, d := range targets {
			if to == "" || room[d] > room[to] {
				to = d
			}
		}
		if to == "" || room[to] < u.size {
			short += u.size
			continue
		}
		room[to] -= u.size
		for _, f := range u.files {
			moves = append(moves, Move{Rel: f.rel, From: from, To: to, Size: f.size})
			size += f.size
		}
	}
	return moves, size, short
}
