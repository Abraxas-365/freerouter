import { execFileSync } from "node:child_process"
import type { Page } from "@playwright/test"
import { test, expect, fx } from "./helpers"

// WS-12 — console behaviour while IAMKit is down (docker compose project
// frv2e2e). DESTRUCTIVE for concurrent workers: run alone.
//   cd e2e/ui && WS=ws12 npx playwright test tests/ws12-outage.spec.ts

test.describe.configure({ timeout: 180_000 })

const compose = (...args: string[]) =>
  execFileSync("docker", ["compose", "-p", "frv2e2e", ...args], { stdio: "pipe" }).toString()

const health = () =>
  execFileSync("docker", ["inspect", "-f", "{{.State.Health.Status}}", "frv2e2e-iamkit-1"]).toString().trim()

async function ensureIamkitUp() {
  if (health() === "healthy") return
  compose("start", "iamkit")
  for (let i = 0; i < 120; i++) {
    if (health() === "healthy") return
    await new Promise((r) => setTimeout(r, 1000))
  }
  throw new Error("iamkit did not become healthy")
}

/** Logs in as admin through the form, retrying IAMKit's shared 429 limiter. */
async function loginAdmin(page: Page) {
  for (let attempt = 0; attempt < 12; attempt++) {
    await page.goto("/login")
    await page.locator("#email").fill(fx.personas.admin.email!)
    await page.locator("#password").fill(fx.personas.admin.password!)
    const res = page.waitForResponse((r) => r.url().endsWith("/identity/v1/login") && r.request().method() === "POST")
    await page.getByRole("button", { name: "Sign in" }).click()
    const status = (await res).status()
    if (status === 429) { await page.waitForTimeout(5_000); continue }
    expect(status).toBe(200)
    await expect(page).not.toHaveURL(/\/login/, { timeout: 10_000 })
    return
  }
  throw new Error("login still rate limited")
}

test.afterEach(async () => { await ensureIamkitUp() })
test.afterAll(async () => { await ensureIamkitUp() })

test("logged-in admin reloads /providers while IAMKit is down: error shown, session kept", async ({ page }) => {
  await loginAdmin(page)
  await page.goto("/providers")
  await expect(page.getByText("e2e-openai")).toBeVisible()

  compose("stop", "iamkit")
  const apiStatuses: number[] = []
  page.on("response", (r) => { if (r.url().includes("/api/v1/")) apiStatuses.push(r.status()) })
  const started = Date.now()
  await page.reload()

  // Settles within 15s: either the list, the login page, or an error — never an endless skeleton.
  await expect
    .poll(async () => {
      if (/\/login/.test(page.url())) return "login"
      if (await page.getByText("e2e-openai").isVisible()) return "list"
      if (await page.getByText(/error|unavailable|failed|try again/i).first().isVisible()) return "error"
      return "loading"
    }, { timeout: 15_000 })
    .not.toBe("loading")
  const settledMs = Date.now() - started
  const storage = await page.evaluate(() => ({ access: !!localStorage.getItem("access_token"), refresh: !!localStorage.getItem("refresh_token") }))
  const outcome = /\/login/.test(page.url()) ? "redirected to /login" : "stayed on /providers"
  console.log(`[ws12] IAMKit down reload: ${outcome} after ${settledMs}ms; api statuses ${JSON.stringify(apiStatuses)}; storage ${JSON.stringify(storage)}`)
  await page.screenshot({ path: test.info().outputPath("iamkit-down-providers.png"), fullPage: true })

  // Expected (plan WS-01): an outage is not a logout — the UI shows an error and keeps the session.
  // FINDING WS12-5: the console treats the 401 as an expired session, the refresh
  // against IAMKit fails (connection refused), and it silently logs the user out.
  expect.soft(storage.refresh, "FINDING WS12-5: refresh token wiped by an IAMKit outage (silent logout)").toBe(true)
  await expect.soft(page, "FINDING WS12-5: IAMKit outage redirects to /login instead of showing an error").not.toHaveURL(/\/login/)

  // Recovery: once IAMKit is back the console works again (after re-login if the session was dropped).
  await ensureIamkitUp()
  if (/\/login/.test(page.url())) await loginAdmin(page)
  await page.goto("/providers")
  await expect(page.getByText("e2e-openai")).toBeVisible({ timeout: 15_000 })
})

test("login page with IAMKit down shows a readable error, then works after recovery", async ({ page }) => {
  await page.goto("/login")
  compose("stop", "iamkit")
  await page.locator("#email").fill(fx.personas.admin.email!)
  await page.locator("#password").fill(fx.personas.admin.password!)
  const started = Date.now()
  await page.getByRole("button", { name: "Sign in" }).click()
  const err = page.locator("p.text-destructive")
  await expect(err).toBeVisible({ timeout: 10_000 })
  const text = (await err.textContent())?.trim() ?? ""
  console.log(`[ws12] login with IAMKit down: "${text}" after ${Date.now() - started}ms`)
  await expect(page).toHaveURL(/\/login/)
  // "Failed to fetch" / "Load failed" is the raw browser error — not a message a user can act on.
  expect.soft(text, "FINDING WS12-6: login error with IAMKit down is a raw network error").not.toMatch(/failed to fetch|load failed|networkerror/i)

  await ensureIamkitUp()
  await loginAdmin(page)
  await page.goto("/providers")
  await expect(page.getByText("e2e-openai")).toBeVisible()
})
