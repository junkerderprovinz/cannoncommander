// Package v1 is the namespace for /api/v1. Every feature package brings its
// own OpenAPI contract and binds handlers to its operations in Routes. An
// operation without a handler answers 501, so the whole contract is
// reachable from the first day and clients can be built against it.
package v1

import (
	"context"
	"database/sql"
	"fmt"
	"net/http"

	"github.com/junkerderprovinz/cannoncommander/backend/internal/ccfs"
	"github.com/junkerderprovinz/cannoncommander/backend/internal/database/users"
	"github.com/junkerderprovinz/cannoncommander/backend/internal/fsindex"
	"github.com/junkerderprovinz/cannoncommander/backend/internal/hub"
)

// Deps are the services handlers share.
type Deps struct {
	FS  *ccfs.Resolver
	Hub *hub.Hub
	DB  *sql.DB
	// Index may be nil, then listings go without folder sizes.
	Index fsindex.Index
	// Users finds a user as they are now, for work that runs outside a request.
	Users func(name string) (*users.User, error)
}

// HandlerFunc is a handler that reports failure by returning an error,
// which the router turns into a problem response.
type HandlerFunc func(w http.ResponseWriter, r *http.Request) error

// Authenticator finds the user behind a request and may refresh the
// session cookie on w. It returns a Problem with status 401 when nobody is
// logged in.
type Authenticator func(w http.ResponseWriter, r *http.Request) (*users.User, error)

// Router collects contracts and handlers.
type Router struct {
	deps  Deps
	auth  Authenticator
	frags []*fragment
	ops   []*Operation
	byKey map[string]*Operation
}

// NewRouter returns an empty router. auth may be nil in tests that only
// look at the contract.
func NewRouter(deps Deps, auth Authenticator) *Router {
	return &Router{deps: deps, auth: auth, byKey: map[string]*Operation{}}
}

// Deps returns the shared services.
func (r *Router) Deps() Deps { return r.deps }

// Contract adds a package's OpenAPI fragment. It panics on a broken
// fragment, because that is a bug that should stop the server from starting.
func (r *Router) Contract(name string, spec []byte) {
	f, ops, err := parseFragment(name, spec)
	if err != nil {
		panic(err)
	}
	for _, op := range ops {
		key := op.Pattern()
		if _, dup := r.byKey[key]; dup {
			panic(fmt.Sprintf("%s: %s is already declared", name, key))
		}
		for _, o := range r.ops {
			if o.ID == op.ID {
				panic(fmt.Sprintf("%s: operationId %s is already used", name, op.ID))
			}
		}
		r.byKey[key] = op
		r.ops = append(r.ops, op)
	}
	r.frags = append(r.frags, f)
}

// Handle binds h to an operation from a contract, given as a net/http
// pattern such as "GET /fs/list". It panics when the contract does not
// know the operation, so the contract and the code cannot drift apart.
func (r *Router) Handle(pattern string, h HandlerFunc) {
	op, ok := r.byKey[pattern]
	if !ok {
		panic(fmt.Sprintf("v1: %s is not in any contract", pattern))
	}
	if op.handler != nil {
		panic(fmt.Sprintf("v1: %s is bound twice", pattern))
	}
	op.handler = h
}

// Authenticator returns what finds the user behind a request, for a handler that
// runs for a long time and has to ask again.
func (r *Router) Authenticator() Authenticator { return r.auth }

// Operations returns every operation in contract order.
func (r *Router) Operations() []*Operation { return r.ops }

// OpenAPI returns the merged contract.
func (r *Router) OpenAPI() ([]byte, error) { return merge(r.frags) }

// Handler serves every operation. It expects paths without the /api/v1
// prefix.
func (r *Router) Handler() (http.Handler, error) {
	spec, err := r.OpenAPI()
	if err != nil {
		return nil, err
	}
	mux := http.NewServeMux()
	for _, op := range r.ops {
		mux.Handle(op.Pattern(), r.wrap(op))
	}
	mux.HandleFunc("GET /openapi.yaml", func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/yaml")
		_, _ = w.Write(spec)
	})
	mux.HandleFunc("/", func(w http.ResponseWriter, req *http.Request) {
		WriteError(w, req, Errorf(http.StatusNotFound, "no such endpoint"))
	})
	return mux, nil
}

func (r *Router) wrap(op *Operation) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		if op.Access != AccessPublic {
			if r.auth == nil {
				WriteError(w, req, Coded(http.StatusUnauthorized, "signInNeeded", "login required"))
				return
			}
			u, err := r.auth(w, req)
			if err != nil {
				WriteError(w, req, err)
				return
			}
			if op.Access == AccessAdmin && !u.Permissions.Admin {
				WriteError(w, req, Coded(http.StatusForbidden, "adminsOnly", "admins only"))
				return
			}
			req = req.WithContext(context.WithValue(req.Context(), userKey{}, u))
		}
		if op.handler == nil {
			WriteError(w, req, Errorf(http.StatusNotImplemented, op.ID+" is not built yet"))
			return
		}
		if err := op.handler(w, req); err != nil {
			WriteError(w, req, err)
		}
	})
}

type userKey struct{}

// UserFrom returns the logged in user. It is nil only for public operations.
func UserFrom(r *http.Request) *users.User {
	u, _ := r.Context().Value(userKey{}).(*users.User)
	return u
}
