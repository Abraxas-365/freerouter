// ============================================================================
// Providers
// ============================================================================

export interface Provider {
  id: string
  name: string
  description: string
  website?: string
  status: "active" | "inactive" | "deprecated"
  streaming: boolean
  created_at: string
}

export interface CreateProviderRequest {
  name: string
  description: string
  website?: string
  streaming?: boolean
}

export interface UpdateProviderRequest {
  name?: string
  description?: string
  website?: string
  status?: Provider["status"]
  streaming?: boolean
}

// ============================================================================
// Models
// ============================================================================

export interface Model {
  id: string
  name: string
  description: string
  family: string
  stability: "stable" | "beta" | "experimental" | "deprecated"
  status: "active" | "inactive" | "deprecated"
  free: boolean
  released_at: string
  created_at: string
}

export interface CreateModelRequest {
  name: string
  description: string
  family: string
  free?: boolean
}

export interface UpdateModelRequest {
  name?: string
  description?: string
  family?: string
  stability?: Model["stability"]
  status?: Model["status"]
  free?: boolean
}

export interface ModelWithMappings {
  model: Model
  mappings: ModelProviderMapping[]
}

// ============================================================================
// Model-Provider Mappings
// ============================================================================

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
  context_size?: number
  max_output?: number
  streaming: boolean
  vision: boolean
  reasoning: boolean
  tools: boolean
  json_output: boolean
  region?: string
  stability: Model["stability"]
  status: Model["status"]
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
  streaming?: boolean
  vision?: boolean
  reasoning?: boolean
  tools?: boolean
  json_output?: boolean
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
  status?: Model["status"]
}

// ============================================================================
// Model Fallbacks
// ============================================================================

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

// ============================================================================
// Provider Keys
// ============================================================================

export interface ProviderKey {
  id: string
  provider_id: string
  tenant_id?: string
  token_masked: string
  base_url?: string
  name: string
  description: string
  managed: boolean
  status: "active" | "inactive" | "revoked" | "rate_limited"
  sort_order?: number
  created_at: string
}

export interface CreateProviderKeyRequest {
  provider_id: string
  tenant_id?: string
  token: string
  base_url?: string
  name: string
  description?: string
}

export interface UpdateProviderKeyRequest {
  token?: string
  base_url?: string
  name?: string
  description?: string
  status?: ProviderKey["status"]
  sort_order?: number
}

export interface TestKeyResult {
  success: boolean
  message: string
  error?: string
}

// ============================================================================
// Billing
// ============================================================================

export interface Balance {
  tenant_id: string
  balance: number
  updated_at: string
}

export interface Transaction {
  id: string
  type: "top_up" | "usage" | "refund" | "adjust"
  amount: number
  balance_after: number
  description: string
  reference_id?: string
  created_at: string
}

export interface TopUpRequest {
  amount: number
  description: string
  reference_id?: string
}

export interface AdjustRequest {
  amount: number
  description: string
}

export interface SpendingLimit {
  tenant_id: string
  daily_limit_usd?: number
  monthly_limit_usd?: number
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
  daily_limit_usd?: number
  monthly_limit_usd?: number
  reason?: string
}

// ============================================================================
// Usage
// ============================================================================

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
  messages?: unknown
  response_body?: unknown
  raw_request?: unknown
  raw_response?: unknown
  upstream_request?: unknown
  upstream_response?: unknown
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
  created_at: string
  updated_at: string
}

export interface UpsertRetentionRequest {
  retention_days: number
  retain_messages: boolean
  retain_response_body: boolean
  retain_debug_payloads: boolean
}

// ============================================================================
// Guardrails
// ============================================================================

export interface SystemRuleConfig {
  enabled: boolean
  action: "block" | "redact" | "warn" | "allow"
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
  system_rules?: Partial<SystemRulesConfig>
}

export interface GuardrailRule {
  id: string
  tenant_id: string
  name: string
  type: "blocked_terms" | "custom_regex"
  config: Record<string, unknown>
  priority: number
  enabled: boolean
  action: "block" | "redact" | "warn"
  created_at: string
  updated_at: string
}

export interface CreateGuardrailRuleRequest {
  name: string
  type: GuardrailRule["type"]
  config: Record<string, unknown>
  priority?: number
  action: GuardrailRule["action"]
}

export interface UpdateGuardrailRuleRequest {
  name?: string
  config?: Record<string, unknown>
  priority?: number
  enabled?: boolean
  action?: GuardrailRule["action"]
}

export interface GuardrailViolation {
  id: string
  tenant_id: string
  rule_id: string
  rule_name: string
  category: string
  action_taken: string
  matched_pattern?: string
  matched_content?: string
  model?: string
  created_at: string
}

export interface GuardrailCheckResult {
  passed: boolean
  blocked: boolean
  violations: Array<{ rule: string; message: string }>
  redactions: Array<{ rule: string; original: string; redacted: string }>
}

// ============================================================================
// Webhooks
// ============================================================================

export interface WebhookConfig {
  id: string
  tenant_id: string
  url: string
  secret?: string // Only on create response
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

export interface WebhookDelivery {
  id: string
  webhook_id: string
  event_type: string
  payload: string
  status: "pending" | "success" | "failed"
  status_code?: number
  attempts: number
  last_error?: string
  next_retry_at?: string
  created_at: string
  completed_at?: string
}

// ============================================================================
// API Keys
// ============================================================================

export interface ApiKey {
  id: string
  key_prefix: string
  tenant_id: string
  user_id?: string
  name: string
  description?: string
  scopes: string[]
  allowed_models?: string[]
  is_active: boolean
  expires_at?: string
  last_used_at?: string
  created_at: string
}

export interface CreateApiKeyRequest {
  name: string
  description?: string
  scopes: string[]
  allowed_models?: string[]
  expires_in?: number
  environment: "live" | "test"
  user_id?: string
}

export interface UpdateApiKeyRequest {
  name?: string
  description?: string
  scopes?: string[]
  allowed_models?: string[]
  is_active?: boolean
}

export interface CreateApiKeyResponse {
  api_key: ApiKey
  secret_key: string
  message: string
}

// ============================================================================
// Gateway Config
// ============================================================================

export interface RateLimitConfig {
  tenant_id: string
  rpm: number
  max_concurrent: number
}

export interface UpsertRateLimitRequest {
  rpm?: number
  max_concurrent?: number
}

export interface RoutingConfig {
  tenant_id: string
  strategy: "cheapest" | "lowest-latency" | "round-robin"
}

export interface UpsertRoutingConfigRequest {
  strategy: RoutingConfig["strategy"]
}

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
  input_price_per_million?: number
  output_price_per_million?: number
  estimated_input_cost: number
  estimated_output_cost: number
  estimated_total_cost: number
}

// ============================================================================
// Users
// ============================================================================

export interface User {
  id: string
  tenant_id: string
  email: string
  name: string
  picture?: string
  oauth_provider: string
  status: "ACTIVE" | "INACTIVE" | "SUSPENDED" | "PENDING"
  scopes: string[]
  email_verified: boolean
  last_login_at?: string
  created_at: string
  updated_at: string
}

// ============================================================================
// Paginated response wrapper
// ============================================================================

export interface Paginated<T> {
  data: T[]
  total: number
}
