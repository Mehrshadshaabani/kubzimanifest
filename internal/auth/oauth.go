package auth

import "os"

// OAuthConfig holds one provider's OAuth app credentials. Enabled() is
// false unless both ClientID and ClientSecret are set — internal/api only
// registers a provider's login/callback routes when its config is enabled,
// so an unconfigured provider simply doesn't appear as a sign-in option
// rather than erroring.
type OAuthConfig struct {
	ClientID     string
	ClientSecret string
	RedirectURL  string // must exactly match what's registered with the provider
}

func (c OAuthConfig) Enabled() bool {
	return c.ClientID != "" && c.ClientSecret != ""
}

func GoogleOAuthConfigFromEnv() OAuthConfig {
	return OAuthConfig{
		ClientID:     os.Getenv("GOOGLE_OAUTH_CLIENT_ID"),
		ClientSecret: os.Getenv("GOOGLE_OAUTH_CLIENT_SECRET"),
		RedirectURL:  os.Getenv("GOOGLE_OAUTH_REDIRECT_URL"),
	}
}

func GitHubOAuthConfigFromEnv() OAuthConfig {
	return OAuthConfig{
		ClientID:     os.Getenv("GITHUB_OAUTH_CLIENT_ID"),
		ClientSecret: os.Getenv("GITHUB_OAUTH_CLIENT_SECRET"),
		RedirectURL:  os.Getenv("GITHUB_OAUTH_REDIRECT_URL"),
	}
}
