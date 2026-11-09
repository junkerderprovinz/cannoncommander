package mcpapi

import (
	"context"
	"errors"
	"fmt"
	"net"
	"net/http"
	"net/url"
	"regexp"
	"strconv"
	"strings"
	"time"
	"unicode"
	"unicode/utf8"

	"github.com/gtsteffaniak/go-logger/logger"

	"github.com/junkerderprovinz/cannoncommander/backend/internal/activity"
	v1 "github.com/junkerderprovinz/cannoncommander/backend/internal/api/v1"
	activitydb "github.com/junkerderprovinz/cannoncommander/backend/internal/database/activity"
	"github.com/junkerderprovinz/cannoncommander/backend/internal/database/users"
	"github.com/junkerderprovinz/cannoncommander/backend/internal/unraid"
	"github.com/junkerderprovinz/cannoncommander/backend/pkg/desktop"
)

// KeyView is one key as the settings card sees it. The key itself is handed
// out once and is not in here; the hint tells two apart.
type KeyView struct {
	ID            string `json:"id"`
	Owner         string `json:"owner"`
	Own           bool   `json:"own"`
	ViaOAuth      bool   `json:"viaOAuth"`
	Label         string `json:"label"`
	Client        string `json:"client"`
	Hint          string `json:"hint"`
	CanWrite      bool   `json:"canWrite"`
	CreatedAt     int64  `json:"createdAt"`
	RotatedAt     int64  `json:"rotatedAt"`
	LastUsedAt    int64  `json:"lastUsedAt"`
	LastUsedFrom  string `json:"lastUsedFrom"`
	RevokedAt     int64  `json:"revokedAt"`
	RevokedReason string `json:"revokedReason"`
	Unusable      string `json:"unusable"`
	CallsToday    int    `json:"callsToday"`
}

var (
	errLabel        = v1.Coded(http.StatusBadRequest, "mcpKeyLabel", "a key needs a name of 1 to 64 characters")
	errClient       = v1.Coded(http.StatusBadRequest, "mcpKeyClient", "a client id is 1 to 32 lower-case letters, digits and dashes")
	errNeedsSignIn  = v1.Coded(http.StatusForbidden, "mcpKeyNeedsSignIn", "without sign-in, a key can only be made from an IP address or a local name")
	errNoSuchKey    = v1.Coded(http.StatusNotFound, "mcpKeyNotFound", "there is no such key")
	errNotYourKey   = v1.Coded(http.StatusForbidden, "mcpKeyNotYours", "only the account a key belongs to can do this")
	errTokenRefused = v1.Coded(http.StatusForbidden, "tokenCannotChange", "sign in to change keys, an API token cannot")
)

func keyProblem(err error) error {
	switch {
	case errors.Is(err, errKeyNotFound):
		return errNoSuchKey
	case errors.Is(err, errKeyLimit):
		return v1.Coded(http.StatusConflict, "mcpKeyLimit", fmt.Sprintf("an account has at most %d active keys; revoke one first", keyLimit))
	case errors.Is(err, errKeyLabelTaken):
		return v1.Coded(http.StatusConflict, "mcpKeyLabelTaken", "another active key has this name")
	case errors.Is(err, errKeyActive):
		return v1.Coded(http.StatusConflict, "mcpKeyActive", "revoke the key before removing it")
	}
	return err
}

// cleanLabel trims a name and reports whether it is usable: 1 to 64 runes,
// no control characters.
func cleanLabel(s string) (string, bool) {
	s = strings.TrimSpace(s)
	if n := utf8.RuneCountInString(s); n == 0 || n > 64 {
		return "", false
	}
	for _, r := range s {
		if unicode.IsControl(r) || unicode.Is(unicode.Cf, r) {
			return "", false
		}
	}
	return s, true
}

var clientID = regexp.MustCompile(`^[a-z0-9][a-z0-9-]{0,31}$`)

var keyIDShape = regexp.MustCompile(`^[0-9a-f]{32}$`)

// privateSuffixes only ever resolve inside a network, so no answer from
// public DNS can point one of them at a LAN address.
var privateSuffixes = []string{".local", ".lan", ".home", ".home.arpa", ".internal", ".localdomain", ".ts.net"}

// hostAllowsKeys reports whether a key may be made from the request host
// while nobody signs in: an IP address, localhost, a single label or a
// private suffix, none of which a public DNS name can rebind.
func hostAllowsKeys(host string) bool {
	if h, _, err := net.SplitHostPort(host); err == nil {
		host = h
	}
	host = strings.ToLower(strings.Trim(strings.TrimSpace(host), "[]"))
	if host == "" {
		return false
	}
	if net.ParseIP(host) != nil || !strings.Contains(host, ".") {
		return true
	}
	for _, suffix := range privateSuffixes {
		if strings.HasSuffix(host, suffix) {
			return true
		}
	}
	return false
}

// keysAllowedFrom reports whether r may make or change a key. A server people
// sign in to settles it with the session, which is bound to the real address;
// a page that rebound its name to this one carries none. Without sign-in, as in
// the desktop app, only an address that cannot be rebound counts.
func (s *Server) keysAllowedFrom(r *http.Request) bool {
	return s.signIn() || hostAllowsKeys(r.Host)
}

// proveOwner makes sure the person at the keyboard owns the account before a
// key is made, replaced or allowed to write: a session, no API token, and the
// password or a fresh sign-in, as for an API token.
func (s *Server) proveOwner(r *http.Request, u *users.User, password string) error {
	if s.login.ByAPIToken(r) {
		return errTokenRefused
	}
	if !s.keysAllowedFrom(r) {
		return errNeedsSignIn
	}
	if !s.signIn() {
		return nil
	}
	if _, ok := s.login.SessionHash(r); !ok {
		return errTokenRefused
	}
	return s.login.Confirm(r, u, password)
}

// unusable names why a key no longer authenticates although nobody revoked
// it. A new signing key makes every digest stop matching at once, and the
// check value tells that apart from a client sending the wrong key.
func (s *Server) unusable(k Key) string {
	if k.RevokedAt == 0 && !sameString(k.Check, keyCheck(s.pepper(), k.ID)) {
		return "signingKeyChanged"
	}
	return ""
}

func (s *Server) view(k Key, me *users.User, owners map[uint64]string, calls map[string]int) KeyView {
	return KeyView{
		ID: k.ID, Owner: owners[k.UserID], Own: k.UserID == me.ID, ViaOAuth: k.Kind == kindOAuth, Label: k.Label, Client: k.Client,
		Hint: k.Hint, CanWrite: k.CanWrite, CreatedAt: k.CreatedAt, RotatedAt: k.RotatedAt, LastUsedAt: k.LastUsedAt,
		LastUsedFrom: k.LastUsedFrom, RevokedAt: k.RevokedAt, RevokedReason: k.RevokedReason, Unusable: s.unusable(k), CallsToday: calls[k.ID],
	}
}

// callsSince is where a key's calls today start: at the midnight the card
// sends, which is the browser's, while the slots still cover it, and at the
// server's own otherwise.
func callsSince(r *http.Request, now time.Time) int64 {
	if since, err := strconv.ParseInt(r.URL.Query().Get("since"), 10, 64); err == nil && since <= now.Unix() && now.Unix()-since < callsCovered {
		return since
	}
	y, m, d := now.Date()
	return time.Date(y, m, d, 0, 0, 0, 0, now.Location()).Unix()
}

func (s *Server) ownerNames(ids ...uint64) map[uint64]string {
	out := map[uint64]string{}
	for _, id := range ids {
		if _, seen := out[id]; seen {
			continue
		}
		if u, err := s.user(id); err == nil {
			out[id] = u.Username
		} else {
			out[id] = ""
		}
	}
	return out
}

func (s *Server) list(w http.ResponseWriter, r *http.Request) error {
	me := v1.UserFrom(r)
	ctx := r.Context()
	all := r.URL.Query().Get("all") == "true"
	if all && !me.Permissions.Admin {
		return v1.Coded(http.StatusForbidden, "adminsOnly", "admins only")
	}
	now := s.now()
	// A grant whose refresh token ran out shows as expired rather than as
	// active until the next sign-in happens to prune it.
	if err := s.st.pruneOAuth(ctx, now.Unix()); err != nil {
		logger.Errorf("mcp: oauth: prune: %v", err)
	}
	userID := me.ID
	if all {
		userID = 0
	}
	rows, err := s.st.keys(ctx, userID)
	if err != nil {
		return err
	}
	calls, err := s.st.callsSince(ctx, callsSince(r, now))
	if err != nil {
		return err
	}
	ids := make([]uint64, 0, len(rows))
	for _, k := range rows {
		ids = append(ids, k.UserID)
	}
	owners := s.ownerNames(ids...)
	active, revoked := []KeyView{}, []KeyView{}
	for _, k := range rows {
		v := s.view(k, me, owners, calls)
		if k.RevokedAt != 0 {
			revoked = append(revoked, v)
			continue
		}
		active = append(active, v)
	}
	oauth, err := s.oauthView(ctx)
	if err != nil {
		return err
	}
	out := map[string]any{
		"endpointPath":   EndpointPath,
		"limit":          keyLimit,
		"signIn":         s.signIn(),
		"hostAllowsKeys": s.keysAllowedFrom(r),
		"desktop":        desktop.Active(),
		"writeLimits": map[string]int64{
			"entriesPerCall": itemsPerCall, "entriesPerHour": itemsPerHour, "filesPerCall": filesPerCall, "callsPerMinute": callsPerMinute,
			"bytesPerCall": bytesPerCall,
		},
		"keys":    active,
		"revoked": revoked,
		"oauth":   oauth,
	}
	if me.Permissions.Admin {
		requests, tools := s.counters()
		out["stats"] = map[string]any{"requests": requests, "tools": tools}
	}
	w.Header().Set("Cache-Control", "no-store")
	return v1.WriteJSON(w, r, http.StatusOK, out)
}

type createRequest struct {
	Label    string `json:"label"`
	Client   string `json:"client"`
	CanWrite bool   `json:"canWrite"`
	Password string `json:"password"`
}

func (s *Server) create(w http.ResponseWriter, r *http.Request) error {
	me := v1.UserFrom(r)
	var in createRequest
	if err := v1.DecodeJSON(r, &in); err != nil {
		return err
	}
	label, ok := cleanLabel(in.Label)
	if !ok {
		return errLabel
	}
	if in.Client != "" && !clientID.MatchString(in.Client) {
		return errClient
	}
	if err := s.proveOwner(r, me, in.Password); err != nil {
		return err
	}
	pepper := s.pepper()
	if len(pepper) == 0 {
		return errors.New("the server has no signing key")
	}
	key, id := newSecret(keyPrefix), newID()
	k, err := s.st.createKey(r.Context(), Key{
		ID: id, UserID: me.ID, Label: label, Client: in.Client,
		Digest: hashKey(pepper, key), Hint: keyHint(key), Check: keyCheck(pepper, id),
		CanWrite: in.CanWrite, CreatedAt: s.now().Unix(),
	})
	if err != nil {
		return keyProblem(err)
	}
	event := "made"
	if k.CanWrite {
		event = "made, allowed to write"
	}
	s.changed(r, k, event)
	w.Header().Set("Cache-Control", "no-store")
	return v1.WriteJSON(w, r, http.StatusCreated, map[string]any{"key": key, "item": s.view(k, me, s.ownerNames(me.ID), nil)})
}

// keyFor returns the key named in the path when me may manage it: the owner
// always, an admin for everybody's.
func (s *Server) keyFor(r *http.Request, me *users.User) (Key, error) {
	id := r.PathValue("id")
	if !keyIDShape.MatchString(id) {
		return Key{}, errNoSuchKey
	}
	k, err := s.st.key(r.Context(), id)
	if err != nil {
		return Key{}, keyProblem(err)
	}
	if k.UserID != me.ID && !me.Permissions.Admin {
		return Key{}, errNoSuchKey
	}
	return k, nil
}

func (s *Server) update(w http.ResponseWriter, r *http.Request) error {
	me := v1.UserFrom(r)
	var in struct {
		Label    *string `json:"label"`
		CanWrite *bool   `json:"canWrite"`
		Password string  `json:"password"`
	}
	if err := v1.DecodeJSON(r, &in); err != nil {
		return err
	}
	k, err := s.keyFor(r, me)
	if err != nil {
		return err
	}
	if s.login.ByAPIToken(r) {
		return errTokenRefused
	}
	if in.Label != nil {
		clean, ok := cleanLabel(*in.Label)
		if !ok {
			return errLabel
		}
		if k.UserID != me.ID {
			return errNotYourKey
		}
		in.Label = &clean
	}
	if in.CanWrite != nil && *in.CanWrite && !k.CanWrite {
		if k.UserID != me.ID {
			return errNotYourKey
		}
		if err := s.proveOwner(r, me, in.Password); err != nil {
			return err
		}
	}
	k, err = s.st.updateKey(r.Context(), k.ID, in.Label, in.CanWrite)
	if err != nil {
		return keyProblem(err)
	}
	if in.CanWrite != nil {
		event := "switched to read only"
		if *in.CanWrite {
			event = "allowed to write"
		}
		s.changed(r, k, event)
	}
	return v1.WriteJSON(w, r, http.StatusOK, s.view(k, me, s.ownerNames(k.UserID), nil))
}

func (s *Server) rotate(w http.ResponseWriter, r *http.Request) error {
	me := v1.UserFrom(r)
	var in struct {
		Password string `json:"password"`
	}
	if r.ContentLength != 0 {
		if err := v1.DecodeJSON(r, &in); err != nil {
			return err
		}
	}
	k, err := s.keyFor(r, me)
	if err != nil {
		return err
	}
	if k.UserID != me.ID {
		return errNotYourKey
	}
	if k.Kind != kindKey || k.RevokedAt != 0 {
		return errNoSuchKey
	}
	if err := s.proveOwner(r, me, in.Password); err != nil {
		return err
	}
	pepper := s.pepper()
	key := newSecret(keyPrefix)
	k, err = s.st.rotateKey(r.Context(), k.ID, hashKey(pepper, key), keyHint(key), keyCheck(pepper, k.ID), s.now().Unix())
	if err != nil {
		return keyProblem(err)
	}
	s.changed(r, k, "replaced")
	w.Header().Set("Cache-Control", "no-store")
	return v1.WriteJSON(w, r, http.StatusOK, map[string]any{"key": key, "item": s.view(k, me, s.ownerNames(me.ID), nil)})
}

func (s *Server) revokeOne(w http.ResponseWriter, r *http.Request) error {
	me := v1.UserFrom(r)
	k, err := s.keyFor(r, me)
	if err != nil {
		return err
	}
	if s.login.ByAPIToken(r) {
		return errTokenRefused
	}
	reason := "owner"
	if k.UserID != me.ID {
		reason = "admin"
	}
	if err := s.st.revokeKey(r.Context(), k.ID, reason, s.now().Unix()); err != nil {
		return keyProblem(err)
	}
	s.changed(r, k, "revoked")
	w.WriteHeader(http.StatusNoContent)
	return nil
}

func (s *Server) purge(w http.ResponseWriter, r *http.Request) error {
	me := v1.UserFrom(r)
	k, err := s.keyFor(r, me)
	if err != nil {
		return err
	}
	if s.login.ByAPIToken(r) {
		return errTokenRefused
	}
	if err := s.st.purgeKey(r.Context(), k.ID); err != nil {
		return keyProblem(err)
	}
	logger.Infof("mcp: key %s ...%s removed", k.ID, k.Hint)
	w.WriteHeader(http.StatusNoContent)
	return nil
}

func (s *Server) activity(w http.ResponseWriter, r *http.Request) error {
	me := v1.UserFrom(r)
	k, err := s.keyFor(r, me)
	if err != nil {
		return err
	}
	events, err := s.st.events(r.Context(), k.ID)
	if err != nil {
		return err
	}
	return v1.WriteJSON(w, r, http.StatusOK, map[string]any{"events": events})
}

// changed logs a change of a key and tells its owner: the card reloads, the
// activity log keeps it, and for a change worth waking somebody for Unraid
// shows a notice. The log line names the id and the hint, never the name.
func (s *Server) changed(r *http.Request, k Key, event string) {
	logger.Infof("mcp: key %s ...%s (%s) %s", k.ID, k.Hint, k.Kind, event)
	owner, err := s.user(k.UserID)
	if err != nil {
		return
	}
	if s.deps.Hub != nil {
		_, _ = s.deps.Hub.PublishJSON("mcp.changed", "", map[string]string{"id": k.ID}, owner.Username)
	}
	switch {
	case strings.HasPrefix(event, "made"), event == "replaced", event == "allowed to write", event == "granted access through OAuth":
		activity.RecordAuth(r, owner, activitydb.EventTokenCreate, activitydb.Details{AffectedTokenName: k.Label, AuthMethod: "mcp"})
		s.notify(owner, k, event)
	case strings.HasPrefix(event, "revoked"):
		activity.RecordAuth(r, owner, activitydb.EventTokenDelete, activitydb.Details{AffectedTokenName: k.Label, AuthMethod: "mcp"})
	}
}

// notify sends a notice to the Unraid notifications, where the server has
// them. It runs beside the request: a create carries the one-time key, which
// must not wait on anything.
func (s *Server) notify(owner *users.User, k Key, event string) {
	if desktop.Active() {
		return
	}
	n := unraid.Notice{
		Subject:     "CannonCommander: MCP key " + event,
		Description: fmt.Sprintf("The key %q of %s was %s. If that was not you, revoke it under Settings, Integrations, MCP server.", k.Label, owner.Username, event),
		Importance:  "warning",
	}
	go func() {
		if err := unraid.Default().Notify(n); err != nil && !errors.Is(err, unraid.ErrWriteOff) {
			logger.Debugf("mcp: notice: %v", err)
		}
	}()
}

type oauthView struct {
	Possible bool   `json:"possible"`
	Enabled  bool   `json:"enabled"`
	Issuer   string `json:"issuer"`
	Active   bool   `json:"active"`
	// Resource is the address a cloud connector is given, the endpoint under
	// the public address.
	Resource   string `json:"resource,omitempty"`
	GrantLimit int    `json:"grantLimit"`
}

func (s *Server) oauthView(ctx context.Context) (oauthView, error) {
	o, err := s.st.oauthSettings(ctx)
	if err != nil {
		return oauthView{}, err
	}
	_, active := s.issuer(ctx)
	v := oauthView{Possible: s.oauthPossible(), Enabled: o.Enabled, Issuer: o.Issuer, Active: active, GrantLimit: keyLimit}
	if o.Issuer != "" {
		v.Resource = resourceOf(o.Issuer, s.basePath())
	}
	return v, nil
}

// setOAuth stores the switch and the public address. Switching off is how an
// admin cuts every cloud client off, so the grants end there instead of
// waking up again with the switch; a new address leaves them with tokens
// nothing accepts any more.
func (s *Server) setOAuth(w http.ResponseWriter, r *http.Request) error {
	var in struct {
		Enabled bool   `json:"enabled"`
		Issuer  string `json:"issuer"`
	}
	if err := v1.DecodeJSON(r, &in); err != nil {
		return err
	}
	if s.login.ByAPIToken(r) {
		return errTokenRefused
	}
	if in.Enabled && !s.oauthPossible() {
		return v1.Coded(http.StatusBadRequest, "mcpOAuthNeedsSignIn", errOAuthNeedsSignIn.Error())
	}
	issuer := ""
	if in.Enabled || strings.TrimSpace(in.Issuer) != "" {
		var ok bool
		if issuer, ok = normalizeIssuer(in.Issuer); !ok {
			return v1.Coded(http.StatusBadRequest, "mcpOAuthIssuer", errOAuthIssuer.Error())
		}
	}
	ctx := r.Context()
	before, err := s.st.oauthSettings(ctx)
	if err != nil {
		return err
	}
	now := s.now().Unix()
	if err := s.st.setOAuthSettings(ctx, OAuthSettings{Enabled: in.Enabled, Issuer: issuer}, now); err != nil {
		return err
	}
	logger.Infof("mcp: sign-in through OAuth switched %s", map[bool]string{true: "on", false: "off"}[in.Enabled])
	reason, why := "", ""
	switch {
	case !in.Enabled:
		reason, why = "oauth-off", "revoked when sign-in through OAuth was switched off"
	case issuer != before.Issuer:
		reason, why = "oauth-moved", "revoked when the public address changed"
	}
	if reason != "" {
		rows, err := s.st.revokeGrants(ctx, reason, now)
		if err != nil {
			return err
		}
		for _, k := range rows {
			s.changed(r, k, why)
		}
	}
	view, err := s.oauthView(ctx)
	if err != nil {
		return err
	}
	return v1.WriteJSON(w, r, http.StatusOK, view)
}

func (s *Server) activeGrants(ctx context.Context, userID uint64, exceptClient string) (int, error) {
	rows, err := s.st.keys(ctx, userID)
	if err != nil {
		return 0, err
	}
	n := 0
	for _, k := range rows {
		if k.Kind == kindOAuth && k.RevokedAt == 0 && k.OAuthClient != exceptClient {
			n++
		}
	}
	return n, nil
}

var errConsentExpired = v1.Coded(http.StatusConflict, "oauthConsentExpired", "this page has expired, start the sign-in again from the client")

// consentInfo checks an authorization request for the consent page and hands
// it a ticket to answer with. Every fault goes to the user and none back to
// the client: anybody can register a client with any https return address,
// and sending a refusal there would make the page an open redirector (RFC
// 9700 section 4.11.2).
func (s *Server) consentInfo(w http.ResponseWriter, r *http.Request) error {
	ctx := r.Context()
	issuer, on := s.issuer(ctx)
	if !on {
		return v1.Coded(http.StatusNotFound, "oauthOff", "sign-in through OAuth is not switched on")
	}
	session, err := s.consentSession(r)
	if err != nil {
		return err
	}
	me := v1.UserFrom(r)
	q := r.URL.Query()
	client, err := s.st.client(ctx, q.Get("client_id"))
	if errors.Is(err, errClientUnknown) {
		return v1.Coded(http.StatusBadRequest, "oauthUnknownClient", "this client is not registered here; remove CannonCommander from the client and add it again")
	}
	if err != nil {
		return err
	}
	redirect := q.Get("redirect_uri")
	if !redirectRegistered(client.RedirectURIs, redirect) {
		return v1.Coded(http.StatusBadRequest, "oauthBadRedirect", "the return address of this request is not one the client registered")
	}
	resource := resourceOf(issuer, s.basePath())
	switch {
	case q.Get("response_type") != "code":
		return v1.Coded(http.StatusBadRequest, "oauthInvalidRequest", "only the authorization code flow is supported")
	case q.Get("code_challenge_method") != "S256" || !validChallenge(q.Get("code_challenge")):
		return v1.Coded(http.StatusBadRequest, "oauthInvalidRequest", "a PKCE code challenge with the S256 method is required")
	case q.Get("resource") != "" && !sameResource(q.Get("resource"), resource):
		return v1.Coded(http.StatusBadRequest, "oauthInvalidRequest", "this server issues tokens for "+resource+" only")
	}
	others, err := s.activeGrants(ctx, me.ID, client.ID)
	if err != nil {
		return err
	}
	u, _ := url.Parse(redirect)
	ticket := s.signConsent(session, consentTicket{
		Client: client.ID, Redirect: redirect, Challenge: q.Get("code_challenge"), Resource: resource,
		State: q.Get("state"), User: me.ID, Expires: s.now().Add(consentTTL).Unix(),
	})
	w.Header().Set("Cache-Control", "no-store")
	return v1.WriteJSON(w, r, http.StatusOK, map[string]any{
		"client": map[string]any{
			"id": client.ID, "name": client.Name, "known": client.Known,
			"redirectHost": u.Hostname(), "loopback": loopback(u.Hostname()),
		},
		"account":      me.Username,
		"password":     me.LoginMethod == users.LoginMethodPassword,
		"ticket":       ticket,
		"limitReached": others >= keyLimit,
		"grantLimit":   keyLimit,
	})
}

// consent takes the user's answer. Allow hands the client a code for the
// token endpoint, which makes a lasting credential like a key, so it asks for
// the password or a fresh sign-in as making a key does. A refused proof
// leaves the ticket unspent, so the page can ask again. Deny tells the client
// access_denied.
func (s *Server) consent(w http.ResponseWriter, r *http.Request) error {
	var in struct {
		Ticket   string `json:"ticket"`
		Allow    bool   `json:"allow"`
		CanWrite bool   `json:"canWrite"`
		Password string `json:"password"`
	}
	if err := v1.DecodeJSON(r, &in); err != nil {
		return err
	}
	ctx := r.Context()
	issuer, on := s.issuer(ctx)
	if !on {
		return v1.Coded(http.StatusNotFound, "oauthOff", "sign-in through OAuth is not switched on")
	}
	session, err := s.consentSession(r)
	if err != nil {
		return err
	}
	me := v1.UserFrom(r)
	if in.Allow {
		if err := s.proveOwner(r, me, in.Password); err != nil {
			return err
		}
	}
	now := s.now()
	t, ok := s.openConsent(session, in.Ticket, now)
	if !ok || t.User != me.ID {
		return errConsentExpired
	}
	client, err := s.st.client(ctx, t.Client)
	if errors.Is(err, errClientUnknown) {
		return v1.Coded(http.StatusBadRequest, "oauthUnknownClient", "this client is not registered here; remove CannonCommander from the client and add it again")
	}
	if err != nil {
		return err
	}
	addr := s.login.ClientIP(r)
	if !in.Allow {
		logger.Infof("mcp: oauth: client %s was denied by %s from %s", client.ID, me.Username, addr)
		return v1.WriteJSON(w, r, http.StatusOK, map[string]string{
			"redirect": authorizationResponse(t.Redirect, issuer, t.State, url.Values{"error": {"access_denied"}}),
		})
	}
	others, err := s.activeGrants(ctx, me.ID, client.ID)
	if err != nil {
		return err
	}
	if others >= keyLimit {
		return v1.Coded(http.StatusConflict, "mcpGrantLimit", "this account already has as many signed-in clients as it may")
	}
	label := knownClientName(client.Known)
	if label == "" {
		label = client.Name
	}
	if label == "" {
		label = "OAuth client"
	}
	code := newSecret(codePrefix)
	pending := &pendingCode{
		clientID: client.ID, redirectURI: t.Redirect, challenge: t.Challenge, resource: t.Resource,
		label: label, known: client.Known, userID: me.ID, canWrite: in.CanWrite, expires: now.Add(codeTTL),
	}
	if !s.oauth.putCode(hashOAuth(s.pepper(), "code", code), pending, now) {
		return v1.Coded(http.StatusConflict, "oauthBusy", "too many sign-ins are waiting, try again in a minute")
	}
	logger.Infof("mcp: oauth: client %s was allowed by %s from %s", client.ID, me.Username, addr)
	w.Header().Set("Cache-Control", "no-store")
	return v1.WriteJSON(w, r, http.StatusOK, map[string]string{
		"redirect": authorizationResponse(t.Redirect, issuer, t.State, url.Values{"code": {code}}),
	})
}
