package config

import "time"

type OAuthConfig struct {
	Google       OAuthProviderConfig
	Microsoft    OAuthProviderConfig
	StateManager StateManagerConfig
}

type OAuthProviderConfig struct {
	Enabled      bool
	ClientID     string
	ClientSecret string
	RedirectURL  string
	Scopes       []string
	AuthURL      string
	TokenURL     string
	UserInfoURL  string
	Timeout      time.Duration
}

type StateManagerConfig struct {
	Type string
	TTL  time.Duration
}

func loadOAuthConfig() OAuthConfig {
	return OAuthConfig{
		Google: OAuthProviderConfig{
			Enabled:      getEnvBool("OAUTH_GOOGLE_ENABLED", false),
			ClientID:     getEnv("OAUTH_GOOGLE_CLIENT_ID", ""),
			ClientSecret: getEnv("OAUTH_GOOGLE_CLIENT_SECRET", ""),
			RedirectURL:  getEnv("OAUTH_GOOGLE_REDIRECT_URL", ""),
			Scopes:       getEnvStringSlice("OAUTH_GOOGLE_SCOPES", []string{"openid", "email", "profile"}),
			AuthURL:      getEnv("OAUTH_GOOGLE_AUTH_URL", "https://accounts.google.com/o/oauth2/auth"),
			TokenURL:     getEnv("OAUTH_GOOGLE_TOKEN_URL", "https://oauth2.googleapis.com/token"),
			UserInfoURL:  getEnv("OAUTH_GOOGLE_USER_INFO_URL", "https://www.googleapis.com/oauth2/v2/userinfo"),
			Timeout:      getEnvDuration("OAUTH_GOOGLE_TIMEOUT", 30*time.Second),
		},
		Microsoft: OAuthProviderConfig{
			Enabled:      getEnvBool("OAUTH_MICROSOFT_ENABLED", false),
			ClientID:     getEnv("OAUTH_MICROSOFT_CLIENT_ID", ""),
			ClientSecret: getEnv("OAUTH_MICROSOFT_CLIENT_SECRET", ""),
			RedirectURL:  getEnv("OAUTH_MICROSOFT_REDIRECT_URL", ""),
			Scopes:       getEnvStringSlice("OAUTH_MICROSOFT_SCOPES", []string{"openid", "email", "profile", "User.Read"}),
			AuthURL:      getEnv("OAUTH_MICROSOFT_AUTH_URL", "https://login.microsoftonline.com/common/oauth2/v2.0/authorize"),
			TokenURL:     getEnv("OAUTH_MICROSOFT_TOKEN_URL", "https://login.microsoftonline.com/common/oauth2/v2.0/token"),
			UserInfoURL:  getEnv("OAUTH_MICROSOFT_USER_INFO_URL", "https://graph.microsoft.com/v1.0/me"),
			Timeout:      getEnvDuration("OAUTH_MICROSOFT_TIMEOUT", 30*time.Second),
		},
		StateManager: StateManagerConfig{
			Type: getEnv("OAUTH_STATE_MANAGER_TYPE", "redis"),
			TTL:  getEnvDuration("OAUTH_STATE_TTL", 10*time.Minute),
		},
	}
}
