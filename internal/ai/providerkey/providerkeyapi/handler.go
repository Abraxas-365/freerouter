package providerkeyapi

import (
	"github.com/Abraxas-365/freerouter/internal/ai/providerkey"
	"github.com/Abraxas-365/freerouter/internal/ai/providerkey/providerkeysrv"
	"github.com/Abraxas-365/freerouter/internal/iam/auth"
	"github.com/Abraxas-365/freerouter/internal/kernel"
	"github.com/gofiber/fiber/v2"
)

type ProviderKeyHandlers struct {
	service *providerkeysrv.ProviderKeyService
}

func NewProviderKeyHandlers(service *providerkeysrv.ProviderKeyService) *ProviderKeyHandlers {
	return &ProviderKeyHandlers{service: service}
}

func (h *ProviderKeyHandlers) RegisterRoutes(router fiber.Router, authMiddleware *auth.UnifiedAuthMiddleware) {
	keys := router.Group("/provider-keys", authMiddleware.Authenticate())

	keys.Post("/", authMiddleware.RequireScope("provider-keys:write"), h.CreateKey)
	keys.Get("/:id", authMiddleware.RequireScope("provider-keys:read"), h.GetKey)
	keys.Put("/:id", authMiddleware.RequireScope("provider-keys:write"), h.UpdateKey)
	keys.Delete("/:id", authMiddleware.RequireScope("provider-keys:delete"), h.DeleteKey)

	keys.Get("/by-provider/:providerId", authMiddleware.RequireScope("provider-keys:read"), h.ListByProvider)
	keys.Get("/by-tenant/:tenantId", authMiddleware.RequireScope("provider-keys:read"), h.ListByTenant)
	keys.Get("/managed", authMiddleware.RequireScope("provider-keys:read"), h.ListManaged)

	keys.Post("/:id/test", authMiddleware.RequireScope("provider-keys:write"), h.TestKey)
}

func (h *ProviderKeyHandlers) CreateKey(c *fiber.Ctx) error {
	req, err := kernel.BindAndValidate[providerkey.CreateProviderKeyRequest](c)
	if err != nil {
		return err
	}

	k, err := h.service.CreateKey(c.Context(), req)
	if err != nil {
		return err
	}
	return c.Status(fiber.StatusCreated).JSON(k.ToDTO())
}

func (h *ProviderKeyHandlers) GetKey(c *fiber.Ctx) error {
	k, err := h.service.GetKey(c.Context(), kernel.NewProviderKeyID(c.Params("id")))
	if err != nil {
		return err
	}
	return c.JSON(k.ToDTO())
}

func (h *ProviderKeyHandlers) UpdateKey(c *fiber.Ctx) error {
	req, err := kernel.BindAndValidate[providerkey.UpdateProviderKeyRequest](c)
	if err != nil {
		return err
	}

	k, err := h.service.UpdateKey(c.Context(), kernel.NewProviderKeyID(c.Params("id")), req)
	if err != nil {
		return err
	}
	return c.JSON(k.ToDTO())
}

func (h *ProviderKeyHandlers) DeleteKey(c *fiber.Ctx) error {
	if err := h.service.DeleteKey(c.Context(), kernel.NewProviderKeyID(c.Params("id"))); err != nil {
		return err
	}
	return c.JSON(fiber.Map{"message": "Provider key deleted successfully"})
}

func (h *ProviderKeyHandlers) ListByProvider(c *fiber.Ctx) error {
	response, err := h.service.ListByProvider(c.Context(), kernel.NewProviderID(c.Params("providerId")))
	if err != nil {
		return err
	}
	return c.JSON(response)
}

func (h *ProviderKeyHandlers) ListByTenant(c *fiber.Ctx) error {
	response, err := h.service.ListByTenant(c.Context(), kernel.NewTenantID(c.Params("tenantId")))
	if err != nil {
		return err
	}
	return c.JSON(response)
}

func (h *ProviderKeyHandlers) ListManaged(c *fiber.Ctx) error {
	response, err := h.service.ListManaged(c.Context())
	if err != nil {
		return err
	}
	return c.JSON(response)
}

func (h *ProviderKeyHandlers) TestKey(c *fiber.Ctx) error {
	result, err := h.service.TestKey(c.Context(), kernel.NewProviderKeyID(c.Params("id")))
	if err != nil {
		return err
	}
	return c.JSON(result)
}
