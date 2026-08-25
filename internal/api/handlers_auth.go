package api

import (
	"context"
	"encoding/json"
	"net/http"
	"strconv"
)

// handleAuthProviders reports which OAuth providers are configured, so
// web/login.html can disable a button instead of sending the user into a
// dead OAuth flow.
func (s *Server) handleAuthProviders(w http.ResponseWriter, r *http.Request) {
	// Login routes are only ever registered when Store != nil (see
	// server.go) — an OAuth provider needs somewhere to persist the user.
	writeJSON(w, http.StatusOK, map[string]bool{
		"google": s.Store != nil && s.GoogleOAuth.Enabled(),
		"github": s.Store != nil && s.GitHubOAuth.Enabled(),
	})
}

// handleGoogleLogin redirects to Google's consent screen. Only registered
// when GoogleOAuth.Enabled() (see internal/api/server.go).
func (s *Server) handleGoogleLogin(w http.ResponseWriter, r *http.Request) {
	beginOAuth(w, r, s.googleConfig())
}

type googleUserInfo struct {
	Sub   string `json:"sub"`
	Email string `json:"email"`
}

// handleGoogleCallback exchanges the auth code, fetches the user's email
// from Google's userinfo endpoint, upserts the account, and redirects into
// the app with a session token.
func (s *Server) handleGoogleCallback(w http.ResponseWriter, r *http.Request) {
	if !checkOAuthState(w, r) {
		http.Error(w, "invalid oauth state", http.StatusBadRequest)
		return
	}

	ctx, cancel := context.WithTimeout(r.Context(), httpClientTimeout)
	defer cancel()

	cfg := s.googleConfig()
	token, err := cfg.Exchange(ctx, r.URL.Query().Get("code"))
	if err != nil {
		http.Error(w, "exchanging google oauth code: "+err.Error(), http.StatusBadGateway)
		return
	}

	resp, err := cfg.Client(ctx, token).Get("https://www.googleapis.com/oauth2/v3/userinfo")
	if err != nil {
		http.Error(w, "fetching google user info: "+err.Error(), http.StatusBadGateway)
		return
	}
	defer resp.Body.Close()

	var info googleUserInfo
	if err := json.NewDecoder(resp.Body).Decode(&info); err != nil || info.Sub == "" || info.Email == "" {
		http.Error(w, "invalid response from google", http.StatusBadGateway)
		return
	}

	user, err := s.Store.UpsertGoogleUser(ctx, info.Sub, info.Email)
	if err != nil {
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}
	finishOAuthLogin(w, r, s.Tokens, user.ID)
}

// handleGitHubLogin redirects to GitHub's consent screen. Only registered
// when GitHubOAuth.Enabled().
func (s *Server) handleGitHubLogin(w http.ResponseWriter, r *http.Request) {
	beginOAuth(w, r, s.githubConfig())
}

type githubUser struct {
	ID    int64  `json:"id"`
	Email string `json:"email"`
}

type githubEmail struct {
	Email    string `json:"email"`
	Primary  bool   `json:"primary"`
	Verified bool   `json:"verified"`
}

// handleGitHubCallback exchanges the auth code, fetches the user's id and
// primary verified email, upserts the account, and redirects into the app
// with a session token. GitHub's /user endpoint often omits email (private
// by default), so /user/emails is checked as a fallback.
func (s *Server) handleGitHubCallback(w http.ResponseWriter, r *http.Request) {
	if !checkOAuthState(w, r) {
		http.Error(w, "invalid oauth state", http.StatusBadRequest)
		return
	}

	ctx, cancel := context.WithTimeout(r.Context(), httpClientTimeout)
	defer cancel()

	cfg := s.githubConfig()
	token, err := cfg.Exchange(ctx, r.URL.Query().Get("code"))
	if err != nil {
		http.Error(w, "exchanging github oauth code: "+err.Error(), http.StatusBadGateway)
		return
	}
	client := cfg.Client(ctx, token)

	var user githubUser
	if resp, err := client.Get("https://api.github.com/user"); err == nil {
		defer resp.Body.Close()
		_ = json.NewDecoder(resp.Body).Decode(&user)
	}
	if user.ID == 0 {
		http.Error(w, "invalid response from github", http.StatusBadGateway)
		return
	}

	email := user.Email
	if email == "" {
		if resp, err := client.Get("https://api.github.com/user/emails"); err == nil {
			defer resp.Body.Close()
			var emails []githubEmail
			_ = json.NewDecoder(resp.Body).Decode(&emails)
			for _, e := range emails {
				if e.Primary && e.Verified {
					email = e.Email
					break
				}
			}
		}
	}
	if email == "" {
		http.Error(w, "github account has no verified email available (check github.com/settings/emails)", http.StatusBadRequest)
		return
	}

	storedUser, err := s.Store.UpsertGitHubUser(ctx, strconv.FormatInt(user.ID, 10), email)
	if err != nil {
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}
	finishOAuthLogin(w, r, s.Tokens, storedUser.ID)
}
