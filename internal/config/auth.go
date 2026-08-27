package config

import "time"

type AuthConfig struct {
	JWT           JWTConfig
	APIKey        APIKeyConfig
	Session       SessionConfig
	OTP           OTPConfig
	Invitation    InvitationConfig
	PasswordReset PasswordResetConfig
	Cookie        CookieConfig
	Password      PasswordConfig
}

type JWTConfig struct {
	SecretKey       string
	AccessTokenTTL  time.Duration
	RefreshTokenTTL time.Duration
	Issuer          string
	Audience        []string
}

type APIKeyConfig struct {
	LivePrefix  string
	TestPrefix  string
	TokenLength int
}

type SessionConfig struct {
	ExpirationTime  time.Duration
	CleanupInterval time.Duration
	MaxSessions     int
}

type OTPConfig struct {
	CodeLength      int
	ExpirationTime  time.Duration
	MaxAttempts     int
	RateLimitWindow time.Duration
	TokenByteLength int
}

type InvitationConfig struct {
	DefaultExpirationDays int
	TokenByteLength       int
	MaxPendingPerTenant   int
}

type PasswordResetConfig struct {
	TokenByteLength      int
	ExpirationTime       time.Duration
	RateLimitWindow      time.Duration
	MaxAttemptsPerWindow int
}

type CookieConfig struct {
	AccessTokenName  string
	RefreshTokenName string
	Domain           string
	Path             string
	Secure           bool
	HTTPOnly         bool
	SameSite         string
}

type PasswordConfig struct {
	BcryptCost int
}

func loadAuthConfig() AuthConfig {
	return AuthConfig{
		JWT: JWTConfig{
			SecretKey:       getEnv("JWT_SECRET_KEY", ""),
			AccessTokenTTL:  getEnvDuration("JWT_ACCESS_TOKEN_TTL", 15*time.Minute),
			RefreshTokenTTL: getEnvDuration("JWT_REFRESH_TOKEN_TTL", 7*24*time.Hour),
			Issuer:          getEnv("JWT_ISSUER", "manifesto"),
			Audience:        getEnvStringSlice("JWT_AUDIENCE", []string{"manifesto-api"}),
		},
		APIKey: APIKeyConfig{
			LivePrefix:  getEnv("API_KEY_LIVE_PREFIX", "manifesto_live"),
			TestPrefix:  getEnv("API_KEY_TEST_PREFIX", "manifesto_test"),
			TokenLength: getEnvInt("API_KEY_TOKEN_LENGTH", 32),
		},
		Session: SessionConfig{
			ExpirationTime:  getEnvDuration("SESSION_EXPIRATION_TIME", 24*time.Hour),
			CleanupInterval: getEnvDuration("SESSION_CLEANUP_INTERVAL", 1*time.Hour),
			MaxSessions:     getEnvInt("SESSION_MAX_PER_USER", 10),
		},
		OTP: OTPConfig{
			CodeLength:      getEnvInt("OTP_CODE_LENGTH", 6),
			ExpirationTime:  getEnvDuration("OTP_EXPIRATION_TIME", 10*time.Minute),
			MaxAttempts:     getEnvInt("OTP_MAX_ATTEMPTS", 5),
			RateLimitWindow: getEnvDuration("OTP_RATE_LIMIT_WINDOW", 1*time.Minute),
			TokenByteLength: getEnvInt("OTP_TOKEN_BYTE_LENGTH", 3),
		},
		Invitation: InvitationConfig{
			DefaultExpirationDays: getEnvInt("INVITATION_DEFAULT_EXPIRATION_DAYS", 7),
			TokenByteLength:       getEnvInt("INVITATION_TOKEN_BYTE_LENGTH", 32),
			MaxPendingPerTenant:   getEnvInt("INVITATION_MAX_PENDING_PER_TENANT", 100),
		},
		PasswordReset: PasswordResetConfig{
			TokenByteLength:      getEnvInt("PASSWORD_RESET_TOKEN_BYTE_LENGTH", 32),
			ExpirationTime:       getEnvDuration("PASSWORD_RESET_EXPIRATION_TIME", 1*time.Hour),
			RateLimitWindow:      getEnvDuration("PASSWORD_RESET_RATE_LIMIT_WINDOW", 15*time.Minute),
			MaxAttemptsPerWindow: getEnvInt("PASSWORD_RESET_MAX_ATTEMPTS", 3),
		},
		Cookie: CookieConfig{
			AccessTokenName:  getEnv("COOKIE_ACCESS_TOKEN_NAME", "access_token"),
			RefreshTokenName: getEnv("COOKIE_REFRESH_TOKEN_NAME", "refresh_token"),
			Domain:           getEnv("COOKIE_DOMAIN", ""),
			Path:             getEnv("COOKIE_PATH", "/"),
			Secure:           getEnvBool("COOKIE_SECURE", false),
			HTTPOnly:         getEnvBool("COOKIE_HTTP_ONLY", true),
			SameSite:         getEnv("COOKIE_SAME_SITE", "Lax"),
		},
		Password: PasswordConfig{
			BcryptCost: getEnvInt("BCRYPT_COST", 10),
		},
	}
}

type TenantConfig struct {
	MaxUsersDefault int
}

func loadTenantConfig() TenantConfig {
	return TenantConfig{
		MaxUsersDefault: getEnvInt("TENANT_MAX_USERS_DEFAULT", 5),
	}
}
