package gatewayhttp

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"time"

	"github.com/Abraxas-365/freerouter/internal/errx"
	"github.com/Abraxas-365/freerouter/internal/gateway"
	"github.com/Abraxas-365/freerouter/internal/guardrail"
	"github.com/Abraxas-365/freerouter/internal/identity"
	"github.com/Abraxas-365/freerouter/internal/provider"
	"github.com/Abraxas-365/freerouter/internal/providerkey"
	"github.com/Abraxas-365/freerouter/internal/query"
	"github.com/Abraxas-365/freerouter/internal/ratelimit/ratelimitsvc"
	"github.com/Abraxas-365/freerouter/internal/routingconfig"
	"github.com/Abraxas-365/freerouter/internal/server"
	"github.com/Abraxas-365/freerouter/internal/usage"
	"github.com/Abraxas-365/freerouter/internal/webhook"
	"github.com/gofiber/fiber/v2"
)

// Handler serves OpenAI-compatible gateway endpoints.
type Handler struct {
	router          *gateway.Router
	upstream        *gateway.Upstream
	healthTracker   *gateway.KeyHealthTracker
	modelQueries    provider.ModelQueries
	usageLogger     usage.Commands         // nil = no usage logging
	rateLimiter     *ratelimitsvc.Service  // nil = no rate limiting
	guardrails      guardrail.Evaluator    // nil = no guardrail checks
	webhooks        webhook.Dispatcher     // nil = no webhook events
	tokenRefresher  OAuthTokenRefresher    // nil = no OAuth refresh
	routingResolver routingconfig.Resolver // nil = always use gateway.StrategyCheapest
	cache           *gateway.ResponseCache // nil = no response caching
	metrics         *gateway.Metrics       // nil = no Prometheus metrics
}

// OAuthTokenRefresher can force-refresh an OAuth key and return new credential.
type OAuthTokenRefresher interface {
	ForceRefresh(ctx context.Context, keyID identity.ProviderKeyID) (providerkey.Credential, error)
}

// New creates a gateway HTTP handler.
func New(
	router *gateway.Router,
	upstream *gateway.Upstream,
	healthTracker *gateway.KeyHealthTracker,
	modelQueries provider.ModelQueries,
	usageLogger usage.Commands,
	rateLimiter *ratelimitsvc.Service,
	guardrails guardrail.Evaluator,
	webhooks webhook.Dispatcher,
	tokenRefresher OAuthTokenRefresher,
	routingResolver routingconfig.Resolver,
	cache *gateway.ResponseCache,
	metrics *gateway.Metrics,
) *Handler {
	return &Handler{
		router:          router,
		upstream:        upstream,
		healthTracker:   healthTracker,
		modelQueries:    modelQueries,
		usageLogger:     usageLogger,
		rateLimiter:     rateLimiter,
		guardrails:      guardrails,
		webhooks:        webhooks,
		tokenRefresher:  tokenRefresher,
		routingResolver: routingResolver,
		cache:           cache,
		metrics:         metrics,
	}
}

// RegisterRoutes mounts gateway endpoints on the given router group.
// Expected to be mounted at /v1.
func (h *Handler) RegisterRoutes(r fiber.Router) {
	r.Post("/chat/completions", h.ChatCompletions)
	r.Post("/messages", h.AnthropicMessages)
	r.Post("/responses", h.Responses)
	r.Get("/models", h.ListModels)
	r.Post("/cost/estimate", h.EstimateCost)
}

// RegisterAdminRoutes mounts operator-facing gateway management endpoints.
// Expected to be mounted at /api/v1/gateway.
func (h *Handler) RegisterAdminRoutes(r fiber.Router) {
	r.Delete("/cache", server.RequirePermissions(server.PermGatewayWrite), h.InvalidateCache)
}

// InvalidateCache clears cached responses. Pass ?subject=<id> to scope the
// invalidation to a single subject; otherwise every cached response is
// cleared.
func (h *Handler) InvalidateCache(c *fiber.Ctx) error {
	if h.cache == nil {
		return c.JSON(fiber.Map{"invalidated": 0})
	}

	var (
		n   int64
		err error
	)
	if subject := c.Query("subject"); subject != "" {
		n, err = h.cache.InvalidateBySubject(c.Context(), subject)
	} else {
		n, err = h.cache.InvalidateAll(c.Context())
	}
	if err != nil {
		return errx.Internal("failed to invalidate cache: " + err.Error())
	}
	return c.JSON(fiber.Map{"invalidated": n})
}

// ── Chat Completions ─────────────────────────────────────────────────

func (h *Handler) ChatCompletions(c *fiber.Ctx) error {
	if h.metrics != nil {
		h.metrics.InFlightRequests.WithLabelValues(string(gateway.ProtocolOpenAI)).Inc()
		defer h.metrics.InFlightRequests.WithLabelValues(string(gateway.ProtocolOpenAI)).Dec()
	}

	// Rate limit check
	subjectID, release, err := h.checkRateLimit(c)
	if err != nil {
		return err
	}
	if release != nil {
		defer release()
	}

	var req gateway.ChatRequest
	if err := c.BodyParser(&req); err != nil {
		return errx.Validation("invalid request body: " + err.Error())
	}
	if req.Model == "" {
		return errx.Validation("model is required")
	}
	if len(req.Messages) == 0 {
		return errx.Validation("messages is required")
	}

	// Guardrails: check messages before routing
	if h.guardrails != nil {
		texts := extractMessageTexts(req.Messages)
		result, err := h.guardrails.CheckMessages(c.Context(), texts, req.Model)
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
			rawBody, _ := json.Marshal(req)
			c.Request().SetBody(rawBody)
		}
	}

	// Response cache: only applies to non-streaming requests. Skips routing
	// entirely on a hit.
	var cacheKey string
	if !req.Stream && h.cache != nil {
		cacheKey = gateway.GenerateKey(subjectID, &req)
		if cached := h.cache.Get(c.Context(), cacheKey); cached != nil {
			c.Set("X-Cache", "HIT")
			if h.metrics != nil {
				h.metrics.ObserveCacheHit()
			}
			return c.JSON(cached)
		}
		c.Set("X-Cache", "MISS")
		if h.metrics != nil {
			h.metrics.ObserveCacheMiss()
		}
	}

	// Resolve all routes (primary + fallbacks), ordered by the subject's
	// configured routing strategy (falls back to StrategyCheapest).
	strategy := h.resolveStrategy(c.Context(), subjectID)
	routes, err := h.router.ResolveAll(c.Context(), req.Model, strategy)
	if err != nil {
		return err
	}
	if len(routes) == 0 {
		return errx.NotFound("no available route for model: " + req.Model)
	}

	rawBody := c.Body()

	if req.Stream {
		return h.streamWithRetry(c, routes, rawBody, req.Model)
	}
	return h.callWithRetry(c, routes, rawBody, req.Model, cacheKey)
}

// callWithRetry tries routes in order for non-streaming requests. On success,
// if cacheKey is non-empty, the response is stored in the response cache.
func (h *Handler) callWithRetry(c *fiber.Ctx, routes []*gateway.RouteResult, rawBody []byte, modelName string, cacheKey string) error {
	var lastErr error
	var lastStatus int

	for attempt, route := range routes {
		if attempt > 0 {
			delay := gateway.RetryDelay(attempt - 1)
			time.Sleep(delay)
		}

		start := time.Now()
		resp, status, err := h.upstream.Call(c.Context(), route, rawBody)
		latency := time.Since(start)

		if err == nil {
			h.healthTracker.ReportSuccessWithLatency(route.KeyID, latency)

			// Override model field with the canonical name
			resp.Model = modelName

			// Log usage (non-blocking)
			h.logUsage(route, modelName, resp, http.StatusOK, latency, false, nil)

			if cacheKey != "" && h.cache != nil {
				h.cache.Set(c.Context(), cacheKey, resp)
			}

			if h.metrics != nil {
				h.metrics.ObserveRequest(modelName, route.ProviderID.String(), gateway.ProtocolOpenAI, gateway.StatusOK, latency)
				if resp.Usage != nil {
					h.metrics.ObserveTokens(modelName, route.ProviderID.String(), resp.Usage.PromptTokens, resp.Usage.CompletionTokens)
				}
			}

			return c.JSON(resp)
		}

		lastErr = err
		lastStatus = status

		// Log failed attempt
		h.logUsage(route, modelName, nil, status, latency, false, err)

		// Report health
		if gateway.IsAuthError(status) {
			// Try OAuth refresh before blacklisting.
			if h.tokenRefresher != nil {
				cred, refreshErr := h.tokenRefresher.ForceRefresh(c.Context(), route.KeyID)
				if refreshErr == nil && cred.OAuth != nil {
					// Retry once with the new token.
					route.Token = cred.OAuth.AccessToken
					retryStart := time.Now()
					retryResp, retryStatus, retryErr := h.upstream.Call(c.Context(), route, rawBody)
					retryLatency := time.Since(retryStart)
					if retryErr == nil {
						h.healthTracker.ReportSuccessWithLatency(route.KeyID, retryLatency)
						retryResp.Model = modelName
						h.logUsage(route, modelName, retryResp, http.StatusOK, retryLatency, false, nil)
						if cacheKey != "" && h.cache != nil {
							h.cache.Set(c.Context(), cacheKey, retryResp)
						}
						if h.metrics != nil {
							h.metrics.ObserveRequest(modelName, route.ProviderID.String(), gateway.ProtocolOpenAI, gateway.StatusOK, retryLatency)
							if retryResp.Usage != nil {
								h.metrics.ObserveTokens(modelName, route.ProviderID.String(), retryResp.Usage.PromptTokens, retryResp.Usage.CompletionTokens)
							}
						}
						return c.JSON(retryResp)
					}
					lastErr = retryErr
					lastStatus = retryStatus
					h.logUsage(route, modelName, nil, retryStatus, retryLatency, false, retryErr)
					if h.metrics != nil {
						h.metrics.ObserveError(route.ProviderID.String(), fmt.Sprintf("%d", retryStatus))
					}
				}
			}
			h.healthTracker.ReportError(route.KeyID, status)
			slog.Warn("auth error, blacklisting key",
				"key_id", route.KeyID,
				"provider", route.ProviderID,
				"status", status)
			h.fireKeyHealthWebhook(route, status)
			if h.metrics != nil {
				h.metrics.ObserveRetry(route.ProviderID.String(), "auth_error")
				h.metrics.ObserveError(route.ProviderID.String(), fmt.Sprintf("%d", status))
			}
			continue
		}

		if gateway.IsRetryable(status) {
			h.healthTracker.ReportError(route.KeyID, status)
			slog.Warn("retryable error, trying next route",
				"key_id", route.KeyID,
				"provider", route.ProviderID,
				"status", status,
				"attempt", attempt+1)
			h.fireKeyHealthWebhook(route, status)
			if h.metrics != nil {
				h.metrics.ObserveRetry(route.ProviderID.String(), fmt.Sprintf("http_%d", status))
				h.metrics.ObserveError(route.ProviderID.String(), fmt.Sprintf("%d", status))
			}
			continue
		}

		// Non-retryable error
		h.healthTracker.ReportError(route.KeyID, status)
		h.fireKeyHealthWebhook(route, status)
		if h.metrics != nil {
			h.metrics.ObserveRequest(modelName, route.ProviderID.String(), gateway.ProtocolOpenAI, gateway.StatusError, latency)
			h.metrics.ObserveError(route.ProviderID.String(), fmt.Sprintf("%d", status))
		}
		break
	}

	if lastErr != nil {
		return lastErr
	}
	return errx.New(
		fmt.Sprintf("all routes exhausted, last status: %d", lastStatus),
		errx.TypeExternal,
	)
}

// streamWithRetry tries routes in order for streaming requests.
func (h *Handler) streamWithRetry(c *fiber.Ctx, routes []*gateway.RouteResult, rawBody []byte, modelName string) error {
	var lastErr error

	for attempt, route := range routes {
		if attempt > 0 {
			delay := gateway.RetryDelay(attempt - 1)
			time.Sleep(delay)
		}

		err := h.doStream(c, route, rawBody, modelName)
		if err == nil {
			return nil
		}

		lastErr = err

		// Check if we've already started writing to the client
		if c.Response().StatusCode() == fiber.StatusOK {
			// Can't retry — headers already sent
			return nil
		}

		slog.Warn("stream attempt failed, trying next route",
			"key_id", route.KeyID,
			"provider", route.ProviderID,
			"attempt", attempt+1)
	}

	if lastErr != nil {
		return lastErr
	}
	return errx.New("all routes exhausted for streaming", errx.TypeExternal)
}

func (h *Handler) doStream(c *fiber.Ctx, route *gateway.RouteResult, rawBody []byte, modelName string) error {
	c.Set("Content-Type", "text/event-stream")
	c.Set("Cache-Control", "no-cache")
	c.Set("Connection", "keep-alive")
	c.Set("Transfer-Encoding", "chunked")

	ctx := c.Context()
	start := time.Now()

	status, err := h.upstream.Stream(c.Context(), route, rawBody, func(chunk []byte) error {
		_, writeErr := ctx.Write(chunk)
		if writeErr != nil {
			return writeErr
		}
		return nil
	})

	latency := time.Since(start)

	if err != nil {
		if gateway.IsAuthError(status) || gateway.IsRetryable(status) {
			h.healthTracker.ReportError(route.KeyID, status)
			h.fireKeyHealthWebhook(route, status)
		}
		h.logUsage(route, modelName, nil, status, latency, true, err)
		if h.metrics != nil {
			h.metrics.ObserveRequest(modelName, route.ProviderID.String(), gateway.ProtocolOpenAI, gateway.StatusError, latency)
			h.metrics.ObserveError(route.ProviderID.String(), fmt.Sprintf("%d", status))
		}
		return err
	}

	h.healthTracker.ReportSuccessWithLatency(route.KeyID, latency)
	h.logUsage(route, modelName, nil, http.StatusOK, latency, true, nil)
	if h.metrics != nil {
		h.metrics.ObserveRequest(modelName, route.ProviderID.String(), gateway.ProtocolOpenAI, gateway.StatusOK, latency)
	}
	return nil
}

// ── List Models ──────────────────────────────────────────────────────

func (h *Handler) ListModels(c *fiber.Ctx) error {
	// List all active models
	activeStatus := provider.ModelStatusActive
	filter := provider.ModelFilter{Status: &activeStatus}
	page := query.Pagination{Limit: 1000, Offset: 0}

	result, err := h.modelQueries.ListModels(c.Context(), filter, page)
	if err != nil {
		return err
	}

	var models []gateway.ModelObject
	for _, m := range result.Items {
		models = append(models, gateway.NewModelObject(m.Name, m.Family, m.CreatedAt))
	}

	if models == nil {
		models = []gateway.ModelObject{}
	}

	return c.JSON(gateway.ModelList{
		Object: "list",
		Data:   models,
	})
}

// EstimateCost returns a pre-flight, informational cost estimate for a chat
// request based on the resolved route's pricing and a rough token-count
// estimate. It performs no billing side effects.
func (h *Handler) EstimateCost(c *fiber.Ctx) error {
	var req gateway.CostEstimateRequest
	if err := c.BodyParser(&req); err != nil {
		return errx.Validation("invalid request body")
	}
	if req.Model == "" {
		return errx.Validation("model is required")
	}

	route, err := h.router.Resolve(c.Context(), req.Model)
	if err != nil {
		return err
	}

	inputTokens := gateway.EstimateMessageTokens(req.Messages)
	maxOutput := gateway.DefaultEstimateMaxOutputTokens
	if req.MaxTokens != nil && *req.MaxTokens > 0 {
		maxOutput = *req.MaxTokens
	}

	inputCost := gateway.CalculateChatCost(route, inputTokens, 0)
	outputCost := gateway.CalculateChatCost(route, 0, maxOutput)

	return c.JSON(gateway.CostEstimateResponse{
		Model:                 req.Model,
		EstimatedInputTokens:  inputTokens,
		EstimatedOutputTokens: maxOutput,
		InputCostUSD:          inputCost,
		OutputCostUSD:         outputCost,
		TotalCostUSD:          inputCost + outputCost,
	})
}

// ── Modality Endpoints ───────────────────────────────────────────────

// RegisterModalityRoutes mounts optional modality endpoints on the given group.
// These are passthrough endpoints that proxy to upstream providers.
func (h *Handler) RegisterModalityRoutes(r fiber.Router) {
	r.Post("/embeddings", h.Embeddings)
	r.Post("/audio/transcriptions", h.Transcription)
	r.Post("/audio/speech", h.Speech)
	r.Post("/moderations", h.Moderation)
	r.Post("/rerank", h.Rerank)
	r.Post("/images/generations", h.ImageGeneration)
}

func (h *Handler) Embeddings(c *fiber.Ctx) error {
	return h.passthroughEndpoint(c, gateway.EndpointEmbeddings)
}

func (h *Handler) Transcription(c *fiber.Ctx) error {
	return h.passthroughEndpoint(c, gateway.EndpointTranscription)
}

func (h *Handler) Speech(c *fiber.Ctx) error {
	return h.passthroughEndpoint(c, gateway.EndpointSpeech)
}

func (h *Handler) Moderation(c *fiber.Ctx) error {
	return h.passthroughEndpoint(c, gateway.EndpointModeration)
}

func (h *Handler) Rerank(c *fiber.Ctx) error {
	return h.passthroughEndpoint(c, gateway.EndpointRerank)
}

func (h *Handler) ImageGeneration(c *fiber.Ctx) error {
	return h.passthroughEndpoint(c, gateway.EndpointImages)
}

// passthroughEndpoint resolves a route and proxies the request body to the upstream.
func (h *Handler) passthroughEndpoint(c *fiber.Ctx, endpoint gateway.Endpoint) error {
	// Rate limit check
	_, release, err := h.checkRateLimit(c)
	if err != nil {
		return err
	}
	if release != nil {
		defer release()
	}

	// Extract model from body
	modelName, err := extractModelFromBody(c.Body())
	if err != nil {
		return err
	}

	route, err := h.router.Resolve(c.Context(), modelName)
	if err != nil {
		return err
	}

	start := time.Now()
	respBody, status, callErr := h.upstream.CallRawWithEndpoint(c.Context(), route, c.Body(), endpoint)
	latency := time.Since(start)

	if callErr != nil {
		h.healthTracker.ReportError(route.KeyID, status)
		return callErr
	}

	h.healthTracker.ReportSuccessWithLatency(route.KeyID, latency)
	c.Set("Content-Type", "application/json")
	return c.Send(respBody)
}

func extractModelFromBody(body []byte) (string, error) {
	var raw struct {
		Model string `json:"model"`
	}
	if err := json.Unmarshal(body, &raw); err != nil {
		return "", errx.Validation("invalid request body")
	}
	if raw.Model == "" {
		return "", errx.Validation("model is required")
	}
	return raw.Model, nil
}

// extractMessageTexts extracts string content from ChatRequest messages for
// guardrail evaluation. Non-string content (e.g. multimodal parts) is skipped.
func extractMessageTexts(messages []gateway.Message) []string {
	texts := make([]string, 0, len(messages))
	for _, m := range messages {
		if v, ok := m.Content.(string); ok {
			texts = append(texts, v)
		}
	}
	return texts
}

// applyRedactions applies PII/secrets redactions to message content in place.
func applyRedactions(messages []gateway.Message, redactions []guardrail.RedactionInfo) {
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
		piiMatches := guardrail.CheckPII(text)
		secretMatches := guardrail.CheckSecrets(text)
		allMatches := append(piiMatches, secretMatches...)
		if len(allMatches) > 0 {
			msg.Content = guardrail.ApplyRedactions(text, allMatches)
		}
	}
}

// checkRateLimit enforces RPM + concurrency limits for the authenticated subject.
// Returns the subject ID, a release function (for concurrency slot), and any error.
// The release function MUST be called when the request completes if non-nil.
func (h *Handler) checkRateLimit(c *fiber.Ctx) (string, func(), error) {
	subjectID := subjectFromClaims(c)

	if h.rateLimiter == nil {
		return subjectID, nil, nil
	}

	result, err := h.rateLimiter.Check(c.Context(), subjectID)
	if err != nil {
		slog.Error("rate limit check failed", "subject", subjectID, "error", err)
		// Fail open: allow the request but log the error
		return subjectID, nil, nil
	}

	// Set rate limit headers
	c.Set("X-RateLimit-Limit", fmt.Sprintf("%d", result.Limit))
	c.Set("X-RateLimit-Remaining", fmt.Sprintf("%d", result.Remaining))

	if !result.Allowed {
		if result.RetryAfter > 0 {
			c.Set("Retry-After", fmt.Sprintf("%d", int(result.RetryAfter.Seconds())))
		}
		if h.metrics != nil {
			h.metrics.ObserveRateLimit("rpm_or_concurrency")
		}
		return subjectID, nil, errx.RateLimited("rate limit exceeded")
	}

	release := func() {
		h.rateLimiter.Release(c.Context(), subjectID)
	}
	return subjectID, release, nil
}

// subjectFromClaims extracts the authenticated subject ID from JWT claims,
// falling back to "anonymous" when no claims are present.
func subjectFromClaims(c *fiber.Ctx) string {
	claims := server.Claims(c)
	if claims != nil && claims.Subject != "" {
		return claims.Subject
	}
	return "anonymous"
}

// resolveStrategy returns the effective routing strategy for a subject.
// Falls back to gateway.StrategyCheapest when no resolver is configured.
func (h *Handler) resolveStrategy(ctx context.Context, subjectID string) gateway.RoutingStrategy {
	if h.routingResolver == nil {
		return gateway.StrategyCheapest
	}
	return gateway.RoutingStrategy(h.routingResolver.Resolve(ctx, subjectID))
}

// logUsage builds a usage log entry (enqueued non-blocking if usage logging
// is enabled) and fires a request.completed/request.failed webhook event.
func (h *Handler) logUsage(
	route *gateway.RouteResult,
	requestedModel string,
	resp *gateway.ChatResponse,
	statusCode int,
	duration time.Duration,
	streamed bool,
	reqErr error,
) {
	log := usage.UsageLog{
		ID:             identity.NewUsageLogID(),
		KeyID:          route.KeyID,
		RequestedModel: requestedModel,
		UsedModel:      route.ExternalID,
		ProviderID:     route.ProviderID,
		MappingID:      route.MappingID,
		DurationMs:     int(duration.Milliseconds()),
		Streamed:       streamed,
		StatusCode:     statusCode,
		IsFallback:     route.IsFallback,
		CreatedAt:      time.Now().UTC(),
	}

	if resp != nil && resp.Usage != nil {
		log.PromptTokens = resp.Usage.PromptTokens
		log.CompletionTokens = resp.Usage.CompletionTokens
		log.TotalTokens = resp.Usage.TotalTokens
		log.CachedTokens = resp.Usage.CacheReadInputTokens

		if route.InputPrice != nil {
			log.InputCost = float64(log.PromptTokens) * *route.InputPrice / 1_000_000
		}
		if route.OutputPrice != nil {
			log.OutputCost = float64(log.CompletionTokens) * *route.OutputPrice / 1_000_000
		}
		log.TotalCost = log.InputCost + log.OutputCost

		if len(resp.Choices) > 0 && resp.Choices[0].FinishReason != nil {
			log.FinishReason = *resp.Choices[0].FinishReason
		}
	}

	if reqErr != nil {
		log.HasError = true
		log.ErrorMessage = reqErr.Error()
	}

	if h.usageLogger != nil {
		h.usageLogger.LogRequest(log)
	}

	h.fireRequestWebhook(route, requestedModel, log, statusCode, reqErr)
}

// fireKeyHealthWebhook emits key.blacklisted (permanent auth failure) or
// key.health_degraded (other reported errors) after a routing failure.
func (h *Handler) fireKeyHealthWebhook(route *gateway.RouteResult, status int) {
	if h.webhooks == nil {
		return
	}

	data := fiber.Map{
		"key_id":      route.KeyID.String(),
		"provider":    route.ProviderID.String(),
		"status_code": status,
	}

	metrics := h.healthTracker.GetMetrics(route.KeyID)
	if metrics.PermanentlyBlacklisted {
		h.webhooks.Fire(webhook.EventKeyBlacklisted, data)
		return
	}
	if metrics.ConsecutiveErrors >= 3 {
		h.webhooks.Fire(webhook.EventKeyHealthDegraded, data)
	}
}

// fireRequestWebhook emits request.completed or request.failed.
func (h *Handler) fireRequestWebhook(route *gateway.RouteResult, requestedModel string, log usage.UsageLog, statusCode int, reqErr error) {
	if h.webhooks == nil {
		return
	}

	data := fiber.Map{
		"model":       requestedModel,
		"provider":    route.ProviderID.String(),
		"status_code": statusCode,
		"duration_ms": log.DurationMs,
	}
	if log.TotalTokens > 0 {
		data["input_tokens"] = log.PromptTokens
		data["output_tokens"] = log.CompletionTokens
		data["total_tokens"] = log.TotalTokens
	}
	if log.TotalCost > 0 {
		data["cost_usd"] = log.TotalCost
	}

	if reqErr != nil {
		data["error"] = reqErr.Error()
		h.webhooks.Fire(webhook.EventRequestFailed, data)
	} else {
		h.webhooks.Fire(webhook.EventRequestCompleted, data)
	}
}
