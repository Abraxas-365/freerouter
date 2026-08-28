import { NavLink, Outlet } from "react-router-dom"
import {
  Sidebar,
  SidebarContent,
  SidebarGroup,
  SidebarGroupLabel,
  SidebarGroupContent,
  SidebarMenu,
  SidebarMenuItem,
  SidebarMenuButton,
  SidebarHeader,
  SidebarFooter,
  SidebarInset,
  SidebarProvider,
  SidebarTrigger,
} from "@/components/ui/sidebar"
import { Separator } from "@/components/ui/separator"
import { useAuth } from "@/contexts/auth"
import {
  LayoutDashboard,
  Server,
  Boxes,
  KeyRound,
  CreditCard,
  BarChart3,
  Shield,
  Webhook,
  Settings,
  LogOut,
} from "lucide-react"
import type { LucideIcon } from "lucide-react"

interface NavItem {
  to: string
  icon: LucideIcon
  label: string
}

const gatewayNav: NavItem[] = [
  { to: "/", icon: LayoutDashboard, label: "Dashboard" },
  { to: "/providers", icon: Server, label: "Providers" },
  { to: "/models", icon: Boxes, label: "Models" },
  { to: "/api-keys", icon: KeyRound, label: "API Keys" },
]

const operationsNav: NavItem[] = [
  { to: "/billing", icon: CreditCard, label: "Billing" },
  { to: "/usage", icon: BarChart3, label: "Usage" },
  { to: "/guardrails", icon: Shield, label: "Guardrails" },
  { to: "/webhooks", icon: Webhook, label: "Webhooks" },
]

const systemNav: NavItem[] = [
  { to: "/settings", icon: Settings, label: "Settings" },
]

function NavSection({ label, items }: { label: string; items: NavItem[] }) {
  return (
    <SidebarGroup>
      <SidebarGroupLabel>{label}</SidebarGroupLabel>
      <SidebarGroupContent>
        <SidebarMenu>
          {items.map((item) => (
            <SidebarMenuItem key={item.to}>
              <NavLink
                to={item.to}
                end={item.to === "/"}
              >
                {({ isActive }) => (
                  <SidebarMenuButton isActive={isActive} tooltip={item.label}>
                    <item.icon className="h-4 w-4" />
                    <span>{item.label}</span>
                  </SidebarMenuButton>
                )}
              </NavLink>
            </SidebarMenuItem>
          ))}
        </SidebarMenu>
      </SidebarGroupContent>
    </SidebarGroup>
  )
}

export function AppLayout() {
  const { user, logout } = useAuth()

  return (
    <SidebarProvider>
      <Sidebar>
        <SidebarHeader className="p-4">
          <div className="flex items-center gap-2">
            <div className="flex h-8 w-8 items-center justify-center rounded-lg bg-primary text-primary-foreground font-bold text-sm">
              FR
            </div>
            <div className="flex flex-col">
              <span className="text-sm font-semibold">FreeRouter</span>
              <span className="text-xs text-muted-foreground">LLM Gateway</span>
            </div>
          </div>
        </SidebarHeader>

        <SidebarContent>
          <NavSection label="Gateway" items={gatewayNav} />
          <NavSection label="Operations" items={operationsNav} />
          <NavSection label="System" items={systemNav} />
        </SidebarContent>

        <SidebarFooter className="p-4">
          {user && (
            <div className="flex items-center justify-between">
              <div className="flex flex-col truncate">
                <span className="text-sm font-medium truncate">{user.name}</span>
                <span className="text-xs text-muted-foreground truncate">
                  {user.email}
                </span>
              </div>
              <button
                onClick={logout}
                className="text-muted-foreground hover:text-foreground p-1"
                title="Log out"
              >
                <LogOut className="h-4 w-4" />
              </button>
            </div>
          )}
        </SidebarFooter>
      </Sidebar>

      <SidebarInset>
        <header className="flex h-14 items-center gap-2 border-b px-4">
          <SidebarTrigger />
          <Separator orientation="vertical" className="h-6" />
        </header>
        <main className="flex-1 p-6">
          <Outlet />
        </main>
      </SidebarInset>
    </SidebarProvider>
  )
}
