import type {
  // Providers
  Provider, CreateProviderRequest, UpdateProviderRequest, ProviderFilter,
  // Models
  Model, CreateModelRequest, UpdateModelRequest, ModelFilter,
  // Mappings
  ModelProviderMapping, CreateMappingRequest, UpdateMappingRequest, MappingFilter,
  // Fallbacks
  ModelFallback, CreateFallbackRequest,
  // Provider Keys
  ProviderKey, CreateProviderKeyRequest, UpdateProviderKeyRequest, ProviderKeyFilter,
  // Rate limits
  RateLimitConfig, CreateRateLimitRequest, UpdateRateLimitRequest, RateLimitFilter,
  // Usage
  UsageLog, UsageQuery, UsageSummaryResponse,
  // Guardrails
  GuardrailConfig, UpsertGuardrailConfigRequest,
  GuardrailRule, CreateGuardrailRuleRequest, UpdateGuardrailRuleRequest,
  GuardrailViolation,
  // Webhooks
  WebhookConfig, CreateWebhookRequest, UpdateWebhookRequest,
  CreateWebhookResponse, WebhookTestResponse, WebhookDelivery,
  // Service Accounts
  ServiceAccount, ServiceAccountCredential, CreateServiceAccountRequest, Application,
  // Access Management
  AccessUser, CreateUserRequest, UpdateUserRequest,
  AccessRole, CreateRoleRequest, UpdateRoleRequest,
  RoleAssignment, AssignRoleRequest,
  // Gateway
  GatewayModelList,
  // Shared
  Paginated, PageParams,
} from "./types"

// =============================================================================
// Provider Port
// =============================================================================

export interface ProviderPort {
  list(filter?: ProviderFilter): Promise<Paginated<Provider>>
  get(id: string): Promise<Provider>
  create(req: CreateProviderRequest): Promise<{ id: string }>
  update(id: string, req: UpdateProviderRequest): Promise<void>
  delete(id: string): Promise<void>
}

// =============================================================================
// Model Port
// =============================================================================

export interface ModelPort {
  list(filter?: ModelFilter): Promise<Paginated<Model>>
  get(id: string): Promise<Model>
  create(req: CreateModelRequest): Promise<{ id: string }>
  update(id: string, req: UpdateModelRequest): Promise<void>
  delete(id: string): Promise<void>
}

// =============================================================================
// Mapping Port
// =============================================================================

export interface MappingPort {
  list(filter?: MappingFilter): Promise<Paginated<ModelProviderMapping>>
  get(id: string): Promise<ModelProviderMapping>
  create(req: CreateMappingRequest): Promise<{ id: string }>
  update(id: string, req: UpdateMappingRequest): Promise<void>
  delete(id: string): Promise<void>
}

// =============================================================================
// Model Fallback Port
// =============================================================================

export interface ModelFallbackPort {
  listByModel(modelId: string): Promise<ModelFallback[]>
  create(req: CreateFallbackRequest): Promise<{ id: string }>
  delete(id: string): Promise<void>
}

// =============================================================================
// Provider Key Port
// =============================================================================

export interface ProviderKeyPort {
  list(filter?: ProviderKeyFilter): Promise<Paginated<ProviderKey>>
  get(id: string): Promise<ProviderKey>
  create(req: CreateProviderKeyRequest): Promise<{ id: string }>
  update(id: string, req: UpdateProviderKeyRequest): Promise<void>
  delete(id: string): Promise<void>
}

// =============================================================================
// Rate Limit Port
// =============================================================================

export interface RateLimitPort {
  list(filter?: RateLimitFilter): Promise<Paginated<RateLimitConfig>>
  get(id: string): Promise<RateLimitConfig>
  create(req: CreateRateLimitRequest): Promise<RateLimitConfig>
  update(id: string, req: UpdateRateLimitRequest): Promise<RateLimitConfig>
  delete(id: string): Promise<void>
}

// =============================================================================
// Usage Port
// =============================================================================

export interface UsagePort {
  list(query?: UsageQuery): Promise<Paginated<UsageLog>>
  get(id: string): Promise<UsageLog>
  getSummary(params?: { from?: string; to?: string }): Promise<UsageSummaryResponse>
}

// =============================================================================
// Guardrails Port
// =============================================================================

export interface GuardrailsPort {
  getConfig(): Promise<GuardrailConfig>
  upsertConfig(req: UpsertGuardrailConfigRequest): Promise<GuardrailConfig>
  listRules(): Promise<GuardrailRule[]>
  getRule(id: string): Promise<GuardrailRule>
  createRule(req: CreateGuardrailRuleRequest): Promise<GuardrailRule>
  updateRule(id: string, req: UpdateGuardrailRuleRequest): Promise<GuardrailRule>
  deleteRule(id: string): Promise<void>
  listViolations(params?: PageParams): Promise<Paginated<GuardrailViolation>>
}

// =============================================================================
// Webhooks Port
// =============================================================================

export interface WebhooksPort {
  list(params?: PageParams): Promise<Paginated<WebhookConfig>>
  get(id: string): Promise<WebhookConfig>
  create(req: CreateWebhookRequest): Promise<CreateWebhookResponse>
  update(id: string, req: UpdateWebhookRequest): Promise<WebhookConfig>
  delete(id: string): Promise<void>
  listEvents(): Promise<string[]>
  listDeliveries(webhookId: string, params?: PageParams): Promise<Paginated<WebhookDelivery>>
  test(id: string): Promise<WebhookTestResponse>
}

// =============================================================================
// Gateway Port (read-only: model catalogue as seen by the OpenAI-compatible surface)
// =============================================================================

export interface GatewayPort {
  listModels(): Promise<GatewayModelList>
}

// =============================================================================
// Service Account Port (IAMKit service-account passthrough)
// =============================================================================

export interface ServiceAccountPort {
  list(): Promise<ServiceAccount[]>
  create(req: CreateServiceAccountRequest): Promise<ServiceAccountCredential>
  revoke(id: string): Promise<void>
  listApplications(): Promise<Application[]>
}

// Legacy alias
export type APIKeyPort = ServiceAccountPort

// =============================================================================
// Access Port (Users, Roles, Role Assignments via IAMKit)
// =============================================================================

export interface AccessPort {
  listUsers(): Promise<AccessUser[]>
  getUser(id: string): Promise<AccessUser>
  createUser(req: CreateUserRequest): Promise<AccessUser>
  updateUser(id: string, req: UpdateUserRequest): Promise<void>
  suspendUser(id: string): Promise<void>

  listRoles(): Promise<AccessRole[]>
  createRole(req: CreateRoleRequest): Promise<AccessRole>
  updateRole(id: string, req: UpdateRoleRequest): Promise<void>
  deleteRole(id: string): Promise<void>

  listAssignments(): Promise<RoleAssignment[]>
  assignRole(req: AssignRoleRequest): Promise<void>
  unassignRole(req: AssignRoleRequest): Promise<void>
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
  rateLimits: RateLimitPort
  usage: UsagePort
  guardrails: GuardrailsPort
  webhooks: WebhooksPort
  serviceAccounts: ServiceAccountPort
  access: AccessPort
  gateway: GatewayPort
}
