package unraid

import (
	"cmp"
	"os"
	"slices"
	"strconv"
	"strings"
)

// Problems the space check reports.
const (
	// SpaceNoDisk: no disk of the share has more than the minimum free
	// space, Unraid would not start the copy.
	SpaceNoDisk = "noDisk"
	// SpaceTooMuch: all files together are more than the share has left.
	SpaceTooMuch = "tooMuch"
	// SpaceBigFile: the disk Unraid would pick cannot hold the largest
	// file, the copy would stop halfway with "disk full".
	SpaceBigFile = "bigFile"
	// SpacePoolFull: the share lives on a pool only and the pool is full.
	SpacePoolFull = "poolFull"
)

// SpaceReport explains why a copy into a user share will not fit. Unraid
// picks the disk when a file is created and cannot know how large it gets,
// so a big file can run out of room while other disks still have plenty.
type SpaceReport struct {
	Problem   string    `json:"problem"`
	Share     string    `json:"share"`
	Allocator string    `json:"allocator"`
	Floor     int64     `json:"floor"`
	Total     int64     `json:"total"`
	Biggest   *FileSize `json:"biggest,omitempty"`
	// Room is the free space above the floor on all disks of the share.
	Room int64 `json:"room"`
	// Disk is the disk Unraid would pick, with its free space.
	Disk string `json:"disk,omitempty"`
	Free int64  `json:"free"`
	// Pool is set for SpacePoolFull.
	Pool string `json:"pool,omitempty"`
	// Roomy is the disk of the share with the most free space that still
	// holds the largest file, a safe target for copying to it directly.
	Roomy     string `json:"roomy,omitempty"`
	RoomyFree int64  `json:"roomyFree,omitempty"`
}

// FileSize names one file and its size.
type FileSize struct {
	Path string `json:"path"`
	Size int64  `json:"size"`
}

func diskFree(st *State, name string) int64 {
	d, _ := st.Disk(name)
	return d.Free
}

// pick returns the disk Unraid would write the next file to: among the
// candidates with more than the floor free, chosen by the share's
// allocation method. It returns "" when none qualifies.
func pick(st *State, sh Share, candidates []string) string {
	var disks []string
	for _, d := range candidates {
		if diskFree(st, d) > sh.Floor {
			disks = append(disks, d)
		}
	}
	if len(disks) == 0 {
		return ""
	}
	switch sh.Allocator {
	case "fillup":
		return disks[0]
	case "mostfree":
		best := disks[0]
		for _, d := range disks[1:] {
			if diskFree(st, d) > diskFree(st, best) {
				best = d
			}
		}
		return best
	}
	// High water starts at half the largest disk and halves the mark until
	// some disk has more free than that, so disks fill in even steps.
	var largest int64
	for _, d := range candidates {
		if disk, _ := st.Disk(d); disk.FsSize > largest {
			largest = disk.FsSize
		}
	}
	for mark := largest / 2; mark > sh.Floor; mark /= 2 {
		for _, d := range disks {
			if diskFree(st, d) >= mark {
				return d
			}
		}
	}
	return disks[0]
}

// splitCandidates narrows the share's disks by its split level. Below that
// level Unraid keeps a folder on the disk that already holds it, so the
// topmost folder that may not be split pins the disks.
func (s *Service) splitCandidates(sh Share, rel string) []string {
	level, err := strconv.Atoi(sh.SplitLevel)
	if sh.SplitLevel == "" || err != nil {
		return sh.Disks
	}
	parts := strings.Split(rel, "/")
	if len(parts) <= level {
		return sh.Disks
	}
	top := strings.Join(parts[:level+1], "/")
	var on []string
	for _, d := range sh.Disks {
		if info, err := os.Stat(s.real(d, top)); err == nil && info.IsDir() {
			on = append(on, d)
		}
	}
	if len(on) == 0 {
		return sh.Disks
	}
	return on
}

// checkSpace tests whether files fit into the user share folder dest. It
// returns nil when they do, and the disk the files land on when that is
// known.
func (s *Service) checkSpace(st *State, files []file, dest Place) (*SpaceReport, string) {
	sh, ok := st.Share(dest.Share)
	if !ok || len(files) == 0 {
		return nil, ""
	}
	var total int64
	big := files[0]
	for _, f := range files {
		total += f.size
		if f.size > big.size {
			big = f
		}
	}
	rep := &SpaceReport{
		Share: sh.Name, Allocator: sh.Allocator, Floor: sh.Floor, Total: total,
		Biggest: &FileSize{Path: shownPath(st, big.dev, big.rel), Size: big.size},
	}
	poolFits := func(name string) bool {
		pool, _ := st.Disk(name)
		return pool.Free-pool.Floor >= total
	}
	poolFull := func(name string) (*SpaceReport, string) {
		rep.Problem, rep.Pool, rep.Free = SpacePoolFull, name, diskFree(st, name)
		return rep, name
	}
	// A pool takes everything while it has room above its floor, after
	// that Unraid overflows into the secondary storage.
	if sh.Primary.Type == "pool" {
		if poolFits(sh.Primary.Name) {
			return nil, sh.Primary.Name
		}
		if sh.Secondary == nil {
			return poolFull(sh.Primary.Name)
		}
		if sh.Secondary.Type == "pool" {
			if poolFits(sh.Secondary.Name) {
				return nil, sh.Secondary.Name
			}
			return poolFull(sh.Secondary.Name)
		}
	}
	for _, d := range sh.Disks {
		rep.Room += max(0, diskFree(st, d)-sh.Floor)
	}
	var roomy []string
	for _, d := range sh.Disks {
		if diskFree(st, d) >= big.size {
			roomy = append(roomy, d)
		}
	}
	slices.SortStableFunc(roomy, func(a, b string) int { return cmp.Compare(diskFree(st, b), diskFree(st, a)) })
	if len(roomy) > 0 {
		rep.Roomy, rep.RoomyFree = roomy[0], diskFree(st, roomy[0])
	}
	disk := pick(st, sh, s.splitCandidates(sh, dest.Rel))
	rep.Disk, rep.Free = disk, diskFree(st, disk)
	switch {
	case disk == "":
		rep.Problem = SpaceNoDisk
	case total > rep.Room:
		rep.Problem = SpaceTooMuch
	case big.size > rep.Free:
		rep.Problem = SpaceBigFile
	default:
		return nil, disk
	}
	return rep, disk
}
