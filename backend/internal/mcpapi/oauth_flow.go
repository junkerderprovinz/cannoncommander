package mcpapi

import (
	"encoding/base64"
	"encoding/json"
	"errors"
	"io"
	"mime"
	"net/http"
	"net/url"
	"slices"
	"strings"
	"time"

	"github.com/gtsteffaniak/go-logger/logger"

	v1 "github.com/junkerderprovinz/cannoncommander/backend/internal/api/v1"
)

// oauthError answers a token, registration or revocation request with the
// error object of RFC 6749 section 5.2.
func oauthError(w http.ResponseWriter, status int, code, description string) {
	if status == http.StatusUnauthorized {
		w.Header().Set("WWW-Authenticate", `Basic realm="cannoncommander"`)
	}
	writeJSON(w, status, map[string]any{"error": code, "error_description": description})
}

// register is dynamic client registration (RFC 7591). Anybody who reaches the
// address may register, so a registration grants nothing: a user still has to
// sign in and allow the client.
func (s *Server) register(w http.ResponseWriter, r *http.Request) {
	if _, on := s.issuer(r.Context()); !on {
		http.NotFound(w, r)
		return
	}
	now := s.now()
	addr := s.login.ClientIP(r)
	if ok, retry := s.oauth.registrations.allow(registrationKey(addr), now); !ok {
		w.Header().Set("Retry-After", seconds(retry))
		oauthError(w, http.StatusTooManyRequests, "invalid_request", "too many registrations from this address, try again later")
		return
	}
	if mt, _, err := mime.ParseMediaType(r.Header.Get("Content-Type")); err != nil || mt != "application/json" {
		oauthError(w, http.StatusBadRequest, "invalid_client_metadata", "the registration must be sent as application/json")
		return
	}
	var body struct {
		RedirectURIs []string `json:"redirect_uris"`
		ClientName   string   `json:"client_name"`
		GrantTypes   []string `json:"grant_types"`
		Response     []string `json:"response_types"`
		AuthMethod   *string  `json:"token_endpoint_auth_method"`
	}
	if err := json.NewDecoder(io.LimitReader(r.Body, formMax)).Decode(&body); err != nil {
		oauthError(w, http.StatusBadRequest, "invalid_client_metadata", "the registration is not valid JSON")
		return
	}
	if len(body.RedirectURIs) == 0 || len(body.RedirectURIs) > 5 {
		oauthError(w, http.StatusBadRequest, "invalid_redirect_uri", "register between one and five redirect URIs")
		return
	}
	for _, uri := range body.RedirectURIs {
		if !validRedirect(uri) {
			oauthError(w, http.StatusBadRequest, "invalid_redirect_uri", "a redirect URI uses https, or http to a loopback address, names its host in ASCII and carries no fragment")
			return
		}
	}
	for _, g := range body.GrantTypes {
		if g != "authorization_code" && g != "refresh_token" {
			oauthError(w, http.StatusBadRequest, "invalid_client_metadata", "only the authorization_code and refresh_token grants are supported")
			return
		}
	}
	for _, rt := range body.Response {
		if rt != "code" {
			oauthError(w, http.StatusBadRequest, "invalid_client_metadata", "only the code response type is supported")
			return
		}
	}
	// RFC 7591 section 2 makes client_secret_basic the method of a client
	// that names none.
	method := "client_secret_basic"
	if body.AuthMethod != nil {
		method = *body.AuthMethod
	}
	if !slices.Contains(authMethods, method) {
		oauthError(w, http.StatusBadRequest, "invalid_client_metadata", "token_endpoint_auth_method is none, client_secret_basic or client_secret_post")
		return
	}
	c := oauthClient{
		ID:           newSecret(clientIDPrefix),
		Name:         cleanClientName(body.ClientName),
		RedirectURIs: body.RedirectURIs,
		AuthMethod:   method,
		Known:        knownClient(body.RedirectURIs),
		CreatedFrom:  addr,
	}
	var secret string
	if method != "none" {
		secret = newSecret(clientSecretPrefix)
		c.SecretDigest = hashOAuth(s.pepper(), "client", secret)
	}
	if err := s.st.createClient(r.Context(), c, now.Unix()); err != nil {
		logger.Errorf("mcp: oauth: store a registration: %v", err)
		oauthError(w, http.StatusInternalServerError, "server_error", "could not store the client")
		return
	}
	logger.Infof("mcp: oauth: client %s registered from %s", c.ID, addr)
	out := map[string]any{
		"client_id":                  c.ID,
		"client_id_issued_at":        now.Unix(),
		"client_name":                c.Name,
		"redirect_uris":              c.RedirectURIs,
		"grant_types":                []string{"authorization_code", "refresh_token"},
		"response_types":             []string{"code"},
		"token_endpoint_auth_method": method,
	}
	if secret != "" {
		out["client_secret"] = secret
		out["client_secret_expires_at"] = 0
	}
	writeJSON(w, http.StatusCreated, out)
}

// consentTicket is what the consent page answers with: the request it showed,
// signed together with the session of the user who saw it. A page on another
// site has neither the session nor the ticket, and the ticket cannot move to
// another session or another request.
type consentTicket struct {
	Client    string `json:"c"`
	Redirect  string `json:"r"`
	Challenge string `json:"x"`
	Resource  string `json:"s"`
	State     string `json:"t"`
	User      uint64 `json:"u"`
	Expires   int64  `json:"e"`
}

func (s *Server) consentMAC(session, payload string) string {
	return mac(s.pepper(), consentDomain+session+"."+payload)
}

func (s *Server) signConsent(session string, t consentTicket) string {
	raw, _ := json.Marshal(t)
	payload := base64.RawURLEncoding.EncodeToString(raw)
	return payload + "." + s.consentMAC(session, payload)
}

// openConsent verifies a ticket against the session that sent it and spends
// it, so an answer cannot be sent twice.
func (s *Server) openConsent(session, ticket string, now time.Time) (consentTicket, bool) {
	payload, sig, found := strings.Cut(ticket, ".")
	if !found || !sameString(sig, s.consentMAC(session, payload)) {
		return consentTicket{}, false
	}
	raw, err := base64.RawURLEncoding.DecodeString(payload)
	var t consentTicket
	if err != nil || json.Unmarshal(raw, &t) != nil || t.Expires <= now.Unix() {
		return consentTicket{}, false
	}
	o := s.oauth
	o.mu.Lock()
	defer o.mu.Unlock()
	for m, exp := range o.tickets {
		if exp <= now.Unix() {
			delete(o.tickets, m)
		}
	}
	if _, spent := o.tickets[sig]; spent {
		return consentTicket{}, false
	}
	o.tickets[sig] = t.Expires
	return t, true
}

// consentSession returns the session a consent is bound to. An API token or a
// key of this server is no session: what it allowed would outlive the person
// who never saw the page.
func (s *Server) consentSession(r *http.Request) (string, error) {
	if s.login.ByAPIToken(r) {
		return "", v1.Coded(http.StatusForbidden, "tokenCannotChange", "sign in to allow a client, an API token cannot")
	}
	hash, ok := s.login.SessionHash(r)
	if !ok {
		return "", v1.Coded(http.StatusForbidden, "sessionNeeded", "sign in to allow a client")
	}
	return hash, nil
}

func withQuery(uri string, v url.Values) string {
	u, err := url.Parse(uri)
	if err != nil {
		return uri
	}
	q := u.Query()
	for k, vals := range v {
		q[k] = vals
	}
	u.RawQuery = q.Encode()
	return u.String()
}

// authorizationResponse is what goes back to the client: the result plus the
// state it sent and the issuer, so it can tell which server answered.
func authorizationResponse(redirect, issuer, state string, v url.Values) string {
	v.Set("iss", issuer)
	if state != "" {
		v.Set("state", state)
	}
	return withQuery(redirect, v)
}

// putCode keeps a code until it expires, refusing a new one while too many
// wait, in all or for the one account, so nobody can fill the place the
// others need for their sign-in.
func (o *oauthState) putCode(digest string, c *pendingCode, now time.Time) bool {
	o.mu.Lock()
	defer o.mu.Unlock()
	mine := 0
	for d, p := range o.codes {
		if !now.Before(p.expires) {
			delete(o.codes, d)
			continue
		}
		if p.userID == c.userID && !p.used {
			mine++
		}
	}
	if len(o.codes) >= pendingMax || mine >= pendingPerUser {
		return false
	}
	o.codes[digest] = c
	return true
}

// redeem hands out a code once. A code that was redeemed already answers with
// the grant it made, which the caller revokes. A wrong client, redirect URI or
// verifier leaves the code as it was, so a forged exchange cannot spend it.
func (o *oauthState) redeem(digest, client, redirect, verifier string, now time.Time) (c *pendingCode, replayedGrant string) {
	o.mu.Lock()
	defer o.mu.Unlock()
	c = o.codes[digest]
	if c == nil {
		return nil, ""
	}
	if !now.Before(c.expires) {
		delete(o.codes, digest)
		return nil, ""
	}
	if c.used {
		return nil, c.grantID
	}
	if c.clientID != client || c.redirectURI != redirect || !validVerifier(verifier) || !pkceMatches(verifier, c.challenge) {
		return nil, ""
	}
	c.used = true
	c.grantID = newID()
	return c, ""
}

// clientFrom authenticates the client of a token or revocation request: by
// client_id alone for a public client, and with its secret from the Basic
// header or the form otherwise.
func (s *Server) clientFrom(r *http.Request) (oauthClient, bool) {
	id, sec := r.PostForm.Get("client_id"), r.PostForm.Get("client_secret")
	if user, pass, basic := r.BasicAuth(); basic {
		u, errU := url.QueryUnescape(user)
		p, errP := url.QueryUnescape(pass)
		if errU != nil || errP != nil || (id != "" && id != u) {
			return oauthClient{}, false
		}
		id, sec = u, p
	}
	if id == "" {
		return oauthClient{}, false
	}
	c, err := s.st.client(r.Context(), id)
	if err != nil {
		if !errors.Is(err, errClientUnknown) {
			logger.Errorf("mcp: oauth: read a client: %v", err)
		}
		return oauthClient{}, false
	}
	if c.AuthMethod == "none" {
		return c, true
	}
	return c, sec != "" && sameString(hashOAuth(s.pepper(), "client", sec), c.SecretDigest)
}

// form reads the form body of the token or revocation endpoint and answers a
// refusal itself.
func form(w http.ResponseWriter, r *http.Request) bool {
	if mt, _, err := mime.ParseMediaType(r.Header.Get("Content-Type")); err != nil || mt != "application/x-www-form-urlencoded" {
		oauthError(w, http.StatusBadRequest, "invalid_request", "the request must be sent as application/x-www-form-urlencoded")
		return false
	}
	r.Body = http.MaxBytesReader(w, r.Body, formMax)
	if err := r.ParseForm(); err != nil {
		oauthError(w, http.StatusBadRequest, "invalid_request", "the request body could not be read")
		return false
	}
	return true
}

// token is the token endpoint: a code for tokens, and a refresh token for new
// ones. Codes and tokens carry 256 bits, so there is nothing to guess and a
// wrong one costs nothing; only a wrong client secret counts, towards a
// throttle of that client.
func (s *Server) token(w http.ResponseWriter, r *http.Request) {
	issuer, on := s.issuer(r.Context())
	if !on {
		http.NotFound(w, r)
		return
	}
	if !form(w, r) {
		return
	}
	now := s.now()
	client, ok := s.clientFrom(r)
	bucket := "client|" + client.ID
	if client.ID != "" && s.authFails.left(bucket, now) <= 0 {
		w.Header().Set("Retry-After", "60")
		oauthError(w, http.StatusTooManyRequests, "invalid_request", "too many failed attempts, wait a minute")
		return
	}
	if !ok {
		if client.ID != "" {
			s.authFails.allow(bucket, now)
		}
		oauthError(w, http.StatusUnauthorized, "invalid_client", "the client is unknown or did not authenticate")
		return
	}
	resource := resourceOf(issuer, s.basePath())
	if res := r.PostForm.Get("resource"); res != "" && !sameResource(res, resource) {
		oauthError(w, http.StatusBadRequest, "invalid_target", "this server issues tokens for "+resource+" only")
		return
	}
	switch r.PostForm.Get("grant_type") {
	case "authorization_code":
		s.exchangeCode(w, r, client, resource)
	case "refresh_token":
		s.refresh(w, r, client, resource)
	default:
		oauthError(w, http.StatusBadRequest, "unsupported_grant_type", "only authorization_code and refresh_token are supported")
	}
}

func (s *Server) newTokens() (access, refresh, accessDigest, refreshDigest string) {
	access, refresh = newSecret(accessPrefix), newSecret(refreshPrefix)
	pepper := s.pepper()
	return access, refresh, hashOAuth(pepper, "access", access), hashOAuth(pepper, "refresh", refresh)
}

func writeTokens(w http.ResponseWriter, access, refresh string) {
	w.Header().Set("Pragma", "no-cache")
	writeJSON(w, http.StatusOK, map[string]any{
		"access_token":  access,
		"token_type":    "Bearer",
		"expires_in":    int(accessTTL.Seconds()),
		"refresh_token": refresh,
		"scope":         scope,
	})
}

func (s *Server) exchangeCode(w http.ResponseWriter, r *http.Request, client oauthClient, resource string) {
	ctx := r.Context()
	now := s.now()
	f := r.PostForm
	code, replayed := s.oauth.redeem(hashOAuth(s.pepper(), "code", f.Get("code")), client.ID, f.Get("redirect_uri"), f.Get("code_verifier"), now)
	if replayed != "" {
		// Somebody holds a copy of the code. The grant it made can no longer
		// be told apart from theirs, so it goes.
		if row, err := s.st.key(ctx, replayed); err == nil {
			if err := s.st.revokeKey(ctx, replayed, "code-replay", now.Unix()); err == nil {
				s.changed(r, row, "revoked after its code was used twice")
			}
		}
	}
	// A code allowed before the public address changed would make a grant
	// whose tokens the endpoint no longer accepts.
	if code == nil || code.resource != resource {
		oauthError(w, http.StatusBadRequest, "invalid_grant", "the code is unknown, expired, already used, or does not match this client, redirect URI and verifier")
		return
	}
	if _, err := s.user(code.userID); err != nil {
		oauthError(w, http.StatusBadRequest, "invalid_grant", "the account that allowed this client is gone")
		return
	}
	access, refresh, accessDigest, refreshDigest := s.newTokens()
	if err := s.st.pruneOAuth(ctx, now.Unix()); err != nil {
		logger.Errorf("mcp: oauth: prune: %v", err)
	}
	g, err := s.st.createGrant(ctx, grant{
		ID: code.grantID, Label: code.label, Client: code.known, OAuthClient: client.ID, Resource: code.resource,
		Check: keyCheck(s.pepper(), code.grantID), UserID: code.userID, CanWrite: code.canWrite,
		AccessDigest: accessDigest, RefreshDigest: refreshDigest,
		AccessExpires: now.Add(accessTTL).Unix(), RefreshExpires: now.Add(refreshTTL).Unix(),
	}, now.Unix())
	if errors.Is(err, errGrantLimit) {
		oauthError(w, http.StatusBadRequest, "invalid_grant", "this account already has as many signed-in clients as it may")
		return
	}
	if err != nil {
		logger.Errorf("mcp: oauth: store a grant: %v", err)
		oauthError(w, http.StatusInternalServerError, "server_error", "could not store the grant")
		return
	}
	s.changed(r, g, "granted access through OAuth")
	writeTokens(w, access, refresh)
}

func (s *Server) refresh(w http.ResponseWriter, r *http.Request, client oauthClient, resource string) {
	now := s.now()
	access, refresh, accessDigest, refreshDigest := s.newTokens()
	old := hashOAuth(s.pepper(), "refresh", r.PostForm.Get("refresh_token"))
	g, err := s.st.rotateRefresh(r.Context(), old, client.ID, resource, accessDigest, refreshDigest,
		now.Add(accessTTL).Unix(), now.Add(refreshTTL).Unix(), now.Unix())
	switch {
	case errors.Is(err, errRefreshReused):
		s.changed(r, g, "revoked after one of its refresh tokens was used twice")
		oauthError(w, http.StatusBadRequest, "invalid_grant", "the refresh token was already used")
		return
	case errors.Is(err, errTokenNotFound):
		oauthError(w, http.StatusBadRequest, "invalid_grant", "the refresh token is unknown, expired or revoked")
		return
	case err != nil:
		logger.Errorf("mcp: oauth: rotate a refresh token: %v", err)
		oauthError(w, http.StatusInternalServerError, "server_error", "could not rotate the refresh token")
		return
	}
	writeTokens(w, access, refresh)
}

// revoke is token revocation (RFC 7009). An unknown token is answered like a
// revoked one.
func (s *Server) revoke(w http.ResponseWriter, r *http.Request) {
	if _, on := s.issuer(r.Context()); !on {
		http.NotFound(w, r)
		return
	}
	if !form(w, r) {
		return
	}
	client, ok := s.clientFrom(r)
	if !ok {
		oauthError(w, http.StatusUnauthorized, "invalid_client", "the client is unknown or did not authenticate")
		return
	}
	token := r.PostForm.Get("token")
	kind := ""
	switch {
	case strings.HasPrefix(token, accessPrefix):
		kind = "access"
	case strings.HasPrefix(token, refreshPrefix):
		kind = "refresh"
	}
	if kind != "" {
		g, err := s.st.revokeToken(r.Context(), hashOAuth(s.pepper(), kind, token), client.ID, s.now().Unix())
		if err != nil {
			logger.Errorf("mcp: oauth: revoke a token: %v", err)
			oauthError(w, http.StatusServiceUnavailable, "server_error", "could not revoke the token")
			return
		}
		if g.ID != "" {
			s.changed(r, g, "revoked by its client")
		}
	}
	w.Header().Set("Cache-Control", "no-store")
	w.WriteHeader(http.StatusOK)
}
