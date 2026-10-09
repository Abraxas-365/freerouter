import type { APIRequestContext, Page } from "@playwright/test"
import { test, expect, fx, login, adminApi, uniq } from "./helpers"

// WS-06 console: provider keys page — create (api_key + oauth), masked display,
// edit/rotate, activate/deactivate, delete with confirmation, validation.
// Each test owns a ws06ui-* provider (fakellm /ws06ui/...) whose keys it
// creates; the provider (cascading its keys) is deleted afterwards.

test.describe.configure({ timeout: 120_000 })

const created: { providers: string[] } = { providers: [] }

test.afterEach(async ({ request }) => {
  const api = adminApi(request)
  for (const id of created.providers.splice(0)) await api.delete(`/providers/${id}`)
})

async function apiProvider(request: APIRequestContext, name: string) {
  const res = await adminApi(request).post("/providers", {
    name, protocol: "openai", base_url: "http://localhost:29100/ws06ui/openai/v1",
  })
  expect(res.status()).toBe(201)
  const { id } = await res.json()
  created.providers.push(id)
  return id as string
}

async function apiKey(request: APIRequestContext, body: Record<string, unknown>) {
  const res = await adminApi(request).post("/provider-keys", body)
  expect(res.status()).toBe(201)
  return (await res.json()).id as string
}

async function keysOf(request: APIRequestContext, providerId: string) {
  const res = await adminApi(request).get(`/provider-keys?provider_id=${providerId}&limit=100`)
  expect(res.status()).toBe(200)
  return (await res.json()).items as any[]
}

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

async function gotoKeys(page: Page) {
  await loginAndWait(page, fx.personas.admin)
  await page.goto("/provider-keys")
  await expect(page.getByRole("heading", { name: "Provider Keys" })).toBeVisible()
  await expect(page.getByText("e2e-openai-key")).toBeVisible()
}

function row(page: Page, name: string) {
  return page.getByRole("row").filter({ hasText: name })
}

async function rowAction(page: Page, name: string, item: string) {
  await row(page, name).getByRole("button").last().click()
  await page.getByRole("menuitem", { name: item }).click()
}

async function openCreate(page: Page, providerName: string) {
  await page.getByRole("button", { name: "Add Key" }).first().click()
  const dlg = page.getByRole("dialog")
  await expect(dlg.getByText("Add an API key or OAuth credential for a provider.")).toBeVisible()
  await dlg.getByRole("combobox").first().click()
  await page.getByRole("option", { name: providerName, exact: true }).click()
  return dlg
}

const mask = (t: string) => t.length <= 8 ? "*".repeat(t.length) : t.slice(0, 4) + "*".repeat(t.length - 8) + t.slice(-4)

test.describe("WS-06 provider keys page", () => {
  test("create api_key through the form; token shown only masked", async ({ page, request }) => {
    const prov = uniq("ws06ui-prov")
    const pid = await apiProvider(request, prov)
    await gotoKeys(page)
    const name = uniq("ws06ui-key")
    const token = `sk-ws06ui-${Date.now()}-SECRETPART`

    const dlg = await openCreate(page, prov)
    const add = dlg.getByRole("button", { name: "Add Key" })
    await expect(add).toBeDisabled()
    await dlg.getByLabel("Name").fill(name)
    await expect(add).toBeDisabled()
    await dlg.getByLabel("API Token").fill("   ")
    await expect(add).toBeDisabled() // whitespace-only token is not accepted
    await dlg.getByLabel("API Token").fill(token)
    await expect(dlg.getByLabel("API Token")).toHaveAttribute("type", "password")
    await dlg.getByLabel("Description").fill("ws06 ui primary ✨")
    await dlg.getByLabel("Base URL (optional)").fill("http://localhost:29100/ws06ui/override/v1")
    await expect(add).toBeEnabled()
    await add.click()

    await expect(page.getByText("Provider key added")).toBeVisible()
    await expect(dlg).toBeHidden()
    const r = row(page, name)
    await expect(r.getByText("API Key")).toBeVisible()
    await expect(r.getByText(mask(token), { exact: true })).toBeVisible()
    await expect(r.getByText("ws06 ui primary ✨")).toBeVisible()
    await expect(r.getByText("http://localhost:29100/ws06ui/override/v1")).toBeVisible()
    await expect(r.getByText("ACTIVE")).toBeVisible()
    // The plaintext never appears anywhere on the page.
    await expect(page.getByText(token)).toHaveCount(0)
    expect(await page.content()).not.toContain("SECRETPART")
    // Provider card header shows name and count.
    await expect(page.getByText(prov, { exact: true })).toBeVisible()
    await expect(page.getByText(prov, { exact: true }).locator("..").getByText("(1 key)")).toBeVisible()

    const keys = await keysOf(request, pid)
    expect(keys).toHaveLength(1)
    expect(keys[0]).toMatchObject({
      name, key_type: "api_key", token_masked: mask(token), description: "ws06 ui primary ✨",
      base_url: "http://localhost:29100/ws06ui/override/v1", status: "active",
    })
  })

  test("create oauth key through the form", async ({ page, request }) => {
    const prov = uniq("ws06ui-oprov")
    const pid = await apiProvider(request, prov)
    await gotoKeys(page)
    const name = uniq("ws06ui-oauth")

    const dlg = await openCreate(page, prov)
    await dlg.getByRole("combobox").nth(1).click()
    await page.getByRole("option", { name: "OAuth Token" }).click()
    await expect(dlg.getByLabel("API Token")).toHaveCount(0)
    const add = dlg.getByRole("button", { name: "Add Key" })
    await dlg.getByLabel("Name").fill(name)
    await dlg.getByLabel("Access Token").fill("acc-ws06ui-oauth-QRST")
    await dlg.getByLabel("Refresh Token").fill("ref-ws06ui-oauth")
    await expect(add).toBeDisabled() // expiry still missing
    await dlg.getByLabel("Expires At").fill("2031-05-01T12:30")
    await expect(add).toBeEnabled()
    await add.click()

    await expect(page.getByText("Provider key added")).toBeVisible()
    const r = row(page, name)
    await expect(r.getByText("OAuth", { exact: true })).toBeVisible()
    await expect(r.getByText("oauth:****QRST")).toBeVisible()
    await expect(page.getByText("acc-ws06ui-oauth-QRST")).toHaveCount(0)

    const keys = await keysOf(request, pid)
    expect(keys).toHaveLength(1)
    expect(keys[0]).toMatchObject({ name, key_type: "oauth", token_masked: "oauth:****QRST", status: "active" })
  })

  test("cancel discards the form and reopening starts clean", async ({ page, request }) => {
    const prov = uniq("ws06ui-cprov")
    const pid = await apiProvider(request, prov)
    await gotoKeys(page)
    const dlg = await openCreate(page, prov)
    await dlg.getByLabel("Name").fill("ws06ui-cancel")
    await dlg.getByLabel("API Token").fill("sk-ws06ui-cancel")
    await dlg.getByRole("button", { name: "Cancel" }).click()
    await expect(dlg).toBeHidden()
    expect(await keysOf(request, pid)).toHaveLength(0)

    await page.getByRole("button", { name: "Add Key" }).first().click()
    const d2 = page.getByRole("dialog")
    await expect(d2.getByLabel("Name")).toHaveValue("")
    await expect(d2.getByLabel("API Token")).toHaveValue("")
    await expect(d2.getByRole("combobox").first()).not.toContainText(prov)
  })

  // FINDING WS06-5: the console swallows server errors on create — the dialog
  // stays open with no message (here: a non-ASCII token → 500, FINDING WS06-3).
  test("server error on create is shown to the user", async ({ page, request }) => {
    const prov = uniq("ws06ui-eprov")
    await apiProvider(request, prov)
    await gotoKeys(page)
    const dlg = await openCreate(page, prov)
    await dlg.getByLabel("Name").fill(uniq("ws06ui-err"))
    await dlg.getByLabel("API Token").fill("abcé-secret-tokené")
    const resp = page.waitForResponse((r) => r.url().endsWith("/api/v1/provider-keys") && r.request().method() === "POST")
    await dlg.getByRole("button", { name: "Add Key" }).click()
    expect((await resp).status()).toBeGreaterThanOrEqual(400)
    await expect(dlg).toBeVisible()
    await expect(page.getByText(/database error|invalid|failed/i)).toBeVisible()
  })

  test("edit: rename, describe and rotate token", async ({ page, request }) => {
    const prov = uniq("ws06ui-uprov")
    const pid = await apiProvider(request, prov)
    const name = uniq("ws06ui-edit")
    const id = await apiKey(request, { provider_id: pid, name, token: "sk-ws06ui-old-token-1111", description: "before" })
    await gotoKeys(page)
    await expect(row(page, name).getByText(mask("sk-ws06ui-old-token-1111"))).toBeVisible()

    await rowAction(page, name, "Edit")
    const dlg = page.getByRole("dialog")
    await expect(dlg.getByText(`Update key "${name}"`)).toBeVisible()
    await expect(dlg.getByLabel("Name")).toHaveValue(name)
    await expect(dlg.getByLabel("Description")).toHaveValue("before")
    await expect(dlg.getByLabel("Replace token (optional)")).toHaveValue("") // secret never pre-filled
    await dlg.getByLabel("Name").fill("")
    await expect(dlg.getByRole("button", { name: "Save Changes" })).toBeDisabled()

    const renamed = `${name}-renamed`
    await dlg.getByLabel("Name").fill(renamed)
    await dlg.getByLabel("Description").fill("after")
    await dlg.getByLabel("Replace token (optional)").fill("sk-ws06ui-new-token-2222")
    await dlg.getByRole("button", { name: "Save Changes" }).click()

    await expect(page.getByText("Provider key updated")).toBeVisible()
    const r = row(page, renamed)
    await expect(r.getByText(mask("sk-ws06ui-new-token-2222"))).toBeVisible()
    await expect(r.getByText("after")).toBeVisible()
    const k = await (await adminApi(request).get(`/provider-keys/${id}`)).json()
    expect(k).toMatchObject({ name: renamed, description: "after", token_masked: mask("sk-ws06ui-new-token-2222") })
  })

  test("edit without touching the token keeps it", async ({ page, request }) => {
    const prov = uniq("ws06ui-kprov")
    const pid = await apiProvider(request, prov)
    const name = uniq("ws06ui-keep")
    const id = await apiKey(request, { provider_id: pid, name, token: "sk-ws06ui-keep-token-3333" })
    await gotoKeys(page)
    await rowAction(page, name, "Edit")
    const dlg = page.getByRole("dialog")
    await dlg.getByLabel("Description").fill("only desc")
    await dlg.getByRole("button", { name: "Save Changes" }).click()
    await expect(page.getByText("Provider key updated")).toBeVisible()
    const k = await (await adminApi(request).get(`/provider-keys/${id}`)).json()
    expect(k).toMatchObject({ description: "only desc", token_masked: mask("sk-ws06ui-keep-token-3333") })
  })

  // FINDING WS06-4: the edit form sends `base_url: baseUrl || undefined`, so
  // clearing the Base URL field keeps the old override.
  test("edit: clearing Base URL removes the override", async ({ page, request }) => {
    const prov = uniq("ws06ui-bprov")
    const pid = await apiProvider(request, prov)
    const name = uniq("ws06ui-burl")
    const id = await apiKey(request, { provider_id: pid, name, token: "sk-ws06ui-burl-4444", base_url: "http://localhost:29100/ws06ui/ovr/v1" })
    await gotoKeys(page)
    await rowAction(page, name, "Edit")
    const dlg = page.getByRole("dialog")
    await expect(dlg.getByLabel("Base URL")).toHaveValue("http://localhost:29100/ws06ui/ovr/v1")
    await dlg.getByLabel("Base URL").fill("")
    await dlg.getByRole("button", { name: "Save Changes" }).click()
    await expect(page.getByText("Provider key updated")).toBeVisible()
    await expect(row(page, name).getByText("http://localhost:29100/ws06ui/ovr/v1")).toHaveCount(0)
    const k = await (await adminApi(request).get(`/provider-keys/${id}`)).json()
    expect(k.base_url ?? "").toBe("")
  })

  test("deactivate and activate from the row menu", async ({ page, request }) => {
    const prov = uniq("ws06ui-sprov")
    const pid = await apiProvider(request, prov)
    const name = uniq("ws06ui-status")
    const id = await apiKey(request, { provider_id: pid, name, token: "sk-ws06ui-status-5555" })
    await gotoKeys(page)

    await rowAction(page, name, "Deactivate")
    await expect(row(page, name).getByText("INACTIVE")).toBeVisible()
    expect((await (await adminApi(request).get(`/provider-keys/${id}`)).json()).status).toBe("inactive")

    await rowAction(page, name, "Activate")
    await expect(row(page, name).getByText("ACTIVE", { exact: true })).toBeVisible()
    expect((await (await adminApi(request).get(`/provider-keys/${id}`)).json()).status).toBe("active")
  })

  test("delete asks for confirmation; cancel keeps, confirm removes", async ({ page, request }) => {
    const prov = uniq("ws06ui-dprov")
    const pid = await apiProvider(request, prov)
    const name = uniq("ws06ui-del")
    const id = await apiKey(request, { provider_id: pid, name, token: "sk-ws06ui-del-6666" })
    await gotoKeys(page)

    await rowAction(page, name, "Delete")
    const dlg = page.getByRole("dialog")
    await expect(dlg.getByText("Delete provider key")).toBeVisible()
    await expect(dlg.getByText(/permanently remove the key/)).toBeVisible()
    await dlg.getByRole("button", { name: "Cancel" }).click()
    await expect(dlg).toBeHidden()
    await expect(row(page, name)).toBeVisible()
    expect((await adminApi(request).get(`/provider-keys/${id}`)).status()).toBe(200)

    await rowAction(page, name, "Delete")
    await page.getByRole("dialog").getByRole("button", { name: "Delete" }).click()
    await expect(page.getByText("Provider key deleted")).toBeVisible()
    await expect(row(page, name)).toHaveCount(0)
    expect((await adminApi(request).get(`/provider-keys/${id}`)).status()).toBe(404)
  })

  test("viewer sees keys masked only", async ({ page }) => {
    await loginAndWait(page, fx.personas.viewer)
    await page.goto("/provider-keys")
    await expect(page.getByRole("heading", { name: "Provider Keys" })).toBeVisible()
    await expect(row(page, "e2e-openai-key").getByText(mask("sk-e2e-openai"))).toBeVisible()
    await expect(page.getByText("sk-e2e-openai", { exact: true })).toHaveCount(0)
  })
})
