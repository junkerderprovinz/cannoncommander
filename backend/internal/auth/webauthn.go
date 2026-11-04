package auth

import (
	"crypto/rand"
	"encoding/base64"
	"encoding/hex"
	"fmt"
	"net/url"
	"strconv"
	"sync"
	"time"

	"github.com/go-webauthn/webauthn/protocol"
	"github.com/go-webauthn/webauthn/webauthn"
	"github.com/junkerderprovinz/cannoncommander/backend/internal/database/users"
	"github.com/junkerderprovinz/cannoncommander/backend/internal/errors"
	"github.com/junkerderprovinz/cannoncommander/backend/internal/ports"
	"github.com/junkerderprovinz/cannoncommander/backend/pkg/settings"
)

type webAuthnSession struct {
	SessionData *webauthn.SessionData
	UserID      uint
	ExpiresAt   time.Time
}

type WebAuthnService struct {
	wa       *webauthn.WebAuthn
	users    ports.UserReader
	sessions sync.Map
	stopCh   chan struct{}
	originMu sync.Mutex
}

var webAuthnInstance *WebAuthnService

func InitWebAuthn(users ports.UserReader) error {
	cfg := &settings.Config.Auth.Methods.PasskeyAuth
	if !cfg.Enabled {
		webAuthnInstance = nil
		return nil
	}

	waCfg, err := deriveWebAuthnConfig(cfg)
	if err != nil {
		return fmt.Errorf("webauthn config derivation failed: %w", err)
	}

	wa, err := webauthn.New(waCfg)
	if err != nil {
		return fmt.Errorf("failed to initialize WebAuthn: %w", err)
	}

	webAuthnInstance = &WebAuthnService{
		wa:     wa,
		users:  users,
		stopCh: make(chan struct{}),
	}
	go webAuthnInstance.cleanupExpiredSessions()
	return nil
}

func GetWebAuthn() *WebAuthnService {
	return webAuthnInstance
}

func IsWebAuthnEnabled() bool {
	return webAuthnInstance != nil
}

// EnsureOrigin dynamically adds the given origin to the allowed RP origins list.
// This supports self-hosted deployments where the access domain may vary.
func (s *WebAuthnService) EnsureOrigin(origin string) {
	s.originMu.Lock()
	defer s.originMu.Unlock()
	for _, o := range s.wa.Config.RPOrigins {
		if o == origin {
			return
		}
	}
	s.wa.Config.RPOrigins = append(s.wa.Config.RPOrigins, origin)
}

// EnsureRPID dynamically updates the relying party ID to match the request host.
func (s *WebAuthnService) EnsureRPID(rpID string) {
	s.originMu.Lock()
	defer s.originMu.Unlock()
	s.wa.Config.RPID = rpID
}

func generateSessionID() string {
	b := make([]byte, 16)
	if _, err := rand.Read(b); err != nil {
		return hex.EncodeToString([]byte(fmt.Sprintf("%d", time.Now().UnixNano())))
	}
	return hex.EncodeToString(b)
}

func (s *WebAuthnService) userByID(id uint64) (*users.User, error) {
	if s.users == nil {
		return nil, errors.ErrNotExist
	}
	u, err := s.users.GetUserByID(id)
	if err != nil {
		return nil, err
	}
	return &u, nil
}

// BeginPasskeyLogin starts a sign in where the browser picks the passkey and
// the passkey names the user, so nobody types a name first.
func (s *WebAuthnService) BeginPasskeyLogin(rpID string) (sessionID string, assertion *protocol.CredentialAssertion, err error) {
	var opts []webauthn.LoginOption
	if rpID != "" {
		opts = append(opts, webauthn.WithLoginRelyingPartyID(rpID))
	}
	opts = append(opts, webauthn.WithUserVerification(protocol.VerificationRequired))
	assertion, sessionData, err := s.wa.BeginDiscoverableLogin(opts...)
	if err != nil {
		return "", nil, fmt.Errorf("failed to begin WebAuthn login: %w", err)
	}
	sessionID = generateSessionID()
	s.sessions.Store(sessionID, &webAuthnSession{SessionData: sessionData, ExpiresAt: time.Now().Add(5 * time.Minute)})
	return sessionID, assertion, nil
}

// FinishPasskeyLogin checks the browser's answer to BeginPasskeyLogin and
// returns the user the passkey belongs to, with the passkey's counter and
// last use updated but not saved. A session works once.
func (s *WebAuthnService) FinishPasskeyLogin(sessionID string, response []byte) (*users.User, error) {
	val, ok := s.sessions.LoadAndDelete(sessionID)
	if !ok {
		return nil, errors.ErrPasskeyInvalidSession
	}
	sess, ok := val.(*webAuthnSession)
	if !ok || time.Now().After(sess.ExpiresAt) {
		return nil, errors.ErrPasskeyInvalidSession
	}
	parsed, err := protocol.ParseCredentialRequestResponseBytes(response)
	if err != nil {
		return nil, fmt.Errorf("%w: %v", errors.ErrPasskeyVerification, err)
	}
	var owner *users.User
	byHandle := func(_, userHandle []byte) (webauthn.User, error) {
		id, err := strconv.ParseUint(string(userHandle), 10, 64)
		if err != nil {
			return nil, errors.ErrPasskeyNoCredential
		}
		if owner, err = s.userByID(id); err != nil {
			return nil, err
		}
		return owner, nil
	}
	credential, err := s.wa.ValidateDiscoverableLogin(byHandle, *sess.SessionData, parsed)
	if err != nil {
		return nil, fmt.Errorf("%w: %v", errors.ErrPasskeyVerification, err)
	}
	id := base64.RawURLEncoding.EncodeToString(credential.ID)
	for i := range owner.PasskeyCredentials {
		if owner.PasskeyCredentials[i].ID == id {
			users.UpdateCredentialFromLibrary(&owner.PasskeyCredentials[i], credential)
		}
	}
	return owner, nil
}

func (s *WebAuthnService) BeginRegistration(user *users.User, rpID string) (sessionID string, creation *protocol.CredentialCreation, err error) {
	var opts []webauthn.RegistrationOption
	if rpID != "" {
		opts = append(opts, webauthn.WithRegistrationRelyingPartyID(rpID))
	}
	// A passkey the authenticator keeps for itself can sign in without a
	// name, which is how the sign in page uses it.
	opts = append(opts, webauthn.WithResidentKeyRequirement(protocol.ResidentKeyRequirementPreferred))
	creation, sessionData, err := s.wa.BeginRegistration(user, opts...)
	if err != nil {
		return "", nil, fmt.Errorf("failed to begin WebAuthn registration: %w", err)
	}

	sessionID = generateSessionID()
	s.sessions.Store(sessionID, &webAuthnSession{
		SessionData: sessionData,
		UserID:      uint(user.ID),
		ExpiresAt:   time.Now().Add(5 * time.Minute),
	})

	return sessionID, creation, nil
}

// FinishRegistrationJSON is FinishRegistration for a credential that came
// as JSON inside a larger request body.
func (s *WebAuthnService) FinishRegistrationJSON(user *users.User, sessionID string, credentialName string, credential []byte) error {
	parsed, err := protocol.ParseCredentialCreationResponseBytes(credential)
	if err != nil {
		return fmt.Errorf("failed to finish WebAuthn registration: %w", err)
	}
	return s.finishRegistration(user, sessionID, credentialName, parsed)
}

func (s *WebAuthnService) finishRegistration(user *users.User, sessionID string, credentialName string, parsed *protocol.ParsedCredentialCreationData) error {
	sessionVal, ok := s.sessions.Load(sessionID)
	if !ok {
		return errors.ErrPasskeyInvalidSession
	}
	sess, ok := sessionVal.(*webAuthnSession)
	if !ok {
		return errors.ErrPasskeyInvalidSession
	}
	defer s.sessions.Delete(sessionID)

	if time.Now().After(sess.ExpiresAt) {
		return errors.ErrPasskeyInvalidSession
	}

	if sess.UserID != uint(user.ID) {
		return errors.ErrUnauthorized
	}

	credential, err := s.wa.CreateCredential(user, *sess.SessionData, parsed)
	if err != nil {
		return fmt.Errorf("failed to finish WebAuthn registration: %w", err)
	}

	credB64 := base64.RawURLEncoding.EncodeToString(credential.ID)
	for _, c := range user.PasskeyCredentials {
		if c.ID == credB64 {
			return errors.ErrPasskeyExists
		}
	}

	newCred := users.CredentialFromLibrary(credentialName, credential)
	user.PasskeyCredentials = append(user.PasskeyCredentials, newCred)
	return nil
}

func (s *WebAuthnService) cleanupExpiredSessions() {
	ticker := time.NewTicker(2 * time.Minute)
	defer ticker.Stop()
	for {
		select {
		case <-ticker.C:
			now := time.Now()
			s.sessions.Range(func(key, value interface{}) bool {
				sess, ok := value.(*webAuthnSession)
				if !ok {
					s.sessions.Delete(key)
					return true
				}
				if now.After(sess.ExpiresAt) {
					s.sessions.Delete(key)
				}
				return true
			})
		case <-s.stopCh:
			return
		}
	}
}

func deriveWebAuthnConfig(cfg *settings.PasskeyAuthConfig) (*webauthn.Config, error) {
	rpID := cfg.RPID
	if rpID == "" {
		if settings.Config.Http.ExternalUrl != "" {
			if u, err := url.Parse(settings.Config.Http.ExternalUrl); err == nil {
				rpID = u.Hostname()
			}
		}
		if rpID == "" {
			rpID = "localhost"
		}
	}

	displayName := cfg.RPDisplayName
	if displayName == "" {
		displayName = settings.Config.Frontend.Name
	}
	if displayName == "" {
		displayName = "CannonCommander"
	}

	origins := cfg.RPOrigins
	if len(origins) == 0 {
		if settings.Config.Http.ExternalUrl != "" {
			origins = append(origins, settings.Config.Http.ExternalUrl)
		}
		if settings.Config.Http.InternalUrl != "" && settings.Config.Http.InternalUrl != settings.Config.Http.ExternalUrl {
			origins = append(origins, settings.Config.Http.InternalUrl)
		}
		if len(origins) == 0 {
			origins = append(origins, "http://localhost")
		}
	}

	return &webauthn.Config{
		RPID:                  rpID,
		RPDisplayName:         displayName,
		RPOrigins:             origins,
		AttestationPreference: protocol.PreferNoAttestation,
		AuthenticatorSelection: protocol.AuthenticatorSelection{
			UserVerification: protocol.VerificationPreferred,
		},
	}, nil
}
