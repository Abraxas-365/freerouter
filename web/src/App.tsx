import { BrowserRouter, Routes, Route } from "react-router-dom"
import AppLayout from "./components/layout/app-layout"
import { RequireAuth } from "./components/layout/require-auth"
import { RequirePermission } from "./components/layout/require-permission"
import { P } from "./hooks/use-permissions"
import { Toaster } from "./components/ui/sonner"
import LoginPage from "./pages/login"
import Dashboard from "./pages/dashboard"
import ProvidersPage from "./pages/providers"
import ModelsPage from "./pages/models"
import ProviderKeysPage from "./pages/provider-keys"
import RateLimitsPage from "./pages/rate-limits"
import UsagePage from "./pages/usage"
import GuardrailsPage from "./pages/guardrails"
import WebhooksPage from "./pages/webhooks"
import APIKeysPage from "./pages/api-keys"
import ServiceAccountsPage from "./pages/service-accounts"
import AccessPage from "./pages/access"

export default function App() {
  return (
    <BrowserRouter>
      <Routes>
        <Route path="/login" element={<LoginPage />} />
        <Route
          path="/*"
          element={
            <RequireAuth>
              <AppLayout>
                <Routes>
                  <Route path="/" element={<Dashboard />} />
                  <Route path="/providers" element={<RequirePermission any={[P.ProvidersRead, P.ProvidersWrite]}><ProvidersPage /></RequirePermission>} />
                  <Route path="/models" element={<RequirePermission any={[P.ProvidersRead, P.ProvidersWrite]}><ModelsPage /></RequirePermission>} />
                  <Route path="/provider-keys" element={<RequirePermission any={[P.ProviderKeysRead, P.ProviderKeysWrite]}><ProviderKeysPage /></RequirePermission>} />
                  <Route path="/rate-limits" element={<RequirePermission any={[P.RateLimitsRead, P.RateLimitsWrite]}><RateLimitsPage /></RequirePermission>} />
                  <Route path="/usage" element={<RequirePermission any={[P.UsageRead]}><UsagePage /></RequirePermission>} />
                  <Route path="/guardrails" element={<RequirePermission any={[P.GuardrailsRead, P.GuardrailsWrite]}><GuardrailsPage /></RequirePermission>} />
                  <Route path="/webhooks" element={<RequirePermission any={[P.WebhooksRead, P.WebhooksWrite]}><WebhooksPage /></RequirePermission>} />
                  <Route path="/api-keys" element={<RequirePermission any={[P.ServiceAccountsRead, P.ServiceAccountsWrite]}><APIKeysPage /></RequirePermission>} />
                  <Route path="/service-accounts" element={<RequirePermission any={[P.ServiceAccountsRead, P.ServiceAccountsWrite]}><ServiceAccountsPage /></RequirePermission>} />
                  <Route path="/access" element={<RequirePermission any={[P.UsersRead, P.UsersWrite, P.RolesRead, P.RolesWrite]}><AccessPage /></RequirePermission>} />
                </Routes>
              </AppLayout>
            </RequireAuth>
          }
        />
      </Routes>
      <Toaster />
    </BrowserRouter>
  )
}
