import {
  createContext, useCallback, useContext, useEffect, useMemo, useState, type ReactNode,
} from "react"
import * as iamkit from "@/lib/iamkit"

const ACCESS_TOKEN_KEY = "access_token"
const REFRESH_TOKEN_KEY = "refresh_token"

export interface AuthState {
  authenticated: boolean
  loading: boolean
  error: string | null
  login: (email: string, password: string) => Promise<void>
  logout: () => Promise<void>
}

const AuthContext = createContext<AuthState | null>(null)

/**
 * Tracks whether the console holds a usable IAMKit access token.
 *
 * FreeRouter has no session endpoint of its own (no `/auth/me`, no orgs) —
 * authentication is entirely delegated to IAMKit. The console only needs to
 * know "do we have a token", not who the user is; per-request authorization
 * is enforced server-side by IAMKit-issued permissions.
 */
export function AuthProvider({ children }: { children: ReactNode }) {
  const [authenticated, setAuthenticated] = useState(() => !!localStorage.getItem(ACCESS_TOKEN_KEY))
  const [loading, setLoading] = useState(false)
  const [error, setError] = useState<string | null>(null)

  useEffect(() => {
    function onExpired() {
      setAuthenticated(false)
    }
    window.addEventListener("auth-expired", onExpired)
    return () => window.removeEventListener("auth-expired", onExpired)
  }, [])

  const login = useCallback(async (email: string, password: string) => {
    setLoading(true)
    setError(null)
    try {
      const tokens = await iamkit.login(email, password)
      localStorage.setItem(ACCESS_TOKEN_KEY, tokens.access_token)
      if (tokens.refresh_token) localStorage.setItem(REFRESH_TOKEN_KEY, tokens.refresh_token)
      setAuthenticated(true)
    } catch (err) {
      setError(err instanceof Error ? err.message : "Sign-in failed")
      throw err
    } finally {
      setLoading(false)
    }
  }, [])

  const logout = useCallback(async () => {
    await iamkit.logout()
    localStorage.removeItem(ACCESS_TOKEN_KEY)
    localStorage.removeItem(REFRESH_TOKEN_KEY)
    setAuthenticated(false)
  }, [])

  const value = useMemo(
    () => ({ authenticated, loading, error, login, logout }),
    [authenticated, loading, error, login, logout],
  )

  return <AuthContext.Provider value={value}>{children}</AuthContext.Provider>
}

export function useAuth(): AuthState {
  const ctx = useContext(AuthContext)
  if (!ctx) throw new Error("useAuth must be used within AuthProvider")
  return ctx
}
