import { BrowserRouter, Routes, Route, Navigate } from "react-router-dom"
import { AuthProvider, useAuth } from "@/contexts/auth"
import { ApiProvider } from "@/api"
import { TooltipProvider } from "@/components/ui/tooltip"
import { AppLayout } from "@/components/layout/app-layout"
import { DashboardPage } from "@/pages/dashboard/page"
import { ProvidersPage } from "@/pages/providers/page"
import { ModelsPage } from "@/pages/models/page"
import { ApiKeysPage } from "@/pages/api-keys/page"
import { BillingPage } from "@/pages/billing/page"
import { UsagePage } from "@/pages/usage/page"
import { GuardrailsPage } from "@/pages/guardrails/page"
import { WebhooksPage } from "@/pages/webhooks/page"
import { SettingsPage } from "@/pages/settings/page"
import { LoginPage } from "@/pages/auth/login"
import type { ReactNode } from "react"

function RequireAuth({ children }: { children: ReactNode }) {
  const { user, loading } = useAuth()

  if (loading) {
    return (
      <div className="flex min-h-screen items-center justify-center">
        <div className="text-muted-foreground">Loading...</div>
      </div>
    )
  }

  if (!user) {
    return <Navigate to="/login" replace />
  }

  return <>{children}</>
}

export default function App() {
  return (
    <BrowserRouter>
      <AuthProvider>
        <ApiProvider>
          <TooltipProvider>
            <Routes>
              <Route path="/login" element={<LoginPage />} />

              <Route
                element={
                  <RequireAuth>
                    <AppLayout />
                  </RequireAuth>
                }
              >
                <Route index element={<DashboardPage />} />
                <Route path="providers" element={<ProvidersPage />} />
                <Route path="models" element={<ModelsPage />} />
                <Route path="api-keys" element={<ApiKeysPage />} />
                <Route path="billing" element={<BillingPage />} />
                <Route path="usage" element={<UsagePage />} />
                <Route path="guardrails" element={<GuardrailsPage />} />
                <Route path="webhooks" element={<WebhooksPage />} />
                <Route path="settings" element={<SettingsPage />} />
              </Route>
            </Routes>
          </TooltipProvider>
        </ApiProvider>
      </AuthProvider>
    </BrowserRouter>
  )
}
