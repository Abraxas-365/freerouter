import { useState } from "react"
import { Navigate } from "react-router-dom"
import { Zap, Loader2 } from "lucide-react"
import { useAuth } from "@/hooks/use-auth"
import { Button } from "@/components/ui/button"
import { Card, CardContent, CardHeader } from "@/components/ui/card"
import { Input } from "@/components/ui/input"
import { Label } from "@/components/ui/label"

/**
 * Sign-in for the server's auth mode: IAMKit email + password (iam) or the
 * admin token (token). In none mode the console is always signed in.
 */
export default function LoginPage() {
  const auth = useAuth()
  const [email, setEmail] = useState("")
  const [password, setPassword] = useState("")
  const [token, setToken] = useState("")
  const [submitting, setSubmitting] = useState(false)

  if (auth.authenticated) return <Navigate to="/" replace />

  async function handleSubmit(e: React.FormEvent) {
    e.preventDefault()
    setSubmitting(true)
    try {
      if (auth.mode === "token") await auth.loginWithToken(token)
      else await auth.login(email, password)
    } catch {
      // error is surfaced via auth.error
    } finally {
      setSubmitting(false)
    }
  }

  return (
    <main className="flex min-h-dvh items-center justify-center bg-muted/30 p-6">
      <Card className="w-full max-w-sm">
        <CardHeader className="space-y-3">
          <div className="flex h-9 w-9 items-center justify-center rounded-lg bg-primary text-primary-foreground">
            <Zap className="h-5 w-5" />
          </div>
          <div>
            <h1 className="font-mono text-xl font-bold tracking-tight">FreeRouter</h1>
            <p className="mt-1 text-sm text-muted-foreground">Sign in to the operator console.</p>
          </div>
        </CardHeader>
        <CardContent>
          {auth.mode === "none" ? (
            // Only reached when the server refused the request (e.g. not on localhost).
            <p className="text-sm text-muted-foreground">
              This server runs without authentication (<code>FREEROUTER_AUTH=none</code>) and only
              answers to <code>localhost</code>. Open the console from the same machine, or run the
              server with <code>FREEROUTER_AUTH=token</code>.
            </p>
          ) : (
            <form className="space-y-4" onSubmit={handleSubmit}>
              {auth.mode === "token" ? (
                <div className="space-y-1.5">
                  <Label htmlFor="token">Admin token</Label>
                  <Input
                    id="token"
                    type="password"
                    autoComplete="current-password"
                    required
                    disabled={submitting}
                    value={token}
                    onChange={(e) => setToken(e.target.value)}
                  />
                  <p className="text-xs text-muted-foreground">
                    The server's <code>FREEROUTER_ADMIN_TOKEN</code>.
                  </p>
                </div>
              ) : (
                <>
                  <div className="space-y-1.5">
                    <Label htmlFor="email">Email</Label>
                    <Input
                      id="email"
                      type="email"
                      autoComplete="username"
                      required
                      disabled={submitting}
                      value={email}
                      onChange={(e) => setEmail(e.target.value)}
                    />
                  </div>
                  <div className="space-y-1.5">
                    <Label htmlFor="password">Password</Label>
                    <Input
                      id="password"
                      type="password"
                      autoComplete="current-password"
                      required
                      disabled={submitting}
                      value={password}
                      onChange={(e) => setPassword(e.target.value)}
                    />
                  </div>
                </>
              )}
              {auth.error && (
                <p className="text-sm text-destructive">{auth.error}</p>
              )}
              <Button type="submit" className="w-full" disabled={submitting}>
                {submitting ? <Loader2 className="h-4 w-4 animate-spin" /> : "Sign in"}
              </Button>
            </form>
          )}
        </CardContent>
      </Card>
    </main>
  )
}
