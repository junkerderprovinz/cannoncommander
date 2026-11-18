package storeroom

import (
	"context"
	_ "embed"
	"errors"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"sync"

	v1 "github.com/junkerderprovinz/cannoncommander/backend/internal/api/v1"
	"github.com/junkerderprovinz/cannoncommander/backend/pkg/settings"
)

//go:embed openapi.yaml
var contract []byte

// ConfigFromEnv returns the paths of a stock Unraid server, the storeroom
// next to the database, with the CC_STOREROOM_* variables applied. Moving
// needs the same switch as every other write to Unraid.
func ConfigFromEnv() Config {
	c := Config{
		Templates: "/boot/config/plugins/dockerMan/templates-user",
		Plugins:   "/boot/config/plugins",
		Appdata:   "/mnt/user/appdata",
		Room:      "/config/storeroom",
	}
	if db := settings.Config.Server.DatabaseV2.Path; db != "" {
		c.Room = filepath.Join(filepath.Dir(db), "storeroom")
	}
	if _, err := os.Stat("/var/run/docker.sock"); err == nil {
		c.Docker = "/var/run/docker.sock"
	}
	for env, dst := range map[string]*string{
		"CC_STOREROOM_TEMPLATES": &c.Templates,
		"CC_STOREROOM_PLUGINS":   &c.Plugins,
		"CC_STOREROOM_APPDATA":   &c.Appdata,
		"CC_STOREROOM_ROOM":      &c.Room,
		"CC_STOREROOM_DOCKER":    &c.Docker,
	} {
		if v := os.Getenv(env); v != "" {
			*dst = v
		}
	}
	c.Write, _ = strconv.ParseBool(os.Getenv("CC_UNRAID_SOCKET"))
	return c
}

var (
	defaultOnce sync.Once
	defaultSvc  *Service
)

// Default returns the service for the environment.
func Default() *Service {
	defaultOnce.Do(func() { defaultSvc = New(ConfigFromEnv()) })
	return defaultSvc
}

// Routes registers the contract and the handlers.
func Routes(r *v1.Router) {
	r.Contract("storeroom", contract)
	bind(r, Default())
}

func bind(r *v1.Router, svc *Service) {
	r.Handle("GET /storeroom/scan", func(w http.ResponseWriter, req *http.Request) error {
		s, err := svc.Scan(req.Context())
		if err != nil {
			return err
		}
		return v1.WriteJSON(w, req, http.StatusOK, s)
	})
	r.Handle("POST /storeroom/plan", func(w http.ResponseWriter, req *http.Request) error {
		return steps(w, req, svc.Plan)
	})
	r.Handle("POST /storeroom/run", func(w http.ResponseWriter, req *http.Request) error {
		return steps(w, req, svc.Run)
	})
}

func steps(w http.ResponseWriter, r *http.Request, do func(context.Context, string, []Ref) ([]Step, error)) error {
	var req struct {
		Op   string `json:"op"`
		Refs []Ref  `json:"refs"`
	}
	if err := v1.DecodeJSON(r, &req); err != nil {
		return err
	}
	out, err := do(r.Context(), req.Op, req.Refs)
	if err != nil {
		return problem(err)
	}
	return v1.WriteJSON(w, r, http.StatusOK, map[string][]Step{"steps": out})
}

func problem(err error) error {
	switch {
	case errors.Is(err, ErrWriteOff):
		return v1.Coded(http.StatusForbidden, "unraidReadOnly", "write access to Unraid is switched off")
	case errors.Is(err, ErrBadOp):
		return v1.Errorf(http.StatusBadRequest, err.Error())
	case errors.Is(err, errNoDocker):
		return v1.Coded(http.StatusServiceUnavailable, "dockerUnavailable", "docker does not answer")
	}
	return err
}
