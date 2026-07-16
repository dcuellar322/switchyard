package httpapi

import (
	"context"
	"errors"
	"net/http"
	"net/url"
	"strings"

	session "switchyard.dev/switchyard/internal/session/application"
)

const (
	sessionCookieName = "switchyard_session"
	csrfHeader        = "X-CSRF-Token"
	idempotencyHeader = "Idempotency-Key"
)

type accessKind string

const (
	accessBrowser accessKind = "browser"
	accessIPC     accessKind = "ipc"
)

type securityContextKey struct{}

type requestIdentity struct {
	Access  accessKind
	ActorID string
}

func withAccess(access accessKind, next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		identity := requestIdentity{Access: access, ActorID: string(access)}
		next.ServeHTTP(w, r.WithContext(context.WithValue(r.Context(), securityContextKey{}, identity)))
	})
}

func withBrowserHeaders(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Security-Policy", "default-src 'self'; connect-src 'self' ws://127.0.0.1:* ws://localhost:* ws://[::1]:*; img-src 'self' data:; style-src 'self' 'unsafe-inline'; frame-ancestors 'none'; base-uri 'none'; form-action 'none'")
		w.Header().Set("Referrer-Policy", "no-referrer")
		w.Header().Set("X-Content-Type-Options", "nosniff")
		w.Header().Set("X-Frame-Options", "DENY")
		next.ServeHTTP(w, r)
	})
}

func identityFrom(ctx context.Context) requestIdentity {
	identity, _ := ctx.Value(securityContextKey{}).(requestIdentity)
	return identity
}

func withBrowserSecurity(sessions sessionService, next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/api/v1/auth/sessions" && r.Method == http.MethodPost {
			next.ServeHTTP(w, r)
			return
		}
		if !strings.HasPrefix(r.URL.Path, "/api/v1/") && r.URL.Path != "/ws/v1/events" {
			next.ServeHTTP(w, r)
			return
		}
		cookie, err := r.Cookie(sessionCookieName)
		if err != nil {
			writeProblem(w, r, http.StatusUnauthorized, "SESSION_REQUIRED", "Browser session required", "Launch the UI through switchyard ui.")
			return
		}
		active, err := sessions.ValidateSession(cookie.Value)
		if err != nil {
			writeProblem(w, r, http.StatusUnauthorized, "SESSION_INVALID", "Browser session invalid", "Launch the UI again to create a fresh session.")
			return
		}
		if r.URL.Path == "/ws/v1/events" && !sameOrigin(r) {
			writeProblem(w, r, http.StatusForbidden, "ORIGIN_INVALID", "WebSocket origin rejected", "The event stream is same-origin only.")
			return
		}
		if isMutation(r.Method) {
			if _, err := sessions.ValidateMutation(cookie.Value, r.Header.Get(csrfHeader)); err != nil {
				code := "CSRF_INVALID"
				if !errors.Is(err, session.ErrInvalidCSRF) {
					code = "SESSION_INVALID"
				}
				writeProblem(w, r, http.StatusForbidden, code, "Mutation authorization failed", "A valid session and CSRF token are required.")
				return
			}
		}
		identity := identityFrom(r.Context())
		identity.ActorID = active.ID
		next.ServeHTTP(w, r.WithContext(context.WithValue(r.Context(), securityContextKey{}, identity)))
	})
}

func withIdempotencyKey(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if isMutation(r.Method) && !strings.HasPrefix(r.URL.Path, "/api/v1/auth/") {
			key := r.Header.Get(idempotencyHeader)
			if len(key) < 8 || len(key) > 128 {
				writeProblem(w, r, http.StatusBadRequest, "IDEMPOTENCY_KEY_INVALID", "Idempotency key invalid", "Use an opaque key between 8 and 128 characters.")
				return
			}
		}
		next.ServeHTTP(w, r)
	})
}

func isMutation(method string) bool {
	return method != http.MethodGet && method != http.MethodHead && method != http.MethodOptions
}

func sameOrigin(r *http.Request) bool {
	origin, err := url.Parse(r.Header.Get("Origin"))
	if err != nil || origin.Scheme == "" || origin.Host == "" {
		return false
	}
	return origin.Scheme == "http" && origin.Host == r.Host
}
