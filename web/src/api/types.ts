// =============================================================================
// Shared
// =============================================================================

// Wire format from internal/query.Paginated[T]: {"items": [...], "page": {...}}
export interface Page {
  total: number
  limit: number
  offset: number
}

export interface Paginated<T> {
  items: T[]
  page: Page
}

export interface PageParams {
  limit?: number
  offset?: number
}

// Wire format from internal/errx.HTTPErrorResponse
export interface ApiErrorBody {
  code: string
  message: string
  type: string
  details?: Record<string, unknown>
  status_code: number
}

// =============================================================================
// Providers
// =============================================================================

export type ProviderStatus = "active" | "inactive"
export type ProviderProtocol = "openai" | "anthropic" | "google" | "azure" | "cohere" | "codex" | "claude-code"

export const PROVIDER_PROTOCOLS: { value: ProviderProtocol; label: string }[] = [
  { value: "openai", label: "OpenAI" },
  { value: "anthropic", label: "Anthropic" },
  { value: "google", label: "Google (Gemini)" },
  { value: "azure", label: "Azure OpenAI" },
  { value: "cohere", label: "Cohere" },
  { value: "codex", label: "Codex (OpenAI)" },
  { value: "claude-code", label: "Claude Code (OAuth)" },
]

export interface Provider {
  id: string
  name: string
  protocol: ProviderProtocol
  description: string
  website?: string
  base_url: string
  status: ProviderStatus
  streaming: boolean
  created_at: string
  updated_at: string
}

export interface CreateProviderRequest {
  name: string
  protocol: ProviderProtocol
  description: string
  website?: string
  base_url: string
  streaming: boolean
}

export interface UpdateProviderRequest {
  name?: string
  protocol?: ProviderProtocol
  description?: string
  website?: string
  base_url?: string
  status?: ProviderStatus
  streaming?: boolean
}

export interface ProviderFilter extends PageParams {
  status?: ProviderStatus
  search?: string
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
  updated_at: string
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

export interface ModelFilter extends PageParams {
  status?: ModelStatus
  family?: string
  search?: string
}

// =============================================================================
// Model-Provider Mappings
// =============================================================================

export interface ModelProviderMapping {
  id: string
  model_id: string
  provider_id: string
  external_id: string

  input_price?: number
  output_price?: number
  cached_input_price?: number
  request_price?: number
  image_input_price?: number

  audio_price_per_minute?: number
  speech_price_per_1k_chars?: number
  rerank_price_per_1k?: number

  context_size?: number
  max_output?: number

  streaming: boolean
  vision: boolean
  reasoning: boolean
  tools: boolean
  json_output: boolean
  audio: boolean
  speech: boolean
  moderation: boolean
  rerank: boolean

  region?: string
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

  audio_price_per_minute?: number
  speech_price_per_1k_chars?: number
  rerank_price_per_1k?: number

  context_size?: number
  max_output?: number

  streaming: boolean
  vision: boolean
  reasoning: boolean
  tools: boolean
  json_output: boolean
  audio: boolean
  speech: boolean
  moderation: boolean
  rerank: boolean
  region?: string
}

export interface UpdateMappingRequest {
  external_id?: string
  input_price?: number
  output_price?: number
  cached_input_price?: number
  request_price?: number
  image_input_price?: number
  audio_price_per_minute?: number
  speech_price_per_1k_chars?: number
  rerank_price_per_1k?: number
  context_size?: number
  max_output?: number
  streaming?: boolean
  vision?: boolean
  reasoning?: boolean
  tools?: boolean
  json_output?: boolean
  audio?: boolean
  speech?: boolean
  moderation?: boolean
  rerank?: boolean
  region?: string
  status?: ModelStatus
}

export interface MappingFilter extends PageParams {
  model_id?: string
  provider_id?: string
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

export interface CreateFallbackRequest {
  model_id: string
  fallback_model_id: string
  priority: number
}

// =============================================================================
// Provider Keys
// =============================================================================

export type KeyStatus = "active" | "inactive"
export type KeyType = "api_key" | "oauth"

export interface OAuthData {
  access_token: string
  refresh_token: string
  expires_at: string
}

export interface ProviderKey {
  id: string
  provider_id: string
  key_type: KeyType
  token_masked: string
  base_url?: string
  name: string
  description: string
  status: KeyStatus
  sort_order?: number
  created_at: string
  updated_at: string
}

export interface CreateProviderKeyRequest {
  provider_id: string
  key_type?: KeyType
  token?: string
  oauth_data?: OAuthData
  base_url?: string
  name: string
  description: string
}

export interface UpdateProviderKeyRequest {
  token?: string
  oauth_data?: OAuthData
  base_url?: string
  name?: string
  description?: string
  status?: KeyStatus
  sort_order?: number
}

export interface ProviderKeyFilter extends PageParams {
  provider_id?: string
  status?: KeyStatus
  key_type?: KeyType
}

// =============================================================================
// Rate Limits
// =============================================================================

export interface RateLimitConfig {
  id: string
  name: string
  subject_id: string
  rpm: number
  max_concurrent: number
  created_at: string
  updated_at: string
}

export interface CreateRateLimitRequest {
  name: string
  subject_id: string
  rpm: number
  max_concurrent: number
}

export interface UpdateRateLimitRequest {
  name?: string
  rpm?: number
  max_concurrent?: number
}

export interface RateLimitFilter extends PageParams {
  subject_id?: string
  name?: string
}

// =============================================================================
// Usage
// =============================================================================

export interface UsageLog {
  id: string
  key_id: string
  requested_model: string
  used_model: string
  provider_id: string
  mapping_id: string
  prompt_tokens: number
  completion_tokens: number
  total_tokens: number
  cached_tokens: number
  input_cost: number
  output_cost: number
  total_cost: number
  duration_ms: number
  streamed: boolean
  status_code: number
  finish_reason: string
  has_error: boolean
  error_message?: string
  is_fallback: boolean
  created_at: string
}

export interface UsageQuery extends PageParams {
  model?: string
  provider?: string
  has_error?: boolean
  from?: string
  to?: string
}

export interface UsageSummary {
  total_requests: number
  total_tokens: number
  prompt_tokens: number
  completion_tokens: number
  total_cost: number
  error_count: number
}

export interface UsageModelSummary {
  model: string
  total_requests: number
  total_tokens: number
  prompt_tokens: number
  completion_tokens: number
  total_cost: number
}

export interface UsageSummaryResponse {
  summary: UsageSummary
  by_model: UsageModelSummary[]
  period_start: string
  period_end: string
}

// =============================================================================
// Guardrails
// =============================================================================

export type GuardrailAction = "block" | "redact" | "warn"

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
  enabled: boolean
  system_rules: SystemRulesConfig
  created_at: string
  updated_at: string
}

export interface UpsertGuardrailConfigRequest {
  enabled?: boolean
  system_rules?: SystemRulesConfig
}

export type GuardrailRuleType = "blocked_terms" | "custom_regex"

export interface BlockedTermsConfig {
  terms: string[]
  match_type: "exact" | "contains" | "regex"
  case_sensitive: boolean
}

export interface CustomRegexConfig {
  pattern: string
}

export interface GuardrailRule {
  id: string
  name: string
  type: GuardrailRuleType
  config: BlockedTermsConfig | CustomRegexConfig | Record<string, unknown>
  priority: number
  enabled: boolean
  action: GuardrailAction
  created_at: string
  updated_at: string
}

export interface CreateGuardrailRuleRequest {
  name: string
  type: GuardrailRuleType
  config: BlockedTermsConfig | CustomRegexConfig
  priority?: number
  action: GuardrailAction
}

export interface UpdateGuardrailRuleRequest {
  name?: string
  config?: BlockedTermsConfig | CustomRegexConfig
  priority?: number
  enabled?: boolean
  action?: GuardrailAction
}

export interface GuardrailViolation {
  id: string
  rule_id: string
  rule_name: string
  category: string
  action_taken: "blocked" | "redacted" | "warned" | string
  matched_pattern?: string
  matched_content?: string
  model?: string
  created_at: string
}

// =============================================================================
// Webhooks
// =============================================================================

export type WebhookEvent =
  | "request.completed"
  | "request.failed"
  | "key.health_degraded"
  | "key.blacklisted"

export interface WebhookConfig {
  id: string
  url: string
  events: string[]
  enabled: boolean
  created_at: string
  updated_at: string
}

// Only returned once, on creation.
export interface CreateWebhookResponse extends WebhookConfig {
  secret: string
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

export type WebhookDeliveryStatus = "pending" | "success" | "failed"

export interface WebhookDelivery {
  id: string
  webhook_id: string
  event_type: string
  payload: string
  status: WebhookDeliveryStatus
  status_code?: number
  attempts: number
  last_error?: string
  next_retry_at?: string
  created_at: string
  completed_at?: string
}

export interface WebhookTestResponse {
  message: string
}

// =============================================================================
// Gateway (OpenAI-compatible surface, read-only from the console's perspective)
// =============================================================================

export interface GatewayModel {
  id: string
  object: "model"
  created: number
  owned_by: string
}

export interface GatewayModelList {
  object: "list"
  data: GatewayModel[]
}

// =============================================================================
// Service Accounts (IAMKit service-account passthrough)
// =============================================================================

export interface ServiceAccount {
  id: string
  name: string
  application_id: string
  resource_id: string
  permissions: string[]
  expires_in?: string
}

/** An IAMKit application a service account can belong to (i.e. "which service"). */
export interface Application {
  id: string
  name: string
  active: boolean
}

export interface ServiceAccountCredential {
  id: string
  secret: string
  expires_at: string
}

export interface CreateServiceAccountRequest {
  name: string
  application_id?: string
  permissions?: string[]
  expires_in?: string
}

/** All permissions recognised by FreeRouter — keep in sync with server/permissions.go */
export const VALID_PERMISSIONS = [
  "freerouter:gateway:invoke",
  "freerouter:gateway:write",
  "freerouter:metrics:read",
  "freerouter:providers:read",
  "freerouter:providers:write",
  "freerouter:provider-keys:read",
  "freerouter:provider-keys:write",
  "freerouter:usage:read",
  "freerouter:usage:write",
  "freerouter:rate-limits:read",
  "freerouter:rate-limits:write",
  "freerouter:routing:read",
  "freerouter:routing:write",
  "freerouter:guardrails:read",
  "freerouter:guardrails:write",
  "freerouter:webhooks:read",
  "freerouter:webhooks:write",
  "freerouter:service-accounts:read",
  "freerouter:service-accounts:write",
  "freerouter:users:read",
  "freerouter:users:write",
  "freerouter:roles:read",
  "freerouter:roles:write",
] as const

export const DEFAULT_PERMISSIONS = ["freerouter:gateway:invoke"]

// Legacy aliases
export type APIKey = ServiceAccount
export type APIKeyCredential = ServiceAccountCredential
export type CreateAPIKeyRequest = CreateServiceAccountRequest

// =============================================================================
// Access Management (Users, Roles, Role Assignments)
// =============================================================================

export interface AccessUser {
  id: string
  email: string
  name: string
  active: boolean
}

export interface CreateUserRequest {
  email: string
  name: string
  password: string
}

export interface UpdateUserRequest {
  name?: string
  active?: boolean
}

export interface AccessRole {
  id: string
  name: string
  permissions: string[]
}

export interface CreateRoleRequest {
  name: string
  permissions: string[]
}

export interface UpdateRoleRequest {
  name: string
  permissions: string[]
}

export interface RoleAssignment {
  user_id: string
  role_id: string
  organization_id: string
}

export interface AssignRoleRequest {
  user_id: string
  role_id: string
}
