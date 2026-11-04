package auth

import (
	"fmt"
	"net/http"
	"net/url"

	"github.com/gtsteffaniak/go-logger/logger"
	"github.com/junkerderprovinz/cannoncommander/backend/internal/database/users"
	"github.com/junkerderprovinz/cannoncommander/backend/internal/errors"
	"github.com/junkerderprovinz/cannoncommander/backend/internal/ports"
	"github.com/junkerderprovinz/cannoncommander/backend/internal/utils"
)

// Service performs authentication using injected user reads. It never persists users.
type Service struct {
	users ports.UserReader
}

var defaultService *Service

// New constructs an auth service with the given user reader.
func New(users ports.UserReader) *Service {
	return &Service{users: users}
}

// SetDefault registers the process-wide auth service (called from app.WireServices).
func SetDefault(s *Service) {
	defaultService = s
}

// AuthenticatePassword authenticates the user via password in request headers.
// When skipRequirePasswordChange is true, a valid password is accepted even if RequirePasswordChange is set (change-required flow).
func (s *Service) AuthenticatePassword(r *http.Request, disableOtp bool, skipRequirePasswordChange bool) (*users.User, error) {
	if s == nil || s.users == nil {
		return nil, fmt.Errorf("auth service not configured")
	}
	username := r.URL.Query().Get("username")
	password := r.Header.Get("X-Password")
	totpCode := r.Header.Get("X-Secret")
	password, err := url.QueryUnescape(password)
	if err != nil {
		return nil, fmt.Errorf("invalid password encoding: %v", err)
	}

	id, resErr := users.ResolveUsernameToID(username)
	var user users.User
	var getErr error
	if resErr != nil {
		getErr = resErr
	} else {
		user, getErr = s.users.GetUserByID(id)
	}
	var passwordHash string
	if getErr != nil {
		passwordHash = utils.InvalidPasswordHash
	} else {
		passwordHash = user.Password
	}
	err = utils.CheckPwd(password, passwordHash)
	if err != nil {
		return nil, err
	}
	if getErr != nil {
		return nil, fmt.Errorf("unable to get user from store: %v", err)
	}
	if user.RequirePasswordChange && user.LoginMethod == users.LoginMethodPassword && !skipRequirePasswordChange {
		return nil, errors.ErrPasswordChangeRequired
	}
	if user.TOTPSecret != "" && !disableOtp {
		if totpCode == "" {
			return nil, errors.ErrNoTotpProvided
		}
		if !redeemRecoveryCode(user.ID, totpCode) {
			if err = VerifyTotpCode(&user, totpCode); err != nil {
				return nil, err
			}
		}
	}
	if user.LoginMethod != users.LoginMethodPassword {
		logger.Debugf("login rejected: user %q attempted password login but account loginMethod is %q", username, user.LoginMethod)
		return nil, errors.ErrWrongLoginMethod
	}
	return &user, nil
}

// RecoveryCodes redeems the one time codes a user may give instead of a
// TOTP code when the phone is not at hand.
type RecoveryCodes interface {
	Redeem(userID uint64, code string) bool
}

var recoveryCodes RecoveryCodes

// UseRecoveryCodes connects the password login to the recovery codes. The
// server calls it once before it starts serving.
func UseRecoveryCodes(r RecoveryCodes) { recoveryCodes = r }

func redeemRecoveryCode(userID uint64, code string) bool {
	return recoveryCodes != nil && recoveryCodes.Redeem(userID, code)
}

// AuthenticatePassword authenticates via the default service.
func AuthenticatePassword(r *http.Request, disableOtp bool, skipRequirePasswordChange bool) (*users.User, error) {
	if defaultService != nil {
		return defaultService.AuthenticatePassword(r, disableOtp, skipRequirePasswordChange)
	}
	return nil, fmt.Errorf("auth service not configured")
}
