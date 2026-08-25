// Command server runs the mflint HTTP API and serves the paste-YAML web UI.
// Configuration is via environment variables:
//
//	PORT               listen port (default 8080)
//	DATABASE_URL       Postgres DSN, e.g. postgres://user:pass@localhost:5432/mflint?sslmode=disable
//	                    If unset, the server still runs with /v1/lint working;
//	                    auth/reports/billing routes are not registered.
//	JWT_SECRET          HMAC secret for signing session tokens (required if DATABASE_URL is set)
//	WEB_DIR             directory to serve at "/" (default "web")
//	GOOGLE_OAUTH_CLIENT_ID / GOOGLE_OAUTH_CLIENT_SECRET / GOOGLE_OAUTH_REDIRECT_URL
//	GITHUB_OAUTH_CLIENT_ID / GITHUB_OAUTH_CLIENT_SECRET / GITHUB_OAUTH_REDIRECT_URL
//	                    sign-in is OAuth-only (no password auth); a provider's /login and
//	                    /callback routes are only registered once both its client id and
//	                    secret are set. REDIRECT_URL must exactly match what's registered
//	                    with that provider, e.g. https://your-domain/v1/auth/google/callback
//	NOWPAYMENTS_API_KEY / NOWPAYMENTS_IPN_SECRET  crypto billing via NOWPayments; unset falls
//	                    back to NoopProvider (checkout 501s). Get these from your account at
//	                    account-sandbox.nowpayments.io (testing, no real funds) or
//	                    account.nowpayments.io (production), Store Settings tab.
//	NOWPAYMENTS_API_BASE     defaults to the sandbox API (https://api-sandbox.nowpayments.io)
//	NOWPAYMENTS_CALLBACK_URL public URL of this server's /v1/billing/webhook/nowpayments route
package main

import (
	"context"
	"log"
	"net/http"
	"os"
	"time"

	"mflint/internal/api"
	"mflint/internal/auth"
	"mflint/internal/billing"
	"mflint/internal/store"
)

func main() {
	port := getenv("PORT", "8080")
	webDir := getenv("WEB_DIR", "web")

	srv := &api.Server{
		Billing:     billing.Provider(billing.NoopProvider{}),
		GoogleOAuth: auth.GoogleOAuthConfigFromEnv(),
		GitHubOAuth: auth.GitHubOAuthConfigFromEnv(),
		WebDir:      webDir,
	}
	if srv.GoogleOAuth.Enabled() {
		log.Println("Google OAuth configured: sign-in with Google is live")
	} else {
		log.Println("GOOGLE_OAUTH_CLIENT_ID/SECRET not set: sign-in with Google disabled")
	}
	if srv.GitHubOAuth.Enabled() {
		log.Println("GitHub OAuth configured: sign-in with GitHub is live")
	} else {
		log.Println("GITHUB_OAUTH_CLIENT_ID/SECRET not set: sign-in with GitHub disabled")
	}

	if apiKey := os.Getenv("NOWPAYMENTS_API_KEY"); apiKey != "" {
		srv.Billing = billing.NewNOWPaymentsProvider(
			apiKey,
			os.Getenv("NOWPAYMENTS_IPN_SECRET"),
			os.Getenv("NOWPAYMENTS_API_BASE"),
			os.Getenv("NOWPAYMENTS_CALLBACK_URL"),
		)
		log.Println("NOWPayments configured: billing checkout is live")
	} else {
		log.Println("NOWPAYMENTS_API_KEY not set: billing checkout will 501 (NoopProvider)")
	}

	if dsn := os.Getenv("DATABASE_URL"); dsn != "" {
		db, err := store.Open(dsn)
		if err != nil {
			log.Fatalf("connecting to Postgres: %v", err)
		}
		defer db.Close()

		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		if err := db.Migrate(ctx); err != nil {
			cancel()
			log.Fatalf("running migrations: %v", err)
		}
		cancel()

		secret := os.Getenv("JWT_SECRET")
		if secret == "" {
			log.Fatal("JWT_SECRET must be set when DATABASE_URL is set (auth needs it to sign session tokens)")
		}

		srv.Store = db
		srv.Tokens = auth.NewTokenIssuer(secret, 30*24*time.Hour)
		log.Println("Postgres connected, migrations applied; auth/reports/billing routes enabled")
	} else {
		log.Println("DATABASE_URL not set: running with /v1/lint only (no auth, reports, or billing)")
	}

	addr := ":" + port
	log.Printf("mflint server listening on %s (web UI at %s)", addr, webDir)
	if err := http.ListenAndServe(addr, srv.Router()); err != nil {
		log.Fatal(err)
	}
}

func getenv(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}
