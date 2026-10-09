// WS-02 — console authorization: sidebar gating, RequirePermission redirects,
// read-only (viewer) write controls, users who cannot get a token, and how a
// role change propagates to an open console session.
import { test, expect, fx, login, adminApi, uniq, type Persona } from "./helpers"
import type { Page, APIRequestContext } from "@playwright/test"

test.describe.configure({ timeout: 240_000 })

const ALL_NAV = [
  "Dashboard", "Providers", "Models", "Provider Keys", "Rate Limits", "Usage",
  "Service Accounts", "Access", "Guardrails", "Webhooks",
]

const ROUTES: Record<string, string> = {
  Providers: "/providers", Models: "/models", "Provider Keys": "/provider-keys",
  "Rate Limits": "/rate-limits", Usage: "/usage", "Service Accounts": "/service-accounts",
  Access: "/access", Guardrails: "/guardrails", Webhooks: "/webhooks",
}

/**
 * Logs in through the real form, waiting out IAMKit's shared 30/min per-IP
 * login limiter (other workstreams log in concurrently). Returns once the
 * console has left /login, or the visible error text if it never does.
 */
async function loginRetry(page: Page, persona: Persona): Promise<string | null> {
  const err = page.locator("p.text-destructive")
  for (let i = 0; i < 30; i++) {
    await login(page, persona)
    const outcome = await Promise.race([
      page.waitForURL((u) => !u.pathname.startsWith("/login"), { timeout: 15_000 }).then(() => "ok"),
      err.waitFor({ state: "visible", timeout: 15_000 }).then(() => "err"),
    ]).catch(() => "timeout")
    if (outcome === "ok") return null
    const text = (await err.textContent().catch(() => "")) ?? ""
    if (!/too many requests/i.test(text)) return text
    await page.waitForTimeout(5_000)
  }
  throw new Error(`login for ${persona.email} kept hitting the rate limit`)
}

async function loginOk(page: Page, persona: Persona) {
  const err = await loginRetry(page, persona)
  expect(err, `login ${persona.email}`).toBeNull()
  await expect(sidebar(page).getByText("Dashboard")).toBeVisible()
}

/** Uncaught page errors, ignoring IAMKit throttling of the backend (WS02-1). */
function pageErrors(page: Page): string[] {
  const errors: string[] = []
  page.on("pageerror", (e) => { if (!/ in IAMKit$/.test(e.message)) errors.push(e.message) })
  return errors
}

const sidebar = (page: Page) => page.locator('[data-sidebar="sidebar"]').first()

async function navItems(page: Page): Promise<string[]> {
  await expect(sidebar(page).getByText("Dashboard")).toBeVisible()
  return (await sidebar(page).locator("a").allInnerTexts()).map((s) => s.trim()).filter(Boolean)
}

/**
 * Waits for a page's initial data load (skeletons gone). IAMKit-backed pages
 * (/access, /service-accounts) hang forever on a 502 caused by IAMKit
 * throttling FreeRouter's backend (findings WS02-1, WS02-4), so those are
 * reloaded a few times before giving up.
 */
async function settled(page: Page) {
  const sk = page.locator('main [data-slot="skeleton"]')
  const iamBacked = /\/(access|service-accounts)$/.test(new URL(page.url()).pathname)
  const tries = iamBacked ? 10 : 1
  for (let i = 0; i < tries; i++) {
    const ok = await expect(sk).toHaveCount(0, { timeout: iamBacked ? 8_000 : 15_000 }).then(() => true, () => false)
    if (ok) return
    if (i < tries - 1) {
      await page.waitForTimeout(3_000)
      await page.reload()
    }
  }
  throw new Error(`${page.url()} never finished loading (skeletons still shown)`)
}

/** API call that waits out IAMKit throttling of FreeRouter's backend (502 "... in IAMKit"). */
async function api(request: APIRequestContext, method: "get" | "post" | "put" | "patch" | "delete", path: string, data?: unknown) {
  const a = adminApi(request)
  for (let i = 0; i < 40; i++) {
    const r = method === "get" || method === "delete" ? await a[method](path) : await a[method](path, data)
    if (r.status() === 502 || (r.status() === 401 && /service account credential/.test(await r.text()))) {
      await new Promise((res) => setTimeout(res, 3_000))
      continue
    }
    return r
  }
  throw new Error(`${method} ${path}: IAMKit kept throttling`)
}

test.describe("WS-02 sidebar + route gating", () => {
  test("admin sees every nav item and can open every page", async ({ page }) => {
    const errors = pageErrors(page)
    await loginOk(page, fx.personas.admin)
    expect(await navItems(page)).toEqual(ALL_NAV)
    for (const [label, path] of Object.entries(ROUTES)) {
      await sidebar(page).getByText(label, { exact: true }).click()
      await expect(page, label).toHaveURL(new RegExp(`${path}$`))
    }
    // Legacy route redirects to its replacement, not to "/".
    await page.goto("/api-keys")
    await expect(page).toHaveURL(/\/service-accounts$/)
    expect(errors).toEqual([])
  })

  test("viewer (every *:read) sees every nav item; pages render", async ({ page }) => {
    const errors = pageErrors(page)
    await loginOk(page, fx.personas.viewer)
    expect(await navItems(page)).toEqual(ALL_NAV)
    for (const path of Object.values(ROUTES)) {
      await page.goto(path)
      await expect(page, path).toHaveURL(new RegExp(`${path}$`))
      await settled(page)
    }
    expect(errors).toEqual([])
  })

  test("providers_only sees Dashboard/Providers/Models; other pages redirect to /", async ({ page }) => {
    await loginOk(page, fx.personas.providers_only)
    expect(await navItems(page)).toEqual(["Dashboard", "Providers", "Models"])
    for (const path of ["/providers", "/models"]) {
      await page.goto(path)
      await expect(page).toHaveURL(new RegExp(`${path}$`))
      await settled(page)
    }
    for (const path of ["/provider-keys", "/rate-limits", "/usage", "/guardrails", "/webhooks", "/service-accounts", "/api-keys", "/access"]) {
      await page.goto(path)
      await expect(page, `${path} should redirect`).toHaveURL(new RegExp(`^${fx.urls.web}/$`))
    }
    // The page under test is still usable after the redirects.
    await expect(sidebar(page).getByText("Providers", { exact: true })).toBeVisible()
  })

  test("providers_only dashboard renders instead of hanging on skeletons", async ({ page }) => {
    // FINDING WS02-4: Dashboard Promise.all() includes usage calls that 403 for
    // this persona; the rejection is unhandled and the page stays in loading.
    await loginOk(page, fx.personas.providers_only)
    await expect(page).toHaveURL(new RegExp(`^${fx.urls.web}/$`))
    await expect(page.locator('main [data-slot="skeleton"]')).toHaveCount(0, { timeout: 10_000 })
  })

  test("unknown console route keeps the shell (no crash)", async ({ page }) => {
    await loginOk(page, fx.personas.viewer)
    await page.goto("/definitely-not-a-page")
    await expect(sidebar(page).getByText("Dashboard")).toBeVisible()
  })
})

test.describe("WS-02 users without a FreeRouter token", () => {
  for (const name of ["noperm", "suspended", "foreign"] as const) {
    test(`${name} cannot sign in and gets a readable error`, async ({ page }) => {
      const errors = pageErrors(page)
      const msg = await loginRetry(page, fx.personas[name])
      // The console shows the HTTP status text because it reads `message`
      // while IAMKit nests it under `error` (finding WS02-6).
      expect(msg).toMatch(/invalid credentials|unauthorized/i)
      await expect(page).toHaveURL(/\/login/)
      expect(await page.evaluate(() => localStorage.getItem("access_token"))).toBeNull()
      // Deep link while signed out goes back to login.
      await page.goto("/providers")
      await expect(page).toHaveURL(/\/login/)
      expect(errors).toEqual([])
    })
  }
})

test("login rejection shows IAMKit's message, not the bare status text", async ({ page }) => {
  // FINDING WS02-6: lib/iamkit.ts reads `err.message`, but IAMKit returns
  // {"error":{"message":...}}, so every rejection reads "Unauthorized".
  const msg = await loginRetry(page, fx.personas.noperm)
  expect(msg).toMatch(/invalid credentials/i)
})

test.describe("WS-02 read-only persona", () => {
  // Every write control a viewer can see. Each entry is a page + accessible
  // button names that must not be offered to a role without the write perm.
  const WRITE_CONTROLS: { path: string; tab?: string; buttons: RegExp[]; switches?: boolean }[] = [
    { path: "/providers", buttons: [/new provider/i] },
    { path: "/provider-keys", buttons: [/add key/i] },
    { path: "/rate-limits", buttons: [/new limit/i] },
    { path: "/guardrails", buttons: [/add rule/i], switches: true },
    { path: "/webhooks", buttons: [/add webhook/i] },
    { path: "/service-accounts", buttons: [/create service account/i] },
    { path: "/access", tab: "Users", buttons: [/create user/i] },
    { path: "/access", tab: "Roles", buttons: [/create role/i] },
    { path: "/access", tab: "Assignments", buttons: [/assign role/i] },
  ]

  test("viewer is not offered create/edit/delete controls", async ({ page }) => {
    // FINDING WS02-2: no console page consults usePermissions() for write
    // controls; every one below is rendered for a read-only role.
    await loginOk(page, fx.personas.viewer)
    for (const c of WRITE_CONTROLS) {
      await page.goto(c.path)
      await settled(page)
      if (c.tab) await page.getByRole("tab", { name: c.tab }).click()
      for (const b of c.buttons) {
        await expect.soft(page.getByRole("button", { name: b }), `${c.path} ${c.tab ?? ""} ${b}`).toHaveCount(0)
      }
      if (c.switches) {
        await expect.soft(page.locator('main [data-slot="switch"]:not([disabled]):not([data-disabled])'), `${c.path} enabled switches`).toHaveCount(0)
      }
    }
  })

  test("viewer attempting a write gets a visible error and nothing is created", async ({ page, request }) => {
    const errors = pageErrors(page)
    await loginOk(page, fx.personas.viewer)
    await page.goto("/providers")
    await settled(page)
    const btn = page.getByRole("button", { name: /new provider/i }).first()
    test.skip((await btn.count()) === 0, "button hidden for viewer (WS02-2 fixed) — nothing to attempt")
    const name = uniq("ws02-viewer-write")
    await btn.click()
    await page.locator("#p-name").fill(name)
    await page.locator("#p-base").fill("http://localhost:29100/ws02/openai/v1")
    const resp = page.waitForResponse((r) => r.url().endsWith("/api/v1/providers") && r.request().method() === "POST")
    await page.getByRole("dialog").getByRole("button", { name: /^create/i }).click()
    expect((await resp).status()).toBe(403)
    // Nothing created server-side.
    const list = await (await api(request, "get", "/providers?limit=100")).json()
    const leaked = (list.items as { id: string; name: string }[]).filter((p) => p.name === name)
    for (const p of leaked) await api(request, "delete", `/providers/${p.id}`)
    expect(leaked).toEqual([])
    // FINDING WS02-5: the 403 is swallowed — no toast, dialog stays open,
    // and an unhandled promise rejection is raised.
    await expect(page.getByText(/insufficient permissions|forbidden|not allowed/i)).toBeVisible({ timeout: 5_000 })
    expect(errors).toEqual([])
  })
})

test.describe("WS-02 permission change propagation", () => {
  test("granting a role shows up after re-login; revoking it locks the open session out", async ({ page, request }) => {
    const email = `${uniq("ws02-ui")}@e2e.test`
    const password = "ws02-password-12345"
    const persona: Persona = { kind: "user", email, password, note: "ws02 temp" }
    const cleanup: (() => Promise<unknown>)[] = []
    try {
      const u = await api(request, "post", "/access/users", { email, name: "WS02 UI", password })
      expect(u.ok(), await u.text()).toBeTruthy()
      const userId = (await u.json()).id as string
      cleanup.push(async () => {
        await api(request, "delete", `/access/users/${userId}`).catch(() => undefined)
        await request.delete(`${fx.urls.iamkit}/management/v1/environments/${fx.iamkit.environment_id}/users/${userId}/permanent`,
          { headers: { "X-API-Key": fx.iamkit.management_key } })
      })
      const mkRole = async (perms: string[]) => {
        const r = await api(request, "post", "/access/roles", { name: uniq("ws02-ui-role"), permissions: perms })
        expect(r.ok(), await r.text()).toBeTruthy()
        const id = (await r.json()).id as string
        cleanup.push(() => api(request, "delete", `/access/roles/${id}`))
        return id
      }
      const assign = async (role: string) => {
        const r = await api(request, "post", "/access/role-assignments", { user_id: userId, role_id: role })
        expect(r.ok(), await r.text()).toBeTruthy()
      }
      const provRole = await mkRole(["freerouter:providers:read"])
      const keysRole = await mkRole(["freerouter:provider-keys:read"])

      // No role yet → cannot sign in.
      expect(await loginRetry(page, persona)).toMatch(/invalid credentials|unauthorized/i)

      await assign(provRole)
      await loginOk(page, persona)
      expect(await navItems(page)).toEqual(["Dashboard", "Providers", "Models"])

      // Grant a second role while signed in: the console decodes permissions
      // from the JWT, so nothing changes until a new token is issued.
      await assign(keysRole)
      await page.reload()
      expect(await navItems(page)).toEqual(["Dashboard", "Providers", "Models"])
      await page.goto("/provider-keys")
      await expect(page).toHaveURL(new RegExp(`^${fx.urls.web}/$`))

      // Re-login → new permission visible and usable.
      await sidebar(page).getByText("Sign out").click()
      await expect(page).toHaveURL(/\/login/)
      await loginOk(page, persona)
      expect(await navItems(page)).toEqual(["Dashboard", "Providers", "Models", "Provider Keys"])
      await page.goto("/provider-keys")
      await expect(page).toHaveURL(/\/provider-keys$/)
      await settled(page)

      // Revoke providers:read while signed in. IAMKit introspection rejects
      // the old token (its permissions are no longer a subset), so the next
      // API call must not succeed with stale permissions.
      const un = await request.delete(`${fx.urls.api}/access/role-assignments`, {
        headers: { Authorization: `Bearer ${fx.personas.admin_key.secret}`, "Content-Type": "application/json" },
        data: { user_id: userId, role_id: provRole },
      })
      expect(un.status(), await un.text()).toBeLessThan(300)
      const provResp = page.waitForResponse((r) => /\/api\/v1\/providers(\?|$)/.test(r.url()) && r.request().method() === "GET")
      await page.goto("/providers")
      expect([401, 403]).toContain((await provResp).status())
      // After the console reacts (refresh or sign-out), providers is no longer offered.
      await expect(async () => {
        const onLogin = /\/login/.test(page.url())
        const items = onLogin ? [] : await sidebar(page).locator("a").allInnerTexts()
        expect(onLogin || !items.includes("Providers")).toBeTruthy()
      }).toPass({ timeout: 15_000 })
    } finally {
      for (const c of cleanup.reverse()) await c().catch(() => undefined)
    }
  })
})
