package web

import (
	"encoding/json"
	"fmt"
	"net/http"
	"runtime"
	"time"

	jwt "github.com/golang-jwt/jwt/v4"
	"github.com/gtsteffaniak/go-logger/logger"
	"github.com/junkerderprovinz/cannoncommander/backend/internal/auth"
	"github.com/junkerderprovinz/cannoncommander/backend/internal/database/users"
	"github.com/junkerderprovinz/cannoncommander/backend/internal/errors"
	"github.com/junkerderprovinz/cannoncommander/backend/internal/state"
	"github.com/junkerderprovinz/cannoncommander/backend/internal/utils"
	"github.com/junkerderprovinz/cannoncommander/backend/pkg/settings"
)

type requestContext = Context

type handleFunc = HandleFunc

// resolveBearerTokenUser maps a validated bearer JWT to its owner via the
// hashed_tokens registry. Session tokens get full owner permissions; named API
// tokens must have stored metadata, otherwise the request is rejected rather
// than granted the owner's uncapped permissions.
func resolveBearerTokenUser(rawToken string) (*users.User, error) {
	ownerID, isSession, ok := state.HashedTokenOwner(rawToken)
	if !ok {
		logger.Debugf("auth rejected: no hashed token mapping (hash=%s)", utils.HashSHA256(rawToken)[:8])
		return nil, fmt.Errorf("token is invalid or revoked")
	}
	userValue, err := state.GetUserByID(ownerID)
	if err != nil {
		logger.Debugf("auth rejected: token owner %d not found (hash=%s)", ownerID, utils.HashSHA256(rawToken)[:8])
		return nil, err
	}
	if isSession {
		return &userValue, nil
	}
	tokenName, ok := state.TokenNameForRawToken(&userValue, rawToken)
	if !ok {
		logger.Debugf("auth rejected: token has no permission metadata for user %s (hash=%s)", userValue.Username, utils.HashSHA256(rawToken)[:8])
		return nil, fmt.Errorf("token has no permission metadata")
	}
	applyNamedApiTokenGlobalCaps(&userValue, tokenName)
	if userValue.Tokens[tokenName].ReadOnly {
		userValue.RestrictToReading()
	}
	return &userValue, nil
}

// Middleware to retrieve and authenticate user
func withUserHelper(fn handleFunc) handleFunc {
	return func(w http.ResponseWriter, r *http.Request, data *requestContext) (int, error) {
		if settings.Config.Auth.Methods.NoAuth {
			userValue, err := state.ResolveNoAuthUser()
			if err == nil {
				data.User = &userValue
			}
			if err != nil {
				logger.Errorf("no auth: %v", err)
				return http.StatusInternalServerError, err
			}
			if fn == nil {
				return http.StatusOK, nil
			}
			return fn(w, r, data)
		}

		keyFunc := auth.JWTSigningKeyFunc()
		if data.Token == "" {
			var err error
			data.Token, err = ExtractToken(r)
			if err != nil {
				return http.StatusUnauthorized, err
			}
		}

		var tk users.AuthToken
		token, err := jwt.ParseWithClaims(data.Token, &tk, keyFunc)
		if err != nil {
			// JWT library automatically validates expiration - if expired, it returns an error
			return http.StatusUnauthorized, fmt.Errorf("invalid token: %v", err)
		}
		if !token.Valid {
			return http.StatusUnauthorized, fmt.Errorf("invalid token")
		}
		if state.IsTokenRevoked(data.Token) {
			return http.StatusUnauthorized, fmt.Errorf("token is expired or revoked")
		}
		// ExpiresAt should always be set in valid tokens created by our system
		// JWT library populates RegisteredClaims.ExpiresAt
		if tk.RegisteredClaims.ExpiresAt == nil {
			return http.StatusUnauthorized, fmt.Errorf("token is invalid or revoked")
		}
		userValue, err := resolveBearerTokenUser(data.Token)
		if err != nil {
			logger.Errorf("Failed to get user from token: %v", err)
			return http.StatusUnauthorized, fmt.Errorf("token is invalid or revoked")
		}
		data.User = userValue

		// Plant the session cookie only when the request didn't already carry
		// one. Some clients like gvfs rely on it for concurrent uploads, but a
		// request that presented a stale cookie must not re-emit it or it could
		// revert the jar to a token retired by a concurrent renew.
		if tk.RegisteredClaims.ExpiresAt != nil {
			if c, err := r.Cookie(sessionCookieName); err != nil || c.Value == "" || c.Value != data.Token {
				SetSessionCookie(w, r, data.Token, tk.RegisteredClaims.ExpiresAt.Time)
			}
		}
		SetUserInResponseWriter(w, data.User)
		if data.User.Username == "" {
			return http.StatusForbidden, errors.ErrUnauthorized
		}
		// Call the handler function, passing in the context (or return OK if no handler)
		if fn == nil {
			return http.StatusOK, nil
		}
		return fn(w, r, data)
	}
}

func wrapHandler(fn handleFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		data := &requestContext{
			Ctx: r.Context(),
		}

		// Call the actual handler function and get status code and error
		status, err := fn(w, r, data)
		// Handle the error case if there is one
		if err != nil {
			// Create an error response in JSON format
			response := &HttpResponse{
				Status:  status, // Use the status code from the middleware
				Message: err.Error(),
			}

			// Set the content type to JSON and status code
			w.Header().Set("Content-Type", "application/json; charset=utf-8")
			w.WriteHeader(status)

			// Marshal the error response to JSON
			errorBytes, marshalErr := json.Marshal(response)
			if marshalErr != nil {
				logger.Errorf("Error marshalling error response: %v", marshalErr)
				http.Error(w, "Internal Server Error", http.StatusInternalServerError)
				return
			}

			// Write the JSON error response
			if _, writeErr := w.Write(errorBytes); writeErr != nil {
				logger.Debugf("Error writing error response: %v", writeErr)
			}
			return
		}

		// No error, proceed to write status if non-zero
		if status != 0 {
			w.WriteHeader(status)
		}
	}
}

func muxWithMiddleware(mux *http.ServeMux) *http.ServeMux {
	wrappedMux := http.NewServeMux()
	wrappedMux.Handle("/", LoggingMiddleware(mux))
	return wrappedMux
}
func LoggingMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// DEFER RECOVERY FUNCTION
		defer func() {
			if rcv := recover(); rcv != nil {
				method := r.Method
				url := r.URL.String()
				username := "unknown" // Default username

				// Attempt to get username from ResponseWriterWrapper if it's set
				if ww, ok := w.(*ResponseWriterWrapper); ok && ww.User != "" {
					username = ww.User
				}
				// Get Go-level stack trace
				buf := make([]byte, 16384)     // Increased buffer size for potentially long CGo traces
				n := runtime.Stack(buf, false) // false for current goroutine only
				stackTrace := string(buf[:n])

				logger.Errorf("PANIC RECOVERED: %v\nUser: %s\nMethod: %s\nURL: %s\nRemoteAddr: %s\nGo Stack Trace:\n%s",
					rcv, username, method, url, GetRemoteIP(r), stackTrace)

				// Attempt to send a 500 error response to the client
				// This is a best-effort; the connection might be broken or process too unstable.
				if ww, ok := w.(*ResponseWriterWrapper); ok {
					if !ww.WroteHeader {
						ww.Header().Set("Content-Type", "application/json; charset=utf-8")
						ww.WriteHeader(http.StatusInternalServerError)
					}
				} else {
					_, _ = RenderJSON(w, r, &HttpResponse{
						Status:  500,
						Message: "A critical internal error occurred. Please try again later.",
					}, http.StatusInternalServerError)
				}

			}
		}()

		start := time.Now()
		wrappedWriter := &ResponseWriterWrapper{ResponseWriter: w, StatusCode: http.StatusOK}

		// Call the next handler in the chain
		next.ServeHTTP(wrappedWriter, r)

		// Existing logging logic for normal requests
		fullURL := r.URL.Path
		if r.URL.RawQuery != "" {
			fullURL += "?" + r.URL.RawQuery
		}
		truncUser := wrappedWriter.User
		if truncUser == "" {
			truncUser = "N/A" // Handle case where user might not be set (e.g., if panic occurred before user auth)
		} else if len(truncUser) > 12 {
			truncUser = truncUser[:10] + ".."
		}
		duration := time.Since(start)

		// ApiPathExclude is applied per logging sink inside go-logger (logger.ApiPath).
		logger.ApiPath(wrappedWriter.StatusCode, fullURL,
			fmt.Sprintf("%-7s | %3d | %-15s | %-12s | %-12s | \"%s\"",
				r.Method,
				wrappedWriter.StatusCode,
				GetRemoteIP(r),
				truncUser,
				fmt.Sprintf("%vms", duration.Milliseconds()),
				fullURL))
	})
}
