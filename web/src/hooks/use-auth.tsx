import {
  createContext, useCallback, useContext, useEffect, useMemo, useState, type ReactNode,
} from "react"
import * as iamkit from "@/lib/iamkit"
import { API_BASE, loadRuntimeConfig, type AuthMode } from "@/lib/runtime-config"

const ACCESS_TOKEN_KEY = "access_token"
const REFRESH_TOKEN_KEY = "refresh_token"

export interface AuthState {
  /** Server auth mode (FREEROUTER_AUTH). */
  mode: AuthMode
  authenticated: boolean
  /** Permissions of the signed-in principal. */
  permissions: Set<string>
  loading: boolean
  error: string | null
  /** iam mode: IAMKit email + password. */
  login: (email: string, password: string) => Promise<void>
  /** token mode: the admin token, traded for an HttpOnly session cookie. */
  loginWithToken: (token: string) => Promise<void>
  logout: () => Promise<void>
}

const AuthContext = createContext<AuthState | null>(null)

/** Decode the JWT payload without verification (validation happens server-side). */
function jwtPermissions(token: string | null): Set<string> {
  if (!token) return new Set()
  try {
    const parts = token.split(".")
    if (parts.length !== 3) return new Set()
    const payload = JSON.parse(atob(parts[1].replace(/-/g, "+").replace(/_/g, "/")))
    return new Set(Array.isArray(payload.permissions) ? (payload.permissions as string[]) : [])
  } catch {
    return new Set()
  }
}

interface SessionState {
  authenticated: boolean
  permissions: string[]
}

function sessionRequest(method: string, body?: unknown): Promise<Response> {
  return fetch(`${API_BASE}/api/v1/session`, {
    method,
    headers: body ? { "Content-Type": "application/json" } : undefined,
    body: body ? JSON.stringify(body) : undefined,
  })
}

async function errorMessage(res: Response): Promise<string> {
  const body = await res.json().catch(() => null)
  return body?.error?.message || body?.message || res.statusText
}

/**
 * Tracks who the console is signed in as, in the server's auth mode:
 *  - iam:   an IAMKit access token in localStorage (permissions from the JWT)
 *  - token: an HttpOnly session cookie set by POST /api/v1/session
 *  - none:  always signed in as the local owner
 * The mode comes from GET /api/v1/config; authorization is enforced
 * server-side either way — the console only hides what it can't use.
 */
export function AuthProvider({ children }: { children: ReactNode }) {
  const [mode, setMode] = useState<AuthMode | null>(null)
  const [authenticated, setAuthenticated] = useState(false)
  const [permissions, setPermissions] = useState<Set<string>>(new Set())
  const [loading, setLoading] = useState(false)
  const [error, setError] = useState<string | null>(null)

  const applySession = useCallback((s: SessionState) => {
    setAuthenticated(s.authenticated)
    setPermissions(new Set(s.authenticated ? s.permissions : []))
  }, [])

  useEffect(() => {
    let cancelled = false
    ;(async () => {
      const cfg = await loadRuntimeConfig()
      if (cfg.auth === "iam") {
        const token = localStorage.getItem(ACCESS_TOKEN_KEY)
        if (cancelled) return
        setAuthenticated(!!token)
        setPermissions(jwtPermissions(token))
      } else {
        const res = await sessionRequest("GET").catch(() => null)
        const s: SessionState = res?.ok ? await res.json() : { authenticated: false, permissions: [] }
        if (cancelled) return
        applySession(s)
      }
      setMode(cfg.auth)
    })()
    return () => {
      cancelled = true
    }
  }, [applySession])

  useEffect(() => {
    function onExpired() {
      setAuthenticated(false)
      setPermissions(new Set())
    }
    function onRefreshed() {
      setPermissions(jwtPermissions(localStorage.getItem(ACCESS_TOKEN_KEY)))
    }
    window.addEventListener("auth-expired", onExpired)
    window.addEventListener("auth-refreshed", onRefreshed)
    return () => {
      window.removeEventListener("auth-expired", onExpired)
      window.removeEventListener("auth-refreshed", onRefreshed)
    }
  }, [])

  const run = useCallback(async (fn: () => Promise<void>) => {
    setLoading(true)
    setError(null)
    try {
      await fn()
    } catch (err) {
      setError(err instanceof Error ? err.message : "Sign-in failed")
      throw err
    } finally {
      setLoading(false)
    }
  }, [])

  const login = useCallback((email: string, password: string) => run(async () => {
    const tokens = await iamkit.login(email, password)
    localStorage.setItem(ACCESS_TOKEN_KEY, tokens.access_token)
    if (tokens.refresh_token) localStorage.setItem(REFRESH_TOKEN_KEY, tokens.refresh_token)
    setPermissions(jwtPermissions(tokens.access_token))
    setAuthenticated(true)
  }), [run])

  const loginWithToken = useCallback((token: string) => run(async () => {
    const res = await sessionRequest("POST", { token })
    if (!res.ok) throw new Error(res.status === 401 ? "Invalid admin token" : await errorMessage(res))
    applySession(await res.json())
  }), [run, applySession])

  const logout = useCallback(async () => {
    if (mode === "iam") {
      await iamkit.logout()
      localStorage.removeItem(ACCESS_TOKEN_KEY)
      localStorage.removeItem(REFRESH_TOKEN_KEY)
    } else if (mode === "token") {
      // Only show signed-out once the server cleared the cookie.
      const res = await sessionRequest("DELETE").catch(() => null)
      if (!res?.ok) throw new Error("Sign out failed — try again")
    } else {
      return // none: nothing to sign out of
    }
    setAuthenticated(false)
    setPermissions(new Set())
  }, [mode])

  const value = useMemo(
    () => (mode === null ? null : { mode, authenticated, permissions, loading, error, login, loginWithToken, logout }),
    [mode, authenticated, permissions, loading, error, login, loginWithToken, logout],
  )

  // Nothing to show until we know the mode and whether we're signed in.
  if (value === null) return null

  return <AuthContext.Provider value={value}>{children}</AuthContext.Provider>
}

export function useAuth(): AuthState {
  const ctx = useContext(AuthContext)
  if (!ctx) throw new Error("useAuth must be used within AuthProvider")
  return ctx
}
