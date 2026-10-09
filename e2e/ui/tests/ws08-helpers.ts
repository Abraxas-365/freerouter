// Shared helpers for the WS-08 console specs (rate limits, guardrails, webhooks).
import type { APIRequestContext, Page } from "@playwright/test"
import { execFileSync } from "node:child_process"
import { expect, fx, login, adminApi } from "./helpers"

/** Log in as admin, retrying with backoff (IAMKit throttles logins per IP). */
export async function ws08Login(page: Page) {
  for (let i = 0; i < 5; i++) {
    await login(page, fx.personas.admin)
    try {
      await expect(page).not.toHaveURL(/\/login/, { timeout: 10_000 })
      return
    } catch (e) {
      if (i === 4) throw e
      await page.waitForTimeout(3_000 * (i + 1))
    }
  }
}

export const row = (page: Page, text: string) => page.getByRole("row").filter({ hasText: text })

export async function rowAction(page: Page, text: string, item: string) {
  await row(page, text).getByRole("button").last().click()
  await page.getByRole("menuitem", { name: item }).click()
}

function psql(sql: string) {
  return execFileSync("psql", [fx.urls.db, "-Atc", sql], { encoding: "utf8" }).trim()
}

/**
 * Snapshot the GLOBAL guardrail config and return a restore function.
 * Existing row → PUT the old values back. No row (GET 404) → the product has
 * no DELETE for the singleton, so the row this test created is removed via psql.
 */
export async function ws08GuardrailSnapshot(request: APIRequestContext): Promise<() => Promise<void>> {
  const api = adminApi(request)
  const r = await api.get("/guardrails/config")
  if (r.status() === 200) {
    const old = await r.json()
    return async () => {
      const p = await api.put("/guardrails/config", { enabled: old.enabled, system_rules: old.system_rules })
      expect(p.status()).toBe(200)
    }
  }
  expect(r.status()).toBe(404)
  const before = psql("SELECT coalesce(string_agg(id::text, ','), '') FROM guardrail_configs")
  return async () => {
    psql(`DELETE FROM guardrail_configs WHERE NOT (id::text = ANY('{${before}}'))`)
  }
}

export const ws08AllOff = {
  prompt_injection: { enabled: false, action: "block" },
  jailbreak: { enabled: false, action: "block" },
  pii_detection: { enabled: false, action: "redact" },
  secrets: { enabled: false, action: "block" },
  document_leakage: { enabled: false, action: "warn" },
}
