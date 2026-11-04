package unraid

import (
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"strconv"
	"strings"
	"time"
)

// Disk is one array slot, pool or the flash drive, as disks.ini lists it.
// Sizes are bytes.
type Disk struct {
	Name string `json:"name"`
	// Kind is parity, data, pool or flash.
	Kind string `json:"kind"`
	// Pool is set on the second and later devices of a pool.
	Pool       string `json:"pool,omitempty"`
	Mount      string `json:"mount,omitempty"`
	Mounted    bool   `json:"mounted"`
	Status     string `json:"status"`
	FsStatus   string `json:"fsStatus,omitempty"`
	FsType     string `json:"fsType,omitempty"`
	Size       int64  `json:"size"`
	FsSize     int64  `json:"fsSize"`
	Free       int64  `json:"free"`
	Used       int64  `json:"used"`
	Floor      int64  `json:"floor,omitempty"`
	Spundown   bool   `json:"spundown"`
	Rotational bool   `json:"rotational"`
	Temp       *int   `json:"temp,omitempty"`
	Errors     int64  `json:"errors"`
	Color      string `json:"color,omitempty"`
}

// Storage is where a share keeps its files: the array or a named pool.
type Storage struct {
	Type string `json:"type"`
	Name string `json:"name,omitempty"`
}

// Share is one user share with the settings that decide where its files go.
type Share struct {
	Name      string   `json:"name"`
	Comment   string   `json:"comment,omitempty"`
	Primary   Storage  `json:"primary"`
	Secondary *Storage `json:"secondary,omitempty"`
	// Mover is toSecondary when the mover empties the primary storage, as
	// for "Cache to Array", and toPrimary for the other direction.
	Mover      string   `json:"mover,omitempty"`
	Allocator  string   `json:"allocator"`
	SplitLevel string   `json:"splitLevel"`
	Floor      int64    `json:"floor"`
	Include    []string `json:"include,omitempty"`
	Exclude    []string `json:"exclude,omitempty"`
	// Disks are the array disks the share may write to, after the global
	// and the share's own include and exclude lists.
	Disks     []string `json:"disks"`
	Free      int64    `json:"free"`
	Used      int64    `json:"used"`
	Exclusive bool     `json:"exclusive"`
}

// Array is the state of the md array.
type Array struct {
	State       string `json:"state"`
	Started     bool   `json:"started"`
	Disks       int64  `json:"disks"`
	Disabled    int64  `json:"disabled"`
	Invalid     int64  `json:"invalid"`
	Missing     int64  `json:"missing"`
	FsState     string `json:"fsState,omitempty"`
	Unmountable int64  `json:"unmountable"`
}

// Parity is the parity check, rebuild or clear the md driver is running.
type Parity struct {
	Running bool `json:"running"`
	Paused  bool `json:"paused"`
	// Kind is check, rebuild or clear while one runs or is paused.
	Kind    string  `json:"kind,omitempty"`
	Action  string  `json:"action,omitempty"`
	Correct bool    `json:"correct,omitempty"`
	Pos     int64   `json:"pos"`
	Size    int64   `json:"size"`
	Percent float64 `json:"percent"`
	// Speed is bytes per second over the last sample.
	Speed      int64      `json:"speed,omitempty"`
	LastSync   *time.Time `json:"lastSync,omitempty"`
	LastErrors int64      `json:"lastErrors"`
	// LastExit is 0 for a finished run, negative when it was cancelled.
	LastExit int64 `json:"lastExit"`
}

// Mover is when the mover carries files between the pools and the array.
type Mover struct {
	// Schedule is the crontab line from the scheduler settings.
	Schedule string `json:"schedule,omitempty"`
	Active   bool   `json:"active"`
}

// Busy reports whether the run keeps the disks busy right now.
func (p Parity) Busy() bool { return p.Running && !p.Paused }

// State is everything the emhttp files say about the server.
type State struct {
	// Available is false when the files are missing, outside Unraid.
	Available bool    `json:"available"`
	Version   string  `json:"version,omitempty"`
	Name      string  `json:"name,omitempty"`
	Array     Array   `json:"array"`
	Parity    Parity  `json:"parity"`
	Disks     []Disk  `json:"disks"`
	Shares    []Share `json:"shares"`
	// Write tells the interface whether spinning down and notifications
	// are switched on.
	Write bool `json:"write"`
	// SpindownDelay is the default in minutes, 0 when disks never sleep.
	SpindownDelay int64 `json:"spindownDelay"`
	Mover         Mover `json:"mover"`

	includeAll, excludeAll []string
}

// Disk returns the disk or pool with the given name.
func (s *State) Disk(name string) (Disk, bool) {
	for _, d := range s.Disks {
		if d.Name == name {
			return d, true
		}
	}
	return Disk{}, false
}

// Share returns the share with the given name.
func (s *State) Share(name string) (Share, bool) {
	for _, sh := range s.Shares {
		if sh.Name == name {
			return sh, true
		}
	}
	return Share{}, false
}

// Devices returns the mounted pools and data disks, the places a file in
// a user share can really lie. Pools come first, as in /mnt.
func (s *State) Devices() []string {
	var pools, data []string
	for _, d := range s.Disks {
		if !d.Mounted || d.Pool != "" {
			continue
		}
		switch d.Kind {
		case "pool":
			pools = append(pools, d.Name)
		case "data":
			data = append(data, d.Name)
		}
	}
	return append(pools, data...)
}

// DataDisks returns the mounted array disks in slot order.
func (s *State) DataDisks() []string {
	var out []string
	for _, d := range s.Disks {
		if d.Kind == "data" && d.Mounted {
			out = append(out, d.Name)
		}
	}
	return out
}

func (s *State) isPool(name string) bool {
	d, ok := s.Disk(name)
	return ok && d.Kind == "pool"
}

// ReadState reads the emhttp files, /proc/mdstat and the share settings.
// Missing files are not an error, the state then says less.
func ReadState(c Config) (*State, error) {
	st := &State{Disks: []Disk{}, Shares: []Share{}, Write: c.Write}
	vars, err := readSections(filepath.Join(c.Emhttp, "var.ini"))
	if err != nil {
		return nil, err
	}
	var v section
	if len(vars) > 0 {
		v = vars[0]
		st.Available = true
		st.Version = v.str("version")
		st.Name = v.str("NAME")
		st.SpindownDelay = v.int("spindownDelay")
		st.Mover = Mover{Schedule: v.str("shareMoverSchedule"), Active: v.str("shareMoverActive") == "yes"}
		st.includeAll = list(v.str("shareUserInclude"))
		st.excludeAll = list(v.str("shareUserExclude"))
		st.Array = Array{
			State:       v.str("mdState"),
			Started:     v.str("mdState") == "STARTED",
			Disks:       v.int("mdNumDisks"),
			Disabled:    v.int("mdNumDisabled"),
			Invalid:     v.int("mdNumInvalid"),
			Missing:     v.int("mdNumMissing"),
			FsState:     v.str("fsState"),
			Unmountable: v.int("fsNumUnmountable"),
		}
	}
	md, err := readSections(c.Mdstat)
	if err != nil {
		return nil, err
	}
	if len(md) > 0 {
		st.Parity = parityFrom(md[0])
	} else if st.Available {
		st.Parity = parityFrom(v)
	}

	disks, err := readSections(filepath.Join(c.Emhttp, "disks.ini"))
	if err != nil {
		return nil, err
	}
	for _, s := range disks {
		if d, ok := diskFrom(s); ok {
			st.Disks = append(st.Disks, d)
		}
	}
	markPoolMembers(st.Disks)

	shares, err := readSections(filepath.Join(c.Emhttp, "shares.ini"))
	if err != nil {
		return nil, err
	}
	seen := map[string]bool{}
	for _, s := range shares {
		sh := shareFrom(st, s, "")
		st.Shares = append(st.Shares, sh)
		seen[sh.Name] = true
	}
	// With the array stopped emhttp has no shares.ini, the settings on the
	// flash drive still say how each share is set up.
	cfgs, err := readCfgs(c.Shares)
	if err != nil {
		return nil, err
	}
	for _, s := range cfgs {
		if !seen[s.name] {
			st.Shares = append(st.Shares, shareFrom(st, s, "share"))
		}
	}
	slices.SortFunc(st.Shares, func(a, b Share) int { return strings.Compare(a.Name, b.Name) })
	return st, nil
}

func readSections(p string) ([]section, error) {
	data, err := os.ReadFile(p)
	if errors.Is(err, fs.ErrNotExist) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return parseINI(data), nil
}

func readCfgs(dir string) ([]section, error) {
	entries, err := os.ReadDir(dir)
	if errors.Is(err, fs.ErrNotExist) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	var out []section
	for _, e := range entries {
		name, ok := strings.CutSuffix(e.Name(), ".cfg")
		if !ok || e.IsDir() {
			continue
		}
		secs, err := readSections(filepath.Join(dir, e.Name()))
		if err != nil {
			return nil, err
		}
		if len(secs) > 0 {
			secs[0].name = name
			out = append(out, secs[0])
		}
	}
	return out, nil
}

func parityFrom(s section) Parity {
	p := Parity{
		Action:     s.str("mdResyncAction"),
		Correct:    s.int("mdResyncCorr") != 0,
		Pos:        s.kib("mdResyncPos"),
		Size:       s.kib("mdResyncSize"),
		LastErrors: s.int("sbSyncErrs"),
		LastExit:   s.int("sbSyncExit"),
	}
	// mdResync holds the size while a run moves and drops to 0 when it is
	// paused, the position stays where it stopped.
	if p.Pos > 0 {
		p.Running = true
		p.Paused = s.int("mdResync") == 0
		if p.Size > 0 {
			p.Percent = float64(int64(float64(p.Pos)/float64(p.Size)*1000)) / 10
		}
		if dt := s.int("mdResyncDt"); dt > 0 {
			p.Speed = s.kib("mdResyncDb") / dt
		}
		switch {
		case strings.HasPrefix(p.Action, "check"):
			p.Kind = "check"
		case strings.HasPrefix(p.Action, "recon"):
			p.Kind = "rebuild"
		case strings.HasPrefix(p.Action, "clear"):
			p.Kind = "clear"
		}
	} else {
		p.Action = ""
	}
	if t := s.int("sbSynced"); t > 0 {
		at := time.Unix(t, 0).UTC()
		p.LastSync = &at
	}
	return p
}

func diskFrom(s section) (Disk, bool) {
	status := s.str("status")
	// Empty slots are listed too, up to the licence limit.
	if strings.HasPrefix(status, "DISK_NP") {
		return Disk{}, false
	}
	d := Disk{
		Name:       s.str("name"),
		Status:     status,
		FsStatus:   s.str("fsStatus"),
		FsType:     s.str("fsType"),
		Mount:      s.str("fsMountpoint"),
		Size:       s.kib("size"),
		FsSize:     s.kib("fsSize"),
		Free:       s.kib("fsFree"),
		Used:       s.kib("fsUsed"),
		Spundown:   s.str("spundown") == "1",
		Rotational: s.str("rotational") == "1",
		Errors:     s.int("numErrors"),
		Color:      s.str("color"),
	}
	if d.Name == "" {
		d.Name = s.name
	}
	d.Mounted = d.FsStatus == "Mounted"
	if t, err := strconv.Atoi(s.str("temp")); err == nil {
		d.Temp = &t
	}
	switch s.str("type") {
	case "Parity":
		d.Kind = "parity"
	case "Data":
		d.Kind = "data"
	case "Cache":
		d.Kind = "pool"
		d.Floor = floorBytes(s.str("shareFloor"))
	case "Flash":
		d.Kind = "flash"
	default:
		d.Kind = strings.ToLower(s.str("type"))
	}
	return d, true
}

var memberName = regexp.MustCompile(`^(.+?)([0-9]+)$`)

// markPoolMembers finds the second and later devices of multi device
// pools. emhttp names them after the pool with a number, cache2, cache3,
// and only the first device carries the file system.
func markPoolMembers(disks []Disk) {
	pools := map[string]bool{}
	for _, d := range disks {
		if d.Kind == "pool" {
			pools[d.Name] = true
		}
	}
	for i, d := range disks {
		if d.Kind != "pool" {
			continue
		}
		m := memberName.FindStringSubmatch(d.Name)
		if m != nil && m[2] != "1" && pools[m[1]] {
			disks[i].Pool = m[1]
			disks[i].Mounted = false
		}
	}
}

// shareFrom reads a share from shares.ini, or from a share's .cfg file
// where every key carries the prefix "share" and starts upper case.
func shareFrom(st *State, s section, prefix string) Share {
	key := func(k string) string {
		if prefix == "" {
			return k
		}
		return prefix + strings.ToUpper(k[:1]) + k[1:]
	}
	sh := Share{
		Name:       s.str("name"),
		Comment:    s.str(key("comment")),
		Allocator:  s.str(key("allocator")),
		SplitLevel: s.str(key("splitLevel")),
		Floor:      floorBytes(s.str(key("floor"))),
		Include:    list(s.str(key("include"))),
		Exclude:    list(s.str(key("exclude"))),
		Free:       s.kib("free"),
		Used:       s.kib("used"),
		Exclusive:  s.yes("exclusive"),
		Disks:      []string{},
	}
	if sh.Name == "" {
		sh.Name = s.name
	}
	if sh.Allocator == "" {
		sh.Allocator = "highwater"
	}
	pool, pool2 := s.str(key("cachePool")), s.str(key("cachePool2"))
	secondary := Storage{Type: "array"}
	if pool2 != "" {
		secondary = Storage{Type: "pool", Name: pool2}
	}
	switch s.str(key("useCache")) {
	case "only":
		sh.Primary = Storage{Type: "pool", Name: pool}
	case "yes":
		sh.Primary = Storage{Type: "pool", Name: pool}
		sh.Secondary, sh.Mover = &secondary, "toSecondary"
	case "prefer":
		sh.Primary = Storage{Type: "pool", Name: pool}
		sh.Secondary, sh.Mover = &secondary, "toPrimary"
	default:
		sh.Primary = Storage{Type: "array"}
	}
	if sh.Primary.Type == "pool" && sh.Primary.Name == "" {
		sh.Primary.Name = "cache"
	}
	if sh.Primary.Type == "array" || (sh.Secondary != nil && sh.Secondary.Type == "array") {
		sh.Disks = shareDisks(st, sh)
	}
	return sh
}

func shareDisks(st *State, sh Share) []string {
	out := []string{}
	for _, d := range st.DataDisks() {
		if len(st.includeAll) > 0 && !slices.Contains(st.includeAll, d) {
			continue
		}
		if slices.Contains(st.excludeAll, d) {
			continue
		}
		if len(sh.Include) > 0 && !slices.Contains(sh.Include, d) {
			continue
		}
		if slices.Contains(sh.Exclude, d) {
			continue
		}
		out = append(out, d)
	}
	return out
}

// floorBytes reads a minimum free space. emhttp stores KiB, the share
// settings page also accepts a unit such as 50GB.
func floorBytes(v string) int64 {
	v = strings.TrimSpace(strings.ToUpper(v))
	if v == "" {
		return 0
	}
	v = strings.TrimSuffix(v, "B")
	mult := int64(1024)
	if n := len(v); n > 0 {
		switch v[n-1] {
		case 'K':
			v = v[:n-1]
		case 'M':
			mult, v = 1<<20, v[:n-1]
		case 'G':
			mult, v = 1<<30, v[:n-1]
		case 'T':
			mult, v = 1<<40, v[:n-1]
		}
	}
	f, err := strconv.ParseFloat(strings.TrimSpace(v), 64)
	if err != nil || f < 0 {
		return 0
	}
	return int64(f * float64(mult))
}
