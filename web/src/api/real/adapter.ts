import { api, qs } from "./client"
import type { ApiPort } from "../ports"
import type {
  Provider, CreateProviderRequest, UpdateProviderRequest, ProviderFilter,
  Model, CreateModelRequest, UpdateModelRequest, ModelFilter,
  ModelProviderMapping, CreateMappingRequest, UpdateMappingRequest, MappingFilter,
  ModelFallback, CreateFallbackRequest,
  ProviderKey, CreateProviderKeyRequest, UpdateProviderKeyRequest, ProviderKeyFilter,
  RateLimitConfig, CreateRateLimitRequest, UpdateRateLimitRequest, RateLimitFilter,
  UsageLog, UsageQuery, UsageSummaryResponse,
  GuardrailConfig, UpsertGuardrailConfigRequest,
  GuardrailRule, CreateGuardrailRuleRequest, UpdateGuardrailRuleRequest,
  GuardrailViolation,
  WebhookConfig, CreateWebhookRequest, UpdateWebhookRequest,
  CreateWebhookResponse, WebhookTestResponse, WebhookDelivery,
  ServiceAccount, ServiceAccountCredential, CreateServiceAccountRequest, Application,
  AccessUser, CreateUserRequest, UpdateUserRequest,
  AccessRole, CreateRoleRequest, UpdateRoleRequest,
  RoleAssignment, AssignRoleRequest,
  GatewayModelList,
  Paginated, PageParams,
} from "../types"

// =============================================================================
// Providers
// =============================================================================

const providers = {
  list: (filter: ProviderFilter = {}) =>
    api.get<Paginated<Provider>>(`/api/v1/providers${qs(filter as Record<string, unknown>)}`),
  get: (id: string) => api.get<Provider>(`/api/v1/providers/${id}`),
  create: (req: CreateProviderRequest) => api.post<{ id: string }>("/api/v1/providers", req),
  update: (id: string, req: UpdateProviderRequest) => api.put<void>(`/api/v1/providers/${id}`, req),
  delete: (id: string) => api.del<void>(`/api/v1/providers/${id}`),
}

// =============================================================================
// Models
// =============================================================================

const models = {
  list: (filter: ModelFilter = {}) =>
    api.get<Paginated<Model>>(`/api/v1/models${qs(filter as Record<string, unknown>)}`),
  get: (id: string) => api.get<Model>(`/api/v1/models/${id}`),
  create: (req: CreateModelRequest) => api.post<{ id: string }>("/api/v1/models", req),
  update: (id: string, req: UpdateModelRequest) => api.put<void>(`/api/v1/models/${id}`, req),
  delete: (id: string) => api.del<void>(`/api/v1/models/${id}`),
}

// =============================================================================
// Mappings
// =============================================================================

const mappings = {
  list: (filter: MappingFilter = {}) =>
    api.get<Paginated<ModelProviderMapping>>(`/api/v1/mappings${qs(filter as Record<string, unknown>)}`),
  get: (id: string) => api.get<ModelProviderMapping>(`/api/v1/mappings/${id}`),
  create: (req: CreateMappingRequest) => api.post<{ id: string }>("/api/v1/mappings", req),
  update: (id: string, req: UpdateMappingRequest) => api.put<void>(`/api/v1/mappings/${id}`, req),
  delete: (id: string) => api.del<void>(`/api/v1/mappings/${id}`),
}

// =============================================================================
// Model Fallbacks
// =============================================================================

const modelFallbacks = {
  listByModel: (modelId: string) => api.get<ModelFallback[]>(`/api/v1/model-fallbacks/by-model/${modelId}`),
  create: (req: CreateFallbackRequest) => api.post<{ id: string }>("/api/v1/model-fallbacks", req),
  delete: (id: string) => api.del<void>(`/api/v1/model-fallbacks/${id}`),
}

// =============================================================================
// Provider Keys
// =============================================================================

const providerKeys = {
  list: (filter: ProviderKeyFilter = {}) =>
    api.get<Paginated<ProviderKey>>(`/api/v1/provider-keys${qs(filter as Record<string, unknown>)}`),
  get: (id: string) => api.get<ProviderKey>(`/api/v1/provider-keys/${id}`),
  create: (req: CreateProviderKeyRequest) => api.post<{ id: string }>("/api/v1/provider-keys", req),
  update: (id: string, req: UpdateProviderKeyRequest) => api.put<void>(`/api/v1/provider-keys/${id}`, req),
  delete: (id: string) => api.del<void>(`/api/v1/provider-keys/${id}`),
}

// =============================================================================
// Rate Limits
// =============================================================================

const rateLimits = {
  list: (filter: RateLimitFilter = {}) =>
    api.get<Paginated<RateLimitConfig>>(`/api/v1/rate-limits${qs(filter as Record<string, unknown>)}`),
  get: (id: string) => api.get<RateLimitConfig>(`/api/v1/rate-limits/${id}`),
  create: (req: CreateRateLimitRequest) => api.post<RateLimitConfig>("/api/v1/rate-limits", req),
  update: (id: string, req: UpdateRateLimitRequest) => api.patch<RateLimitConfig>(`/api/v1/rate-limits/${id}`, req),
  delete: (id: string) => api.del<void>(`/api/v1/rate-limits/${id}`),
}

// =============================================================================
// Usage
// =============================================================================

const usage = {
  list: (query: UsageQuery = {}) =>
    api.get<Paginated<UsageLog>>(`/api/v1/usage${qs(query as Record<string, unknown>)}`),
  get: (id: string) => api.get<UsageLog>(`/api/v1/usage/${id}`),
  getSummary: (params: { from?: string; to?: string } = {}) =>
    api.get<UsageSummaryResponse>(`/api/v1/usage/summary${qs(params)}`),
}

// =============================================================================
// Guardrails
// =============================================================================

const guardrails = {
  getConfig: () => api.get<GuardrailConfig>("/api/v1/guardrails/config"),
  upsertConfig: (req: UpsertGuardrailConfigRequest) => api.put<GuardrailConfig>("/api/v1/guardrails/config", req),
  listRules: () => api.get<GuardrailRule[]>("/api/v1/guardrails/rules"),
  getRule: (id: string) => api.get<GuardrailRule>(`/api/v1/guardrails/rules/${id}`),
  createRule: (req: CreateGuardrailRuleRequest) => api.post<GuardrailRule>("/api/v1/guardrails/rules", req),
  updateRule: (id: string, req: UpdateGuardrailRuleRequest) => api.patch<GuardrailRule>(`/api/v1/guardrails/rules/${id}`, req),
  deleteRule: (id: string) => api.del<void>(`/api/v1/guardrails/rules/${id}`),
  listViolations: (params: PageParams = {}) =>
    api.get<Paginated<GuardrailViolation>>(`/api/v1/guardrails/violations${qs(params as Record<string, unknown>)}`),
}

// =============================================================================
// Webhooks
// =============================================================================

const webhooks = {
  list: (params: PageParams = {}) =>
    api.get<Paginated<WebhookConfig>>(`/api/v1/webhooks${qs(params as Record<string, unknown>)}`),
  get: (id: string) => api.get<WebhookConfig>(`/api/v1/webhooks/${id}`),
  create: (req: CreateWebhookRequest) => api.post<CreateWebhookResponse>("/api/v1/webhooks", req),
  update: (id: string, req: UpdateWebhookRequest) => api.patch<WebhookConfig>(`/api/v1/webhooks/${id}`, req),
  delete: (id: string) => api.del<void>(`/api/v1/webhooks/${id}`),
  listEvents: async () => (await api.get<{ events: string[] }>("/api/v1/webhooks/events")).events,
  listDeliveries: (webhookId: string, params: PageParams = {}) =>
    api.get<Paginated<WebhookDelivery>>(`/api/v1/webhooks/${webhookId}/deliveries${qs(params as Record<string, unknown>)}`),
  test: (id: string) => api.post<WebhookTestResponse>(`/api/v1/webhooks/${id}/test`),
}

// =============================================================================
// Gateway
// =============================================================================

const gateway = {
  listModels: () => api.get<GatewayModelList>("/v1/models"),
}

// =============================================================================
// Service Accounts
// =============================================================================

const serviceAccounts = {
  list: () => api.get<ServiceAccount[]>("/api/v1/service-accounts"),
  create: (req: CreateServiceAccountRequest) => api.post<ServiceAccountCredential>("/api/v1/service-accounts", req),
  revoke: (id: string) => api.del<void>(`/api/v1/service-accounts/${id}`),
  listApplications: () => api.get<Application[]>("/api/v1/service-accounts/applications"),
}

// =============================================================================
// Access Management (Users, Roles, Role Assignments)
// =============================================================================

const access = {
  listUsers: () => api.get<AccessUser[]>("/api/v1/access/users"),
  getUser: (id: string) => api.get<AccessUser>(`/api/v1/access/users/${id}`),
  createUser: (req: CreateUserRequest) => api.post<AccessUser>("/api/v1/access/users", req),
  updateUser: (id: string, req: UpdateUserRequest) => api.patch<void>(`/api/v1/access/users/${id}`, req),
  suspendUser: (id: string) => api.del<void>(`/api/v1/access/users/${id}`),

  listRoles: () => api.get<AccessRole[]>("/api/v1/access/roles"),
  createRole: (req: CreateRoleRequest) => api.post<AccessRole>("/api/v1/access/roles", req),
  updateRole: (id: string, req: UpdateRoleRequest) => api.put<void>(`/api/v1/access/roles/${id}`, req),
  deleteRole: (id: string) => api.del<void>(`/api/v1/access/roles/${id}`),

  listAssignments: () => api.get<RoleAssignment[]>("/api/v1/access/role-assignments"),
  assignRole: (req: AssignRoleRequest) => api.post<void>("/api/v1/access/role-assignments", req),
  unassignRole: (req: AssignRoleRequest) => api.del<void>("/api/v1/access/role-assignments", req),
}

// =============================================================================
// Combined Real API
// =============================================================================

export const realApi: ApiPort = {
  providers,
  models,
  mappings,
  modelFallbacks,
  providerKeys,
  rateLimits,
  usage,
  guardrails,
  webhooks,
  serviceAccounts,
  access,
  gateway,
}
