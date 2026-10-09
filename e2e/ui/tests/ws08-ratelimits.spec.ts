import type { APIRequestContext, Page } from "@playwright/test"
import { test, expect, adminApi, uniq } from "./helpers"
import { ws08Login, row } from "./ws08-helpers"

// WS-08 console: rate limits page. Only ws08-ui-* custom subjects are used;
// the "default" config is never created or modified.

test.describe.configure({ timeout: 90_000 })

const cleanup: (() => Promise<unknown>)[] = []
test.afterEach(async () => {
  for (const fn of cleanup.splice(0).reverse()) await fn().catch(() => undefined)
})

async function rlBySubject(request: APIRequestContext, subject: string) {
  const r = await adminApi(request).get(`/rate-limits?subject_id=${encodeURIComponent(subject)}`)
  expect(r.status()).toBe(200)
  return (await r.json()).items as any[]
}

function trackSubject(request: APIRequestContext, subject: string) {
  cleanup.push(async () => {
    for (const it of await rlBySubject(request, subject)) await adminApi(request).delete(`/rate-limits/${it.id}`)
  })
}

async function gotoRL(page: Page) {
  await ws08Login(page)
  await page.goto("/rate-limits")
  await expect(page.getByRole("heading", { name: "Rate Limits", exact: true })).toBeVisible()
}

async function openCreate(page: Page, name: string, subject: string) {
  await page.getByRole("button", { name: "New limit" }).first().click()
  const dlg = page.getByRole("dialog")
  await expect(dlg.getByRole("heading", { name: "New rate limit" })).toBeVisible()
  await dlg.getByLabel("Name").fill(name)
  await dlg.getByRole("combobox").click()
  await page.getByRole("option", { name: "Custom subject ID…" }).click()
  await dlg.getByPlaceholder("JWT subject UUID").fill(subject)
  return dlg
}

test.describe("WS-08 rate limits page", () => {
  test("create, edit and delete a per-subject limit", async ({ page, request }) => {
    const subject = uniq("ws08-ui-subj")
    const name = uniq("ws08-ui-rl")
    trackSubject(request, subject)
    await gotoRL(page)

    const dlg = await openCreate(page, name, subject)
    await expect(dlg.getByLabel("Requests / min")).toHaveValue("60")
    await expect(dlg.getByLabel("Max concurrent")).toHaveValue("10")
    await dlg.getByLabel("Requests / min").fill("120")
    await dlg.getByLabel("Max concurrent").fill("0")
    await dlg.getByRole("button", { name: "Create" }).click()
    await expect(page.getByText("Rate limit created")).toBeVisible()
    await expect(dlg).toBeHidden()
    const r = row(page, name)
    await expect(r).toContainText(subject)
    await expect(r).toContainText("120")
    await expect(r).toContainText("unlimited")
    const [cfg] = await rlBySubject(request, subject)
    expect(cfg).toMatchObject({ name, subject_id: subject, rpm: 120, max_concurrent: 0 })

    // Edit: subject is locked, values prefilled.
    await r.getByRole("button").last().click()
    await page.getByRole("menuitem", { name: "Edit" }).click()
    const ed = page.getByRole("dialog")
    await expect(ed.getByRole("heading", { name: "Edit rate limit" })).toBeVisible()
    await expect(ed.getByRole("combobox")).toBeDisabled()
    await expect(ed.getByLabel("Requests / min")).toHaveValue("120")
    await ed.getByLabel("Requests / min").fill("0")
    await ed.getByLabel("Max concurrent").fill("7")
    await ed.getByRole("button", { name: "Save" }).click()
    await expect(page.getByText("Rate limit updated")).toBeVisible()
    await expect(row(page, name)).toContainText("7")
    expect((await rlBySubject(request, subject))[0]).toMatchObject({ rpm: 0, max_concurrent: 7 })

    // Delete with confirmation; cancel first.
    await row(page, name).getByRole("button").last().click()
    await page.getByRole("menuitem", { name: "Delete" }).click()
    const cd = page.getByRole("dialog")
    await expect(cd.getByText("The subject will fall back to the default limit")).toBeVisible()
    await cd.getByRole("button", { name: "Cancel" }).click()
    await expect(row(page, name)).toBeVisible()
    await row(page, name).getByRole("button").last().click()
    await page.getByRole("menuitem", { name: "Delete" }).click()
    await page.getByRole("dialog").getByRole("button", { name: "Delete" }).click()
    await expect(page.getByText("Rate limit deleted")).toBeVisible()
    await expect(row(page, name)).toHaveCount(0)
    expect(await rlBySubject(request, subject)).toHaveLength(0)
  })

  test("create is disabled until name and subject are set", async ({ page }) => {
    await gotoRL(page)
    await page.getByRole("button", { name: "New limit" }).first().click()
    const dlg = page.getByRole("dialog")
    const create = dlg.getByRole("button", { name: "Create" })
    await expect(create).toBeDisabled()
    await dlg.getByLabel("Name").fill("ws08-ui-x")
    await expect(create).toBeDisabled()
    await dlg.getByRole("combobox").click()
    await page.getByRole("option", { name: "Custom subject ID…" }).click()
    await dlg.getByPlaceholder("JWT subject UUID").fill("   ")
    await expect(create).toBeDisabled()
    await dlg.getByRole("button", { name: "Cancel" }).click()
    await expect(dlg).toBeHidden()
  })

  // FINDING WS08-15: API validation errors are swallowed by the console forms
  // (unhandled promise rejection; no message, dialog just stays open).
  test("negative rpm shows the server validation error", async ({ page, request }) => {
    const subject = uniq("ws08-ui-neg")
    trackSubject(request, subject)
    await gotoRL(page)
    const dlg = await openCreate(page, uniq("ws08-ui-neg"), subject)
    await dlg.getByLabel("Requests / min").fill("-5")
    await dlg.getByRole("button", { name: "Create" }).click()
    await expect(page.getByText("rpm must be non-negative (0 = unlimited)")).toBeVisible()
    expect(await rlBySubject(request, subject)).toHaveLength(0)
  })

  test("duplicate subject shows a conflict error", async ({ page, request }) => {
    const subject = uniq("ws08-ui-dup")
    trackSubject(request, subject)
    const first = await adminApi(request).post("/rate-limits", { name: uniq("ws08-ui-a"), subject_id: subject, rpm: 5, max_concurrent: 1 })
    expect(first.status()).toBe(201)
    await gotoRL(page)
    const dlg = await openCreate(page, uniq("ws08-ui-b"), subject)
    await dlg.getByRole("button", { name: "Create" }).click()
    await expect(page.getByText("rate limit config already exists")).toBeVisible()
    expect(await rlBySubject(request, subject)).toHaveLength(1)
  })
})
