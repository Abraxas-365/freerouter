const ACCESS_TOKEN_KEY = "access_token"

/** Permission constants — keep in sync with server/permissions.go */
export const P = {
  GatewayInvoke:       "freerouter:gateway:invoke",
  GatewayWrite:        "freerouter:gateway:write",
  MetricsRead:         "freerouter:metrics:read",
  ProvidersRead:       "freerouter:providers:read",
  ProvidersWrite:      "freerouter:providers:write",
  ProviderKeysRead:    "freerouter:provider-keys:read",
  ProviderKeysWrite:   "freerouter:provider-keys:write",
  UsageRead:           "freerouter:usage:read",
  UsageWrite:          "freerouter:usage:write",
  RateLimitsRead:      "freerouter:rate-limits:read",
  RateLimitsWrite:     "freerouter:rate-limits:write",
  RoutingRead:         "freerouter:routing:read",
  RoutingWrite:        "freerouter:routing:write",
  GuardrailsRead:      "freerouter:guardrails:read",
  GuardrailsWrite:     "freerouter:guardrails:write",
  WebhooksRead:        "freerouter:webhooks:read",
  WebhooksWrite:       "freerouter:webhooks:write",
  ServiceAccountsRead: "freerouter:service-accounts:read",
  ServiceAccountsWrite:"freerouter:service-accounts:write",
  UsersRead:           "freerouter:users:read",
  UsersWrite:          "freerouter:users:write",
  RolesRead:           "freerouter:roles:read",
  RolesWrite:          "freerouter:roles:write",
} as const

/** Decode the JWT payload without verification (validation happens server-side). */
function decodePayload(token: string): Record<string, unknown> | null {
  try {
    const parts = token.split(".")
    if (parts.length !== 3) return null
    const payload = atob(parts[1].replace(/-/g, "+").replace(/_/g, "/"))
    return JSON.parse(payload)
  } catch {
    return null
  }
}

export interface Permissions {
  /** All permissions from the JWT as a Set for O(1) lookup. */
  set: Set<string>
  /** Returns true if the user has the given permission. */
  has: (perm: string) => boolean
  /** Returns true if the user has ALL of the given permissions. */
  hasAll: (...perms: string[]) => boolean
  /** Returns true if the user has ANY of the given permissions. */
  hasAny: (...perms: string[]) => boolean
}

/**
 * Decodes the stored access token and returns a permission checker.
 * Safe to call when unauthenticated — returns an empty set.
 * Re-evaluates on re-mount (login/logout causes remount via AuthProvider).
 */
export function usePermissions(): Permissions {
  const token = localStorage.getItem(ACCESS_TOKEN_KEY)
  if (!token) return empty()

  const payload = decodePayload(token)
  if (!payload) return empty()

  const perms = Array.isArray(payload.permissions)
    ? new Set<string>(payload.permissions as string[])
    : new Set<string>()

  return {
    set: perms,
    has: (p: string) => perms.has(p),
    hasAll: (...ps: string[]) => ps.every((p) => perms.has(p)),
    hasAny: (...ps: string[]) => ps.some((p) => perms.has(p)),
  }
}

function empty(): Permissions {
  const s = new Set<string>()
  return { set: s, has: () => false, hasAll: () => false, hasAny: () => false }
}
