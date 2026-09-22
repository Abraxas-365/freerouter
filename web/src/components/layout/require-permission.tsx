import { Navigate } from "react-router-dom"
import { usePermissions } from "@/hooks/use-permissions"

/**
 * Route guard that checks the JWT for at least one of the required permissions.
 * Redirects to Dashboard if the user lacks access.
 */
export function RequirePermission({
  any,
  children,
}: {
  /** User needs at least one of these permissions. */
  any: string[]
  children: React.ReactNode
}) {
  const perms = usePermissions()
  if (!perms.hasAny(...any)) return <Navigate to="/" replace />
  return <>{children}</>
}
