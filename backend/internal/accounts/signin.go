package accounts

import (
	"context"
	"database/sql"
	"fmt"
	"net"
	"net/url"
	"strconv"
	"strings"
	"sync/atomic"

	"github.com/gtsteffaniak/go-logger/logger"
	v1 "github.com/junkerderprovinz/cannoncommander/backend/internal/api/v1"
	"github.com/junkerderprovinz/cannoncommander/backend/internal/state"
	"github.com/junkerderprovinz/cannoncommander/backend/internal/utils"
	"github.com/junkerderprovinz/cannoncommander/backend/pkg/settings"
	"github.com/junkerderprovinz/parleyport/kit/signin"
	"github.com/junkerderprovinz/parleyport/kit/signin/sqlstore"
)

// roles are the app's roles for the sign-in, admin first. They mirror
// roleOf, so an account is no more in the sign-in than in the users table.
var roles = []string{roleAdmin, roleUser, roleReadOnly}

// signIns is the sign-in of the running server, nil before Open and under
// noauth.
var signIns atomic.Pointer[signin.Auth]

// SignIn returns the sign-in Open made, or nil.
func SignIn() *signin.Auth { return signIns.Load() }

// Open opens the sign-in on the main database of d. On its first start it
// imports the accounts the server had before, and from then on it keeps the
// users table in step with what it decides.
func Open(ctx context.Context, d v1.Deps) (*signin.Auth, error) {
	svc, err := serviceFor(d.DB)
	if err != nil {
		return nil, err
	}
	h := handlers{db: d.DB, fs: d.FS, deps: d, svc: svc}
	if err = renameOnce(ctx, h); err != nil {
		return nil, err
	}
	if err = checkNames(); err != nil {
		return nil, err
	}
	service, err := configService()
	if err != nil {
		return nil, err
	}
	if state.Reimport() {
		if err = h.forgetSignIn(ctx, service); err != nil {
			return nil, fmt.Errorf("import the accounts again: %w", err)
		}
	}
	cfg, err := h.config(ctx, service)
	if err != nil {
		return nil, err
	}
	auth, err := signin.Open(cfg)
	if err != nil {
		return nil, err
	}
	if err := state.ArmLegacyFingerprint(); err != nil {
		_ = auth.Close()
		return nil, err
	}
	warnAboutConfig(ctx, h)
	signIns.Store(auth)
	state.UseSignIn(auth.Anyone)
	return auth, nil
}

// Close stops the sign-in Open made.
func Close() {
	if auth := signIns.Swap(nil); auth != nil {
		_ = auth.Close()
	}
}

func (h handlers) config(ctx context.Context, service *signin.ServiceSpec) (signin.Config, error) {
	key, err := state.SealKey()
	if err != nil {
		return signin.Config{}, err
	}
	seal, err := signin.NewSealer(key)
	if err != nil {
		return signin.Config{}, err
	}
	store, err := sqlstore.New(ctx, h.db, docsTable)
	if err != nil {
		return signin.Config{}, err
	}
	cfg := signin.Config{
		Domain:      "cc",
		Name:        "CannonCommander",
		Store:       store,
		Seal:        seal,
		Roles:       roles,
		TokenPrefix: "cc",
		Public:      publicURL,
		Base:        settings.Config.Http.BaseURL,
		MinPassword: max(minPassword, settings.Config.Auth.Methods.PasswordAuth.MinLength),
		NewID:       newAccountID,
		Import:      h.importAccounts,
		Legacy:      h.legacy(),
		Hooks: signin.Hooks{
			Changing: h.changing,
			Admit:    admit,
			SignedIn: h.signedIn,
			Ended:    h.ended,
			Logged:   logged,
		},
	}
	if settings.Config.Http.TrustProxyHeaders {
		cfg.Proxies = settings.TrustedProxyPrefixes()
		cfg.TrustedPeer = settings.IsTrustedProxyPeer
	}
	if service != nil {
		cfg.Services = []signin.ServiceSpec{*service}
		cfg.Closed = func() bool { return true }
	}
	return cfg, nil
}

// publicURL is http.externalUrl as browsers write it: a lower-case host
// without the default port. The base path is the sign-in's Base.
func publicURL() *url.URL {
	u, err := url.Parse(strings.TrimSpace(settings.Config.Http.ExternalUrl))
	if err != nil || u.Host == "" || (u.Scheme != "https" && u.Scheme != "http") {
		return nil
	}
	host, port := strings.ToLower(u.Hostname()), u.Port()
	if (u.Scheme == "https" && port == "443") || (u.Scheme == "http" && port == "80") {
		port = ""
	}
	if port != "" {
		host = net.JoinHostPort(host, port)
	} else if strings.Contains(host, ":") {
		host = "[" + host + "]"
	}
	return &url.URL{Scheme: u.Scheme, Host: host}
}

func newAccountID() (string, error) {
	id, err := utils.RandomUint64ID()
	if err != nil {
		return "", err
	}
	return strconv.FormatUint(id, 10), nil
}

// forgetSignIn throws the sign-in's documents away, so the next Open imports
// the accounts as the older version left them. What is still waiting to be
// ended runs first, and every MCP key and grant goes, as they may have come
// from a way in that is gone.
func (h handlers) forgetSignIn(ctx context.Context, service *signin.ServiceSpec) error {
	cfg, err := h.config(ctx, service)
	if err != nil {
		return err
	}
	cfg.Import = nil
	auth, err := signin.Open(cfg)
	if err != nil {
		return err
	}
	_ = auth.Close()
	counts := map[string]int{}
	rows, err := h.db.QueryContext(ctx, `SELECT kind, COUNT(*) FROM `+docsTable+` GROUP BY kind`)
	if err != nil {
		return err
	}
	for rows.Next() {
		var kind string
		var n int
		if err = rows.Scan(&kind, &n); err != nil {
			rows.Close()
			return err
		}
		counts[kind] = n
	}
	rows.Close()
	if _, err = h.db.ExecContext(ctx, `DELETE FROM `+docsTable); err != nil {
		return err
	}
	keys, err := revokeAllKeys(ctx, h.db)
	if err != nil {
		return err
	}
	logger.Warningf("signin: imported the accounts again, dropping %v and %d MCP keys and grants", counts, keys)
	return nil
}

// revokeAllKeys is set by the MCP server, which keeps keys and grants in
// tables of its own.
var revokeAllKeys = func(context.Context, *sql.DB) (int, error) { return 0, nil }

// OnReimport sets what revokes every MCP key and grant when the accounts are
// imported again. It is meant for init functions.
func OnReimport(f func(ctx context.Context, db *sql.DB) (int, error)) { revokeAllKeys = f }
