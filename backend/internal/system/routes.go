// Package system reports the version and state of the server and its settings.
package system

import (
	"context"
	_ "embed"
	"net/http"
	"strings"

	v1 "github.com/junkerderprovinz/cannoncommander/backend/internal/api/v1"
	"github.com/junkerderprovinz/cannoncommander/backend/internal/search"
	"github.com/junkerderprovinz/cannoncommander/backend/pkg/settings"
)

//go:embed openapi.yaml
var contract []byte

// Routes registers the contract and the handlers. The admin/index
// operations belong to the search package, which owns the indexer.
func Routes(r *v1.Router) {
	r.Contract("system", contract)
	captureBoot()
	s := &service{
		deps:        r.Deps(),
		store:       storeFor(r.Deps()),
		apply:       apply,
		running:     func() []*settings.Source { return settings.Config.Server.Sources },
		sourcesFile: settings.SourcesFile,
		test:        newTester().run,
		releases:    releasesAPI,
	}
	s.ownSections()
	bind(r, s)
	search.IndexRoutes(r)
}

type service struct {
	deps        v1.Deps
	store       *store
	apply       func(App)
	running     func() []*settings.Source
	sourcesFile func() string
	test        func(ctx context.Context, kind, url, secret string) IntegrationResult
	releases    string
	tools       toolbox
	updates     updateCache
}

func bind(r *v1.Router, s *service) {
	r.Handle("GET /system", s.getSystem)
	r.Handle("POST /system/update", s.update)
	r.Handle("GET /admin/sources", s.getSources)
	r.Handle("PUT /admin/sources", s.putSources)
	r.Handle("POST /admin/integrations/test", s.testIntegration)
	r.Handle("GET /admin/app", s.getApp)
	r.Handle("PUT /admin/app", s.putApp)
	r.Handle("POST /admin/app/restart", s.restart)
	r.Handle("GET /admin/app/export", s.getExport)
	r.Handle("POST /admin/app/export", s.postExport)
	r.Handle("POST /admin/app/import", s.postImport)
}

func (s *service) getSystem(w http.ResponseWriter, r *http.Request) error {
	in, err := s.info(r.Context())
	if err != nil {
		return err
	}
	// The uptime moves every second, the rest hardly ever.
	still := in
	still.Uptime = 0
	return v1.WriteJSONTagged(w, r, http.StatusOK, in, still)
}

func (s *service) getSources(w http.ResponseWriter, r *http.Request) error {
	out, err := s.sources()
	if err != nil {
		return err
	}
	return v1.WriteJSON(w, r, http.StatusOK, out)
}

func (s *service) putSources(w http.ResponseWriter, r *http.Request) error {
	var in struct {
		Sources []settings.StoredSource `json:"sources"`
	}
	if err := v1.DecodeJSON(r, &in); err != nil {
		return err
	}
	out, err := s.saveSources(in.Sources)
	if err != nil {
		return err
	}
	return v1.WriteJSON(w, r, http.StatusOK, out)
}

// appDoc is the settings as the page reads and saves them.
type appDoc struct {
	App
	// RestartNeeded is only ever sent. A page that saves what it read
	// sends it back, which does no harm.
	RestartNeeded bool `json:"restartNeeded"`
}

func (s *service) getApp(w http.ResponseWriter, r *http.Request) error {
	a, err := s.store.get(r.Context())
	if err != nil {
		return err
	}
	return v1.WriteJSON(w, r, http.StatusOK, appDoc{App: a.public(), RestartNeeded: restartNeeded(a)})
}

// putApp saves the fields the body carries over the saved ones, so a page
// can send one switch at a time. A secret or token that is left out stays,
// an empty one is removed.
func (s *service) putApp(w http.ResponseWriter, r *http.Request) error {
	a, err := s.store.update(r.Context(), func(a *App) error {
		doc := appDoc{App: *a}
		if err := v1.DecodeJSON(r, &doc); err != nil {
			return err
		}
		*a = doc.App
		return nil
	})
	if err != nil {
		return err
	}
	s.apply(a)
	return v1.WriteJSON(w, r, http.StatusOK, appDoc{App: a.public(), RestartNeeded: restartNeeded(a)})
}

func (s *service) testIntegration(w http.ResponseWriter, r *http.Request) error {
	var in IntegrationTest
	if err := v1.DecodeJSON(r, &in); err != nil {
		return err
	}
	switch in.Kind {
	case KindOnlyOffice, KindEuroOffice, KindCollabora, KindArrowLoop:
	default:
		return v1.Errorf(http.StatusBadRequest, "kind must be onlyoffice, eurooffice, collabora or arrowloop")
	}
	base := strings.TrimRight(strings.TrimSpace(in.URL), "/")
	if base == "" {
		return v1.Errorf(http.StatusBadRequest, "enter the address first")
	}
	if err := checkURL(base); err != nil {
		return v1.Errorf(http.StatusBadRequest, err.Error())
	}
	saved, err := s.store.get(r.Context())
	if err != nil {
		return err
	}
	secret := ""
	switch {
	case in.Kind == KindArrowLoop && in.Token != nil:
		secret = *in.Token
	case in.Kind == KindArrowLoop && base == saved.ArrowLoop.URL:
		secret = saved.ArrowLoop.Token
	case in.Kind != KindArrowLoop && in.Secret != nil:
		secret = *in.Secret
	case in.Kind != KindArrowLoop && base == saved.Office.URL:
		secret = saved.Office.Secret
	}
	return v1.WriteJSON(w, r, http.StatusOK, s.test(r.Context(), in.Kind, base, secret))
}
