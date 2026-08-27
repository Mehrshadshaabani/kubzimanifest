package api

import (
	"context"
	"net/http"
	"strings"
	"sync"

	"golang.org/x/time/rate"
)

type ctxKey int

const userIDKey ctxKey = iota

// lintRateLimiter caps /v1/lint calls per client IP (1 req/s, burst 5).
// In-memory only — no rate-limit persistence in this build, so it resets on
// restart and isn't shared across replicas. That's an acceptable MVP
// tradeoff for a single-instance deploy.
func lintRateLimiter() func(http.Handler) http.Handler {
	var mu sync.Mutex
	limiters := map[string]*rate.Limiter{}

	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			ip := clientIP(r)

			mu.Lock()
			lim, ok := limiters[ip]
			if !ok {
				lim = rate.NewLimiter(1, 5)
				limiters[ip] = lim
			}
			mu.Unlock()

			if !lim.Allow() {
				http.Error(w, "rate limit exceeded, slow down", http.StatusTooManyRequests)
				return
			}
			next.ServeHTTP(w, r)
		})
	}
}

func clientIP(r *http.Request) string {
	if fwd := r.Header.Get("X-Forwarded-For"); fwd != "" {
		return strings.TrimSpace(strings.Split(fwd, ",")[0])
	}
	return r.RemoteAddr
}

// requireAuth rejects requests without a valid Authorization: Bearer header
// (session JWT or API key — see Server.bearerUserID) and stashes the user
// ID in the request context on success.
func (s *Server) requireAuth(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		userID, ok := s.bearerUserID(r)
		if !ok {
			http.Error(w, "missing or invalid bearer token", http.StatusUnauthorized)
			return
		}
		next.ServeHTTP(w, r.WithContext(context.WithValue(r.Context(), userIDKey, userID)))
	})
}

func userIDFromContext(ctx context.Context) (int64, bool) {
	id, ok := ctx.Value(userIDKey).(int64)
	return id, ok
}

// requireAdmin must be chained after requireAuth (it reads the user ID that
// sets in the request context). There's no separate admin role in the
// users table — it looks the signed-in user's email up and checks it
// against Server.AdminEmails, since that's the one place admin identity is
// configured (see cmd/server/main.go's ADMIN_EMAILS env var).
func (s *Server) requireAdmin(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		userID, ok := userIDFromContext(r.Context())
		if !ok {
			http.Error(w, "missing or invalid bearer token", http.StatusUnauthorized)
			return
		}
		user, err := s.Store.GetUserByID(r.Context(), userID)
		if err != nil {
			http.Error(w, "forbidden", http.StatusForbidden)
			return
		}
		if !s.isAdminEmail(user.Email) {
			http.Error(w, "forbidden", http.StatusForbidden)
			return
		}
		next.ServeHTTP(w, r)
	})
}

func (s *Server) isAdminEmail(email string) bool {
	for _, e := range s.AdminEmails {
		if strings.EqualFold(e, email) {
			return true
		}
	}
	return false
}
