import type {
  ApiPort,
  ProviderPort,
  ModelPort,
  MappingPort,
  ModelFallbackPort,
  ProviderKeyPort,
  GatewayConfigPort,
  BillingPort,
  WalletsPort,
  UsagePort,
  GuardrailsPort,
  WebhooksPort,
  ApiKeysPort,
  UsersPort,
  RolesPort,
  InvitationsPort,
} from "../ports"
import type {
  Provider,
  Model,
  ModelProviderMapping,
  ModelFallback,
  ProviderKey,
  RateLimitConfig,
  RoutingConfig,
  Transaction,
  SpendingLimit,
  Wallet,
  DataRetentionConfig,
  GuardrailConfig,
  GuardrailRule,
  WebhookConfig,
  WebhookDelivery,
  ApiKey,
  User,
  Role,
  Invitation,
} from "../types"
import {
  providers,
  models,
  mappings,
  fallbacks,
  providerKeys,
  rateLimits,
  routingConfigs,
  balances,
  transactions,
  spendingLimits,
  wallets as walletsData,
  usageLogs,
  guardrailConfigs,
  guardrailRules,
  guardrailViolations,
  webhooks,
  webhookDeliveries,
  roles,
  users,
  invitations,
  apiKeys,
  TENANT,
} from "./data"

// =============================================================================
// Helpers
// =============================================================================

const delay = () => new Promise((r) => setTimeout(r, 100 + Math.random() * 200))

const maskToken = (token: string) => `${token.slice(0, 3)}....${token.slice(-4)}`

const defaultTenantId = () => TENANT

// Local state not modeled as arrays in ./data (kept here since these
// resources have no dedicated mock data collection).
const retentionConfigs: DataRetentionConfig[] = []
const roleAssignments: Array<{ role_id: string; user_id: string }> = users
  .map((u) => {
    const role = roles.find((r) => r.scopes === u.scopes)
    return role ? { role_id: role.id, user_id: u.id } : null
  })
  .filter((a): a is { role_id: string; user_id: string } => a !== null)

// =============================================================================
// Provider Port
// =============================================================================

export const providerPort: ProviderPort = {
  async list() {
    await delay()
    return { data: [...providers], total: providers.length }
  },
  async get(id) {
    await delay()
    const item = providers.find((p) => p.id === id)
    if (!item) throw new Error("Not found")
    return item
  },
  async create(req) {
    await delay()
    const item: Provider = {
      id: crypto.randomUUID(),
      name: req.name,
      description: req.description,
      website: req.website,
      status: "active",
      streaming: req.streaming,
      created_at: new Date().toISOString(),
    }
    providers.push(item)
    return item
  },
  async update(id, req) {
    await delay()
    const item = providers.find((p) => p.id === id)
    if (!item) throw new Error("Not found")
    Object.assign(item, req)
    return item
  },
  async delete(id) {
    await delay()
    const idx = providers.findIndex((p) => p.id === id)
    if (idx !== -1) providers.splice(idx, 1)
  },
}

// =============================================================================
// Model Port
// =============================================================================

export const modelPort: ModelPort = {
  async list() {
    await delay()
    return { data: [...models], total: models.length }
  },
  async get(id) {
    await delay()
    const item = models.find((m) => m.id === id)
    if (!item) throw new Error("Not found")
    return item
  },
  async getWithMappings(id) {
    await delay()
    const model = models.find((m) => m.id === id)
    if (!model) throw new Error("Not found")
    const modelMappings = mappings.filter((m) => m.model_id === id)
    return { model, mappings: modelMappings }
  },
  async create(req) {
    await delay()
    const item: Model = {
      id: crypto.randomUUID(),
      name: req.name,
      description: req.description,
      family: req.family,
      stability: "stable",
      status: "active",
      free: req.free,
      released_at: new Date().toISOString(),
      created_at: new Date().toISOString(),
    }
    models.push(item)
    return item
  },
  async update(id, req) {
    await delay()
    const item = models.find((m) => m.id === id)
    if (!item) throw new Error("Not found")
    Object.assign(item, req)
    return item
  },
  async delete(id) {
    await delay()
    const idx = models.findIndex((m) => m.id === id)
    if (idx !== -1) models.splice(idx, 1)
  },
}

// =============================================================================
// Mapping Port
// =============================================================================

export const mappingPort: MappingPort = {
  async get(id) {
    await delay()
    const item = mappings.find((m) => m.id === id)
    if (!item) throw new Error("Not found")
    return item
  },
  async create(req) {
    await delay()
    const item: ModelProviderMapping = {
      id: crypto.randomUUID(),
      model_id: req.model_id,
      provider_id: req.provider_id,
      external_id: req.external_id,
      input_price: req.input_price ?? null,
      output_price: req.output_price ?? null,
      cached_input_price: req.cached_input_price ?? null,
      request_price: req.request_price ?? null,
      image_input_price: req.image_input_price ?? null,
      context_size: req.context_size ?? null,
      max_output: req.max_output ?? null,
      streaming: req.streaming,
      vision: req.vision,
      reasoning: req.reasoning,
      tools: req.tools,
      json_output: req.json_output,
      region: req.region ?? null,
      stability: "stable",
      status: "active",
      created_at: new Date().toISOString(),
      updated_at: new Date().toISOString(),
    }
    mappings.push(item)
    return item
  },
  async update(id, req) {
    await delay()
    const item = mappings.find((m) => m.id === id)
    if (!item) throw new Error("Not found")
    Object.assign(item, req)
    item.updated_at = new Date().toISOString()
    return item
  },
  async delete(id) {
    await delay()
    const idx = mappings.findIndex((m) => m.id === id)
    if (idx !== -1) mappings.splice(idx, 1)
  },
}

// =============================================================================
// Model Fallback Port
// =============================================================================

export const modelFallbackPort: ModelFallbackPort = {
  async listByModel(modelId) {
    await delay()
    const filtered = fallbacks.filter((f) => f.model_id === modelId)
    return { data: filtered, total: filtered.length }
  },
  async create(req) {
    await delay()
    const item: ModelFallback = {
      id: crypto.randomUUID(),
      model_id: req.model_id,
      fallback_model_id: req.fallback_model_id,
      priority: req.priority,
      enabled: true,
      created_at: new Date().toISOString(),
    }
    fallbacks.push(item)
    return item
  },
  async delete(id) {
    await delay()
    const idx = fallbacks.findIndex((f) => f.id === id)
    if (idx !== -1) fallbacks.splice(idx, 1)
  },
}

// =============================================================================
// Provider Key Port
// =============================================================================

export const providerKeyPort: ProviderKeyPort = {
  async get(id) {
    await delay()
    const item = providerKeys.find((k) => k.id === id)
    if (!item) throw new Error("Not found")
    return item
  },
  async create(req) {
    await delay()
    const item: ProviderKey = {
      id: crypto.randomUUID(),
      provider_id: req.provider_id,
      tenant_id: req.tenant_id ?? null,
      token_masked: maskToken(req.token),
      base_url: req.base_url ?? null,
      name: req.name,
      description: req.description,
      managed: !req.tenant_id,
      status: "active",
      sort_order: null,
      created_at: new Date().toISOString(),
    }
    providerKeys.push(item)
    return item
  },
  async update(id, req) {
    await delay()
    const item = providerKeys.find((k) => k.id === id)
    if (!item) throw new Error("Not found")
    if (req.token !== undefined) item.token_masked = maskToken(req.token)
    if (req.base_url !== undefined) item.base_url = req.base_url
    if (req.name !== undefined) item.name = req.name
    if (req.description !== undefined) item.description = req.description
    if (req.status !== undefined) item.status = req.status
    if (req.sort_order !== undefined) item.sort_order = req.sort_order
    return item
  },
  async delete(id) {
    await delay()
    const idx = providerKeys.findIndex((k) => k.id === id)
    if (idx !== -1) providerKeys.splice(idx, 1)
  },
  async listByProvider(providerId) {
    await delay()
    const filtered = providerKeys.filter((k) => k.provider_id === providerId)
    return { data: filtered, total: filtered.length }
  },
  async listByTenant(tenantId) {
    await delay()
    const filtered = providerKeys.filter((k) => k.tenant_id === tenantId)
    return { data: filtered, total: filtered.length }
  },
  async listManaged() {
    await delay()
    const filtered = providerKeys.filter((k) => k.managed)
    return { data: filtered, total: filtered.length }
  },
  async test(id) {
    await delay()
    const item = providerKeys.find((k) => k.id === id)
    if (!item) throw new Error("Not found")
    return { success: item.status === "active", latency_ms: Math.round(50 + Math.random() * 200) }
  },
}

// =============================================================================
// Gateway Config Port
// =============================================================================

export const gatewayConfigPort: GatewayConfigPort = {
  async getRateLimit(tenantId) {
    await delay()
    const item = rateLimits.find((r) => r.tenant_id === tenantId)
    if (!item) throw new Error("Not found")
    return item
  },
  async upsertRateLimit(tenantId, req) {
    await delay()
    const existing = rateLimits.find((r) => r.tenant_id === tenantId)
    if (existing) {
      if (req.rpm !== undefined) existing.rpm = req.rpm
      if (req.max_concurrent !== undefined) existing.max_concurrent = req.max_concurrent
      return existing
    }
    const item: RateLimitConfig = {
      tenant_id: tenantId,
      rpm: req.rpm ?? 60,
      max_concurrent: req.max_concurrent ?? 10,
    }
    rateLimits.push(item)
    return item
  },
  async deleteRateLimit(tenantId) {
    await delay()
    const idx = rateLimits.findIndex((r) => r.tenant_id === tenantId)
    if (idx !== -1) rateLimits.splice(idx, 1)
  },
  async getRouting(tenantId) {
    await delay()
    const item = routingConfigs.find((r) => r.tenant_id === tenantId)
    if (!item) throw new Error("Not found")
    return item
  },
  async upsertRouting(tenantId, req) {
    await delay()
    const existing = routingConfigs.find((r) => r.tenant_id === tenantId)
    if (existing) {
      existing.strategy = req.strategy
      existing.updated_at = new Date().toISOString()
      return existing
    }
    const item: RoutingConfig = {
      tenant_id: tenantId,
      strategy: req.strategy,
      created_at: new Date().toISOString(),
      updated_at: new Date().toISOString(),
    }
    routingConfigs.push(item)
    return item
  },
  async deleteRouting(tenantId) {
    await delay()
    const idx = routingConfigs.findIndex((r) => r.tenant_id === tenantId)
    if (idx !== -1) routingConfigs.splice(idx, 1)
  },
  async invalidateCache(_tenantId) {
    await delay()
  },
  async estimateCost(req) {
    await delay()
    const mapping = mappings.find((m) => m.external_id === req.model)
    const provider = mapping ? providers.find((p) => p.id === mapping.provider_id) : undefined
    const estimatedInputTokens = req.messages.reduce((sum, m) => sum + Math.ceil(m.content.length / 4), 0)
    const maxOutputTokens = req.max_tokens ?? mapping?.max_output ?? 1024
    const inputPrice = mapping?.input_price ?? null
    const outputPrice = mapping?.output_price ?? null
    const estimatedInputCost = inputPrice ? (estimatedInputTokens / 1_000_000) * inputPrice : 0
    const estimatedOutputCost = outputPrice ? (maxOutputTokens / 1_000_000) * outputPrice : 0
    return {
      model: req.model,
      provider: provider?.name ?? "unknown",
      estimated_input_tokens: estimatedInputTokens,
      max_output_tokens: maxOutputTokens,
      input_price_per_million: inputPrice,
      output_price_per_million: outputPrice,
      estimated_input_cost_usd: Number(estimatedInputCost.toFixed(6)),
      estimated_output_cost_usd: Number(estimatedOutputCost.toFixed(6)),
      estimated_total_cost_usd: Number((estimatedInputCost + estimatedOutputCost).toFixed(6)),
    }
  },
}

// =============================================================================
// Billing Port
// =============================================================================

export const billingPort: BillingPort = {
  async getBalance() {
    await delay()
    const balance = balances[0]
    if (!balance) throw new Error("Not found")
    return balance
  },
  async topUp(req) {
    await delay()
    const balance = balances[0]
    if (!balance) throw new Error("Not found")
    balance.balance += req.amount
    balance.updated_at = new Date().toISOString()
    const txn: Transaction = {
      id: crypto.randomUUID(),
      type: "top_up",
      amount: req.amount,
      balance_after: balance.balance,
      description: req.description,
      reference_id: req.reference_id,
      created_at: new Date().toISOString(),
    }
    transactions.unshift(txn)
    return { balance, transaction: txn }
  },
  async adjust(req) {
    await delay()
    const balance = balances[0]
    if (!balance) throw new Error("Not found")
    balance.balance += req.amount
    balance.updated_at = new Date().toISOString()
    const txn: Transaction = {
      id: crypto.randomUUID(),
      type: "adjust",
      amount: req.amount,
      balance_after: balance.balance,
      description: req.description,
      reference_id: crypto.randomUUID(),
      created_at: new Date().toISOString(),
    }
    transactions.unshift(txn)
    return { balance, transaction: txn }
  },
  async listTransactions(params) {
    await delay()
    let filtered = [...transactions]
    const type = params?.type
    const from = params?.from
    const to = params?.to
    if (type) filtered = filtered.filter((t) => t.type === type)
    if (from) filtered = filtered.filter((t) => t.created_at >= from)
    if (to) filtered = filtered.filter((t) => t.created_at <= to)
    const total = filtered.length
    const offset = params?.offset ?? 0
    const limit = params?.limit ?? total
    return { data: filtered.slice(offset, offset + limit), total }
  },
  async getConfig() {
    await delay()
    return { stripe_enabled: true, min_topup_usd: 5, max_topup_usd: 10000 }
  },
  async createCheckout(req) {
    await delay()
    // Mock: simulate an instant successful purchase, then "redirect" back
    const balance = balances[0]
    if (!balance) throw new Error("Not found")
    balance.balance += req.amount_usd
    balance.updated_at = new Date().toISOString()
    const txn: Transaction = {
      id: crypto.randomUUID(),
      type: "top_up",
      amount: req.amount_usd,
      balance_after: balance.balance,
      description: `Credit purchase of $${req.amount_usd.toFixed(2)} via Stripe`,
      reference_id: `stripe:cs_mock_${crypto.randomUUID().slice(0, 8)}`,
      created_at: new Date().toISOString(),
    }
    transactions.unshift(txn)
    return {
      session_id: txn.reference_id.replace("stripe:", ""),
      url: "/billing?checkout=success",
    }
  },
  async getSpendingLimit(tenantId) {
    await delay()
    const item = spendingLimits.find((s) => s.tenant_id === tenantId)
    if (!item) throw new Error("Not found")
    return item
  },
  async upsertSpendingLimit(tenantId, req) {
    await delay()
    const existing = spendingLimits.find((s) => s.tenant_id === tenantId)
    if (existing) {
      if (req.daily_limit_usd !== undefined) existing.daily_limit_usd = req.daily_limit_usd
      if (req.monthly_limit_usd !== undefined) existing.monthly_limit_usd = req.monthly_limit_usd
      existing.updated_at = new Date().toISOString()
      return existing
    }
    const item: SpendingLimit = {
      tenant_id: tenantId,
      daily_limit_usd: req.daily_limit_usd ?? null,
      monthly_limit_usd: req.monthly_limit_usd ?? null,
      created_at: new Date().toISOString(),
      updated_at: new Date().toISOString(),
    }
    spendingLimits.push(item)
    return item
  },
  async deleteSpendingLimit(tenantId) {
    await delay()
    const idx = spendingLimits.findIndex((s) => s.tenant_id === tenantId)
    if (idx !== -1) spendingLimits.splice(idx, 1)
  },
  async checkSpendingLimit(tenantId) {
    await delay()
    const limit = spendingLimits.find((s) => s.tenant_id === tenantId)
    const dayAgo = Date.now() - 86400000
    const monthAgo = Date.now() - 30 * 86400000
    const dailySpend = transactions
      .filter((t) => t.type === "usage" && new Date(t.created_at).getTime() >= dayAgo)
      .reduce((sum, t) => sum + Math.abs(t.amount), 0)
    const monthlySpend = transactions
      .filter((t) => t.type === "usage" && new Date(t.created_at).getTime() >= monthAgo)
      .reduce((sum, t) => sum + Math.abs(t.amount), 0)
    const dailyLimit = limit?.daily_limit_usd ?? null
    const monthlyLimit = limit?.monthly_limit_usd ?? null
    const allowed =
      (dailyLimit === null || dailySpend < dailyLimit) && (monthlyLimit === null || monthlySpend < monthlyLimit)
    return {
      allowed,
      daily_spend_usd: Number(dailySpend.toFixed(2)),
      monthly_spend_usd: Number(monthlySpend.toFixed(2)),
      daily_limit_usd: dailyLimit,
      monthly_limit_usd: monthlyLimit,
      reason: allowed ? "within limits" : "spending limit exceeded",
    }
  },
}

// =============================================================================
// Wallets Port
// =============================================================================

const walletTxn = (type: Transaction["type"], amount: number, balanceAfter: number, description: string) => {
  const txn: Transaction = {
    id: crypto.randomUUID(),
    type,
    amount,
    balance_after: balanceAfter,
    description,
    reference_id: "",
    created_at: new Date().toISOString(),
  }
  transactions.unshift(txn)
  return txn
}

export const walletsPort: WalletsPort = {
  async list() {
    await delay()
    return { wallets: [...walletsData], total: walletsData.length }
  },
  async get(id) {
    await delay()
    const w = walletsData.find((w) => w.id === id)
    if (!w) throw new Error("Wallet not found")
    return w
  },
  async create(req) {
    await delay()
    if (walletsData.some((w) => w.name === req.name)) throw new Error("A wallet with this name already exists")
    const w: Wallet = {
      id: crypto.randomUUID(),
      tenant_id: TENANT,
      name: req.name,
      description: req.description ?? "",
      balance: 0,
      created_at: new Date().toISOString(),
      updated_at: new Date().toISOString(),
    }
    walletsData.push(w)
    return w
  },
  async update(id, req) {
    await delay()
    const w = walletsData.find((w) => w.id === id)
    if (!w) throw new Error("Wallet not found")
    if (req.name !== undefined) w.name = req.name
    if (req.description !== undefined) w.description = req.description
    w.updated_at = new Date().toISOString()
    return w
  },
  async delete(id) {
    await delay()
    const idx = walletsData.findIndex((w) => w.id === id)
    if (idx === -1) throw new Error("Wallet not found")
    const w = walletsData[idx]!
    if (w.balance > 0) throw new Error("Wallet must be empty before deletion")
    if (apiKeys.some((k) => k.wallet_id === id)) throw new Error("Wallet is bound to one or more API keys")
    walletsData.splice(idx, 1)
  },
  async fund(id, req) {
    await delay()
    const w = walletsData.find((w) => w.id === id)
    const balance = balances[0]
    if (!w || !balance) throw new Error("Wallet not found")
    if (balance.balance < req.amount) throw new Error("Insufficient main balance to fund wallet")
    balance.balance -= req.amount
    balance.updated_at = new Date().toISOString()
    w.balance += req.amount
    w.updated_at = new Date().toISOString()
    walletTxn("wallet_fund", -req.amount, balance.balance, req.description || `Fund wallet "${w.name}" with $${req.amount.toFixed(2)}`)
    return { wallet: w, main_balance: balance }
  },
  async withdraw(id, req) {
    await delay()
    const w = walletsData.find((w) => w.id === id)
    const balance = balances[0]
    if (!w || !balance) throw new Error("Wallet not found")
    if (w.balance < req.amount) throw new Error("Insufficient wallet funds")
    w.balance -= req.amount
    w.updated_at = new Date().toISOString()
    balance.balance += req.amount
    balance.updated_at = new Date().toISOString()
    walletTxn("wallet_withdraw", req.amount, balance.balance, req.description || `Withdraw $${req.amount.toFixed(2)} from wallet "${w.name}"`)
    return { wallet: w, main_balance: balance }
  },
}

// =============================================================================
// Usage Port
// =============================================================================

export const usagePort: UsagePort = {
  async queryLogs(query) {
    await delay()
    let filtered = [...usageLogs]
    const { model, provider, from, to, limit, offset } = query
    if (model) filtered = filtered.filter((l) => l.requested_model === model || l.used_model === model)
    if (provider) filtered = filtered.filter((l) => l.used_provider === provider)
    if (from) filtered = filtered.filter((l) => l.created_at >= from)
    if (to) filtered = filtered.filter((l) => l.created_at <= to)
    const total = filtered.length
    const off = offset ?? 0
    const lim = limit ?? total
    return { data: filtered.slice(off, off + lim), total }
  },
  async getLog(id) {
    await delay()
    const log = usageLogs.find((l) => l.id === id)
    if (!log) throw new Error("Not found")
    return {
      ...log,
      messages: [{ role: "user", content: "Mock request message content." }],
      response_body: {
        id: log.id,
        choices: [{ message: { role: "assistant", content: "Mock response content." } }],
      },
      raw_request: {
        model: log.requested_model,
        messages: [{ role: "user", content: "Mock request message content." }],
      },
      raw_response: {
        model: log.used_model,
        usage: { prompt_tokens: log.prompt_tokens, completion_tokens: log.completion_tokens },
      },
      upstream_request: { provider: log.used_provider, model: log.used_model },
      upstream_response: { status: log.status_code },
      is_debug: false,
    }
  },
  async getSummary(params) {
    await delay()
    let logs = usageLogs
    const from = params?.from
    const to = params?.to
    if (from) logs = logs.filter((l) => l.created_at >= from)
    if (to) logs = logs.filter((l) => l.created_at <= to)

    const byModelMap = new Map<
      string,
      {
        model: string
        total_requests: number
        total_tokens: number
        prompt_tokens: number
        completion_tokens: number
        total_cost: number
      }
    >()
    for (const log of logs) {
      const existing = byModelMap.get(log.used_model) ?? {
        model: log.used_model,
        total_requests: 0,
        total_tokens: 0,
        prompt_tokens: 0,
        completion_tokens: 0,
        total_cost: 0,
      }
      existing.total_requests += 1
      existing.total_tokens += log.total_tokens
      existing.prompt_tokens += log.prompt_tokens
      existing.completion_tokens += log.completion_tokens
      existing.total_cost += log.total_cost
      byModelMap.set(log.used_model, existing)
    }

    return {
      summary: {
        tenant_id: defaultTenantId(),
        total_requests: logs.length,
        total_tokens: logs.reduce((sum, l) => sum + l.total_tokens, 0),
        prompt_tokens: logs.reduce((sum, l) => sum + l.prompt_tokens, 0),
        completion_tokens: logs.reduce((sum, l) => sum + l.completion_tokens, 0),
        total_cost: Number(logs.reduce((sum, l) => sum + l.total_cost, 0).toFixed(6)),
        error_count: logs.filter((l) => l.has_error).length,
      },
      by_model: Array.from(byModelMap.values()).map((m) => ({ ...m, total_cost: Number(m.total_cost.toFixed(6)) })),
      period_start: from ?? logs[logs.length - 1]?.created_at ?? new Date().toISOString(),
      period_end: to ?? logs[0]?.created_at ?? new Date().toISOString(),
    }
  },
  async getRetention(tenantId) {
    await delay()
    const item = retentionConfigs.find((r) => r.tenant_id === tenantId)
    if (!item) throw new Error("Not found")
    return item
  },
  async upsertRetention(tenantId, req) {
    await delay()
    const existing = retentionConfigs.find((r) => r.tenant_id === tenantId)
    if (existing) {
      existing.retention_days = req.retention_days
      existing.retain_messages = req.retain_messages
      existing.retain_response_body = req.retain_response_body
      existing.retain_debug_payloads = req.retain_debug_payloads
      existing.updated_at = new Date().toISOString()
      return existing
    }
    const item: DataRetentionConfig = {
      tenant_id: tenantId,
      retention_days: req.retention_days,
      retain_messages: req.retain_messages,
      retain_response_body: req.retain_response_body,
      retain_debug_payloads: req.retain_debug_payloads,
      created_at: new Date().toISOString(),
      updated_at: new Date().toISOString(),
    }
    retentionConfigs.push(item)
    return item
  },
  async deleteRetention(tenantId) {
    await delay()
    const idx = retentionConfigs.findIndex((r) => r.tenant_id === tenantId)
    if (idx !== -1) retentionConfigs.splice(idx, 1)
  },
}

// =============================================================================
// Guardrails Port
// =============================================================================

export const guardrailsPort: GuardrailsPort = {
  async getConfig() {
    await delay()
    const config = guardrailConfigs[0]
    if (!config) throw new Error("Not found")
    return config
  },
  async upsertConfig(req) {
    await delay()
    const existing = guardrailConfigs[0]
    if (existing) {
      if (req.enabled !== undefined) existing.enabled = req.enabled
      if (req.system_rules !== undefined) existing.system_rules = req.system_rules
      existing.updated_at = new Date().toISOString()
      return existing
    }
    const config: GuardrailConfig = {
      id: crypto.randomUUID(),
      tenant_id: defaultTenantId(),
      enabled: req.enabled ?? true,
      system_rules: req.system_rules ?? {
        prompt_injection: { enabled: true, action: "block" },
        jailbreak: { enabled: false, action: "warn" },
        pii_detection: { enabled: true, action: "redact" },
        secrets: { enabled: false, action: "block" },
        document_leakage: { enabled: false, action: "warn" },
      },
      created_at: new Date().toISOString(),
      updated_at: new Date().toISOString(),
    }
    guardrailConfigs.push(config)
    return config
  },
  async listRules() {
    await delay()
    return { data: [...guardrailRules], total: guardrailRules.length }
  },
  async createRule(req) {
    await delay()
    const rule: GuardrailRule = {
      id: crypto.randomUUID(),
      tenant_id: defaultTenantId(),
      name: req.name,
      type: req.type,
      config: req.config,
      priority: req.priority ?? guardrailRules.length + 1,
      enabled: true,
      action: req.action,
      created_at: new Date().toISOString(),
      updated_at: new Date().toISOString(),
    }
    guardrailRules.push(rule)
    return rule
  },
  async updateRule(ruleId, req) {
    await delay()
    const rule = guardrailRules.find((r) => r.id === ruleId)
    if (!rule) throw new Error("Not found")
    Object.assign(rule, req)
    rule.updated_at = new Date().toISOString()
    return rule
  },
  async deleteRule(ruleId) {
    await delay()
    const idx = guardrailRules.findIndex((r) => r.id === ruleId)
    if (idx !== -1) guardrailRules.splice(idx, 1)
  },
  async listViolations() {
    await delay()
    return { data: [...guardrailViolations], total: guardrailViolations.length }
  },
  async testCheck(messages) {
    await delay()
    const activeRules = guardrailRules.filter((r) => r.enabled)
    const violations = activeRules.flatMap((rule) => {
      if (rule.type === "blocked_terms") {
        const terms = (rule.config as { terms: string[] }).terms
        return messages.flatMap((msg) =>
          terms
            .filter((term) => msg.content.toLowerCase().includes(term.toLowerCase()))
            .map((term) => ({
              rule_id: rule.id,
              rule_name: rule.name,
              category: rule.type,
              action: rule.action,
              matched_pattern: term,
              matched_content: msg.content,
            })),
        )
      }
      if (rule.type === "custom_regex") {
        const pattern = (rule.config as { pattern: string }).pattern
        const regex = new RegExp(pattern)
        return messages.flatMap((msg) => {
          const match = msg.content.match(regex)
          return match
            ? [
                {
                  rule_id: rule.id,
                  rule_name: rule.name,
                  category: rule.type,
                  action: rule.action,
                  matched_pattern: pattern,
                  matched_content: match[0],
                },
              ]
            : []
        })
      }
      return []
    })
    const redactions = violations
      .filter((v) => v.action === "redact")
      .map((v, index) => ({
        message_index: index,
        matches: [v.matched_pattern],
        replacement: "[REDACTED]",
      }))
    const blocked = violations.some((v) => v.action === "block")
    return {
      passed: !blocked,
      blocked,
      violations,
      redactions,
    }
  },
}

// =============================================================================
// Webhooks Port
// =============================================================================

export const webhooksPort: WebhooksPort = {
  async list() {
    await delay()
    return { data: [...webhooks], total: webhooks.length }
  },
  async get(id) {
    await delay()
    const webhook = webhooks.find((w) => w.id === id)
    if (!webhook) throw new Error("Not found")
    return webhook
  },
  async create(req) {
    await delay()
    const webhook: WebhookConfig = {
      id: crypto.randomUUID(),
      tenant_id: defaultTenantId(),
      url: req.url,
      events: req.events,
      enabled: true,
      created_at: new Date().toISOString(),
      updated_at: new Date().toISOString(),
    }
    webhooks.push(webhook)
    return {
      id: webhook.id,
      tenant_id: webhook.tenant_id,
      url: webhook.url,
      secret: "whsec_" + crypto.randomUUID().replace(/-/g, ""),
      events: webhook.events,
      enabled: webhook.enabled,
      created_at: webhook.created_at,
    }
  },
  async update(id, req) {
    await delay()
    const webhook = webhooks.find((w) => w.id === id)
    if (!webhook) throw new Error("Not found")
    Object.assign(webhook, req)
    webhook.updated_at = new Date().toISOString()
    return webhook
  },
  async delete(id) {
    await delay()
    const idx = webhooks.findIndex((w) => w.id === id)
    if (idx !== -1) webhooks.splice(idx, 1)
  },
  async listEvents() {
    await delay()
    return ["request.completed", "request.failed", "spending.warning", "spending.exceeded", "key.health_degraded", "key.blacklisted"]
  },
  async listDeliveries(webhookId) {
    await delay()
    const filtered = webhookDeliveries.filter((d) => d.webhook_id === webhookId)
    return { data: filtered, total: filtered.length }
  },
  async test(id) {
    await delay()
    const webhook = webhooks.find((w) => w.id === id)
    if (!webhook) throw new Error("Not found")
    const delivery: WebhookDelivery = {
      id: crypto.randomUUID(),
      webhook_id: webhook.id,
      event_type: webhook.events[0] ?? "test.event",
      payload: JSON.stringify({ test: true }),
      status: "success",
      status_code: 200,
      attempts: 1,
      last_error: null,
      next_retry_at: null,
      created_at: new Date().toISOString(),
      completed_at: new Date().toISOString(),
    }
    webhookDeliveries.push(delivery)
    return { message: "Test webhook delivered successfully" }
  },
}

// =============================================================================
// API Keys Port
// =============================================================================

export const apiKeysPort: ApiKeysPort = {
  async list() {
    await delay()
    return { data: [...apiKeys], total: apiKeys.length }
  },
  async get(id) {
    await delay()
    const key = apiKeys.find((k) => k.id === id)
    if (!key) throw new Error("Not found")
    return key
  },
  async create(req) {
    await delay()
    const newId = crypto.randomUUID()
    const apiKey: ApiKey = {
      id: newId,
      key_prefix: `fr_${req.environment}_${newId.slice(0, 6)}`,
      tenant_id: defaultTenantId(),
      user_id: req.user_id ?? null,
      wallet_id: req.wallet_id || null,
      name: req.name,
      description: req.description,
      scopes: req.scopes,
      allowed_models: req.allowed_models,
      is_active: true,
      expires_at: req.expires_in ? new Date(Date.now() + req.expires_in * 1000).toISOString() : null,
      last_used_at: null,
      created_at: new Date().toISOString(),
    }
    apiKeys.push(apiKey)
    return {
      api_key: apiKey,
      secret_key: `sk-${newId}`,
      message: "Store this key securely. It will not be shown again.",
    }
  },
  async update(id, req) {
    await delay()
    const key = apiKeys.find((k) => k.id === id)
    if (!key) throw new Error("Not found")
    Object.assign(key, req)
    if (req.wallet_id !== undefined) key.wallet_id = req.wallet_id || null
    return key
  },
  async revoke(id) {
    await delay()
    const key = apiKeys.find((k) => k.id === id)
    if (!key) throw new Error("Not found")
    key.is_active = false
  },
  async delete(id) {
    await delay()
    const idx = apiKeys.findIndex((k) => k.id === id)
    if (idx !== -1) apiKeys.splice(idx, 1)
  },
}

// =============================================================================
// Users Port
// =============================================================================

export const usersPort: UsersPort = {
  async list() {
    await delay()
    return { data: [...users], total: users.length }
  },
  async get(id) {
    await delay()
    const user = users.find((u) => u.id === id)
    if (!user) throw new Error("Not found")
    return user
  },
  async create(req) {
    await delay()
    const user: User = {
      id: crypto.randomUUID(),
      tenant_id: TENANT,
      name: req.name,
      email: req.email,
      picture: null,
      is_active: true,
      scopes: req.scopes,
      oauth_provider: "email",
      created_at: new Date().toISOString(),
    }
    users.push(user)
    return user
  },
  async update(id, req) {
    await delay()
    const user = users.find((u) => u.id === id)
    if (!user) throw new Error("Not found")
    if (req.name !== undefined) user.name = req.name
    if (req.scopes !== undefined) user.scopes = req.scopes
    if (req.status !== undefined) user.is_active = req.status === "ACTIVE"
    return user
  },
  async activate(id) {
    await delay()
    const user = users.find((u) => u.id === id)
    if (!user) throw new Error("Not found")
    user.is_active = true
  },
  async suspend(id, _reason) {
    await delay()
    const user = users.find((u) => u.id === id)
    if (!user) throw new Error("Not found")
    user.is_active = false
  },
  async delete(id) {
    await delay()
    const idx = users.findIndex((u) => u.id === id)
    if (idx !== -1) users.splice(idx, 1)
  },
}

// =============================================================================
// Roles Port
// =============================================================================

export const rolesPort: RolesPort = {
  async list() {
    await delay()
    return { data: [...roles], total: roles.length }
  },
  async get(id) {
    await delay()
    const role = roles.find((r) => r.id === id)
    if (!role) throw new Error("Not found")
    return role
  },
  async create(req) {
    await delay()
    const role: Role = {
      id: crypto.randomUUID(),
      tenant_id: defaultTenantId(),
      name: req.name,
      description: req.description,
      scopes: req.scopes,
      created_at: new Date().toISOString(),
      updated_at: new Date().toISOString(),
    }
    roles.push(role)
    return role
  },
  async update(id, req) {
    await delay()
    const role = roles.find((r) => r.id === id)
    if (!role) throw new Error("Not found")
    Object.assign(role, req)
    role.updated_at = new Date().toISOString()
    return role
  },
  async delete(id) {
    await delay()
    const idx = roles.findIndex((r) => r.id === id)
    if (idx !== -1) roles.splice(idx, 1)
    for (let i = roleAssignments.length - 1; i >= 0; i--) {
      if (roleAssignments[i].role_id === id) roleAssignments.splice(i, 1)
    }
  },
  async assign(roleId, req) {
    await delay()
    const role = roles.find((r) => r.id === roleId)
    if (!role) throw new Error("Not found")
    const exists = roleAssignments.some((a) => a.role_id === roleId && a.user_id === req.user_id)
    if (!exists) roleAssignments.push({ role_id: roleId, user_id: req.user_id })
  },
  async unassign(roleId, userId) {
    await delay()
    const idx = roleAssignments.findIndex((a) => a.role_id === roleId && a.user_id === userId)
    if (idx !== -1) roleAssignments.splice(idx, 1)
  },
  async getUserRoles(userId) {
    await delay()
    const user = users.find((u) => u.id === userId)
    if (!user) throw new Error("Not found")
    const assignedRoleIds = roleAssignments.filter((a) => a.user_id === userId).map((a) => a.role_id)
    const userRoles = roles.filter((r) => assignedRoleIds.includes(r.id))
    const roleScopes = userRoles.flatMap((r) => r.scopes)
    const effectiveScopes = Array.from(new Set([...user.scopes, ...roleScopes]))
    return {
      user_id: userId,
      roles: userRoles,
      direct_scopes: user.scopes,
      effective_scopes: effectiveScopes,
    }
  },
}

// =============================================================================
// Invitations Port
// =============================================================================

export const invitationsPort: InvitationsPort = {
  async list() {
    await delay()
    return { data: [...invitations], total: invitations.length }
  },
  async listPending() {
    await delay()
    const pending = invitations.filter((i) => i.status === "PENDING")
    return { data: pending, total: pending.length }
  },
  async get(id) {
    await delay()
    const invitation = invitations.find((i) => i.id === id)
    if (!invitation) throw new Error("Not found")
    return invitation
  },
  async create(req) {
    await delay()
    const invitation: Invitation = {
      id: crypto.randomUUID(),
      tenant_id: defaultTenantId(),
      email: req.email,
      status: "PENDING",
      scopes: req.scopes,
      role_id: req.role_id ?? null,
      expires_at: new Date(Date.now() + (req.expires_in ?? 7 * 86400) * 1000).toISOString(),
      accepted_at: null,
      created_at: new Date().toISOString(),
    }
    invitations.push(invitation)
    return invitation
  },
  async delete(id) {
    await delay()
    const idx = invitations.findIndex((i) => i.id === id)
    if (idx !== -1) invitations.splice(idx, 1)
  },
  async revoke(id) {
    await delay()
    const invitation = invitations.find((i) => i.id === id)
    if (!invitation) throw new Error("Not found")
    invitation.status = "REVOKED"
  },
  async validateToken(token) {
    await delay()
    const invitation = invitations.find((i) => i.id === token)
    if (!invitation) throw new Error("Not found")
    const expired = new Date(invitation.expires_at).getTime() < Date.now()
    const valid = invitation.status === "PENDING" && !expired
    return {
      valid,
      invitation,
      message: valid ? "Invitation is valid" : "Invitation is no longer valid",
    }
  },
  async getByToken(token) {
    await delay()
    const invitation = invitations.find((i) => i.id === token)
    if (!invitation) throw new Error("Not found")
    return invitation
  },
}

// =============================================================================
// Combined Mock API
// =============================================================================

export const mockApi: ApiPort = {
  providers: providerPort,
  models: modelPort,
  mappings: mappingPort,
  modelFallbacks: modelFallbackPort,
  providerKeys: providerKeyPort,
  gatewayConfig: gatewayConfigPort,
  billing: billingPort,
  wallets: walletsPort,
  usage: usagePort,
  guardrails: guardrailsPort,
  webhooks: webhooksPort,
  apiKeys: apiKeysPort,
  users: usersPort,
  roles: rolesPort,
  invitations: invitationsPort,
}
