import { test, expect, openAs, PAGES, heading, ERROR_TEXT, shot } from "./ws10-helpers"
import { stubApi, API_RE } from "./ws10-stub"

// WS-10: loading / empty / error states of every console page, driven by
// page.route() stubs so the result does not depend on shared e2e data.
// Every test also fails on uncaught page errors (see ws10-helpers browserErrors).

test.use({ viewport: { width: 1280, height: 800 } })
test.describe.configure({ timeout: 90_000 })

const EMPTY_TEXT: Record<string, RegExp> = {
  "/": /No requests yet|No usage data yet/,
  "/providers": /No providers/,
  "/models": /No models/,
  "/provider-keys": /No provider keys/,
  "/rate-limits": /No rate limits configured/,
  "/usage": /No logs found/,
  "/guardrails": /No custom rules configured/,
  "/webhooks": /No webhooks configured/,
  "/service-accounts": /No service accounts/,
  "/access": /No users/,
}

test.describe("WS-10 loading state", () => {
  for (const p of PAGES) {
    test(`${p.path} shows the page title and a skeleton while loading`, async ({ page }) => {
      await openAs(page, "admin", "/login")
      await stubApi(page, { delayMs: 2_500 })
      await page.goto(p.path)
      await expect(heading(page, p.heading)).toBeVisible()
      await expect(page.locator("[data-slot=skeleton]").first()).toBeVisible()
      // ...and resolves to content afterwards (no infinite skeleton).
      await expect(page.locator("[data-slot=skeleton]")).toHaveCount(0, { timeout: 15_000 })
    })
  }
})

test.describe("WS-10 empty state", () => {
  for (const p of PAGES) {
    test(`${p.path} shows a readable empty state`, async ({ page }) => {
      await openAs(page, "admin", "/login")
      await stubApi(page, { empty: true })
      await page.goto(p.path)
      await expect(heading(page, p.heading)).toBeVisible()
      await expect(page.getByText(EMPTY_TEXT[p.path]).first()).toBeVisible()
    })
  }
})

test.describe("WS-10 error state", () => {
  // FINDING WS10-1: no console page has an error state. Every page `await`s its
  // list calls in a useEffect without try/catch: a 500 (or network failure)
  // leaves the page on its loading skeleton forever and raises an unhandled
  // promise rejection (pageerror). These tests demonstrate it per page.
  // One test per failure mode, every page checked with soft assertions so the
  // report lists exactly which pages lack an error state.
  for (const mode of ["500", "abort"] as const) {
    test(`every page shows a readable error when the API ${mode === "500" ? "returns 500" : "is unreachable"}`, async ({ page, browserErrors }) => {
      test.setTimeout(180_000)
      await openAs(page, "admin", "/login")
      await page.route(API_RE, (r) => mode === "500"
        ? r.fulfill({ status: 500, json: { code: "INTERNAL", message: "ws10 simulated failure", type: "INTERNAL", status_code: 500 } })
        : r.abort("connectionrefused"))
      for (const p of PAGES) {
        const before = browserErrors.length
        await page.goto(p.path)
        await expect(heading(page, p.heading)).toBeVisible()
        // A human must be told something went wrong, and the skeleton must stop.
        const msg = page.locator("main").getByText(ERROR_TEXT).first()
        await msg.waitFor({ timeout: 4_000 }).catch(() => {})
        const ok = await msg.isVisible()
        expect.soft(ok, `${p.path}: readable error message visible`).toBe(true)
        expect.soft(await page.locator("[data-slot=skeleton]").count(), `${p.path}: skeleton gone`).toBe(0)
        const thrown = browserErrors.slice(before).filter((x) => x.startsWith("pageerror"))
        expect.soft(thrown, `${p.path}: no uncaught exception`).toEqual([])
        if (!ok) await shot(page, `WS10-1-error-${mode}${p.path === "/" ? "_dashboard" : p.path.replace(/\//g, "_")}`)
      }
      // Per-page results are reported by the soft assertions above.
      browserErrors.length = 0
    })
  }
})
