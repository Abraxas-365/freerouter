import type {
  ApiPort, ProviderPort, ModelPort, MappingPort, ModelFallbackPort,
  ProviderKeyPort, BillingPort, UsagePort, GuardrailsPort,
  WebhooksPort, ApiKeysPort, GatewayConfigPort, UsersPort,
} from "./ports"
import type {
  Provider, Model, ModelWithMappings, ModelProviderMapping,
  ModelFallback, ProviderKey, TestKeyResult,
  Balance, Transaction, SpendingLimit, SpendingCheck,
  UsageLog, UsageLogDetail, UsageSummaryResponse,
  DataRetentionConfig, GuardrailConfig, GuardrailRule,
  GuardrailViolation, GuardrailCheckResult,
  WebhookConfig, WebhookDelivery,
  ApiKey, CreateApiKeyResponse,
  RateLimitConfig, RoutingConfig, CostEstimateResponse, User,
} from "./types"

// ============================================================================
// Helpers
// ============================================================================

const delay = (ms = 200) => new Promise((r) => setTimeout(r, ms + Math.random() * 200))
let idCounter = 1000
const newId = () => `mock-${++idCounter}`
const now = () => new Date().toISOString()

// ============================================================================
// Mock Data
// ============================================================================

const PROVIDERS: Provider[] = [
  { id: "openai", name: "OpenAI", description: "GPT models and DALL-E", website: "https://openai.com", status: "active", streaming: true, created_at: "2024-01-01T00:00:00Z" },
  { id: "anthropic", name: "Anthropic", description: "Claude models", website: "https://anthropic.com", status: "active", streaming: true, created_at: "2024-01-01T00:00:00Z" },
  { id: "google", name: "Google AI Studio", description: "Gemini models", website: "https://ai.google.dev", status: "active", streaming: true, created_at: "2024-01-01T00:00:00Z" },
  { id: "mistral", name: "Mistral AI", description: "Mistral and Codestral", website: "https://mistral.ai", status: "active", streaming: true, created_at: "2024-01-01T00:00:00Z" },
  { id: "deepseek", name: "DeepSeek", description: "DeepSeek Chat and Reasoner", website: "https://deepseek.com", status: "active", streaming: true, created_at: "2024-01-01T00:00:00Z" },
  { id: "xai", name: "xAI", description: "Grok models", website: "https://x.ai", status: "active", streaming: true, created_at: "2024-06-01T00:00:00Z" },
  { id: "groq", name: "Groq", description: "Ultra-fast LPU inference", website: "https://groq.com", status: "active", streaming: true, created_at: "2024-03-01T00:00:00Z" },
  { id: "together", name: "Together AI", description: "Open-source model hosting", website: "https://together.ai", status: "active", streaming: true, created_at: "2024-02-01T00:00:00Z" },
]

const MODELS: Model[] = [
  { id: "gpt-4o", name: "GPT-4o", description: "Most capable GPT model", family: "openai", stability: "stable", status: "active", free: false, released_at: "2024-05-01T00:00:00Z", created_at: "2024-05-01T00:00:00Z" },
  { id: "gpt-4o-mini", name: "GPT-4o Mini", description: "Affordable small model", family: "openai", stability: "stable", status: "active", free: false, released_at: "2024-07-01T00:00:00Z", created_at: "2024-07-01T00:00:00Z" },
  { id: "gpt-4.1", name: "GPT-4.1", description: "Latest GPT-4 series", family: "openai", stability: "stable", status: "active", free: false, released_at: "2025-04-01T00:00:00Z", created_at: "2025-04-01T00:00:00Z" },
  { id: "o3", name: "o3", description: "Reasoning model", family: "openai", stability: "stable", status: "active", free: false, released_at: "2025-04-01T00:00:00Z", created_at: "2025-04-01T00:00:00Z" },
  { id: "claude-sonnet-4-20250514", name: "Claude Sonnet 4", description: "Balanced performance", family: "anthropic", stability: "stable", status: "active", free: false, released_at: "2025-05-01T00:00:00Z", created_at: "2025-05-01T00:00:00Z" },
  { id: "claude-3-5-haiku-20241022", name: "Claude 3.5 Haiku", description: "Fast and affordable", family: "anthropic", stability: "stable", status: "active", free: false, released_at: "2024-10-01T00:00:00Z", created_at: "2024-10-01T00:00:00Z" },
  { id: "gemini-2.5-pro", name: "Gemini 2.5 Pro", description: "Google's most capable", family: "google", stability: "stable", status: "active", free: false, released_at: "2025-03-01T00:00:00Z", created_at: "2025-03-01T00:00:00Z" },
  { id: "gemini-2.5-flash", name: "Gemini 2.5 Flash", description: "Fast and efficient", family: "google", stability: "stable", status: "active", free: false, released_at: "2025-03-01T00:00:00Z", created_at: "2025-03-01T00:00:00Z" },
  { id: "deepseek-chat", name: "DeepSeek Chat", description: "General chat model", family: "deepseek", stability: "stable", status: "active", free: false, released_at: "2024-12-01T00:00:00Z", created_at: "2024-12-01T00:00:00Z" },
  { id: "text-embedding-3-small", name: "Text Embedding 3 Small", description: "Embedding model", family: "openai", stability: "stable", status: "active", free: false, released_at: "2024-01-01T00:00:00Z", created_at: "2024-01-01T00:00:00Z" },
  { id: "dall-e-3", name: "DALL-E 3", description: "Image generation", family: "openai", stability: "stable", status: "active", free: false, released_at: "2024-01-01T00:00:00Z", created_at: "2024-01-01T00:00:00Z" },
]

const MAPPINGS: ModelProviderMapping[] = [
  { id: "map-1", model_id: "gpt-4o", provider_id: "openai", external_id: "gpt-4o-2024-08-06", input_price: 2.5, output_price: 10, context_size: 128000, max_output: 16384, streaming: true, vision: true, reasoning: false, tools: true, json_output: true, stability: "stable", status: "active" },
  { id: "map-2", model_id: "gpt-4o-mini", provider_id: "openai", external_id: "gpt-4o-mini-2024-07-18", input_price: 0.15, output_price: 0.6, context_size: 128000, max_output: 16384, streaming: true, vision: true, reasoning: false, tools: true, json_output: true, stability: "stable", status: "active" },
  { id: "map-3", model_id: "claude-sonnet-4-20250514", provider_id: "anthropic", external_id: "claude-sonnet-4-20250514", input_price: 3, output_price: 15, context_size: 200000, max_output: 8192, streaming: true, vision: true, reasoning: false, tools: true, json_output: true, stability: "stable", status: "active" },
  { id: "map-4", model_id: "gemini-2.5-pro", provider_id: "google", external_id: "gemini-2.5-pro-preview-06-05", input_price: 1.25, output_price: 10, context_size: 1000000, max_output: 65536, streaming: true, vision: true, reasoning: true, tools: true, json_output: true, stability: "stable", status: "active" },
  { id: "map-5", model_id: "deepseek-chat", provider_id: "deepseek", external_id: "deepseek-chat", input_price: 0.14, output_price: 0.28, context_size: 64000, max_output: 8192, streaming: true, vision: false, reasoning: false, tools: true, json_output: true, stability: "stable", status: "active" },
]

const PROVIDER_KEYS: ProviderKey[] = [
  { id: "pk-1", provider_id: "openai", token_masked: "sk-proj-...a1b2", name: "OpenAI Production", description: "Main production key", managed: true, status: "active", sort_order: 1, created_at: "2024-06-01T00:00:00Z" },
  { id: "pk-2", provider_id: "anthropic", token_masked: "sk-ant-...c3d4", name: "Anthropic Production", description: "Claude API key", managed: true, status: "active", sort_order: 1, created_at: "2024-06-01T00:00:00Z" },
  { id: "pk-3", provider_id: "google", token_masked: "AIza...e5f6", name: "Google AI Key", description: "Gemini API key", managed: true, status: "active", sort_order: 1, created_at: "2024-06-01T00:00:00Z" },
  { id: "pk-4", provider_id: "openai", tenant_id: "tenant-1", token_masked: "sk-proj-...g7h8", name: "My OpenAI Key", description: "BYOK key", managed: false, status: "active", created_at: "2024-08-01T00:00:00Z" },
]

const MOCK_TENANT = "tenant-mock-1"

let mockBalance = 142.57
const TRANSACTIONS: Transaction[] = [
  { id: "tx-1", type: "top_up", amount: 200, balance_after: 200, description: "Initial credit", created_at: "2024-06-01T00:00:00Z" },
  { id: "tx-2", type: "usage", amount: -12.43, balance_after: 187.57, description: "gpt-4o usage", created_at: "2025-08-25T10:30:00Z" },
  { id: "tx-3", type: "usage", amount: -3.21, balance_after: 184.36, description: "claude-sonnet-4 usage", created_at: "2025-08-25T14:15:00Z" },
  { id: "tx-4", type: "usage", amount: -0.89, balance_after: 183.47, description: "gpt-4o-mini usage", created_at: "2025-08-26T09:00:00Z" },
  { id: "tx-5", type: "top_up", amount: 50, balance_after: 233.47, description: "Monthly top-up", created_at: "2025-08-26T12:00:00Z" },
  { id: "tx-6", type: "usage", amount: -90.90, balance_after: 142.57, description: "Batch processing", created_at: "2025-08-27T08:00:00Z" },
]

const USAGE_LOGS: UsageLog[] = Array.from({ length: 25 }, (_, i) => {
  const models = ["gpt-4o", "gpt-4o-mini", "claude-sonnet-4-20250514", "gemini-2.5-pro", "deepseek-chat"]
  const providers = ["openai", "openai", "anthropic", "google", "deepseek"]
  const idx = i % 5
  const promptTokens = 100 + Math.floor(Math.random() * 2000)
  const completionTokens = 50 + Math.floor(Math.random() * 1500)
  return {
    id: `log-${i + 1}`,
    tenant_id: MOCK_TENANT,
    requested_model: models[idx],
    used_model: models[idx],
    used_provider: providers[idx],
    prompt_tokens: promptTokens,
    completion_tokens: completionTokens,
    total_tokens: promptTokens + completionTokens,
    input_cost: promptTokens * 0.0000025,
    output_cost: completionTokens * 0.00001,
    total_cost: promptTokens * 0.0000025 + completionTokens * 0.00001,
    duration_ms: 500 + Math.floor(Math.random() * 3000),
    streamed: Math.random() > 0.3,
    status_code: i === 7 ? 500 : 200,
    has_error: i === 7,
    created_at: new Date(Date.now() - i * 3600000).toISOString(),
  }
})

const API_KEYS: ApiKey[] = [
  { id: "ak-1", key_prefix: "fr_live_a1b2c3d4...", tenant_id: MOCK_TENANT, name: "Production Key", description: "Main API key for production", scopes: ["gateway:read", "gateway:chat", "billing:read", "usage:read"], is_active: true, last_used_at: "2025-08-27T10:00:00Z", created_at: "2024-06-01T00:00:00Z" },
  { id: "ak-2", key_prefix: "fr_live_e5f6g7h8...", tenant_id: MOCK_TENANT, name: "Analytics Key", description: "Read-only for dashboards", scopes: ["billing:read", "usage:read"], is_active: true, created_at: "2024-07-01T00:00:00Z" },
  { id: "ak-3", key_prefix: "fr_live_i9j0k1l2...", tenant_id: MOCK_TENANT, name: "GPT-4o Only", description: "Restricted to GPT-4o", scopes: ["gateway:chat"], allowed_models: ["gpt-4o", "gpt-4o-mini"], is_active: true, created_at: "2025-01-15T00:00:00Z" },
  { id: "ak-4", key_prefix: "fr_test_m3n4o5p6...", tenant_id: MOCK_TENANT, name: "Test Key", scopes: ["gateway:chat"], is_active: false, expires_at: "2025-06-01T00:00:00Z", created_at: "2025-01-01T00:00:00Z" },
]

const WEBHOOKS: WebhookConfig[] = [
  { id: "wh-1", tenant_id: MOCK_TENANT, url: "https://hooks.example.com/freerouter", events: ["request.completed", "request.failed"], enabled: true, created_at: "2025-01-01T00:00:00Z", updated_at: "2025-08-01T00:00:00Z" },
  { id: "wh-2", tenant_id: MOCK_TENANT, url: "https://monitoring.example.com/alerts", events: ["spending.warning", "spending.exceeded", "key.health_degraded"], enabled: true, created_at: "2025-03-01T00:00:00Z", updated_at: "2025-03-01T00:00:00Z" },
]

const GUARDRAIL_RULES: GuardrailRule[] = [
  { id: "gr-1", tenant_id: MOCK_TENANT, name: "Block profanity", type: "blocked_terms", config: { terms: ["badword1", "badword2"], match_type: "contains", case_sensitive: false }, priority: 1, enabled: true, action: "block", created_at: "2025-02-01T00:00:00Z", updated_at: "2025-02-01T00:00:00Z" },
  { id: "gr-2", tenant_id: MOCK_TENANT, name: "SSN detection", type: "custom_regex", config: { pattern: "\\d{3}-\\d{2}-\\d{4}" }, priority: 2, enabled: true, action: "redact", created_at: "2025-02-15T00:00:00Z", updated_at: "2025-02-15T00:00:00Z" },
]

// ============================================================================
// Mock Adapters
// ============================================================================

const providers: ProviderPort = {
  list: async () => { await delay(); return { data: [...PROVIDERS], total: PROVIDERS.length } },
  get: async (id) => { await delay(); const p = PROVIDERS.find(p => p.id === id); if (!p) throw new Error("Not found"); return { ...p } },
  create: async (req) => { await delay(); const p: Provider = { id: req.name.toLowerCase().replace(/\s+/g, "-"), ...req, website: req.website ?? "", status: "active", streaming: req.streaming ?? false, created_at: now() }; PROVIDERS.push(p); return p },
  update: async (id, req) => { await delay(); const p = PROVIDERS.find(p => p.id === id); if (!p) throw new Error("Not found"); Object.assign(p, req); return { ...p } },
  delete: async (id) => { await delay(); const idx = PROVIDERS.findIndex(p => p.id === id); if (idx >= 0) PROVIDERS.splice(idx, 1) },
}

const models: ModelPort = {
  list: async () => { await delay(); return { data: [...MODELS], total: MODELS.length } },
  get: async (id) => { await delay(); const m = MODELS.find(m => m.id === id); if (!m) throw new Error("Not found"); return { ...m } },
  getWithMappings: async (id) => { await delay(); const m = MODELS.find(m => m.id === id); if (!m) throw new Error("Not found"); return { model: { ...m }, mappings: MAPPINGS.filter(mp => mp.model_id === id) } as ModelWithMappings },
  create: async (req) => { await delay(); const m: Model = { id: req.name.toLowerCase().replace(/\s+/g, "-"), ...req, stability: "stable", status: "active", free: req.free ?? false, released_at: now(), created_at: now() }; MODELS.push(m); return m },
  update: async (id, req) => { await delay(); const m = MODELS.find(m => m.id === id); if (!m) throw new Error("Not found"); Object.assign(m, req); return { ...m } },
  delete: async (id) => { await delay(); const idx = MODELS.findIndex(m => m.id === id); if (idx >= 0) MODELS.splice(idx, 1) },
}

const mappings: MappingPort = {
  get: async (id) => { await delay(); const m = MAPPINGS.find(m => m.id === id); if (!m) throw new Error("Not found"); return { ...m } },
  create: async (req) => { await delay(); const m = { id: newId(), ...req, streaming: req.streaming ?? false, vision: req.vision ?? false, reasoning: req.reasoning ?? false, tools: req.tools ?? false, json_output: req.json_output ?? false, stability: "stable" as const, status: "active" as const }; MAPPINGS.push(m); return m },
  update: async (id, req) => { await delay(); const m = MAPPINGS.find(m => m.id === id); if (!m) throw new Error("Not found"); Object.assign(m, req); return { ...m } },
  delete: async (id) => { await delay(); const idx = MAPPINGS.findIndex(m => m.id === id); if (idx >= 0) MAPPINGS.splice(idx, 1) },
}

const FALLBACKS: ModelFallback[] = [
  { id: "fb-1", model_id: "gpt-4o", fallback_model_id: "gpt-4o-mini", priority: 1, enabled: true, created_at: "2025-01-01T00:00:00Z" },
]

const modelFallbacks: ModelFallbackPort = {
  listByModel: async (modelId) => { await delay(); const data = FALLBACKS.filter(f => f.model_id === modelId); return { data, total: data.length } },
  create: async (req) => { await delay(); const f: ModelFallback = { id: newId(), ...req, enabled: true, created_at: now() }; FALLBACKS.push(f); return f },
  delete: async (id) => { await delay(); const idx = FALLBACKS.findIndex(f => f.id === id); if (idx >= 0) FALLBACKS.splice(idx, 1) },
}

const providerKeys: ProviderKeyPort = {
  listByProvider: async (providerId) => { await delay(); const data = PROVIDER_KEYS.filter(k => k.provider_id === providerId); return { data, total: data.length } },
  listByTenant: async (tenantId) => { await delay(); const data = PROVIDER_KEYS.filter(k => k.tenant_id === tenantId); return { data, total: data.length } },
  listManaged: async () => { await delay(); const data = PROVIDER_KEYS.filter(k => k.managed); return { data, total: data.length } },
  get: async (id) => { await delay(); const k = PROVIDER_KEYS.find(k => k.id === id); if (!k) throw new Error("Not found"); return { ...k } },
  create: async (req) => { await delay(); const k: ProviderKey = { id: newId(), provider_id: req.provider_id, tenant_id: req.tenant_id, token_masked: req.token.slice(0, 8) + "...", base_url: req.base_url, name: req.name, description: req.description ?? "", managed: !req.tenant_id, status: "active", created_at: now() }; PROVIDER_KEYS.push(k); return k },
  update: async (id, req) => { await delay(); const k = PROVIDER_KEYS.find(k => k.id === id); if (!k) throw new Error("Not found"); Object.assign(k, req); return { ...k } },
  delete: async (id) => { await delay(); const idx = PROVIDER_KEYS.findIndex(k => k.id === id); if (idx >= 0) PROVIDER_KEYS.splice(idx, 1) },
  test: async () => { await delay(500); return { success: true, message: "Connection successful. Models available." } as TestKeyResult },
}

const billing: BillingPort = {
  getBalance: async () => { await delay(); return { tenant_id: MOCK_TENANT, balance: mockBalance, updated_at: now() } as Balance },
  topUp: async (req) => { await delay(); mockBalance += req.amount; const tx: Transaction = { id: newId(), type: "top_up", amount: req.amount, balance_after: mockBalance, description: req.description, reference_id: req.reference_id, created_at: now() }; TRANSACTIONS.unshift(tx); return { balance: { tenant_id: MOCK_TENANT, balance: mockBalance, updated_at: now() }, transaction: tx } },
  adjust: async (req) => { await delay(); mockBalance += req.amount; const tx: Transaction = { id: newId(), type: "adjust", amount: req.amount, balance_after: mockBalance, description: req.description, created_at: now() }; TRANSACTIONS.unshift(tx); return { balance: { tenant_id: MOCK_TENANT, balance: mockBalance, updated_at: now() }, transaction: tx } },
  listTransactions: async (params = {}) => { await delay(); let data = [...TRANSACTIONS]; if (params.type) data = data.filter(t => t.type === params.type); const offset = params.offset ?? 0; const limit = params.limit ?? 50; return { data: data.slice(offset, offset + limit), total: data.length } },
  getSpendingLimit: async () => { await delay(); return { tenant_id: MOCK_TENANT, daily_limit_usd: 50, monthly_limit_usd: 500, created_at: "2025-01-01T00:00:00Z", updated_at: "2025-08-01T00:00:00Z" } as SpendingLimit },
  upsertSpendingLimit: async (_tenantId, req) => { await delay(); return { tenant_id: MOCK_TENANT, ...req, created_at: "2025-01-01T00:00:00Z", updated_at: now() } as SpendingLimit },
  deleteSpendingLimit: async () => { await delay() },
  checkSpending: async () => { await delay(); return { allowed: true, daily_spend_usd: 12.43, monthly_spend_usd: 107.43, daily_limit_usd: 50, monthly_limit_usd: 500 } as SpendingCheck },
}

const usage: UsagePort = {
  listLogs: async (params = {}) => { await delay(); let data = [...USAGE_LOGS]; if (params.model) data = data.filter(l => l.requested_model === params.model); if (params.provider) data = data.filter(l => l.used_provider === params.provider); const offset = params.offset ?? 0; const limit = params.limit ?? 50; return { data: data.slice(offset, offset + limit), total: data.length } },
  getLog: async (id) => { await delay(); const log = USAGE_LOGS.find(l => l.id === id); if (!log) throw new Error("Not found"); return { ...log, messages: [{ role: "user", content: "Hello, how are you?" }], response_body: { choices: [{ message: { role: "assistant", content: "I'm doing well!" } }] }, is_debug: false } as UsageLogDetail },
  getSummary: async () => {
    await delay()
    const totalCost = USAGE_LOGS.reduce((s, l) => s + l.total_cost, 0)
    const totalTokens = USAGE_LOGS.reduce((s, l) => s + l.total_tokens, 0)
    return {
      summary: { tenant_id: MOCK_TENANT, total_requests: USAGE_LOGS.length, total_tokens: totalTokens, prompt_tokens: Math.floor(totalTokens * 0.6), completion_tokens: Math.floor(totalTokens * 0.4), total_cost: totalCost, error_count: 1 },
      by_model: [
        { model: "gpt-4o", total_requests: 10, total_tokens: 15000, prompt_tokens: 9000, completion_tokens: 6000, total_cost: totalCost * 0.45 },
        { model: "claude-sonnet-4-20250514", total_requests: 5, total_tokens: 8000, prompt_tokens: 5000, completion_tokens: 3000, total_cost: totalCost * 0.30 },
        { model: "gpt-4o-mini", total_requests: 5, total_tokens: 6000, prompt_tokens: 3600, completion_tokens: 2400, total_cost: totalCost * 0.05 },
        { model: "gemini-2.5-pro", total_requests: 3, total_tokens: 4000, prompt_tokens: 2400, completion_tokens: 1600, total_cost: totalCost * 0.15 },
        { model: "deepseek-chat", total_requests: 2, total_tokens: 3000, prompt_tokens: 1800, completion_tokens: 1200, total_cost: totalCost * 0.05 },
      ],
      period_start: new Date(Date.now() - 30 * 86400000).toISOString(),
      period_end: now(),
    } as UsageSummaryResponse
  },
  getRetention: async () => { await delay(); return { tenant_id: MOCK_TENANT, retention_days: 30, retain_messages: true, retain_response_body: true, retain_debug_payloads: false, created_at: "2025-01-01T00:00:00Z", updated_at: "2025-08-01T00:00:00Z" } as DataRetentionConfig },
  upsertRetention: async (_tenantId, req) => { await delay(); return { tenant_id: MOCK_TENANT, ...req, created_at: "2025-01-01T00:00:00Z", updated_at: now() } as DataRetentionConfig },
  deleteRetention: async () => { await delay() },
}

const guardrails: GuardrailsPort = {
  getConfig: async () => { await delay(); return { id: "gc-1", tenant_id: MOCK_TENANT, enabled: true, system_rules: { prompt_injection: { enabled: true, action: "block" }, jailbreak: { enabled: true, action: "block" }, pii_detection: { enabled: true, action: "redact" }, secrets: { enabled: true, action: "block" }, document_leakage: { enabled: false, action: "allow" } }, created_at: "2025-02-01T00:00:00Z", updated_at: "2025-08-01T00:00:00Z" } as GuardrailConfig },
  upsertConfig: async (req) => { await delay(); return { id: "gc-1", tenant_id: MOCK_TENANT, enabled: req.enabled ?? true, system_rules: { prompt_injection: { enabled: true, action: "block" }, jailbreak: { enabled: true, action: "block" }, pii_detection: { enabled: true, action: "redact" }, secrets: { enabled: true, action: "block" }, document_leakage: { enabled: false, action: "allow" }, ...req.system_rules }, created_at: "2025-02-01T00:00:00Z", updated_at: now() } as GuardrailConfig },
  listRules: async () => { await delay(); return { data: [...GUARDRAIL_RULES], total: GUARDRAIL_RULES.length } },
  createRule: async (req) => { await delay(); const r: GuardrailRule = { id: newId(), tenant_id: MOCK_TENANT, name: req.name, type: req.type, config: req.config, priority: req.priority ?? 10, enabled: true, action: req.action, created_at: now(), updated_at: now() }; GUARDRAIL_RULES.push(r); return r },
  updateRule: async (id, req) => { await delay(); const r = GUARDRAIL_RULES.find(r => r.id === id); if (!r) throw new Error("Not found"); Object.assign(r, req, { updated_at: now() }); return { ...r } },
  deleteRule: async (id) => { await delay(); const idx = GUARDRAIL_RULES.findIndex(r => r.id === id); if (idx >= 0) GUARDRAIL_RULES.splice(idx, 1) },
  listViolations: async () => {
    await delay()
    const data: GuardrailViolation[] = [
      { id: "v-1", tenant_id: MOCK_TENANT, rule_id: "system-pii", rule_name: "PII Detection", category: "pii", action_taken: "redacted", matched_pattern: "\\d{3}-\\d{2}-\\d{4}", matched_content: "***-**-****", model: "gpt-4o", created_at: "2025-08-27T09:00:00Z" },
      { id: "v-2", tenant_id: MOCK_TENANT, rule_id: "gr-1", rule_name: "Block profanity", category: "custom", action_taken: "blocked", model: "gpt-4o-mini", created_at: "2025-08-26T15:30:00Z" },
    ]
    return { data, total: data.length }
  },
  testCheck: async (messages) => { await delay(); const hasSSN = messages.some(m => /\d{3}-\d{2}-\d{4}/.test(m)); return { passed: !hasSSN, blocked: false, violations: hasSSN ? [{ rule: "SSN detection", message: "Found SSN pattern" }] : [], redactions: [] } as GuardrailCheckResult },
}

const webhooks: WebhooksPort = {
  list: async () => { await delay(); return { data: [...WEBHOOKS], total: WEBHOOKS.length } },
  listEvents: async () => { await delay(); return ["request.completed", "request.failed", "spending.warning", "spending.exceeded", "key.health_degraded", "key.blacklisted"] },
  get: async (id) => { await delay(); const w = WEBHOOKS.find(w => w.id === id); if (!w) throw new Error("Not found"); return { ...w } },
  create: async (req) => { await delay(); const w: WebhookConfig = { id: newId(), tenant_id: MOCK_TENANT, url: req.url, secret: "whsec_mock_secret_" + Math.random().toString(36).slice(2), events: req.events, enabled: true, created_at: now(), updated_at: now() }; WEBHOOKS.push(w); return w },
  update: async (id, req) => { await delay(); const w = WEBHOOKS.find(w => w.id === id); if (!w) throw new Error("Not found"); Object.assign(w, req, { updated_at: now() }); return { ...w } },
  delete: async (id) => { await delay(); const idx = WEBHOOKS.findIndex(w => w.id === id); if (idx >= 0) WEBHOOKS.splice(idx, 1) },
  listDeliveries: async () => {
    await delay()
    const data: WebhookDelivery[] = [
      { id: "wd-1", webhook_id: "wh-1", event_type: "request.completed", payload: '{"event":"request.completed","data":{}}', status: "success", status_code: 200, attempts: 1, created_at: "2025-08-27T10:00:00Z", completed_at: "2025-08-27T10:00:01Z" },
      { id: "wd-2", webhook_id: "wh-1", event_type: "request.completed", payload: '{"event":"request.completed","data":{}}', status: "success", status_code: 200, attempts: 1, created_at: "2025-08-27T09:30:00Z", completed_at: "2025-08-27T09:30:01Z" },
      { id: "wd-3", webhook_id: "wh-1", event_type: "request.failed", payload: '{"event":"request.failed","data":{}}', status: "failed", attempts: 3, last_error: "connection refused", created_at: "2025-08-26T22:00:00Z" },
    ]
    return { data, total: data.length }
  },
  test: async () => { await delay(); return { message: "Test event dispatched" } },
}

const apiKeys: ApiKeysPort = {
  list: async () => { await delay(); return { data: [...API_KEYS], total: API_KEYS.length } },
  get: async (id) => { await delay(); const k = API_KEYS.find(k => k.id === id); if (!k) throw new Error("Not found"); return { ...k } },
  create: async (req) => { await delay(); const key: ApiKey = { id: newId(), key_prefix: "fr_live_" + Math.random().toString(36).slice(2, 10) + "...", tenant_id: MOCK_TENANT, name: req.name, description: req.description, scopes: req.scopes, allowed_models: req.allowed_models, is_active: true, created_at: now() }; API_KEYS.push(key); return { api_key: key, secret_key: "fr_live_" + Math.random().toString(36).slice(2) + Math.random().toString(36).slice(2), message: "Store this key securely. It will not be shown again." } as CreateApiKeyResponse },
  update: async (id, req) => { await delay(); const k = API_KEYS.find(k => k.id === id); if (!k) throw new Error("Not found"); Object.assign(k, req); return { ...k } },
  revoke: async (id) => { await delay(); const k = API_KEYS.find(k => k.id === id); if (k) k.is_active = false },
  delete: async (id) => { await delay(); const idx = API_KEYS.findIndex(k => k.id === id); if (idx >= 0) API_KEYS.splice(idx, 1) },
}

let mockRateLimit: RateLimitConfig = { tenant_id: MOCK_TENANT, rpm: 60, max_concurrent: 10 }
let mockRouting: RoutingConfig = { tenant_id: MOCK_TENANT, strategy: "cheapest" }

const gatewayConfig: GatewayConfigPort = {
  getRateLimit: async () => { await delay(); return { ...mockRateLimit } },
  upsertRateLimit: async (_tenantId, req) => { await delay(); mockRateLimit = { ...mockRateLimit, ...req }; return { ...mockRateLimit } },
  deleteRateLimit: async () => { await delay(); mockRateLimit = { tenant_id: MOCK_TENANT, rpm: 60, max_concurrent: 10 } },
  getRouting: async () => { await delay(); return { ...mockRouting } },
  upsertRouting: async (_tenantId, req) => { await delay(); mockRouting = { ...mockRouting, ...req }; return { ...mockRouting } },
  deleteRouting: async () => { await delay(); mockRouting = { tenant_id: MOCK_TENANT, strategy: "cheapest" } },
  invalidateCache: async () => { await delay(); return { message: "Cache invalidated", keys_deleted: 42 } },
  estimateCost: async (req) => { await delay(); return { model: req.model, provider: "openai", estimated_input_tokens: 150, max_output_tokens: req.max_tokens ?? 4096, input_price_per_million: 2.5, output_price_per_million: 10, estimated_input_cost: 0.000375, estimated_output_cost: 0.04096, estimated_total_cost: 0.041335 } as CostEstimateResponse },
}

const users: UsersPort = {
  me: async () => { await delay(); return { id: "user-1", tenant_id: MOCK_TENANT, email: "admin@example.com", name: "Admin User", oauth_provider: "GOOGLE", status: "ACTIVE", scopes: ["*"], email_verified: true, last_login_at: now(), created_at: "2024-06-01T00:00:00Z", updated_at: now() } as User },
  list: async () => { await delay(); const data: User[] = [{ id: "user-1", tenant_id: MOCK_TENANT, email: "admin@example.com", name: "Admin User", oauth_provider: "GOOGLE", status: "ACTIVE", scopes: ["*"], email_verified: true, last_login_at: now(), created_at: "2024-06-01T00:00:00Z", updated_at: now() }, { id: "user-2", tenant_id: MOCK_TENANT, email: "dev@example.com", name: "Developer", oauth_provider: "GOOGLE", status: "ACTIVE", scopes: ["gateway:chat", "usage:read"], email_verified: true, created_at: "2024-08-01T00:00:00Z", updated_at: now() }]; return { data, total: data.length } },
  get: async (id) => { await delay(); return { id, tenant_id: MOCK_TENANT, email: "admin@example.com", name: "Admin User", oauth_provider: "GOOGLE", status: "ACTIVE", scopes: ["*"], email_verified: true, created_at: "2024-06-01T00:00:00Z", updated_at: now() } as User },
  activate: async () => { await delay() },
  suspend: async () => { await delay() },
  delete: async () => { await delay() },
}

// ============================================================================
// Combined Mock API
// ============================================================================

export const mockApi: ApiPort = {
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
}
