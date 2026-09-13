package auth

import (
	"crypto/subtle"
	"errors"
	"net/http"
	"strings"

	"github.com/go-chi/chi/v5/middleware"
	"go.uber.org/zap"
)

var ErrNoAPIKey = errors.New("API_KEY is required")

// sessionIDHeader is the MCP streamable-HTTP header that carries a session identifier on follow-up requests. Once a
// session is established by an authenticated request, subsequent requests address it via this header rather than by
// re-sending credentials.
const sessionIDHeader = "Mcp-Session-Id"

// hasSession reports whether the request carries an MCP session identifier to prevent re-authentication.
func hasSession(r *http.Request) bool {
	return strings.TrimSpace(r.Header.Get(sessionIDHeader)) != ""
}

type AuthMode int

const (
	AuthBearer AuthMode = iota
	AuthBasic
)

func (m AuthMode) middleware(log *zap.Logger, apiKey string) func(http.Handler) http.Handler {
	switch m {
	case AuthBasic:
		return basicAuth(log, apiKey)
	default:
		return bearerAuth(log, apiKey)
	}
}

func bearerAuth(log *zap.Logger, apiKey string) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// Follow-up requests carry an MCP session ID and are already authenticated by the session-establishing request.
			if hasSession(r) {
				next.ServeHTTP(w, r)
				return
			}

			token, reason, ok := bearerToken(r)
			if !ok || subtle.ConstantTimeCompare([]byte(token), []byte(apiKey)) != 1 {
				if ok {
					reason = "invalid token"
				}
				authFailed(log, r, reason)
				unauthorized(w, `Bearer realm="mcp"`)
				return
			}

			next.ServeHTTP(w, r)
		})
	}
}

func basicAuth(log *zap.Logger, apiKey string) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if hasSession(r) {
				next.ServeHTTP(w, r)

				return
			}

			user, pass, ok := r.BasicAuth()
			if !ok {
				authFailed(log, r, "missing basic auth credentials")
				unauthorized(w, `Basic realm="mcp"`)
				return
			}

			// The API key doubles as both the username and the password.

			if subtle.ConstantTimeCompare([]byte(user), []byte(apiKey)) != 1 ||
				subtle.ConstantTimeCompare([]byte(pass), []byte(apiKey)) != 1 {
				authFailed(log, r, "invalid basic auth credentials")
				unauthorized(w, `Basic realm="mcp"`)
				return
			}

			next.ServeHTTP(w, r)
		})
	}
}

func Middleware(mode AuthMode, log *zap.Logger, apiKey string) func(http.Handler) http.Handler {
	return mode.middleware(log, apiKey)
}

func bearerToken(r *http.Request) (token, reason string, ok bool) {
	header := r.Header.Get("Authorization")
	if header == "" {
		return "", "missing authorization header", false
	}

	const prefix = "Bearer "
	if !strings.HasPrefix(header, prefix) {
		return "", "unsupported authorization scheme", false
	}

	token = strings.TrimSpace(header[len(prefix):])
	if token == "" {
		return "", "empty token", false
	}

	return token, "", true
}

func authFailed(log *zap.Logger, r *http.Request, reason string) {
	log.Warn("authentication failed",
		zap.String("reason", reason),
		zap.String("remote_addr", r.RemoteAddr),
		zap.String("request_id", middleware.GetReqID(r.Context())),
	)
}

func unauthorized(w http.ResponseWriter, challenge string) {
	w.Header().Set("WWW-Authenticate", challenge)
	w.WriteHeader(http.StatusUnauthorized)
	_, _ = w.Write([]byte("unauthorized"))
}
