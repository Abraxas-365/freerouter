// Deterministic console API stubs for WS-10 (snapshots, empty/loading/error
// states). Shared e2e data changes constantly (other workers), so visual
// baselines are taken against these canned responses, not the live server.
import type { Page, Route } from "@playwright/test"

const T = "2026-01-15T10:00:00Z"
export const FIXED_NOW = new Date("2026-01-15T12:00:00Z")

const page = <X>(items: X[]) => ({ items, page: { total: items.length, limit: 100, offset: 0 } })

const providers = [
  { id: "p1", name: "OpenAI", protocol: "openai", description: "OpenAI platform", website: "https://openai.com", base_url: "https://api.openai.com/v1", status: "active", streaming: true, created_at: T, updated_at: T },
  { id: "p2", name: "Anthropic", protocol: "anthropic", description: "Claude models", website: "https://anthropic.com", base_url: "https://api.anthropic.com", status: "active", streaming: true, created_at: T, updated_at: T },
  { id: "p3", name: "Legacy", protocol: "openai", description: "", base_url: "https://legacy.example", status: "inactive", streaming: false, created_at: T, updated_at: T },
]
const models = [
  { id: "m1", name: "gpt-4o", description: "Flagship", family: "gpt", stability: "stable", status: "active", free: false, released_at: T, created_at: T, updated_at: T },
  { id: "m2", name: "claude-sonnet", description: "Balanced", family: "claude", stability: "stable", status: "active", free: false, released_at: T, created_at: T, updated_at: T },
]
const mappings = [
  { id: "mp1", model_id: "m1", provider_id: "p1", external_id: "gpt-4o", input_price: 2.5, output_price: 10, context_size: 128000, max_output: 16384, streaming: true, vision: true, reasoning: false, tools: true, json_output: true, audio: false, speech: false, moderation: false, rerank: false, stability: "stable", status: "active", created_at: T, updated_at: T },
  { id: "mp2", model_id: "m2", provider_id: "p2", external_id: "claude-sonnet", input_price: 3, output_price: 15, context_size: 200000, max_output: 8192, streaming: true, vision: true, reasoning: true, tools: true, json_output: false, audio: false, speech: false, moderation: false, rerank: false, stability: "stable", status: "active", created_at: T, updated_at: T },
]
const keys = [
  { id: "k1", provider_id: "p1", key_type: "api_key", token_masked: "sk-a*****1234", name: "prod-openai", description: "Production", status: "active", created_at: T, updated_at: T },
  { id: "k2", provider_id: "p2", key_type: "api_key", token_masked: "sk-a*****5678", name: "prod-anthropic", description: "", status: "inactive", created_at: T, updated_at: T },
]
const rateLimits = [
  { id: "r1", name: "default", subject_id: "default", rpm: 60, max_concurrent: 10, created_at: T, updated_at: T },
  { id: "r2", name: "batch-job", subject_id: "u1", rpm: 600, max_concurrent: 50, created_at: T, updated_at: T },
]
const logs = [
  { id: "l1", key_id: "k1", requested_model: "gpt-4o", used_model: "gpt-4o", provider_id: "p1", mapping_id: "mp1", prompt_tokens: 120, completion_tokens: 80, total_tokens: 200, cached_tokens: 0, input_cost: 0.0003, output_cost: 0.0008, total_cost: 0.0011, duration_ms: 420, streamed: false, status_code: 200, finish_reason: "stop", has_error: false, is_fallback: false, created_at: "2026-01-15T11:55:00Z" },
  { id: "l2", key_id: "k2", requested_model: "claude-sonnet", used_model: "claude-sonnet", provider_id: "p2", mapping_id: "mp2", prompt_tokens: 50, completion_tokens: 0, total_tokens: 50, cached_tokens: 0, input_cost: 0, output_cost: 0, total_cost: 0, duration_ms: 1200, streamed: true, status_code: 502, finish_reason: "", has_error: true, error_message: "upstream error", is_fallback: false, created_at: "2026-01-15T11:00:00Z" },
]
const summary = {
  summary: { total_requests: 2, total_tokens: 250, prompt_tokens: 170, completion_tokens: 80, total_cost: 0.0011, error_count: 1 },
  by_model: [
    { model: "gpt-4o", total_requests: 1, total_tokens: 200, prompt_tokens: 120, completion_tokens: 80, total_cost: 0.0011 },
    { model: "claude-sonnet", total_requests: 1, total_tokens: 50, prompt_tokens: 50, completion_tokens: 0, total_cost: 0 },
  ],
  period_start: "2025-12-16T00:00:00Z", period_end: "2026-01-15T12:00:00Z",
}
const sys = (enabled: boolean, action: string) => ({ enabled, action })
const grConfig = { id: "g1", enabled: true, created_at: T, updated_at: T, system_rules: {
  prompt_injection: sys(true, "block"), jailbreak: sys(true, "block"), pii_detection: sys(false, "redact"), secrets: sys(true, "block"), document_leakage: sys(false, "warn"),
} }
const grRules = [
  { id: "gr1", name: "no-competitors", type: "blocked_terms", config: { terms: ["acme", "globex"], match_type: "contains", case_sensitive: false }, priority: 10, enabled: true, action: "warn", created_at: T, updated_at: T },
]
const violations = [
  { id: "v1", rule_id: "gr1", rule_name: "no-competitors", category: "custom", action_taken: "warned", matched_pattern: "acme", model: "gpt-4o", created_at: "2026-01-15T11:30:00Z" },
]
const webhooks = [
  { id: "w1", url: "https://hooks.example.com/freerouter", events: ["request.completed", "request.failed"], enabled: true, created_at: T, updated_at: T },
]
const sas = [
  { id: "s1", name: "ci-gateway", application_id: "a1", resource_id: "res", permissions: ["freerouter:gateway:invoke"], expires_in: "" },
]
const apps = [{ id: "a1", name: "FreeRouter", active: true }]
const users = [
  { id: "u1", email: "alice@example.com", name: "Alice", active: true },
  { id: "u2", email: "bob@example.com", name: "Bob", active: false },
]
const roles = [{ id: "ro1", name: "viewer", permissions: ["freerouter:usage:read", "freerouter:providers:read"] }]
const assignments = [{ user_id: "u1", role_id: "ro1", organization_id: "o1" }]

/** GET responses by path (pathname after /api/v1). */
export function stubBody(path: string, empty = false): unknown {
  const E = empty
  const table: [RegExp, () => unknown][] = [
    [/^\/usage\/summary$/, () => E ? { summary: { total_requests: 0, total_tokens: 0, prompt_tokens: 0, completion_tokens: 0, total_cost: 0, error_count: 0 }, by_model: [], period_start: T, period_end: T } : summary],
    [/^\/usage\/[^/]+$/, () => logs[0]],
    [/^\/usage$/, () => page(E ? [] : logs)],
    [/^\/providers$/, () => page(E ? [] : providers)],
    [/^\/models$/, () => page(E ? [] : models)],
    [/^\/mappings$/, () => page(E ? [] : mappings)],
    [/^\/provider-keys$/, () => page(E ? [] : keys)],
    [/^\/rate-limits$/, () => page(E ? [] : rateLimits)],
    [/^\/guardrails\/config$/, () => grConfig],
    [/^\/guardrails\/rules$/, () => E ? [] : grRules],
    [/^\/guardrails\/violations$/, () => page(E ? [] : violations)],
    [/^\/webhooks\/events$/, () => ({ events: ["request.completed", "request.failed", "key.health_degraded", "key.blacklisted"] })],
    [/^\/webhooks\/[^/]+\/deliveries$/, () => page([])],
    [/^\/webhooks$/, () => page(E ? [] : webhooks)],
    [/^\/service-accounts\/applications$/, () => apps],
    [/^\/service-accounts$/, () => E ? [] : sas],
    [/^\/access\/users$/, () => E ? [] : users],
    [/^\/access\/roles$/, () => E ? [] : roles],
    [/^\/access\/role-assignments$/, () => E ? [] : assignments],
  ]
  for (const [re, fn] of table) if (re.test(path)) return fn()
  return undefined
}

export const API_RE = /localhost:23000\/(api\/v1|v1)\//

/** Serve every console API GET from the canned data. */
export async function stubApi(p: Page, opts: { empty?: boolean; delayMs?: number } = {}) {
  await p.route(API_RE, async (route: Route) => {
    const url = new URL(route.request().url())
    if (opts.delayMs) await new Promise((r) => setTimeout(r, opts.delayMs))
    if (url.pathname === "/v1/models") {
      return route.fulfill({ json: { object: "list", data: [] } }).catch(() => {})
    }
    const body = stubBody(url.pathname.replace(/^\/api\/v1/, ""), opts.empty)
    if (route.request().method() !== "GET" || body === undefined) {
      return route.fulfill({ status: 404, json: { code: "NOT_FOUND", message: "ws10 stub: not stubbed", status_code: 404 } }).catch(() => {})
    }
    return route.fulfill({ json: body }).catch(() => {})
  })
}
