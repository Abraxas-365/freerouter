async function request<T>(method: string, path: string, body?: unknown): Promise<T> {
  const token = localStorage.getItem("access_token")
  const headers: Record<string, string> = { "Content-Type": "application/json" }
  if (token) headers["Authorization"] = `Bearer ${token}`

  const res = await fetch(path, {
    method,
    headers,
    body: body ? JSON.stringify(body) : undefined,
  })

  if (res.status === 401) {
    localStorage.removeItem("access_token")
    window.location.href = "/login"
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
  del: <T>(path: string, body?: unknown) => request<T>("DELETE", path, body),
}

export { qs }
