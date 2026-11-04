package remotes

import (
	"context"
	"encoding/json"

	v1 "github.com/junkerderprovinz/cannoncommander/backend/internal/api/v1"
	"github.com/junkerderprovinz/cannoncommander/backend/internal/system"
)

// exported is a remote in the settings file, without its secrets.
type exported struct {
	Name   string            `json:"name"`
	Type   string            `json:"type"`
	Path   string            `json:"path"`
	Mount  string            `json:"mount"`
	Params map[string]string `json:"params"`
}

func (s *Service) exportSection() system.Section {
	return system.Section{
		Name:  "remotes",
		Order: 20,
		Export: func(ctx context.Context, _ v1.Deps) (any, error) {
			list, err := s.List(ctx, true)
			if err != nil {
				return nil, err
			}
			out := []exported{}
			for _, r := range list.Remotes {
				out = append(out, exported{Name: r.Name, Type: r.Type, Path: r.Path, Mount: r.Mount, Params: r.Params})
			}
			return out, nil
		},
		Import: s.importRemotes,
	}
}

// importRemotes adds the remotes this server does not have yet. They come
// without credentials, so they stay unmounted until an admin enters them.
func (s *Service) importRemotes(ctx context.Context, _ v1.Deps, raw json.RawMessage) ([]string, error) {
	var list []exported
	if err := json.Unmarshal(raw, &list); err != nil {
		return nil, err
	}
	var notes []string
	for _, e := range list {
		_, there, err := s.db.get(ctx, e.Name)
		if err != nil {
			return notes, err
		}
		if there {
			notes = append(notes, "remote "+e.Name+" is already there and stays as it is")
			continue
		}
		in := Input{Name: e.Name, Type: e.Type, Path: &e.Path, Mount: e.Mount, Params: e.Params}
		if _, err := s.create(ctx, in, false); err != nil {
			notes = append(notes, "remote "+e.Name+" was left out: "+problem(err).Error())
			continue
		}
		notes = append(notes, "remote "+e.Name+" is added without its credentials, enter them before you mount it")
	}
	return notes, nil
}
