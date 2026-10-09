import { test, expect, fx, loginAndWait, adminApi, uniq } from "./helpers"
import type { APIRequestContext, Page } from "@playwright/test"

// WS-03 — Access management console (/access): users, roles, assignments.
// Everything created is prefixed ws03- and removed in afterEach (users are
// purged through IAMKit's management API: the product only suspends).

const PASSWORD = "ws03-password-123456"
const MG = `${fx.urls.iamkit}/management/v1/environments/${fx.iamkit.environment_id}`
const mgHeaders = { "X-API-Key": fx.iamkit.management_key, "Content-Type": "application/json" }

let createdUsers: string[] = []
let createdRoles: string[] = []

test.beforeEach(() => { test.setTimeout(150_000); createdUsers = []; createdRoles = [] })

test.afterEach(async ({ request }) => {
  test.setTimeout(240_000)
  // Cleanup goes straight to the management API and retries IAMKit throttling (429).
  const del = async (url: string) => {
    for (let i = 0; i < 40; i++) {
      const s = (await request.delete(url, { headers: mgHeaders })).status()
      if (s !== 429 && s !== 502) return
      await new Promise((res) => setTimeout(res, 3000))
    }
  }
  for (const id of createdRoles) await del(`${MG}/roles/${id}`)
  for (const id of createdUsers) await del(`${MG}/users/${id}/permanent`)
})

/** Retries FreeRouter calls that fail with 502 because IAMKit throttled the shared backend budget. */
async function retry<T>(fn: () => Promise<{ status(): number; json(): Promise<T> }>, ok = [200, 201, 204]) {
  for (let i = 0; i < 40; i++) {
    const r = await fn()
    if (ok.includes(r.status())) return r
    if (r.status() !== 502 && r.status() !== 429) throw new Error(`unexpected ${r.status()}`)
    await new Promise((res) => setTimeout(res, 3000))
  }
  throw new Error("IAMKit budget never recovered")
}

async function apiUser(request: APIRequestContext, tag: string) {
  const email = `${uniq(`ws03-${tag}`)}@e2e.test`
  const r = await retry(() => adminApi(request).post("/access/users", { email, name: `WS03 ${tag}`, password: PASSWORD }))
  const u = (await r.json()) as { id: string; email: string; name: string }
  createdUsers.push(u.id)
  return u
}

async function apiRole(request: APIRequestContext, tag: string, permissions = ["freerouter:metrics:read"]) {
  const name = uniq(`ws03-${tag}`)
  const r = await retry(() => adminApi(request).post("/access/roles", { name, permissions }))
  const role = (await r.json()) as { id: string; name: string }
  createdRoles.push(role.id)
  return role
}

async function idOfUser(request: APIRequestContext, email: string) {
  const r = await retry(() => adminApi(request).get("/access/users"))
  const u = ((await r.json()) as { id: string; email: string }[]).find((x) => x.email === email)
  if (u) createdUsers.push(u.id)
  return u?.id
}

async function idOfRole(request: APIRequestContext, name: string) {
  const r = await retry(() => adminApi(request).get("/access/roles"))
  const role = ((await r.json()) as { id: string; name: string }[]).find((x) => x.name === name)
  if (role) createdRoles.push(role.id)
  return role?.id
}

async function openAccess(page: Page, tab?: "Roles" | "Assignments") {
  await loginAndWait(page, fx.personas.admin)
  // The page has no error state (FINDING WS03-9): if any of its three list
  // calls 502s (IAMKit throttling of the shared backend account) it stays on
  // the skeleton forever. Reload until it renders so the rest is measurable.
  for (let i = 0; ; i++) {
    await page.goto("/access")
    await expect(page.getByRole("heading", { name: "Access Management" })).toBeVisible()
    try {
      await expect(page.getByRole("tab", { name: "Users" })).toBeVisible({ timeout: 8_000 })
      break
    } catch (e) {
      if (i >= 20) throw e
      await page.waitForTimeout(3_000)
    }
  }
  if (tab) await page.getByRole("tab", { name: tab }).click()
}

const row = (page: Page, text: string) => page.getByRole("row").filter({ hasText: text })

// ── users ───────────────────────────────────────────────────────────

test.describe("WS-03 users", () => {
  test("create user through the form; list refreshes with Active badge", async ({ page, request }) => {
    await openAccess(page)
    const email = `${uniq("ws03-ui")}@e2e.test`
    await page.getByRole("button", { name: "Create User" }).first().click()
    const dlg = page.getByRole("dialog", { name: "Create User" })
    const create = dlg.getByRole("button", { name: "Create" })

    // Client-side gating: Create stays disabled until every field is valid.
    await expect(create).toBeDisabled()
    await dlg.getByPlaceholder("Alice Smith").fill("WS03 UI User")
    await expect(create).toBeDisabled()
    await dlg.getByPlaceholder("alice@example.com").fill(email)
    await dlg.getByPlaceholder("Minimum 8 characters").fill("short")
    await expect(create).toBeDisabled()
    await dlg.getByPlaceholder("Alice Smith").fill("   ")
    await dlg.getByPlaceholder("Minimum 8 characters").fill(PASSWORD)
    await expect(create).toBeDisabled() // blank name
    await dlg.getByPlaceholder("Alice Smith").fill("WS03 UI User")
    await expect(create).toBeEnabled()

    await create.click()
    await expect(page.getByText("User created")).toBeVisible()
    await expect(dlg).toBeHidden()
    const r = row(page, email)
    await expect(r).toBeVisible()
    await expect(r.getByText("WS03 UI User")).toBeVisible()
    await expect(r.getByText("Active", { exact: true })).toBeVisible()
    expect(await idOfUser(request, email)).toBeTruthy()
  })

  test("duplicate email shows a readable error and keeps the dialog open", async ({ page, request }) => {
    // FINDING WS03-2: the console swallows API errors (unhandled rejection, no toast).
    const existing = await apiUser(request, "uidup")
    await openAccess(page)
    await page.getByRole("button", { name: "Create User" }).first().click()
    const dlg = page.getByRole("dialog", { name: "Create User" })
    await dlg.getByPlaceholder("Alice Smith").fill("Dup")
    await dlg.getByPlaceholder("alice@example.com").fill(existing.email)
    await dlg.getByPlaceholder("Minimum 8 characters").fill(PASSWORD)
    await dlg.getByRole("button", { name: "Create" }).click()
    await expect(dlg).toBeVisible()
    await expect(page.getByText(/email is taken|already (exists|in use)/i)).toBeVisible()
  })

  test("password accepted by the form but rejected by the server shows the policy", async ({ page, request }) => {
    // FINDING WS03-2 + WS03-8: form allows 8+ chars, IAMKit requires 12.
    await openAccess(page)
    const email = `${uniq("ws03-uipw")}@e2e.test`
    await page.getByRole("button", { name: "Create User" }).first().click()
    const dlg = page.getByRole("dialog", { name: "Create User" })
    await dlg.getByPlaceholder("Alice Smith").fill("PW")
    await dlg.getByPlaceholder("alice@example.com").fill(email)
    await dlg.getByPlaceholder("Minimum 8 characters").fill("12345678")
    await expect(dlg.getByRole("button", { name: "Create" })).toBeEnabled()
    await dlg.getByRole("button", { name: "Create" }).click()
    await expect(page.getByText(/12-72 characters/)).toBeVisible()
    expect(await idOfUser(request, email)).toBeUndefined()
  })

  test("edit user name; list refreshes", async ({ page, request }) => {
    const u = await apiUser(request, "uiedit")
    await openAccess(page)
    await row(page, u.email).getByRole("button").first().click()
    const dlg = page.getByRole("dialog", { name: "Edit User" })
    const input = dlg.getByRole("textbox")
    await expect(input).toHaveValue(u.name)
    await input.fill("   ")
    await expect(dlg.getByRole("button", { name: "Save" })).toBeDisabled()
    await input.fill("WS03 Edited Ünïcødé")
    await dlg.getByRole("button", { name: "Save" }).click()
    await expect(page.getByText("User updated")).toBeVisible()
    await expect(row(page, u.email).getByText("WS03 Edited Ünïcødé")).toBeVisible()
  })

  test("suspend: cancel keeps the user active; confirm marks Suspended and hides the button", async ({ page, request }) => {
    const u = await apiUser(request, "uisusp")
    await openAccess(page)
    const r = row(page, u.email)
    await expect(r.getByRole("button")).toHaveCount(2) // edit + suspend

    await r.getByRole("button").nth(1).click()
    const dlg = page.getByRole("dialog", { name: "Suspend User" })
    await expect(dlg.getByText("They will lose all access until reactivated")).toBeVisible()
    await dlg.getByRole("button", { name: "Cancel" }).click()
    await expect(dlg).toBeHidden()
    await expect(r.getByText("Active", { exact: true })).toBeVisible()
    const after = await retry(() => adminApi(request).get(`/access/users/${u.id}`))
    expect(((await after.json()) as { active: boolean }).active).toBe(true)

    await r.getByRole("button").nth(1).click()
    await page.getByRole("dialog", { name: "Suspend User" }).getByRole("button", { name: "Suspend" }).click()
    await expect(page.getByText("User suspended")).toBeVisible()
    await expect(r.getByText("Suspended", { exact: true })).toBeVisible()
    await expect(r.getByRole("button")).toHaveCount(1) // only edit left
  })

  test("seeded suspended persona shows as Suspended", async ({ page }) => {
    await openAccess(page)
    await expect(row(page, fx.personas.suspended.email!).getByText("Suspended", { exact: true })).toBeVisible()
  })

  test("foreign-org user is not shown", async ({ page }) => {
    // FINDING WS03-1 (critical): users of other organizations are listed.
    await openAccess(page)
    await expect(row(page, fx.personas.admin.email!)).toBeVisible()
    await expect(page.getByText(fx.personas.foreign.email!)).toHaveCount(0)
  })
})

// ── roles ───────────────────────────────────────────────────────────

test.describe("WS-03 roles", () => {
  test("create role through the form; permissions shown; foreign role hidden", async ({ page, request }) => {
    await openAccess(page, "Roles")
    await expect(page.getByText("e2e-foreign-iam-role")).toHaveCount(0)
    await expect(row(page, "e2e-viewer")).toBeVisible()

    const name = uniq("ws03-uirole")
    await page.getByRole("button", { name: "Create Role" }).first().click()
    const dlg = page.getByRole("dialog", { name: "Create Role" })
    const create = dlg.getByRole("button", { name: "Create" })
    await expect(create).toBeDisabled()
    await dlg.getByPlaceholder(/Admin, Viewer/).fill(name)
    await expect(create).toBeDisabled() // no permission yet
    // Only freerouter permissions are offered.
    const boxes = dlg.getByRole("checkbox")
    await expect(boxes).toHaveCount(fx.permissions.length)
    await expect(dlg.getByText(/^iam:/)).toHaveCount(0)
    await dlg.getByText("metrics:read", { exact: true }).click()
    await dlg.getByText("usage:read", { exact: true }).click()
    await expect(create).toBeEnabled()
    await create.click()

    await expect(page.getByText("Role created")).toBeVisible()
    const r = row(page, name)
    await expect(r.getByText("freerouter:metrics:read")).toBeVisible()
    await expect(r.getByText("freerouter:usage:read")).toBeVisible()
    expect(await idOfRole(request, name)).toBeTruthy()
  })

  test("duplicate role name shows a readable error", async ({ page, request }) => {
    // FINDING WS03-2 + WS03-7.
    const existing = await apiRole(request, "uiduprole")
    await openAccess(page, "Roles")
    await page.getByRole("button", { name: "Create Role" }).first().click()
    const dlg = page.getByRole("dialog", { name: "Create Role" })
    await dlg.getByPlaceholder(/Admin, Viewer/).fill(existing.name)
    await dlg.getByText("metrics:read", { exact: true }).click()
    await dlg.getByRole("button", { name: "Create" }).click()
    await expect(dlg).toBeVisible()
    await expect(page.getByText(/name.*(taken|exists|in use)|already exists/i)).toBeVisible()
  })

  test("edit role permissions; list refreshes", async ({ page, request }) => {
    const role = await apiRole(request, "uieditrole", ["freerouter:metrics:read"])
    await openAccess(page, "Roles")
    await row(page, role.name).getByRole("button").first().click()
    const dlg = page.getByRole("dialog", { name: "Edit Role" })
    await expect(dlg.getByRole("textbox")).toHaveValue(role.name)
    await dlg.getByText("metrics:read", { exact: true }).click() // uncheck
    await expect(dlg.getByRole("button", { name: "Save" })).toBeDisabled()
    await dlg.getByText("webhooks:read", { exact: true }).click()
    await dlg.getByRole("button", { name: "Save" }).click()
    await expect(page.getByText("Role updated")).toBeVisible()
    const r = row(page, role.name)
    await expect(r.getByText("freerouter:webhooks:read")).toBeVisible()
    await expect(r.getByText("freerouter:metrics:read")).toHaveCount(0)
  })

  test("delete role: cancel keeps it, confirm removes it", async ({ page, request }) => {
    const role = await apiRole(request, "uidelrole")
    await openAccess(page, "Roles")
    await row(page, role.name).getByRole("button").nth(1).click()
    let dlg = page.getByRole("dialog", { name: "Delete Role" })
    await dlg.getByRole("button", { name: "Cancel" }).click()
    await expect(row(page, role.name)).toBeVisible()

    await row(page, role.name).getByRole("button").nth(1).click()
    dlg = page.getByRole("dialog", { name: "Delete Role" })
    await dlg.getByRole("button", { name: "Delete" }).click()
    await expect(page.getByText("Role deleted")).toBeVisible()
    await expect(row(page, role.name)).toHaveCount(0)
  })
})

// ── assignments ─────────────────────────────────────────────────────

test.describe("WS-03 assignments", () => {
  test("assign through the form, then unassign (cancel first)", async ({ page, request }) => {
    const u = await apiUser(request, "uiassign")
    const role = await apiRole(request, "uiassign")
    await openAccess(page, "Assignments")

    await page.getByRole("button", { name: "Assign Role" }).first().click()
    const dlg = page.getByRole("dialog", { name: "Assign Role" })
    const assignBtn = dlg.getByRole("button", { name: "Assign" })
    await expect(assignBtn).toBeDisabled()
    await dlg.getByText("Select a user").click()
    await page.getByRole("option", { name: `WS03 uiassign (${u.email})` }).click()
    await expect(assignBtn).toBeDisabled()
    await dlg.getByText("Select a role").click()
    await page.getByRole("option", { name: role.name }).click()
    await assignBtn.click()

    await expect(page.getByText("Role assigned")).toBeVisible()
    const r = row(page, u.email)
    await expect(r.getByText(role.name)).toBeVisible()

    await r.getByRole("button").click()
    await page.getByRole("dialog", { name: "Unassign Role" }).getByRole("button", { name: "Cancel" }).click()
    await expect(r).toBeVisible()

    await r.getByRole("button").click()
    await page.getByRole("dialog", { name: "Unassign Role" }).getByRole("button", { name: "Unassign" }).click()
    await expect(page.getByText("Role unassigned")).toBeVisible()
    await expect(row(page, u.email)).toHaveCount(0)
  })

  test("assign picker offers only active users and own roles", async ({ page }) => {
    await openAccess(page, "Assignments")
    await page.getByRole("button", { name: "Assign Role" }).first().click()
    const dlg = page.getByRole("dialog", { name: "Assign Role" })
    await dlg.getByText("Select a user").click()
    await expect(page.getByRole("option", { name: new RegExp(fx.personas.viewer.email!) })).toBeVisible()
    await expect(page.getByRole("option", { name: new RegExp(fx.personas.suspended.email!) })).toHaveCount(0)
    await page.keyboard.press("Escape")
    await dlg.getByText("Select a role").click()
    await expect(page.getByRole("option", { name: "e2e-viewer" })).toBeVisible()
    await expect(page.getByRole("option", { name: "e2e-foreign-iam-role" })).toHaveCount(0)
  })

  test("assigning an already-held role shows a readable error", async ({ page, request }) => {
    // FINDING WS03-2: 409 "already holds this role" is swallowed by the console.
    const u = await apiUser(request, "uiassign2")
    const role = await apiRole(request, "uiassign2")
    await retry(() => adminApi(request).post("/access/role-assignments", { user_id: u.id, role_id: role.id }))
    await openAccess(page, "Assignments")
    await page.getByRole("button", { name: "Assign Role" }).first().click()
    const dlg = page.getByRole("dialog", { name: "Assign Role" })
    await dlg.getByText("Select a user").click()
    await page.getByRole("option", { name: new RegExp(u.email) }).click()
    await dlg.getByText("Select a role").click()
    await page.getByRole("option", { name: role.name }).click()
    await dlg.getByRole("button", { name: "Assign" }).click()
    await expect(page.getByText(/already holds this role/i)).toBeVisible()
  })
})

// ── permissions / large lists ──────────────────────────────────────

test.describe("WS-03 permissions and scale", () => {
  test("viewer (read-only role) can open Access and see users", async ({ page }) => {
    await loginAndWait(page, fx.personas.viewer)
    await page.goto("/access")
    await expect(row(page, fx.personas.admin.email!)).toBeVisible({ timeout: 15_000 })
  })

  test("providers_only persona cannot open /access", async ({ page }) => {
    await loginAndWait(page, fx.personas.providers_only)
    await page.goto("/access")
    await expect(page.getByRole("heading", { name: "Access Management" })).toHaveCount(0)
    await expect(page.getByText(fx.personas.admin.email!)).toHaveCount(0)
  })

  test("60 users are all rendered", async ({ page, request }) => {
    test.setTimeout(240_000)
    const prefix = uniq("ws03-uibulk")
    const emails: string[] = []
    for (let i = 0; i < 60; i++) {
      const email = `${prefix}-${String(i).padStart(2, "0")}@e2e.test`
      const r = await request.post(`${MG}/users`, {
        headers: mgHeaders,
        data: { name: `WS03 Bulk ${i}`, email, password: PASSWORD, home_organization_id: fx.iamkit.organization_id },
      })
      expect(r.status()).toBe(201)
      createdUsers.push(((await r.json()) as { id: string }).id)
      emails.push(email)
    }
    await openAccess(page)
    await expect(page.getByRole("row").filter({ hasText: prefix })).toHaveCount(60, { timeout: 15_000 })
    await row(page, emails[59]).scrollIntoViewIfNeeded()
    await expect(row(page, emails[59])).toBeVisible()
  })
})
