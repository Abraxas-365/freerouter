package gatewayapi

import (
	"bufio"
	"encoding/json"
	"fmt"
	"time"

	"github.com/Abraxas-365/freerouter/internal/ai/gateway"
	"github.com/Abraxas-365/freerouter/internal/iam/auth"
	"github.com/gofiber/fiber/v2"
)

type GatewayHandlers struct {
	router   *gateway.Router
	upstream *gateway.Upstream
}

func NewGatewayHandlers(router *gateway.Router, upstream *gateway.Upstream) *GatewayHandlers {
	return &GatewayHandlers{
		router:   router,
		upstream: upstream,
	}
}

func (h *GatewayHandlers) RegisterRoutes(router fiber.Router, authMiddleware *auth.UnifiedAuthMiddleware) {
	v1 := router.Group("/v1", authMiddleware.Authenticate())
	v1.Post("/chat/completions", authMiddleware.RequireScope("gateway:chat"), h.ChatCompletions)
}

func (h *GatewayHandlers) ChatCompletions(c *fiber.Ctx) error {
	authCtx, ok := auth.GetAuthContext(c)
	if !ok {
		return fiber.NewError(fiber.StatusUnauthorized, "unauthorized")
	}

	// Parse request
	var req gateway.ChatRequest
	if err := json.Unmarshal(c.Body(), &req); err != nil {
		return fiber.NewError(fiber.StatusBadRequest, "invalid request body")
	}

	if req.Model == "" {
		return fiber.NewError(fiber.StatusBadRequest, "model is required")
	}

	// Route: resolve model → provider → credential
	tenantID := &authCtx.TenantID
	route, err := h.router.Resolve(c.Context(), req.Model, tenantID)
	if err != nil {
		return err
	}

	// Rewrite model to provider's external ID
	req.Model = route.ExternalID

	body, err := json.Marshal(req)
	if err != nil {
		return fiber.NewError(fiber.StatusInternalServerError, "failed to marshal request")
	}

	if req.Stream {
		return h.handleStream(c, route, body)
	}
	return h.handleNonStream(c, route, body)
}

func (h *GatewayHandlers) handleNonStream(c *fiber.Ctx, route *gateway.RouteResult, body []byte) error {
	start := time.Now()

	resp, statusCode, err := h.upstream.Call(c.Context(), route, body)
	if err != nil {
		_ = start // TODO: log failed request
		return err
	}

	_ = statusCode // TODO: log request with timing, tokens, cost

	return c.JSON(resp)
}

func (h *GatewayHandlers) handleStream(c *fiber.Ctx, route *gateway.RouteResult, body []byte) error {
	c.Set("Content-Type", "text/event-stream")
	c.Set("Cache-Control", "no-cache")
	c.Set("Connection", "keep-alive")
	c.Set("Transfer-Encoding", "chunked")

	c.Context().SetBodyStreamWriter(func(w *bufio.Writer) {
		err := h.upstream.Stream(c.Context(), route, body, func(chunk []byte) error {
			// Write SSE format: "data: <json>\n\n"
			if _, err := fmt.Fprintf(w, "data: %s\n\n", chunk); err != nil {
				return err
			}
			return w.Flush()
		})

		if err != nil {
			// Write error as SSE event
			errJSON, _ := json.Marshal(fiber.Map{"error": err.Error()})
			fmt.Fprintf(w, "data: %s\n\n", errJSON)
			w.Flush()
			return
		}

		// Write [DONE] marker
		fmt.Fprintf(w, "data: [DONE]\n\n")
		w.Flush()
	})

	return nil
}
