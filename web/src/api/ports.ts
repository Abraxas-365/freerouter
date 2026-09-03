import type {
  // Providers
  Provider, CreateProviderRequest, UpdateProviderRequest,
  // Models
  Model, CreateModelRequest, UpdateModelRequest, ModelWithMappings,
  // Mappings
  ModelProviderMapping, CreateMappingRequest, UpdateMappingRequest,
  // Fallbacks
  ModelFallback, CreateModelFallbackRequest,
  // Provider Keys
  ProviderKey, CreateProviderKeyRequest, UpdateProviderKeyRequest, TestKeyResult,
  // Gateway Config
  RateLimitConfig, UpsertRateLimitRequest,
  RoutingConfig, UpsertRoutingConfigRequest,
  CostEstimateRequest, CostEstimateResponse,
  // Billing
  Balance, Transaction, TopUpRequest, AdjustRequest, BillingMutationResponse,
  CreateCheckoutRequest, CheckoutSession,
  SpendingLimit, UpsertSpendingLimitRequest, SpendingCheck,
  // Wallets
  Wallet, CreateWalletRequest, UpdateWalletRequest,
  WalletTransferRequest, WalletListResponse, WalletTransferResponse,
  // Usage
  UsageLog, UsageLogDetail, UsageSummaryResponse, UsageQuery,
  DataRetentionConfig, UpsertRetentionRequest,
  // Guardrails
  GuardrailConfig, UpsertGuardrailConfigRequest,
  GuardrailRule, CreateGuardrailRuleRequest, UpdateGuardrailRuleRequest,
  GuardrailViolation, GuardrailCheckResult,
  // Webhooks
  WebhookConfig, CreateWebhookRequest, UpdateWebhookRequest,
  CreateWebhookResponse, WebhookTestResponse, WebhookDelivery,
  // API Keys
  ApiKey, CreateApiKeyRequest, UpdateApiKeyRequest, CreateApiKeyResponse,
  // Users
  User, CreateUserRequest, UpdateUserRequest,
  // Roles
  Role, CreateRoleRequest, UpdateRoleRequest, AssignRoleRequest, UserRolesResponse,
  // Invitations
  Invitation, CreateInvitationRequest, ValidateInvitationResponse,
  // Shared
  Paginated,
} from "./types"

// =============================================================================
// Provider Port
// =============================================================================

export interface ProviderPort {
  list(): Promise<Paginated<Provider>>
  get(id: string): Promise<Provider>
  create(req: CreateProviderRequest): Promise<Provider>
  update(id: string, req: UpdateProviderRequest): Promise<Provider>
  delete(id: string): Promise<void>
}

// =============================================================================
// Model Port
// =============================================================================

export interface ModelPort {
  list(): Promise<Paginated<Model>>
  get(id: string): Promise<Model>
  getWithMappings(id: string): Promise<ModelWithMappings>
  create(req: CreateModelRequest): Promise<Model>
  update(id: string, req: UpdateModelRequest): Promise<Model>
  delete(id: string): Promise<void>
}

// =============================================================================
// Mapping Port
// =============================================================================

export interface MappingPort {
  get(id: string): Promise<ModelProviderMapping>
  create(req: CreateMappingRequest): Promise<ModelProviderMapping>
  update(id: string, req: UpdateMappingRequest): Promise<ModelProviderMapping>
  delete(id: string): Promise<void>
}

// =============================================================================
// Model Fallback Port
// =============================================================================

export interface ModelFallbackPort {
  listByModel(modelId: string): Promise<Paginated<ModelFallback>>
  create(req: CreateModelFallbackRequest): Promise<ModelFallback>
  delete(id: string): Promise<void>
}

// =============================================================================
// Provider Key Port
// =============================================================================

export interface ProviderKeyPort {
  get(id: string): Promise<ProviderKey>
  create(req: CreateProviderKeyRequest): Promise<ProviderKey>
  update(id: string, req: UpdateProviderKeyRequest): Promise<ProviderKey>
  delete(id: string): Promise<void>
  listByProvider(providerId: string): Promise<Paginated<ProviderKey>>
  listByTenant(tenantId: string): Promise<Paginated<ProviderKey>>
  listManaged(): Promise<Paginated<ProviderKey>>
  test(id: string): Promise<TestKeyResult>
}

// =============================================================================
// Gateway Config Port
// =============================================================================

export interface GatewayConfigPort {
  // Rate limits
  getRateLimit(tenantId: string): Promise<RateLimitConfig>
  upsertRateLimit(tenantId: string, req: UpsertRateLimitRequest): Promise<RateLimitConfig>
  deleteRateLimit(tenantId: string): Promise<void>
  // Routing
  getRouting(tenantId: string): Promise<RoutingConfig>
  upsertRouting(tenantId: string, req: UpsertRoutingConfigRequest): Promise<RoutingConfig>
  deleteRouting(tenantId: string): Promise<void>
  // Cache
  invalidateCache(tenantId?: string): Promise<void>
  // Cost estimation
  estimateCost(req: CostEstimateRequest): Promise<CostEstimateResponse>
}

// =============================================================================
// Billing Port
// =============================================================================

export interface BillingPort {
  getBalance(): Promise<Balance>
  topUp(req: TopUpRequest): Promise<BillingMutationResponse>
  adjust(req: AdjustRequest): Promise<BillingMutationResponse>
  listTransactions(params?: { type?: string; from?: string; to?: string; limit?: number; offset?: number }): Promise<Paginated<Transaction>>
  createCheckout(req: CreateCheckoutRequest): Promise<CheckoutSession>
  // Spending limits
  getSpendingLimit(tenantId: string): Promise<SpendingLimit>
  upsertSpendingLimit(tenantId: string, req: UpsertSpendingLimitRequest): Promise<SpendingLimit>
  deleteSpendingLimit(tenantId: string): Promise<void>
  checkSpendingLimit(tenantId: string): Promise<SpendingCheck>
}

// =============================================================================
// Usage Port
// =============================================================================

export interface UsagePort {
  queryLogs(query: UsageQuery): Promise<Paginated<UsageLog>>
  getLog(id: string): Promise<UsageLogDetail>
  getSummary(params?: { from?: string; to?: string }): Promise<UsageSummaryResponse>
  // Data retention
  getRetention(tenantId: string): Promise<DataRetentionConfig>
  upsertRetention(tenantId: string, req: UpsertRetentionRequest): Promise<DataRetentionConfig>
  deleteRetention(tenantId: string): Promise<void>
}

// =============================================================================
// Guardrails Port
// =============================================================================

export interface GuardrailsPort {
  getConfig(): Promise<GuardrailConfig>
  upsertConfig(req: UpsertGuardrailConfigRequest): Promise<GuardrailConfig>
  listRules(): Promise<Paginated<GuardrailRule>>
  createRule(req: CreateGuardrailRuleRequest): Promise<GuardrailRule>
  updateRule(ruleId: string, req: UpdateGuardrailRuleRequest): Promise<GuardrailRule>
  deleteRule(ruleId: string): Promise<void>
  listViolations(): Promise<Paginated<GuardrailViolation>>
  testCheck(messages: Array<{ role: string; content: string }>): Promise<GuardrailCheckResult>
}

// =============================================================================
// Webhooks Port
// =============================================================================

export interface WebhooksPort {
  list(): Promise<Paginated<WebhookConfig>>
  get(id: string): Promise<WebhookConfig>
  create(req: CreateWebhookRequest): Promise<CreateWebhookResponse>
  update(id: string, req: UpdateWebhookRequest): Promise<WebhookConfig>
  delete(id: string): Promise<void>
  listEvents(): Promise<string[]>
  listDeliveries(webhookId: string): Promise<Paginated<WebhookDelivery>>
  test(id: string): Promise<WebhookTestResponse>
}

// =============================================================================
// Wallets Port
// =============================================================================

export interface WalletsPort {
  list(): Promise<WalletListResponse>
  get(id: string): Promise<Wallet>
  create(req: CreateWalletRequest): Promise<Wallet>
  update(id: string, req: UpdateWalletRequest): Promise<Wallet>
  delete(id: string): Promise<void>
  fund(id: string, req: WalletTransferRequest): Promise<WalletTransferResponse>
  withdraw(id: string, req: WalletTransferRequest): Promise<WalletTransferResponse>
}

// =============================================================================
// API Keys Port
// =============================================================================

export interface ApiKeysPort {
  list(): Promise<Paginated<ApiKey>>
  get(id: string): Promise<ApiKey>
  create(req: CreateApiKeyRequest): Promise<CreateApiKeyResponse>
  update(id: string, req: UpdateApiKeyRequest): Promise<ApiKey>
  revoke(id: string): Promise<void>
  delete(id: string): Promise<void>
}

// =============================================================================
// Users Port
// =============================================================================

export interface UsersPort {
  list(): Promise<Paginated<User>>
  get(id: string): Promise<User>
  create(req: CreateUserRequest): Promise<User>
  update(id: string, req: UpdateUserRequest): Promise<User>
  activate(id: string): Promise<void>
  suspend(id: string, reason?: string): Promise<void>
  delete(id: string): Promise<void>
}

// =============================================================================
// Roles Port
// =============================================================================

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

// =============================================================================
// Invitations Port
// =============================================================================

export interface InvitationsPort {
  list(): Promise<Paginated<Invitation>>
  listPending(): Promise<Paginated<Invitation>>
  get(id: string): Promise<Invitation>
  create(req: CreateInvitationRequest): Promise<Invitation>
  delete(id: string): Promise<void>
  revoke(id: string): Promise<void>
  validateToken(token: string): Promise<ValidateInvitationResponse>
  getByToken(token: string): Promise<Invitation>
}

// =============================================================================
// Combined API Port
// =============================================================================

export interface ApiPort {
  providers: ProviderPort
  models: ModelPort
  mappings: MappingPort
  modelFallbacks: ModelFallbackPort
  providerKeys: ProviderKeyPort
  gatewayConfig: GatewayConfigPort
  billing: BillingPort
  wallets: WalletsPort
  usage: UsagePort
  guardrails: GuardrailsPort
  webhooks: WebhooksPort
  apiKeys: ApiKeysPort
  users: UsersPort
  roles: RolesPort
  invitations: InvitationsPort
}
