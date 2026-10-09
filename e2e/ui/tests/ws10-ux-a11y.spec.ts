import type { Page } from "@playwright/test"
import { test, expect, openAs, PAGES, heading, shot } from "./ws10-helpers"
import { stubApi } from "./ws10-stub"

// WS-10: accessibility basics + keyboard-only use of create dialogs.
// Runs against canned API data (ws10-stub) so nothing is created for real.

test.use({ viewport: { width: 1280, height: 800 } })
test.describe.configure({ timeout: 90_000 })

type DialogSpec = {
  path: string
  open: string            // button that opens the create dialog
  title: string           // dialog accessible name
  labels: string[]        // every field label that must resolve via getByLabel
  tab?: string            // access page tab to select first
}

const DIALOGS: DialogSpec[] = [
  { path: "/providers", open: "New provider", title: "New provider", labels: ["Name", "Protocol", "Description", "Base URL", "Website", "Supports streaming"] },
  { path: "/provider-keys", open: "Add Key", title: "Add Provider Key", labels: ["Provider", "Key Type", "Name", "Description", "API Token", "Base URL (optional)"] },
  { path: "/rate-limits", open: "New limit", title: "New rate limit", labels: ["Name", "Subject", "Requests / min", "Max concurrent"] },
  { path: "/guardrails", open: "Add Rule", title: "Create Rule", labels: ["Name", "Type", "Terms (comma-separated)", "Action", "Priority"] },
  { path: "/webhooks", open: "Add Webhook", title: "Add Webhook", labels: ["Endpoint URL"] },
  { path: "/service-accounts", open: "Create Service Account", title: "Create Service Account", labels: ["Name", "Service", "Expires In (optional)"] },
  { path: "/access", open: "Create User", title: "Create User", labels: ["Name", "Email", "Password"], tab: "Users" },
  { path: "/access", open: "Create Role", title: "Create Role", labels: ["Name"], tab: "Roles" },
  { path: "/access", open: "Assign Role", title: "Assign Role", labels: ["User", "Role"], tab: "Assignments" },
]

async function gotoStubbed(page: Page, path: string, tab?: string) {
  await openAs(page, "admin", "/login")
  await stubApi(page)
  await page.goto(path)
  const p = PAGES.find((x) => x.path === path)!
  await expect(heading(page, p.heading)).toBeVisible()
  await expect(page.locator("[data-slot=skeleton]")).toHaveCount(0)
  if (tab) await page.getByRole("tab", { name: new RegExp(tab) }).click()
}

test.describe("WS-10 a11y: dialogs and form labels", () => {
  for (const d of DIALOGS) {
    test(`${d.path} "${d.open}" dialog has role=dialog, a name, and labelled fields`, async ({ page }) => {
      await gotoStubbed(page, d.path, d.tab)
      await page.getByRole("button", { name: d.open }).first().click()
      const dlg = page.getByRole("dialog", { name: d.title })
      await expect(dlg).toBeVisible()
      // FINDING WS10-3: several dialogs render <Label> without htmlFor, so
      // getByLabel() (and screen readers) cannot find the field.
      const missing: string[] = []
      for (const l of d.labels) {
        // base-ui mirrors Switch/Select into an aria-hidden <input>; count only
        // controls exposed to assistive tech.
        const n = await dlg.getByLabel(l, { exact: true }).and(page.locator(":not([aria-hidden=true])")).count()
        if (n !== 1) missing.push(l)
      }
      if (missing.length) await shot(page, `WS10-3-labels${d.path.replace(/\//g, "_")}-${d.open.replace(/\s+/g, "_")}`)
      expect(missing, `labels not associated with a control in "${d.title}"`).toEqual([])
    })
  }
})

test.describe("WS-10 a11y: accessible names", () => {
  test("every button and link on every page has an accessible name", async ({ page }) => {
    // FINDING WS10-2: icon-only row action menus ("…") and the provider website
    // link have no accessible name (screen readers announce just "button").
    await gotoStubbed(page, "/")
    for (const p of PAGES) {
      await page.goto(p.path)
      await expect(heading(page, p.heading)).toBeVisible()
      await expect(page.locator("[data-slot=skeleton]")).toHaveCount(0)
      const unnamed = await page.evaluate(() => [...document.querySelectorAll("button, a[href], [role=button]")]
        .filter((el) => {
          const h = el as HTMLElement
          if (h.offsetParent === null) return false
          const name = (h.getAttribute("aria-label") ?? "") + (h.getAttribute("title") ?? "") + (h.innerText ?? "").trim()
          const lbl = h.getAttribute("aria-labelledby")
          return !name && !(lbl && document.getElementById(lbl)?.textContent?.trim())
        })
        .map((el) => el.outerHTML.replace(/class="[^"]*"/g, "").slice(0, 90)))
      expect.soft(unnamed, `${p.path}: controls without an accessible name`).toEqual([])
      if (unnamed.length) await shot(page, `WS10-2-unnamed${p.path === "/" ? "_dashboard" : p.path.replace(/\//g, "_")}`)
    }
  })

  test("row action menus are not <button> nested in <button> (invalid HTML)", async ({ page, knownConsole }) => {
    // FINDING WS10-2: <DropdownMenuTrigger><Button/></DropdownMenuTrigger>
    // renders a button inside a button (React logs "cannot be a descendant of").
    await gotoStubbed(page, "/providers")
    const nested = await page.locator("button button").count()
    expect(nested, "nested <button> elements on /providers").toBe(0)
    expect(knownConsole, "React invalid-nesting warnings").toEqual([])
  })
})

test.describe("WS-10 keyboard-only", () => {
  test("providers: open dialog with Enter, Tab is trapped, Escape closes, focus returns", async ({ page }) => {
    await gotoStubbed(page, "/providers")
    const trigger = page.getByRole("button", { name: "New provider" })
    await trigger.focus()
    await page.keyboard.press("Enter")
    const dlg = page.getByRole("dialog", { name: "New provider" })
    await expect(dlg).toBeVisible()
    // Focus lands inside the dialog, on the first field.
    await expect(dlg.getByLabel("Name", { exact: true })).toBeFocused()
    // Tab through every control: focus never leaves the dialog (trap) and wraps.
    const seen = new Set<string>()
    for (let i = 0; i < 12; i++) {
      await page.keyboard.press("Tab")
      // base-ui uses focus-guard spans that bounce focus back on the next
      // frame; give the trap a moment before sampling document.activeElement.
      await page.waitForTimeout(100)
      const where = await page.evaluate(() => {
        const a = document.activeElement as HTMLElement | null
        return { inDialog: !!a?.closest("[role=dialog]"), id: a?.id || a?.innerText?.trim() || a?.tagName || "",
          html: a?.outerHTML.slice(0, 120) ?? "null" }
      })
      expect(where.inDialog, `Tab #${i + 1} left the dialog (focus on ${where.html})`).toBe(true)
      seen.add(where.id)
    }
    for (const id of ["p-protocol", "p-desc", "p-base", "p-website", "Cancel"]) expect(seen).toContain(id)
    await page.keyboard.press("Escape")
    await expect(dlg).toBeHidden()
    await expect(trigger).toBeFocused()
  })

  // Enter in a text field should submit the create form. Only the provider-key
  // dialog is a <form>; the others ignore Enter (FINDING WS10-6).
  const ENTER_SUBMIT: { path: string; open: string; title: string; fill: [string, string][]; post: RegExp }[] = [
    { path: "/providers", open: "New provider", title: "New provider", fill: [["Name", "ws10-kbd"], ["Base URL", "http://localhost:29100/ws10/v1"]], post: /\/api\/v1\/providers$/ },
    { path: "/rate-limits", open: "New limit", title: "New rate limit", fill: [["Name", "ws10-kbd"]], post: /\/api\/v1\/rate-limits$/ },
    { path: "/webhooks", open: "Add Webhook", title: "Add Webhook", fill: [], post: /\/api\/v1\/webhooks$/ },
    { path: "/service-accounts", open: "Create Service Account", title: "Create Service Account", fill: [], post: /\/api\/v1\/service-accounts$/ },
  ]
  for (const c of ENTER_SUBMIT) {
    test(`${c.path}: Enter in the last field submits the create dialog`, async ({ page }) => {
      await gotoStubbed(page, c.path)
      const posts: string[] = []
      await page.route(c.post, (r) => {
        if (r.request().method() !== "POST") return r.fallback()
        posts.push(r.request().postData() ?? "")
        return r.fulfill({ status: 201, json: { id: "new", name: "ws10-kbd", secret: "x", url: "x", events: [] } })
      })
      await page.getByRole("button", { name: c.open }).click()
      const dlg = page.getByRole("dialog", { name: c.title })
      await expect(dlg).toBeVisible()
      const inputs = dlg.locator("input:not([type=checkbox]):not([type=hidden]):not([aria-hidden=true])")
      if (c.path === "/rate-limits") {
        await dlg.getByRole("combobox").click()
        await page.getByRole("option", { name: /^default/ }).click()
      }
      if (c.path === "/webhooks") {
        await inputs.first().fill("https://hooks.example.com/ws10")
      }
      if (c.path === "/service-accounts") await inputs.first().fill("ws10-kbd")
      for (const [l, v] of c.fill) await dlg.getByLabel(l, { exact: true }).fill(v)
      await inputs.last().focus()
      await page.keyboard.press("Enter")
      await expect.poll(() => posts.length, { timeout: 3_000, message: "Enter did not submit" }).toBe(1)
    })
  }

  test("provider-keys: Enter submits the form (only dialog that is a <form>)", async ({ page }) => {
    await gotoStubbed(page, "/provider-keys")
    let posts = 0
    await page.route(/\/api\/v1\/provider-keys$/, (r) => {
      if (r.request().method() !== "POST") return r.fallback()
      posts++
      return r.fulfill({ status: 201, json: { id: "new" } })
    })
    await page.getByRole("button", { name: "Add Key" }).click()
    const dlg = page.getByRole("dialog", { name: "Add Provider Key" })
    await dlg.getByRole("combobox").first().click()
    await page.getByRole("option", { name: "OpenAI" }).click()
    await dlg.getByLabel("Name", { exact: true }).fill("ws10-kbd")
    await dlg.getByLabel("API Token").fill("sk-ws10")
    await dlg.getByLabel("API Token").press("Enter")
    await expect.poll(() => posts).toBe(1)
    await expect(dlg).toBeHidden()
    await expect(page.getByText("Provider key added")).toBeVisible()
  })

  test("confirm dialog: Escape cancels a delete and nothing is deleted", async ({ page }) => {
    await gotoStubbed(page, "/rate-limits")
    let deletes = 0
    await page.route(/\/api\/v1\/rate-limits\/r2$/, (r) => { deletes++; return r.fulfill({ status: 204 }) })
    await page.getByRole("row").filter({ hasText: "batch-job" }).getByRole("button").last().click()
    await page.getByRole("menuitem", { name: "Delete" }).click()
    const dlg = page.getByRole("dialog")
    await expect(dlg).toBeVisible()
    await page.keyboard.press("Escape")
    await expect(dlg).toBeHidden()
    expect(deletes).toBe(0)
  })
})
