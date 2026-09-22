package config

import (
	"fmt"
	"os"
	"strconv"
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

// IAMKit holds IAMKit service configuration.
type IAMKit struct {
	BaseURL        string // e.g. "http://localhost:8080"
	ManagementKey  string // ik_mgmt_... for server-side setup
	EnvironmentID  string // IAMKit environment UUID
	ApplicationID  string // IAMKit application UUID
	ResourceID     string // IAMKit resource UUID
	OrganizationID string // Default organization UUID
	JWTIssuer      string // Token issuer URL (must match IAMKit's JWT_ISSUER)
	Audience       string // Resource audience URL
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
			ManagementKey:  envOr("IAMKIT_MANAGEMENT_KEY", ""),
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
