import type { APIRequestContext, Page, Response } from "@playwright/test"
import { test, expect, fx, login, adminApi, uniq } from "./helpers"

// WS-09 console: dashboard and usage pages.
// Other workers produce usage concurrently, so dashboard numbers are compared
// with the exact API responses the page itself received (not with globals),
// and the usage page is exercised through filters scoped to our own models.

test.describe.configure({ timeout: 120_000 })

const cleanup: (() => Promise<unknown>)[] = []
test.afterEach(async () => {
  for (const fn of cleanup.splice(0).reverse()) await fn().catch(() => undefined)
})

async function retry502<T extends { status(): number }>(fn: () => Promise<T>): Promise<T> {
  let r = await fn()
  for (let i = 0; i < 5 && (r.status() === 502 || r.status() === 429); i++) {
    await new Promise((res) => setTimeout(res, 1_000 * (i + 1)))
    r = await fn()
  }
  return r
}

type Env = { seg: string; provider: string; ok: string; err: string; secret: string }

/** Own provider + two models (fake-ok, fake-500) + gateway service account; drives 2 ok + 1 failing call. */
async function seedUsage(request: APIRequestContext): Promise<Env> {
  const api = adminApi(request)
  const seg = uniq("ws09ui")
  let r = await api.post("/providers", { name: seg, protocol: "openai", base_url: `${fx.urls.fakellm}/ws09/${seg}/openai/v1`, streaming: true })
  expect(r.status()).toBe(201)
  const provider = (await r.json()).id as string
  cleanup.push(() => api.delete(`/providers/${provider}`))
  r = await api.post("/provider-keys", { provider_id: provider, name: `${seg}-key`, key_type: "api_key", token: `sk-ws09-${seg}` })
  expect(r.status()).toBe(201)
  const key = (await r.json()).id as string
  cleanup.push(() => api.delete(`/provider-keys/${key}`))
  const names: Record<string, string> = {}
  for (const [kind, ext] of [["ok", "fake-ok"], ["err", "fake-500"]]) {
    const name = `${seg}-${kind}`
    r = await api.post("/models", { name, family: "ws09", description: "ws09 ui" })
    expect(r.status()).toBe(201)
    const id = (await r.json()).id as string
    cleanup.push(() => api.delete(`/models/${id}`))
    r = await api.post("/mappings", { model_id: id, provider_id: provider, external_id: ext, input_price: 3, output_price: 7 })
    expect(r.status()).toBe(201)
    names[kind] = name
  }
  r = await retry502(() => api.post("/service-accounts", { name: uniq("ws09-ui-gw"), permissions: ["freerouter:gateway:invoke"] }))
  expect([200, 201]).toContain(r.status())
  const sa = await r.json()
  cleanup.push(() => retry502(() => api.delete(`/service-accounts/${sa.id}`)))

  const chat = async (model: string, want: number) => {
    const headers = { Authorization: `Bearer ${sa.secret}` }
    const data = { model, messages: [{ role: "user", content: uniq("ws09 ui") }] }
    let res = await request.post(`${fx.urls.gateway}/chat/completions`, { headers, data })
    for (let i = 0; i < 5 && res.status() === 401; i++) {
      await new Promise((ok) => setTimeout(ok, 1_000))
      res = await request.post(`${fx.urls.gateway}/chat/completions`, { headers, data })
    }
    expect(res.status(), await res.text()).toBe(want)
  }
  await chat(names.ok, 200)
  await chat(names.ok, 200)
  await chat(names.err, 502)
  await expect.poll(async () => {
    const res = await api.get(`/usage?provider=${provider}`)
    return (await res.json()).page.total
  }, { timeout: 15_000 }).toBe(3)
  return { seg, provider, ok: names.ok, err: names.err, secret: sa.secret }
}

async function loginAndWait(page: Page, persona: typeof fx.personas.admin) {
  for (let i = 0; i < 6; i++) {
    await login(page, persona)
    try {
      await expect(page).not.toHaveURL(/\/login/, { timeout: 10_000 })
      return
    } catch (e) {
      if (i === 5) throw e
      await page.waitForTimeout(5_000)
    }
  }
}

const isApi = (path: string) => (r: Response) => {
  const u = new URL(r.url())
  return u.pathname === `/api/v1${path}` && r.request().method() === "GET"
}

function formatNumber(n: number): string {
  if (n >= 1_000_000) return `${(n / 1_000_000).toFixed(1)}M`
  if (n >= 1_000) return `${(n / 1_000).toFixed(1)}K`
  return n.toString()
}

function metricCard(page: Page, label: string) {
  return page.locator('[data-slot="card"]').filter({ has: page.locator('[data-slot="card-description"]', { hasText: new RegExp(`^${label}$`, "i") }) })
}

test("dashboard KPIs and recent requests match the API responses the page received", async ({ page, request }) => {
  const env = await seedUsage(request)
  await loginAndWait(page, fx.personas.admin)
  await page.goto("/usage") // leave the dashboard so the next visit is a fresh load
  await expect(page.getByRole("heading", { name: "Usage" })).toBeVisible()
  const summaryRes = page.waitForResponse(isApi("/usage/summary"))
  const logsRes = page.waitForResponse((r) => isApi("/usage")(r) && new URL(r.url()).searchParams.get("limit") === "5")
  await page.goto("/")
  const summary = await (await summaryRes).json()
  const logs = await (await logsRes).json()
  // the page asks for the summary without a time window
  expect(new URL((await summaryRes).url()).search).toBe("")

  const s = summary.summary
  await expect(metricCard(page, "Requests").locator("p").first()).toHaveText(formatNumber(s.total_requests))
  const rate = s.total_requests > 0 ? (s.error_count / s.total_requests * 100).toFixed(1) : "0.0"
  await expect(metricCard(page, "Requests")).toContainText(`${rate}% error rate`)
  await expect(metricCard(page, "Tokens").locator("p").first()).toHaveText(formatNumber(s.total_tokens))
  await expect(metricCard(page, "Total Cost").locator("p").first()).toHaveText(`$${s.total_cost.toFixed(2)}`)

  // our own models are part of the summary the dashboard used
  const okRow = summary.by_model.find((m: any) => m.model === env.ok)
  expect(okRow, "own model in by_model").toBeTruthy()
  expect(okRow.total_requests).toBe(2)
  expect(okRow.total_tokens).toBe(18)

  // recent requests table mirrors /usage?limit=5 (used_model, tokens, cost, status)
  const rows = page.getByRole("table").last().locator("tbody tr")
  await expect(rows).toHaveCount(Math.max(logs.items.length, 1))
  for (const [i, l] of logs.items.entries()) {
    const row = rows.nth(i)
    await expect(row).toContainText(l.used_model)
    await expect(row.locator("td").nth(1)).toHaveText(String(l.total_tokens))
    await expect(row.locator("td").nth(2)).toHaveText(`$${l.total_cost.toFixed(4)}`)
    await expect(row.locator("td").nth(3)).toHaveText(l.has_error ? /ERROR/ : /OK/)
  }
})

test("dashboard and usage page empty states", async ({ page }) => {
  const emptySummary = {
    summary: { total_requests: 0, total_tokens: 0, prompt_tokens: 0, completion_tokens: 0, total_cost: 0, error_count: 0 },
    by_model: [], period_start: new Date().toISOString(), period_end: new Date().toISOString(),
  }
  const emptyPage = (limit: number) => ({ items: [], page: { total: 0, limit, offset: 0 } })
  // Simulate a fresh install: the usage endpoints answer empty (no real data is deleted).
  await page.route(/\/api\/v1\/usage\/summary(\?.*)?$/, (r) => r.fulfill({ json: emptySummary }))
  await page.route(/\/api\/v1\/usage(\?.*)?$/, (r) => r.fulfill({ json: emptyPage(Number(new URL(r.request().url()).searchParams.get("limit") ?? 20)) }))
  await loginAndWait(page, fx.personas.admin)
  await page.goto("/")
  await expect(page.getByText("No usage data yet")).toBeVisible()
  await expect(page.getByText("No requests yet")).toBeVisible()
  await expect(metricCard(page, "Requests").locator("p").first()).toHaveText("0")
  await expect(metricCard(page, "Requests")).toContainText("0.0% error rate")
  await expect(metricCard(page, "Total Cost").locator("p").first()).toHaveText("$0.00")

  await page.goto("/usage")
  await expect(page.getByText("No logs found")).toBeVisible()
  await expect(page.getByText("No cost data")).toBeVisible()
  await expect(page.getByText("No request data")).toBeVisible()
  await expect(page.getByText("0 total requests")).toBeVisible()
  await expect(metricCard(page, "Error Rate").locator("p").first()).toHaveText("0%")
  await expect(metricCard(page, "Avg Latency").locator("p").first()).toHaveText("0ms")
})

test("usage page: model and status filters, detail dialog, clear", async ({ page, request }) => {
  const env = await seedUsage(request)
  await loginAndWait(page, fx.personas.admin)
  await page.goto("/usage")
  await expect(page.getByRole("heading", { name: "Usage" })).toBeVisible()
  const table = page.getByRole("table")
  const body = table.locator("tbody tr")
  const filter = page.getByPlaceholder("Filter by model...")

  // filter by exact model name → our 2 successful calls
  let listed = page.waitForResponse((r) => isApi("/usage")(r) && new URL(r.url()).searchParams.get("model") === env.ok)
  await filter.fill(env.ok)
  await listed
  await expect(page.getByText("2 total requests")).toBeVisible()
  await expect(body).toHaveCount(2)
  for (const i of [0, 1]) {
    const row = body.nth(i)
    await expect(row).toContainText("fake-ok")
    await expect(row).toContainText(`requested: ${env.ok}`)
    await expect(row).toContainText(env.seg) // provider name resolved from id
    await expect(row.locator("td").nth(2)).toHaveText("9")
    await expect(row.locator("td").nth(3)).toHaveText("$0.0000") // 43e-6 rounds to 4 decimals
    await expect(row.locator("td").nth(5)).toHaveText("200")
  }

  // status filter on our failing model
  listed = page.waitForResponse((r) => isApi("/usage")(r) && new URL(r.url()).searchParams.get("model") === env.err)
  await filter.fill(env.err)
  await listed
  await expect(body).toHaveCount(1)
  await page.getByRole("combobox").click()
  listed = page.waitForResponse((r) => isApi("/usage")(r) && new URL(r.url()).searchParams.get("has_error") === "false")
  await page.getByRole("option", { name: "Success" }).click()
  await listed
  await expect(page.getByText("No logs found")).toBeVisible()
  await expect(page.getByText("0 total requests")).toBeVisible()
  await page.getByRole("combobox").click()
  listed = page.waitForResponse((r) => isApi("/usage")(r) && new URL(r.url()).searchParams.get("has_error") === "true")
  await page.getByRole("option", { name: "Errors" }).click()
  await listed
  await expect(body).toHaveCount(1)
  await expect(body.first().locator("td").nth(5)).toHaveText("500")

  // detail of the failed request
  const detail = page.waitForResponse((r) => /\/api\/v1\/usage\/[0-9a-f-]{36}$/.test(new URL(r.url()).pathname))
  await body.first().click()
  const log = await (await detail).json()
  const dialog = page.getByRole("dialog")
  await expect(dialog.getByText("Request Detail")).toBeVisible()
  await expect(dialog).toContainText(log.id)
  await expect(dialog).toContainText("500 (error)")
  await expect(dialog).toContainText(env.err)
  await expect(dialog).toContainText(env.seg)
  await expect(dialog).toContainText("upstream returned 500")
  await expect(dialog).not.toContainText(`sk-ws09-${env.seg}`)
  await page.keyboard.press("Escape")
  await expect(dialog).toBeHidden()

  // detail of a success: full-precision cost
  listed = page.waitForResponse((r) => isApi("/usage")(r) && new URL(r.url()).searchParams.get("model") === env.ok)
  await filter.fill(env.ok)
  await listed
  await page.getByRole("combobox").click()
  await page.getByRole("option", { name: "All status" }).click()
  await expect(body).toHaveCount(2)
  await body.first().click()
  await expect(dialog).toContainText("200 (ok)")
  await expect(dialog).toContainText("$0.000015") // 5 × $3 / 1M
  await expect(dialog).toContainText("$0.000028") // 4 × $7 / 1M
  await expect(dialog).toContainText("$0.000043")
  await expect(dialog).not.toContainText("Error")
  await page.keyboard.press("Escape")

  // unknown model → empty state; Clear resets filters
  await filter.fill(`${env.seg}-does-not-exist`)
  await expect(page.getByText("No logs found")).toBeVisible()
  await page.getByRole("button", { name: "Clear" }).click()
  await expect(filter).toHaveValue("")
  await expect(page.getByRole("button", { name: "Clear" })).toBeHidden()
  await expect(body.first()).not.toContainText("No logs found")
})

test("usage page KPIs reflect /usage/summary", async ({ page }) => {
  await loginAndWait(page, fx.personas.admin)
  await expect(page.getByRole("heading", { name: "Dashboard" })).toBeVisible()
  const summaryRes = page.waitForResponse(isApi("/usage/summary"))
  await page.goto("/usage")
  const s = (await (await summaryRes).json()).summary
  await expect(metricCard(page, "Total Requests").locator("p").first()).toHaveText(s.total_requests.toLocaleString("en-US"))
  await expect(metricCard(page, "Total Cost").locator("p").first()).toHaveText(`$${s.total_cost.toFixed(4)}`)
  const rate = s.total_requests > 0 ? ((s.error_count / s.total_requests) * 100).toFixed(1) : "0"
  await expect(metricCard(page, "Error Rate").locator("p").first()).toHaveText(`${rate}%`)
  await expect(metricCard(page, "Error Rate")).toContainText(`${s.error_count} errors`)
})
