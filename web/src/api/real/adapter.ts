import { api, qs } from "./client"
import type { ApiPort } from "../ports"
import type {
  Provider, CreateProviderRequest, UpdateProviderRequest,
  Model, CreateModelRequest, UpdateModelRequest, ModelWithMappings,
  ModelProviderMapping, CreateMappingRequest, UpdateMappingRequest,
  ModelFallback, CreateModelFallbackRequest,
  ProviderKey, CreateProviderKeyRequest, UpdateProviderKeyRequest, TestKeyResult,
  RateLimitConfig, UpsertRateLimitRequest,
  RoutingConfig, UpsertRoutingConfigRequest,
  CostEstimateRequest, CostEstimateResponse,
  Balance, TopUpRequest, AdjustRequest, Transaction, BillingMutationResponse,
  CreateCheckoutRequest, CheckoutSession,
  SpendingLimit, UpsertSpendingLimitRequest, SpendingCheck,
  Wallet, CreateWalletRequest, UpdateWalletRequest,
  WalletTransferRequest, WalletListResponse, WalletTransferResponse,
  UsageLog, UsageLogDetail, UsageSummaryResponse, UsageQuery,
  DataRetentionConfig, UpsertRetentionRequest,
  GuardrailConfig, UpsertGuardrailConfigRequest,
  GuardrailRule, CreateGuardrailRuleRequest, UpdateGuardrailRuleRequest,
  GuardrailViolation, GuardrailCheckResult,
  WebhookConfig, CreateWebhookRequest, UpdateWebhookRequest, WebhookDelivery,
  CreateWebhookResponse, WebhookTestResponse,
  ApiKey, CreateApiKeyRequest, UpdateApiKeyRequest, CreateApiKeyResponse,
  User, CreateUserRequest, UpdateUserRequest,
  Role, CreateRoleRequest, UpdateRoleRequest, AssignRoleRequest, UserRolesResponse,
  Invitation, CreateInvitationRequest, ValidateInvitationResponse,
  Paginated,
} from "../types"

// =============================================================================
// Providers
// =============================================================================

const providers = {
  list: () => api.get<Paginated<Provider>>("/api/v1/providers"),
  get: (id: string) => api.get<Provider>(`/api/v1/providers/${id}`),
  create: (req: CreateProviderRequest) => api.post<Provider>("/api/v1/providers", req),
  update: (id: string, req: UpdateProviderRequest) => api.put<Provider>(`/api/v1/providers/${id}`, req),
  delete: (id: string) => api.del<void>(`/api/v1/providers/${id}`),
}

// =============================================================================
// Models
// =============================================================================

const models = {
  list: () => api.get<Paginated<Model>>("/api/v1/models"),
  get: (id: string) => api.get<Model>(`/api/v1/models/${id}`),
  getWithMappings: (id: string) => api.get<ModelWithMappings>(`/api/v1/models/${id}/with-mappings`),
  create: (req: CreateModelRequest) => api.post<Model>("/api/v1/models", req),
  update: (id: string, req: UpdateModelRequest) => api.put<Model>(`/api/v1/models/${id}`, req),
  delete: (id: string) => api.del<void>(`/api/v1/models/${id}`),
}

// =============================================================================
// Mappings
// =============================================================================

const mappings = {
  get: (id: string) => api.get<ModelProviderMapping>(`/api/v1/mappings/${id}`),
  create: (req: CreateMappingRequest) => api.post<ModelProviderMapping>("/api/v1/mappings", req),
  update: (id: string, req: UpdateMappingRequest) => api.put<ModelProviderMapping>(`/api/v1/mappings/${id}`, req),
  delete: (id: string) => api.del<void>(`/api/v1/mappings/${id}`),
}

// =============================================================================
// Model Fallbacks
// =============================================================================

const modelFallbacks = {
  listByModel: (modelId: string) => api.get<Paginated<ModelFallback>>(`/api/v1/fallbacks/model/${modelId}`),
  create: (req: CreateModelFallbackRequest) => api.post<ModelFallback>("/api/v1/fallbacks", req),
  delete: (id: string) => api.del<void>(`/api/v1/fallbacks/${id}`),
}

// =============================================================================
// Provider Keys
// =============================================================================

const providerKeys = {
  get: (id: string) => api.get<ProviderKey>(`/api/v1/provider-keys/${id}`),
  create: (req: CreateProviderKeyRequest) => api.post<ProviderKey>("/api/v1/provider-keys", req),
  update: (id: string, req: UpdateProviderKeyRequest) => api.put<ProviderKey>(`/api/v1/provider-keys/${id}`, req),
  delete: (id: string) => api.del<void>(`/api/v1/provider-keys/${id}`),
  listByProvider: (providerId: string) => api.get<Paginated<ProviderKey>>(`/api/v1/provider-keys/provider/${providerId}`),
  listByTenant: (tenantId: string) => api.get<Paginated<ProviderKey>>(`/api/v1/provider-keys/tenant/${tenantId}`),
  listManaged: () => api.get<Paginated<ProviderKey>>("/api/v1/provider-keys/managed"),
  test: (id: string) => api.post<TestKeyResult>(`/api/v1/provider-keys/${id}/test`),
}

// =============================================================================
// Gateway Config
// =============================================================================

const gatewayConfig = {
  getRateLimit: (tenantId: string) => api.get<RateLimitConfig>(`/api/v1/rate-limits/${tenantId}`),
  upsertRateLimit: (tenantId: string, req: UpsertRateLimitRequest) => api.put<RateLimitConfig>(`/api/v1/rate-limits/${tenantId}`, req),
  deleteRateLimit: (tenantId: string) => api.del<void>(`/api/v1/rate-limits/${tenantId}`),
  getRouting: (tenantId: string) => api.get<RoutingConfig>(`/api/v1/routing/${tenantId}`),
  upsertRouting: (tenantId: string, req: UpsertRoutingConfigRequest) => api.put<RoutingConfig>(`/api/v1/routing/${tenantId}`, req),
  deleteRouting: (tenantId: string) => api.del<void>(`/api/v1/routing/${tenantId}`),
  invalidateCache: (tenantId?: string) => api.post<void>("/api/v1/cache/invalidate", tenantId ? { tenant_id: tenantId } : undefined),
  estimateCost: (req: CostEstimateRequest) => api.post<CostEstimateResponse>("/api/v1/cost/estimate", req),
}

// =============================================================================
// Billing
// =============================================================================

const billing = {
  getBalance: () => api.get<Balance>("/api/v1/billing/balance"),
  topUp: (req: TopUpRequest) => api.post<BillingMutationResponse>("/api/v1/billing/top-up", req),
  adjust: (req: AdjustRequest) => api.post<BillingMutationResponse>("/api/v1/billing/adjust", req),
  listTransactions: (params?: { type?: string; from?: string; to?: string; limit?: number; offset?: number }) =>
    api.get<Paginated<Transaction>>(`/api/v1/billing/transactions${qs(params ?? {})}`),
  createCheckout: (req: CreateCheckoutRequest) => api.post<CheckoutSession>("/api/v1/billing/checkout", req),
  getSpendingLimit: (tenantId: string) => api.get<SpendingLimit>(`/api/v1/spending-limits/${tenantId}`),
  upsertSpendingLimit: (tenantId: string, req: UpsertSpendingLimitRequest) => api.put<SpendingLimit>(`/api/v1/spending-limits/${tenantId}`, req),
  deleteSpendingLimit: (tenantId: string) => api.del<void>(`/api/v1/spending-limits/${tenantId}`),
  checkSpendingLimit: (tenantId: string) => api.get<SpendingCheck>(`/api/v1/spending-limits/${tenantId}/check`),
}

// =============================================================================
// Wallets
// =============================================================================

const wallets = {
  list: () => api.get<WalletListResponse>("/api/v1/wallets"),
  get: (id: string) => api.get<Wallet>(`/api/v1/wallets/${id}`),
  create: (req: CreateWalletRequest) => api.post<Wallet>("/api/v1/wallets", req),
  update: (id: string, req: UpdateWalletRequest) => api.put<Wallet>(`/api/v1/wallets/${id}`, req),
  delete: (id: string) => api.del<void>(`/api/v1/wallets/${id}`),
  fund: (id: string, req: WalletTransferRequest) => api.post<WalletTransferResponse>(`/api/v1/wallets/${id}/fund`, req),
  withdraw: (id: string, req: WalletTransferRequest) => api.post<WalletTransferResponse>(`/api/v1/wallets/${id}/withdraw`, req),
}

// =============================================================================
// Usage
// =============================================================================

const usage = {
  queryLogs: (query: UsageQuery) => api.get<Paginated<UsageLog>>(`/api/v1/usage/logs${qs(query as Record<string, unknown>)}`),
  getLog: (id: string) => api.get<UsageLogDetail>(`/api/v1/usage/logs/${id}`),
  getSummary: (params?: { from?: string; to?: string }) => api.get<UsageSummaryResponse>(`/api/v1/usage/summary${qs(params ?? {})}`),
  getRetention: (tenantId: string) => api.get<DataRetentionConfig>(`/api/v1/usage/retention/${tenantId}`),
  upsertRetention: (tenantId: string, req: UpsertRetentionRequest) => api.put<DataRetentionConfig>(`/api/v1/usage/retention/${tenantId}`, req),
  deleteRetention: (tenantId: string) => api.del<void>(`/api/v1/usage/retention/${tenantId}`),
}

// =============================================================================
// Guardrails
// =============================================================================

const guardrails = {
  getConfig: () => api.get<GuardrailConfig>("/api/v1/guardrails/config"),
  upsertConfig: (req: UpsertGuardrailConfigRequest) => api.put<GuardrailConfig>("/api/v1/guardrails/config", req),
  listRules: () => api.get<Paginated<GuardrailRule>>("/api/v1/guardrails/rules"),
  createRule: (req: CreateGuardrailRuleRequest) => api.post<GuardrailRule>("/api/v1/guardrails/rules", req),
  updateRule: (ruleId: string, req: UpdateGuardrailRuleRequest) => api.put<GuardrailRule>(`/api/v1/guardrails/rules/${ruleId}`, req),
  deleteRule: (ruleId: string) => api.del<void>(`/api/v1/guardrails/rules/${ruleId}`),
  listViolations: () => api.get<Paginated<GuardrailViolation>>("/api/v1/guardrails/violations"),
  testCheck: (messages: Array<{ role: string; content: string }>) => api.post<GuardrailCheckResult>("/api/v1/guardrails/check", { messages }),
}

// =============================================================================
// Webhooks
// =============================================================================

const webhooks = {
  list: () => api.get<Paginated<WebhookConfig>>("/api/v1/webhooks"),
  get: (id: string) => api.get<WebhookConfig>(`/api/v1/webhooks/${id}`),
  create: (req: CreateWebhookRequest) => api.post<CreateWebhookResponse>("/api/v1/webhooks", req),
  update: (id: string, req: UpdateWebhookRequest) => api.put<WebhookConfig>(`/api/v1/webhooks/${id}`, req),
  delete: (id: string) => api.del<void>(`/api/v1/webhooks/${id}`),
  listEvents: () => api.get<string[]>("/api/v1/webhooks/events"),
  listDeliveries: (webhookId: string) => api.get<Paginated<WebhookDelivery>>(`/api/v1/webhooks/${webhookId}/deliveries`),
  test: (id: string) => api.post<WebhookTestResponse>(`/api/v1/webhooks/${id}/test`),
}

// =============================================================================
// API Keys
// =============================================================================

const apiKeys = {
  list: () => api.get<Paginated<ApiKey>>("/api/v1/api-keys"),
  get: (id: string) => api.get<ApiKey>(`/api/v1/api-keys/${id}`),
  create: (req: CreateApiKeyRequest) => api.post<CreateApiKeyResponse>("/api/v1/api-keys", req),
  update: (id: string, req: UpdateApiKeyRequest) => api.put<ApiKey>(`/api/v1/api-keys/${id}`, req),
  revoke: (id: string) => api.post<void>(`/api/v1/api-keys/${id}/revoke`),
  delete: (id: string) => api.del<void>(`/api/v1/api-keys/${id}`),
}

// =============================================================================
// Users
// =============================================================================

const users = {
  list: () => api.get<Paginated<User>>("/api/v1/users"),
  get: (id: string) => api.get<User>(`/api/v1/users/${id}`),
  create: (req: CreateUserRequest) => api.post<User>("/api/v1/users", req),
  update: (id: string, req: UpdateUserRequest) => api.put<User>(`/api/v1/users/${id}`, req),
  activate: (id: string) => api.post<void>(`/api/v1/users/${id}/activate`),
  suspend: (id: string, reason?: string) => api.post<void>(`/api/v1/users/${id}/suspend`, reason ? { reason } : undefined),
  delete: (id: string) => api.del<void>(`/api/v1/users/${id}`),
}

// =============================================================================
// Roles
// =============================================================================

const roles = {
  list: () => api.get<Paginated<Role>>("/api/v1/roles"),
  get: (id: string) => api.get<Role>(`/api/v1/roles/${id}`),
  create: (req: CreateRoleRequest) => api.post<Role>("/api/v1/roles", req),
  update: (id: string, req: UpdateRoleRequest) => api.put<Role>(`/api/v1/roles/${id}`, req),
  delete: (id: string) => api.del<void>(`/api/v1/roles/${id}`),
  assign: (roleId: string, req: AssignRoleRequest) => api.post<void>(`/api/v1/roles/${roleId}/assign`, req),
  unassign: (roleId: string, userId: string) => api.del<void>(`/api/v1/roles/${roleId}/assign/${userId}`),
  getUserRoles: (userId: string) => api.get<UserRolesResponse>(`/api/v1/roles/users/${userId}`),
}

// =============================================================================
// Invitations
// =============================================================================

const invitations = {
  list: () => api.get<Paginated<Invitation>>("/api/v1/invitations"),
  listPending: () => api.get<Paginated<Invitation>>("/api/v1/invitations/pending"),
  get: (id: string) => api.get<Invitation>(`/api/v1/invitations/${id}`),
  create: (req: CreateInvitationRequest) => api.post<Invitation>("/api/v1/invitations", req),
  delete: (id: string) => api.del<void>(`/api/v1/invitations/${id}`),
  revoke: (id: string) => api.post<void>(`/api/v1/invitations/${id}/revoke`),
  validateToken: (token: string) => api.get<ValidateInvitationResponse>(`/api/v1/invitations/validate/${token}`),
  getByToken: (token: string) => api.get<Invitation>(`/api/v1/invitations/token/${token}`),
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
  gatewayConfig,
  billing,
  wallets,
  usage,
  guardrails,
  webhooks,
  apiKeys,
  users,
  roles,
  invitations,
}
