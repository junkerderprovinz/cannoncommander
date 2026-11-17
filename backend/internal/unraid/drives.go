package unraid

import (
	"bufio"
	"bytes"
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"time"
)

// Drives is the answer of GET /unraid/drives: the pools with their
// devices, then the array's data disks and parity.
type Drives struct {
	Pools  []Drive      `json:"pools"`
	Array  []Drive      `json:"array"`
	Parity *ParityCheck `json:"parity,omitempty"`
}

// Drive is one pool or disk. Sizes are bytes.
type Drive struct {
	Name string `json:"name"`
	// Kind is pool, data or parity.
	Kind       string `json:"kind"`
	Device     string `json:"device,omitempty"`
	Status     string `json:"status"`
	FsType     string `json:"fsType,omitempty"`
	Profile    string `json:"profile,omitempty"`
	Transport  string `json:"transport,omitempty"`
	Rotational bool   `json:"rotational"`
	Size       int64  `json:"size"`
	FsSize     int64  `json:"fsSize"`
	Used       int64  `json:"used"`
	Asleep     bool   `json:"asleep"`
	// AsleepSince is missing when the disk slept before CannonCommander
	// started.
	AsleepSince *time.Time `json:"asleepSince,omitempty"`
	Temp        *int       `json:"temp,omitempty"`
	// TempAt is set when Temp is the last value read before the disk fell
	// asleep.
	TempAt  *time.Time `json:"tempAt,omitempty"`
	History []Sample   `json:"history"`
	// FullIn is the estimate of FullIn for the last 4, 8 and 12 weeks,
	// keyed by the weeks, where there is one.
	FullIn  map[string]float64 `json:"fullIn,omitempty"`
	Members []Drive            `json:"members,omitempty"`
	Zfs     *PoolFacts         `json:"zfs,omitempty"`
}

// PoolFacts are what ZFS keeps in memory about a pool.
type PoolFacts struct {
	Fragmentation *int    `json:"fragmentation,omitempty"`
	CompressRatio float64 `json:"compressRatio,omitempty"`
	Scrub         *Run    `json:"scrub,omitempty"`
}

// Run is a finished scrub or parity check.
type Run struct {
	At      time.Time `json:"at"`
	Seconds int64     `json:"seconds,omitempty"`
	Errors  int64     `json:"errors"`
}

// ParityCheck is the last parity run. Exit is 0 when it finished and
// negative when it was cancelled.
type ParityCheck struct {
	Run
	Exit int64 `json:"exit"`
}

// factsFor keeps the ZFS answers a while, so a window that opens and
// closes does not ask again each time.
const factsFor = 10 * time.Minute

// runner starts a program and returns what it printed.
type runner func(ctx context.Context, name string, args ...string) ([]byte, error)

func runTool(ctx context.Context, name string, args ...string) ([]byte, error) {
	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	return exec.CommandContext(ctx, name, args...).Output()
}

// Drives reports every pool and disk. It reads emhttp's files, the parity
// log on the flash drive and the history. A pool that is awake is asked
// about its ZFS properties, which the kernel answers from memory; a
// sleeping one keeps the answer from when it was awake.
func (s *Service) Drives(ctx context.Context) (*Drives, error) {
	st, err := s.State()
	if err != nil {
		return nil, err
	}
	out := &Drives{Pools: []Drive{}, Array: []Drive{}, Parity: s.lastParity(st)}
	var parity []Drive
	for _, d := range st.Disks {
		switch {
		case d.Kind == "pool" && d.Pool == "" && d.Mounted:
			out.Pools = append(out.Pools, s.pool(ctx, st, d))
		case d.Kind == "data" && d.Mounted:
			out.Array = append(out.Array, s.drive(d))
		case d.Kind == "parity":
			parity = append(parity, s.drive(d))
		}
	}
	out.Array = append(out.Array, parity...)
	return out, nil
}

func (s *Service) drive(d Disk) Drive {
	tr := s.history.Track(d.Name)
	out := Drive{
		Name: d.Name, Kind: d.Kind, Device: d.Device, Status: d.Status, FsType: d.FsType, Profile: d.Profile,
		Transport: d.Transport, Rotational: d.Rotational, Size: d.Size, Asleep: d.Spundown, Temp: d.Temp,
		History: tr.Days,
	}
	if out.History == nil {
		out.History = []Sample{}
	}
	if used, ok := usedOf(d); ok {
		out.FsSize, out.Used = d.FsSize, used
		for _, w := range []int{4, 8, 12} {
			if v, ok := FullIn(tr.Days, d.FsSize, w); ok {
				if out.FullIn == nil {
					out.FullIn = map[string]float64{}
				}
				out.FullIn[strconv.Itoa(w)] = v
			}
		}
	}
	if d.Spundown {
		if tr.AsleepSince > 0 {
			at := time.Unix(tr.AsleepSince, 0).UTC()
			out.AsleepSince = &at
		}
		if tr.Temp != nil {
			at := time.Unix(tr.TempAt, 0).UTC()
			out.Temp, out.TempAt = tr.Temp, &at
		}
	}
	return out
}

func (s *Service) pool(ctx context.Context, st *State, d Disk) Drive {
	out := s.drive(d)
	asleep := d.Spundown
	// A pool's own entry is also its first device; the members carry no
	// file system of their own.
	member := func(m Disk) Drive {
		x := s.drive(m)
		x.FsSize, x.Used, x.History, x.FullIn = 0, 0, []Sample{}, nil
		return x
	}
	var members []Drive
	for _, m := range st.Disks {
		if m.Pool == d.Name {
			members = append(members, member(m))
			asleep = asleep || m.Spundown
		}
	}
	if members != nil {
		out.Members = append([]Drive{member(d)}, members...)
	}
	if d.FsType != "zfs" {
		return out
	}
	if asleep {
		out.Zfs = s.history.Track(d.Name).Zfs
		return out
	}
	out.Zfs = s.zfsFacts(ctx, d.Name)
	return out
}

func (s *Service) zfsFacts(ctx context.Context, pool string) *PoolFacts {
	s.mu.Lock()
	if f, ok := s.facts[pool]; ok && time.Since(f.at) < factsFor {
		s.mu.Unlock()
		return f.facts
	}
	s.mu.Unlock()
	var f PoolFacts
	if b, err := s.run(ctx, "zpool", "get", "-Hp", "-o", "value", "fragmentation", pool); err == nil {
		if n, err := strconv.Atoi(strings.TrimSuffix(strings.TrimSpace(string(b)), "%")); err == nil {
			f.Fragmentation = &n
		}
	}
	if b, err := s.run(ctx, "zfs", "get", "-Hp", "-o", "value", "compressratio", pool); err == nil {
		if r, err := strconv.ParseFloat(strings.TrimSuffix(strings.TrimSpace(string(b)), "x"), 64); err == nil {
			f.CompressRatio = r
		}
	}
	if b, err := s.run(ctx, "zpool", "status", pool); err == nil {
		if r, ok := parseScrub(string(b)); ok {
			f.Scrub = &r
		}
	}
	if f.Fragmentation == nil && f.CompressRatio == 0 && f.Scrub == nil {
		return s.history.Track(pool).Zfs
	}
	s.mu.Lock()
	s.facts[pool] = cachedFacts{facts: &f, at: time.Now()}
	s.mu.Unlock()
	s.history.keepFacts(pool, f)
	return &f
}

type cachedFacts struct {
	facts *PoolFacts
	at    time.Time
}

var scrubLine = regexp.MustCompile(`scan: scrub repaired \S+ in (?:(\d+) days? )?(\d+):(\d+):(\d+) with (\d+) errors on (.+)$`)

// parseScrub reads the last finished scrub from zpool status.
func parseScrub(status string) (Run, bool) {
	for line := range strings.Lines(status) {
		m := scrubLine.FindStringSubmatch(strings.TrimSpace(line))
		if m == nil {
			continue
		}
		at, err := time.ParseInLocation("Mon Jan _2 15:04:05 2006", m[6], time.Local)
		if err != nil {
			return Run{}, false
		}
		n := func(i int) int64 { v, _ := strconv.ParseInt(m[i], 10, 64); return v }
		return Run{At: at, Seconds: n(1)*86400 + n(2)*3600 + n(3)*60 + n(4), Errors: n(5)}, true
	}
	return Run{}, false
}

// lastParity takes the newest of the last line of parity-checks.log and
// what var.ini says about the last run.
func (s *Service) lastParity(st *State) *ParityCheck {
	var best *ParityCheck
	if st.Parity.LastSync != nil {
		p := ParityCheck{Run: Run{At: *st.Parity.LastSync, Errors: st.Parity.LastErrors}, Exit: st.Parity.LastExit}
		if st.Parity.LastSyncEnd != nil {
			p.Seconds = int64(st.Parity.LastSyncEnd.Sub(p.At).Seconds())
			p.At = *st.Parity.LastSyncEnd
		}
		best = &p
	}
	raw, err := os.ReadFile(filepath.Join(s.cfg.BootConfig, "parity-checks.log"))
	if err != nil {
		return best
	}
	var last *ParityCheck
	sc := bufio.NewScanner(bytes.NewReader(raw))
	for sc.Scan() {
		if p, ok := parseParityLine(sc.Text()); ok {
			last = &p
		}
	}
	if last != nil && (best == nil || !last.At.Before(best.At.Add(-time.Minute))) {
		return last
	}
	return best
}

// parseParityLine reads one line of parity-checks.log. Unraid writes the
// end of the run, its seconds, the speed, the exit code and the errors,
// separated by bars; older versions put the year after the time.
func parseParityLine(line string) (ParityCheck, bool) {
	f := strings.Split(line, "|")
	if len(f) < 5 {
		return ParityCheck{}, false
	}
	date := strings.Join(strings.Fields(f[0]), " ")
	var at time.Time
	var err error
	for _, layout := range []string{"2006 Jan 2 15:04:05", "Jan 2 15:04:05 2006"} {
		if at, err = time.ParseInLocation(layout, date, time.Local); err == nil {
			break
		}
	}
	if err != nil {
		return ParityCheck{}, false
	}
	n := func(i int) int64 { v, _ := strconv.ParseInt(strings.TrimSpace(f[i]), 10, 64); return v }
	return ParityCheck{Run: Run{At: at, Seconds: n(1), Errors: n(4)}, Exit: n(3)}, true
}
