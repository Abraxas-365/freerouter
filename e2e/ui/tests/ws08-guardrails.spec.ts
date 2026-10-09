import type { Page } from "@playwright/test"
import { test, expect, fx, adminApi, uniq } from "./helpers"
import { ws08Login, row, rowAction, ws08GuardrailSnapshot, ws08AllOff } from "./ws08-helpers"

// WS-08 console: guardrails page. The config is a GLOBAL singleton: each test
// snapshots it and restores it in afterEach; system detectors are switched
// off immediately after toggling them in the UI.

test.describe.configure({ timeout: 90_000 })

const cleanup: (() => Promise<unknown>)[] = []
test.afterEach(async () => {
  for (const fn of cleanup.splice(0).reverse()) await fn().catch((e) => console.error("ws08 cleanup:", e))
})

async function gotoGuardrails(page: Page) {
  await ws08Login(page)
  await page.goto("/guardrails")
  await expect(page.getByRole("heading", { name: "Guardrails" })).toBeVisible()
  await expect(page.getByText("System Rules")).toBeVisible()
}

/** The switch of a system rule row, located by its label. */
const sysSwitch = (page: Page, label: string) =>
  page.locator("div.flex.items-center.justify-between").filter({ has: page.getByText(label, { exact: true }) }).getByRole("switch")

test.describe("WS-08 guardrails page", () => {
  test("master toggle and a system rule toggle persist", async ({ page, request }) => {
    const api = adminApi(request)
    cleanup.push(await ws08GuardrailSnapshot(request))
    // Start from a known, inert state: master off, all detectors off.
    expect((await api.put("/guardrails/config", { enabled: false, system_rules: ws08AllOff })).status()).toBe(200)
    await gotoGuardrails(page)

    const master = page.getByRole("switch").first()
    await expect(master).not.toBeChecked()
    await master.click()
    await expect(master).toBeChecked()
    await expect.poll(async () => (await (await api.get("/guardrails/config")).json()).enabled).toBe(true)
    await master.click() // keep the enabled window short
    await expect(master).not.toBeChecked()
    await expect.poll(async () => (await (await api.get("/guardrails/config")).json()).enabled).toBe(false)

    const docs = sysSwitch(page, "Document Leakage")
    await expect(docs).not.toBeChecked()
    await docs.click()
    await expect(docs).toBeChecked()
    await expect.poll(async () => (await (await api.get("/guardrails/config")).json()).system_rules.document_leakage.enabled).toBe(true)
    await docs.click()
    await expect.poll(async () => (await (await api.get("/guardrails/config")).json()).system_rules.document_leakage.enabled).toBe(false)

    // Reload reflects server state.
    await page.reload()
    await expect(page.getByRole("switch").first()).not.toBeChecked()
  })

  test("custom rule create, edit, disable and delete", async ({ page, request }) => {
    const api = adminApi(request)
    const name = uniq("ws08-ui-rule")
    cleanup.push(async () => {
      for (const r of await (await api.get("/guardrails/rules")).json()) if (r.name.startsWith(name)) await api.delete(`/guardrails/rules/${r.id}`)
    })
    await gotoGuardrails(page)

    await page.getByRole("button", { name: "Add Rule" }).click()
    const dlg = page.getByRole("dialog")
    await expect(dlg.getByRole("heading", { name: "Create Rule" })).toBeVisible()
    const create = dlg.getByRole("button", { name: "Create" })
    await expect(create).toBeDisabled()
    await dlg.getByPlaceholder("e.g. Block competitor mentions").fill(name)
    await dlg.getByPlaceholder("term1, term2, term3").fill("ws08-forbidden-xyzzy, , ws08-ui-two ")
    await dlg.getByPlaceholder("e.g. 1").fill("7")
    await create.click()
    await expect(page.getByText("Rule created")).toBeVisible()
    const r = row(page, name)
    await expect(r).toContainText("2 terms")
    await expect(r).toContainText("Terms")
    await expect(r).toContainText("block")
    await expect(r).toContainText("7")
    const rules = (await (await api.get("/guardrails/rules")).json()).filter((x: any) => x.name === name)
    expect(rules).toHaveLength(1)
    expect(rules[0].config.terms).toEqual(["ws08-forbidden-xyzzy", "ws08-ui-two"])

    // Edit: rename + switch action to warn.
    await rowAction(page, name, "Edit")
    const ed = page.getByRole("dialog")
    await expect(ed.getByRole("heading", { name: "Edit Rule" })).toBeVisible()
    await expect(ed.getByPlaceholder("term1, term2, term3")).toHaveValue("ws08-forbidden-xyzzy, ws08-ui-two")
    await ed.getByPlaceholder("e.g. Block competitor mentions").fill(name + "-v2")
    await ed.getByRole("combobox").first().click()
    await page.getByRole("option", { name: "Warn" }).click()
    await ed.getByRole("button", { name: "Update" }).click()
    await expect(page.getByText("Rule updated")).toBeVisible()
    // FINDING WS08-16: after Update the dialog stays open as an empty "Create Rule" form.
    await expect(page.getByRole("dialog")).toBeHidden()
    await expect(row(page, name + "-v2")).toContainText("warn")

    // Disable via menu.
    await rowAction(page, name + "-v2", "Disable")
    await expect(row(page, name + "-v2")).toContainText(/inactive/i)

    // Delete with confirm.
    await rowAction(page, name + "-v2", "Delete")
    await expect(page.getByText("This rule will be permanently deleted.")).toBeVisible()
    await page.getByRole("dialog").getByRole("button", { name: "Confirm" }).click()
    await expect(page.getByText("Rule deleted")).toBeVisible()
    await expect(row(page, name + "-v2")).toHaveCount(0)
  })

  test("regex rule form", async ({ page, request }) => {
    const api = adminApi(request)
    const name = uniq("ws08-ui-regex")
    cleanup.push(async () => {
      for (const r of await (await api.get("/guardrails/rules")).json()) if (r.name === name) await api.delete(`/guardrails/rules/${r.id}`)
    })
    await gotoGuardrails(page)
    await page.getByRole("button", { name: "Add Rule" }).click()
    const dlg = page.getByRole("dialog")
    await dlg.getByPlaceholder("e.g. Block competitor mentions").fill(name)
    await dlg.getByRole("combobox").first().click()
    await page.getByRole("option", { name: "Custom Regex" }).click()
    await dlg.getByPlaceholder("e.g. \\b\\d{4}-\\d{4}\\b").fill("ws08-order-\\d{6}")
    await dlg.getByRole("button", { name: "Create" }).click()
    await expect(page.getByText("Rule created")).toBeVisible()
    await expect(row(page, name)).toContainText("ws08-order-\\d{6}")
    await expect(row(page, name)).toContainText("Regex")
  })

  // FINDING WS08-10 (UI side): an invalid regex is accepted and saved.
  test("invalid regex is rejected by the form", async ({ page, request }) => {
    const api = adminApi(request)
    const name = uniq("ws08-ui-badre")
    cleanup.push(async () => {
      for (const r of await (await api.get("/guardrails/rules")).json()) if (r.name === name) await api.delete(`/guardrails/rules/${r.id}`)
    })
    await gotoGuardrails(page)
    await page.getByRole("button", { name: "Add Rule" }).click()
    const dlg = page.getByRole("dialog")
    await dlg.getByPlaceholder("e.g. Block competitor mentions").fill(name)
    await dlg.getByRole("combobox").first().click()
    await page.getByRole("option", { name: "Custom Regex" }).click()
    await dlg.getByPlaceholder("e.g. \\b\\d{4}-\\d{4}\\b").fill("ws08([unclosed")
    await dlg.getByRole("button", { name: "Create" }).click()
    await expect(dlg).toBeVisible()
    await expect(page.getByText(/invalid|regex|pattern/i).first()).toBeVisible()
    const saved = (await (await api.get("/guardrails/rules")).json()).filter((x: any) => x.name === name)
    expect(saved).toHaveLength(0)
  })

  test("violations table shows a blocked request", async ({ page, request }) => {
    const api = adminApi(request)
    const term = uniq("ws08-ui-forbidden")
    cleanup.push(await ws08GuardrailSnapshot(request))
    const rr = await api.post("/guardrails/rules", { name: term, type: "blocked_terms", action: "block", config: { terms: [term], match_type: "contains" } })
    expect(rr.status()).toBe(201)
    const rid = (await rr.json()).id
    cleanup.push(() => api.delete(`/guardrails/rules/${rid}`))
    expect((await api.put("/guardrails/config", { enabled: true, system_rules: ws08AllOff })).status()).toBe(200)
    // Seeded model (read-only use); the guardrail check runs before routing.
    const g = await request.post(`${fx.urls.gateway}/chat/completions`, {
      headers: { Authorization: `Bearer ${fx.personas.gw_key.secret}` },
      data: { model: "e2e-ok", messages: [{ role: "user", content: `say ${term}` }] },
    })
    expect((await api.put("/guardrails/config", { enabled: false })).status()).toBe(200)
    expect(g.status()).toBe(400)

    await expect.poll(async () => {
      const v = await (await api.get("/guardrails/violations?limit=100")).json()
      return v.items.some((x: any) => x.rule_id === rid)
    }, { timeout: 10_000 }).toBe(true)
    await gotoGuardrails(page)
    const vr = row(page, term).filter({ hasText: "blocked" })
    await expect(vr.first()).toBeVisible()
    await expect(vr.first()).toContainText("blocked_terms")
    await expect(vr.first()).toContainText("e2e-ok")
  })
})
