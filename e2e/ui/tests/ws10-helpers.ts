// WS-10 console UX sweep — shared helpers (only used by ws10-ux-*.spec.ts).
import { test as base, expect, type Page, type APIRequestContext } from "@playwright/test"
import { existsSync, readFileSync, writeFileSync } from "node:fs"
import { tmpdir, homedir } from "node:os"
import { join } from "node:path"
import { fx, adminApi, login, type Persona } from "./helpers"

export { fx, adminApi, expect }

/**
 * Every WS-10 test fails on uncaught page errors (incl. unhandled promise
 * rejections), console errors and native dialogs (alert() = executed XSS).
 * Network "Failed to load resource" console lines are ignored: error-state
 * tests provoke them on purpose.
 */
export const test = base.extend<{ browserErrors: string[]; knownConsole: string[] }>({
  // React's invalid-nesting warning for the row action menus (<button> inside
  // <button>) is FINDING WS10-2 and is asserted by its own test in
  // ws10-ux-a11y.spec.ts; it is collected here instead of failing every test.
  knownConsole: [async ({}, use) => { await use([]) }, { auto: true }],
  browserErrors: [async ({ page, knownConsole }, use) => {
    const errs: string[] = []
    page.on("pageerror", (e) => errs.push(`pageerror: ${e.message}`))
    page.on("console", (m) => {
      if (m.type() !== "error" || /Failed to load resource|net::ERR_/.test(m.text())) return
      if (/cannot be a descendant of|cannot contain a nested/.test(m.text())) knownConsole.push(m.text())
      else errs.push(`console.error: ${m.text()}`)
    })
    page.on("dialog", (d) => { errs.push(`native dialog (${d.type()}): ${d.message()}`); d.dismiss().catch(() => {}) })
    await use(errs)
    expect(errs, "uncaught browser errors / native dialogs").toEqual([])
  }, { auto: true }],
})

// ---------------------------------------------------------------------------
// Authentication. IAMKit allows 30 logins/min/IP shared by every e2e worker,
// so most tests reuse a token obtained once (directly from IAMKit, exactly as
// the console does) and inject it into localStorage. The UI login form itself
// is exercised by the navigation spec and WS-01.
// ---------------------------------------------------------------------------

type Tok = { access_token: string; refresh_token?: string; exp: number }
const cacheFile = join(tmpdir(), "frv2-ws10-tokens.json")

function readCache(): Record<string, Tok> {
  try { return existsSync(cacheFile) ? JSON.parse(readFileSync(cacheFile, "utf8")) : {} } catch { return {} }
}

export async function tokenFor(persona: "admin" | "viewer"): Promise<Tok> {
  const cache = readCache()
  const hit = cache[persona]
  if (hit && hit.exp - Date.now() > 120_000) return hit
  const p = fx.personas[persona]
  const body = {
    environment_id: fx.iamkit.environment_id, organization_id: fx.iamkit.organization_id,
    application_id: fx.iamkit.application_id, resource_id: fx.iamkit.resource_id,
    email: p.email, password: p.password,
  }
  for (let i = 0; ; i++) {
    const r = await fetch(`${fx.urls.iamkit}/identity/v1/login`, {
      method: "POST", headers: { "Content-Type": "application/json" }, body: JSON.stringify(body),
    })
    if (r.ok) {
      const t = await r.json()
      const tok: Tok = { access_token: t.access_token, refresh_token: t.refresh_token, exp: Date.now() + t.expires_in * 1000 }
      writeFileSync(cacheFile, JSON.stringify({ ...readCache(), [persona]: tok }))
      return tok
    }
    if (r.status !== 429 || i >= 20) throw new Error(`IAMKit login ${persona}: ${r.status} ${await r.text()}`)
    await new Promise((res) => setTimeout(res, 5_000))
  }
}

/** Put a valid session in localStorage, then open `path`. */
export async function openAs(page: Page, persona: "admin" | "viewer", path = "/") {
  const tok = await tokenFor(persona)
  await page.goto("/login")
  await page.evaluate((t) => {
    localStorage.setItem("access_token", t.access_token)
    // No refresh token: a refresh would rotate the cached session for other tests.
    localStorage.removeItem("refresh_token")
  }, tok)
  await page.goto(path)
  await expect(page).not.toHaveURL(/\/login/)
}

/** Real UI login with retries on IAMKit "Too Many Requests". */
export async function uiLogin(page: Page, persona: Persona) {
  for (let i = 0; i < 12; i++) {
    await login(page, persona)
    try {
      await expect(page).not.toHaveURL(/\/login/, { timeout: 5_000 })
      return
    } catch {
      await page.waitForTimeout(5_000)
    }
  }
  await expect(page).not.toHaveURL(/\/login/)
}

/**
 * Replay console API calls that FreeRouter failed with 502 because IAMKit
 * throttled /api/v1 (shared 120 req/min budget, WS04-4). Throttled calls did
 * nothing, so replaying is safe. Registered first so per-test routes win.
 */
export async function retryThrottled(page: Page) {
  await page.route(/\/api\/v1\/(access|service-accounts)/, async (route) => {
    try {
      let resp = await route.fetch()
      for (let i = 0; i < 20 && resp.status() === 502; i++) {
        await new Promise((res) => setTimeout(res, 3_000))
        resp = await route.fetch()
      }
      await route.fulfill({ response: resp })
    } catch { /* page closed */ }
  })
}

export async function retry502<T extends { status(): number }>(fn: () => Promise<T>): Promise<T> {
  for (let i = 0; ; i++) {
    const r = await fn()
    if (r.status() !== 502 || i >= 20) return r
    await new Promise((res) => setTimeout(res, 3_000))
  }
}

// ---------------------------------------------------------------------------
// Pages
// ---------------------------------------------------------------------------

export const PAGES: { path: string; heading: string; ready: (p: Page) => Promise<void> }[] = [
  { path: "/", heading: "Dashboard", ready: async (p) => { await expect(p.getByText("Requests by Model")).toBeVisible() } },
  { path: "/providers", heading: "Providers", ready: async (p) => { await expect(p.getByText("e2e-openai").first()).toBeVisible() } },
  { path: "/models", heading: "Models", ready: async (p) => { await expect(p.getByText("e2e-ok", { exact: true }).first()).toBeVisible() } },
  { path: "/provider-keys", heading: "Provider Keys", ready: async (p) => { await expect(p.getByText("e2e-openai").first()).toBeVisible() } },
  { path: "/rate-limits", heading: "Rate Limits", ready: async (p) => { await expect(p.getByText("Configs match by")).toBeVisible() } },
  { path: "/usage", heading: "Usage", ready: async (p) => { await expect(p.locator("table").first()).toBeVisible() } },
  { path: "/guardrails", heading: "Guardrails", ready: async (p) => { await expect(p.getByText("Tenant-specific content filtering rules")).toBeVisible() } },
  { path: "/webhooks", heading: "Webhooks", ready: async (p) => { await expect(p.getByText("Endpoints", { exact: true })).toBeVisible() } },
  { path: "/service-accounts", heading: "Service Accounts", ready: async (p) => { await expect(p.getByRole("cell", { name: "e2e-gateway-only", exact: true })).toBeVisible() } },
  { path: "/access", heading: "Access Management", ready: async (p) => { await expect(p.getByRole("tab", { name: /Users/ })).toBeVisible() } },
]

export const heading = (page: Page, name: string) => page.getByRole("heading", { name, exact: true })

/** Text that a human would read as an error message. */
export const ERROR_TEXT = /error|failed|could not|couldn't|unable|went wrong|try again|unavailable/i

/** Finding screenshots live next to the findings report (test-results is wiped per run). */
export const SHOTS_DIR = join(homedir(), ".rness/plans/freerouterv2/e2e-findings/WS-10-screenshots")

export async function shot(page: Page, name: string) {
  const path = join(SHOTS_DIR, `${name}.png`)
  await page.screenshot({ path, fullPage: true })
  return path
}

// ---------------------------------------------------------------------------
// Test-data helpers (everything named ws10-*)
// ---------------------------------------------------------------------------

export const LONG = "ws10-" + "L".repeat(195) // 200 chars
export const XSS = `ws10-<img src=x onerror=alert(1)>`
export const EMOJI = "ws10-🚀🔥 émoji ✓ 漢字"
export const RTL = "ws10-مرحبا بالعالم"

export function api(request: APIRequestContext) { return adminApi(request) }
