// =============================================================================
// Shared
// =============================================================================

export interface Paginated<T> {
  data: T[]
  total: number
}

export interface MessageResponse {
  message: string
}

// =============================================================================
// Providers
// =============================================================================

export type ProviderStatus = "active" | "inactive"

export interface Provider {
  id: string
  name: string
  description: string
  website: string
  status: ProviderStatus
  streaming: boolean
  created_at: string
}

export interface CreateProviderRequest {
  name: string
  description: string
  website: string
  streaming: boolean
}

export interface UpdateProviderRequest {
  name?: string
  description?: string
  website?: string
  status?: ProviderStatus
  streaming?: boolean
}

// =============================================================================
// Models
// =============================================================================

export type ModelStability = "stable" | "beta" | "experimental"
export type ModelStatus = "active" | "inactive"

export interface Model {
  id: string
  name: string
  description: string
  family: string
  stability: ModelStability
  status: ModelStatus
  free: boolean
  released_at: string
  created_at: string
}

export interface CreateModelRequest {
  name: string
  description: string
  family: string
  free: boolean
}

export interface UpdateModelRequest {
  name?: string
  description?: string
  family?: string
  stability?: ModelStability
  status?: ModelStatus
  free?: boolean
}

export interface ModelWithMappings {
  model: Model
  mappings: ModelProviderMapping[]
}

// =============================================================================
// Model-Provider Mappings
// =============================================================================

export interface ModelProviderMapping {
  id: string
  model_id: string
  provider_id: string
  external_id: string
  input_price: number | null
  output_price: number | null
  cached_input_price: number | null
  request_price: number | null
  image_input_price: number | null
  context_size: number | null
  max_output: number | null
  streaming: boolean
  vision: boolean
  reasoning: boolean
  tools: boolean
  json_output: boolean
  region: string | null
  stability: ModelStability
  status: ModelStatus
  created_at: string
  updated_at: string
}

export interface CreateMappingRequest {
  model_id: string
  provider_id: string
  external_id: string
  input_price?: number
  output_price?: number
  cached_input_price?: number
  request_price?: number
  image_input_price?: number
  context_size?: number
  max_output?: number
  streaming: boolean
  vision: boolean
  reasoning: boolean
  tools: boolean
  json_output: boolean
  region?: string
}

export interface UpdateMappingRequest {
  external_id?: string
  input_price?: number
  output_price?: number
  cached_input_price?: number
  request_price?: number
  image_input_price?: number
  context_size?: number
  max_output?: number
  streaming?: boolean
  vision?: boolean
  reasoning?: boolean
  tools?: boolean
  json_output?: boolean
  region?: string
  status?: ModelStatus
}

// =============================================================================
// Model Fallbacks
// =============================================================================

export interface ModelFallback {
  id: string
  model_id: string
  fallback_model_id: string
  priority: number
  enabled: boolean
  created_at: string
}

export interface CreateModelFallbackRequest {
  model_id: string
  fallback_model_id: string
  priority: number
}

// =============================================================================
// Provider Keys
// =============================================================================

export type KeyStatus = "active" | "inactive"

export interface ProviderKey {
  id: string
  provider_id: string
  tenant_id: string | null
  token_masked: string
  base_url: string | null
  name: string
  description: string
  managed: boolean
  status: KeyStatus
  sort_order: number | null
  created_at: string
}

export interface CreateProviderKeyRequest {
  provider_id: string
  tenant_id?: string
  token: string
  base_url?: string
  name: string
  description: string
}

export interface UpdateProviderKeyRequest {
  token?: string
  base_url?: string
  name?: string
  description?: string
  status?: KeyStatus
  sort_order?: number
}

export interface TestKeyResult {
  success: boolean
  latency_ms: number
  error?: string
}

// =============================================================================
// Gateway Config: Rate Limits
// =============================================================================

export interface RateLimitConfig {
  tenant_id: string
  rpm: number
  max_concurrent: number
}

export interface UpsertRateLimitRequest {
  rpm?: number
  max_concurrent?: number
}

// =============================================================================
// Gateway Config: Routing
// =============================================================================

export type RoutingStrategy = "cheapest" | "lowest-latency" | "round-robin"

export interface RoutingConfig {
  tenant_id: string
  strategy: RoutingStrategy
  created_at: string
  updated_at: string
}

export interface UpsertRoutingConfigRequest {
  strategy: RoutingStrategy
}

// =============================================================================
// Gateway: Cost Estimation
// =============================================================================

export interface CostEstimateRequest {
  model: string
  messages: Array<{ role: string; content: string }>
  max_tokens?: number
}

export interface CostEstimateResponse {
  model: string
  provider: string
  estimated_input_tokens: number
  max_output_tokens: number
  input_price_per_million: number | null
  output_price_per_million: number | null
  estimated_input_cost_usd: number
  estimated_output_cost_usd: number
  estimated_total_cost_usd: number
}

// =============================================================================
// Billing
// =============================================================================

export interface Balance {
  tenant_id: string
  balance: number
  updated_at: string
}

// =============================================================================
// Wallets
// =============================================================================

export interface Wallet {
  id: string
  tenant_id: string
  name: string
  description: string
  balance: number
  created_at: string
  updated_at: string
}

export interface CreateWalletRequest {
  name: string
  description?: string
}

export interface UpdateWalletRequest {
  name?: string
  description?: string
}

export interface WalletTransferRequest {
  amount: number
  description?: string
}

export interface WalletListResponse {
  wallets: Wallet[]
  total: number
}

export interface WalletTransferResponse {
  wallet: Wallet
  main_balance: Balance
}

export type TransactionType = "top_up" | "usage" | "refund" | "adjust" | "wallet_fund" | "wallet_withdraw"

export interface Transaction {
  id: string
  type: TransactionType
  amount: number
  balance_after: number
  description: string
  reference_id: string
  created_at: string
}

export interface TopUpRequest {
  amount: number
  description: string
  reference_id: string
}

export interface AdjustRequest {
  amount: number
  description: string
}

export interface BillingMutationResponse {
  balance: Balance
  transaction: Transaction
}

export interface CreateCheckoutRequest {
  amount_usd: number
}

export interface CheckoutSession {
  session_id: string
  url: string
}

export interface BillingConfig {
  stripe_enabled: boolean
  min_topup_usd: number
  max_topup_usd: number
}

export interface SpendingLimit {
  tenant_id: string
  daily_limit_usd: number | null
  monthly_limit_usd: number | null
  created_at: string
  updated_at: string
}

export interface UpsertSpendingLimitRequest {
  daily_limit_usd?: number
  monthly_limit_usd?: number
}

export interface SpendingCheck {
  allowed: boolean
  daily_spend_usd: number
  monthly_spend_usd: number
  daily_limit_usd: number | null
  monthly_limit_usd: number | null
  reason: string
}

// =============================================================================
// Usage Logs
// =============================================================================

export interface UsageLog {
  id: string
  tenant_id: string
  requested_model: string
  used_model: string
  used_provider: string
  prompt_tokens: number
  completion_tokens: number
  total_tokens: number
  input_cost: number
  output_cost: number
  total_cost: number
  duration_ms: number
  streamed: boolean
  status_code: number
  has_error: boolean
  created_at: string
}

export interface UsageLogDetail extends UsageLog {
  messages: unknown
  response_body: unknown
  raw_request: unknown
  raw_response: unknown
  upstream_request: unknown
  upstream_response: unknown
  is_debug: boolean
}

export interface UsageSummary {
  tenant_id: string
  total_requests: number
  total_tokens: number
  prompt_tokens: number
  completion_tokens: number
  total_cost: number
  error_count: number
}

export interface ModelUsageSummary {
  model: string
  total_requests: number
  total_tokens: number
  prompt_tokens: number
  completion_tokens: number
  total_cost: number
}

export interface UsageSummaryResponse {
  summary: UsageSummary
  by_model: ModelUsageSummary[]
  period_start: string
  period_end: string
}

export interface DataRetentionConfig {
  tenant_id: string
  retention_days: number
  retain_messages: boolean
  retain_response_body: boolean
  retain_debug_payloads: boolean
  created_at?: string
  updated_at?: string
}

export interface UpsertRetentionRequest {
  retention_days: number
  retain_messages: boolean
  retain_response_body: boolean
  retain_debug_payloads: boolean
}

export interface UsageQuery {
  model?: string
  provider?: string
  from?: string
  to?: string
  limit?: number
  offset?: number
}

// =============================================================================
// Guardrails
// =============================================================================

export type GuardrailAction = "block" | "redact" | "warn" | "allow"
export type RuleType = "blocked_terms" | "custom_regex"

export interface SystemRuleConfig {
  enabled: boolean
  action: GuardrailAction
}

export interface SystemRulesConfig {
  prompt_injection: SystemRuleConfig
  jailbreak: SystemRuleConfig
  pii_detection: SystemRuleConfig
  secrets: SystemRuleConfig
  document_leakage: SystemRuleConfig
}

export interface GuardrailConfig {
  id: string
  tenant_id: string
  enabled: boolean
  system_rules: SystemRulesConfig
  created_at: string
  updated_at: string
}

export interface UpsertGuardrailConfigRequest {
  enabled?: boolean
  system_rules?: SystemRulesConfig
}

export interface GuardrailRule {
  id: string
  tenant_id: string
  name: string
  type: RuleType
  config: unknown
  priority: number
  enabled: boolean
  action: GuardrailAction
  created_at: string
  updated_at: string
}

export interface CreateGuardrailRuleRequest {
  name: string
  type: RuleType
  config: unknown
  priority?: number
  action: GuardrailAction
}

export interface UpdateGuardrailRuleRequest {
  name?: string
  config?: unknown
  priority?: number
  enabled?: boolean
  action?: GuardrailAction
}

export interface GuardrailViolation {
  id: string
  tenant_id: string
  rule_id: string
  rule_name: string
  category: string
  action_taken: string
  matched_pattern: string | null
  matched_content: string | null
  model: string | null
  created_at: string
}

export interface RuleViolation {
  rule_id: string
  rule_name: string
  category: string
  action: GuardrailAction
  matched_pattern: string
  matched_content: string
}

export interface RedactionInfo {
  message_index: number
  matches: string[]
  replacement: string
}

export interface GuardrailCheckResult {
  passed: boolean
  blocked: boolean
  violations: RuleViolation[]
  redactions: RedactionInfo[]
}

// =============================================================================
// Webhooks
// =============================================================================

export type DeliveryStatus = "pending" | "success" | "failed"

export interface WebhookConfig {
  id: string
  tenant_id: string
  url: string
  events: string[]
  enabled: boolean
  created_at: string
  updated_at: string
}

export interface CreateWebhookRequest {
  url: string
  events: string[]
}

export interface UpdateWebhookRequest {
  url?: string
  events?: string[]
  enabled?: boolean
}

export interface CreateWebhookResponse {
  id: string
  tenant_id: string
  url: string
  secret: string
  events: string[]
  enabled: boolean
  created_at: string
}

export interface WebhookTestResponse {
  message: string
}

export interface WebhookDelivery {
  id: string
  webhook_id: string
  event_type: string
  payload: string
  status: DeliveryStatus
  status_code: number | null
  attempts: number
  last_error: string | null
  next_retry_at: string | null
  created_at: string
  completed_at: string | null
}

// =============================================================================
// IAM: API Keys
// =============================================================================

export interface ApiKey {
  id: string
  key_prefix: string
  tenant_id: string
  user_id: string | null
  wallet_id: string | null
  name: string
  description: string
  scopes: string[]
  allowed_models: string[]
  is_active: boolean
  expires_at: string | null
  last_used_at: string | null
  created_at: string
}

export interface CreateApiKeyRequest {
  name: string
  description: string
  scopes: string[]
  allowed_models: string[]
  expires_in?: number
  environment: string
  user_id?: string
  wallet_id?: string
}

export interface UpdateApiKeyRequest {
  name?: string
  description?: string
  scopes?: string[]
  allowed_models?: string[]
  is_active?: boolean
  wallet_id?: string
}

export interface CreateApiKeyResponse {
  api_key: ApiKey
  secret_key: string
  message: string
}

// =============================================================================
// IAM: Users
// =============================================================================

export type UserStatus = "ACTIVE" | "INACTIVE" | "SUSPENDED" | "PENDING"

export interface User {
  id: string
  tenant_id: string
  name: string
  email: string
  picture: string | null
  is_active: boolean
  scopes: string[]
  oauth_provider: string
  created_at: string
}

export interface CreateUserRequest {
  email: string
  name: string
  scopes: string[]
}

export interface UpdateUserRequest {
  name?: string
  status?: UserStatus
  scopes?: string[]
}

// =============================================================================
// IAM: Roles
// =============================================================================

export interface Role {
  id: string
  tenant_id: string
  name: string
  description: string
  scopes: string[]
  created_at: string
  updated_at: string
}

export interface CreateRoleRequest {
  name: string
  description: string
  scopes: string[]
}

export interface UpdateRoleRequest {
  name?: string
  description?: string
  scopes?: string[]
}

export interface AssignRoleRequest {
  user_id: string
}

export interface UserRolesResponse {
  user_id: string
  roles: Role[]
  direct_scopes: string[]
  effective_scopes: string[]
}

// =============================================================================
// IAM: Invitations
// =============================================================================

export type InvitationStatus = "PENDING" | "ACCEPTED" | "EXPIRED" | "REVOKED"

export interface Invitation {
  id: string
  tenant_id: string
  email: string
  status: InvitationStatus
  scopes: string[]
  role_id: string | null
  expires_at: string
  accepted_at: string | null
  created_at: string
}

export interface CreateInvitationRequest {
  email: string
  scopes: string[]
  role_id?: string
  expires_in?: number
}

export interface ValidateInvitationResponse {
  valid: boolean
  invitation: Invitation
  message: string
}

// =============================================================================
// IAM: Scopes (catalog only, no management port)
// =============================================================================

export interface ScopeDetail {
  name: string
  description: string
  category: string
}

// =============================================================================
// Shared (bottom)
// =============================================================================

