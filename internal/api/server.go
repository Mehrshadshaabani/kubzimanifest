// Package api wires parser/rules/cost into an HTTP API (chi router), plus
// Postgres-backed auth, saved reports, and a billing surface backed by
// whatever internal/billing.Provider is configured (NoopProvider by
// default in this build).
package api

import (
	"net/http"
	"path/filepath"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"

	"mflint/internal/auth"
	"mflint/internal/billing"
	"mflint/internal/store"
)

// Server holds every dependency the API needs. Store is optional: when nil,
// /v1/lint still works (it's stateless), but auth/reports/billing routes
// are not registered at all, since they have nothing to persist to.
type Server struct {
	Store       *store.Store
	Tokens      *auth.TokenIssuer
	Billing     billing.Provider
	GoogleOAuth auth.OAuthConfig
	GitHubOAuth auth.OAuthConfig
	WebDir      string // directory containing landing.html and index.html
}

func (s *Server) Router() http.Handler {
	r := chi.NewRouter()
	r.Use(middleware.Logger)
	r.Use(middleware.Recoverer)
	r.Use(middleware.Timeout(15 * time.Second))

	r.Route("/v1", func(v1 chi.Router) {
		v1.With(lintRateLimiter()).Post("/lint", s.handleLint)
		v1.Get("/auth/providers", s.handleAuthProviders)

		if s.Store != nil {
			// Authenticates via the provider's own HMAC signature, not a
			// bearer token, so it stays outside the requireAuth group.
			v1.Post("/billing/webhook/nowpayments", s.handleNOWPaymentsWebhook)

			v1.Group(func(protected chi.Router) {
				protected.Use(s.requireAuth)
				protected.Get("/reports", s.handleListReports)
				protected.Get("/billing/plan", s.handleGetPlan)
				protected.Post("/billing/checkout", s.handleCheckout)
				protected.Get("/usage", s.handleGetUsage)
				protected.Get("/api-keys", s.handleListAPIKeys)
				protected.Post("/api-keys", s.handleCreateAPIKey)
				protected.Delete("/api-keys/{id}", s.handleDeleteAPIKey)
			})

			if s.GoogleOAuth.Enabled() {
				v1.Get("/auth/google/login", s.handleGoogleLogin)
				v1.Get("/auth/google/callback", s.handleGoogleCallback)
			}
			if s.GitHubOAuth.Enabled() {
				v1.Get("/auth/github/login", s.handleGitHubLogin)
				v1.Get("/auth/github/callback", s.handleGitHubCallback)
			}
		}
	})

	if s.WebDir != "" {
		r.Get("/", s.serveWebFile("landing.html"))
		r.Get("/app", s.serveWebFile("index.html"))
		r.Get("/app/", s.serveWebFile("index.html"))
		r.Get("/login", s.serveWebFile("login.html"))
		r.Get("/docs", s.serveWebFile("docs.html"))
	}

	return r
}

func (s *Server) serveWebFile(name string) http.HandlerFunc {
	path := filepath.Join(s.WebDir, name)
	return func(w http.ResponseWriter, r *http.Request) {
		http.ServeFile(w, r, path)
	}
}
