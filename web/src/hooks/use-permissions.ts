import { useAuth } from "@/hooks/use-auth"

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

export interface Permissions {
  /** All permissions of the principal as a Set for O(1) lookup. */
  set: Set<string>
  /** Returns true if the user has the given permission. */
  has: (perm: string) => boolean
  /** Returns true if the user has ALL of the given permissions. */
  hasAll: (...perms: string[]) => boolean
  /** Returns true if the user has ANY of the given permissions. */
  hasAny: (...perms: string[]) => boolean
}

/**
 * Permission checker for the signed-in principal: the IAMKit token's
 * permissions (iam mode) or the session's (token/none modes).
 */
export function usePermissions(): Permissions {
  const { permissions: perms } = useAuth()
  return {
    set: perms,
    has: (p: string) => perms.has(p),
    hasAll: (...ps: string[]) => ps.every((p) => perms.has(p)),
    hasAny: (...ps: string[]) => ps.some((p) => perms.has(p)),
  }
}

