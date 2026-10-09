import type { Page, Request } from "@playwright/test"
import { test, expect, fx, adminApi, uniq } from "./helpers"

// WS-01 — console authentication & session handling.
// IAMKit rate-limits /identity/v1/login per IP (30/min, shared by every
// workstream), so logins here retry on "Too Many Requests".

const LOGIN = "/identity/v1/login"
const REFRESH = "/identity/v1/refresh"
const LOGOUT = "/identity/v1/logout"

// Login retries on the shared limiter can exceed the default 30s.
test.describe.configure({ timeout: 120_000 })

const err = (page: Page) => page.locator("p.text-destructive")

/** Submits the login form, retrying while IAMKit answers 429. Returns the error text, or null on success. */
async function submitLogin(page: Page, email: string, password: string): Promise<string | null> {
  for (let attempt = 0; attempt < 12; attempt++) {
    if (!page.url().endsWith("/login")) await page.goto("/login")
    await page.locator("#email").fill(email)
    await page.locator("#password").fill(password)
    const res = page.waitForResponse((r) => r.url().endsWith(LOGIN) && r.request().method() === "POST")
    await page.getByRole("button", { name: "Sign in" }).click()
    const status = (await res).status()
    if (status === 429) {
      await page.waitForTimeout(5_000)
      continue
    }
    if (status === 200) {
      await expect(page).not.toHaveURL(/\/login/, { timeout: 10_000 })
      return null
    }
    await expect(err(page)).toBeVisible()
    return (await err(page).textContent())?.trim() ?? ""
  }
  throw new Error("login still rate limited after retries")
}

async function mustLogin(page: Page, email: string, password: string) {
  const e = await submitLogin(page, email, password)
  expect(e, "login should succeed").toBeNull()
}

const storage = (page: Page) =>
  page.evaluate(() => ({ access: localStorage.getItem("access_token"), refresh: localStorage.getItem("refresh_token") }))

/** Replaces the stored access token's signature so the server rejects it (simulates expiry). */
async function tamperAccessToken(page: Page) {
  await page.evaluate(() => {
    const t = localStorage.getItem("access_token")!
    const [h, p] = t.split(".")
    localStorage.setItem("access_token", `${h}.${p}.dGFtcGVyZWQtc2lnbmF0dXJl`)
  })
}

function track(page: Page, suffix: string): Request[] {
  const seen: Request[] = []
  page.on("request", (r) => {
    if (r.method() === "POST" && r.url().endsWith(suffix)) seen.push(r)
  })
  return seen
}

/** Creates a viewer-role user via the admin API; returns id/email and a cleanup. */
async function makeUser(request: Parameters<typeof adminApi>[0], prefix: string) {
  const api = adminApi(request)
  const email = `${uniq(`ws01-${prefix}`)}@e2e.test`.toLowerCase()
  const password = "e2e-password-12345"
  // IAMKit's /api/v1 limiter is shared with every workstream; FreeRouter
  // surfaces its 429 as 502, so retry those.
  let r = await api.post("/access/users", { email, name: `WS01 ${prefix}`, password })
  for (let i = 0; i < 10 && r.status() === 502; i++) {
    await new Promise((res) => setTimeout(res, 5_000))
    r = await api.post("/access/users", { email, name: `WS01 ${prefix}`, password })
  }
  expect(r.status(), await r.text()).toBe(201)
  const id = (await r.json()).id as string
  const a = await api.post("/access/role-assignments", { user_id: id, role_id: fx.roles.viewer })
  expect(a.ok(), await a.text()).toBeTruthy()
  const cleanup = () =>
    request.delete(`${fx.urls.iamkit}/management/v1/environments/${fx.iamkit.environment_id}/users/${id}/permanent`, {
      headers: { "X-API-Key": fx.iamkit.management_key },
    })
  return { id, email, password, cleanup }
}

test.describe("WS-01 login form", () => {
  test("valid credentials land on the dashboard with both tokens stored", async ({ page }) => {
    const v = fx.personas.viewer
    await mustLogin(page, v.email!, v.password!)
    await expect(page).toHaveURL(/\/$/)
    await expect(page.getByRole("heading", { name: "Dashboard" })).toBeVisible()
    const s = await storage(page)
    expect(s.access).toMatch(/^eyJ[\w-]+\.[\w-]+\.[\w-]+$/)
    expect(s.refresh).toBeTruthy()
    // Visiting /login while authenticated bounces back to the app.
    await page.goto("/login")
    await expect(page).toHaveURL(/\/$/)
  })

  test("email is case-insensitive and surrounding whitespace is ignored", async ({ page }) => {
    const v = fx.personas.viewer
    await mustLogin(page, `  ${v.email!.toUpperCase()} `, v.password!)
    await expect(page.getByRole("heading", { name: "Dashboard" })).toBeVisible()
  })

  test("empty fields are blocked client-side without calling IAMKit", async ({ page }) => {
    const logins = track(page, LOGIN)
    await page.goto("/login")
    await page.getByRole("button", { name: "Sign in" }).click()
    await expect(page.locator("#email:invalid")).toHaveCount(1)
    await page.locator("#email").fill(fx.personas.viewer.email!)
    await page.getByRole("button", { name: "Sign in" }).click()
    await expect(page.locator("#password:invalid")).toHaveCount(1)
    await page.locator("#email").fill("not-an-email")
    await page.locator("#password").fill("x")
    await page.getByRole("button", { name: "Sign in" }).click()
    await expect(page.locator("#email:invalid")).toHaveCount(1)
    await expect(page).toHaveURL(/\/login$/)
    expect(logins).toHaveLength(0)
  })

  test("wrong password, unknown email and suspended user stay on /login with an error and no tokens", async ({ page }) => {
    for (const [email, pw] of [
      [fx.personas.viewer.email!, "definitely-wrong-password"],
      ["nobody-ws01@e2e.test", "e2e-password-12345"],
      [fx.personas.suspended.email!, fx.personas.suspended.password!],
    ]) {
      const e = await submitLogin(page, email, pw)
      expect(e, `${email} must fail`).not.toBeNull()
      await expect(page).toHaveURL(/\/login$/)
      expect(await storage(page)).toEqual({ access: null, refresh: null })
      // The form stays usable after an error.
      await expect(page.getByRole("button", { name: "Sign in" })).toBeEnabled()
    }
  })

  // FINDING WS01-1: the console reads `message` at the top level but IAMKit
  // returns {"error":{"message":…}}, so the user sees the bare HTTP status
  // text ("Unauthorized") instead of a credentials message.
  test("wrong password shows a human-readable credentials error", async ({ page }) => {
    const e = await submitLogin(page, fx.personas.viewer.email!, "definitely-wrong-password")
    expect(e).toMatch(/invalid (credentials|email or password)/i)
  })
})

test.describe("WS-01 session", () => {
  test("deep link while logged out goes to /login; after login lands on the dashboard (no return-to)", async ({ page }) => {
    await page.goto("/providers")
    await expect(page).toHaveURL(/\/login$/)
    await mustLogin(page, fx.personas.viewer.email!, fx.personas.viewer.password!)
    // Documented behaviour: the original /providers target is not preserved.
    await expect(page).toHaveURL(/\/$/)
    await page.goto("/providers")
    await expect(page.getByText("e2e-openai")).toBeVisible()
  })

  test("tampered access token is refreshed silently with exactly one refresh for concurrent 401s", async ({ page }) => {
    await mustLogin(page, fx.personas.viewer.email!, fx.personas.viewer.password!)
    await expect(page.getByRole("heading", { name: "Dashboard" })).toBeVisible()
    const before = await storage(page)
    await tamperAccessToken(page)

    const refreshes = track(page, REFRESH)
    const unauthorized: string[] = []
    page.on("response", (r) => {
      if (r.url().startsWith(fx.urls.api) && r.status() === 401) unauthorized.push(r.url())
    })
    await page.reload()
    // The dashboard fires 5 API calls in parallel; all must succeed after one refresh
    // (it only leaves its skeleton once every call resolved).
    await expect(page.getByText("Requests by Model")).toBeVisible({ timeout: 10_000 })
    await expect(page).toHaveURL(/\/$/)
    expect(unauthorized.length, "concurrent requests hit 401 first").toBeGreaterThanOrEqual(2)
    expect(refreshes, "refresh coalesced").toHaveLength(1)

    const after = await storage(page)
    expect(after.access).not.toEqual(before.access)
    expect(after.refresh).not.toEqual(before.refresh)
    const ok = await page.request.get(`${fx.urls.api}/providers`, { headers: { Authorization: `Bearer ${after.access}` } })
    expect(ok.status()).toBe(200)
  })

  test("invalid refresh token logs out to /login and clears storage", async ({ page }) => {
    await mustLogin(page, fx.personas.viewer.email!, fx.personas.viewer.password!)
    await expect(page.getByRole("heading", { name: "Dashboard" })).toBeVisible()
    const real = await storage(page)
    await tamperAccessToken(page)
    await page.evaluate(() => localStorage.setItem("refresh_token", "ws01-garbage-refresh-token"))
    const refreshes = track(page, REFRESH)
    await page.reload()
    await expect(page).toHaveURL(/\/login$/, { timeout: 10_000 })
    expect(await storage(page)).toEqual({ access: null, refresh: null })
    expect(refreshes.length).toBeGreaterThanOrEqual(1)
    // Clean up the real session we abandoned.
    await page.request.post(`${fx.urls.iamkit}${LOGOUT}`, {
      headers: { Authorization: `Bearer ${real.access}` },
      data: { environment_id: fx.iamkit.environment_id, audience: fx.iamkit.audience },
    })
  })

  test("missing refresh token with a rejected access token logs out", async ({ page }) => {
    await mustLogin(page, fx.personas.viewer.email!, fx.personas.viewer.password!)
    await expect(page.getByRole("heading", { name: "Dashboard" })).toBeVisible()
    const real = await storage(page)
    await tamperAccessToken(page)
    await page.evaluate(() => localStorage.removeItem("refresh_token"))
    const refreshes = track(page, REFRESH)
    await page.goto("/providers")
    await expect(page).toHaveURL(/\/login$/, { timeout: 10_000 })
    expect(await storage(page)).toEqual({ access: null, refresh: null })
    expect(refreshes).toHaveLength(0)
    await page.request.post(`${fx.urls.iamkit}${LOGOUT}`, {
      headers: { Authorization: `Bearer ${real.access}` },
      data: { environment_id: fx.iamkit.environment_id, audience: fx.iamkit.audience },
    })
  })

  test("sign out revokes the session at IAMKit and clears storage", async ({ page }) => {
    await mustLogin(page, fx.personas.viewer.email!, fx.personas.viewer.password!)
    await expect(page.getByRole("heading", { name: "Dashboard" })).toBeVisible()
    const s = await storage(page)
    const logouts = track(page, LOGOUT)

    await page.getByRole("button", { name: "Sign out" }).click()
    await expect(page).toHaveURL(/\/login$/)
    expect(await storage(page)).toEqual({ access: null, refresh: null })
    expect(logouts).toHaveLength(1)
    expect((await logouts[0].response())?.status()).toBe(204)

    const api = await page.request.get(`${fx.urls.api}/providers`, { headers: { Authorization: `Bearer ${s.access}` } })
    expect(api.status(), "old access token after sign out").toBe(401)
    const refresh = await page.request.post(`${fx.urls.iamkit}${REFRESH}`, {
      data: {
        environment_id: fx.iamkit.environment_id, organization_id: fx.iamkit.organization_id,
        application_id: fx.iamkit.application_id, resource_id: fx.iamkit.resource_id, refresh_token: s.refresh,
      },
    })
    expect(refresh.status(), "old refresh token after sign out").toBe(401)

    // Navigating back into the app must not resurrect the session.
    await page.goto("/providers")
    await expect(page).toHaveURL(/\/login$/)
  })

  test("user suspended mid-session is sent to /login on the next call", async ({ page, request }) => {
    const u = await makeUser(request, "uisusp")
    try {
      await mustLogin(page, u.email, u.password)
      await expect(page.getByRole("heading", { name: "Dashboard" })).toBeVisible()
      const del = await adminApi(request).delete(`/access/users/${u.id}`)
      expect(del.status()).toBe(204)

      const refreshes = track(page, REFRESH)
      await page.goto("/providers")
      await expect(page).toHaveURL(/\/login$/, { timeout: 10_000 })
      expect(await storage(page)).toEqual({ access: null, refresh: null })
      expect(refreshes, "one refresh attempt, then give up").toHaveLength(1)
      expect((await refreshes[0].response())?.status()).toBe(401)
    } finally {
      await u.cleanup()
    }
  })
})
