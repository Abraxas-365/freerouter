package walletapi

import (
	"github.com/Abraxas-365/freerouter/internal/iam/auth"
	"github.com/Abraxas-365/freerouter/internal/iam/scopes"
	"github.com/Abraxas-365/freerouter/internal/kernel"
	"github.com/Abraxas-365/freerouter/internal/wallet"
	"github.com/Abraxas-365/freerouter/internal/wallet/walletsrv"
	"github.com/gofiber/fiber/v2"
)

type WalletHandlers struct {
	service *walletsrv.WalletService
}

func NewWalletHandlers(service *walletsrv.WalletService) *WalletHandlers {
	return &WalletHandlers{service: service}
}

func (h *WalletHandlers) RegisterRoutes(router fiber.Router, authMiddleware *auth.UnifiedAuthMiddleware) {
	w := router.Group("/wallets", authMiddleware.Authenticate())

	w.Post("/", authMiddleware.RequireScope(scopes.ScopeWalletsWrite), h.CreateWallet)
	w.Get("/", authMiddleware.RequireScope(scopes.ScopeWalletsRead), h.ListWallets)
	w.Get("/:id", authMiddleware.RequireScope(scopes.ScopeWalletsRead), h.GetWallet)
	w.Put("/:id", authMiddleware.RequireScope(scopes.ScopeWalletsWrite), h.UpdateWallet)
	w.Delete("/:id", authMiddleware.RequireScope(scopes.ScopeWalletsDelete), h.DeleteWallet)

	w.Post("/:id/fund", authMiddleware.RequireScope(scopes.ScopeWalletsTransfer), h.Fund)
	w.Post("/:id/withdraw", authMiddleware.RequireScope(scopes.ScopeWalletsTransfer), h.Withdraw)
}

func (h *WalletHandlers) CreateWallet(c *fiber.Ctx) error {
	authCtx, ok := auth.GetAuthContext(c)
	if !ok {
		return fiber.NewError(fiber.StatusUnauthorized, "unauthorized")
	}

	req, err := kernel.BindAndValidate[wallet.CreateWalletRequest](c)
	if err != nil {
		return err
	}

	w, err := h.service.CreateWallet(c.Context(), authCtx.TenantID, req)
	if err != nil {
		return err
	}
	return c.Status(fiber.StatusCreated).JSON(w.ToDTO())
}

func (h *WalletHandlers) ListWallets(c *fiber.Ctx) error {
	authCtx, ok := auth.GetAuthContext(c)
	if !ok {
		return fiber.NewError(fiber.StatusUnauthorized, "unauthorized")
	}

	resp, err := h.service.ListWallets(c.Context(), authCtx.TenantID)
	if err != nil {
		return err
	}
	return c.JSON(resp)
}

func (h *WalletHandlers) GetWallet(c *fiber.Ctx) error {
	authCtx, ok := auth.GetAuthContext(c)
	if !ok {
		return fiber.NewError(fiber.StatusUnauthorized, "unauthorized")
	}

	w, err := h.service.GetWallet(c.Context(), authCtx.TenantID, kernel.NewWalletID(c.Params("id")))
	if err != nil {
		return err
	}
	return c.JSON(w.ToDTO())
}

func (h *WalletHandlers) UpdateWallet(c *fiber.Ctx) error {
	authCtx, ok := auth.GetAuthContext(c)
	if !ok {
		return fiber.NewError(fiber.StatusUnauthorized, "unauthorized")
	}

	req, err := kernel.BindAndValidate[wallet.UpdateWalletRequest](c)
	if err != nil {
		return err
	}

	w, err := h.service.UpdateWallet(c.Context(), authCtx.TenantID, kernel.NewWalletID(c.Params("id")), req)
	if err != nil {
		return err
	}
	return c.JSON(w.ToDTO())
}

func (h *WalletHandlers) DeleteWallet(c *fiber.Ctx) error {
	authCtx, ok := auth.GetAuthContext(c)
	if !ok {
		return fiber.NewError(fiber.StatusUnauthorized, "unauthorized")
	}

	if err := h.service.DeleteWallet(c.Context(), authCtx.TenantID, kernel.NewWalletID(c.Params("id"))); err != nil {
		return err
	}
	return c.JSON(fiber.Map{"message": "wallet deleted"})
}

func (h *WalletHandlers) Fund(c *fiber.Ctx) error {
	authCtx, ok := auth.GetAuthContext(c)
	if !ok {
		return fiber.NewError(fiber.StatusUnauthorized, "unauthorized")
	}

	req, err := kernel.BindAndValidate[wallet.TransferRequest](c)
	if err != nil {
		return err
	}

	resp, err := h.service.Fund(c.Context(), authCtx.TenantID, kernel.NewWalletID(c.Params("id")), req)
	if err != nil {
		return err
	}
	return c.JSON(resp)
}

func (h *WalletHandlers) Withdraw(c *fiber.Ctx) error {
	authCtx, ok := auth.GetAuthContext(c)
	if !ok {
		return fiber.NewError(fiber.StatusUnauthorized, "unauthorized")
	}

	req, err := kernel.BindAndValidate[wallet.TransferRequest](c)
	if err != nil {
		return err
	}

	resp, err := h.service.Withdraw(c.Context(), authCtx.TenantID, kernel.NewWalletID(c.Params("id")), req)
	if err != nil {
		return err
	}
	return c.JSON(resp)
}
