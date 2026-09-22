/**
 * IAMKit environment/application/resource boundary configuration.
 *
 * FreeRouter does not host its own IAM — it delegates all authentication
 * to an IAMKit sidecar (see `docker-compose.yml`, `internal/server/auth.go`).
 * The console talks to IAMKit's `/identity/v1` API directly to obtain a JWT,
 * then sends that JWT as a Bearer token to the FreeRouter API.
 *
 * These IDs are created once via IAMKit's management API when the deployment
 * is set up (see `make iamkit-logs` and the IAMKit docs) and must match the
 * `IAMKIT_ENVIRONMENT_ID` / `IAMKIT_APPLICATION_ID` / `IAMKIT_RESOURCE_ID`
 * values configured on the FreeRouter server (`.env`).
 */

export const IAMKIT_URL = (import.meta.env.VITE_IAMKIT_URL as string | undefined) ?? "http://localhost:8080"

export const IAMKIT_BOUNDARY = {
  environment_id: (import.meta.env.VITE_IAMKIT_ENVIRONMENT_ID as string | undefined) ?? "",
  organization_id: (import.meta.env.VITE_IAMKIT_ORGANIZATION_ID as string | undefined) ?? "",
  application_id: (import.meta.env.VITE_IAMKIT_APPLICATION_ID as string | undefined) ?? "",
  resource_id: (import.meta.env.VITE_IAMKIT_RESOURCE_ID as string | undefined) ?? "",
}

export const IAMKIT_AUDIENCE = (import.meta.env.VITE_IAMKIT_AUDIENCE as string | undefined) ?? ""

export interface TokenPair {
  access_token: string
  refresh_token?: string
  token_type: string
  expires_in: number
}

async function iamRequest<T>(path: string, body: unknown, token?: string): Promise<T> {
  const headers: Record<string, string> = { "Content-Type": "application/json" }
  if (token) headers["Authorization"] = `Bearer ${token}`
  const res = await fetch(`${IAMKIT_URL}/identity/v1${path}`, {
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

/** Logs in against IAMKit with the configured environment/application/resource boundary. */
export function login(email: string, password: string): Promise<TokenPair> {
  return iamRequest<TokenPair>("/login", {
    ...IAMKIT_BOUNDARY,
    email,
    password,
  })
}

export function refresh(refreshToken: string): Promise<TokenPair> {
  return iamRequest<TokenPair>("/refresh", {
    ...IAMKIT_BOUNDARY,
    refresh_token: refreshToken,
  })
}

/** Revokes the current session at IAMKit. Requires the still-live access token. */
export async function logout(): Promise<void> {
  const accessToken = localStorage.getItem("access_token")
  if (!accessToken) return
  try {
    await iamRequest("/logout", {
      environment_id: IAMKIT_BOUNDARY.environment_id,
      audience: IAMKIT_AUDIENCE,
    }, accessToken)
  } catch {
    // best-effort — the tokens are cleared locally regardless
  }
}
