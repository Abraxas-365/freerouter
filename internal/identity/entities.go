package identity

// ── Provider ────────────────────────────────────────────────────────

type providerTag struct{}

// ProviderID identifies an LLM provider (OpenAI, Anthropic, etc.).
type ProviderID = ID[providerTag]

func NewProviderID() ProviderID                      { return NewID[providerTag]() }
func ParseProviderID(raw string) (ProviderID, error) { return ParseID[providerTag](raw) }
func MustParseProviderID(raw string) ProviderID      { return MustParseID[providerTag](raw) }

// ── Model ───────────────────────────────────────────────────────────

type modelTag struct{}

// ModelID identifies a model variant (gpt-4o, claude-sonnet, etc.).
type ModelID = ID[modelTag]

func NewModelID() ModelID                      { return NewID[modelTag]() }
func ParseModelID(raw string) (ModelID, error) { return ParseID[modelTag](raw) }
func MustParseModelID(raw string) ModelID      { return MustParseID[modelTag](raw) }

// ── ProviderKey ─────────────────────────────────────────────────────

type providerKeyTag struct{}

// ProviderKeyID identifies an API key for a provider.
type ProviderKeyID = ID[providerKeyTag]

func NewProviderKeyID() ProviderKeyID                      { return NewID[providerKeyTag]() }
func ParseProviderKeyID(raw string) (ProviderKeyID, error) { return ParseID[providerKeyTag](raw) }
func MustParseProviderKeyID(raw string) ProviderKeyID      { return MustParseID[providerKeyTag](raw) }

// ── UsageLog ────────────────────────────────────────────────────────

type usageLogTag struct{}

// UsageLogID identifies a usage log entry.
type UsageLogID = ID[usageLogTag]

func NewUsageLogID() UsageLogID                      { return NewID[usageLogTag]() }
func ParseUsageLogID(raw string) (UsageLogID, error) { return ParseID[usageLogTag](raw) }
func MustParseUsageLogID(raw string) UsageLogID      { return MustParseID[usageLogTag](raw) }

// ── GuardrailConfig ─────────────────────────────────────────────────

type guardrailConfigTag struct{}

// GuardrailConfigID identifies a guardrail configuration.
type GuardrailConfigID = ID[guardrailConfigTag]

func NewGuardrailConfigID() GuardrailConfigID { return NewID[guardrailConfigTag]() }
func ParseGuardrailConfigID(raw string) (GuardrailConfigID, error) {
	return ParseID[guardrailConfigTag](raw)
}
func MustParseGuardrailConfigID(raw string) GuardrailConfigID {
	return MustParseID[guardrailConfigTag](raw)
}

// ── GuardrailRule ────────────────────────────────────────────────────

type guardrailRuleTag struct{}

// GuardrailRuleID identifies a custom guardrail rule.
type GuardrailRuleID = ID[guardrailRuleTag]

func NewGuardrailRuleID() GuardrailRuleID                      { return NewID[guardrailRuleTag]() }
func ParseGuardrailRuleID(raw string) (GuardrailRuleID, error) { return ParseID[guardrailRuleTag](raw) }
func MustParseGuardrailRuleID(raw string) GuardrailRuleID      { return MustParseID[guardrailRuleTag](raw) }

// ── GuardrailViolation ───────────────────────────────────────────────

type guardrailViolationTag struct{}

// GuardrailViolationID identifies a logged guardrail violation.
type GuardrailViolationID = ID[guardrailViolationTag]

func NewGuardrailViolationID() GuardrailViolationID { return NewID[guardrailViolationTag]() }
func ParseGuardrailViolationID(raw string) (GuardrailViolationID, error) {
	return ParseID[guardrailViolationTag](raw)
}
func MustParseGuardrailViolationID(raw string) GuardrailViolationID {
	return MustParseID[guardrailViolationTag](raw)
}

// ── Mapping ─────────────────────────────────────────────────────────

type mappingTag struct{}

// MappingID identifies a model-provider mapping.
type MappingID = ID[mappingTag]

func NewMappingID() MappingID                      { return NewID[mappingTag]() }
func ParseMappingID(raw string) (MappingID, error) { return ParseID[mappingTag](raw) }
func MustParseMappingID(raw string) MappingID      { return MustParseID[mappingTag](raw) }

// ── ModelFallback ───────────────────────────────────────────────────

type modelFallbackTag struct{}

// ModelFallbackID identifies a model fallback relationship.
type ModelFallbackID = ID[modelFallbackTag]

func NewModelFallbackID() ModelFallbackID                      { return NewID[modelFallbackTag]() }
func ParseModelFallbackID(raw string) (ModelFallbackID, error) { return ParseID[modelFallbackTag](raw) }
func MustParseModelFallbackID(raw string) ModelFallbackID      { return MustParseID[modelFallbackTag](raw) }

// ── RateLimitConfig ──────────────────────────────────────────────────

type rateLimitConfigTag struct{}

// RateLimitConfigID identifies a rate limit configuration.
type RateLimitConfigID = ID[rateLimitConfigTag]

func NewRateLimitConfigID() RateLimitConfigID { return NewID[rateLimitConfigTag]() }
func ParseRateLimitConfigID(raw string) (RateLimitConfigID, error) {
	return ParseID[rateLimitConfigTag](raw)
}
func MustParseRateLimitConfigID(raw string) RateLimitConfigID {
	return MustParseID[rateLimitConfigTag](raw)
}

// ── RoutingConfig ────────────────────────────────────────────────────

type routingConfigTag struct{}

// RoutingConfigID identifies a per-subject routing strategy configuration.
type RoutingConfigID = ID[routingConfigTag]

func NewRoutingConfigID() RoutingConfigID                      { return NewID[routingConfigTag]() }
func ParseRoutingConfigID(raw string) (RoutingConfigID, error) { return ParseID[routingConfigTag](raw) }
func MustParseRoutingConfigID(raw string) RoutingConfigID      { return MustParseID[routingConfigTag](raw) }

// ── APIKey ──────────────────────────────────────────────────────────

type apiKeyTag struct{}

// APIKeyID identifies an API key for gateway access.
type APIKeyID = ID[apiKeyTag]

func NewAPIKeyID() APIKeyID                      { return NewID[apiKeyTag]() }
func ParseAPIKeyID(raw string) (APIKeyID, error) { return ParseID[apiKeyTag](raw) }
func MustParseAPIKeyID(raw string) APIKeyID      { return MustParseID[apiKeyTag](raw) }

// ── Webhook ─────────────────────────────────────────────────────────

type webhookTag struct{}

// WebhookID identifies a webhook subscription.
type WebhookID = ID[webhookTag]

func NewWebhookID() WebhookID                      { return NewID[webhookTag]() }
func ParseWebhookID(raw string) (WebhookID, error) { return ParseID[webhookTag](raw) }
func MustParseWebhookID(raw string) WebhookID      { return MustParseID[webhookTag](raw) }

// ── WebhookDelivery ─────────────────────────────────────────────────

type webhookDeliveryTag struct{}

// WebhookDeliveryID identifies a single webhook delivery attempt record.
type WebhookDeliveryID = ID[webhookDeliveryTag]

func NewWebhookDeliveryID() WebhookDeliveryID { return NewID[webhookDeliveryTag]() }
func ParseWebhookDeliveryID(raw string) (WebhookDeliveryID, error) {
	return ParseID[webhookDeliveryTag](raw)
}
func MustParseWebhookDeliveryID(raw string) WebhookDeliveryID {
	return MustParseID[webhookDeliveryTag](raw)
}

// ── RetentionConfig ─────────────────────────────────────────────────

type retentionConfigTag struct{}

// RetentionConfigID identifies the global usage-log data retention configuration.
type RetentionConfigID = ID[retentionConfigTag]

func NewRetentionConfigID() RetentionConfigID { return NewID[retentionConfigTag]() }
func ParseRetentionConfigID(raw string) (RetentionConfigID, error) {
	return ParseID[retentionConfigTag](raw)
}
func MustParseRetentionConfigID(raw string) RetentionConfigID {
	return MustParseID[retentionConfigTag](raw)
}
