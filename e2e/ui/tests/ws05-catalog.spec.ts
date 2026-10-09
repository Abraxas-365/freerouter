import type { APIRequestContext, Page } from "@playwright/test"
import { test, expect, fx, login, adminApi, uniq } from "./helpers"

// WS-05 console: providers (create/edit/delete via real forms) and the model
// catalog browser. Everything is named ws05-* and cleaned up via the admin API.

// Allow time for login retries (see loginAndWait below).
test.describe.configure({ timeout: 120_000 })

const created: { providers: string[]; models: string[] } = { providers: [], models: [] }

test.afterEach(async ({ request }) => {
  const api = adminApi(request)
  for (const id of created.models.splice(0)) await api.delete(`/models/${id}`)
  for (const id of created.providers.splice(0)) await api.delete(`/providers/${id}`)
})

async function findProvider(request: APIRequestContext, name: string) {
  const res = await adminApi(request).get(`/providers?search=${encodeURIComponent(name)}&limit=100`)
  expect(res.status()).toBe(200)
  const body = await res.json()
  return (body.items as any[]).filter((p) => p.name === name)
}

async function apiProvider(request: APIRequestContext, name: string, extra: Record<string, unknown> = {}) {
  const res = await adminApi(request).post("/providers", {
    name, protocol: "openai", base_url: "http://localhost:29100/ws05ui/openai/v1", ...extra,
  })
  expect(res.status()).toBe(201)
  const { id } = await res.json()
  created.providers.push(id)
  return id as string
}

async function apiModel(request: APIRequestContext, name: string, description = "ws05 ui model") {
  const res = await adminApi(request).post("/models", { name, family: "ws05ui", description })
  expect(res.status()).toBe(201)
  const { id } = await res.json()
  created.models.push(id)
  return id as string
}

// Login with retries: IAMKit login is rate-limited per IP and shared by every
// concurrent e2e worker, so a single attempt occasionally bounces.
async function loginAndWait(page: Page, persona: typeof fx.personas.admin) {
  for (let i = 0; i < 4; i++) {
    await login(page, persona)
    try {
      await expect(page).not.toHaveURL(/\/login/, { timeout: 10_000 })
      return
    } catch (e) {
      if (i === 3) throw e
      await page.waitForTimeout(3_000)
    }
  }
}

function row(page: Page, name: string) {
  return page.getByRole("row").filter({ hasText: name })
}

async function openRowMenu(page: Page, name: string) {
  await row(page, name).getByRole("button").last().click()
}

async function gotoProviders(page: Page) {
  await loginAndWait(page, fx.personas.admin)
  await page.goto("/providers")
  await expect(page.getByRole("heading", { name: "Providers" })).toBeVisible()
  await expect(page.getByText("e2e-openai")).toBeVisible()
}

test.describe("WS-05 providers page", () => {
  test("create provider through the form", async ({ page, request }) => {
    await gotoProviders(page)
    const name = uniq("ws05-ui-create")

    await page.getByRole("button", { name: "New provider" }).first().click()
    const dlg = page.getByRole("dialog")
    await expect(dlg.getByText("Register a new upstream provider.")).toBeVisible()
    const createBtn = dlg.getByRole("button", { name: "Create" })

    // Required fields gate the submit button.
    await expect(createBtn).toBeDisabled()
    await dlg.getByLabel("Name").fill(name)
    await expect(createBtn).toBeDisabled()
    await dlg.getByLabel("Base URL").fill("http://localhost:29100/ws05ui/anthropic")
    await expect(createBtn).toBeEnabled()

    await dlg.locator("#p-protocol").click()
    await page.getByRole("option", { name: "Anthropic" }).click()
    await dlg.getByLabel("Description").fill("created by ws05 ui")
    await dlg.getByLabel("Website").fill("https://example.com/ws05")
    await createBtn.click()

    await expect(page.getByText("Provider created")).toBeVisible()
    await expect(dlg).toBeHidden()
    const r = row(page, name)
    await expect(r).toBeVisible()
    await expect(r.getByText("Anthropic")).toBeVisible()
    await expect(r.getByText("created by ws05 ui")).toBeVisible()
    await expect(r.getByText("ACTIVE")).toBeVisible()
    await expect(r.getByText("stream")).toBeVisible() // streaming defaults to on
    await expect(r.locator('a[href="https://example.com/ws05"]')).toBeVisible()

    const found = await findProvider(request, name)
    expect(found).toHaveLength(1)
    created.providers.push(found[0].id)
    expect(found[0]).toMatchObject({
      protocol: "anthropic", base_url: "http://localhost:29100/ws05ui/anthropic",
      description: "created by ws05 ui", website: "https://example.com/ws05", streaming: true, status: "active",
    })
  })

  test("cancel discards the form", async ({ page, request }) => {
    await gotoProviders(page)
    const name = uniq("ws05-ui-cancel")
    await page.getByRole("button", { name: "New provider" }).first().click()
    const dlg = page.getByRole("dialog")
    await dlg.getByLabel("Name").fill(name)
    await dlg.getByLabel("Base URL").fill("http://x")
    await dlg.getByRole("button", { name: "Cancel" }).click()
    await expect(dlg).toBeHidden()
    expect(await findProvider(request, name)).toHaveLength(0)

    // Re-opening starts from a clean form.
    await page.getByRole("button", { name: "New provider" }).first().click()
    await expect(page.getByRole("dialog").getByLabel("Name")).toHaveValue("")
  })

  // FINDING WS05-11: a server-side validation error (whitespace-only name passes the
  // client "required" check) leaves the dialog open with no message at all.
  test("server validation error is shown to the user", async ({ page, request }) => {
    await gotoProviders(page)
    await page.getByRole("button", { name: "New provider" }).first().click()
    const dlg = page.getByRole("dialog")
    await dlg.getByLabel("Name").fill("   ")
    await dlg.getByLabel("Base URL").fill("http://localhost:29100/ws05ui/openai/v1")
    const resp = page.waitForResponse((r) => r.url().endsWith("/api/v1/providers") && r.request().method() === "POST")
    await dlg.getByRole("button", { name: "Create" }).click()
    expect((await resp).status()).toBe(400)
    await expect(dlg).toBeVisible()
    await expect(page.getByText("provider name is required")).toBeVisible()
  })

  test("edit provider through the form", async ({ page, request }) => {
    const name = uniq("ws05-ui-edit")
    const id = await apiProvider(request, name, { description: "before", streaming: true, website: "https://example.com" })
    await gotoProviders(page)

    await openRowMenu(page, name)
    await page.getByRole("menuitem", { name: "Edit" }).click()
    const dlg = page.getByRole("dialog")
    await expect(dlg.getByText("Update the provider configuration.")).toBeVisible()
    // Pre-filled with current values.
    await expect(dlg.getByLabel("Name")).toHaveValue(name)
    await expect(dlg.getByLabel("Base URL")).toHaveValue("http://localhost:29100/ws05ui/openai/v1")
    await expect(dlg.getByLabel("Description")).toHaveValue("before")

    // Clearing a required field disables Save.
    await dlg.getByLabel("Base URL").fill("")
    await expect(dlg.getByRole("button", { name: "Save" })).toBeDisabled()
    await dlg.getByLabel("Base URL").fill("http://localhost:29100/ws05ui/v2")

    const renamed = `${name}-renamed ✨`
    await dlg.getByLabel("Name").fill(renamed)
    await dlg.getByLabel("Description").fill("after")
    await dlg.getByRole("switch").click() // streaming off
    await dlg.getByRole("button", { name: "Save" }).click()

    await expect(page.getByText("Provider updated")).toBeVisible()
    const r = row(page, renamed)
    await expect(r.getByText("after")).toBeVisible()
    await expect(r.getByText("stream")).toHaveCount(0)

    const p = await (await adminApi(request).get(`/providers/${id}`)).json()
    expect(p).toMatchObject({ name: renamed, description: "after", base_url: "http://localhost:29100/ws05ui/v2", streaming: false })
  })

  test("delete asks for confirmation; cancel keeps, confirm removes", async ({ page, request }) => {
    const name = uniq("ws05-ui-del")
    const id = await apiProvider(request, name)
    await gotoProviders(page)

    await openRowMenu(page, name)
    await page.getByRole("menuitem", { name: "Delete" }).click()
    const dlg = page.getByRole("dialog")
    await expect(dlg.getByText("Delete provider")).toBeVisible()
    await expect(dlg.getByText(/permanently remove the provider/)).toBeVisible()
    await dlg.getByRole("button", { name: "Cancel" }).click()
    await expect(dlg).toBeHidden()
    await expect(row(page, name)).toBeVisible()
    expect((await adminApi(request).get(`/providers/${id}`)).status()).toBe(200)

    await openRowMenu(page, name)
    await page.getByRole("menuitem", { name: "Delete" }).click()
    await page.getByRole("dialog").getByRole("button", { name: "Delete" }).click()
    await expect(page.getByText("Provider deleted")).toBeVisible()
    await expect(row(page, name)).toHaveCount(0)
    expect((await adminApi(request).get(`/providers/${id}`)).status()).toBe(404)
  })

  test("long and unicode names render without breaking the table", async ({ page, request }) => {
    const name = `ws05-ui-ünï-🚀-${"x".repeat(120)}-${Date.now()}`
    await apiProvider(request, name, { description: "d".repeat(500) })
    await gotoProviders(page)
    await expect(row(page, name)).toBeVisible()
    // Actions menu must remain reachable.
    await openRowMenu(page, name)
    await expect(page.getByRole("menuitem", { name: "Edit" })).toBeVisible()
  })

  test("empty state when there are no providers", async ({ page }) => {
    await loginAndWait(page, fx.personas.admin)
    await page.route("**/api/v1/providers?*", (route) =>
      route.fulfill({ json: { items: [], page: { total: 0, limit: 100, offset: 0 } } }),
    )
    await page.goto("/providers")
    await expect(page.getByText("No providers", { exact: true })).toBeVisible()
    await expect(page.getByText("Add a provider to start routing requests.")).toBeVisible()
    await expect(page.getByText("0 total")).toBeVisible()
    await page.getByRole("button", { name: "New provider" }).last().click()
    await expect(page.getByRole("dialog").getByText("Register a new upstream provider.")).toBeVisible()
  })
})

test.describe("WS-05 models page", () => {
  test("model with mappings shows providers, pricing and capabilities", async ({ page, request }) => {
    const pname = uniq("ws05-ui-mprov")
    const pid = await apiProvider(request, pname)
    const mname = uniq("ws05-ui-model")
    const mid = await apiModel(request, mname, "ws05 described model")
    const mres = await adminApi(request).post("/mappings", {
      model_id: mid, provider_id: pid, external_id: "fake-ok-ws05", input_price: 1.25, output_price: 2.5,
      context_size: 128000, max_output: 8000, streaming: true, tools: true,
    })
    expect(mres.status()).toBe(201)

    await loginAndWait(page, fx.personas.admin)
    await page.goto("/models")
    await expect(page.getByRole("heading", { name: "Models" })).toBeVisible()
    const card = page.locator("[data-slot=card]").filter({ hasText: mname })
    await expect(card).toBeVisible()
    await expect(card.getByText("1 provider", { exact: true })).toBeVisible()
    await expect(card.getByText(pname)).toBeVisible()
    await expect(card.getByText("from $1.25/M in")).toBeVisible()
    await expect(card.getByText("128K ctx")).toBeVisible()
    await expect(card.getByText("stream")).toBeVisible()
    await expect(card.getByText("tools")).toBeVisible()

    await card.click()
    const dlg = page.getByRole("dialog")
    await expect(dlg.getByText("ws05 described model")).toBeVisible()
    await expect(dlg.getByText("Providers (1)")).toBeVisible()
    await expect(dlg.getByText("fake-ok-ws05")).toBeVisible()
    await expect(dlg.getByText("$1.25/M")).toBeVisible()
    await expect(dlg.getByText("$2.5/M")).toBeVisible()
    await expect(dlg.getByText("max out: 8K")).toBeVisible()
  })

  test("model without mappings and inactive model", async ({ page, request }) => {
    const mname = uniq("ws05-ui-lonely")
    const mid = await apiModel(request, mname)
    expect((await adminApi(request).put(`/models/${mid}`, { status: "inactive", stability: "experimental" })).status()).toBe(204)

    await loginAndWait(page, fx.personas.admin)
    await page.goto("/models")
    const card = page.locator("[data-slot=card]").filter({ hasText: mname })
    await expect(card.getByText("0 providers")).toBeVisible()
    await expect(card.getByText("INACTIVE")).toBeVisible()
    await expect(card.getByText("experimental")).toBeVisible()
    await card.click()
    await expect(page.getByRole("dialog").getByText("No providers connected to this model.")).toBeVisible()
  })

  test("catalog changes appear after reload; deleted provider disappears from model", async ({ page, request }) => {
    const pname = uniq("ws05-ui-gone")
    const pid = await apiProvider(request, pname)
    const mname = uniq("ws05-ui-refresh")
    const mid = await apiModel(request, mname)
    expect((await adminApi(request).post("/mappings", { model_id: mid, provider_id: pid, external_id: "fake-ok" })).status()).toBe(201)

    await loginAndWait(page, fx.personas.admin)
    await page.goto("/models")
    const card = page.locator("[data-slot=card]").filter({ hasText: mname })
    await expect(card.getByText(pname)).toBeVisible()

    expect((await adminApi(request).delete(`/providers/${pid}`)).status()).toBe(204)
    await page.reload()
    await expect(card.getByText("0 providers")).toBeVisible()
    await expect(card.getByText(pname)).toHaveCount(0)
  })

  test("empty state when the catalog has no models", async ({ page }) => {
    await loginAndWait(page, fx.personas.admin)
    await page.route("**/api/v1/models?*", (route) =>
      route.fulfill({ json: { items: [], page: { total: 0, limit: 100, offset: 0 } } }),
    )
    await page.goto("/models")
    await expect(page.getByText("No models", { exact: true })).toBeVisible()
    await expect(page.getByText("No models have been configured by the operator.")).toBeVisible()
  })

  // FINDING WS05-8: the console has no way to create/edit/delete models,
  // mappings or fallbacks, and no search on either catalog page.
  test("models page offers catalog management", async ({ page }) => {
    await loginAndWait(page, fx.personas.admin)
    await page.goto("/models")
    await expect(page.getByRole("heading", { name: "Models" })).toBeVisible()
    await expect(page.getByRole("button", { name: /new model|add model|create/i })).toBeVisible()
  })
})
