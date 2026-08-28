package billingsrv

import (
	"context"
	"fmt"
	"time"

	"github.com/Abraxas-365/freerouter/internal/billing"
	"github.com/Abraxas-365/freerouter/internal/errx"
	"github.com/Abraxas-365/freerouter/internal/kernel"
)

type BillingService struct {
	repo          billing.BillingRepository
	spendingRepo  billing.SpendingLimitRepository // nil = no spending limits
}

func NewBillingService(repo billing.BillingRepository, spendingRepo billing.SpendingLimitRepository) *BillingService {
	return &BillingService{repo: repo, spendingRepo: spendingRepo}
}

// GetBalance returns the current balance for a tenant
func (s *BillingService) GetBalance(ctx context.Context, tenantID kernel.TenantID) (*billing.TenantBalance, error) {
	return s.repo.GetBalance(ctx, tenantID)
}

// HasSufficientBalance checks if a tenant can afford an estimated cost
func (s *BillingService) HasSufficientBalance(ctx context.Context, tenantID kernel.TenantID) (bool, error) {
	b, err := s.repo.GetBalance(ctx, tenantID)
	if err != nil {
		return false, err
	}
	return b.Balance > 0, nil
}

// TopUp adds credits to a tenant's balance
func (s *BillingService) TopUp(ctx context.Context, tenantID kernel.TenantID, req billing.TopUpRequest) (*billing.TenantBalance, *billing.Transaction, error) {
	desc := req.Description
	if desc == "" {
		desc = fmt.Sprintf("Top-up of $%.4f", req.Amount)
	}

	return s.repo.Credit(ctx, tenantID, req.Amount, billing.TxTypeTopUp, desc, req.ReferenceID)
}

// DebitUsage deducts credits for a gateway request
func (s *BillingService) DebitUsage(ctx context.Context, tenantID kernel.TenantID, cost float64, usageLogID string) (*billing.TenantBalance, error) {
	if cost <= 0 {
		return s.repo.GetBalance(ctx, tenantID)
	}

	desc := fmt.Sprintf("API usage: $%.6f", cost)
	balance, _, err := s.repo.Debit(ctx, tenantID, cost, billing.TxTypeUsage, desc, usageLogID)
	if err != nil {
		return nil, err
	}
	return balance, nil
}

// Refund adds credits back for a failed/cancelled request
func (s *BillingService) Refund(ctx context.Context, tenantID kernel.TenantID, amount float64, referenceID, description string) (*billing.TenantBalance, *billing.Transaction, error) {
	if description == "" {
		description = fmt.Sprintf("Refund of $%.6f", amount)
	}
	return s.repo.Credit(ctx, tenantID, amount, billing.TxTypeRefund, description, referenceID)
}

// Adjust makes a manual admin adjustment (positive or negative)
func (s *BillingService) Adjust(ctx context.Context, tenantID kernel.TenantID, req billing.AdjustRequest) (*billing.TenantBalance, *billing.Transaction, error) {
	if req.Amount > 0 {
		return s.repo.Credit(ctx, tenantID, req.Amount, billing.TxTypeAdjust, req.Description, "")
	}
	return s.repo.Debit(ctx, tenantID, -req.Amount, billing.TxTypeAdjust, req.Description, "")
}

// QueryTransactions returns paginated transactions for a tenant
func (s *BillingService) QueryTransactions(ctx context.Context, q billing.TransactionQuery) (*billing.TransactionListResponse, error) {
	txns, total, err := s.repo.QueryTransactions(ctx, q)
	if err != nil {
		return nil, errx.Wrap(err, "failed to query transactions", errx.TypeInternal)
	}

	dtos := make([]billing.TransactionDTO, len(txns))
	for i, t := range txns {
		dtos[i] = t.ToDTO()
	}
	return &billing.TransactionListResponse{Transactions: dtos, Total: total}, nil
}

// GetSpendSummary returns total spend for a tenant in a time period
func (s *BillingService) GetSpendSummary(ctx context.Context, tenantID kernel.TenantID, from, to *time.Time) (float64, error) {
	q := billing.TransactionQuery{
		TenantID: tenantID,
		Type:     billing.TxTypeUsage,
		From:     from,
		To:       to,
		Limit:    0, // We just need the total
	}

	txns, _, err := s.repo.QueryTransactions(ctx, q)
	if err != nil {
		return 0, err
	}

	var totalSpend float64
	for _, t := range txns {
		totalSpend += -t.Amount // Usage amounts are negative
	}
	return totalSpend, nil
}

// ============================================================================
// Spending Limits
// ============================================================================

// CheckSpendingLimit verifies the tenant hasn't exceeded daily/monthly caps.
func (s *BillingService) CheckSpendingLimit(ctx context.Context, tenantID kernel.TenantID) (*billing.SpendingCheckResult, error) {
	if s.spendingRepo == nil {
		return &billing.SpendingCheckResult{Allowed: true}, nil
	}

	cfg, err := s.spendingRepo.GetByTenantID(ctx, tenantID)
	if err != nil {
		return &billing.SpendingCheckResult{Allowed: true}, nil // fail open on error
	}
	if cfg == nil {
		return &billing.SpendingCheckResult{Allowed: true}, nil // no limits configured
	}

	result := &billing.SpendingCheckResult{
		Allowed:      true,
		DailyLimit:   cfg.DailyLimitUSD,
		MonthlyLimit: cfg.MonthlyLimitUSD,
	}

	// Check daily limit
	if cfg.DailyLimitUSD != nil {
		dailySpend, err := s.spendingRepo.GetDailySpend(ctx, tenantID)
		if err != nil {
			return &billing.SpendingCheckResult{Allowed: true}, nil
		}
		result.DailySpend = dailySpend
		if dailySpend >= *cfg.DailyLimitUSD {
			result.Allowed = false
			result.Reason = fmt.Sprintf("daily spending limit exceeded: $%.2f / $%.2f", dailySpend, *cfg.DailyLimitUSD)
			return result, nil
		}
	}

	// Check monthly limit
	if cfg.MonthlyLimitUSD != nil {
		monthlySpend, err := s.spendingRepo.GetMonthlySpend(ctx, tenantID)
		if err != nil {
			return &billing.SpendingCheckResult{Allowed: true}, nil
		}
		result.MonthlySpend = monthlySpend
		if monthlySpend >= *cfg.MonthlyLimitUSD {
			result.Allowed = false
			result.Reason = fmt.Sprintf("monthly spending limit exceeded: $%.2f / $%.2f", monthlySpend, *cfg.MonthlyLimitUSD)
			return result, nil
		}
	}

	return result, nil
}

// GetSpendingLimit returns the spending limit config for a tenant.
func (s *BillingService) GetSpendingLimit(ctx context.Context, tenantID kernel.TenantID) (*billing.SpendingLimitConfig, error) {
	if s.spendingRepo == nil {
		return nil, nil
	}
	return s.spendingRepo.GetByTenantID(ctx, tenantID)
}

// UpsertSpendingLimit creates or updates spending limits for a tenant.
func (s *BillingService) UpsertSpendingLimit(ctx context.Context, tenantID kernel.TenantID, req billing.UpsertSpendingLimitRequest) (*billing.SpendingLimitConfig, error) {
	if s.spendingRepo == nil {
		return nil, fmt.Errorf("spending limits not configured")
	}
	cfg := &billing.SpendingLimitConfig{
		TenantID:        tenantID,
		DailyLimitUSD:   req.DailyLimitUSD,
		MonthlyLimitUSD: req.MonthlyLimitUSD,
	}
	return s.spendingRepo.Upsert(ctx, cfg)
}

// DeleteSpendingLimit removes spending limits for a tenant.
func (s *BillingService) DeleteSpendingLimit(ctx context.Context, tenantID kernel.TenantID) error {
	if s.spendingRepo == nil {
		return fmt.Errorf("spending limits not configured")
	}
	return s.spendingRepo.Delete(ctx, tenantID)
}
