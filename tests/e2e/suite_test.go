package e2e

import (
	"bytes"
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"runtime"
	"sort"
	"strings"
	"testing"
	"time"

	"github.com/Abraxas-365/freerouter/internal/ai/aicontainer"
	"github.com/Abraxas-365/freerouter/internal/billing"
	"github.com/Abraxas-365/freerouter/internal/billing/billingcontainer"
	"github.com/Abraxas-365/freerouter/internal/config"
	"github.com/Abraxas-365/freerouter/internal/iam/apikey"
	"github.com/Abraxas-365/freerouter/internal/iam/iamcontainer"
	"github.com/Abraxas-365/freerouter/internal/kernel"
	"github.com/Abraxas-365/freerouter/internal/webhook/webhookcontainer"
	"github.com/gofiber/fiber/v2"
	"github.com/google/uuid"
	"github.com/jmoiron/sqlx"
	"github.com/lib/pq"
	_ "github.com/lib/pq"
	"github.com/redis/go-redis/v9"
	"github.com/testcontainers/testcontainers-go"
	tcpostgres "github.com/testcontainers/testcontainers-go/modules/postgres"
	tcredis "github.com/testcontainers/testcontainers-go/modules/redis"
	"github.com/testcontainers/testcontainers-go/wait"
)

// ============================================================================
// Test Suite
// ============================================================================

type Suite struct {
	T             *testing.T
	DB            *sqlx.DB
	Redis         *redis.Client
	App           *fiber.App
	IAM           *iamcontainer.Container
	AI            *aicontainer.Container
	Billing       *billingcontainer.Container
	Webhook       *webhookcontainer.Container
	Config        *config.Config
	MockUpstream  *httptest.Server
	TenantID      kernel.TenantID
	UserID        kernel.UserID
	JWTToken      string
	APIKey        string
	pgContainer   testcontainers.Container
	redisContainer testcontainers.Container
}

func NewSuite(t *testing.T) *Suite {
	t.Helper()
	ctx := context.Background()

	s := &Suite{T: t}

	// Start PostgreSQL container
	pgC, err := tcpostgres.Run(ctx,
		"postgres:16-alpine",
		tcpostgres.WithDatabase("freerouter_test"),
		tcpostgres.WithUsername("test"),
		tcpostgres.WithPassword("test"),
		testcontainers.WithWaitStrategy(
			wait.ForLog("database system is ready to accept connections").
				WithOccurrence(2).
				WithStartupTimeout(30*time.Second),
		),
	)
	if err != nil {
		t.Fatalf("failed to start postgres container: %v", err)
	}
	s.pgContainer = pgC

	pgConnStr, err := pgC.ConnectionString(ctx, "sslmode=disable")
	if err != nil {
		t.Fatalf("failed to get postgres connection string: %v", err)
	}

	// Start Redis container
	redisC, err := tcredis.Run(ctx, "redis:7-alpine")
	if err != nil {
		t.Fatalf("failed to start redis container: %v", err)
	}
	s.redisContainer = redisC

	redisEndpoint, err := redisC.Endpoint(ctx, "")
	if err != nil {
		t.Fatalf("failed to get redis endpoint: %v", err)
	}

	// Connect to DB
	s.DB, err = sqlx.Connect("postgres", pgConnStr)
	if err != nil {
		t.Fatalf("failed to connect to postgres: %v", err)
	}

	// Connect to Redis
	s.Redis = redis.NewClient(&redis.Options{Addr: redisEndpoint})

	// Run migrations
	s.runMigrations()

	// Build config
	s.Config = s.buildConfig(pgConnStr, redisEndpoint)

	// Start mock upstream provider
	s.MockUpstream = s.startMockUpstream()

	// Initialize containers
	s.initContainers()

	// Create test tenant, user, and auth tokens
	s.seedTestData()

	// Build Fiber app with all routes
	s.buildApp()

	// Cleanup on test finish
	t.Cleanup(func() {
		s.App.Shutdown()
		s.MockUpstream.Close()
		s.DB.Close()
		s.Redis.Close()
		s.pgContainer.Terminate(ctx)
		s.redisContainer.Terminate(ctx)
	})

	return s
}

// ============================================================================
// HTTP helpers
// ============================================================================

func (s *Suite) Request(method, path string, body any) *http.Request {
	var bodyReader io.Reader
	if body != nil {
		data, _ := json.Marshal(body)
		bodyReader = bytes.NewReader(data)
	}
	req, _ := http.NewRequest(method, path, bodyReader)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+s.JWTToken)
	return req
}

func (s *Suite) RequestWithAPIKey(method, path string, body any) *http.Request {
	var bodyReader io.Reader
	if body != nil {
		data, _ := json.Marshal(body)
		bodyReader = bytes.NewReader(data)
	}
	req, _ := http.NewRequest(method, path, bodyReader)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+s.APIKey)
	return req
}

func (s *Suite) Do(req *http.Request) (*http.Response, []byte) {
	resp, err := s.App.Test(req, -1)
	if err != nil {
		s.T.Fatalf("request failed: %v", err)
	}
	body, _ := io.ReadAll(resp.Body)
	resp.Body.Close()
	return resp, body
}

func (s *Suite) DoJSON(req *http.Request, out any) *http.Response {
	resp, body := s.Do(req)
	if out != nil {
		if err := json.Unmarshal(body, out); err != nil {
			s.T.Fatalf("failed to unmarshal response: %v\nbody: %s", err, body)
		}
	}
	return resp
}

// ============================================================================
// Internal setup
// ============================================================================

func (s *Suite) runMigrations() {
	// Find migrations directory
	_, filename, _, _ := runtime.Caller(0)
	projectRoot := filepath.Join(filepath.Dir(filename), "..", "..")
	migrationsDir := filepath.Join(projectRoot, "migrations")

	files, err := os.ReadDir(migrationsDir)
	if err != nil {
		s.T.Fatalf("failed to read migrations dir: %v", err)
	}

	// Sort and run .up.sql files in order
	var upFiles []string
	for _, f := range files {
		if strings.HasSuffix(f.Name(), ".up.sql") {
			upFiles = append(upFiles, f.Name())
		}
	}
	sort.Strings(upFiles)

	for _, f := range upFiles {
		data, err := os.ReadFile(filepath.Join(migrationsDir, f))
		if err != nil {
			s.T.Fatalf("failed to read migration %s: %v", f, err)
		}
		if _, err := s.DB.Exec(string(data)); err != nil {
			s.T.Fatalf("failed to run migration %s: %v", f, err)
		}
	}
}

func (s *Suite) buildConfig(pgConnStr, redisAddr string) *config.Config {
	return &config.Config{
		Server: config.ServerConfig{
			Port:        0,
			Environment: "test",
		},
		Environment: config.EnvironmentDevelopment,
		Auth: config.AuthConfig{
			JWT: config.JWTConfig{
				SecretKey:       "e2e-test-secret-key-at-least-32-bytes-long",
				AccessTokenTTL:  1 * time.Hour,
				RefreshTokenTTL: 24 * time.Hour,
				Issuer:          "freerouter-test",
				Audience:        []string{"freerouter-api"},
			},
			APIKey: config.APIKeyConfig{
				LivePrefix:  "fr_live",
				TestPrefix:  "fr_test",
				TokenLength: 32,
			},
		},
		TenantConfig: config.TenantConfig{
			MaxUsersDefault: 10,
		},
		AI: config.AIConfig{
			EncryptionKey:   "0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef", // 64 hex chars = 32 bytes
			UsageBufferSize: 100,
		},
	}
}

func (s *Suite) startMockUpstream() *httptest.Server {
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Image generation endpoint
		if strings.HasSuffix(r.URL.Path, "/images/generations") {
			s.respondImageGeneration(w)
			return
		}

		// Embeddings endpoint
		if strings.HasSuffix(r.URL.Path, "/embeddings") {
			s.respondEmbedding(w)
			return
		}

		// Check if streaming
		body, _ := io.ReadAll(r.Body)
		var req map[string]any
		json.Unmarshal(body, &req)

		if stream, ok := req["stream"].(bool); ok && stream {
			s.respondStream(w)
			return
		}

		s.respondNonStream(w)
	}))
}

func (s *Suite) respondNonStream(w http.ResponseWriter) {
	resp := map[string]any{
		"id":      "chatcmpl-e2e-123",
		"object":  "chat.completion",
		"created": time.Now().Unix(),
		"model":   "gpt-4o-2024-08-06",
		"choices": []map[string]any{{
			"index": 0,
			"message": map[string]any{
				"role":    "assistant",
				"content": "Hello from E2E test!",
			},
			"finish_reason": "stop",
		}},
		"usage": map[string]any{
			"prompt_tokens":     10,
			"completion_tokens": 6,
			"total_tokens":      16,
		},
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(resp)
}

func (s *Suite) respondStream(w http.ResponseWriter) {
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")

	flusher, ok := w.(http.Flusher)
	if !ok {
		http.Error(w, "streaming not supported", 500)
		return
	}

	chunks := []string{
		`{"id":"chatcmpl-e2e-123","object":"chat.completion.chunk","created":1234567890,"model":"gpt-4o","choices":[{"index":0,"delta":{"role":"assistant"},"finish_reason":null}]}`,
		`{"id":"chatcmpl-e2e-123","object":"chat.completion.chunk","created":1234567890,"model":"gpt-4o","choices":[{"index":0,"delta":{"content":"Hello"},"finish_reason":null}]}`,
		`{"id":"chatcmpl-e2e-123","object":"chat.completion.chunk","created":1234567890,"model":"gpt-4o","choices":[{"index":0,"delta":{"content":" from E2E!"},"finish_reason":null}]}`,
		`{"id":"chatcmpl-e2e-123","object":"chat.completion.chunk","created":1234567890,"model":"gpt-4o","choices":[{"index":0,"delta":{},"finish_reason":"stop"}],"usage":{"prompt_tokens":10,"completion_tokens":3,"total_tokens":13}}`,
	}

	for _, chunk := range chunks {
		fmt.Fprintf(w, "data: %s\n\n", chunk)
		flusher.Flush()
	}
	fmt.Fprintf(w, "data: [DONE]\n\n")
	flusher.Flush()
}

func (s *Suite) respondImageGeneration(w http.ResponseWriter) {
	resp := map[string]any{
		"created": time.Now().Unix(),
		"data": []map[string]any{{
			"url":             "https://example.com/generated-image.png",
			"revised_prompt":  "A cute cat sitting on a windowsill, detailed digital art",
		}},
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(resp)
}

func (s *Suite) respondEmbedding(w http.ResponseWriter) {
	resp := map[string]any{
		"object": "list",
		"data": []map[string]any{{
			"object":    "embedding",
			"embedding": []float64{0.1, 0.2, 0.3, 0.4, 0.5},
			"index":     0,
		}},
		"model": "text-embedding-3-small",
		"usage": map[string]any{
			"prompt_tokens": 8,
			"total_tokens":  8,
		},
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(resp)
}

func (s *Suite) initContainers() {
	apikey.InitAPIKeyConfig(
		s.Config.Auth.APIKey.LivePrefix,
		s.Config.Auth.APIKey.TestPrefix,
		s.Config.Auth.APIKey.TokenLength,
	)

	s.IAM = iamcontainer.New(iamcontainer.Deps{
		DB:    s.DB,
		Redis: s.Redis,
		Cfg:   s.Config,
	})

	s.Billing = billingcontainer.New(s.DB, s.Config.Stripe)
	s.Webhook = webhookcontainer.New(s.DB)

	s.AI = aicontainer.New(aicontainer.Deps{
		DB:             s.DB,
		Redis:          s.Redis,
		Cfg:            s.Config,
		BillingRepo:    s.Billing.Repo,
		BillingService: s.Billing.Service,
	})
}

func (s *Suite) seedTestData() {
	ctx := context.Background()

	// Create test tenant
	s.TenantID = kernel.NewTenantID(uuid.NewString())
	now := time.Now().UTC()
	_, err := s.DB.ExecContext(ctx,
		`INSERT INTO tenants (id, company_name, status, max_users, created_at, updated_at)
		 VALUES ($1, $2, $3, $4, $5, $6)`,
		s.TenantID.String(), "E2E Test Tenant", "ACTIVE", 10, now, now,
	)
	if err != nil {
		s.T.Fatalf("failed to create test tenant: %v", err)
	}

	// Create test user
	userID := uuid.NewString()
	s.UserID = kernel.NewUserID(userID)
	_, err = s.DB.ExecContext(ctx,
		`INSERT INTO users (id, email, name, status, tenant_id, oauth_provider, oauth_provider_id, email_verified, created_at, updated_at)
		 VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10)`,
		userID, "test@e2e.com", "E2E Test User", "ACTIVE", s.TenantID.String(), "GOOGLE", "e2e-test-oauth-id", true, now, now,
	)
	if err != nil {
		s.T.Fatalf("failed to create test user: %v", err)
	}

	// Generate JWT token with all scopes
	allScopes := []string{
		"tenant:read", "tenant:write",
		"providers:read", "providers:write", "providers:delete",
		"models:read", "models:write", "models:delete",
		"gateway:read", "gateway:write", "gateway:chat",
		"billing:read", "billing:write", "billing:admin",
		"usage:read",
		"api_key:read", "api_key:write",
		"api_keys:read", "api_keys:write", "api_keys:delete",
		"provider-keys:read", "provider-keys:write", "provider-keys:delete",
		"rate-limits:read", "rate-limits:write",
		"webhooks:read", "webhooks:write",
		"guardrails:read", "guardrails:write",
	}
	s.JWTToken, err = s.IAM.TokenService.GenerateAccessToken(
		s.UserID, s.TenantID,
		map[string]any{
			"email":  "test@e2e.com",
			"name":   "E2E Test User",
			"scopes": allScopes,
		},
	)
	if err != nil {
		s.T.Fatalf("failed to generate JWT: %v", err)
	}

	// Create API key directly in DB
	rawKey := make([]byte, 32)
	rand.Read(rawKey)
	keySecret := hex.EncodeToString(rawKey)
	s.APIKey = fmt.Sprintf("fr_live_%s", keySecret)
	keyHash := apikey.HashAPIKey(s.APIKey)

	apiKeyID := uuid.NewString()
	_, err = s.DB.ExecContext(ctx,
		`INSERT INTO api_keys (id, key_hash, key_prefix, tenant_id, user_id, name, scopes, is_active, created_at, updated_at)
		 VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10)`,
		apiKeyID,
		keyHash,
		fmt.Sprintf("fr_live_%s...", keySecret[:8]),
		s.TenantID.String(),
		userID,
		"E2E Test Key",
		pq.Array([]string{"gateway:read", "gateway:chat", "billing:read", "usage:read"}),
		true,
		now, now,
	)
	if err != nil {
		s.T.Fatalf("failed to create API key: %v", err)
	}

	// Seed billing balance (give the tenant $100)
	_, _, err = s.Billing.Service.TopUp(ctx, s.TenantID, billing.TopUpRequest{
		Amount:      100.0,
		Description: "E2E test credit",
	})
	if err != nil {
		s.T.Fatalf("failed to top up billing: %v", err)
	}

	// Insert a managed provider key for openai pointing to mock upstream
	mockToken := "sk-mock-test-token-for-e2e"
	ciphertext, err := s.AI.ProviderKey.Encryptor.Encrypt(mockToken)
	if err != nil {
		s.T.Fatalf("failed to encrypt mock token: %v", err)
	}
	mockURL := s.MockUpstream.URL

	provKeyID := uuid.NewString()
	_, err = s.DB.ExecContext(ctx,
		`INSERT INTO provider_keys (id, provider_id, tenant_id, token_ciphertext, token_hash, token_masked, base_url, name, description, managed, status, created_at, updated_at)
		 VALUES ($1, $2, NULL, $3, $4, $5, $6, $7, $8, true, 'active', $9, $10)`,
		provKeyID, "openai",
		ciphertext,
		s.AI.ProviderKey.Encryptor.Hash(mockToken),
		s.AI.ProviderKey.Encryptor.Mask(mockToken),
		mockURL,
		"E2E Mock Key",
		"Mock provider key for E2E testing",
		now, now,
	)
	if err != nil {
		s.T.Fatalf("failed to create provider key: %v", err)
	}
}

func (s *Suite) buildApp() {
	s.App = fiber.New(fiber.Config{
		DisableStartupMessage: true,
	})

	api := s.App.Group("/api")
	protected := api.Group("/v1", s.IAM.UnifiedAuthMiddleware.Authenticate())

	// Provider routes
	s.AI.Provider.Handlers.RegisterRoutes(protected, s.IAM.UnifiedAuthMiddleware)

	// Usage routes
	s.AI.Usage.Handlers.RegisterRoutes(protected, s.IAM.UnifiedAuthMiddleware)

	// Billing routes
	s.Billing.Handlers.RegisterRoutes(protected, s.IAM.UnifiedAuthMiddleware)

	// Guardrails routes
	s.AI.Guardrails.Handlers.RegisterRoutes(protected, s.IAM.UnifiedAuthMiddleware)

	// Gateway routes (registered on app root, same as server.go)
	// This puts routes at /v1/models, /v1/chat/completions, etc.
	s.AI.Gateway.Handlers.RegisterRoutes(s.App, s.IAM.UnifiedAuthMiddleware)

	// Rate limit config routes (under /api/v1)
	s.AI.Gateway.Handlers.RegisterAdminRoutes(protected, s.IAM.UnifiedAuthMiddleware)

	// Provider key routes
	s.AI.ProviderKey.Handlers.RegisterRoutes(protected, s.IAM.UnifiedAuthMiddleware)

	// Webhook routes
	s.Webhook.Handlers.RegisterRoutes(protected, s.IAM.UnifiedAuthMiddleware)

	// IAM API key routes
	s.IAM.APIKeyHandlers.RegisterRoutes(protected, s.IAM.UnifiedAuthMiddleware)

	// Wire webhooks into gateway
	s.AI.Gateway.Handlers.SetWebhooks(s.Webhook.Service)
}
