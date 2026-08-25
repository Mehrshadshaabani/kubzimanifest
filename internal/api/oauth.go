package api

import (
	"crypto/rand"
	"encoding/hex"
	"net/http"
	"time"

	"golang.org/x/oauth2"

	"mflint/internal/auth"
)

var googleEndpoint = oauth2.Endpoint{
	AuthURL:  "https://accounts.google.com/o/oauth2/v2/auth",
	TokenURL: "https://oauth2.googleapis.com/token",
}

var githubEndpoint = oauth2.Endpoint{
	AuthURL:  "https://github.com/login/oauth/authorize",
	TokenURL: "https://github.com/login/oauth/access_token",
}

func oauth2Config(cfg auth.OAuthConfig, endpoint oauth2.Endpoint, scopes []string) *oauth2.Config {
	return &oauth2.Config{
		ClientID:     cfg.ClientID,
		ClientSecret: cfg.ClientSecret,
		RedirectURL:  cfg.RedirectURL,
		Endpoint:     endpoint,
		Scopes:       scopes,
	}
}

func (s *Server) googleConfig() *oauth2.Config {
	return oauth2Config(s.GoogleOAuth, googleEndpoint, []string{"openid", "email"})
}

func (s *Server) githubConfig() *oauth2.Config {
	return oauth2Config(s.GitHubOAuth, githubEndpoint, []string{"read:user", "user:email"})
}

const oauthStateCookie = "mflint_oauth_state"

// beginOAuth stores a random CSRF state in a short-lived cookie and
// redirects to the provider's consent screen with the same value, so the
// callback can confirm the request round-tripped through the same browser.
func beginOAuth(w http.ResponseWriter, r *http.Request, cfg *oauth2.Config) {
	state := randomHex(16)
	http.SetCookie(w, &http.Cookie{
		Name:     oauthStateCookie,
		Value:    state,
		Path:     "/",
		HttpOnly: true,
		MaxAge:   600,
		SameSite: http.SameSiteLaxMode,
	})
	http.Redirect(w, r, cfg.AuthCodeURL(state, oauth2.AccessTypeOnline), http.StatusFound)
}

// checkOAuthState verifies the callback's ?state= against the cookie set by
// beginOAuth, and clears the cookie either way.
func checkOAuthState(w http.ResponseWriter, r *http.Request) bool {
	http.SetCookie(w, &http.Cookie{Name: oauthStateCookie, Value: "", Path: "/", MaxAge: -1})
	cookie, err := r.Cookie(oauthStateCookie)
	if err != nil || cookie.Value == "" {
		return false
	}
	return r.URL.Query().Get("state") == cookie.Value
}

func randomHex(n int) string {
	buf := make([]byte, n)
	_, _ = rand.Read(buf)
	return hex.EncodeToString(buf)
}

// finishOAuthLogin issues a session token for userID and redirects the
// browser back into the tool with it. web/index.html picks up ?token=...,
// stores it, and strips it from the URL.
func finishOAuthLogin(w http.ResponseWriter, r *http.Request, tokens *auth.TokenIssuer, userID int64) {
	token, err := tokens.Issue(userID)
	if err != nil {
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}
	http.Redirect(w, r, "/app?token="+token, http.StatusFound)
}

var httpClientTimeout = 15 * time.Second
