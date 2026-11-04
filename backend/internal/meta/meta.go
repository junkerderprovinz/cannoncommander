// Package meta reads what is inside files for the list's columns and the
// field search: photo details, film and music streams, tags, pages,
// archives, disk images, programs and checksums. What it reads is kept in
// meta_cache until the file changes.
package meta

import (
	"context"
	"database/sql"
	"errors"
	"path"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/gtsteffaniak/go-logger/logger"

	"github.com/junkerderprovinz/cannoncommander/backend/internal/meta/places"
	"github.com/junkerderprovinz/cannoncommander/backend/internal/unraid"
	"github.com/junkerderprovinz/cannoncommander/backend/pkg/indexing"
)

// group is one reader's share of the fields. The cache remembers which
// groups it has read for a file, so a column asked for later only costs
// its own reader.
type group uint32

const (
	gImage group = 1 << iota
	gAV
	gTags
	gPaged
	gOffice
	gText
	gArchive
	gDisk
	gProgram
	gSums
)

// SumsLimit is the largest file whose checksums fs/meta works out while the
// list waits. Bigger files are left to the checksum tool, which runs as a
// job.
const SumsLimit = 1 << 30

// errNoTool means a reader needs a program that is not installed. The
// group stays unread, so it is tried again once the program is there.
var errNoTool = errors.New("program not installed")

type reader struct {
	g      group
	exts   map[string]bool
	fields []string
	read   func(ctx context.Context, s *Service, f File, rec Record) error
}

// readers in the order their values win: tags before the stream reader,
// so a song's title comes from its tags rather than from the container.
var readers = []reader{
	{gImage, imageExts, []string{"width", "height", "camera", "make", "lens", "focal", "aperture", "exposure", "iso", "flash", "taken", "lat", "lon", "orientation", "depth", "dpi"}, readImage},
	{gTags, taggedExts, []string{"artist", "album", "title", "albumartist", "track", "year", "genre", "composer"}, readTags},
	{gAV, union(videoExts, audioExts), []string{"width", "height", "duration", "vcodec", "acodec", "framerate", "vbitrate", "aspect", "hdr", "audiotracks", "subtitles", "abitrate", "samplerate", "channels", "title", "year", "artist", "album"}, readAV},
	{gPaged, pagedExts, []string{"pages", "author", "doctitle", "subject", "app", "encrypted"}, readPaged},
	{gOffice, officeExts, []string{"pages", "words", "author", "lastauthor", "doctitle", "subject", "app", "encrypted"}, readOffice},
	{gText, textExts, []string{"lines", "words", "encoding"}, readText},
	{gArchive, archiveExts, []string{"unpacked", "entries", "method", "archcrypt"}, readArchive},
	{gDisk, diskExts, []string{"fs", "volume", "bootable"}, readDisk},
	{gProgram, programExts, []string{"version", "vendor", "arch", "signed"}, readProgram},
	{gSums, nil, []string{"sha256", "crc32", "md5", "sha1"}, readSums},
}

func union(a, b map[string]bool) map[string]bool {
	out := make(map[string]bool, len(a)+len(b))
	for k := range a {
		out[k] = true
	}
	for k := range b {
		out[k] = true
	}
	return out
}

// File is one file to describe. Path is the API path, which keys the cache.
type File struct {
	Path  string
	Real  string
	Size  int64
	MTime time.Time
	Dir   bool
}

// Service reads and caches metadata. It is safe for concurrent use.
type Service struct {
	cache   cache
	tools   *tools
	places  func() (*places.Table, error)
	devices func(apiPath string) []string
	// slots bounds the files read at the same time, since most readers
	// start a program or seek on a disk that may have to spin up.
	slots chan struct{}
}

// New returns a service that caches in the database db returns, which may
// be nil while there is none.
func New(db func() *sql.DB) *Service {
	return &Service{
		cache:   cache{db: db},
		tools:   &tools{},
		places:  places.Default,
		devices: func(p string) []string { return unraid.Default().DevicesOf(p) },
		slots:   make(chan struct{}, 4),
	}
}

var defaultService = sync.OnceValue(func() *Service {
	return New(func() *sql.DB {
		if db := indexing.GetIndexDB(); db != nil {
			return db.DB()
		}
		return nil
	})
})

// Default returns the service that caches in the index database.
func Default() *Service { return defaultService() }

// Values are the column values of one file. Columns without a value are
// left out.
type Values map[string]any

// Columns fills cols for files and returns the values by API path. A file
// whose reading was cut short by ctx keeps what was read before.
func (s *Service) Columns(ctx context.Context, files []File, cols []string) map[string]Values {
	out, _ := s.Lookup(ctx, files, cols)
	return out
}

// Lookup is Columns that also tells which files still lack a reading for
// some of the columns, because ctx ended first. With a ctx that has ended
// already it answers from the cache alone.
func (s *Service) Lookup(ctx context.Context, files []File, cols []string) (map[string]Values, map[string]bool) {
	var want []string
	for _, c := range cols {
		want = append(want, needs(c)...)
	}
	paths := make([]string, 0, len(files))
	for _, f := range files {
		if !f.Dir {
			paths = append(paths, f.Path)
		}
	}
	cached, err := s.cache.load(ctx, paths)
	if err != nil {
		logger.Debugf("meta cache: %v", err)
	}
	out := make(map[string]Values, len(files))
	unread := map[string]bool{}
	var mu sync.Mutex
	var wg sync.WaitGroup
	for _, f := range files {
		e, missing := s.plan(f, cached[f.Path], want)
		if missing == 0 {
			out[f.Path] = s.output(f, e.rec, cols)
			continue
		}
		slot := false
		if ctx.Err() == nil {
			select {
			case s.slots <- struct{}{}:
				slot = true
			case <-ctx.Done():
			}
		}
		if !slot {
			out[f.Path] = s.output(f, e.rec, cols)
			unread[f.Path] = true
			continue
		}
		wg.Go(func() {
			defer func() { <-s.slots }()
			e = s.read(ctx, f, e, missing)
			v := s.output(f, e.rec, cols)
			mu.Lock()
			out[f.Path] = v
			if e.done&missing != missing {
				unread[f.Path] = true
			}
			mu.Unlock()
		})
	}
	wg.Wait()
	return out, unread
}

// plan returns the cache entry that still holds for f and the groups that
// have to be read for the wanted fields.
func (s *Service) plan(f File, e entry, want []string) (entry, group) {
	if e.size != f.Size || e.mtime != f.MTime.Unix() {
		e = entry{size: f.Size, mtime: f.MTime.Unix(), rec: Record{}}
	}
	if f.Dir {
		return e, 0
	}
	return e, groupsFor(f, want) &^ e.done
}

func groupsFor(f File, want []string) group {
	ext := extOf(path.Base(f.Path))
	var g group
	for _, r := range readers {
		if r.g == gSums {
			if f.Size > SumsLimit {
				continue
			}
		} else if !r.exts[ext] {
			continue
		}
		for _, w := range want {
			if containsField(r.fields, w) {
				g |= r.g
				break
			}
		}
	}
	return g
}

func containsField(list []string, f string) bool {
	for _, x := range list {
		if x == f {
			return true
		}
	}
	return false
}

// read runs the readers of the missing groups and stores what they found.
func (s *Service) read(ctx context.Context, f File, e entry, missing group) entry {
	got := Record{}
	for _, r := range readers {
		if missing&r.g == 0 {
			continue
		}
		rec := Record{}
		err := r.read(ctx, s, f, rec)
		if ctx.Err() != nil {
			break
		}
		if errors.Is(err, errNoTool) {
			continue
		}
		if err != nil {
			logger.Debugf("meta: %s: %v", f.Path, err)
		}
		e.done |= r.g
		for k, v := range rec {
			if _, taken := got[k]; !taken && !empty(v) {
				got[k] = v
			}
		}
	}
	for k, v := range got {
		e.rec[k] = v
	}
	if err := s.cache.save(context.WithoutCancel(ctx), f.Path, e); err != nil {
		logger.Debugf("meta cache: %v", err)
	}
	return e
}

func empty(v any) bool {
	switch x := v.(type) {
	case nil:
		return true
	case string:
		return x == ""
	}
	return false
}

// fileTimes are the times of a file that change when it is only read or
// when its owner or rights change.
type fileTimes struct {
	accessed, changed time.Time
}

var episodeName = regexp.MustCompile(`(?i)s(\d{1,2})e(\d{1,3})|\b(\d{1,2})x(\d{2,3})\b`)

// episodeOf reads season and episode the way release names write them:
// S01E02 or 1x02.
func episodeOf(name string) (season, episode int64, ok bool) {
	if !videoExts[extOf(name)] {
		return 0, 0, false
	}
	m := episodeName.FindStringSubmatch(name)
	if m == nil {
		return 0, 0, false
	}
	s, e := m[1], m[2]
	if s == "" {
		s, e = m[3], m[4]
	}
	sn, _ := strconv.ParseInt(s, 10, 64)
	en, _ := strconv.ParseInt(e, 10, 64)
	return sn, en, true
}

// output turns a record into the values of the asked columns.
func (s *Service) output(f File, rec Record, cols []string) Values {
	out := Values{}
	var times *fileTimes
	for _, c := range cols {
		var v any
		switch c {
		case "dimensions":
			v = dimensions(rec)
		case "ratio":
			v = ratio(rec, f.Size)
		case "place":
			v = s.placeOf(rec)
		case "season", "episode":
			if sn, en, ok := episodeOf(path.Base(f.Path)); ok {
				v = sn
				if c == "episode" {
					v = en
				}
			}
		case "accessed", "statchange":
			if times == nil {
				t := timesOf(f.Real)
				times = &t
			}
			at := times.accessed
			if c == "statchange" {
				at = times.changed
			}
			if !at.IsZero() {
				v = at.UnixMilli()
			}
		case "disk":
			if s.devices != nil {
				v = strings.Join(s.devices(f.Path), ", ")
			}
		default:
			v = rec[c]
		}
		if !empty(v) {
			out[c] = v
		}
	}
	return out
}

func (s *Service) placeOf(rec Record) string {
	lat, ok1 := rec["lat"].(float64)
	lon, ok2 := rec["lon"].(float64)
	if !ok1 || !ok2 || s.places == nil {
		return ""
	}
	t, err := s.places()
	if err != nil {
		logger.Errorf("places: %v", err)
		return ""
	}
	p, _, ok := t.Nearest(lat, lon)
	if !ok {
		return ""
	}
	return p.Name
}

// Forget drops the cached entries below dir whose file is gone.
func (s *Service) Forget(ctx context.Context, dir string, exists func(apiPath string) bool) (int, error) {
	return s.cache.forget(ctx, dir, exists)
}

// Count returns how many files below dir have cached metadata.
func (s *Service) Count(ctx context.Context, dir string) (int64, error) {
	return s.cache.count(ctx, dir)
}
