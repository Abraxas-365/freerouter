package webhookapi

import (
	"github.com/Abraxas-365/freerouter/internal/iam/auth"
	"github.com/Abraxas-365/freerouter/internal/kernel"
	"github.com/Abraxas-365/freerouter/internal/webhook"
	"github.com/Abraxas-365/freerouter/internal/webhook/webhooksrv"
	"github.com/gofiber/fiber/v2"
)

type WebhookHandlers struct {
	service *webhooksrv.WebhookService
}

func NewWebhookHandlers(service *webhooksrv.WebhookService) *WebhookHandlers {
	return &WebhookHandlers{service: service}
}

func (h *WebhookHandlers) RegisterRoutes(router fiber.Router, authMiddleware *auth.UnifiedAuthMiddleware) {
	wh := router.Group("/webhooks", authMiddleware.Authenticate())
	wh.Post("/", authMiddleware.RequireScope("webhooks:write"), h.Create)
	wh.Get("/", authMiddleware.RequireScope("webhooks:read"), h.List)
	wh.Get("/events", authMiddleware.RequireScope("webhooks:read"), h.ListEvents)
	wh.Get("/:id", authMiddleware.RequireScope("webhooks:read"), h.Get)
	wh.Put("/:id", authMiddleware.RequireScope("webhooks:write"), h.Update)
	wh.Delete("/:id", authMiddleware.RequireScope("webhooks:write"), h.Delete)
	wh.Get("/:id/deliveries", authMiddleware.RequireScope("webhooks:read"), h.ListDeliveries)
	wh.Post("/:id/test", authMiddleware.RequireScope("webhooks:write"), h.Test)
}

func (h *WebhookHandlers) Create(c *fiber.Ctx) error {
	authCtx, ok := auth.GetAuthContext(c)
	if !ok {
		return fiber.NewError(fiber.StatusUnauthorized, "unauthorized")
	}

	req, err := kernel.BindAndValidate[webhook.CreateWebhookRequest](c)
	if err != nil {
		return err
	}

	w, err := h.service.Create(c.Context(), authCtx.TenantID, req)
	if err != nil {
		return err
	}

	// Return secret only on create
	return c.Status(fiber.StatusCreated).JSON(fiber.Map{
		"id":         w.ID,
		"tenant_id":  w.TenantID,
		"url":        w.URL,
		"secret":     w.Secret,
		"events":     w.Events,
		"enabled":    w.Enabled,
		"created_at": w.CreatedAt,
	})
}

func (h *WebhookHandlers) List(c *fiber.Ctx) error {
	authCtx, ok := auth.GetAuthContext(c)
	if !ok {
		return fiber.NewError(fiber.StatusUnauthorized, "unauthorized")
	}

	webhooks, err := h.service.List(c.Context(), authCtx.TenantID)
	if err != nil {
		return err
	}
	if webhooks == nil {
		webhooks = []*webhook.WebhookConfig{}
	}
	return c.JSON(fiber.Map{"webhooks": webhooks, "total": len(webhooks)})
}

func (h *WebhookHandlers) ListEvents(c *fiber.Ctx) error {
	return c.JSON(fiber.Map{"events": webhook.AllEvents()})
}

func (h *WebhookHandlers) Get(c *fiber.Ctx) error {
	w, err := h.service.Get(c.Context(), kernel.NewWebhookID(c.Params("id")))
	if err != nil {
		return err
	}
	return c.JSON(w)
}

func (h *WebhookHandlers) Update(c *fiber.Ctx) error {
	var req webhook.UpdateWebhookRequest
	if err := c.BodyParser(&req); err != nil {
		return fiber.NewError(fiber.StatusBadRequest, "invalid request body")
	}

	w, err := h.service.Update(c.Context(), kernel.NewWebhookID(c.Params("id")), req)
	if err != nil {
		return err
	}
	return c.JSON(w)
}

func (h *WebhookHandlers) Delete(c *fiber.Ctx) error {
	if err := h.service.Delete(c.Context(), kernel.NewWebhookID(c.Params("id"))); err != nil {
		return err
	}
	return c.JSON(fiber.Map{"message": "Webhook deleted"})
}

func (h *WebhookHandlers) ListDeliveries(c *fiber.Ctx) error {
	limit := c.QueryInt("limit", 50)
	deliveries, err := h.service.GetDeliveries(c.Context(), kernel.NewWebhookID(c.Params("id")), limit)
	if err != nil {
		return err
	}
	if deliveries == nil {
		deliveries = []*webhook.WebhookDelivery{}
	}
	return c.JSON(fiber.Map{"deliveries": deliveries, "total": len(deliveries)})
}

func (h *WebhookHandlers) Test(c *fiber.Ctx) error {
	authCtx, ok := auth.GetAuthContext(c)
	if !ok {
		return fiber.NewError(fiber.StatusUnauthorized, "unauthorized")
	}

	h.service.Fire(authCtx.TenantID, "webhook.test", fiber.Map{
		"message":    "This is a test webhook event",
		"webhook_id": c.Params("id"),
	})

	return c.JSON(fiber.Map{"message": "Test event dispatched"})
}
