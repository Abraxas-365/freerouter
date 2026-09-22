import { Navigate } from "react-router-dom"

/** Legacy redirect — API Keys page moved to /service-accounts */
export default function APIKeysPage() {
  return <Navigate to="/service-accounts" replace />
}
