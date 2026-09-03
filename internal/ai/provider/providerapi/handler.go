package providerapi

import (
	"github.com/Abraxas-365/freerouter/internal/ai/provider"
	"github.com/Abraxas-365/freerouter/internal/ai/provider/providersrv"
	"github.com/Abraxas-365/freerouter/internal/iam/auth"
	"github.com/Abraxas-365/freerouter/internal/iam/scopes"
	"github.com/Abraxas-365/freerouter/internal/kernel"
	"github.com/gofiber/fiber/v2"
)

type ProviderHandlers struct {
	service *providersrv.ProviderService
}

func NewProviderHandlers(service *providersrv.ProviderService) *ProviderHandlers {
	return &ProviderHandlers{service: service}
}

func (h *ProviderHandlers) RegisterRoutes(router fiber.Router, authMiddleware *auth.UnifiedAuthMiddleware) {
	providers := router.Group("/providers", authMiddleware.Authenticate())
	providers.Post("/", authMiddleware.RequireScope(scopes.ScopeProvidersWrite), h.CreateProvider)
	providers.Get("/", authMiddleware.RequireScope(scopes.ScopeProvidersRead), h.ListProviders)
	providers.Get("/:id", authMiddleware.RequireScope(scopes.ScopeProvidersRead), h.GetProvider)
	providers.Put("/:id", authMiddleware.RequireScope(scopes.ScopeProvidersWrite), h.UpdateProvider)
	providers.Delete("/:id", authMiddleware.RequireScope(scopes.ScopeProvidersDelete), h.DeleteProvider)

	models := router.Group("/models", authMiddleware.Authenticate())
	models.Post("/", authMiddleware.RequireScope(scopes.ScopeModelsWrite), h.CreateModel)
	models.Get("/", authMiddleware.RequireScope(scopes.ScopeModelsRead), h.ListModels)
	models.Get("/:id", authMiddleware.RequireScope(scopes.ScopeModelsRead), h.GetModel)
	models.Get("/:id/mappings", authMiddleware.RequireScope(scopes.ScopeModelsRead), h.GetModelWithMappings)
	models.Put("/:id", authMiddleware.RequireScope(scopes.ScopeModelsWrite), h.UpdateModel)
	models.Delete("/:id", authMiddleware.RequireScope(scopes.ScopeModelsDelete), h.DeleteModel)

	mappings := router.Group("/mappings", authMiddleware.Authenticate())
	mappings.Post("/", authMiddleware.RequireScope(scopes.ScopeModelsWrite), h.CreateMapping)
	mappings.Get("/:id", authMiddleware.RequireScope(scopes.ScopeModelsRead), h.GetMapping)
	mappings.Put("/:id", authMiddleware.RequireScope(scopes.ScopeModelsWrite), h.UpdateMapping)
	mappings.Delete("/:id", authMiddleware.RequireScope(scopes.ScopeModelsDelete), h.DeleteMapping)

	fallbacks := router.Group("/model-fallbacks", authMiddleware.Authenticate())
	fallbacks.Post("/", authMiddleware.RequireScope(scopes.ScopeModelsWrite), h.CreateFallback)
	fallbacks.Get("/by-model/:modelId", authMiddleware.RequireScope(scopes.ScopeModelsRead), h.ListFallbacks)
	fallbacks.Delete("/:id", authMiddleware.RequireScope(scopes.ScopeModelsDelete), h.DeleteFallback)
}

// ============================================================================
// Provider handlers
// ============================================================================

func (h *ProviderHandlers) CreateProvider(c *fiber.Ctx) error {
	req, err := kernel.BindAndValidate[provider.CreateProviderRequest](c)
	if err != nil {
		return err
	}

	p, err := h.service.CreateProvider(c.Context(), req)
	if err != nil {
		return err
	}
	return c.Status(fiber.StatusCreated).JSON(p.ToDTO())
}

func (h *ProviderHandlers) ListProviders(c *fiber.Ctx) error {
	response, err := h.service.ListProviders(c.Context())
	if err != nil {
		return err
	}
	return c.JSON(response)
}

func (h *ProviderHandlers) GetProvider(c *fiber.Ctx) error {
	p, err := h.service.GetProvider(c.Context(), kernel.NewProviderID(c.Params("id")))
	if err != nil {
		return err
	}
	return c.JSON(p.ToDTO())
}

func (h *ProviderHandlers) UpdateProvider(c *fiber.Ctx) error {
	req, err := kernel.BindAndValidate[provider.UpdateProviderRequest](c)
	if err != nil {
		return err
	}

	p, err := h.service.UpdateProvider(c.Context(), kernel.NewProviderID(c.Params("id")), req)
	if err != nil {
		return err
	}
	return c.JSON(p.ToDTO())
}

func (h *ProviderHandlers) DeleteProvider(c *fiber.Ctx) error {
	if err := h.service.DeleteProvider(c.Context(), kernel.NewProviderID(c.Params("id"))); err != nil {
		return err
	}
	return c.JSON(fiber.Map{"message": "Provider deleted successfully"})
}

// ============================================================================
// Model handlers
// ============================================================================

func (h *ProviderHandlers) CreateModel(c *fiber.Ctx) error {
	req, err := kernel.BindAndValidate[provider.CreateModelRequest](c)
	if err != nil {
		return err
	}

	m, err := h.service.CreateModel(c.Context(), req)
	if err != nil {
		return err
	}
	return c.Status(fiber.StatusCreated).JSON(m.ToDTO())
}

func (h *ProviderHandlers) ListModels(c *fiber.Ctx) error {
	response, err := h.service.ListModels(c.Context())
	if err != nil {
		return err
	}
	return c.JSON(response)
}

func (h *ProviderHandlers) GetModel(c *fiber.Ctx) error {
	m, err := h.service.GetModel(c.Context(), kernel.NewModelID(c.Params("id")))
	if err != nil {
		return err
	}
	return c.JSON(m.ToDTO())
}

func (h *ProviderHandlers) GetModelWithMappings(c *fiber.Ctx) error {
	result, err := h.service.GetModelWithMappings(c.Context(), kernel.NewModelID(c.Params("id")))
	if err != nil {
		return err
	}
	return c.JSON(result)
}

func (h *ProviderHandlers) UpdateModel(c *fiber.Ctx) error {
	req, err := kernel.BindAndValidate[provider.UpdateModelRequest](c)
	if err != nil {
		return err
	}

	m, err := h.service.UpdateModel(c.Context(), kernel.NewModelID(c.Params("id")), req)
	if err != nil {
		return err
	}
	return c.JSON(m.ToDTO())
}

func (h *ProviderHandlers) DeleteModel(c *fiber.Ctx) error {
	if err := h.service.DeleteModel(c.Context(), kernel.NewModelID(c.Params("id"))); err != nil {
		return err
	}
	return c.JSON(fiber.Map{"message": "Model deleted successfully"})
}

// ============================================================================
// Mapping handlers
// ============================================================================

func (h *ProviderHandlers) CreateMapping(c *fiber.Ctx) error {
	req, err := kernel.BindAndValidate[provider.CreateMappingRequest](c)
	if err != nil {
		return err
	}

	m, err := h.service.CreateMapping(c.Context(), req)
	if err != nil {
		return err
	}
	return c.Status(fiber.StatusCreated).JSON(m.ToDTO())
}

func (h *ProviderHandlers) GetMapping(c *fiber.Ctx) error {
	m, err := h.service.GetMapping(c.Context(), kernel.NewMappingID(c.Params("id")))
	if err != nil {
		return err
	}
	return c.JSON(m.ToDTO())
}

func (h *ProviderHandlers) UpdateMapping(c *fiber.Ctx) error {
	req, err := kernel.BindAndValidate[provider.UpdateMappingRequest](c)
	if err != nil {
		return err
	}

	m, err := h.service.UpdateMapping(c.Context(), kernel.NewMappingID(c.Params("id")), req)
	if err != nil {
		return err
	}
	return c.JSON(m.ToDTO())
}

func (h *ProviderHandlers) DeleteMapping(c *fiber.Ctx) error {
	if err := h.service.DeleteMapping(c.Context(), kernel.NewMappingID(c.Params("id"))); err != nil {
		return err
	}
	return c.JSON(fiber.Map{"message": "Mapping deleted successfully"})
}

// ============================================================================
// Fallback handlers
// ============================================================================

func (h *ProviderHandlers) CreateFallback(c *fiber.Ctx) error {
	req, err := kernel.BindAndValidate[provider.CreateModelFallbackRequest](c)
	if err != nil {
		return err
	}

	f, err := h.service.CreateFallback(c.Context(), req)
	if err != nil {
		return err
	}
	return c.Status(fiber.StatusCreated).JSON(f)
}

func (h *ProviderHandlers) ListFallbacks(c *fiber.Ctx) error {
	fallbacks, err := h.service.ListFallbacks(c.Context(), kernel.NewModelID(c.Params("modelId")))
	if err != nil {
		return err
	}
	if fallbacks == nil {
		fallbacks = []*provider.ModelFallback{}
	}
	return c.JSON(fiber.Map{"fallbacks": fallbacks, "total": len(fallbacks)})
}

func (h *ProviderHandlers) DeleteFallback(c *fiber.Ctx) error {
	if err := h.service.DeleteFallback(c.Context(), kernel.NewModelFallbackID(c.Params("id"))); err != nil {
		return err
	}
	return c.JSON(fiber.Map{"message": "Fallback deleted successfully"})
}
