import { BrowserRouter, Routes, Route } from "react-router-dom"
import AppLayout from "./components/layout/app-layout"
import { Toaster } from "./components/ui/sonner"
import Dashboard from "./pages/dashboard"
import ProvidersPage from "./pages/providers"
import ModelsPage from "./pages/models"
import ProviderKeysPage from "./pages/provider-keys"
import GatewayConfigPage from "./pages/gateway-config"
import BillingPage from "./pages/billing"
import UsagePage from "./pages/usage"
import GuardrailsPage from "./pages/guardrails"
import WebhooksPage from "./pages/webhooks"
import ApiKeysPage from "./pages/api-keys"

export default function App() {
  return (
    <BrowserRouter>
      <AppLayout>
        <Routes>
          <Route path="/" element={<Dashboard />} />
          <Route path="/providers" element={<ProvidersPage />} />
          <Route path="/models" element={<ModelsPage />} />
          <Route path="/provider-keys" element={<ProviderKeysPage />} />
          <Route path="/gateway" element={<GatewayConfigPage />} />
          <Route path="/billing" element={<BillingPage />} />
          <Route path="/usage" element={<UsagePage />} />
          <Route path="/guardrails" element={<GuardrailsPage />} />
          <Route path="/webhooks" element={<WebhooksPage />} />
          <Route path="/api-keys" element={<ApiKeysPage />} />
        </Routes>
      </AppLayout>
      <Toaster />
    </BrowserRouter>
  )
}
