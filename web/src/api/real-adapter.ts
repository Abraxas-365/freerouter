import { api } from "./client"
import type {
  ApiPort, ProviderPort, ModelPort, MappingPort, ModelFallbackPort,
  ProviderKeyPort, BillingPort, UsagePort, GuardrailsPort,
  WebhooksPort, ApiKeysPort, GatewayConfigPort, UsersPort,
  RolesPort, TenantsPort, InvitationsPort, ScopesPort,
} from "./ports"
import type {
  Provider, Model, ModelFallback,
  ProviderKey, Transaction,
  UsageLog, GuardrailRule,
  GuardrailViolation, WebhookConfig, WebhookDelivery,
  ApiKey, User, Role, TenantResponse, Invitation,
} from "./types"

// Helper to build query strings
function qs(params: Record<string, unknown>): string {
  const entries = Object.entries(params).filter(([, v]) => v !== undefined)
  if (entries.length === 0) return ""
  return "?" + entries.map(([k, v]) => `${k}=${encodeURIComponent(String(v))}`).join("&")
}

// ============================================================================
// Real Adapters
// ============================================================================

const providers: ProviderPort = {
  list: () => api.get<{ providers: Provider[]; total: number }>("/providers").then(r => ({ data: r.providers, total: r.total })),
  get: (id) => api.get(`/providers/${id}`),
  create: (req) => api.post("/providers", req),
  update: (id, req) => api.put(`/providers/${id}`, req),
  delete: (id) => api.del(`/providers/${id}`),
}

const models: ModelPort = {
  list: () => api.get<{ models: Model[]; total: number }>("/models").then(r => ({ data: r.models, total: r.total })),
  get: (id) => api.get(`/models/${id}`),
  getWithMappings: (id) => api.get(`/models/${id}/mappings`),
  create: (req) => api.post("/models", req),
  update: (id, req) => api.put(`/models/${id}`, req),
  delete: (id) => api.del(`/models/${id}`),
}

const mappings: MappingPort = {
  get: (id) => api.get(`/mappings/${id}`),
  create: (req) => api.post("/mappings", req),
  update: (id, req) => api.put(`/mappings/${id}`, req),
  delete: (id) => api.del(`/mappings/${id}`),
}

const modelFallbacks: ModelFallbackPort = {
  listByModel: (modelId) => api.get<{ fallbacks: ModelFallback[]; total: number }>(`/model-fallbacks/by-model/${modelId}`).then(r => ({ data: r.fallbacks, total: r.total })),
  create: (req) => api.post("/model-fallbacks", req),
  delete: (id) => api.del(`/model-fallbacks/${id}`),
}

const providerKeys: ProviderKeyPort = {
  listByProvider: (providerId) => api.get<{ keys: ProviderKey[]; total: number }>(`/provider-keys/by-provider/${providerId}`).then(r => ({ data: r.keys, total: r.total })),
  listByTenant: (tenantId) => api.get<{ keys: ProviderKey[]; total: number }>(`/provider-keys/by-tenant/${tenantId}`).then(r => ({ data: r.keys, total: r.total })),
  listManaged: () => api.get<{ keys: ProviderKey[]; total: number }>("/provider-keys/managed").then(r => ({ data: r.keys, total: r.total })),
  get: (id) => api.get(`/provider-keys/${id}`),
  create: (req) => api.post("/provider-keys", req),
  update: (id, req) => api.put(`/provider-keys/${id}`, req),
  delete: (id) => api.del(`/provider-keys/${id}`),
  test: (id) => api.post(`/provider-keys/${id}/test`),
}

const billing: BillingPort = {
  getBalance: () => api.get("/billing/balance"),
  topUp: (req) => api.post("/billing/top-up", req),
  adjust: (req) => api.post("/billing/adjust", req),
  listTransactions: (params = {}) => api.get<{ transactions: Transaction[]; total: number }>(`/billing/transactions${qs(params)}`).then(r => ({ data: r.transactions, total: r.total })),
  getSpendingLimit: (tenantId) => api.get(`/spending-limits/${tenantId}`),
  upsertSpendingLimit: (tenantId, req) => api.put(`/spending-limits/${tenantId}`, req),
  deleteSpendingLimit: (tenantId) => api.del(`/spending-limits/${tenantId}`),
  checkSpending: (tenantId) => api.get(`/spending-limits/${tenantId}/check`),
}

const usage: UsagePort = {
  listLogs: (params = {}) => api.get<{ logs: UsageLog[]; total: number }>(`/usage/logs${qs(params)}`).then(r => ({ data: r.logs, total: r.total })),
  getLog: (id) => api.get(`/usage/logs/${id}`),
  getSummary: (params = {}) => api.get(`/usage/summary${qs(params)}`),
  getRetention: (tenantId) => api.get(`/usage/retention/${tenantId}`),
  upsertRetention: (tenantId, req) => api.put(`/usage/retention/${tenantId}`, req),
  deleteRetention: (tenantId) => api.del(`/usage/retention/${tenantId}`),
}

const guardrails: GuardrailsPort = {
  getConfig: () => api.get("/guardrails/config"),
  upsertConfig: (req) => api.put("/guardrails/config", req),
  listRules: () => api.get<{ rules: GuardrailRule[]; total: number }>("/guardrails/rules").then(r => ({ data: r.rules, total: r.total })),
  createRule: (req) => api.post("/guardrails/rules", req),
  updateRule: (id, req) => api.put(`/guardrails/rules/${id}`, req),
  deleteRule: (id) => api.del(`/guardrails/rules/${id}`),
  listViolations: (params = {}) => api.get<{ violations: GuardrailViolation[]; total: number }>(`/guardrails/violations${qs(params)}`).then(r => ({ data: r.violations, total: r.total })),
  testCheck: (messages) => api.post("/guardrails/test", { messages }),
}

const webhooks: WebhooksPort = {
  list: () => api.get<{ webhooks: WebhookConfig[]; total: number }>("/webhooks").then(r => ({ data: r.webhooks, total: r.total })),
  listEvents: () => api.get<{ events: string[] }>("/webhooks/events").then(r => r.events),
  get: (id) => api.get(`/webhooks/${id}`),
  create: (req) => api.post("/webhooks", req),
  update: (id, req) => api.put(`/webhooks/${id}`, req),
  delete: (id) => api.del(`/webhooks/${id}`),
  listDeliveries: (id, params = {}) => api.get<{ deliveries: WebhookDelivery[]; total: number }>(`/webhooks/${id}/deliveries${qs(params)}`).then(r => ({ data: r.deliveries, total: r.total })),
  test: (id) => api.post(`/webhooks/${id}/test`),
}

const apiKeys: ApiKeysPort = {
  list: () => api.get<{ api_keys: ApiKey[]; total: number }>("/api-keys").then(r => ({ data: r.api_keys, total: r.total })),
  get: (id) => api.get(`/api-keys/${id}`),
  create: (req) => api.post("/api-keys", req),
  update: (id, req) => api.put(`/api-keys/${id}`, req),
  revoke: (id) => api.post(`/api-keys/${id}/revoke`),
  delete: (id) => api.del(`/api-keys/${id}`),
}

const gatewayConfig: GatewayConfigPort = {
  getRateLimit: (tenantId) => api.get(`/rate-limits/${tenantId}`),
  upsertRateLimit: (tenantId, req) => api.put(`/rate-limits/${tenantId}`, req),
  deleteRateLimit: (tenantId) => api.del(`/rate-limits/${tenantId}`),
  getRouting: (tenantId) => api.get(`/routing/${tenantId}`),
  upsertRouting: (tenantId, req) => api.put(`/routing/${tenantId}`, req),
  deleteRouting: (tenantId) => api.del(`/routing/${tenantId}`),
  invalidateCache: (tenantId) => tenantId ? api.del(`/cache/${tenantId}`) : api.del("/cache"),
  estimateCost: (req) => api.post("/cost/estimate", req),
}

const users: UsersPort = {
  me: () => api.get("/users/me"),
  list: () => api.get<{ users: User[]; total: number }>("/users").then(r => ({ data: r.users, total: r.total })),
  get: (id) => api.get(`/users/${id}`),
  activate: (id) => api.post(`/users/${id}/activate`),
  suspend: (id, reason) => api.post(`/users/${id}/suspend`, { reason }),
  delete: (id) => api.del(`/users/${id}`),
}

const roles: RolesPort = {
  list: () => api.get<{ roles: Role[]; total: number }>("/roles").then(r => ({ data: r.roles, total: r.total })),
  get: (id) => api.get(`/roles/${id}`),
  create: (req) => api.post("/roles", req),
  update: (id, req) => api.put(`/roles/${id}`, req),
  delete: (id) => api.del(`/roles/${id}`),
  assign: (roleId, req) => api.post(`/roles/${roleId}/assign`, req),
  unassign: (roleId, userId) => api.del(`/roles/${roleId}/users/${userId}`),
  getUserRoles: (userId) => api.get(`/users/${userId}/roles`),
}

const tenants: TenantsPort = {
  list: () => api.get<{ tenants: TenantResponse[]; total: number }>("/tenants").then(r => ({ data: r.tenants, total: r.total })),
  get: (id) => api.get(`/tenants/${id}`),
  create: (req) => api.post("/tenants", req),
  update: (id, req) => api.put(`/tenants/${id}`, req),
  delete: (id) => api.del(`/tenants/${id}`),
  suspend: (id, reason) => api.post(`/tenants/${id}/suspend`, { reason }),
  activate: (id, comments) => api.post(`/tenants/${id}/activate`, { comments }),
  getStats: (id) => api.get(`/tenants/${id}/stats`),
  getUsage: (id) => api.get(`/tenants/${id}/usage`),
  getUsers: (id) => api.get<{ users: User[]; total: number }>(`/tenants/${id}/users`).then(r => ({ data: r.users, total: r.total })),
  getConfig: (id) => api.get(`/tenants/${id}/config`),
  setConfig: (id, key, value) => api.put(`/tenants/${id}/config`, { key, value }),
  deleteConfig: (id, key) => api.del(`/tenants/${id}/config/${key}`),
}

const invitations: InvitationsPort = {
  list: () => api.get<{ invitations: Array<{ invitation: Invitation }>; total: number }>("/invitations").then(r => ({ data: r.invitations.map(i => i.invitation), total: r.total })),
  listPending: () => api.get<{ invitations: Array<{ invitation: Invitation }>; total: number }>("/invitations/pending").then(r => ({ data: r.invitations.map(i => i.invitation), total: r.total })),
  get: (id) => api.get<{ invitation: Invitation }>(`/invitations/${id}`).then(r => r.invitation),
  create: (req) => api.post<{ invitation: Invitation }>("/invitations", req).then(r => r.invitation),
  delete: (id) => api.del(`/invitations/${id}`),
  revoke: (id, reason) => api.post(`/invitations/${id}/revoke`, { reason }),
  validateToken: (token) => api.get(`/invitations/public/validate?token=${encodeURIComponent(token)}`),
  getByToken: (token) => api.get<{ invitation: Invitation }>(`/invitations/public/token/${token}`).then(r => r.invitation),
}

const scopes: ScopesPort = {
  listAvailable: () => api.get("/scopes"),
  getUserScopes: (userId) => api.get(`/users/${userId}/scopes`),
  setUserScopes: (userId, scopeList) => api.put(`/users/${userId}/scopes`, { scopes: scopeList }),
  addUserScopes: (userId, scopeList) => api.post(`/users/${userId}/scopes`, { scopes: scopeList }),
  removeUserScopes: (userId) => api.del(`/users/${userId}/scopes`),
}

// ============================================================================
// Combined Real API
// ============================================================================

export const realApi: ApiPort = {
  providers,
  models,
  mappings,
  modelFallbacks,
  providerKeys,
  billing,
  usage,
  guardrails,
  webhooks,
  apiKeys,
  gatewayConfig,
  users,
  roles,
  tenants,
  invitations,
  scopes,
}
