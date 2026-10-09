import { test, expect as baseExpect, fx, login, adminApi, uniq, type Persona } from "./helpers"
import type { APIRequestContext, Page } from "@playwright/test"

// WS-04: service accounts console page (/service-accounts).

// Long expect timeout: API calls may be held back while IAMKit throttles (see retryThrottled).
const expect = baseExpect.configure({ timeout: 90_000 })
test.describe.configure({ timeout: 300_000 })

type Account = { id: string; name: string; permissions: string[]; expires_in?: string | null }

// IAMKit throttles /api/v1 at 120 req/min per IP (shared by every e2e worker);
// FreeRouter surfaces that as 502 "... in IAMKit" (FINDING WS04-4). Retry those.
async function retry502<T extends { status(): number }>(fn: () => Promise<T>): Promise<T> {
  for (let i = 0; ; i++) {
    const r = await fn()
    if (r.status() !== 502 || i >= 30) return r
    await new Promise((res) => setTimeout(res, 3000))
  }
}

async function listAccounts(request: APIRequestContext): Promise<Account[]> {
  const r = await retry502(() => adminApi(request).get("/service-accounts"))
  expect(r.status()).toBe(200)
  return r.json()
}

// Transparently retry console API calls that FreeRouter failed with 502 because
// IAMKit throttled it (429, shared budget). A throttled call created nothing in
// IAMKit, so replaying it is safe. Product behaviour on 502 is covered separately.
async function retryThrottled(page: Page) {
  await page.route(`${fx.urls.api}/service-accounts**`, async (route) => {
    try {
      let resp = await route.fetch()
      for (let i = 0; i < 30 && resp.status() === 502; i++) {
        await new Promise((res) => setTimeout(res, 3000))
        resp = await route.fetch()
      }
      await route.fulfill({ response: resp })
    } catch { /* page closed while retrying */ }
  })
}

// loginAndWait that retries while IAMKit answers "Too Many Requests" (shared per-IP budget).
async function loginAndWait(page: Page, persona: Persona) {
  await retryThrottled(page)
  for (let i = 0; i < 12; i++) {
    await login(page, persona)
    try {
      await expect(page).not.toHaveURL(/\/login/, { timeout: 5_000 })
      return
    } catch {
      await expect(page.getByText("Too Many Requests")).toBeVisible({ timeout: 1_000 })
      await page.waitForTimeout(5_000)
    }
  }
  await expect(page).not.toHaveURL(/\/login/)
}

// Wait until the service-accounts table rendered (seeded accounts always exist).
async function waitForTable(page: Page) {
  await expect(page.getByRole("cell", { name: "e2e-gateway-only", exact: true })).toBeVisible()
}

// Revoke every ws04-ui-* account a test created (by name prefix of that test).
async function cleanup(request: APIRequestContext, prefix: string) {
  for (const a of await listAccounts(request)) {
    if (a.name.startsWith(prefix)) await retry502(() => adminApi(request).delete(`/service-accounts/${a.id}`))
  }
}

async function openPage(page: Page) {
  await loginAndWait(page, fx.personas.admin)
  await page.goto("/service-accounts")
  await expect(page.getByRole("heading", { name: "Service Accounts" })).toBeVisible()
  await waitForTable(page) // seeded accounts always exist
}

const banner = (page: Page) => page.locator("div.rounded-md", { hasText: "Service Account Created" })

async function fillCreate(page: Page, name: string, opts: { perms?: string[]; expires?: string } = {}) {
  await page.getByRole("button", { name: "Create Service Account" }).first().click()
  const dlg = page.getByRole("dialog")
  await expect(dlg.getByRole("heading", { name: "Create Service Account" })).toBeVisible()
  await dlg.getByPlaceholder("e.g. production-backend").fill(name)
  if (opts.perms) {
    // start from the default (gateway:invoke) and toggle to the wanted set
    for (const p of fx.permissions) {
      const box = dlg.getByRole("checkbox", { name: p, exact: true })
      if ((await box.isChecked()) !== opts.perms.includes(p)) await box.click()
    }
  }
  if (opts.expires !== undefined) await dlg.getByPlaceholder(/8760h/).fill(opts.expires)
  return dlg
}

test.describe("WS-04 service accounts console", () => {
  test("create via form: secret banner shown once, copy works, key works, gone after reload", async ({ page, context, request }) => {
    const prefix = uniq("ws04-ui-create")
    try {
      await context.grantPermissions(["clipboard-read", "clipboard-write"])
      await openPage(page)
      const dlg = await fillCreate(page, prefix, { perms: ["freerouter:gateway:invoke", "freerouter:providers:read"], expires: "720h" })
      await dlg.getByRole("button", { name: "Create", exact: true }).click()
      await expect(dlg).toBeHidden()
      await expect(page.getByText("Service account created", { exact: true })).toBeVisible()

      const b = banner(page)
      await expect(b).toBeVisible()
      await expect(b.getByText("it won't be shown again")).toBeVisible()
      const secret = (await b.locator("code.select-all").innerText()).trim()
      expect(secret).toMatch(/^ik_svc_[A-Za-z0-9_-]{20,}$/)

      // copy button (first button in the banner) puts the secret on the clipboard
      await b.getByRole("button").first().click()
      expect(await page.evaluate(() => navigator.clipboard.readText())).toBe(secret)

      // new row with the chosen permissions
      const row = page.getByRole("row", { name: new RegExp(prefix) })
      await expect(row).toBeVisible()
      await expect(row.getByText("freerouter:providers:read")).toBeVisible()
      await expect(row.getByText("freerouter:gateway:invoke")).toBeVisible()

      // the secret really works with exactly those permissions
      const auth = { Authorization: `Bearer ${secret}` }
      const pre = await request.get(fx.urls.gateway + "/models", { headers: auth })
      expect(pre.status(), await pre.text()).toBe(200)
      expect((await request.get(fx.urls.api + "/providers", { headers: auth })).status()).toBe(200)
      expect((await request.get(fx.urls.api + "/service-accounts", { headers: auth })).status()).toBe(403)

      // secret never appears in the list API nor after reload
      const listBody = await (await retry502(() => adminApi(request).get("/service-accounts"))).text()
      expect(listBody).not.toContain(secret)
      await page.reload()
      await waitForTable(page)
      await expect(page.getByRole("row", { name: new RegExp(prefix) })).toBeVisible()
      await expect(banner(page)).toHaveCount(0)
      await expect(page.getByText(secret)).toHaveCount(0)
    } finally {
      await cleanup(request, prefix)
    }
  })

  test("dismiss hides the banner", async ({ page, request }) => {
    const prefix = uniq("ws04-ui-dismiss")
    try {
      await openPage(page)
      const dlg = await fillCreate(page, prefix)
      await dlg.getByRole("button", { name: "Create", exact: true }).click()
      await expect(banner(page)).toBeVisible()
      await banner(page).getByRole("button", { name: "Dismiss" }).click()
      await expect(banner(page)).toHaveCount(0)
      const row = page.getByRole("row", { name: new RegExp(prefix) })
      await expect(row.getByText("freerouter:gateway:invoke")).toBeVisible() // default permission
    } finally {
      await cleanup(request, prefix)
    }
  })

  test("Create is disabled for empty / whitespace name; cancel creates nothing", async ({ page, request }) => {
    const prefix = uniq("ws04-ui-cancel")
    await openPage(page)
    const dlg = await fillCreate(page, "")
    const create = dlg.getByRole("button", { name: "Create", exact: true })
    await expect(create).toBeDisabled()
    await dlg.getByPlaceholder("e.g. production-backend").fill("    ")
    await expect(create).toBeDisabled()
    await dlg.getByPlaceholder("e.g. production-backend").fill(prefix)
    await expect(create).toBeEnabled()
    await dlg.getByRole("button", { name: "Cancel" }).click()
    await expect(dlg).toBeHidden()
    expect((await listAccounts(request)).filter((a) => a.name.startsWith(prefix))).toHaveLength(0)
    // reopening resets the form
    await page.getByRole("button", { name: "Create Service Account" }).first().click()
    await expect(page.getByRole("dialog").getByPlaceholder("e.g. production-backend")).toHaveValue("")
  })

  test("invalid expiry shows the server error and creates nothing", async ({ page, request }) => {
    const prefix = uniq("ws04-ui-badexp")
    try {
      await openPage(page)
      const dlg = await fillCreate(page, prefix, { expires: "59m" })
      await dlg.getByRole("button", { name: "Create", exact: true }).click()
      expect((await listAccounts(request)).filter((a) => a.name.startsWith(prefix))).toHaveLength(0)
      await expect(page.getByRole("dialog")).toBeVisible() // form kept for correction
      await expect(banner(page)).toHaveCount(0)
      // A human must be told why nothing happened (FINDING WS04-5 when absent).
      await expect(page.getByText(/expires_in must be at least 1h/)).toBeVisible({ timeout: 10_000 })
    } finally {
      await cleanup(request, prefix)
    }
  })

  test("expiry column reflects the chosen expiry", async ({ page, request }) => {
    const prefix = uniq("ws04-ui-exp")
    try {
      await openPage(page)
      const dlg = await fillCreate(page, prefix, { expires: "1h" })
      await dlg.getByRole("button", { name: "Create", exact: true }).click()
      const row = page.getByRole("row", { name: new RegExp(prefix) })
      await expect(row).toBeVisible()
      // FINDING WS04-2: list carries no expiry, column shows "never" for a 1h key.
      await expect(row.getByRole("cell", { name: "never", exact: true })).toHaveCount(0, { timeout: 5_000 })
    } finally {
      await cleanup(request, prefix)
    }
  })

  test("revoke flow: cancel keeps, confirm removes and key stops working", async ({ page, request }) => {
    const prefix = uniq("ws04-ui-revoke")
    const created = await retry502(() => adminApi(request).post("/service-accounts", { name: prefix }))
    expect(created.status()).toBe(201)
    const { secret } = await created.json()
    try {
      await openPage(page)
      const row = page.getByRole("row", { name: new RegExp(prefix) })
      await expect(row).toBeVisible()

      await row.getByRole("button").click()
      const confirm = page.getByRole("dialog")
      await expect(confirm.getByRole("heading", { name: "Revoke Service Account" })).toBeVisible()
      await expect(confirm.getByText(/immediately lose access/)).toBeVisible()
      await confirm.getByRole("button", { name: "Cancel" }).click()
      await expect(confirm).toBeHidden()
      await expect(row).toBeVisible()
      const auth = { Authorization: `Bearer ${secret}` }
      const pre = await request.get(fx.urls.gateway + "/models", { headers: auth })
      expect(pre.status(), await pre.text()).toBe(200)

      await row.getByRole("button").click()
      await page.getByRole("dialog").getByRole("button", { name: "Revoke" }).click()
      await expect(page.getByText("Service account revoked", { exact: true })).toBeVisible()
      await expect(page.getByRole("row", { name: new RegExp(prefix) })).toHaveCount(0)
      const post = await request.get(fx.urls.gateway + "/models", { headers: auth })
      expect(post.status(), await post.text()).toBe(401)
      await page.reload()
      await waitForTable(page)
      await expect(page.getByRole("row", { name: new RegExp(prefix) })).toHaveCount(0)
    } finally {
      await cleanup(request, prefix)
    }
  })

  test("backend service account is never shown", async ({ page }) => {
    await openPage(page)
    await expect(page.getByText("freerouter-backend")).toHaveCount(0)
    await expect(page.getByText(fx.boundary.backend_service_account_id)).toHaveCount(0)
    // only FreeRouter permissions are rendered
    await expect(page.getByText(/^iam:/)).toHaveCount(0)
  })

  test("legacy /api-keys redirects to /service-accounts", async ({ page }) => {
    await loginAndWait(page, fx.personas.admin)
    await page.goto("/api-keys")
    await expect(page).toHaveURL(/\/service-accounts$/)
    await expect(page.getByRole("heading", { name: "Service Accounts" })).toBeVisible()
  })

  test("nav link opens the page; viewer can see the list", async ({ page }) => {
    await loginAndWait(page, fx.personas.viewer)
    await page.getByRole("link", { name: "Service Accounts" }).click()
    await expect(page).toHaveURL(/\/service-accounts$/)
    await waitForTable(page)
  })

  test("user without service-accounts permission cannot open the page", async ({ page }) => {
    await loginAndWait(page, fx.personas.providers_only)
    await expect(page.getByRole("link", { name: "Providers" }).first()).toBeVisible()
    await expect(page.getByRole("link", { name: "Service Accounts" })).toHaveCount(0)
    await page.goto("/service-accounts")
    await expect(page).toHaveURL(new RegExp(`^${fx.urls.web}/?$`)) // RequirePermission → dashboard
    await expect(page.getByRole("heading", { name: "Service Accounts" })).toHaveCount(0)
    await expect(page.getByRole("cell", { name: "e2e-gateway-only", exact: true })).toHaveCount(0)
    await expect(page.getByRole("button", { name: "Create Service Account" })).toHaveCount(0)
  })
})
