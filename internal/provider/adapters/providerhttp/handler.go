package providerhttp

import (
	"github.com/Abraxas-365/freerouter/internal/errx"
	"github.com/Abraxas-365/freerouter/internal/httpx"
	"github.com/Abraxas-365/freerouter/internal/identity"
	"github.com/Abraxas-365/freerouter/internal/provider"
	"github.com/Abraxas-365/freerouter/internal/server"
	"github.com/gofiber/fiber/v2"
)

// Handler exposes provider module endpoints over HTTP.
type Handler struct {
	providers struct {
		cmd provider.Commands
		qry provider.Queries
	}
	models struct {
		cmd provider.ModelCommands
		qry provider.ModelQueries
	}
	mappings struct {
		cmd provider.MappingCommands
		qry provider.MappingQueries
	}
	fallbacks struct {
		cmd provider.FallbackCommands
		qry provider.FallbackQueries
	}
}

// New creates a provider HTTP handler.
func New(
	providerCmd provider.Commands, providerQry provider.Queries,
	modelCmd provider.ModelCommands, modelQry provider.ModelQueries,
	mappingCmd provider.MappingCommands, mappingQry provider.MappingQueries,
	fallbackCmd provider.FallbackCommands, fallbackQry provider.FallbackQueries,
) *Handler {
	h := &Handler{}
	h.providers.cmd = providerCmd
	h.providers.qry = providerQry
	h.models.cmd = modelCmd
	h.models.qry = modelQry
	h.mappings.cmd = mappingCmd
	h.mappings.qry = mappingQry
	h.fallbacks.cmd = fallbackCmd
	h.fallbacks.qry = fallbackQry
	return h
}

// RegisterRoutes mounts all provider-module routes on the given router.
func (h *Handler) RegisterRoutes(r fiber.Router) {
	// Providers
	providers := r.Group("/providers")
	providers.Post("/", server.RequirePermissions(server.PermProvidersWrite), h.createProvider)
	providers.Get("/", server.RequirePermissions(server.PermProvidersRead), h.listProviders)
	providers.Get("/:id", server.RequirePermissions(server.PermProvidersRead), h.findProvider)
	providers.Put("/:id", server.RequirePermissions(server.PermProvidersWrite), h.updateProvider)
	providers.Delete("/:id", server.RequirePermissions(server.PermProvidersWrite), h.deleteProvider)

	// Models
	models := r.Group("/models")
	models.Post("/", server.RequirePermissions(server.PermProvidersWrite), h.createModel)
	models.Get("/", server.RequirePermissions(server.PermProvidersRead), h.listModels)
	models.Get("/:id", server.RequirePermissions(server.PermProvidersRead), h.findModel)
	models.Put("/:id", server.RequirePermissions(server.PermProvidersWrite), h.updateModel)
	models.Delete("/:id", server.RequirePermissions(server.PermProvidersWrite), h.deleteModel)

	// Mappings
	mappings := r.Group("/mappings")
	mappings.Post("/", server.RequirePermissions(server.PermProvidersWrite), h.createMapping)
	mappings.Get("/", server.RequirePermissions(server.PermProvidersRead), h.listMappings)
	mappings.Get("/:id", server.RequirePermissions(server.PermProvidersRead), h.findMapping)
	mappings.Put("/:id", server.RequirePermissions(server.PermProvidersWrite), h.updateMapping)
	mappings.Delete("/:id", server.RequirePermissions(server.PermProvidersWrite), h.deleteMapping)

	// Fallbacks
	fallbacks := r.Group("/model-fallbacks")
	fallbacks.Post("/", server.RequirePermissions(server.PermProvidersWrite), h.createFallback)
	fallbacks.Get("/by-model/:modelId", server.RequirePermissions(server.PermProvidersRead), h.listFallbacks)
	fallbacks.Delete("/:id", server.RequirePermissions(server.PermProvidersWrite), h.deleteFallback)
}

// ════════════════════════════════════════════════════════════════════
// Provider handlers
// ════════════════════════════════════════════════════════════════════

func (h *Handler) createProvider(c *fiber.Ctx) error {
	var input provider.Create
	if err := c.BodyParser(&input); err != nil {
		return errx.Validation("invalid request body")
	}
	id, err := h.providers.cmd.Create(c.Context(), input)
	if err != nil {
		return err
	}
	return c.Status(fiber.StatusCreated).JSON(fiber.Map{"id": id})
}

func (h *Handler) findProvider(c *fiber.Ctx) error {
	id, err := identity.ParseProviderID(c.Params("id"))
	if err != nil {
		return err
	}
	p, err := h.providers.qry.Find(c.Context(), id)
	if err != nil {
		return err
	}
	return c.JSON(p)
}

func (h *Handler) listProviders(c *fiber.Ctx) error {
	page := httpx.PaginationFromCtx(c)
	filter := provider.Filter{}

	if status := c.Query("status"); status != "" {
		s := provider.ProviderStatus(status)
		filter.Status = &s
	}
	if search := c.Query("search"); search != "" {
		filter.Search = &search
	}

	result, err := h.providers.qry.List(c.Context(), filter, page)
	if err != nil {
		return err
	}
	return c.JSON(result)
}

func (h *Handler) updateProvider(c *fiber.Ctx) error {
	id, err := identity.ParseProviderID(c.Params("id"))
	if err != nil {
		return err
	}
	var input provider.Update
	if err := c.BodyParser(&input); err != nil {
		return errx.Validation("invalid request body")
	}
	if err := h.providers.cmd.Update(c.Context(), id, input); err != nil {
		return err
	}
	return c.SendStatus(fiber.StatusNoContent)
}

func (h *Handler) deleteProvider(c *fiber.Ctx) error {
	id, err := identity.ParseProviderID(c.Params("id"))
	if err != nil {
		return err
	}
	if err := h.providers.cmd.Delete(c.Context(), id); err != nil {
		return err
	}
	return c.SendStatus(fiber.StatusNoContent)
}

// ════════════════════════════════════════════════════════════════════
// Model handlers
// ════════════════════════════════════════════════════════════════════

func (h *Handler) createModel(c *fiber.Ctx) error {
	var input provider.CreateModel
	if err := c.BodyParser(&input); err != nil {
		return errx.Validation("invalid request body")
	}
	id, err := h.models.cmd.CreateModel(c.Context(), input)
	if err != nil {
		return err
	}
	return c.Status(fiber.StatusCreated).JSON(fiber.Map{"id": id})
}

func (h *Handler) findModel(c *fiber.Ctx) error {
	id, err := identity.ParseModelID(c.Params("id"))
	if err != nil {
		return err
	}
	m, err := h.models.qry.FindModel(c.Context(), id)
	if err != nil {
		return err
	}
	return c.JSON(m)
}

func (h *Handler) listModels(c *fiber.Ctx) error {
	page := httpx.PaginationFromCtx(c)
	filter := provider.ModelFilter{}

	if status := c.Query("status"); status != "" {
		s := provider.ModelStatus(status)
		filter.Status = &s
	}
	if family := c.Query("family"); family != "" {
		filter.Family = &family
	}
	if search := c.Query("search"); search != "" {
		filter.Search = &search
	}

	result, err := h.models.qry.ListModels(c.Context(), filter, page)
	if err != nil {
		return err
	}
	return c.JSON(result)
}

func (h *Handler) updateModel(c *fiber.Ctx) error {
	id, err := identity.ParseModelID(c.Params("id"))
	if err != nil {
		return err
	}
	var input provider.UpdateModel
	if err := c.BodyParser(&input); err != nil {
		return errx.Validation("invalid request body")
	}
	if err := h.models.cmd.UpdateModel(c.Context(), id, input); err != nil {
		return err
	}
	return c.SendStatus(fiber.StatusNoContent)
}

func (h *Handler) deleteModel(c *fiber.Ctx) error {
	id, err := identity.ParseModelID(c.Params("id"))
	if err != nil {
		return err
	}
	if err := h.models.cmd.DeleteModel(c.Context(), id); err != nil {
		return err
	}
	return c.SendStatus(fiber.StatusNoContent)
}

// ════════════════════════════════════════════════════════════════════
// Mapping handlers
// ════════════════════════════════════════════════════════════════════

func (h *Handler) createMapping(c *fiber.Ctx) error {
	var input provider.CreateMapping
	if err := c.BodyParser(&input); err != nil {
		return errx.Validation("invalid request body")
	}
	id, err := h.mappings.cmd.CreateMapping(c.Context(), input)
	if err != nil {
		return err
	}
	return c.Status(fiber.StatusCreated).JSON(fiber.Map{"id": id})
}

func (h *Handler) findMapping(c *fiber.Ctx) error {
	id, err := identity.ParseMappingID(c.Params("id"))
	if err != nil {
		return err
	}
	m, err := h.mappings.qry.FindMapping(c.Context(), id)
	if err != nil {
		return err
	}
	return c.JSON(m)
}

func (h *Handler) listMappings(c *fiber.Ctx) error {
	page := httpx.PaginationFromCtx(c)
	filter := provider.MappingFilter{}

	if modelID := c.Query("model_id"); modelID != "" {
		id, err := identity.ParseModelID(modelID)
		if err != nil {
			return err
		}
		filter.ModelID = &id
	}
	if providerID := c.Query("provider_id"); providerID != "" {
		id, err := identity.ParseProviderID(providerID)
		if err != nil {
			return err
		}
		filter.ProviderID = &id
	}
	if status := c.Query("status"); status != "" {
		s := provider.ModelStatus(status)
		filter.Status = &s
	}

	result, err := h.mappings.qry.ListMappings(c.Context(), filter, page)
	if err != nil {
		return err
	}
	return c.JSON(result)
}

func (h *Handler) updateMapping(c *fiber.Ctx) error {
	id, err := identity.ParseMappingID(c.Params("id"))
	if err != nil {
		return err
	}
	var input provider.UpdateMapping
	if err := c.BodyParser(&input); err != nil {
		return errx.Validation("invalid request body")
	}
	if err := h.mappings.cmd.UpdateMapping(c.Context(), id, input); err != nil {
		return err
	}
	return c.SendStatus(fiber.StatusNoContent)
}

func (h *Handler) deleteMapping(c *fiber.Ctx) error {
	id, err := identity.ParseMappingID(c.Params("id"))
	if err != nil {
		return err
	}
	if err := h.mappings.cmd.DeleteMapping(c.Context(), id); err != nil {
		return err
	}
	return c.SendStatus(fiber.StatusNoContent)
}

// ════════════════════════════════════════════════════════════════════
// Fallback handlers
// ════════════════════════════════════════════════════════════════════

func (h *Handler) createFallback(c *fiber.Ctx) error {
	var input provider.CreateFallback
	if err := c.BodyParser(&input); err != nil {
		return errx.Validation("invalid request body")
	}
	id, err := h.fallbacks.cmd.CreateFallback(c.Context(), input)
	if err != nil {
		return err
	}
	return c.Status(fiber.StatusCreated).JSON(fiber.Map{"id": id})
}

func (h *Handler) listFallbacks(c *fiber.Ctx) error {
	modelID, err := identity.ParseModelID(c.Params("modelId"))
	if err != nil {
		return err
	}
	items, err := h.fallbacks.qry.ListFallbacks(c.Context(), modelID)
	if err != nil {
		return err
	}
	return c.JSON(items)
}

func (h *Handler) deleteFallback(c *fiber.Ctx) error {
	id, err := identity.ParseModelFallbackID(c.Params("id"))
	if err != nil {
		return err
	}
	if err := h.fallbacks.cmd.DeleteFallback(c.Context(), id); err != nil {
		return err
	}
	return c.SendStatus(fiber.StatusNoContent)
}
