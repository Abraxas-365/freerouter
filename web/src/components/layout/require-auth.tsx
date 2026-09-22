import { Navigate } from "react-router-dom"
import { useAuth } from "@/hooks/use-auth"

export function RequireAuth({ children }: { children: React.ReactNode }) {
  const { authenticated } = useAuth()
  if (!authenticated) return <Navigate to="/login" replace />
  return <>{children}</>
}
