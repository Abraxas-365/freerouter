package config

import (
	"errors"
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"
)

// Config holds all application configuration loaded from environment variables.
type Config struct {
	Server     Server
	Database   Database
	Redis      Redis
	Cache      Cache
	Metrics    Metrics
	IAMKit     IAMKit
	Encryption Encryption
}

// Server holds HTTP server configuration.
type Server struct {
	Port string
	Env  string // "development", "staging", "production"
}

// Database holds PostgreSQL connection configuration.
type Database struct {
	Host     string
	Port     string
	User     string
	Password string
	Name     string
	SSLMode  string
}

// DSN returns the PostgreSQL connection string.
func (d Database) DSN() string {
	return fmt.Sprintf(
		"host=%s port=%s user=%s password=%s dbname=%s sslmode=%s",
		d.Host, d.Port, d.User, d.Password, d.Name, d.SSLMode,
	)
}

// Redis holds Redis connection configuration.
type Redis struct {
	Addr     string
	Password string
	DB       int
}

// Cache holds response cache configuration.
type Cache struct {
	Enabled bool
	TTL     time.Duration
}

// Metrics holds Prometheus metrics configuration.
type Metrics struct {
	Enabled bool
}

// IAMKit holds IAMKit service configuration. The boundary IDs are trusted
// configuration: every token is checked against them, never against values
// taken from the token or the request.
type IAMKit struct {
	BaseURL        string // e.g. "http://localhost:8080"
	ServiceSecret  string // ik_svc_... backend service account on the environment's IAM resource (optional)
	EnvironmentID  string // IAMKit environment UUID
	ApplicationID  string // IAMKit application UUID
	ResourceID     string // IAMKit resource UUID
	OrganizationID string // Organization FreeRouter users and role assignments live in
	JWTIssuer      string // Token issuer URL (must match IAMKit's JWT_ISSUER)
	Audience       string // Resource audience URL
}

// Validate fails fast on a configuration that would reject every token or
// leak a privileged credential type into the backend.
func (c IAMKit) Validate() error {
	required := []struct{ name, value string }{
		{"IAMKIT_BASE_URL", c.BaseURL},
		{"IAMKIT_ENVIRONMENT_ID", c.EnvironmentID},
		{"IAMKIT_APPLICATION_ID", c.ApplicationID},
		{"IAMKIT_RESOURCE_ID", c.ResourceID},
		{"IAMKIT_JWT_ISSUER", c.JWTIssuer},
		{"IAMKIT_AUDIENCE", c.Audience},
	}
	var missing []string
	for _, r := range required {
		if strings.TrimSpace(r.value) == "" {
			missing = append(missing, r.name)
		}
	}
	if len(missing) > 0 {
		return errors.New("missing IAMKit configuration: " + strings.Join(missing, ", ") + " (run make bootstrap)")
	}
	if c.ServiceSecret != "" {
		if !strings.HasPrefix(c.ServiceSecret, "ik_svc_") {
			return errors.New("IAMKIT_SERVICE_SECRET must be an IAMKit service account credential (ik_svc_...); management keys (ik_mgmt_) are not accepted")
		}
		if strings.TrimSpace(c.OrganizationID) == "" {
			return errors.New("IAMKIT_ORGANIZATION_ID is required when IAMKIT_SERVICE_SECRET is set")
		}
	}
	return nil
}

// Encryption holds secrets for token encryption (provider keys).
type Encryption struct {
	Key string // 32-byte hex-encoded key for NaCl secretbox
}

// Load reads configuration from environment variables with sensible defaults.
func Load() Config {
	return Config{
		Server: Server{
			Port: envOr("SERVER_PORT", "3000"),
			Env:  envOr("APP_ENV", "development"),
		},
		Database: Database{
			Host:     envOr("DB_HOST", "localhost"),
			Port:     envOr("DB_PORT", "5432"),
			User:     envOr("DB_USER", "freerouter"),
			Password: envOr("DB_PASSWORD", "freerouter"),
			Name:     envOr("DB_NAME", "freerouter"),
			SSLMode:  envOr("DB_SSLMODE", "disable"),
		},
		Redis: Redis{
			Addr:     envOr("REDIS_ADDR", "localhost:6379"),
			Password: envOr("REDIS_PASSWORD", ""),
			DB:       envOrInt("REDIS_DB", 0),
		},
		Cache: Cache{
			Enabled: envOrBool("CACHE_ENABLED", true),
			TTL:     time.Duration(envOrInt("CACHE_TTL_SECONDS", 60)) * time.Second,
		},
		Metrics: Metrics{
			Enabled: envOrBool("METRICS_ENABLED", true),
		},
		IAMKit: IAMKit{
			BaseURL:        envOr("IAMKIT_BASE_URL", "http://localhost:8080"),
			ServiceSecret:  envOr("IAMKIT_SERVICE_SECRET", ""),
			EnvironmentID:  envOr("IAMKIT_ENVIRONMENT_ID", ""),
			ApplicationID:  envOr("IAMKIT_APPLICATION_ID", ""),
			ResourceID:     envOr("IAMKIT_RESOURCE_ID", ""),
			OrganizationID: envOr("IAMKIT_ORGANIZATION_ID", ""),
			JWTIssuer:      envOr("IAMKIT_JWT_ISSUER", "http://localhost:8080"),
			Audience:       envOr("IAMKIT_AUDIENCE", ""),
		},
		Encryption: Encryption{
			Key: envOr("ENCRYPTION_KEY", ""),
		},
	}
}

func envOr(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

func envOrInt(key string, fallback int) int {
	v := os.Getenv(key)
	if v == "" {
		return fallback
	}
	n, err := strconv.Atoi(v)
	if err != nil {
		return fallback
	}
	return n
}

func envOrBool(key string, fallback bool) bool {
	v := os.Getenv(key)
	if v == "" {
		return fallback
	}
	b, err := strconv.ParseBool(v)
	if err != nil {
		return fallback
	}
	return b
}
