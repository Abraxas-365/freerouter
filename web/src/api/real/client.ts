import { refresh as iamkitRefresh } from "@/lib/iamkit"

const API_BASE = (import.meta.env.VITE_API_URL as string | undefined) ?? ""

let refreshInFlight: Promise<string | null> | null = null

/** Attempts to exchange the stored refresh token for a new access token. Coalesces concurrent callers. */
async function tryRefresh(): Promise<string | null> {
  if (refreshInFlight) return refreshInFlight
  refreshInFlight = (async () => {
    const refreshToken = localStorage.getItem("refresh_token")
    if (!refreshToken) return null
    try {
      const tokens = await iamkitRefresh(refreshToken)
      localStorage.setItem("access_token", tokens.access_token)
      if (tokens.refresh_token) localStorage.setItem("refresh_token", tokens.refresh_token)
      return tokens.access_token
    } catch {
      return null
    }
  })()
  try {
    return await refreshInFlight
  } finally {
    refreshInFlight = null
  }
}

function clearSession() {
  localStorage.removeItem("access_token")
  localStorage.removeItem("refresh_token")
  window.dispatchEvent(new Event("auth-expired"))
}

async function request<T>(method: string, path: string, body?: unknown, isRetry = false): Promise<T> {
  const token = localStorage.getItem("access_token")
  const headers: Record<string, string> = { "Content-Type": "application/json" }
  if (token) headers["Authorization"] = `Bearer ${token}`

  const res = await fetch(`${API_BASE}${path}`, {
    method,
    headers,
    body: body ? JSON.stringify(body) : undefined,
  })

  if (res.status === 401) {
    if (!isRetry) {
      const newToken = await tryRefresh()
      if (newToken) return request<T>(method, path, body, true)
    }
    clearSession()
    throw new Error("Unauthorized")
  }

  if (!res.ok) {
    const err = await res.json().catch(() => ({ message: res.statusText }))
    throw new Error(err.message || res.statusText)
  }

  if (res.status === 204) return undefined as T
  return res.json()
}

function qs(params: Record<string, unknown>): string {
  const parts: string[] = []
  for (const [k, v] of Object.entries(params)) {
    if (v !== undefined && v !== null) parts.push(`${k}=${encodeURIComponent(String(v))}`)
  }
  return parts.length ? `?${parts.join("&")}` : ""
}

export const api = {
  get: <T>(path: string) => request<T>("GET", path),
  post: <T>(path: string, body?: unknown) => request<T>("POST", path, body),
  put: <T>(path: string, body?: unknown) => request<T>("PUT", path, body),
  patch: <T>(path: string, body?: unknown) => request<T>("PATCH", path, body),
  del: <T>(path: string, body?: unknown) => request<T>("DELETE", path, body),
}

export { qs }
