package config

// StripeConfig holds Stripe payment settings.
type StripeConfig struct {
	SecretKey     string // sk_live_... / sk_test_...
	WebhookSecret string // whsec_... (from Stripe webhook endpoint config)
	SuccessURL    string // where Stripe redirects after successful payment
	CancelURL     string // where Stripe redirects if the user cancels
	MinTopUpUSD   float64
	MaxTopUpUSD   float64
}

func (c StripeConfig) Enabled() bool {
	return c.SecretKey != ""
}

func loadStripeConfig() StripeConfig {
	return StripeConfig{
		SecretKey:     getEnv("STRIPE_SECRET_KEY", ""),
		WebhookSecret: getEnv("STRIPE_WEBHOOK_SECRET", ""),
		SuccessURL:    getEnv("STRIPE_SUCCESS_URL", "http://localhost:5173/billing?checkout=success"),
		CancelURL:     getEnv("STRIPE_CANCEL_URL", "http://localhost:5173/billing?checkout=cancelled"),
		MinTopUpUSD:   getEnvFloat("STRIPE_MIN_TOPUP_USD", 5),
		MaxTopUpUSD:   getEnvFloat("STRIPE_MAX_TOPUP_USD", 10000),
	}
}
