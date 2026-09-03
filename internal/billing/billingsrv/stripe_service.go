package billingsrv

import (
	"context"
	"encoding/json"
	"fmt"
	"math"

	"github.com/Abraxas-365/freerouter/internal/billing"
	"github.com/Abraxas-365/freerouter/internal/config"
	"github.com/Abraxas-365/freerouter/internal/errx"
	"github.com/Abraxas-365/freerouter/internal/kernel"
	"github.com/Abraxas-365/freerouter/internal/logx"
	"github.com/stripe/stripe-go/v82"
	"github.com/stripe/stripe-go/v82/checkout/session"
	"github.com/stripe/stripe-go/v82/webhook"
)

// StripeService handles credit purchases through Stripe Checkout.
type StripeService struct {
	cfg     config.StripeConfig
	billing *BillingService
	repo    billing.BillingRepository
}

func NewStripeService(cfg config.StripeConfig, billingService *BillingService, repo billing.BillingRepository) *StripeService {
	if cfg.Enabled() {
		stripe.Key = cfg.SecretKey
	}
	return &StripeService{cfg: cfg, billing: billingService, repo: repo}
}

func (s *StripeService) Enabled() bool { return s.cfg.Enabled() }

// CreateCheckoutSession creates a Stripe Checkout session for buying credits.
func (s *StripeService) CreateCheckoutSession(ctx context.Context, tenantID kernel.TenantID, userEmail string, req billing.CreateCheckoutRequest) (*billing.CheckoutSessionResponse, error) {
	if !s.Enabled() {
		return nil, errx.Business("Stripe payments are not configured")
	}
	if req.AmountUSD < s.cfg.MinTopUpUSD {
		return nil, errx.Validation(fmt.Sprintf("Minimum top-up is $%.2f", s.cfg.MinTopUpUSD)).WithDetail("field", "amount_usd")
	}
	if req.AmountUSD > s.cfg.MaxTopUpUSD {
		return nil, errx.Validation(fmt.Sprintf("Maximum top-up is $%.2f", s.cfg.MaxTopUpUSD)).WithDetail("field", "amount_usd")
	}

	amountCents := int64(math.Round(req.AmountUSD * 100))

	params := &stripe.CheckoutSessionParams{
		Mode: stripe.String(string(stripe.CheckoutSessionModePayment)),
		LineItems: []*stripe.CheckoutSessionLineItemParams{
			{
				PriceData: &stripe.CheckoutSessionLineItemPriceDataParams{
					Currency: stripe.String(string(stripe.CurrencyUSD)),
					ProductData: &stripe.CheckoutSessionLineItemPriceDataProductDataParams{
						Name:        stripe.String("freerouter credits"),
						Description: stripe.String(fmt.Sprintf("$%.2f of API credits", req.AmountUSD)),
					},
					UnitAmount: stripe.Int64(amountCents),
				},
				Quantity: stripe.Int64(1),
			},
		},
		SuccessURL: stripe.String(s.cfg.SuccessURL),
		CancelURL:  stripe.String(s.cfg.CancelURL),
		Metadata: map[string]string{
			"tenant_id":  tenantID.String(),
			"amount_usd": fmt.Sprintf("%.2f", req.AmountUSD),
		},
	}
	if userEmail != "" {
		params.CustomerEmail = stripe.String(userEmail)
	}

	sess, err := session.New(params)
	if err != nil {
		return nil, errx.Wrap(err, "failed to create Stripe checkout session", errx.TypeExternal)
	}

	return &billing.CheckoutSessionResponse{
		SessionID: sess.ID,
		URL:       sess.URL,
	}, nil
}

// HandleWebhook verifies and processes a Stripe webhook event.
// Credits the tenant balance on checkout.session.completed (paid).
func (s *StripeService) HandleWebhook(ctx context.Context, payload []byte, signature string) error {
	if !s.Enabled() || s.cfg.WebhookSecret == "" {
		return errx.Business("Stripe webhooks are not configured")
	}

	event, err := webhook.ConstructEvent(payload, signature, s.cfg.WebhookSecret)
	if err != nil {
		return errx.Wrap(err, "invalid Stripe webhook signature", errx.TypeAuthorization)
	}

	if event.Type != "checkout.session.completed" {
		return nil // ignore other events
	}

	var sess stripe.CheckoutSession
	if err := json.Unmarshal(event.Data.Raw, &sess); err != nil {
		return errx.Wrap(err, "failed to parse checkout session", errx.TypeInternal)
	}

	if sess.PaymentStatus != stripe.CheckoutSessionPaymentStatusPaid {
		logx.Infof("Stripe checkout %s completed but not paid (status=%s), skipping", sess.ID, sess.PaymentStatus)
		return nil
	}

	if sess.Currency != stripe.CurrencyUSD {
		return errx.Business("unexpected currency in checkout session: " + string(sess.Currency))
	}

	tenantIDStr := sess.Metadata["tenant_id"]
	if tenantIDStr == "" {
		return errx.Business("checkout session missing tenant_id metadata")
	}

	// Idempotency: skip if this session was already credited
	referenceID := "stripe:" + sess.ID
	exists, err := s.repo.HasTransactionWithReference(ctx, referenceID)
	if err != nil {
		return err
	}
	if exists {
		logx.Infof("Stripe checkout %s already credited, skipping", sess.ID)
		return nil
	}

	amountUSD := float64(sess.AmountTotal) / 100
	if amountUSD <= 0 {
		return errx.Business("invalid amount in checkout session")
	}

	_, _, err = s.billing.TopUp(ctx, kernel.NewTenantID(tenantIDStr), billing.TopUpRequest{
		Amount:      amountUSD,
		Description: fmt.Sprintf("Credit purchase of $%.2f via Stripe", amountUSD),
		ReferenceID: referenceID,
	})
	if err != nil {
		return err
	}

	logx.Infof("Credited $%.2f to tenant %s (stripe session %s)", amountUSD, tenantIDStr, sess.ID)
	return nil
}
