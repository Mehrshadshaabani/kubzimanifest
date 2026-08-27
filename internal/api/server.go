// Package api wires parser/rules/cost into an HTTP API (chi router), plus
// Postgres-backed auth, saved reports, and a billing surface backed by
// whatever internal/billing.Provider is configured (NoopProvider by
// default in this build).
package api

import (
	"net/http"
	"path/filepath"
	"regexp"
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
	// AdminEmails are the sign-in emails allowed to use /v1/admin/* and the
	// /admin page (see requireAdmin in middleware.go). There's no separate
	// admin account type — an admin is just a regular OAuth user whose
	// email is in this list.
	AdminEmails []string
}

func (s *Server) Router() http.Handler {
	r := chi.NewRouter()
	r.Use(middleware.Logger)
	r.Use(middleware.Recoverer)
	r.Use(middleware.Timeout(15 * time.Second))

	r.Route("/v1", func(v1 chi.Router) {
		v1.With(lintRateLimiter()).Post("/lint", s.handleLint)
		v1.Get("/auth/providers", s.handleAuthProviders)
		v1.Get("/services", s.handleListServices)
		v1.Post("/consultations", s.handleCreateConsultation)

		if s.Store != nil {
			// Authenticate via the provider's own mechanism, not a bearer
			// token, so these stay outside the requireAuth group.
			v1.Post("/billing/webhook/nowpayments", s.handleNOWPaymentsWebhook)
			v1.Post("/billing/webhook/coingate", s.handleCoinGateWebhook)

			v1.Group(func(protected chi.Router) {
				protected.Use(s.requireAuth)
				protected.Get("/reports", s.handleListReports)
				protected.Get("/billing/plan", s.handleGetPlan)
				protected.Post("/billing/checkout", s.handleCheckout)
				protected.Get("/usage", s.handleGetUsage)
				protected.Get("/api-keys", s.handleListAPIKeys)
				protected.Post("/api-keys", s.handleCreateAPIKey)
				protected.Delete("/api-keys/{id}", s.handleDeleteAPIKey)
				protected.Post("/services/orders", s.handleCreateServiceOrder)
				protected.Get("/services/orders", s.handleListServiceOrders)

				protected.Group(func(admin chi.Router) {
					admin.Use(s.requireAdmin)
					admin.Get("/admin/whoami", s.handleAdminWhoAmI)
					admin.Get("/admin/service-orders", s.handleAdminListServiceOrders)
					admin.Post("/admin/service-orders/{id}/status", s.handleAdminUpdateServiceOrderStatus)
					admin.Get("/admin/consultations", s.handleAdminListConsultations)
				})
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
		r.Get("/privacy", s.serveWebFile("privacy.html"))
		r.Get("/services", s.serveWebFile("services.html"))
		r.Get("/consulting", s.serveWebFile("consulting.html"))
		r.Get("/admin", s.serveWebFile("admin.html"))
		r.Get("/blog", s.serveWebFile("blog.html"))
		// Registered before "/blog/{slug}" — chi matches this literal path
		// prefix ahead of the param route below for the "images" segment,
		// so /blog/images/* doesn't get swallowed as a slug lookup.
		r.Handle("/blog/images/*", http.StripPrefix("/blog/images/", http.FileServer(http.Dir(filepath.Join(s.WebDir, "blog", "images")))))
		r.Get("/blog/{slug}", s.serveBlogPost)
		r.Handle("/images/*", http.StripPrefix("/images/", http.FileServer(http.Dir(filepath.Join(s.WebDir, "images")))))
		r.Get("/robots.txt", s.serveWebFile("robots.txt"))
		r.Get("/sitemap.xml", s.serveWebFile("sitemap.xml"))
	}

	return r
}

func (s *Server) serveWebFile(name string) http.HandlerFunc {
	path := filepath.Join(s.WebDir, name)
	return func(w http.ResponseWriter, r *http.Request) {
		http.ServeFile(w, r, path)
	}
}

// blogSlugPattern only allows lowercase letters, digits, and hyphens — the
// slug feeds directly into a filesystem path, so this also rules out "..",
// "/", and anything else that could escape web/blog/.
var blogSlugPattern = regexp.MustCompile(`^[a-z0-9-]+$`)

func (s *Server) serveBlogPost(w http.ResponseWriter, r *http.Request) {
	slug := chi.URLParam(r, "slug")
	if !blogSlugPattern.MatchString(slug) {
		http.NotFound(w, r)
		return
	}
	http.ServeFile(w, r, filepath.Join(s.WebDir, "blog", slug+".html"))
}
