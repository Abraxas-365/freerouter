import type { APIRequestContext, Page } from "@playwright/test"
import { test, expect, openAs, api, retry502, retryThrottled, heading, shot, LONG, XSS, EMOJI, RTL } from "./ws10-helpers"

// WS-10: real create flows with hostile/long/unicode input, double-submit,
// and toast feedback. Everything created is named ws10-* and removed in afterEach.

test.use({ viewport: { width: 1280, height: 800 } })
test.describe.configure({ timeout: 120_000 })

async function cleanup(request: APIRequestContext) {
  const a = api(request)
  const list = async (path: string) => {
    const r = await retry502(() => a.get(path))
    if (!r.ok()) return []
    const j = await r.json()
    return (Array.isArray(j) ? j : j.items ?? []) as Record<string, unknown>[]
  }
  const isOurs = (s: unknown) => typeof s === "string" && s.includes("ws10-")
  for (const p of await list("/providers?limit=100&search=ws10")) if (isOurs(p.name)) await a.delete(`/providers/${p.id}`)
  for (const r of await list("/rate-limits?limit=100")) if (isOurs(r.name)) await a.delete(`/rate-limits/${r.id}`)
  for (const r of await list("/guardrails/rules")) if (isOurs(r.name)) await a.delete(`/guardrails/rules/${r.id}`)
  for (const w of await list("/webhooks?limit=100")) if (isOurs(w.url)) await a.delete(`/webhooks/${w.id}`)
  for (const s of await list("/service-accounts")) if (isOurs(s.name)) await retry502(() => a.delete(`/service-accounts/${s.id}`))
}

test.afterEach(async ({ request }) => { await cleanup(request) })

async function countByName(request: APIRequestContext, path: string, field: string, value: string) {
  const r = await retry502(() => api(request).get(path))
  expect(r.ok(), `${path} -> ${r.status()}`).toBe(true)
  const j = await r.json()
  return ((Array.isArray(j) ? j : j.items) as Record<string, unknown>[]).filter((x) => x[field] === value).length
}

async function openPage(page: Page, path: string, h: string) {
  await retryThrottled(page)
  await openAs(page, "admin", path)
  await expect(heading(page, h)).toBeVisible()
  await expect(page.locator("[data-slot=skeleton]")).toHaveCount(0, { timeout: 30_000 })
}

/** Asserts the hostile string is shown literally and never became markup. */
async function assertRenderedAsText(page: Page, value: string) {
  await expect(page.getByText(value, { exact: true }).first()).toBeVisible({ timeout: 30_000 })
  expect(await page.locator("img[src='x']").count(), "injected <img> present in DOM").toBe(0)
}

async function createProvider(page: Page, name: string, base = "http://localhost:29100/ws10/v1") {
  await page.getByRole("button", { name: "New provider" }).click()
  const dlg = page.getByRole("dialog", { name: "New provider" })
  await dlg.getByLabel("Name", { exact: true }).fill(name)
  await dlg.getByLabel("Description").fill(`${RTL} ${XSS}`)
  await dlg.getByLabel("Base URL").fill(base)
  return dlg
}

test.describe("WS-10 hostile / long / unicode input", () => {
  for (const [label, value] of [["xss", XSS], ["emoji", EMOJI], ["rtl", RTL], ["200-char", LONG]] as const) {
    test(`providers: ${label} name renders as text`, async ({ page, request }) => {
      await openPage(page, "/providers", "Providers")
      const dlg = await createProvider(page, value)
      await dlg.getByRole("button", { name: "Create" }).click()
      await expect(page.getByText("Provider created")).toBeVisible()
      await assertRenderedAsText(page, value)
      expect(await countByName(request, "/providers?limit=100&search=ws10", "name", value)).toBe(1)
      if (label === "200-char") {
        // Long names must not push the table wider than the page.
        const overflow = await page.evaluate(() => document.documentElement.scrollWidth - document.documentElement.clientWidth)
        if (overflow > 0) await shot(page, "WS10-4-long-provider-name")
        expect(overflow, "page scrolls horizontally because of a 200-char name").toBeLessThanOrEqual(0)
      }
    })
  }

  test("rate-limits: xss + rtl name renders as text", async ({ page, request }) => {
    await openPage(page, "/rate-limits", "Rate Limits")
    const name = `${XSS} ${RTL}`
    await page.getByRole("button", { name: "New limit" }).first().click()
    const dlg = page.getByRole("dialog", { name: "New rate limit" })
    await dlg.getByLabel("Name", { exact: true }).fill(name)
    await dlg.getByRole("combobox").click()
    await page.getByRole("option", { name: "Custom subject ID…" }).click()
    await dlg.getByPlaceholder("JWT subject UUID").fill("ws10-subject-<script>alert(2)</script>")
    await dlg.getByRole("button", { name: "Create" }).click()
    await expect(page.getByText("Rate limit created")).toBeVisible()
    await assertRenderedAsText(page, name)
    await expect(page.getByText("ws10-subject-<script>alert(2)</script>").first()).toBeVisible()
    expect(await countByName(request, "/rate-limits?limit=100", "name", name)).toBe(1)
  })

  test("guardrails: xss rule name and emoji terms render as text", async ({ page, request }) => {
    await openPage(page, "/guardrails", "Guardrails")
    const name = `${XSS} ${EMOJI}`
    await page.getByRole("button", { name: "Add Rule" }).click()
    const dlg = page.getByRole("dialog", { name: "Create Rule" })
    // Labels are not associated (WS10-3): locate by placeholder/order instead.
    const inputs = dlg.locator("input:not([aria-hidden=true]):not([type=hidden])")
    await inputs.first().fill(name)
    await dlg.locator("textarea, input").nth(1).fill("ws10-<b>bold</b>, 🚀")
    await dlg.getByRole("button", { name: "Create" }).click()
    await expect(page.getByText("Rule created")).toBeVisible()
    await assertRenderedAsText(page, name)
    expect(await page.locator("main b").filter({ hasText: "bold" }).count()).toBe(0)
    expect(await countByName(request, "/guardrails/rules", "name", name)).toBe(1)
  })

  test("webhooks: xss in the URL path renders as text", async ({ page, request }) => {
    await openPage(page, "/webhooks", "Webhooks")
    const url = `https://hooks.example.com/ws10-<img src=x onerror=alert(1)>`
    await page.getByRole("button", { name: "Add Webhook" }).click()
    const dlg = page.getByRole("dialog", { name: "Add Webhook" })
    await dlg.getByPlaceholder("https://example.com/webhooks").fill(url)
    await dlg.getByText("request.completed", { exact: true }).click()
    await dlg.getByRole("button", { name: "Create" }).click()
    await expect(page.getByText("Webhook created")).toBeVisible()
    // The one-time secret dialog is shown; close it.
    await page.keyboard.press("Escape")
    await assertRenderedAsText(page, url)
    expect(await countByName(request, "/webhooks?limit=100", "url", url)).toBe(1)
  })

  test("service-accounts: unicode + xss name renders as text", async ({ page, request }) => {
    await openPage(page, "/service-accounts", "Service Accounts")
    const name = `${XSS} ${EMOJI}`
    await page.getByRole("button", { name: "Create Service Account" }).first().click()
    const dlg = page.getByRole("dialog", { name: "Create Service Account" })
    await dlg.locator("input:not([type=checkbox]):not([aria-hidden=true])").first().fill(name)
    await dlg.getByRole("checkbox").first().check()
    await dlg.getByRole("button", { name: "Create" }).click()
    await expect(page.getByText("Service account created").first()).toBeVisible()
    await page.keyboard.press("Escape")
    await assertRenderedAsText(page, name)
    expect(await countByName(request, "/service-accounts", "name", name)).toBe(1)
  })
})

test.describe("WS-10 rapid double-submit", () => {
  test("providers: double-clicking Create creates exactly one provider", async ({ page, request }) => {
    await openPage(page, "/providers", "Providers")
    const name = `ws10-dbl-${Date.now()}`
    const dlg = await createProvider(page, name)
    await dlg.getByRole("button", { name: "Create" }).dblclick()
    await expect(page.getByText("Provider created").first()).toBeVisible()
    await page.waitForTimeout(1_500)
    const n = await countByName(request, "/providers?limit=100&search=ws10", "name", name)
    if (n !== 1) await shot(page, "WS10-5-double-submit-providers")
    expect(n, "providers created by a double click").toBe(1)
  })

  test("rate-limits: double-clicking Create creates exactly one rate limit", async ({ page, request }) => {
    await openPage(page, "/rate-limits", "Rate Limits")
    const name = `ws10-dbl-${Date.now()}`
    await page.getByRole("button", { name: "New limit" }).first().click()
    const dlg = page.getByRole("dialog", { name: "New rate limit" })
    await dlg.getByLabel("Name", { exact: true }).fill(name)
    await dlg.getByRole("combobox").click()
    await page.getByRole("option", { name: "Custom subject ID…" }).click()
    await dlg.getByPlaceholder("JWT subject UUID").fill(`ws10-dbl-subject-${Date.now()}`)
    await dlg.getByRole("button", { name: "Create" }).dblclick()
    await expect(page.getByText("Rate limit created").first()).toBeVisible()
    await page.waitForTimeout(1_500)
    const n = await countByName(request, "/rate-limits?limit=100", "name", name)
    if (n !== 1) await shot(page, "WS10-5-double-submit-rate-limits")
    expect(n, "rate limits created by a double click").toBe(1)
  })

  test("webhooks: double-clicking Create creates exactly one webhook", async ({ page, request }) => {
    await openPage(page, "/webhooks", "Webhooks")
    const url = `https://hooks.example.com/ws10-dbl-${Date.now()}`
    await page.getByRole("button", { name: "Add Webhook" }).click()
    const dlg = page.getByRole("dialog", { name: "Add Webhook" })
    await dlg.getByPlaceholder("https://example.com/webhooks").fill(url)
    await dlg.getByText("request.failed", { exact: true }).click()
    await dlg.getByRole("button", { name: "Create" }).dblclick()
    await expect(page.getByText("Webhook created").first()).toBeVisible()
    await page.waitForTimeout(1_500)
    const n = await countByName(request, "/webhooks?limit=100", "url", url)
    if (n !== 1) await shot(page, "WS10-5-double-submit-webhooks")
    expect(n, "webhooks created by a double click").toBe(1)
  })
})

test.describe("WS-10 toasts", () => {
  test("providers: create and delete both show a success toast", async ({ page, request }) => {
    await openPage(page, "/providers", "Providers")
    const name = `ws10-toast-${Date.now()}`
    const dlg = await createProvider(page, name)
    await dlg.getByRole("button", { name: "Create" }).click()
    await expect(page.getByText("Provider created")).toBeVisible()
    await page.getByRole("row").filter({ hasText: name }).getByRole("button").last().click()
    await page.getByRole("menuitem", { name: "Delete" }).click()
    await page.getByRole("dialog").getByRole("button", { name: /^Delete/ }).click()
    await expect(page.getByText("Provider deleted")).toBeVisible()
    await expect(page.getByText(name)).toHaveCount(0)
    expect(await countByName(request, "/providers?limit=100&search=ws10", "name", name)).toBe(0)
  })

  test("rate-limits: a failed delete tells the user (no silent failure)", async ({ page, request }) => {
    // Known class of bug (WS03-2: console swallows API errors); this checks the
    // delete path, which earlier workstreams did not cover. Uses an own ws10 row.
    const name = `ws10-delfail-${Date.now()}`
    const c = await api(request).post("/rate-limits", { name, subject_id: `ws10-delfail-${Date.now()}`, rpm: 1, max_concurrent: 1 })
    expect(c.ok()).toBe(true)
    await openPage(page, "/rate-limits", "Rate Limits")
    await page.route(/\/api\/v1\/rate-limits\/[^/]+$/, (r) => r.request().method() === "DELETE"
      ? r.fulfill({ status: 500, json: { code: "INTERNAL", message: "ws10 simulated delete failure", type: "INTERNAL", status_code: 500 } })
      : r.fallback())
    const row = page.getByRole("row").filter({ hasText: name })
    await row.getByRole("button").last().click()
    await page.getByRole("menuitem", { name: "Delete" }).click()
    await page.getByRole("dialog").getByRole("button", { name: /^Delete/ }).click()
    try {
      await expect(page.getByText(/ws10 simulated delete failure|failed|error/i).first()).toBeVisible({ timeout: 5_000 })
    } catch (e) {
      await shot(page, "WS10-7-delete-failure-silent")
      throw e
    }
  })
})
