package system

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"path"
	"path/filepath"
	"slices"
	"strings"
	"sync"
	"time"

	v1 "github.com/junkerderprovinz/cannoncommander/backend/internal/api/v1"
	"github.com/junkerderprovinz/cannoncommander/backend/internal/ccfs"
	"github.com/junkerderprovinz/cannoncommander/backend/internal/version"
	"github.com/junkerderprovinz/cannoncommander/backend/pkg/settings"
)

// Section is one part of the settings file that a package owns. Neither
// side ever carries passwords, tokens, passkeys or the credentials of a
// remote.
type Section struct {
	Name string
	// Order sets when an import takes the section, the lowest first. The
	// sources go first, since their folders are the likeliest to be
	// missing on another server.
	Order int
	// People marks users, groups and rules, which go along only when asked.
	People bool
	Export func(ctx context.Context, d v1.Deps) (any, error)
	// Import takes the section from a file. Notes are things it left out,
	// such as a remote that could not be added.
	Import func(ctx context.Context, d v1.Deps, raw json.RawMessage) (notes []string, err error)
}

var sections struct {
	mu   sync.Mutex
	list []Section
}

// RegisterSection adds a part to the settings file. Packages call it from
// their Routes.
func RegisterSection(s Section) {
	sections.mu.Lock()
	defer sections.mu.Unlock()
	for i, o := range sections.list {
		if o.Name == s.Name {
			sections.list[i] = s
			return
		}
	}
	sections.list = append(sections.list, s)
	slices.SortStableFunc(sections.list, func(a, b Section) int { return a.Order - b.Order })
}

func registered(people bool) []Section {
	sections.mu.Lock()
	defer sections.mu.Unlock()
	var out []Section
	for _, s := range sections.list {
		if !s.People || people {
			out = append(out, s)
		}
	}
	return out
}

// exportFile is the settings file. App tells it apart from other JSON.
type exportFile struct {
	App      string                     `json:"app"`
	Version  string                     `json:"version"`
	Exported time.Time                  `json:"exported"`
	Sections map[string]json.RawMessage `json:"sections"`
}

const exportApp = "cannoncommander"

func (s *service) ownSections() {
	RegisterSection(Section{
		Name: "sources",
		Export: func(context.Context, v1.Deps) (any, error) {
			src, err := s.sources()
			return src.Sources, err
		},
		Import: func(_ context.Context, _ v1.Deps, raw json.RawMessage) ([]string, error) {
			var list []settings.StoredSource
			if err := json.Unmarshal(raw, &list); err != nil {
				return nil, err
			}
			_, err := s.saveSources(list)
			return nil, err
		},
	})
	RegisterSection(Section{
		Name:  "app",
		Order: 10,
		Export: func(ctx context.Context, _ v1.Deps) (any, error) {
			a, err := s.store.get(ctx)
			return a.public(), err
		},
		Import: func(ctx context.Context, _ v1.Deps, raw json.RawMessage) ([]string, error) {
			a, err := s.store.update(ctx, func(a *App) error {
				return json.Unmarshal(raw, a)
			})
			if err != nil {
				return nil, err
			}
			s.apply(a)
			return nil, nil
		},
	})
}

func (s *service) export(ctx context.Context, people bool) (exportFile, error) {
	f := exportFile{App: exportApp, Version: version.Version, Exported: time.Now().UTC().Truncate(time.Second), Sections: map[string]json.RawMessage{}}
	for _, sec := range registered(people) {
		v, err := sec.Export(ctx, s.deps)
		if err != nil {
			return exportFile{}, fmt.Errorf("%s: %w", sec.Name, err)
		}
		raw, err := json.Marshal(v)
		if err != nil {
			return exportFile{}, err
		}
		f.Sections[sec.Name] = raw
	}
	return f, nil
}

// ImportResult says what an import took over and what it left out.
type ImportResult struct {
	Sections      []string `json:"sections"`
	Notes         []string `json:"notes"`
	RestartNeeded bool     `json:"restartNeeded"`
}

func (s *service) importFile(ctx context.Context, f exportFile, people bool) (ImportResult, error) {
	if f.App != exportApp {
		return ImportResult{}, v1.Errorf(http.StatusBadRequest, "this is not a settings file of CannonCommander")
	}
	res := ImportResult{Sections: []string{}, Notes: []string{}}
	for _, sec := range registered(people) {
		raw, ok := f.Sections[sec.Name]
		if !ok || sec.Import == nil {
			continue
		}
		notes, err := sec.Import(ctx, s.deps, raw)
		if err != nil {
			var p *v1.Problem
			if errors.As(err, &p) {
				return res, v1.Errorf(p.Status, sec.Name+": "+p.Detail)
			}
			return res, v1.Errorf(http.StatusBadRequest, sec.Name+": "+err.Error())
		}
		res.Sections = append(res.Sections, sec.Name)
		res.Notes = append(res.Notes, notes...)
	}
	src, err := s.sources()
	if err != nil {
		return res, err
	}
	a, err := s.store.get(ctx)
	if err != nil {
		return res, err
	}
	res.RestartNeeded = src.RestartNeeded || restartNeeded(a)
	return res, nil
}

// fileRequest names a file on the server for export and import.
type fileRequest struct {
	Path  string `json:"path"`
	Users bool   `json:"users"`
}

func (s *service) getExport(w http.ResponseWriter, r *http.Request) error {
	f, err := s.export(r.Context(), r.URL.Query().Get("users") == "true")
	if err != nil {
		return err
	}
	name := "cannoncommander-settings-" + time.Now().Format("2006-01-02") + ".json"
	w.Header().Set("Content-Disposition", `attachment; filename="`+name+`"`)
	return v1.WriteJSON(w, r, http.StatusOK, f)
}

// postExport writes the settings file on the server, in a folder the admin
// picks, not in the downloads of the browser.
func (s *service) postExport(w http.ResponseWriter, r *http.Request) error {
	var in fileRequest
	if err := v1.DecodeJSON(r, &in); err != nil {
		return err
	}
	if !strings.HasSuffix(strings.ToLower(in.Path), ".json") {
		return v1.Errorf(http.StatusBadRequest, "the file name ends with .json")
	}
	dir, err := s.deps.FS.Check(v1.UserFrom(r), path.Dir(in.Path), ccfs.Create)
	if err != nil {
		return err
	}
	if dir.Virtual() || dir.InArchive() {
		return v1.Errorf(http.StatusBadRequest, "pick a folder below a source")
	}
	name := path.Base(in.Path)
	if !ccfs.ValidName(name) {
		return v1.Errorf(http.StatusBadRequest, "this is not a valid file name")
	}
	f, err := s.export(r.Context(), in.Users)
	if err != nil {
		return err
	}
	data, err := json.MarshalIndent(f, "", "  ")
	if err != nil {
		return err
	}
	target := path.Join(dir.Path, name)
	out, err := os.OpenFile(filepath.Join(dir.Real, name), os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o666)
	if errors.Is(err, os.ErrExist) {
		return v1.Errorf(http.StatusConflict, "this file is already there")
	}
	if err != nil {
		return err
	}
	if _, err := out.Write(append(data, '\n')); err != nil {
		out.Close()
		return err
	}
	if err := out.Close(); err != nil {
		return err
	}
	return v1.WriteJSON(w, r, http.StatusCreated, map[string]any{"path": target, "size": len(data) + 1})
}

func (s *service) postImport(w http.ResponseWriter, r *http.Request) error {
	var in fileRequest
	if err := v1.DecodeJSON(r, &in); err != nil {
		return err
	}
	loc, err := s.deps.FS.Check(v1.UserFrom(r), in.Path, ccfs.View)
	if err != nil {
		return err
	}
	if loc.Virtual() || loc.InArchive() {
		return v1.Errorf(http.StatusBadRequest, "this is not a settings file of CannonCommander")
	}
	file, err := os.Open(loc.Real)
	if err != nil {
		return err
	}
	defer file.Close()
	var f exportFile
	if err := json.NewDecoder(io.LimitReader(file, 16<<20)).Decode(&f); err != nil {
		return v1.Errorf(http.StatusBadRequest, "this is not a settings file of CannonCommander")
	}
	res, err := s.importFile(r.Context(), f, in.Users)
	if err != nil {
		return err
	}
	return v1.WriteJSON(w, r, http.StatusOK, res)
}

func (s *service) restart(w http.ResponseWriter, _ *http.Request) error {
	requestRestart()
	w.WriteHeader(http.StatusAccepted)
	return nil
}

func (s *service) update(w http.ResponseWriter, r *http.Request) error {
	out, err := s.cachedUpdate(r.Context())
	if err != nil {
		return v1.Errorf(http.StatusBadGateway, err.Error())
	}
	return v1.WriteJSON(w, r, http.StatusOK, out)
}
