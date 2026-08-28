import { useSearchParams, Navigate } from "react-router-dom"
import { useAuth } from "@/contexts/auth"
import { useEffect } from "react"

export function LoginPage() {
  const { login, user } = useAuth()
  const [params] = useSearchParams()

  useEffect(() => {
    const token = params.get("token")
    if (token) {
      login(token)
    }
  }, [params, login])

  if (user) {
    return <Navigate to="/" replace />
  }

  return (
    <div className="flex min-h-screen items-center justify-center">
      <div className="w-full max-w-sm space-y-6 p-6">
        <div className="space-y-2 text-center">
          <div className="flex items-center justify-center gap-2">
            <div className="flex h-10 w-10 items-center justify-center rounded-lg bg-primary text-primary-foreground font-bold">
              FR
            </div>
          </div>
          <h1 className="text-2xl font-bold">FreeRouter</h1>
          <p className="text-muted-foreground">
            Sign in to manage your LLM gateway
          </p>
        </div>

        <div className="space-y-3">
          <a
            href="/auth/login?provider=google"
            className="flex w-full items-center justify-center gap-2 rounded-md border px-4 py-2 text-sm font-medium hover:bg-accent"
          >
            Sign in with Google
          </a>
          <a
            href="/auth/login?provider=microsoft"
            className="flex w-full items-center justify-center gap-2 rounded-md border px-4 py-2 text-sm font-medium hover:bg-accent"
          >
            Sign in with Microsoft
          </a>
        </div>
      </div>
    </div>
  )
}
