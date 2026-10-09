import type { APIRequestContext, Page } from "@playwright/test"
import { test, expect, fx, adminApi, uniq } from "./helpers"
import { ws08Login, row, rowAction } from "./ws08-helpers"

// WS-08 console: webhooks page. Webhooks point at the e2e sink under
// /hook/ws08-ui-*; deliveries are matched by exact sink path.

test.describe.configure({ timeout: 90_000 })

const cleanup: (() => Promise<unknown>)[] = []
test.afterEach(async () => {
  for (const fn of cleanup.splice(0).reverse()) await fn().catch(() => undefined)
})

async function hooksByUrl(request: APIRequestContext, url: string) {
  const r = await adminApi(request).get("/webhooks?limit=100")
  expect(r.status()).toBe(200)
  return ((await r.json()).items as any[]).filter((w) => w.url === url)
}

function trackUrl(request: APIRequestContext, url: string) {
  cleanup.push(async () => {
    for (const w of await hooksByUrl(request, url)) await adminApi(request).delete(`/webhooks/${w.id}`)
  })
}

async function gotoHooks(page: Page) {
  await ws08Login(page)
  await page.goto("/webhooks")
  await expect(page.getByRole("heading", { name: "Webhooks" })).toBeVisible()
  await expect(page.getByText("Endpoints", { exact: true })).toBeVisible()
}

/** Sink deliveries on exactly this path, optionally only of one event type. */
async function sinkCount(request: APIRequestContext, path: string, event?: string) {
  const r = await request.get(`${fx.urls.webhook_sink}/_deliveries`)
  return ((await r.json()) as any[]).filter((d) => d.path === path && (!event || d.headers["X-Webhook-Event"] === event)).length
}

test.describe("WS-08 webhooks page", () => {
  test("create shows the secret once; event chips; edit; delete", async ({ page, request }) => {
    const path = `/hook/${uniq("ws08-ui-hook")}`
    const url = fx.urls.webhook_sink + path
    trackUrl(request, url)
    await gotoHooks(page)

    await page.getByRole("button", { name: "Add Webhook" }).click()
    const dlg = page.getByRole("dialog")
    const create = dlg.getByRole("button", { name: "Create" })
    await expect(create).toBeDisabled()
    await dlg.getByPlaceholder("https://example.com/webhooks").fill(url)
    await expect(create).toBeDisabled() // no events yet
    for (const ev of ["request.completed", "request.failed", "key.health_degraded", "key.blacklisted"]) {
      await expect(dlg.getByText(ev, { exact: true })).toBeVisible()
    }
    await dlg.getByText("request.failed", { exact: true }).click()
    await dlg.getByText("key.blacklisted", { exact: true }).click()
    await dlg.getByText("key.blacklisted", { exact: true }).click() // toggle off again
    await expect(create).toBeEnabled()
    await create.click()

    await expect(page.getByText("Webhook created")).toBeVisible()
    await expect(page.getByText("Copy this secret now. It won't be shown again.")).toBeVisible()
    const secret = (await page.locator("code").filter({ hasText: /^whsec_/ }).textContent())!.trim()
    expect(secret).toMatch(/^whsec_[0-9a-f]{48}$/)
    const [hook] = await hooksByUrl(request, url)
    expect(hook.events).toEqual(["request.failed"])
    expect(JSON.stringify(hook)).not.toContain(secret)
    await expect(row(page, url)).toContainText("request.failed")
    await expect(row(page, url)).not.toContainText("key.blacklisted")

    await page.getByRole("button", { name: "Dismiss" }).click()
    await expect(page.getByText(secret)).toHaveCount(0)
    await page.reload()
    await expect(row(page, url)).toBeVisible()
    await expect(page.getByText(/^whsec_/)).toHaveCount(0)

    // Edit events.
    await rowAction(page, url, "Edit")
    const ed = page.getByRole("dialog")
    await expect(ed.getByPlaceholder("https://example.com/webhooks")).toHaveValue(url)
    await ed.getByText("request.completed", { exact: true }).click()
    await ed.getByRole("button", { name: "Update" }).click()
    await expect(page.getByText("Webhook updated")).toBeVisible()
    await expect(row(page, url)).toContainText("request.completed")
    expect((await hooksByUrl(request, url))[0].events.sort()).toEqual(["request.completed", "request.failed"])

    // Disable / enable from the menu.
    await rowAction(page, url, "Disable")
    await expect(row(page, url)).toContainText(/inactive/i)
    expect((await hooksByUrl(request, url))[0].enabled).toBe(false)
    await rowAction(page, url, "Enable")
    await expect(row(page, url)).toContainText(/active/i)

    // Delete: cancel, then confirm.
    await rowAction(page, url, "Delete")
    const cd = page.getByRole("dialog")
    await expect(cd.getByText("This webhook and all its delivery history will be permanently deleted.")).toBeVisible()
    await cd.getByRole("button", { name: "Cancel" }).click()
    await expect(row(page, url)).toBeVisible()
    await rowAction(page, url, "Delete")
    await page.getByRole("dialog").getByRole("button", { name: "Confirm" }).click()
    await expect(page.getByText("Webhook deleted")).toBeVisible()
    await expect(row(page, url)).toHaveCount(0)
    expect(await hooksByUrl(request, url)).toHaveLength(0)
  })

  test("deliveries panel lists a real delivery", async ({ page, request }) => {
    const api = adminApi(request)
    const path = `/hook/${uniq("ws08-ui-deliv")}`
    const url = fx.urls.webhook_sink + path
    trackUrl(request, url)
    const c = await api.post("/webhooks", { url, events: ["request.failed"] })
    expect(c.status()).toBe(201)
    const id = (await c.json()).id
    // Produce request.failed through an own provider mapped to fakellm's fake-400.
    const seg = uniq("ws08ui")
    let r = await api.post("/providers", { name: seg, protocol: "openai", base_url: `${fx.urls.fakellm}/ws08/${seg}/openai/v1`, streaming: true })
    expect(r.status()).toBe(201)
    const pid = (await r.json()).id
    cleanup.push(() => api.delete(`/providers/${pid}`))
    expect((await api.post("/provider-keys", { provider_id: pid, name: `${seg}-k`, key_type: "api_key", token: `sk-${seg}` })).status()).toBe(201)
    r = await api.post("/models", { name: seg, family: "ws08", description: "ws08 ui" })
    expect(r.status()).toBe(201)
    const mid = (await r.json()).id
    cleanup.push(() => api.delete(`/models/${mid}`))
    expect((await api.post("/mappings", { model_id: mid, provider_id: pid, external_id: "fake-400", input_price: 1, output_price: 1 })).status()).toBe(201)
    const g = await request.post(`${fx.urls.gateway}/chat/completions`, {
      headers: { Authorization: `Bearer ${fx.personas.gw_key.secret}` },
      data: { model: seg, messages: [{ role: "user", content: uniq("ws08 ui") }] },
    })
    expect(g.status()).toBeGreaterThanOrEqual(400)
    await expect.poll(async () => (await (await api.get(`/webhooks/${id}/deliveries`)).json()).items.some((d: any) => d.payload.includes(seg) && d.status === "success"), { timeout: 15_000 }).toBe(true)

    await gotoHooks(page)
    await row(page, url).locator("td").first().click()
    await expect(page.getByText("Deliveries", { exact: true })).toBeVisible()
    const drow = page.getByRole("row").filter({ hasText: "request.failed" }).filter({ hasText: "success" })
    await expect(drow.first()).toBeVisible()
    await expect(drow.first()).toContainText("200")
    await expect(drow.first()).toContainText("just now")
    await page.getByRole("button", { name: "Close" }).click()
    await expect(page.getByText("Deliveries", { exact: true })).toHaveCount(0)
  })

  // FINDING WS08-12 (UI side): "Test" says dispatched but nothing arrives.
  test("test button dispatches an event to the endpoint", async ({ page, request }) => {
    const path = `/hook/${uniq("ws08-ui-test")}`
    const url = fx.urls.webhook_sink + path
    trackUrl(request, url)
    expect((await adminApi(request).post("/webhooks", { url, events: ["request.failed"] })).status()).toBe(201)
    await gotoHooks(page)
    await rowAction(page, url, "Test")
    await expect(page.getByText("test event dispatched")).toBeVisible()
    // Other workers' gateway failures also fire request.failed here; count only the test event.
    await expect.poll(() => sinkCount(request, path, "webhook.test"), { timeout: 8_000 }).toBe(1)
  })

  // FINDING WS08-15 (UI side): server rejection is not shown.
  test("invalid URL shows the server validation error", async ({ page, request }) => {
    await gotoHooks(page)
    await page.getByRole("button", { name: "Add Webhook" }).click()
    const dlg = page.getByRole("dialog")
    await dlg.getByPlaceholder("https://example.com/webhooks").fill("ftp://ws08-ui.example.com/hook")
    await dlg.getByText("request.failed", { exact: true }).click()
    await dlg.getByRole("button", { name: "Create" }).click()
    await expect(page.getByText("webhook url must be http(s), without credentials or a fragment")).toBeVisible()
    expect(await hooksByUrl(request, "ftp://ws08-ui.example.com/hook")).toHaveLength(0)
  })
})
