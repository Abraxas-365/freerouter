package scopes

// Domain-specific scopes for the freerouter LLM gateway platform.

const (
	// Provider management scopes
	ScopeProvidersAll    = "providers:*"
	ScopeProvidersRead   = "providers:read"
	ScopeProvidersWrite  = "providers:write"
	ScopeProvidersDelete = "providers:delete"

	// Model management scopes
	ScopeModelsAll    = "models:*"
	ScopeModelsRead   = "models:read"
	ScopeModelsWrite  = "models:write"
	ScopeModelsDelete = "models:delete"

	// Provider key scopes
	ScopeProviderKeysAll    = "provider-keys:*"
	ScopeProviderKeysRead   = "provider-keys:read"
	ScopeProviderKeysWrite  = "provider-keys:write"
	ScopeProviderKeysDelete = "provider-keys:delete"

	// Gateway scopes
	ScopeGatewayAll  = "gateway:*"
	ScopeGatewayRead = "gateway:read"
	ScopeGatewayChat = "gateway:chat"

	// Billing scopes
	ScopeBillingAll   = "billing:*"
	ScopeBillingRead  = "billing:read"
	ScopeBillingWrite = "billing:write"
	ScopeBillingAdmin = "billing:admin"

	// Rate limit scopes
	ScopeRateLimitsAll   = "rate-limits:*"
	ScopeRateLimitsRead  = "rate-limits:read"
	ScopeRateLimitsWrite = "rate-limits:write"

	// Usage scopes
	ScopeUsageAll   = "usage:*"
	ScopeUsageRead  = "usage:read"
	ScopeUsageWrite = "usage:write"

	// Guardrails scopes
	ScopeGuardrailsAll   = "guardrails:*"
	ScopeGuardrailsRead  = "guardrails:read"
	ScopeGuardrailsWrite = "guardrails:write"

	// Webhook scopes
	ScopeWebhooksAll   = "webhooks:*"
	ScopeWebhooksRead  = "webhooks:read"
	ScopeWebhooksWrite = "webhooks:write"
)

// DomainScopeCategories organizes domain-specific scopes by category.
var DomainScopeCategories = map[string][]string{
	"Providers": {
		ScopeProvidersAll,
		ScopeProvidersRead,
		ScopeProvidersWrite,
		ScopeProvidersDelete,
	},
	"Models": {
		ScopeModelsAll,
		ScopeModelsRead,
		ScopeModelsWrite,
		ScopeModelsDelete,
	},
	"Provider Keys": {
		ScopeProviderKeysAll,
		ScopeProviderKeysRead,
		ScopeProviderKeysWrite,
		ScopeProviderKeysDelete,
	},
	"Gateway": {
		ScopeGatewayAll,
		ScopeGatewayRead,
		ScopeGatewayChat,
	},
	"Billing": {
		ScopeBillingAll,
		ScopeBillingRead,
		ScopeBillingWrite,
		ScopeBillingAdmin,
	},
	"Rate Limits": {
		ScopeRateLimitsAll,
		ScopeRateLimitsRead,
		ScopeRateLimitsWrite,
	},
	"Usage": {
		ScopeUsageAll,
		ScopeUsageRead,
		ScopeUsageWrite,
	},
	"Guardrails": {
		ScopeGuardrailsAll,
		ScopeGuardrailsRead,
		ScopeGuardrailsWrite,
	},
	"Webhooks": {
		ScopeWebhooksAll,
		ScopeWebhooksRead,
		ScopeWebhooksWrite,
	},
}

// DomainScopeDescriptions provides descriptions for domain scopes.
var DomainScopeDescriptions = map[string]string{
	ScopeProvidersAll:    "Full access to provider management",
	ScopeProvidersRead:   "View providers",
	ScopeProvidersWrite:  "Create and edit providers",
	ScopeProvidersDelete: "Delete providers",

	ScopeModelsAll:    "Full access to model management",
	ScopeModelsRead:   "View models and mappings",
	ScopeModelsWrite:  "Create and edit models and mappings",
	ScopeModelsDelete: "Delete models and mappings",

	ScopeProviderKeysAll:    "Full access to provider key management",
	ScopeProviderKeysRead:   "View provider keys",
	ScopeProviderKeysWrite:  "Create and edit provider keys",
	ScopeProviderKeysDelete: "Delete provider keys",

	ScopeGatewayAll:  "Full access to gateway",
	ScopeGatewayRead: "View gateway configuration",
	ScopeGatewayChat: "Use gateway for LLM requests",

	ScopeBillingAll:   "Full access to billing",
	ScopeBillingRead:  "View billing information",
	ScopeBillingWrite: "Manage billing",
	ScopeBillingAdmin: "Administer billing and spending limits",

	ScopeRateLimitsAll:   "Full access to rate limit management",
	ScopeRateLimitsRead:  "View rate limit configuration",
	ScopeRateLimitsWrite: "Configure rate limits",

	ScopeUsageAll:   "Full access to usage data",
	ScopeUsageRead:  "View usage logs",
	ScopeUsageWrite: "Configure usage data retention policies",

	ScopeGuardrailsAll:   "Full access to guardrails",
	ScopeGuardrailsRead:  "View guardrail configuration",
	ScopeGuardrailsWrite: "Create and edit guardrails",

	ScopeWebhooksAll:   "Full access to webhook management",
	ScopeWebhooksRead:  "View webhooks and deliveries",
	ScopeWebhooksWrite: "Create, edit, and delete webhooks",
}
