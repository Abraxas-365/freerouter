// Shared helpers for the console (web/) functional tests.
// Reads e2e/.run/fixtures.json produced by e2e/up.sh.
import { test as base, expect, type Page, type APIRequestContext } from "@playwright/test"
import { readFileSync } from "node:fs"
import { join } from "node:path"

export type Persona = {
  kind: "user" | "service_account"
  email?: string
  password?: string
  secret?: string
  user_id?: string
  id?: string
  note: string
}

export type Fixtures = {
  urls: {
    server: string; api: string; gateway: string; iamkit: string; web: string
    fakellm: string; webhook_sink: string; db: string; redis: string
  }
  iamkit: {
    environment_id: string; organization_id: string; application_id: string
    resource_id: string; audience: string; management_key: string
  }
  personas: Record<"admin" | "viewer" | "providers_only" | "noperm" | "suspended" | "foreign" | "admin_key" | "gw_key" | "noperm_key" | "expired_key", Persona>
  roles: Record<string, string>
  boundary: Record<string, string>
  providers: Record<"openai" | "anthropic" | "google" | "cohere", string>
  provider_keys: Record<string, string>
  models: Record<string, string>
  permissions: string[]
}

export const fx: Fixtures = JSON.parse(readFileSync(join(__dirname, "..", "..", ".run", "fixtures.json"), "utf8"))

/** Log in through the real login page. */
export async function login(page: Page, persona: Persona) {
  if (persona.kind !== "user") throw new Error("login needs a user persona")
  await page.goto("/login")
  await page.locator("#email").fill(persona.email!)
  await page.locator("#password").fill(persona.password!)
  await page.getByRole("button", { name: /sign in|log in|login/i }).click()
}

/** Log in and wait for the authenticated shell to render. */
export async function loginAndWait(page: Page, persona: Persona) {
  await login(page, persona)
  await expect(page).not.toHaveURL(/\/login/, { timeout: 10_000 })
}

/** Admin API helper (service account, all freerouter permissions) for setup/teardown. */
export function adminApi(request: APIRequestContext) {
  const headers = { Authorization: `Bearer ${fx.personas.admin_key.secret}`, "Content-Type": "application/json" }
  return {
    get: (path: string) => request.get(fx.urls.api + path, { headers }),
    post: (path: string, data?: unknown) => request.post(fx.urls.api + path, { headers, data }),
    put: (path: string, data?: unknown) => request.put(fx.urls.api + path, { headers, data }),
    patch: (path: string, data?: unknown) => request.patch(fx.urls.api + path, { headers, data }),
    delete: (path: string) => request.delete(fx.urls.api + path, { headers }),
  }
}

/** Unique name so tests do not collide across runs. */
export const uniq = (prefix: string) => `${prefix}-${Date.now().toString(36)}-${Math.random().toString(36).slice(2, 6)}`

export const test = base
export { expect }
