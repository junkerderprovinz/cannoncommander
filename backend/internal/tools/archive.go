package tools

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"os"
	"path"
	"path/filepath"
	"slices"
	"strings"

	v1 "github.com/junkerderprovinz/cannoncommander/backend/internal/api/v1"
	"github.com/junkerderprovinz/cannoncommander/backend/internal/archives"
	"github.com/junkerderprovinz/cannoncommander/backend/internal/ccfs"
	"github.com/junkerderprovinz/cannoncommander/backend/internal/database/users"
	"github.com/junkerderprovinz/cannoncommander/backend/internal/fschange"
	"github.com/junkerderprovinz/cannoncommander/backend/internal/jobs"
)

// Job kinds for archives.
const (
	KindPack        = "pack"
	KindUnpack      = "unpack"
	KindTestArchive = "testArchive"
)

func init() {
	jobs.RegisterKind(KindPack, jobs.Kind{Prepare: preparePack, Run: runPack, Heavy: true})
	jobs.RegisterKind(KindUnpack, jobs.Kind{Prepare: prepareUnpack, Run: runUnpack, Heavy: true})
	jobs.RegisterKind(KindTestArchive, jobs.Kind{Prepare: prepareTestArchive, Run: runTestArchive})
}

// PackParams are the params of pack. The sources lie in one folder.
type PackParams struct {
	Sources []string `json:"sources"`
	Dest    string   `json:"dest"`
	// Name is the archive's name without its ending.
	Name     string `json:"name"`
	Format   string `json:"format"`
	Level    *int   `json:"level,omitempty"`
	Password string `json:"password,omitempty"`
}

type packStored struct {
	Base   string   `json:"base"`
	Names  []string `json:"names"`
	Dest   string   `json:"dest"`
	File   string   `json:"file"`
	Format string   `json:"format"`
	Level  int      `json:"level"`
	Secret string   `json:"secret,omitempty"`
}

func preparePack(_ context.Context, d v1.Deps, u *users.User, raw json.RawMessage) (*jobs.Plan, error) {
	var p PackParams
	if err := jobs.Decode(raw, &p); err != nil {
		return nil, err
	}
	f := archives.Format(p.Format)
	if !slices.Contains(archives.PackFormats, f) {
		return nil, badRequest("format must be zip, tar.gz, tar.zst, 7z or rar")
	}
	if !archives.CanPack(f) {
		return nil, v1.Errorf(http.StatusUnprocessableEntity, "this server has no program to write "+p.Format)
	}
	if p.Password != "" && (f == archives.TarGz || f == archives.TarZst) {
		return nil, badRequest("tar archives cannot have a password")
	}
	level := 6
	if p.Level != nil {
		level = *p.Level
	}
	if level < 0 || level > 9 {
		return nil, badRequest("level must be between 0 and 9")
	}
	file := p.Name + "." + p.Format
	if !ccfs.ValidName(file) || strings.TrimSpace(p.Name) == "" {
		return nil, badRequest("invalid name")
	}
	if len(p.Sources) == 0 {
		return nil, badRequest("sources are required")
	}
	if sameDir(p.Sources) == "" {
		return nil, badRequest("the sources have to lie in one folder")
	}
	dest, err := folder(d.FS, u, p.Dest, ccfs.Create)
	if err != nil {
		return nil, err
	}
	st := packStored{Dest: dest.Path, File: file, Format: p.Format, Level: level, Secret: keepSecret(p.Password)}
	plan := &jobs.Plan{Name: file, Dest: dest.Path}
	for i, s := range p.Sources {
		loc, _, err := entry(d.FS, u, s, ccfs.View|ccfs.Download)
		if err != nil {
			return nil, err
		}
		st.Base = filepath.Dir(loc.Real)
		st.Names = append(st.Names, filepath.Base(loc.Real))
		plan.Items = append(plan.Items, jobs.Item{Seq: i, Path: loc.Path, Name: path.Base(loc.Path)})
	}
	plan.Params = st
	return plan, nil
}

func runPack(ctx context.Context, run *jobs.Run) error {
	var st packStored
	if err := run.Params(&st); err != nil {
		return err
	}
	items := pending(run)
	if len(items) == 0 {
		return nil
	}
	if st.Secret != "" && secret(st.Secret) == "" {
		return errPasswordGone
	}
	var total int64
	for _, it := range items {
		bytes, files, err := jobs.TreeSize(ctx, filepath.Join(st.Base, it.Name))
		if err != nil {
			return err
		}
		run.Measure(it.Seq, bytes, files)
		total += bytes
		run.Begin(it.Seq)
	}
	dest, err := run.Deps().FS.Resolve(st.Dest)
	if err != nil {
		return err
	}
	name := ccfs.FreeName(dest.Real, st.File)
	part := filepath.Join(dest.Real, "."+name+partSuffix)
	_ = os.Remove(part)
	err = archives.Pack(ctx, part, st.Base, st.Names, archives.PackOptions{
		Format:   archives.Format(st.Format),
		Level:    st.Level,
		Password: secret(st.Secret),
		Total:    total,
		Wrote:    progress(ctx, run),
		Next:     func(n string) error { run.Current(n); return nil },
	})
	if err == nil {
		name = ccfs.FreeName(dest.Real, name)
		err = os.Rename(part, filepath.Join(dest.Real, name))
	}
	if err != nil {
		_ = os.Remove(part)
		if ctx.Err() != nil {
			return ctx.Err()
		}
		for _, it := range items {
			run.Finish(it.Seq, jobs.ItemFailed, err)
		}
		return err
	}
	forgetSecret(st.Secret)
	out := path.Join(st.Dest, name)
	fschange.Created(out)
	for _, it := range items {
		run.Finish(it.Seq, jobs.ItemDone, nil)
	}
	run.SetResult(map[string]string{"path": out})
	return nil
}

// UnpackParams are the params of unpack. An archive can be given with a
// folder inside it, as in /a.zip::/Strand, to unpack only that folder, and
// Items then picks names in it.
type UnpackParams struct {
	Archives []string `json:"archives"`
	Items    []string `json:"items,omitempty"`
	Dest     string   `json:"dest"`
	// Own unpacks every archive into a folder named after it.
	Own      bool   `json:"own,omitempty"`
	Clash    string `json:"clash,omitempty"`
	Password string `json:"password,omitempty"`
	Charset  string `json:"charset,omitempty"`
}

type unpackStored struct {
	Real    map[int]string `json:"real"`
	Inner   map[int]string `json:"inner"`
	Items   []string       `json:"items,omitempty"`
	Dest    string         `json:"dest"`
	Own     bool           `json:"own"`
	Clash   string         `json:"clash"`
	Secret  string         `json:"secret,omitempty"`
	Charset string         `json:"charset,omitempty"`
}

func prepareUnpack(_ context.Context, d v1.Deps, u *users.User, raw json.RawMessage) (*jobs.Plan, error) {
	var p UnpackParams
	if err := jobs.Decode(raw, &p); err != nil {
		return nil, err
	}
	if p.Clash == "" {
		p.Clash = archives.ClashBoth
	}
	if !slices.Contains([]string{archives.ClashSkip, archives.ClashBoth, archives.ClashReplace}, p.Clash) {
		return nil, badRequest("clash must be skip, both or replace")
	}
	if len(p.Archives) == 0 {
		return nil, badRequest("archives are required")
	}
	if len(p.Items) > 0 && len(p.Archives) != 1 {
		return nil, badRequest("items can only be picked from one archive")
	}
	perm := ccfs.Create
	if p.Clash == archives.ClashReplace {
		perm |= ccfs.Modify | ccfs.Delete
	}
	dest, err := folder(d.FS, u, p.Dest, perm)
	if err != nil {
		return nil, err
	}
	st := unpackStored{Real: map[int]string{}, Inner: map[int]string{}, Items: p.Items, Dest: dest.Path, Own: p.Own,
		Clash: p.Clash, Secret: keepSecret(p.Password), Charset: p.Charset}
	plan := &jobs.Plan{Dest: dest.Path}
	for i, a := range p.Archives {
		loc, err := d.FS.Check(u, a, ccfs.View|ccfs.Download)
		if err != nil {
			return nil, err
		}
		if loc.Virtual() {
			return nil, badRequest("not an archive")
		}
		if err := ccfs.Contained(loc); err != nil {
			return nil, err
		}
		if archives.FormatOf(loc.Real) == "" {
			return nil, badRequest(path.Base(loc.Real) + " is not an archive that can be unpacked")
		}
		if _, err := os.Stat(loc.Real); err != nil {
			return nil, err
		}
		st.Real[i] = loc.Real
		st.Inner[i] = strings.Trim(loc.Inner, "/")
		archivePath, _, _ := strings.Cut(loc.Path, ccfs.ArchiveSep)
		plan.Items = append(plan.Items, jobs.Item{Seq: i, Path: loc.Path, Name: path.Base(archivePath)})
	}
	plan.Params = st
	return plan, nil
}

func runUnpack(ctx context.Context, run *jobs.Run) error {
	var st unpackStored
	if err := run.Params(&st); err != nil {
		return err
	}
	if st.Secret != "" && secret(st.Secret) == "" {
		return errPasswordGone
	}
	opts := archives.Options{Password: secret(st.Secret), Charset: st.Charset}
	dest, err := run.Deps().FS.Resolve(st.Dest)
	if err != nil {
		return err
	}
	var landed []string
	skipped := 0
	for _, it := range pending(run) {
		real := st.Real[it.Seq]
		if ix, err := archives.Load(real, opts); err == nil {
			run.Measure(it.Seq, unpackedSize(ix, st.Inner[it.Seq]), int64(ix.Len()))
		}
		run.Begin(it.Seq)
		target, into := dest.Real, st.Dest
		if st.Own {
			folder := archives.Into(real)
			target, into = filepath.Join(dest.Real, folder), path.Join(st.Dest, folder)
			if err := os.MkdirAll(target, 0o777); err != nil {
				run.Finish(it.Seq, jobs.ItemFailed, err)
				continue
			}
			landed = append(landed, into)
		}
		res, err := unpackOne(ctx, run, real, target, opts, archives.ExtractOptions{Clash: st.Clash, Prefix: st.Inner[it.Seq], Only: st.Items})
		if ctx.Err() != nil {
			return ctx.Err()
		}
		for _, top := range res.Top {
			fschange.Created(path.Join(into, top))
			if !st.Own {
				landed = append(landed, path.Join(into, top))
			}
		}
		skipped += len(res.Skipped)
		run.Finish(it.Seq, jobs.ItemDone, err)
	}
	forgetSecret(st.Secret)
	run.SetResult(map[string]any{"paths": landed, "skipped": skipped})
	return nil
}

func unpackOne(ctx context.Context, run *jobs.Run, real, target string, o archives.Options, x archives.ExtractOptions) (archives.ExtractResult, error) {
	a, err := archives.Open(real, o)
	if err != nil {
		return archives.ExtractResult{}, err
	}
	defer a.Close()
	x.Wrote = progress(ctx, run)
	x.Next = func(name string) error { run.Current(name); return run.Wait(ctx) }
	return archives.Extract(a, target, x)
}

func unpackedSize(ix *archives.Index, prefix string) int64 {
	var total int64
	var walk func(dir string)
	walk = func(dir string) {
		kids, _ := ix.Children(dir)
		for _, e := range kids {
			total += e.Size
			if e.Dir {
				walk(e.Name)
			}
		}
	}
	walk(prefix)
	return total
}

// TestArchiveParams are the params of testArchive.
type TestArchiveParams struct {
	Paths    []string `json:"paths"`
	Password string   `json:"password,omitempty"`
}

type testStored struct {
	Real   map[int]string `json:"real"`
	Secret string         `json:"secret,omitempty"`
}

// TestResult is the outcome of testing one archive.
type TestResult struct {
	Path    string           `json:"path"`
	Members int              `json:"members"`
	OK      bool             `json:"ok"`
	Damage  *archives.Damage `json:"damage,omitempty"`
	// Password is set when the archive needs one that was not given.
	Password bool `json:"password,omitempty"`
}

func prepareTestArchive(_ context.Context, d v1.Deps, u *users.User, raw json.RawMessage) (*jobs.Plan, error) {
	var p TestArchiveParams
	if err := jobs.Decode(raw, &p); err != nil {
		return nil, err
	}
	if len(p.Paths) == 0 {
		return nil, badRequest("paths are required")
	}
	st := testStored{Real: map[int]string{}, Secret: keepSecret(p.Password)}
	plan := &jobs.Plan{}
	for i, a := range p.Paths {
		loc, _, err := file(d.FS, u, a, ccfs.View|ccfs.Download)
		if err != nil {
			return nil, err
		}
		if archives.FormatOf(loc.Real) == "" {
			return nil, badRequest(path.Base(loc.Path) + " is not an archive that can be tested")
		}
		st.Real[i] = loc.Real
		plan.Items = append(plan.Items, jobs.Item{Seq: i, Path: loc.Path, Name: path.Base(loc.Path)})
	}
	plan.Params = st
	return plan, nil
}

func runTestArchive(ctx context.Context, run *jobs.Run) error {
	var st testStored
	if err := run.Params(&st); err != nil {
		return err
	}
	if st.Secret != "" && secret(st.Secret) == "" {
		return errPasswordGone
	}
	var results []TestResult
	for _, it := range run.Items() {
		if it.State != jobs.ItemPending && it.State != jobs.ItemRunning {
			continue
		}
		if info, err := os.Stat(st.Real[it.Seq]); err == nil {
			run.Measure(it.Seq, info.Size(), 1)
		}
		run.Begin(it.Seq)
		run.Current(it.Name)
		n, err := archives.Test(st.Real[it.Seq], archives.Options{Password: secret(st.Secret)}, progress(ctx, run))
		if ctx.Err() != nil {
			return ctx.Err()
		}
		r := TestResult{Path: it.Path, Members: n, OK: err == nil}
		var dmg *archives.Damage
		switch {
		case errors.As(err, &dmg):
			r.Damage = dmg
		case errors.Is(err, archives.ErrPassword):
			r.Password = true
		}
		results = append(results, r)
		run.Finish(it.Seq, jobs.ItemDone, err)
	}
	forgetSecret(st.Secret)
	run.SetResult(map[string]any{"archives": results})
	return nil
}

// ArchiveInfo is the answer of GET /tools/archive.
type ArchiveInfo struct {
	Path      string `json:"path"`
	Format    string `json:"format"`
	Entries   int    `json:"entries"`
	Unpacked  int64  `json:"unpacked"`
	Encrypted bool   `json:"encrypted"`
}

// Formats is the answer of GET /tools/archives.
type Formats struct {
	Read []archives.Format `json:"read"`
	Pack []archives.Format `json:"pack"`
}

func (h handlers) formats(w http.ResponseWriter, r *http.Request) error {
	out := Formats{Read: []archives.Format{archives.Zip, archives.Tar, archives.TarGz, archives.TarZst, archives.TarXz, archives.TarBz2, archives.SevenZ, archives.Rar}}
	for _, f := range archives.PackFormats {
		if archives.CanPack(f) {
			out.Pack = append(out.Pack, f)
		}
	}
	return v1.WriteJSON(w, r, http.StatusOK, out)
}

func (h handlers) archiveInfo(w http.ResponseWriter, r *http.Request) error {
	loc, _, err := file(h.fs, v1.UserFrom(r), r.URL.Query().Get("path"), ccfs.View)
	if err != nil {
		return err
	}
	f := archives.FormatOf(loc.Real)
	if f == "" {
		return badRequest("not an archive")
	}
	ix, err := archives.Load(loc.Real, archives.Options{Password: r.Header.Get("X-Archive-Password")})
	if err != nil {
		return archives.Problem(err)
	}
	out := ArchiveInfo{Path: loc.Path, Format: string(f), Entries: ix.Len(), Unpacked: unpackedSize(ix, "")}
	var walk func(dir string)
	walk = func(dir string) {
		kids, _ := ix.Children(dir)
		for _, e := range kids {
			out.Encrypted = out.Encrypted || e.Encrypted
			if e.Dir {
				walk(e.Name)
			}
		}
	}
	walk("")
	return v1.WriteJSON(w, r, http.StatusOK, out)
}
