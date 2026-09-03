package gatewayapi

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"time"

	"github.com/Abraxas-365/freerouter/internal/ai/gateway"
	"github.com/Abraxas-365/freerouter/internal/ai/guardrails"
	"github.com/Abraxas-365/freerouter/internal/ai/guardrails/guardrailssrv"
	"github.com/Abraxas-365/freerouter/internal/ai/provider"
	"github.com/Abraxas-365/freerouter/internal/ai/usage"
	"github.com/Abraxas-365/freerouter/internal/ai/usage/usagesrv"
	"github.com/Abraxas-365/freerouter/internal/billing/billingsrv"
	"github.com/Abraxas-365/freerouter/internal/iam/auth"
	"github.com/Abraxas-365/freerouter/internal/iam/scopes"
	"github.com/Abraxas-365/freerouter/internal/kernel"
	"github.com/Abraxas-365/freerouter/internal/wallet/walletsrv"
	"github.com/Abraxas-365/freerouter/internal/webhook"
	"github.com/Abraxas-365/freerouter/internal/webhook/webhooksrv"
	"github.com/gofiber/fiber/v2"
)

// isDebugMode returns true if debug content logging should be enabled for
// this request, either via the X-Debug header or the FORCE_DEBUG_MODE env var.
func isDebugMode(c *fiber.Ctx) bool {
	if c.Get("X-Debug") == "true" {
		return true
	}
	return os.Getenv("FORCE_DEBUG_MODE") == "true"
}

// buildRequestContent builds the usage.RequestContent for a completed
// non-streaming request. Messages and response body are always stored; the
// raw/upstream payloads are only populated when debug mode is enabled.
func buildRequestContent(c *fiber.Ctx, messages []gateway.Message, resp *gateway.ChatResponse, upstreamBody []byte) *usage.RequestContent {
	messagesJSON, _ := json.Marshal(messages)
	respJSON, _ := json.Marshal(resp)
	content := &usage.RequestContent{
		Messages:     messagesJSON,
		ResponseBody: respJSON,
		IsDebug:      isDebugMode(c),
	}
	if content.IsDebug {
		content.RawRequest = json.RawMessage(c.Body())
		content.UpstreamRequest = upstreamBody
		content.UpstreamResponse = respJSON
		content.RawResponse = respJSON
	}
	return content
}

// buildStreamRequestContent builds the usage.RequestContent for a completed
// streaming request. Only the input messages are stored (no full response
// body is available for streams); raw payloads are populated in debug mode.
// isDebug and rawBody must be captured before entering the async stream
// writer goroutine since the fiber.Ctx is not safe to use there.
func buildStreamRequestContent(isDebug bool, rawBody []byte, messages []gateway.Message) *usage.RequestContent {
	messagesJSON, _ := json.Marshal(messages)
	content := &usage.RequestContent{
		Messages: messagesJSON,
		IsDebug:  isDebug,
	}
	if content.IsDebug {
		content.RawRequest = json.RawMessage(rawBody)
	}
	return content
}

type GatewayHandlers struct {
	router        *gateway.Router
	upstream      *gateway.Upstream
	usage         *usagesrv.UsageService
	billing       *billingsrv.BillingService
	modelRepo     provider.ModelRepository
	mappingRepo   provider.MappingRepository
	healthTracker *gateway.KeyHealthTracker
	guardrails    *guardrailssrv.GuardrailsService
	rateLimiter   *gateway.RateLimiter
	cache         *gateway.ResponseCache
	metrics       *gateway.Metrics
	webhooks      *webhooksrv.WebhookService      // optional, nil = no webhook notifications
	wallet        *walletsrv.WalletService        // optional, nil = wallet billing disabled
	routingRepo   gateway.RoutingConfigRepository // optional, nil = no per-tenant strategy
}

func NewGatewayHandlers(
	router *gateway.Router,
	upstream *gateway.Upstream,
	usage *usagesrv.UsageService,
	billing *billingsrv.BillingService,
	modelRepo provider.ModelRepository,
	mappingRepo provider.MappingRepository,
	healthTracker *gateway.KeyHealthTracker,
	guardrailsSvc *guardrailssrv.GuardrailsService,
	rateLimiter *gateway.RateLimiter,
	cache *gateway.ResponseCache,
	metrics *gateway.Metrics,
) *GatewayHandlers {
	return &GatewayHandlers{
		router:        router,
		upstream:      upstream,
		usage:         usage,
		billing:       billing,
		modelRepo:     modelRepo,
		mappingRepo:   mappingRepo,
		healthTracker: healthTracker,
		guardrails:    guardrailsSvc,
		rateLimiter:   rateLimiter,
		cache:         cache,
		metrics:       metrics,
	}
}

// SetWebhooks sets the webhook service for firing events.
func (h *GatewayHandlers) SetWebhooks(ws *webhooksrv.WebhookService) {
	h.webhooks = ws
}

// SetWalletService sets the optional wallet service used for API-key wallet billing.
func (h *GatewayHandlers) SetWalletService(ws *walletsrv.WalletService) {
	h.wallet = ws
}

// SetRoutingConfigRepo sets the routing config repository for per-tenant strategy lookup.
func (h *GatewayHandlers) SetRoutingConfigRepo(repo gateway.RoutingConfigRepository) {
	h.routingRepo = repo
}

// checkModelAccess verifies that the API key (if used) is allowed to access
// the requested model. If AllowedModels is empty, all models are allowed.
func (h *GatewayHandlers) checkModelAccess(authCtx *kernel.AuthContext, model string) error {
	if !authCtx.IsAPIKey || len(authCtx.AllowedModels) == 0 {
		return nil
	}
	for _, m := range authCtx.AllowedModels {
		if m == model {
			return nil
		}
	}
	return fiber.NewError(fiber.StatusForbidden, fmt.Sprintf("model %q is not allowed for this API key", model))
}

func (h *GatewayHandlers) RegisterRoutes(router fiber.Router, authMiddleware *auth.UnifiedAuthMiddleware) {
	v1 := router.Group("/v1", authMiddleware.Authenticate())
	v1.Get("/models", authMiddleware.RequireScope(scopes.ScopeGatewayRead), h.ListModels)
	v1.Post("/chat/completions", authMiddleware.RequireScope(scopes.ScopeGatewayChat), h.ChatCompletions)
	v1.Post("/messages", authMiddleware.RequireScope(scopes.ScopeGatewayChat), h.AnthropicMessages)
	v1.Post("/responses", authMiddleware.RequireScope(scopes.ScopeGatewayChat), h.Responses)
	v1.Post("/images/generations", authMiddleware.RequireScope(scopes.ScopeGatewayChat), h.ImageGeneration)
	v1.Post("/embeddings", authMiddleware.RequireScope(scopes.ScopeGatewayChat), h.Embeddings)
	v1.Post("/cost/estimate", authMiddleware.RequireScope(scopes.ScopeGatewayRead), h.EstimateCost)
}

// RegisterAdminRoutes registers rate limit config and cache invalidation routes (under /api/v1).
func (h *GatewayHandlers) RegisterAdminRoutes(router fiber.Router, authMiddleware *auth.UnifiedAuthMiddleware) {
	rl := router.Group("/rate-limits", authMiddleware.Authenticate())
	rl.Get("/:tenantId", authMiddleware.RequireScope(scopes.ScopeRateLimitsRead), auth.ValidateTenantAccess(), h.GetRateLimitConfig)
	rl.Put("/:tenantId", authMiddleware.RequireScope(scopes.ScopeRateLimitsWrite), auth.ValidateTenantAccess(), h.UpsertRateLimitConfig)
	rl.Delete("/:tenantId", authMiddleware.RequireScope(scopes.ScopeRateLimitsWrite), auth.ValidateTenantAccess(), h.DeleteRateLimitConfig)

	cache := router.Group("/cache", authMiddleware.Authenticate())
	cache.Delete("/", authMiddleware.RequireScope(scopes.PlatformAdmin), h.InvalidateCacheAll)
	cache.Delete("/:tenantId", authMiddleware.RequireScope(scopes.ScopeGatewayChat), auth.ValidateTenantAccess(), h.InvalidateCacheTenant)

	routing := router.Group("/routing", authMiddleware.Authenticate())
	routing.Get("/:tenantId", authMiddleware.RequireScope(scopes.ScopeGatewayRead), auth.ValidateTenantAccess(), h.GetRoutingConfig)
	routing.Put("/:tenantId", authMiddleware.RequireScope(scopes.ScopeGatewayWrite), auth.ValidateTenantAccess(), h.UpsertRoutingConfig)
	routing.Delete("/:tenantId", authMiddleware.RequireScope(scopes.ScopeGatewayWrite), auth.ValidateTenantAccess(), h.DeleteRoutingConfig)
}

// ListModels returns all active models in OpenAI-compatible format.
func (h *GatewayHandlers) ListModels(c *fiber.Ctx) error {
	models, err := h.modelRepo.FindActive(c.Context())
	if err != nil {
		return fiber.NewError(fiber.StatusInternalServerError, "failed to list models")
	}

	type openAIModel struct {
		ID      string `json:"id"`
		Object  string `json:"object"`
		Created int64  `json:"created"`
		OwnedBy string `json:"owned_by"`
	}

	data := make([]openAIModel, 0, len(models))
	for _, m := range models {
		data = append(data, openAIModel{
			ID:      m.ID.String(),
			Object:  "model",
			Created: m.CreatedAt.Unix(),
			OwnedBy: m.Family,
		})
	}

	return c.JSON(fiber.Map{
		"object": "list",
		"data":   data,
	})
}

func (h *GatewayHandlers) EstimateCost(c *fiber.Ctx) error {
	var req gateway.CostEstimateRequest
	if err := json.Unmarshal(c.Body(), &req); err != nil {
		return fiber.NewError(fiber.StatusBadRequest, "invalid request body")
	}
	if req.Model == "" {
		return fiber.NewError(fiber.StatusBadRequest, "model is required")
	}

	pricing, err := h.router.GetPricing(c.Context(), req.Model)
	if err != nil {
		return err
	}

	inputTokens := gateway.EstimateMessageTokens(req.Messages)

	maxOutput := 4096
	if req.MaxTokens != nil && *req.MaxTokens > 0 {
		maxOutput = *req.MaxTokens
	} else if pricing.MaxOutput != nil && *pricing.MaxOutput > 0 {
		maxOutput = *pricing.MaxOutput / 4 // conservative: assume 1/4 of max
	}

	var inputCost, outputCost float64
	if pricing.InputPrice != nil {
		inputCost = float64(inputTokens) * *pricing.InputPrice / 1_000_000
	}
	if pricing.OutputPrice != nil {
		outputCost = float64(maxOutput) * *pricing.OutputPrice / 1_000_000
	}

	return c.JSON(gateway.CostEstimateResponse{
		Model:                 req.Model,
		Provider:              pricing.ProviderID,
		EstimatedInputTokens:  inputTokens,
		MaxOutputTokens:       maxOutput,
		InputPricePerMillion:  pricing.InputPrice,
		OutputPricePerMillion: pricing.OutputPrice,
		EstimatedInputCost:    inputCost,
		EstimatedOutputCost:   outputCost,
		EstimatedTotalCost:    inputCost + outputCost,
	})
}

func (h *GatewayHandlers) ChatCompletions(c *fiber.Ctx) error {
	authCtx, ok := auth.GetAuthContext(c)
	if !ok {
		return fiber.NewError(fiber.StatusUnauthorized, "unauthorized")
	}

	// Rate limit check
	if err := h.checkRateLimit(c, authCtx.TenantID); err != nil {
		return err
	}
	defer h.rateLimiter.Release(c.Context(), authCtx.TenantID.String())

	// Spending limit check
	if err := h.checkSpendingLimit(c, authCtx.TenantID); err != nil {
		return err
	}
	if err := h.checkWalletBalance(c, authCtx); err != nil {
		return err
	}

	var req gateway.ChatRequest
	if err := json.Unmarshal(c.Body(), &req); err != nil {
		return fiber.NewError(fiber.StatusBadRequest, "invalid request body")
	}

	if req.Model == "" {
		return fiber.NewError(fiber.StatusBadRequest, "model is required")
	}

	requestedModel := req.Model

	// Per-key model restriction
	if err := h.checkModelAccess(authCtx, requestedModel); err != nil {
		return err
	}

	// Guardrails: check messages before routing
	if h.guardrails != nil {
		texts := extractMessageTexts(req.Messages)
		result, err := h.guardrails.CheckMessages(c.Context(), authCtx.TenantID, texts, req.Model)
		if err != nil {
			slog.Error("guardrails check failed", "error", err)
		} else if result.Blocked {
			return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
				"error": fiber.Map{
					"message":    "Request blocked by content policy",
					"type":       "guardrail_violation",
					"code":       "content_policy_violation",
					"violations": result.Violations,
				},
			})
		} else if len(result.Redactions) > 0 {
			applyRedactions(req.Messages, result.Redactions)
		}
	}

	// Resolve all candidate routes for retry/fallback
	tenantID := &authCtx.TenantID
	routes, err := h.router.ResolveAll(c.Context(), req.Model, tenantID, req.MaxTokens)
	if err != nil {
		return err
	}

	if req.Stream {
		req.Model = routes[0].ExternalID // will be overwritten per-route in handleStreamWithRetry
		body, _ := json.Marshal(req)
		return h.handleStreamWithRetry(c, routes, &req, requestedModel, authCtx.TenantID, authCtx.WalletID, body)
	}

	// Check response cache for non-streaming requests
	cacheKey := gateway.GenerateKey(authCtx.TenantID.String(), &req)
	if h.cache != nil {
		if cached := h.cache.Get(c.Context(), cacheKey); cached != nil {
			c.Set("X-Cache", "HIT")
			if h.metrics != nil {
				h.metrics.ObserveCacheHit()
			}
			return c.JSON(cached)
		}
		if h.metrics != nil {
			h.metrics.ObserveCacheMiss()
		}
	}

	return h.handleNonStreamWithRetry(c, routes, &req, requestedModel, authCtx.TenantID, authCtx.WalletID, cacheKey)
}

func (h *GatewayHandlers) handleNonStreamWithRetry(c *fiber.Ctx, routes []*gateway.RouteResult, req *gateway.ChatRequest, requestedModel string, tenantID kernel.TenantID, walletID *kernel.WalletID, cacheKey string) error {
	maxAttempts := gateway.MaxRetries + 1
	if maxAttempts > len(routes) {
		maxAttempts = len(routes)
	}

	var lastErr error
	var lastStatus int

	for attempt := 0; attempt < maxAttempts; attempt++ {
		route := routes[attempt]

		// Rewrite model to this route's external ID
		req.Model = route.ExternalID
		body, _ := json.Marshal(req)

		start := time.Now()
		resp, statusCode, err := h.upstream.Call(c.Context(), route, body)
		duration := time.Since(start)

		if err != nil {
			h.healthTracker.ReportError(route.KeyID, statusCode)
			lastErr = err
			lastStatus = statusCode

			// Auth errors: permanently blacklist, try next route
			if gateway.IsAuthError(statusCode) {
				slog.Warn("auth error from provider, trying fallback",
					"provider", route.ProviderID, "key_id", route.KeyID, "status", statusCode, "attempt", attempt+1)
				if h.metrics != nil {
					h.metrics.ObserveRetry(route.ProviderID.String(), "auth_error")
					h.metrics.ObserveError(route.ProviderID.String(), fmt.Sprintf("%d", statusCode))
				}
				continue
			}

			// Retryable errors: try next route
			if gateway.IsRetryable(statusCode) && attempt < maxAttempts-1 {
				slog.Warn("retryable error from provider, trying fallback",
					"provider", route.ProviderID, "key_id", route.KeyID, "status", statusCode, "attempt", attempt+1)
				if h.metrics != nil {
					h.metrics.ObserveRetry(route.ProviderID.String(), fmt.Sprintf("http_%d", statusCode))
					h.metrics.ObserveError(route.ProviderID.String(), fmt.Sprintf("%d", statusCode))
				}
				time.Sleep(gateway.RetryDelay(attempt))
				continue
			}

			// Non-retryable error: fail immediately
			if h.metrics != nil {
				h.metrics.ObserveRequest(requestedModel, route.ProviderID.String(), "openai", "error", duration)
				h.metrics.ObserveError(route.ProviderID.String(), fmt.Sprintf("%d", statusCode))
			}
			h.usage.LogRequest(tenantID, route, requestedModel, nil, statusCode, duration, false, err, nil)
			return err
		}

		h.healthTracker.ReportSuccessWithLatency(route.KeyID, duration)

		// Debit tenant balance for token usage
		cost := calculateCost(route, resp)
		h.debitUsage(c.Context(), tenantID, walletID, cost)

		// Set retry headers if we retried
		if attempt > 0 {
			c.Set("X-Retry-Count", fmt.Sprintf("%d", attempt))
		}

		// Store in cache
		if h.cache != nil {
			c.Set("X-Cache", "MISS")
			h.cache.Set(c.Context(), cacheKey, resp)
		}

		if h.metrics != nil {
			h.metrics.ObserveRequest(requestedModel, route.ProviderID.String(), "openai", "ok", duration)
			if resp.Usage != nil {
				h.metrics.ObserveTokens(requestedModel, route.ProviderID.String(), resp.Usage.PromptTokens, resp.Usage.CompletionTokens)
			}
		}

		h.usage.LogRequest(tenantID, route, requestedModel, resp, http.StatusOK, duration, false, nil, buildRequestContent(c, req.Messages, resp, body))
		h.fireRequestWebhook(tenantID, route, requestedModel, resp, http.StatusOK, duration, nil)
		return c.JSON(resp)
	}

	// All retries exhausted
	h.usage.LogRequest(tenantID, routes[0], requestedModel, nil, lastStatus, 0, false, lastErr, nil)
	h.fireRequestWebhook(tenantID, routes[0], requestedModel, nil, lastStatus, 0, lastErr)
	return lastErr
}

func (h *GatewayHandlers) handleStreamWithRetry(c *fiber.Ctx, routes []*gateway.RouteResult, req *gateway.ChatRequest, requestedModel string, tenantID kernel.TenantID, walletID *kernel.WalletID, _ []byte) error {
	c.Set("Content-Type", "text/event-stream")
	c.Set("Cache-Control", "no-cache")
	c.Set("Connection", "keep-alive")
	c.Set("Transfer-Encoding", "chunked")

	// Capture debug mode + raw body before entering the async stream writer;
	// the fiber.Ctx is not safe to use once the handler returns.
	debugMode := isDebugMode(c)
	rawBody := append([]byte(nil), c.Body()...)

	c.Context().SetBodyStreamWriter(func(w *bufio.Writer) {
		streamCtx, streamCancel := context.WithTimeout(context.Background(), 5*time.Minute)
		defer streamCancel()

		maxAttempts := gateway.MaxRetries + 1
		if maxAttempts > len(routes) {
			maxAttempts = len(routes)
		}

		var lastErr error
		var lastStatus int

		for attempt := 0; attempt < maxAttempts; attempt++ {
			route := routes[attempt]

			// Rewrite model to this route's external ID
			req.Model = route.ExternalID
			body, _ := json.Marshal(req)

			start := time.Now()
			var lastChunk gateway.ChatStreamChunk

			upstreamStatus, streamErr := h.upstream.Stream(streamCtx, route, body, func(chunk []byte) error {
				_ = json.Unmarshal(chunk, &lastChunk)
				if _, err := fmt.Fprintf(w, "data: %s\n\n", chunk); err != nil {
					return err
				}
				return w.Flush()
			})

			duration := time.Since(start)

			if streamErr != nil {
				h.healthTracker.ReportError(route.KeyID, upstreamStatus)

				// If data was already written to client (200 OK, mid-stream failure), can't retry
				if upstreamStatus == http.StatusOK {
					errJSON, _ := json.Marshal(fiber.Map{"error": streamErr.Error()})
					fmt.Fprintf(w, "data: %s\n\n", errJSON)
					w.Flush()

					var resp *gateway.ChatResponse
					if lastChunk.Usage != nil {
						resp = &gateway.ChatResponse{Usage: lastChunk.Usage, Choices: lastChunk.Choices}
					}
					cost := calculateCost(route, resp)
					debitCtx, dc := context.WithTimeout(context.Background(), 5*time.Second)
					defer dc()
					h.debitUsage(debitCtx, tenantID, walletID, cost)
					h.usage.LogRequest(tenantID, route, requestedModel, resp, http.StatusBadGateway, duration, true, streamErr, nil)
					return
				}

				// Pre-stream error: can retry
				lastErr = streamErr
				lastStatus = upstreamStatus

				if gateway.IsAuthError(upstreamStatus) {
					slog.Warn("stream: auth error, trying fallback",
						"provider", route.ProviderID, "key_id", route.KeyID, "status", upstreamStatus, "attempt", attempt+1)
					if h.metrics != nil {
						h.metrics.ObserveRetry(route.ProviderID.String(), "auth_error")
						h.metrics.ObserveError(route.ProviderID.String(), fmt.Sprintf("%d", upstreamStatus))
					}
					continue
				}

				if gateway.IsRetryable(upstreamStatus) && attempt < maxAttempts-1 {
					slog.Warn("stream: retryable error, trying fallback",
						"provider", route.ProviderID, "key_id", route.KeyID, "status", upstreamStatus, "attempt", attempt+1)
					if h.metrics != nil {
						h.metrics.ObserveRetry(route.ProviderID.String(), fmt.Sprintf("http_%d", upstreamStatus))
						h.metrics.ObserveError(route.ProviderID.String(), fmt.Sprintf("%d", upstreamStatus))
					}
					time.Sleep(gateway.RetryDelay(attempt))
					continue
				}

				// Non-retryable pre-stream error
				errJSON, _ := json.Marshal(fiber.Map{"error": streamErr.Error()})
				fmt.Fprintf(w, "data: %s\n\n", errJSON)
				w.Flush()
				h.usage.LogRequest(tenantID, route, requestedModel, nil, upstreamStatus, duration, true, streamErr, nil)
				return
			}

			// Success
			h.healthTracker.ReportSuccessWithLatency(route.KeyID, duration)

			var resp *gateway.ChatResponse
			if lastChunk.Usage != nil {
				resp = &gateway.ChatResponse{Usage: lastChunk.Usage, Choices: lastChunk.Choices}
			}

			fmt.Fprintf(w, "data: [DONE]\n\n")
			w.Flush()

			cost := calculateCost(route, resp)
			debitCtx, dc := context.WithTimeout(context.Background(), 5*time.Second)
			defer dc()
			h.debitUsage(debitCtx, tenantID, walletID, cost)

			if h.metrics != nil {
				h.metrics.ObserveRequest(requestedModel, route.ProviderID.String(), "openai", "ok", duration)
				if resp != nil && resp.Usage != nil {
					h.metrics.ObserveTokens(requestedModel, route.ProviderID.String(), resp.Usage.PromptTokens, resp.Usage.CompletionTokens)
				}
			}

			h.usage.LogRequest(tenantID, route, requestedModel, resp, http.StatusOK, duration, true, nil, buildStreamRequestContent(debugMode, rawBody, req.Messages))
			h.fireRequestWebhook(tenantID, route, requestedModel, resp, http.StatusOK, duration, nil)
			return
		}

		// All attempts exhausted (the success path returns inside the loop)
		errJSON, _ := json.Marshal(fiber.Map{"error": fmt.Sprintf("all %d stream attempts failed: %v", maxAttempts, lastErr)})
		fmt.Fprintf(w, "data: %s\n\n", errJSON)
		w.Flush()
		h.usage.LogRequest(tenantID, routes[0], requestedModel, nil, lastStatus, 0, true, lastErr, nil)
		h.fireRequestWebhook(tenantID, routes[0], requestedModel, nil, lastStatus, 0, lastErr)
	})

	return nil
}

// calculateCost computes the total cost in USD for a request based on token
// counts and the mapping's per-million-token pricing.
func calculateCost(route *gateway.RouteResult, resp *gateway.ChatResponse) float64 {
	if resp == nil || resp.Usage == nil {
		return 0
	}
	var cost float64
	if route.InputPrice != nil {
		cost += float64(resp.Usage.PromptTokens) * *route.InputPrice / 1_000_000
	}
	if route.OutputPrice != nil {
		cost += float64(resp.Usage.CompletionTokens) * *route.OutputPrice / 1_000_000
	}
	return cost
}

// checkRateLimit checks both RPM and concurrency limits.
// Returns a 429 error with Retry-After if the tenant is rate limited.
func (h *GatewayHandlers) checkRateLimit(c *fiber.Ctx, tenantID kernel.TenantID) error {
	if h.rateLimiter == nil {
		return nil
	}
	result, err := h.rateLimiter.Check(c.Context(), tenantID.String())
	if err != nil {
		return nil // fail open
	}
	if result.Limit > 0 {
		c.Set("X-RateLimit-Limit", fmt.Sprintf("%d", result.Limit))
		c.Set("X-RateLimit-Remaining", fmt.Sprintf("%d", result.Remaining))
	}
	if !result.Allowed {
		c.Set("Retry-After", fmt.Sprintf("%d", int(result.RetryAfter.Seconds()+1)))
		if h.metrics != nil {
			h.metrics.ObserveRateLimit("rpm_or_concurrency")
		}
		return c.Status(fiber.StatusTooManyRequests).JSON(fiber.Map{
			"error": fiber.Map{
				"message": "Rate limit exceeded",
				"type":    "rate_limit_error",
				"code":    "rate_limit_exceeded",
			},
		})
	}
	return nil
}

// debitUsage charges a request cost to the wallet bound to the API key, or to
// the tenant main balance when no wallet is bound. Failures are logged, not fatal.
func (h *GatewayHandlers) debitUsage(ctx context.Context, tenantID kernel.TenantID, walletID *kernel.WalletID, cost float64) {
	if cost <= 0 {
		return
	}
	if walletID != nil && !walletID.IsEmpty() && h.wallet != nil {
		if _, err := h.wallet.DebitUsage(ctx, tenantID, *walletID, cost, ""); err != nil {
			slog.Error("failed to debit wallet usage", "tenant_id", tenantID, "wallet_id", *walletID, "cost", cost, "error", err)
		}
		return
	}
	if _, err := h.billing.DebitUsage(ctx, tenantID, cost, ""); err != nil {
		slog.Error("failed to debit usage", "tenant_id", tenantID, "cost", cost, "error", err)
	}
}

// checkWalletBalance rejects the request with 402 when the API key is bound to
// a wallet that has no funds left.
func (h *GatewayHandlers) checkWalletBalance(c *fiber.Ctx, authCtx *kernel.AuthContext) error {
	if authCtx.WalletID == nil || authCtx.WalletID.IsEmpty() || h.wallet == nil {
		return nil
	}
	ok, err := h.wallet.HasSufficientFunds(c.Context(), authCtx.TenantID, *authCtx.WalletID)
	if err != nil {
		return nil // fail open, like checkSpendingLimit
	}
	if !ok {
		return c.Status(fiber.StatusPaymentRequired).JSON(fiber.Map{
			"error": fiber.Map{
				"message": "Wallet has insufficient funds",
				"type":    "insufficient_funds_error",
				"code":    "wallet_insufficient_funds",
			},
		})
	}
	return nil
}

// checkSpendingLimit checks daily/monthly spending caps for the tenant.
func (h *GatewayHandlers) checkSpendingLimit(c *fiber.Ctx, tenantID kernel.TenantID) error {
	if h.billing == nil {
		return nil
	}
	result, err := h.billing.CheckSpendingLimit(c.Context(), tenantID)
	if err != nil {
		return nil // fail open
	}
	if !result.Allowed {
		return c.Status(fiber.StatusPaymentRequired).JSON(fiber.Map{
			"error": fiber.Map{
				"message": result.Reason,
				"type":    "spending_limit_error",
				"code":    "spending_limit_exceeded",
			},
		})
	}
	return nil
}

// ===========================================================================
// Rate Limit Config CRUD
// ===========================================================================

func (h *GatewayHandlers) GetRateLimitConfig(c *fiber.Ctx) error {
	tenantID := kernel.NewTenantID(c.Params("tenantId"))

	cfg, err := h.rateLimiter.GetConfig(c.Context(), tenantID)
	if err != nil {
		return fiber.NewError(fiber.StatusInternalServerError, "failed to get rate limit config")
	}
	if cfg == nil {
		// Return defaults if no custom config
		defaults := gateway.DefaultRateLimitConfig()
		defaults.TenantID = tenantID
		return c.JSON(defaults)
	}
	return c.JSON(cfg)
}

func (h *GatewayHandlers) UpsertRateLimitConfig(c *fiber.Ctx) error {
	tenantID := kernel.NewTenantID(c.Params("tenantId"))

	var req gateway.UpsertRateLimitRequest
	if err := c.BodyParser(&req); err != nil {
		return fiber.NewError(fiber.StatusBadRequest, "invalid request body")
	}

	// Build config with defaults for unset fields
	defaults := gateway.DefaultRateLimitConfig()
	cfg := &gateway.RateLimitConfig{
		TenantID:      tenantID,
		RPM:           defaults.RPM,
		MaxConcurrent: defaults.MaxConcurrent,
	}
	if req.RPM != nil {
		cfg.RPM = *req.RPM
	}
	if req.MaxConcurrent != nil {
		cfg.MaxConcurrent = *req.MaxConcurrent
	}

	saved, err := h.rateLimiter.UpsertConfig(c.Context(), cfg)
	if err != nil {
		return fiber.NewError(fiber.StatusInternalServerError, "failed to save rate limit config")
	}

	return c.JSON(saved)
}

func (h *GatewayHandlers) DeleteRateLimitConfig(c *fiber.Ctx) error {
	tenantID := kernel.NewTenantID(c.Params("tenantId"))

	if err := h.rateLimiter.DeleteConfig(c.Context(), tenantID); err != nil {
		return fiber.NewError(fiber.StatusInternalServerError, "failed to delete rate limit config")
	}

	return c.JSON(fiber.Map{"message": "Rate limit config deleted, tenant will use defaults"})
}

// ===========================================================================
// Cache Invalidation
// ===========================================================================

func (h *GatewayHandlers) InvalidateCacheAll(c *fiber.Ctx) error {
	if h.cache == nil {
		return c.JSON(fiber.Map{"message": "cache not configured", "keys_deleted": 0})
	}
	deleted, err := h.cache.InvalidateAll(c.Context())
	if err != nil {
		return fiber.NewError(fiber.StatusInternalServerError, "failed to invalidate cache")
	}
	return c.JSON(fiber.Map{"message": "all cache invalidated", "keys_deleted": deleted})
}

func (h *GatewayHandlers) InvalidateCacheTenant(c *fiber.Ctx) error {
	if h.cache == nil {
		return c.JSON(fiber.Map{"message": "cache not configured", "keys_deleted": 0})
	}
	tenantID := c.Params("tenantId")
	deleted, err := h.cache.InvalidateByTenant(c.Context(), tenantID)
	if err != nil {
		return fiber.NewError(fiber.StatusInternalServerError, "failed to invalidate cache")
	}
	return c.JSON(fiber.Map{
		"message":      fmt.Sprintf("cache invalidated for tenant %s", tenantID),
		"keys_deleted": deleted,
	})
}

// ===========================================================================
// Routing Config CRUD
// ===========================================================================

func (h *GatewayHandlers) GetRoutingConfig(c *fiber.Ctx) error {
	if h.routingRepo == nil {
		return c.JSON(gateway.RoutingConfig{
			TenantID: kernel.NewTenantID(c.Params("tenantId")),
			Strategy: gateway.StrategyCheapest,
		})
	}

	tenantID := kernel.NewTenantID(c.Params("tenantId"))
	cfg, err := h.routingRepo.GetByTenantID(c.Context(), tenantID)
	if err != nil {
		return fiber.NewError(fiber.StatusInternalServerError, "failed to get routing config")
	}
	if cfg == nil {
		return c.JSON(gateway.RoutingConfig{
			TenantID: tenantID,
			Strategy: gateway.StrategyCheapest,
		})
	}
	return c.JSON(cfg)
}

func (h *GatewayHandlers) UpsertRoutingConfig(c *fiber.Ctx) error {
	if h.routingRepo == nil {
		return fiber.NewError(fiber.StatusServiceUnavailable, "routing config not available")
	}

	tenantID := kernel.NewTenantID(c.Params("tenantId"))

	var req gateway.UpsertRoutingConfigRequest
	if err := c.BodyParser(&req); err != nil {
		return fiber.NewError(fiber.StatusBadRequest, "invalid request body")
	}

	strategy := gateway.RoutingStrategy(req.Strategy)
	if !strategy.IsValid() {
		return fiber.NewError(fiber.StatusBadRequest, fmt.Sprintf(
			"invalid strategy %q, must be one of: cheapest, lowest-latency, round-robin", req.Strategy,
		))
	}

	cfg := &gateway.RoutingConfig{
		TenantID: tenantID,
		Strategy: strategy,
	}

	saved, err := h.routingRepo.Upsert(c.Context(), cfg)
	if err != nil {
		return fiber.NewError(fiber.StatusInternalServerError, "failed to save routing config")
	}

	// Invalidate the router's cached strategy for this tenant
	h.router.InvalidateStrategyCache(tenantID.String())

	return c.JSON(saved)
}

func (h *GatewayHandlers) DeleteRoutingConfig(c *fiber.Ctx) error {
	if h.routingRepo == nil {
		return fiber.NewError(fiber.StatusServiceUnavailable, "routing config not available")
	}

	tenantID := kernel.NewTenantID(c.Params("tenantId"))

	if err := h.routingRepo.Delete(c.Context(), tenantID); err != nil {
		return fiber.NewError(fiber.StatusInternalServerError, "failed to delete routing config")
	}

	h.router.InvalidateStrategyCache(tenantID.String())

	return c.JSON(fiber.Map{"message": "Routing config deleted, tenant will use default (cheapest)"})
}

// fireRequestWebhook fires request.completed or request.failed events.
func (h *GatewayHandlers) fireRequestWebhook(tenantID kernel.TenantID, route *gateway.RouteResult, requestedModel string, resp *gateway.ChatResponse, statusCode int, duration time.Duration, err error) {
	if h.webhooks == nil {
		return
	}

	data := fiber.Map{
		"model":       requestedModel,
		"provider":    route.ProviderID.String(),
		"status_code": statusCode,
		"duration_ms": duration.Milliseconds(),
	}
	if resp != nil && resp.Usage != nil {
		data["input_tokens"] = resp.Usage.PromptTokens
		data["output_tokens"] = resp.Usage.CompletionTokens
		data["total_tokens"] = resp.Usage.TotalTokens
	}
	cost := calculateCost(route, resp)
	if cost > 0 {
		data["cost_usd"] = cost
	}

	if err != nil {
		data["error"] = err.Error()
		h.webhooks.Fire(tenantID, webhook.EventRequestFailed, data)
	} else {
		h.webhooks.Fire(tenantID, webhook.EventRequestCompleted, data)
	}
}

// extractMessageTexts extracts text content from ChatRequest messages.
func extractMessageTexts(messages []gateway.Message) []string {
	texts := make([]string, 0, len(messages))
	for _, m := range messages {
		switch v := m.Content.(type) {
		case string:
			texts = append(texts, v)
		}
	}
	return texts
}

// applyRedactions applies PII/secrets redactions to message content in place.
func applyRedactions(messages []gateway.Message, redactions []guardrails.RedactionInfo) {
	for _, r := range redactions {
		if r.MessageIndex >= len(messages) {
			continue
		}
		msg := &messages[r.MessageIndex]
		text, ok := msg.Content.(string)
		if !ok {
			continue
		}
		// Run the detectors again to get match positions for replacement
		piiMatches := guardrails.CheckPII(text)
		secretMatches := guardrails.CheckSecrets(text)
		allMatches := append(piiMatches, secretMatches...)
		if len(allMatches) > 0 {
			msg.Content = guardrails.ApplyRedactions(text, allMatches)
		}
	}
}
