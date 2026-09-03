package walletsrv

import (
	"context"
	"fmt"
	"time"

	"github.com/Abraxas-365/freerouter/internal/billing"
	"github.com/Abraxas-365/freerouter/internal/kernel"
	"github.com/Abraxas-365/freerouter/internal/wallet"
	"github.com/google/uuid"
)

type WalletService struct {
	repo        wallet.WalletRepository
	billingRepo billing.BillingRepository
}

func NewWalletService(repo wallet.WalletRepository, billingRepo billing.BillingRepository) *WalletService {
	return &WalletService{repo: repo, billingRepo: billingRepo}
}

// CreateWallet creates a new empty wallet for the tenant.
func (s *WalletService) CreateWallet(ctx context.Context, tenantID kernel.TenantID, req wallet.CreateWalletRequest) (*wallet.Wallet, error) {
	now := time.Now().UTC()
	w := &wallet.Wallet{
		ID:          kernel.NewWalletID(uuid.NewString()),
		TenantID:    tenantID,
		Name:        req.Name,
		Description: req.Description,
		Balance:     0,
		CreatedAt:   now,
		UpdatedAt:   now,
	}
	return s.repo.Create(ctx, w)
}

// GetWallet returns a wallet scoped to the tenant.
func (s *WalletService) GetWallet(ctx context.Context, tenantID kernel.TenantID, id kernel.WalletID) (*wallet.Wallet, error) {
	return s.repo.GetByID(ctx, tenantID, id)
}

// ListWallets returns all wallets for the tenant.
func (s *WalletService) ListWallets(ctx context.Context, tenantID kernel.TenantID) (*wallet.WalletListResponse, error) {
	wallets, err := s.repo.ListByTenant(ctx, tenantID)
	if err != nil {
		return nil, err
	}
	dtos := make([]wallet.WalletDTO, len(wallets))
	for i, w := range wallets {
		dtos[i] = w.ToDTO()
	}
	return &wallet.WalletListResponse{Wallets: dtos, Total: len(dtos)}, nil
}

// UpdateWallet renames a wallet or changes its description.
func (s *WalletService) UpdateWallet(ctx context.Context, tenantID kernel.TenantID, id kernel.WalletID, req wallet.UpdateWalletRequest) (*wallet.Wallet, error) {
	w, err := s.repo.GetByID(ctx, tenantID, id)
	if err != nil {
		return nil, err
	}
	if req.Name != nil {
		w.Name = *req.Name
	}
	if req.Description != nil {
		w.Description = *req.Description
	}
	w.UpdatedAt = time.Now().UTC()
	return s.repo.Update(ctx, w)
}

// DeleteWallet removes a wallet. The wallet must be empty and not bound to
// any API key; withdraw remaining funds and unbind keys first.
func (s *WalletService) DeleteWallet(ctx context.Context, tenantID kernel.TenantID, id kernel.WalletID) error {
	w, err := s.repo.GetByID(ctx, tenantID, id)
	if err != nil {
		return err
	}
	if !w.IsEmpty() {
		return wallet.ErrWalletNotEmpty().WithDetail("balance", w.Balance)
	}
	bound, err := s.repo.CountBoundAPIKeys(ctx, tenantID, id)
	if err != nil {
		return err
	}
	if bound > 0 {
		return wallet.ErrWalletInUse().WithDetail("api_keys", bound)
	}
	return s.repo.Delete(ctx, tenantID, id)
}

// Fund moves credits from the tenant main balance into the wallet.
func (s *WalletService) Fund(ctx context.Context, tenantID kernel.TenantID, id kernel.WalletID, req wallet.TransferRequest) (*wallet.TransferResponse, error) {
	w, err := s.repo.GetByID(ctx, tenantID, id)
	if err != nil {
		return nil, err
	}

	desc := req.Description
	if desc == "" {
		desc = fmt.Sprintf("Fund wallet %q with $%.4f", w.Name, req.Amount)
	}

	w, err = s.repo.Fund(ctx, tenantID, id, req.Amount, desc)
	if err != nil {
		return nil, err
	}
	return s.transferResponse(ctx, tenantID, w)
}

// Withdraw moves credits from the wallet back to the tenant main balance.
func (s *WalletService) Withdraw(ctx context.Context, tenantID kernel.TenantID, id kernel.WalletID, req wallet.TransferRequest) (*wallet.TransferResponse, error) {
	w, err := s.repo.GetByID(ctx, tenantID, id)
	if err != nil {
		return nil, err
	}

	desc := req.Description
	if desc == "" {
		desc = fmt.Sprintf("Withdraw $%.4f from wallet %q", req.Amount, w.Name)
	}

	w, err = s.repo.Withdraw(ctx, tenantID, id, req.Amount, desc)
	if err != nil {
		return nil, err
	}
	return s.transferResponse(ctx, tenantID, w)
}

// DebitUsage deducts a gateway request cost from the wallet.
// Called by the gateway when the requesting API key is bound to a wallet.
func (s *WalletService) DebitUsage(ctx context.Context, tenantID kernel.TenantID, id kernel.WalletID, cost float64, usageLogID string) (*wallet.Wallet, error) {
	if cost <= 0 {
		return s.repo.GetByID(ctx, tenantID, id)
	}
	return s.repo.DebitUsage(ctx, tenantID, id, cost, usageLogID)
}

// HasSufficientFunds reports whether the wallet can pay for anything at all.
// Mirrors BillingService.HasSufficientBalance for the gateway pre-check.
func (s *WalletService) HasSufficientFunds(ctx context.Context, tenantID kernel.TenantID, id kernel.WalletID) (bool, error) {
	w, err := s.repo.GetByID(ctx, tenantID, id)
	if err != nil {
		return false, err
	}
	return w.Balance > 0, nil
}

// WalletExists verifies a wallet exists and belongs to the tenant.
// Satisfies apikeysrv.WalletValidator for API key wallet binding.
func (s *WalletService) WalletExists(ctx context.Context, tenantID kernel.TenantID, id kernel.WalletID) error {
	_, err := s.repo.GetByID(ctx, tenantID, id)
	return err
}

func (s *WalletService) transferResponse(ctx context.Context, tenantID kernel.TenantID, w *wallet.Wallet) (*wallet.TransferResponse, error) {
	main, err := s.billingRepo.GetBalance(ctx, tenantID)
	if err != nil {
		return nil, err
	}
	return &wallet.TransferResponse{
		Wallet:      w.ToDTO(),
		MainBalance: main.ToDTO(),
	}, nil
}
