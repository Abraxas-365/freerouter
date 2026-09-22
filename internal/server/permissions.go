package server

// ResourcePrefix is the IAMKit resource prefix used when registering permissions.
// JWT claims carry fully-qualified names like "freerouter:gateway:invoke".
const ResourcePrefix = "freerouter"

// Permission constants used across FreeRouter route guards.
// Service accounts are created with a subset of these.
const (
	// Gateway — LLM proxy endpoints (/v1/*)
	PermGatewayInvoke = "freerouter:gateway:invoke" // chat/completions, messages, responses, models, cost, modalities
	PermGatewayWrite  = "freerouter:gateway:write"  // cache invalidation

	// Metrics
	PermMetricsRead = "freerouter:metrics:read"

	// Providers & models (admin)
	PermProvidersRead  = "freerouter:providers:read"
	PermProvidersWrite = "freerouter:providers:write"

	// Provider keys (admin)
	PermProviderKeysRead  = "freerouter:provider-keys:read"
	PermProviderKeysWrite = "freerouter:provider-keys:write"

	// Usage logs
	PermUsageRead  = "freerouter:usage:read"
	PermUsageWrite = "freerouter:usage:write"

	// Rate limits (admin)
	PermRateLimitsRead  = "freerouter:rate-limits:read"
	PermRateLimitsWrite = "freerouter:rate-limits:write"

	// Routing config (admin)
	PermRoutingRead  = "freerouter:routing:read"
	PermRoutingWrite = "freerouter:routing:write"

	// Guardrails (admin)
	PermGuardrailsRead  = "freerouter:guardrails:read"
	PermGuardrailsWrite = "freerouter:guardrails:write"

	// Webhooks (admin)
	PermWebhooksRead  = "freerouter:webhooks:read"
	PermWebhooksWrite = "freerouter:webhooks:write"

	// Service account management (admin)
	PermServiceAccountsRead  = "freerouter:service-accounts:read"
	PermServiceAccountsWrite = "freerouter:service-accounts:write"

	// User management (admin)
	PermUsersRead  = "freerouter:users:read"
	PermUsersWrite = "freerouter:users:write"

	// Role management (admin)
	PermRolesRead  = "freerouter:roles:read"
	PermRolesWrite = "freerouter:roles:write"
)

// ValidPermissions is the set of all permissions FreeRouter recognises.
var ValidPermissions = map[string]bool{
	PermGatewayInvoke:        true,
	PermGatewayWrite:         true,
	PermMetricsRead:          true,
	PermProvidersRead:        true,
	PermProvidersWrite:       true,
	PermProviderKeysRead:     true,
	PermProviderKeysWrite:    true,
	PermUsageRead:            true,
	PermUsageWrite:           true,
	PermRateLimitsRead:       true,
	PermRateLimitsWrite:      true,
	PermRoutingRead:          true,
	PermRoutingWrite:         true,
	PermGuardrailsRead:       true,
	PermGuardrailsWrite:      true,
	PermWebhooksRead:         true,
	PermWebhooksWrite:        true,
	PermServiceAccountsRead:  true,
	PermServiceAccountsWrite: true,
	PermUsersRead:            true,
	PermUsersWrite:           true,
	PermRolesRead:            true,
	PermRolesWrite:           true,
}

// DefaultPermissions is assigned to service accounts when no permissions
// are explicitly provided — allows only gateway usage.
var DefaultPermissions = []string{PermGatewayInvoke}
