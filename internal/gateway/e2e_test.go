package gateway_test

// End-to-end tests for the gateway flow: HTTP request in -> router resolves
// provider/key from real Postgres -> upstream call to a fake OpenAI-compatible
// server -> response out, with usage logging and rate limiting backed by real
// Postgres + Redis (via testcontainers). Guardrails are exercised for the
// blocking path. No IAMKit auth middleware is mounted here — the handler is
// tested directly, matching how it is unit-composed by gatewaymodule.

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"

	"github.com/Abraxas-365/freerouter/internal/gateway"
	"github.com/Abraxas-365/freerouter/internal/gateway/gatewaymodule"
	"github.com/Abraxas-365/freerouter/internal/guardrail"
	"github.com/Abraxas-365/freerouter/internal/guardrail/adapters/guardrailpg"
	"github.com/Abraxas-365/freerouter/internal/guardrail/guardrailsvc"
	"github.com/Abraxas-365/freerouter/internal/identity"
	"github.com/Abraxas-365/freerouter/internal/provider"
	"github.com/Abraxas-365/freerouter/internal/provider/adapters/providerpg"
	"github.com/Abraxas-365/freerouter/internal/provider/providersvc"
	"github.com/Abraxas-365/freerouter/internal/providerkey"
	"github.com/Abraxas-365/freerouter/internal/providerkey/adapters/providerkeyinfra"
	"github.com/Abraxas-365/freerouter/internal/providerkey/adapters/providerkeypg"
	"github.com/Abraxas-365/freerouter/internal/providerkey/providerkeysvc"
	"github.com/Abraxas-365/freerouter/internal/ratelimit/adapters/ratelimitpg"
	"github.com/Abraxas-365/freerouter/internal/ratelimit/adapters/ratelimitredis"
	"github.com/Abraxas-365/freerouter/internal/ratelimit/ratelimitsvc"
	"github.com/Abraxas-365/freerouter/internal/server"
	"github.com/Abraxas-365/freerouter/internal/testutil"
	"github.com/Abraxas-365/freerouter/internal/usage/adapters/usagepg"
	"github.com/Abraxas-365/freerouter/internal/usage/usagesvc"
	"github.com/gofiber/fiber/v2"
	"github.com/jmoiron/sqlx"
)


const testEncryptionKey = "5b93f4c4a02a002bab06625d086ad1ced0b270e0db5c4c77a4902ab9099a4e67"

// e2eEnv bundles everything needed to exercise the gateway HTTP handler
// end-to-end against real infrastructure.
type e2eEnv struct {
	db          *sqlx.DB
	app         *fiber.App
	providerSvc *providersvc.Service
	keySvc      *providerkeysvc.Service
	usageSvc    *usagesvc.Service
	guardrail   *guardrailsvc.Service
}

func newE2EEnv(t *testing.T) *e2eEnv {
	t.Helper()
	db := testutil.PostgresDB(t)
	redisClient := testutil.RedisClient(t)

	providerRepo := providerpg.NewProvider(db)
	modelRepo := providerpg.NewModel(db)
	mappingRepo := providerpg.NewMapping(db)
	fallbackRepo := providerpg.NewFallback(db)
	provSvc := providersvc.New(providerRepo, modelRepo, mappingRepo, fallbackRepo)

	encryptor, err := providerkeyinfra.NewEncryptor(testEncryptionKey)
	if err != nil {
		t.Fatalf("failed to create encryptor: %v", err)
	}
	keyRepo := providerkeypg.New(db)
	keySvc := providerkeysvc.New(keyRepo, encryptor, provSvc)

	usageRepo := usagepg.New(db)
	uSvc := usagesvc.New(usageRepo, 100)
	t.Cleanup(uSvc.Close)

	rlRepo := ratelimitpg.New(db)
	rlLimiter := ratelimitredis.New(redisClient)
	rlSvc := ratelimitsvc.New(rlRepo, rlLimiter)

	grRepo := guardrailpg.New(db)
	grSvc := guardrailsvc.New(grRepo)

	mod := gatewaymodule.New(gatewaymodule.Deps{
		Models:      provSvc,
		Mappings:    provSvc,
		Providers:   provSvc,
		Fallbacks:   provSvc,
		Keys:        keySvc,
		Encryptor:   encryptor,
		UsageLogger: uSvc,
		RateLimiter: rlSvc,
		Guardrails:  grSvc,
		Webhooks:    nil,
	})

	srv := server.New("0")
	app := srv.App()
	v1 := app.Group("/v1")
	mod.HTTP.RegisterRoutes(v1)

	return &e2eEnv{
		db:          db,
		app:         app,
		providerSvc: provSvc,
		keySvc:      keySvc,
		usageSvc:    uSvc,
		guardrail:   grSvc,
	}
}

// newE2EEnvWithCache mirrors newE2EEnv but enables response caching against
// the same disposable Redis instance used for rate limiting.
func newE2EEnvWithCache(t *testing.T, ttl time.Duration) *e2eEnv {
	t.Helper()
	db := testutil.PostgresDB(t)
	redisClient := testutil.RedisClient(t)

	providerRepo := providerpg.NewProvider(db)
	modelRepo := providerpg.NewModel(db)
	mappingRepo := providerpg.NewMapping(db)
	fallbackRepo := providerpg.NewFallback(db)
	provSvc := providersvc.New(providerRepo, modelRepo, mappingRepo, fallbackRepo)

	encryptor, err := providerkeyinfra.NewEncryptor(testEncryptionKey)
	if err != nil {
		t.Fatalf("failed to create encryptor: %v", err)
	}
	keyRepo := providerkeypg.New(db)
	keySvc := providerkeysvc.New(keyRepo, encryptor, provSvc)

	usageRepo := usagepg.New(db)
	uSvc := usagesvc.New(usageRepo, 100)
	t.Cleanup(uSvc.Close)

	rlRepo := ratelimitpg.New(db)
	rlLimiter := ratelimitredis.New(redisClient)
	rlSvc := ratelimitsvc.New(rlRepo, rlLimiter)

	grRepo := guardrailpg.New(db)
	grSvc := guardrailsvc.New(grRepo)

	mod := gatewaymodule.New(gatewaymodule.Deps{
		Models:       provSvc,
		Mappings:     provSvc,
		Providers:    provSvc,
		Fallbacks:    provSvc,
		Keys:         keySvc,
		Encryptor:    encryptor,
		UsageLogger:  uSvc,
		RateLimiter:  rlSvc,
		Guardrails:   grSvc,
		Webhooks:     nil,
		Redis:        redisClient,
		CacheEnabled: true,
		CacheTTL:     ttl,
	})

	srv := server.New("0")
	app := srv.App()
	v1 := app.Group("/v1")
	mod.HTTP.RegisterRoutes(v1)

	return &e2eEnv{
		db:          db,
		app:         app,
		providerSvc: provSvc,
		keySvc:      keySvc,
		usageSvc:    uSvc,
		guardrail:   grSvc,
	}
}

// newE2EEnvWithMetrics mirrors newE2EEnv but enables Prometheus metrics and
// returns the collector alongside the environment for assertions.
func newE2EEnvWithMetrics(t *testing.T) (*e2eEnv, *gateway.Metrics) {
	t.Helper()
	db := testutil.PostgresDB(t)
	redisClient := testutil.RedisClient(t)

	providerRepo := providerpg.NewProvider(db)
	modelRepo := providerpg.NewModel(db)
	mappingRepo := providerpg.NewMapping(db)
	fallbackRepo := providerpg.NewFallback(db)
	provSvc := providersvc.New(providerRepo, modelRepo, mappingRepo, fallbackRepo)

	encryptor, err := providerkeyinfra.NewEncryptor(testEncryptionKey)
	if err != nil {
		t.Fatalf("failed to create encryptor: %v", err)
	}
	keyRepo := providerkeypg.New(db)
	keySvc := providerkeysvc.New(keyRepo, encryptor, provSvc)

	usageRepo := usagepg.New(db)
	uSvc := usagesvc.New(usageRepo, 100)
	t.Cleanup(uSvc.Close)

	rlRepo := ratelimitpg.New(db)
	rlLimiter := ratelimitredis.New(redisClient)
	rlSvc := ratelimitsvc.New(rlRepo, rlLimiter)

	grRepo := guardrailpg.New(db)
	grSvc := guardrailsvc.New(grRepo)

	mod := gatewaymodule.New(gatewaymodule.Deps{
		Models:         provSvc,
		Mappings:       provSvc,
		Providers:      provSvc,
		Fallbacks:      provSvc,
		Keys:           keySvc,
		Encryptor:      encryptor,
		UsageLogger:    uSvc,
		RateLimiter:    rlSvc,
		Guardrails:     grSvc,
		Webhooks:       nil,
		MetricsEnabled: true,
	})

	srv := server.New("0")
	app := srv.App()
	v1 := app.Group("/v1")
	mod.HTTP.RegisterRoutes(v1)

	return &e2eEnv{
		db:          db,
		app:         app,
		providerSvc: provSvc,
		keySvc:      keySvc,
		usageSvc:    uSvc,
		guardrail:   grSvc,
	}, mod.Metrics
}

// registerRoute creates a provider + model + mapping + active key pointing at
// the given upstream base URL, and returns the model name and key ID.
func (e *e2eEnv) registerRoute(t *testing.T, provName, upstreamBaseURL, externalID, modelName string) (identity.ModelID, identity.ProviderKeyID) {
	t.Helper()
	ctx := context.Background()

	provID, err := e.providerSvc.Create(ctx, provider.Create{
		Name:     provName,
		Protocol: provider.ProtocolOpenAI,
		BaseURL:  upstreamBaseURL,
	})
	if err != nil {
		t.Fatalf("create provider: %v", err)
	}

	modelID, err := e.providerSvc.CreateModel(ctx, provider.CreateModel{
		Name:   modelName,
		Family: "test",
	})
	if err != nil {
		t.Fatalf("create model: %v", err)
	}

	_, err = e.providerSvc.CreateMapping(ctx, provider.CreateMapping{
		ModelID:    modelID,
		ProviderID: provID,
		ExternalID: externalID,
	})
	if err != nil {
		t.Fatalf("create mapping: %v", err)
	}

	keyID, err := e.keySvc.Create(ctx, providerkey.Create{
		ProviderID: provID,
		Token:      "sk-test-token",
		Name:       provName + "-key",
	})
	if err != nil {
		t.Fatalf("create provider key: %v", err)
	}

	return modelID, keyID
}

func chatRequestBody(model, content string) []byte {
	body, _ := json.Marshal(gateway.ChatRequest{
		Model: model,
		Messages: []gateway.Message{
			{Role: "user", Content: content},
		},
	})
	return body
}

func doRequest(t *testing.T, app *fiber.App, method, path string, body []byte) *http.Response {
	t.Helper()
	req := httptest.NewRequest(method, path, bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	resp, err := app.Test(req, -1)
	if err != nil {
		t.Fatalf("request failed: %v", err)
	}
	return resp
}

// ════════════════════════════════════════════════════════════════════
// Happy path
// ════════════════════════════════════════════════════════════════════

func TestE2E_ChatCompletions_HappyPath(t *testing.T) {
	env := newE2EEnv(t)

	var gotAuth string
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotAuth = r.Header.Get("Authorization")
		if r.URL.Path != "/chat/completions" {
			t.Errorf("unexpected upstream path: %s", r.URL.Path)
		}
		var reqBody map[string]any
		_ = json.NewDecoder(r.Body).Decode(&reqBody)
		if reqBody["model"] != "gpt-real" {
			t.Errorf("expected translated model 'gpt-real', got %v", reqBody["model"])
		}

		resp := gateway.ChatResponse{
			ID:      "chatcmpl-e2e",
			Object:  "chat.completion",
			Created: time.Now().Unix(),
			Model:   "gpt-real",
			Choices: []gateway.Choice{
				{
					Index: 0,
					Message: &gateway.Message{
						Role:    "assistant",
						Content: "Hello from upstream!",
					},
					FinishReason: strPtr("stop"),
				},
			},
			Usage: &gateway.Usage{PromptTokens: 5, CompletionTokens: 3, TotalTokens: 8},
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(resp)
	}))
	defer upstream.Close()

	env.registerRoute(t, "e2e-openai", upstream.URL, "gpt-real", "gpt-e2e")

	resp := doRequest(t, env.app, http.MethodPost, "/v1/chat/completions", chatRequestBody("gpt-e2e", "hi there"))
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200, got %d", resp.StatusCode)
	}
	if gotAuth != "Bearer sk-test-token" {
		t.Fatalf("expected upstream Authorization header with decrypted token, got %q", gotAuth)
	}

	var chatResp gateway.ChatResponse
	if err := json.NewDecoder(resp.Body).Decode(&chatResp); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if chatResp.Model != "gpt-e2e" {
		t.Fatalf("expected response model overridden to canonical name 'gpt-e2e', got %q", chatResp.Model)
	}
	if len(chatResp.Choices) != 1 || chatResp.Choices[0].Message.Content != "Hello from upstream!" {
		t.Fatalf("unexpected choices: %+v", chatResp.Choices)
	}

	// Usage logging is async — poll until the row lands.
	waitForUsageLog(t, env.db, "gpt-e2e")
}

// ════════════════════════════════════════════════════════════════════
// Fallback: primary key errors with a retryable status, fallback succeeds
// ════════════════════════════════════════════════════════════════════

func TestE2E_ChatCompletions_FallbackOnRetryableError(t *testing.T) {
	env := newE2EEnv(t)
	ctx := context.Background()

	var primaryCalls, fallbackCalls int32

	primaryUpstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt32(&primaryCalls, 1)
		w.WriteHeader(http.StatusServiceUnavailable)
		_, _ = w.Write([]byte(`{"error":"primary down"}`))
	}))
	defer primaryUpstream.Close()

	fallbackUpstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt32(&fallbackCalls, 1)
		resp := gateway.ChatResponse{
			ID:      "chatcmpl-fallback",
			Object:  "chat.completion",
			Created: time.Now().Unix(),
			Model:   "fallback-real",
			Choices: []gateway.Choice{
				{Index: 0, Message: &gateway.Message{Role: "assistant", Content: "from fallback"}, FinishReason: strPtr("stop")},
			},
			Usage: &gateway.Usage{PromptTokens: 1, CompletionTokens: 1, TotalTokens: 2},
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(resp)
	}))
	defer fallbackUpstream.Close()

	primaryModelID, _ := env.registerRoute(t, "e2e-primary", primaryUpstream.URL, "primary-real", "primary-model")
	fallbackModelID, _ := env.registerRoute(t, "e2e-fallback", fallbackUpstream.URL, "fallback-real", "fallback-model")

	if _, err := env.providerSvc.CreateFallback(ctx, provider.CreateFallback{
		ModelID:         primaryModelID,
		FallbackModelID: fallbackModelID,
		Priority:        1,
	}); err != nil {
		t.Fatalf("create fallback: %v", err)
	}

	resp := doRequest(t, env.app, http.MethodPost, "/v1/chat/completions", chatRequestBody("primary-model", "hi"))
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200 via fallback, got %d", resp.StatusCode)
	}
	if atomic.LoadInt32(&primaryCalls) < 1 {
		t.Fatalf("expected primary upstream to be attempted at least once")
	}
	if atomic.LoadInt32(&fallbackCalls) != 1 {
		t.Fatalf("expected fallback upstream to be called exactly once, got %d", fallbackCalls)
	}

	var chatResp gateway.ChatResponse
	if err := json.NewDecoder(resp.Body).Decode(&chatResp); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if chatResp.Choices[0].Message.Content != "from fallback" {
		t.Fatalf("expected fallback content, got %+v", chatResp.Choices)
	}
}

// ════════════════════════════════════════════════════════════════════
// Guardrail block: blocked-term rule prevents the upstream call entirely
// ════════════════════════════════════════════════════════════════════

func TestE2E_ChatCompletions_GuardrailBlocks(t *testing.T) {
	env := newE2EEnv(t)
	ctx := context.Background()

	var upstreamCalled int32
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt32(&upstreamCalled, 1)
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{}`))
	}))
	defer upstream.Close()

	env.registerRoute(t, "e2e-guardrail", upstream.URL, "guarded-real", "guarded-model")

	enabled := true
	if _, err := env.guardrail.UpsertConfig(ctx, guardrail.UpsertConfig{Enabled: &enabled}); err != nil {
		t.Fatalf("upsert guardrail config: %v", err)
	}

	blockedConfig, _ := json.Marshal(guardrail.BlockedTermsConfig{
		Terms:     []string{"forbiddenword"},
		MatchType: "contains",
	})
	if _, err := env.guardrail.CreateRule(ctx, guardrail.CreateRule{
		Name:   "block-test-term",
		Type:   guardrail.RuleTypeBlockedTerms,
		Config: blockedConfig,
		Action: guardrail.ActionBlock,
	}); err != nil {
		t.Fatalf("create rule: %v", err)
	}

	resp := doRequest(t, env.app, http.MethodPost, "/v1/chat/completions",
		chatRequestBody("guarded-model", "this message contains forbiddenword in it"))
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("expected 400 guardrail block, got %d", resp.StatusCode)
	}
	if atomic.LoadInt32(&upstreamCalled) != 0 {
		t.Fatalf("expected upstream to never be called when guardrail blocks, got %d calls", upstreamCalled)
	}

	var body map[string]any
	if err := json.NewDecoder(resp.Body).Decode(&body); err != nil {
		t.Fatalf("decode error body: %v", err)
	}
	errObj, ok := body["error"].(map[string]any)
	if !ok || errObj["type"] != "guardrail_violation" {
		t.Fatalf("unexpected error body: %+v", body)
	}
}

// ════════════════════════════════════════════════════════════════════
// Model not found
// ════════════════════════════════════════════════════════════════════

func TestE2E_ChatCompletions_UnknownModel(t *testing.T) {
	env := newE2EEnv(t)

	resp := doRequest(t, env.app, http.MethodPost, "/v1/chat/completions", chatRequestBody("does-not-exist", "hi"))
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusNotFound {
		t.Fatalf("expected 404 for unknown model, got %d", resp.StatusCode)
	}
}

// ════════════════════════════════════════════════════════════════════
// GET /v1/models
// ════════════════════════════════════════════════════════════════════

func TestE2E_ListModels(t *testing.T) {
	env := newE2EEnv(t)
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer upstream.Close()

	env.registerRoute(t, "e2e-list", upstream.URL, "list-real", "list-model")

	resp := doRequest(t, env.app, http.MethodGet, "/v1/models", nil)
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200, got %d", resp.StatusCode)
	}

	var list gateway.ModelList
	if err := json.NewDecoder(resp.Body).Decode(&list); err != nil {
		t.Fatalf("decode model list: %v", err)
	}

	found := false
	for _, m := range list.Data {
		if m.ID == "list-model" {
			found = true
		}
	}
	if !found {
		t.Fatalf("expected 'list-model' in model list, got %+v", list.Data)
	}
}

// ════════════════════════════════════════════════════════════════════
// Response caching
// ════════════════════════════════════════════════════════════════════

func TestE2E_ChatCompletions_CacheHit(t *testing.T) {
	env := newE2EEnvWithCache(t, 60*time.Second)

	var upstreamCalls int32
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt32(&upstreamCalls, 1)
		resp := gateway.ChatResponse{
			ID:      "chatcmpl-cache",
			Object:  "chat.completion",
			Created: time.Now().Unix(),
			Model:   "gpt-real",
			Choices: []gateway.Choice{
				{
					Index:        0,
					Message:      &gateway.Message{Role: "assistant", Content: "Cached response"},
					FinishReason: strPtr("stop"),
				},
			},
			Usage: &gateway.Usage{PromptTokens: 5, CompletionTokens: 3, TotalTokens: 8},
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(resp)
	}))
	defer upstream.Close()

	env.registerRoute(t, "e2e-cache", upstream.URL, "gpt-real", "gpt-cache")
	body := chatRequestBody("gpt-cache", "cache me please")

	// First request: cache miss, hits upstream.
	resp1 := doRequest(t, env.app, http.MethodPost, "/v1/chat/completions", body)
	defer func() { _ = resp1.Body.Close() }()
	if resp1.StatusCode != http.StatusOK {
		t.Fatalf("expected 200, got %d", resp1.StatusCode)
	}
	if got := resp1.Header.Get("X-Cache"); got != "MISS" {
		t.Fatalf("expected X-Cache: MISS on first request, got %q", got)
	}

	// Second identical request: cache hit, upstream not called again.
	resp2 := doRequest(t, env.app, http.MethodPost, "/v1/chat/completions", body)
	defer func() { _ = resp2.Body.Close() }()
	if resp2.StatusCode != http.StatusOK {
		t.Fatalf("expected 200, got %d", resp2.StatusCode)
	}
	if got := resp2.Header.Get("X-Cache"); got != "HIT" {
		t.Fatalf("expected X-Cache: HIT on second request, got %q", got)
	}

	var chatResp gateway.ChatResponse
	if err := json.NewDecoder(resp2.Body).Decode(&chatResp); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if len(chatResp.Choices) != 1 || chatResp.Choices[0].Message.Content != "Cached response" {
		t.Fatalf("unexpected cached choices: %+v", chatResp.Choices)
	}

	if calls := atomic.LoadInt32(&upstreamCalls); calls != 1 {
		t.Fatalf("expected upstream to be called exactly once, got %d calls", calls)
	}
}

func TestE2E_ChatCompletions_CacheMissForDifferentContent(t *testing.T) {
	env := newE2EEnvWithCache(t, 60*time.Second)

	var upstreamCalls int32
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		n := atomic.AddInt32(&upstreamCalls, 1)
		resp := gateway.ChatResponse{
			ID:      fmt.Sprintf("chatcmpl-%d", n),
			Object:  "chat.completion",
			Created: time.Now().Unix(),
			Model:   "gpt-real",
			Choices: []gateway.Choice{
				{Index: 0, Message: &gateway.Message{Role: "assistant", Content: "resp"}, FinishReason: strPtr("stop")},
			},
			Usage: &gateway.Usage{PromptTokens: 5, CompletionTokens: 3, TotalTokens: 8},
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(resp)
	}))
	defer upstream.Close()

	env.registerRoute(t, "e2e-cache-miss", upstream.URL, "gpt-real", "gpt-cache-miss")

	resp1 := doRequest(t, env.app, http.MethodPost, "/v1/chat/completions", chatRequestBody("gpt-cache-miss", "first message"))
	_ = resp1.Body.Close()
	resp2 := doRequest(t, env.app, http.MethodPost, "/v1/chat/completions", chatRequestBody("gpt-cache-miss", "second, different message"))
	_ = resp2.Body.Close()

	if got := resp2.Header.Get("X-Cache"); got != "MISS" {
		t.Fatalf("expected X-Cache: MISS for different content, got %q", got)
	}
	if calls := atomic.LoadInt32(&upstreamCalls); calls != 2 {
		t.Fatalf("expected upstream to be called twice for distinct requests, got %d calls", calls)
	}
}

// ════════════════════════════════════════════════════════════════════
// Prometheus metrics
// ════════════════════════════════════════════════════════════════════

func TestE2E_ChatCompletions_MetricsRecorded(t *testing.T) {
	env, metrics := newE2EEnvWithMetrics(t)

	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		resp := gateway.ChatResponse{
			ID:      "chatcmpl-metrics",
			Object:  "chat.completion",
			Created: time.Now().Unix(),
			Model:   "gpt-real",
			Choices: []gateway.Choice{
				{Index: 0, Message: &gateway.Message{Role: "assistant", Content: "hi"}, FinishReason: strPtr("stop")},
			},
			Usage: &gateway.Usage{PromptTokens: 7, CompletionTokens: 4, TotalTokens: 11},
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(resp)
	}))
	defer upstream.Close()

	env.registerRoute(t, "e2e-metrics", upstream.URL, "gpt-real", "gpt-metrics")

	resp := doRequest(t, env.app, http.MethodPost, "/v1/chat/completions", chatRequestBody("gpt-metrics", "hello"))
	_ = resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200, got %d", resp.StatusCode)
	}

	families, err := metrics.Registry.Gather()
	if err != nil {
		t.Fatalf("gather metrics: %v", err)
	}

	var sawRequestsTotal, sawTokensTotal bool
	for _, fam := range families {
		switch fam.GetName() {
		case "freerouter_gateway_requests_total":
			for _, mtr := range fam.GetMetric() {
				if mtr.GetCounter().GetValue() > 0 {
					sawRequestsTotal = true
				}
			}
		case "freerouter_gateway_tokens_total":
			for _, mtr := range fam.GetMetric() {
				if mtr.GetCounter().GetValue() > 0 {
					sawTokensTotal = true
				}
			}
		}
	}

	if !sawRequestsTotal {
		t.Error("expected freerouter_gateway_requests_total to have a recorded value")
	}
	if !sawTokensTotal {
		t.Error("expected freerouter_gateway_tokens_total to have a recorded value")
	}
}

// ════════════════════════════════════════════════════════════════════
// Anthropic Messages API (/v1/messages)
// ════════════════════════════════════════════════════════════════════

func TestE2E_AnthropicMessages_HappyPath(t *testing.T) {
	env := newE2EEnv(t)

	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var reqBody map[string]any
		_ = json.NewDecoder(r.Body).Decode(&reqBody)
		if reqBody["model"] != "gpt-real" {
			t.Errorf("expected translated model 'gpt-real', got %v", reqBody["model"])
		}

		resp := gateway.ChatResponse{
			ID:      "chatcmpl-anthro",
			Object:  "chat.completion",
			Created: time.Now().Unix(),
			Model:   "gpt-real",
			Choices: []gateway.Choice{
				{
					Index:        0,
					Message:      &gateway.Message{Role: "assistant", Content: "Hello from upstream!"},
					FinishReason: strPtr("stop"),
				},
			},
			Usage: &gateway.Usage{PromptTokens: 6, CompletionTokens: 4, TotalTokens: 10},
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(resp)
	}))
	defer upstream.Close()

	env.registerRoute(t, "e2e-anthropic", upstream.URL, "gpt-real", "claude-e2e")

	body, _ := json.Marshal(map[string]any{
		"model":      "claude-e2e",
		"max_tokens": 256,
		"messages": []map[string]any{
			{"role": "user", "content": "hi there"},
		},
	})

	resp := doRequest(t, env.app, http.MethodPost, "/v1/messages", body)
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200, got %d", resp.StatusCode)
	}

	var ar struct {
		Type    string `json:"type"`
		Role    string `json:"role"`
		Model   string `json:"model"`
		Content []struct {
			Type string `json:"type"`
			Text string `json:"text"`
		} `json:"content"`
		StopReason string `json:"stop_reason"`
		Usage      struct {
			InputTokens  int `json:"input_tokens"`
			OutputTokens int `json:"output_tokens"`
		} `json:"usage"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&ar); err != nil {
		t.Fatalf("decode response: %v", err)
	}

	if ar.Type != "message" {
		t.Fatalf("expected type message, got %q", ar.Type)
	}
	if ar.Role != "assistant" {
		t.Fatalf("expected role assistant, got %q", ar.Role)
	}
	if len(ar.Content) != 1 || ar.Content[0].Type != "text" || ar.Content[0].Text != "Hello from upstream!" {
		t.Fatalf("unexpected content: %+v", ar.Content)
	}
	if ar.StopReason != "end_turn" {
		t.Fatalf("expected stop_reason end_turn, got %q", ar.StopReason)
	}
	if ar.Usage.InputTokens != 6 || ar.Usage.OutputTokens != 4 {
		t.Fatalf("unexpected usage: %+v", ar.Usage)
	}

	waitForUsageLog(t, env.db, "claude-e2e")
}

func TestE2E_AnthropicMessages_UnknownModel(t *testing.T) {
	env := newE2EEnv(t)

	body, _ := json.Marshal(map[string]any{
		"model":      "does-not-exist",
		"max_tokens": 128,
		"messages":   []map[string]any{{"role": "user", "content": "hi"}},
	})

	resp := doRequest(t, env.app, http.MethodPost, "/v1/messages", body)
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusNotFound {
		t.Fatalf("expected 404, got %d", resp.StatusCode)
	}

	var errResp struct {
		Type  string `json:"type"`
		Error struct {
			Type    string `json:"type"`
			Message string `json:"message"`
		} `json:"error"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&errResp); err != nil {
		t.Fatalf("decode error response: %v", err)
	}
	if errResp.Type != "error" {
		t.Fatalf("expected Anthropic error envelope, got %+v", errResp)
	}
}

func TestE2E_AnthropicMessages_MissingMaxTokens(t *testing.T) {
	env := newE2EEnv(t)

	body, _ := json.Marshal(map[string]any{
		"model":    "claude-e2e",
		"messages": []map[string]any{{"role": "user", "content": "hi"}},
	})

	resp := doRequest(t, env.app, http.MethodPost, "/v1/messages", body)
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", resp.StatusCode)
	}
}

// ════════════════════════════════════════════════════════════════════
// OpenAI Responses API (/v1/responses)
// ════════════════════════════════════════════════════════════════════

func TestE2E_Responses_HappyPath(t *testing.T) {
	env := newE2EEnv(t)

	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var reqBody map[string]any
		_ = json.NewDecoder(r.Body).Decode(&reqBody)
		if reqBody["model"] != "gpt-real" {
			t.Errorf("expected translated model 'gpt-real', got %v", reqBody["model"])
		}

		resp := gateway.ChatResponse{
			ID:      "chatcmpl-resp",
			Object:  "chat.completion",
			Created: time.Now().Unix(),
			Model:   "gpt-real",
			Choices: []gateway.Choice{
				{
					Index:        0,
					Message:      &gateway.Message{Role: "assistant", Content: "Hello from responses upstream!"},
					FinishReason: strPtr("stop"),
				},
			},
			Usage: &gateway.Usage{PromptTokens: 8, CompletionTokens: 6, TotalTokens: 14},
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(resp)
	}))
	defer upstream.Close()

	env.registerRoute(t, "e2e-responses", upstream.URL, "gpt-real", "resp-e2e")

	body, _ := json.Marshal(map[string]any{
		"model": "resp-e2e",
		"input": "hi there",
	})

	resp := doRequest(t, env.app, http.MethodPost, "/v1/responses", body)
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200, got %d", resp.StatusCode)
	}

	var rr struct {
		Object string `json:"object"`
		Status string `json:"status"`
		Model  string `json:"model"`
		Output []struct {
			Type    string `json:"type"`
			Role    string `json:"role"`
			Status  string `json:"status"`
			Content []struct {
				Type string `json:"type"`
				Text string `json:"text"`
			} `json:"content"`
		} `json:"output"`
		Usage struct {
			InputTokens  int `json:"input_tokens"`
			OutputTokens int `json:"output_tokens"`
		} `json:"usage"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&rr); err != nil {
		t.Fatalf("decode response: %v", err)
	}

	if rr.Object != "response" {
		t.Fatalf("expected object response, got %q", rr.Object)
	}
	if rr.Status != "completed" {
		t.Fatalf("expected status completed, got %q", rr.Status)
	}
	if len(rr.Output) != 1 || rr.Output[0].Type != "message" || rr.Output[0].Role != "assistant" {
		t.Fatalf("unexpected output: %+v", rr.Output)
	}
	if len(rr.Output[0].Content) != 1 || rr.Output[0].Content[0].Text != "Hello from responses upstream!" {
		t.Fatalf("unexpected content: %+v", rr.Output[0].Content)
	}
	if rr.Usage.InputTokens != 8 || rr.Usage.OutputTokens != 6 {
		t.Fatalf("unexpected usage: %+v", rr.Usage)
	}

	waitForUsageLog(t, env.db, "resp-e2e")
}

func TestE2E_Responses_UnknownModel(t *testing.T) {
	env := newE2EEnv(t)

	body, _ := json.Marshal(map[string]any{
		"model": "does-not-exist",
		"input": "hi",
	})

	resp := doRequest(t, env.app, http.MethodPost, "/v1/responses", body)
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusNotFound {
		t.Fatalf("expected 404, got %d", resp.StatusCode)
	}

	var errResp struct {
		Error struct {
			Message string `json:"message"`
			Type    string `json:"type"`
		} `json:"error"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&errResp); err != nil {
		t.Fatalf("decode error response: %v", err)
	}
	if errResp.Error.Message == "" {
		t.Fatalf("expected error envelope, got %+v", errResp)
	}
}

func TestE2E_Responses_MissingModel(t *testing.T) {
	env := newE2EEnv(t)

	body, _ := json.Marshal(map[string]any{
		"input": "hi",
	})

	resp := doRequest(t, env.app, http.MethodPost, "/v1/responses", body)
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", resp.StatusCode)
	}
}

// ════════════════════════════════════════════════════════════════════
// Cost estimation (/v1/cost/estimate)
// ════════════════════════════════════════════════════════════════════

func TestE2E_EstimateCost_HappyPath(t *testing.T) {
	env := newE2EEnv(t)
	ctx := context.Background()

	provID, err := env.providerSvc.Create(ctx, provider.Create{
		Name: "e2e-cost", Protocol: provider.ProtocolOpenAI, BaseURL: "http://unused.invalid",
	})
	if err != nil {
		t.Fatalf("create provider: %v", err)
	}
	modelID, err := env.providerSvc.CreateModel(ctx, provider.CreateModel{
		Name: "cost-e2e", Family: "test",
	})
	if err != nil {
		t.Fatalf("create model: %v", err)
	}
	inputPrice := 5.0
	outputPrice := 15.0
	_, err = env.providerSvc.CreateMapping(ctx, provider.CreateMapping{
		ModelID: modelID, ProviderID: provID, ExternalID: "cost-real",
		InputPrice: &inputPrice, OutputPrice: &outputPrice,
	})
	if err != nil {
		t.Fatalf("create mapping: %v", err)
	}
	if _, err := env.keySvc.Create(ctx, providerkey.Create{
		ProviderID: provID, Token: "sk-test-token", Name: "e2e-cost-key",
	}); err != nil {
		t.Fatalf("create provider key: %v", err)
	}

	maxTokens := 100
	body, _ := json.Marshal(gateway.CostEstimateRequest{
		Model: "cost-e2e",
		Messages: []gateway.Message{
			{Role: "user", Content: "This is a test message for cost estimation."},
		},
		MaxTokens: &maxTokens,
	})

	resp := doRequest(t, env.app, http.MethodPost, "/v1/cost/estimate", body)
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200, got %d", resp.StatusCode)
	}

	var est gateway.CostEstimateResponse
	if err := json.NewDecoder(resp.Body).Decode(&est); err != nil {
		t.Fatalf("decode response: %v", err)
	}

	if est.Model != "cost-e2e" {
		t.Errorf("expected model cost-e2e, got %q", est.Model)
	}
	if est.EstimatedInputTokens <= 0 {
		t.Errorf("expected positive estimated input tokens, got %d", est.EstimatedInputTokens)
	}
	if est.EstimatedOutputTokens != maxTokens {
		t.Errorf("expected estimated output tokens %d, got %d", maxTokens, est.EstimatedOutputTokens)
	}
	if est.InputCostUSD <= 0 {
		t.Errorf("expected positive input cost, got %f", est.InputCostUSD)
	}
	wantOutputCost := float64(maxTokens) * outputPrice / 1_000_000
	if est.OutputCostUSD != wantOutputCost {
		t.Errorf("expected output cost %f, got %f", wantOutputCost, est.OutputCostUSD)
	}
	if est.TotalCostUSD != est.InputCostUSD+est.OutputCostUSD {
		t.Errorf("expected total cost to equal input+output, got %f vs %f", est.TotalCostUSD, est.InputCostUSD+est.OutputCostUSD)
	}
}

func TestE2E_EstimateCost_UnknownModel(t *testing.T) {
	env := newE2EEnv(t)

	body, _ := json.Marshal(gateway.CostEstimateRequest{
		Model:    "does-not-exist",
		Messages: []gateway.Message{{Role: "user", Content: "hi"}},
	})

	resp := doRequest(t, env.app, http.MethodPost, "/v1/cost/estimate", body)
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusNotFound {
		t.Fatalf("expected 404, got %d", resp.StatusCode)
	}
}

func TestE2E_EstimateCost_MissingModel(t *testing.T) {
	env := newE2EEnv(t)

	body, _ := json.Marshal(map[string]any{
		"messages": []map[string]any{{"role": "user", "content": "hi"}},
	})

	resp := doRequest(t, env.app, http.MethodPost, "/v1/cost/estimate", body)
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", resp.StatusCode)
	}
}

// ── helpers ─────────────────────────────────────────────────────────

func strPtr(s string) *string { return &s }

// waitForUsageLog polls the usage_logs table for a log matching the given
// requested model, since usage logging is fire-and-forget/async.
func waitForUsageLog(t *testing.T, db *sqlx.DB, requestedModel string) {
	t.Helper()
	deadline := time.Now().Add(3 * time.Second)
	for time.Now().Before(deadline) {
		var count int
		err := db.Get(&count, `SELECT count(*) FROM usage_logs WHERE requested_model = $1`, requestedModel)
		if err == nil && count > 0 {
			return
		}
		time.Sleep(50 * time.Millisecond)
	}
	t.Fatalf("timed out waiting for usage log for model %q", requestedModel)
}
