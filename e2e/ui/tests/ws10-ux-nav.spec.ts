import { test, expect, openAs, uiLogin, fx, PAGES, heading, shot, retryThrottled } from "./ws10-helpers"
import { stubApi } from "./ws10-stub"

// WS-10: navigation (back/forward/refresh), mobile viewport, theme, and a
// mis-configured web/.env (wrong IAMKit environment id) on the login page.

test.describe.configure({ timeout: 120_000 })

test.describe("WS-10 navigation", () => {
  test.use({ viewport: { width: 1280, height: 800 } })

  test("sidebar navigation + browser back/forward keep the right page", async ({ page }) => {
    await retryThrottled(page)
    await openAs(page, "admin", "/")
    await expect(heading(page, "Dashboard")).toBeVisible()
    for (const [link, h] of [["Providers", "Providers"], ["Rate Limits", "Rate Limits"], ["Webhooks", "Webhooks"]]) {
      await page.getByRole("link", { name: link, exact: true }).click()
      await expect(heading(page, h)).toBeVisible()
    }
    await page.goBack()
    await expect(page).toHaveURL(/\/rate-limits$/)
    await expect(heading(page, "Rate Limits")).toBeVisible()
    await page.goBack()
    await expect(heading(page, "Providers")).toBeVisible()
    await page.goForward()
    await expect(heading(page, "Rate Limits")).toBeVisible()
    // The active nav item follows history navigation.
    await expect(page.locator("[data-sidebar=menu-button][data-active]").filter({ hasText: "Rate Limits" })).toHaveCount(1)
  })

  for (const p of PAGES) {
    test(`refresh on ${p.path} stays logged in on the same page`, async ({ page }) => {
      await retryThrottled(page) // IAMKit 120 req/min throttle -> 502 (known WS04-4)
      await openAs(page, "admin", p.path)
      await expect(heading(page, p.heading)).toBeVisible()
      await page.reload()
      await expect(page).toHaveURL(new RegExp(`${p.path === "/" ? "/$" : p.path + "$"}`))
      await expect(heading(page, p.heading)).toBeVisible()
    })
  }

  test("refresh mid-form: dialog closes cleanly, nothing is created, page usable", async ({ page, request }) => {
    await openAs(page, "admin", "/providers")
    const name = `ws10-midform-${Date.now()}`
    await page.getByRole("button", { name: "New provider" }).click()
    const dlg = page.getByRole("dialog", { name: "New provider" })
    await dlg.getByLabel("Name", { exact: true }).fill(name)
    await page.reload()
    await expect(heading(page, "Providers")).toBeVisible()
    await expect(page.getByRole("dialog")).toHaveCount(0)
    await expect(page.getByRole("button", { name: "New provider" })).toBeEnabled()
    const r = await request.get(`${fx.urls.api}/providers?limit=100&search=ws10-midform`, {
      headers: { Authorization: `Bearer ${fx.personas.admin_key.secret}` },
    })
    expect((await r.json()).items.filter((x: { name: string }) => x.name === name)).toHaveLength(0)
  })

  test("unknown route shows something readable (not a blank page)", async ({ page }) => {
    await openAs(page, "admin", "/")
    await page.goto("/ws10-does-not-exist")
    const text = (await page.locator("body").innerText()).trim()
    if (!text) await shot(page, "unknown-route-blank")
    expect(text.length, "body text on an unknown route").toBeGreaterThan(0)
  })

  test("theme toggle (N/A: console is dark-only, no toggle rendered)", async ({ page }) => {
    await openAs(page, "admin", "/")
    await expect(heading(page, "Dashboard")).toBeVisible()
    await expect(page.getByRole("button", { name: /theme|dark|light/i })).toHaveCount(0)
  })

  test("real UI login form lands on the dashboard (no token injection)", async ({ page }) => {
    await uiLogin(page, fx.personas.admin)
    await expect(heading(page, "Dashboard")).toBeVisible()
  })
})

test.describe("WS-10 mis-set web/.env (wrong IAMKit environment id)", () => {
  test.use({ viewport: { width: 1280, height: 800 } })

  // Exactly what IAMKit returns (checked with curl) for a wrong environment_id.
  const CASES = [
    { label: "unknown UUID env id -> 401", status: 401, body: { error: { code: "AUTHORIZATION", message: "invalid credentials or access token", type: "AUTHORIZATION", http_status: 401 } } },
    { label: "malformed env id -> 400", status: 400, body: { error: { code: "VALIDATION", message: "invalid request", type: "VALIDATION", http_status: 400 } } },
    { label: "wrong IAMKit URL -> 404", status: 404, body: "404 page not found" },
  ]
  for (const c of CASES) {
    test(`login page shows a readable error: ${c.label}`, async ({ page }) => {
      await page.route(/\/identity\/v1\/login$/, (r) => typeof c.body === "string"
        ? r.fulfill({ status: c.status, contentType: "text/plain", body: c.body })
        : r.fulfill({ status: c.status, json: c.body }))
      await page.goto("/login")
      await page.locator("#email").fill(fx.personas.admin.email!)
      await page.locator("#password").fill("whatever-ws10")
      await page.getByRole("button", { name: "Sign in" }).click()
      await expect(page).toHaveURL(/\/login/)
      const err = page.locator("form p.text-destructive")
      await expect(err).toBeVisible()
      const text = (await err.innerText()).trim()
      // Readable = non-empty and not an empty HTTP reason phrase only; it is fine
      // if it says the credentials are invalid (WS01-1 covers the wording).
      if (!text) await shot(page, `WS10-8-login-misset-env-${c.status}`)
      expect(text.length).toBeGreaterThan(0)
      await expect(page.getByRole("button", { name: "Sign in" })).toBeEnabled()
    })
  }

  test("IAMKit unreachable: login shows a readable error, button re-enabled", async ({ page }) => {
    await page.route(/\/identity\/v1\/login$/, (r) => r.abort("connectionrefused"))
    await page.goto("/login")
    await page.locator("#email").fill(fx.personas.admin.email!)
    await page.locator("#password").fill("whatever-ws10")
    await page.getByRole("button", { name: "Sign in" }).click()
    const err = page.locator("form p.text-destructive")
    await expect(err).toBeVisible()
    const text = (await err.innerText()).trim()
    if (/^Failed to fetch$/i.test(text)) await shot(page, "WS10-8-login-iamkit-unreachable")
    // "Failed to fetch" is the raw browser exception: it says nothing to a human.
    expect(text, "login error when IAMKit is unreachable").not.toMatch(/^Failed to fetch$/i)
    await expect(page.getByRole("button", { name: "Sign in" })).toBeEnabled()
  })
})

test.describe("WS-10 mobile 375x812", () => {
  test.use({ viewport: { width: 375, height: 812 }, hasTouch: true, isMobile: true })

  test("sidebar opens as a sheet, navigates, and closes", async ({ page }) => {
    await openAs(page, "admin", "/login")
    await stubApi(page)
    await page.goto("/")
    await expect(heading(page, "Dashboard")).toBeVisible()
    await expect(page.getByRole("link", { name: "Providers", exact: true })).toBeHidden()
    await page.getByRole("button", { name: "Toggle Sidebar" }).click()
    const sheet = page.getByRole("dialog")
    await expect(sheet).toBeVisible()
    await sheet.getByRole("link", { name: "Webhooks", exact: true }).click()
    await expect(heading(page, "Webhooks")).toBeVisible()
    // The sheet must close after navigating, otherwise the page stays covered.
    try {
      await expect(page.locator("[data-mobile=true]")).toBeHidden({ timeout: 3_000 })
    } catch (e) {
      await shot(page, "WS10-9-mobile-sheet-stays-open")
      throw e
    }
  })

  for (const p of PAGES) {
    test(`${p.path}: no horizontal page overflow; wide tables scroll inside their container`, async ({ page }) => {
      await openAs(page, "admin", "/login")
      await stubApi(page)
      await page.goto(p.path)
      await expect(heading(page, p.heading)).toBeVisible()
      await expect(page.locator("[data-slot=skeleton]")).toHaveCount(0)
      const overflow = await page.evaluate(() => document.documentElement.scrollWidth - document.documentElement.clientWidth)
      if (overflow > 0) await shot(page, `mobile-overflow${p.path === "/" ? "_dashboard" : p.path.replace(/\//g, "_")}`)
      expect(overflow, `${p.path} page is wider than the 375px viewport by`).toBeLessThanOrEqual(0)
      // Primary action buttons in the header are reachable on screen.
      for (const b of await page.locator("main header ~ * button, main > div button").filter({ hasText: /New|Add|Create/ }).all()) {
        if (!(await b.isVisible())) continue
        const box = (await b.boundingBox())!
        expect(box.x + box.width, "action button clipped off the right edge").toBeLessThanOrEqual(375)
      }
    })
  }

  for (const d of [
    { path: "/providers", open: "New provider", title: "New provider" },
    { path: "/service-accounts", open: "Create Service Account", title: "Create Service Account" },
    { path: "/access", open: "Create Role", title: "Create Role", tab: "Roles" },
  ]) {
    test(`${d.path}: "${d.title}" dialog fits the viewport and its buttons are reachable`, async ({ page }) => {
      await openAs(page, "admin", "/login")
      await stubApi(page)
      await page.goto(d.path)
      await expect(page.locator("[data-slot=skeleton]")).toHaveCount(0)
      if (d.tab) await page.getByRole("tab", { name: new RegExp(d.tab) }).click()
      await page.getByRole("button", { name: d.open }).first().click()
      const dlg = page.getByRole("dialog", { name: d.title })
      await expect(dlg).toBeVisible()
      const box = (await dlg.boundingBox())!
      expect(box.x).toBeGreaterThanOrEqual(0)
      expect(box.x + box.width).toBeLessThanOrEqual(375)
      const submit = dlg.getByRole("button", { name: "Create" })
      await submit.scrollIntoViewIfNeeded()
      const sb = (await submit.boundingBox())!
      if (sb.y + sb.height > 812 || sb.y < 0) await shot(page, `mobile-dialog${d.path.replace(/\//g, "_")}`)
      expect(sb.y + sb.height, "submit button below the fold and unreachable").toBeLessThanOrEqual(812)
      expect(sb.y).toBeGreaterThanOrEqual(0)
    })
  }
})
