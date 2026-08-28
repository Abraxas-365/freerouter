import type {
  Provider, CreateProviderRequest, UpdateProviderRequest,
  Model, CreateModelRequest, UpdateModelRequest, ModelWithMappings,
  ModelProviderMapping, CreateMappingRequest, UpdateMappingRequest,
  ModelFallback, CreateModelFallbackRequest,
  ProviderKey, CreateProviderKeyRequest, UpdateProviderKeyRequest, TestKeyResult,
  Balance, Transaction, TopUpRequest, AdjustRequest,
  SpendingLimit, UpsertSpendingLimitRequest, SpendingCheck,
  UsageLog, UsageLogDetail, UsageSummaryResponse,
  DataRetentionConfig, UpsertRetentionRequest,
  GuardrailConfig, UpsertGuardrailConfigRequest,
  GuardrailRule, CreateGuardrailRuleRequest, UpdateGuardrailRuleRequest,
  GuardrailViolation, GuardrailCheckResult,
  WebhookConfig, CreateWebhookRequest, UpdateWebhookRequest, WebhookDelivery,
  ApiKey, CreateApiKeyRequest, UpdateApiKeyRequest, CreateApiKeyResponse,
  RateLimitConfig, UpsertRateLimitRequest,
  RoutingConfig, UpsertRoutingConfigRequest,
  CostEstimateRequest, CostEstimateResponse,
  User, Paginated,
  Role, CreateRoleRequest, UpdateRoleRequest, AssignRoleRequest, UserRolesResponse,
  TenantResponse, CreateTenantRequest, UpdateTenantRequest,
  TenantStats, TenantUsage, TenantConfig,
  Invitation, CreateInvitationRequest, ValidateInvitationResponse,
  UserScopesResponse, AvailableScopesResponse,
} from "./types"

// ============================================================================
// Provider Port
// ============================================================================

export interface ProviderPort {
  list(): Promise<Paginated<Provider>>
  get(id: string): Promise<Provider>
  create(req: CreateProviderRequest): Promise<Provider>
  update(id: string, req: UpdateProviderRequest): Promise<Provider>
  delete(id: string): Promise<void>
}

// ============================================================================
// Model Port
// ============================================================================

export interface ModelPort {
  list(): Promise<Paginated<Model>>
  get(id: string): Promise<Model>
  getWithMappings(id: string): Promise<ModelWithMappings>
  create(req: CreateModelRequest): Promise<Model>
  update(id: string, req: UpdateModelRequest): Promise<Model>
  delete(id: string): Promise<void>
}

// ============================================================================
// Mapping Port
// ============================================================================

export interface MappingPort {
  get(id: string): Promise<ModelProviderMapping>
  create(req: CreateMappingRequest): Promise<ModelProviderMapping>
  update(id: string, req: UpdateMappingRequest): Promise<ModelProviderMapping>
  delete(id: string): Promise<void>
}

// ============================================================================
// Model Fallback Port
// ============================================================================

export interface ModelFallbackPort {
  listByModel(modelId: string): Promise<Paginated<ModelFallback>>
  create(req: CreateModelFallbackRequest): Promise<ModelFallback>
  delete(id: string): Promise<void>
}

// ============================================================================
// Provider Key Port
// ============================================================================

export interface ProviderKeyPort {
  listByProvider(providerId: string): Promise<Paginated<ProviderKey>>
  listByTenant(tenantId: string): Promise<Paginated<ProviderKey>>
  listManaged(): Promise<Paginated<ProviderKey>>
  get(id: string): Promise<ProviderKey>
  create(req: CreateProviderKeyRequest): Promise<ProviderKey>
  update(id: string, req: UpdateProviderKeyRequest): Promise<ProviderKey>
  delete(id: string): Promise<void>
  test(id: string): Promise<TestKeyResult>
}

// ============================================================================
// Billing Port
// ============================================================================

export interface BillingPort {
  getBalance(): Promise<Balance>
  topUp(req: TopUpRequest): Promise<{ balance: Balance; transaction: Transaction }>
  adjust(req: AdjustRequest): Promise<{ balance: Balance; transaction: Transaction }>
  listTransactions(params?: {
    type?: Transaction["type"]
    from?: string
    to?: string
    limit?: number
    offset?: number
  }): Promise<Paginated<Transaction>>
  getSpendingLimit(tenantId: string): Promise<SpendingLimit>
  upsertSpendingLimit(tenantId: string, req: UpsertSpendingLimitRequest): Promise<SpendingLimit>
  deleteSpendingLimit(tenantId: string): Promise<void>
  checkSpending(tenantId: string): Promise<SpendingCheck>
}

// ============================================================================
// Usage Port
// ============================================================================

export interface UsagePort {
  listLogs(params?: {
    model?: string
    provider?: string
    from?: string
    to?: string
    limit?: number
    offset?: number
  }): Promise<Paginated<UsageLog>>
  getLog(id: string): Promise<UsageLogDetail>
  getSummary(params?: { from?: string; to?: string }): Promise<UsageSummaryResponse>
  getRetention(tenantId: string): Promise<DataRetentionConfig>
  upsertRetention(tenantId: string, req: UpsertRetentionRequest): Promise<DataRetentionConfig>
  deleteRetention(tenantId: string): Promise<void>
}

// ============================================================================
// Guardrails Port
// ============================================================================

export interface GuardrailsPort {
  getConfig(): Promise<GuardrailConfig>
  upsertConfig(req: UpsertGuardrailConfigRequest): Promise<GuardrailConfig>
  listRules(): Promise<Paginated<GuardrailRule>>
  createRule(req: CreateGuardrailRuleRequest): Promise<GuardrailRule>
  updateRule(id: string, req: UpdateGuardrailRuleRequest): Promise<GuardrailRule>
  deleteRule(id: string): Promise<void>
  listViolations(params?: { limit?: number; offset?: number }): Promise<Paginated<GuardrailViolation>>
  testCheck(messages: string[]): Promise<GuardrailCheckResult>
}

// ============================================================================
// Webhooks Port
// ============================================================================

export interface WebhooksPort {
  list(): Promise<Paginated<WebhookConfig>>
  listEvents(): Promise<string[]>
  get(id: string): Promise<WebhookConfig>
  create(req: CreateWebhookRequest): Promise<WebhookConfig>
  update(id: string, req: UpdateWebhookRequest): Promise<WebhookConfig>
  delete(id: string): Promise<void>
  listDeliveries(id: string, params?: { limit?: number }): Promise<Paginated<WebhookDelivery>>
  test(id: string): Promise<{ message: string }>
}

// ============================================================================
// API Keys Port
// ============================================================================

export interface ApiKeysPort {
  list(): Promise<Paginated<ApiKey>>
  get(id: string): Promise<ApiKey>
  create(req: CreateApiKeyRequest): Promise<CreateApiKeyResponse>
  update(id: string, req: UpdateApiKeyRequest): Promise<ApiKey>
  revoke(id: string): Promise<void>
  delete(id: string): Promise<void>
}

// ============================================================================
// Gateway Config Port
// ============================================================================

export interface GatewayConfigPort {
  getRateLimit(tenantId: string): Promise<RateLimitConfig>
  upsertRateLimit(tenantId: string, req: UpsertRateLimitRequest): Promise<RateLimitConfig>
  deleteRateLimit(tenantId: string): Promise<void>
  getRouting(tenantId: string): Promise<RoutingConfig>
  upsertRouting(tenantId: string, req: UpsertRoutingConfigRequest): Promise<RoutingConfig>
  deleteRouting(tenantId: string): Promise<void>
  invalidateCache(tenantId?: string): Promise<{ message: string; keys_deleted: number }>
  estimateCost(req: CostEstimateRequest): Promise<CostEstimateResponse>
}

// ============================================================================
// Users Port
// ============================================================================

export interface UsersPort {
  me(): Promise<User>
  list(): Promise<Paginated<User>>
  get(id: string): Promise<User>
  activate(id: string): Promise<void>
  suspend(id: string, reason: string): Promise<void>
  delete(id: string): Promise<void>
}

// ============================================================================
// Roles Port
// ============================================================================

export interface RolesPort {
  list(): Promise<Paginated<Role>>
  get(id: string): Promise<Role>
  create(req: CreateRoleRequest): Promise<Role>
  update(id: string, req: UpdateRoleRequest): Promise<Role>
  delete(id: string): Promise<void>
  assign(roleId: string, req: AssignRoleRequest): Promise<void>
  unassign(roleId: string, userId: string): Promise<void>
  getUserRoles(userId: string): Promise<UserRolesResponse>
}

// ============================================================================
// Tenants Port
// ============================================================================

export interface TenantsPort {
  list(): Promise<Paginated<TenantResponse>>
  get(id: string): Promise<TenantResponse>
  create(req: CreateTenantRequest): Promise<TenantResponse>
  update(id: string, req: UpdateTenantRequest): Promise<TenantResponse>
  delete(id: string): Promise<void>
  suspend(id: string, reason: string): Promise<void>
  activate(id: string, comments?: string): Promise<void>
  getStats(id: string): Promise<TenantStats>
  getUsage(id: string): Promise<TenantUsage>
  getUsers(id: string): Promise<Paginated<User>>
  getConfig(id: string): Promise<TenantConfig>
  setConfig(id: string, key: string, value: string): Promise<TenantConfig>
  deleteConfig(id: string, key: string): Promise<void>
}

// ============================================================================
// Invitations Port
// ============================================================================

export interface InvitationsPort {
  list(): Promise<Paginated<Invitation>>
  listPending(): Promise<Paginated<Invitation>>
  get(id: string): Promise<Invitation>
  create(req: CreateInvitationRequest): Promise<Invitation>
  delete(id: string): Promise<void>
  revoke(id: string, reason?: string): Promise<void>
  validateToken(token: string): Promise<ValidateInvitationResponse>
  getByToken(token: string): Promise<Invitation>
}

// ============================================================================
// Scopes Port
// ============================================================================

export interface ScopesPort {
  listAvailable(): Promise<AvailableScopesResponse>
  getUserScopes(userId: string): Promise<UserScopesResponse>
  setUserScopes(userId: string, scopes: string[]): Promise<UserScopesResponse>
  addUserScopes(userId: string, scopes: string[]): Promise<UserScopesResponse>
  removeUserScopes(userId: string, scopes: string[]): Promise<UserScopesResponse>
}

// ============================================================================
// Combined API Port (all services)
// ============================================================================

export interface ApiPort {
  providers: ProviderPort
  models: ModelPort
  mappings: MappingPort
  modelFallbacks: ModelFallbackPort
  providerKeys: ProviderKeyPort
  billing: BillingPort
  usage: UsagePort
  guardrails: GuardrailsPort
  webhooks: WebhooksPort
  apiKeys: ApiKeysPort
  gatewayConfig: GatewayConfigPort
  users: UsersPort
  roles: RolesPort
  tenants: TenantsPort
  invitations: InvitationsPort
  scopes: ScopesPort
}
