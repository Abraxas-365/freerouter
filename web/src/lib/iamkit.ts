/**
 * IAMKit sign-in for FREEROUTER_AUTH=iam.
 *
 * The console talks to IAMKit's `/identity/v1` API directly to obtain a JWT,
 * then sends that JWT as a Bearer token to the FreeRouter API. The IAMKit
 * URL and boundary (environment/organization/application/resource/audience)
 * come from the server at runtime (`GET /api/v1/config`, see
 * `runtime-config.ts`), so they always match the server's `IAMKIT_*` values.
 */

import { runtimeConfig, type IAMKitConfig } from "@/lib/runtime-config"

export interface TokenPair {
  access_token: string
  refresh_token?: string
  token_type: string
  expires_in: number
}

function iamkit(): IAMKitConfig {
  const cfg = runtimeConfig().iamkit
  if (!cfg) throw new Error("IAMKit is not configured on this server")
  return cfg
}

function boundary() {
  const c = iamkit()
  return {
    environment_id: c.environment_id,
    organization_id: c.organization_id,
    application_id: c.application_id,
    resource_id: c.resource_id,
  }
}

async function iamRequest<T>(path: string, body: unknown, token?: string): Promise<T> {
  const headers: Record<string, string> = { "Content-Type": "application/json" }
  if (token) headers["Authorization"] = `Bearer ${token}`
  const res = await fetch(`${iamkit().url}/identity/v1${path}`, {
    method: "POST",
    headers,
    body: JSON.stringify(body),
  })
  if (res.status === 204) return undefined as T
  if (!res.ok) {
    const err = await res.json().catch(() => ({ message: res.statusText }))
    throw new Error(err.message || res.statusText)
  }
  return res.json()
}

/** Logs in against IAMKit with the server's environment/application/resource boundary. */
export function login(email: string, password: string): Promise<TokenPair> {
  return iamRequest<TokenPair>("/login", { ...boundary(), email, password })
}

export function refresh(refreshToken: string): Promise<TokenPair> {
  return iamRequest<TokenPair>("/refresh", { ...boundary(), refresh_token: refreshToken })
}

/** Revokes the current session at IAMKit. Requires the still-live access token. */
export async function logout(): Promise<void> {
  const accessToken = localStorage.getItem("access_token")
  if (!accessToken) return
  try {
    await iamRequest("/logout", {
      environment_id: iamkit().environment_id,
      audience: iamkit().audience,
    }, accessToken)
  } catch {
    // best-effort — the tokens are cleared locally regardless
  }
}
