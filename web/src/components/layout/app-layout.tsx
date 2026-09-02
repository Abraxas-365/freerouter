import { Link, useLocation } from "react-router-dom"
import type { LucideIcon } from "lucide-react"
import {
  LayoutDashboard, Server, Box, Key, Gauge, DollarSign, BarChart3,
  Shield, Webhook, Users, UserCog, Mail, Lock,
} from "lucide-react"
import {
  Sidebar, SidebarContent, SidebarFooter, SidebarGroup, SidebarGroupContent,
  SidebarGroupLabel, SidebarHeader, SidebarInset, SidebarMenu, SidebarMenuButton,
  SidebarMenuItem, SidebarProvider, SidebarRail, SidebarSeparator, SidebarTrigger,
} from "@/components/ui/sidebar"
import { Separator } from "@/components/ui/separator"

interface NavItem {
  label: string
  icon: LucideIcon
  href: string
}

const NAV_GROUPS: { label: string; items: NavItem[] }[] = [
  {
    label: "Overview",
    items: [
      { label: "Dashboard", icon: LayoutDashboard, href: "/" },
    ],
  },
  {
    label: "AI Gateway",
    items: [
      { label: "Providers", icon: Server, href: "/providers" },
      { label: "Models", icon: Box, href: "/models" },
      { label: "Provider Keys", icon: Key, href: "/provider-keys" },
      { label: "Gateway Config", icon: Gauge, href: "/gateway" },
    ],
  },
  {
    label: "Billing & Usage",
    items: [
      { label: "Billing", icon: DollarSign, href: "/billing" },
      { label: "Usage", icon: BarChart3, href: "/usage" },
    ],
  },
  {
    label: "Security",
    items: [
      { label: "Guardrails", icon: Shield, href: "/guardrails" },
      { label: "Webhooks", icon: Webhook, href: "/webhooks" },
      { label: "API Keys", icon: Lock, href: "/api-keys" },
    ],
  },
  {
    label: "Organization",
    items: [
      { label: "Users", icon: Users, href: "/users" },
      { label: "Roles", icon: UserCog, href: "/roles" },
      { label: "Invitations", icon: Mail, href: "/invitations" },
    ],
  },
]

export default function AppLayout({ children }: { children: React.ReactNode }) {
  const location = useLocation()

  return (
    <SidebarProvider>
      <Sidebar variant="inset">
        <SidebarHeader>
          <SidebarMenu>
            <SidebarMenuItem>
              <SidebarMenuButton size="lg">
                <div className="flex items-center gap-2">
                  <div className="flex h-7 w-7 items-center justify-center rounded-sm bg-primary text-primary-foreground font-mono text-xs font-bold">
                    FR
                  </div>
                  <div className="flex flex-col">
                    <span className="font-mono text-sm font-semibold">FreeRouter</span>
                    <span className="text-[10px] text-muted-foreground">LLM Gateway</span>
                  </div>
                </div>
              </SidebarMenuButton>
            </SidebarMenuItem>
          </SidebarMenu>
        </SidebarHeader>

        <SidebarSeparator />

        <SidebarContent>
          {NAV_GROUPS.map((group) => (
            <SidebarGroup key={group.label}>
              <SidebarGroupLabel className="font-mono text-[10px] uppercase tracking-wider">
                {group.label}
              </SidebarGroupLabel>
              <SidebarGroupContent>
                <SidebarMenu>
                  {group.items.map((item) => {
                    const isActive = item.href === "/"
                      ? location.pathname === "/"
                      : location.pathname.startsWith(item.href)
                    return (
                      <SidebarMenuItem key={item.href}>
                        <Link to={item.href}>
                          <SidebarMenuButton isActive={isActive} tooltip={item.label}>
                            <item.icon className="h-4 w-4" />
                            <span>{item.label}</span>
                          </SidebarMenuButton>
                        </Link>
                      </SidebarMenuItem>
                    )
                  })}
                </SidebarMenu>
              </SidebarGroupContent>
            </SidebarGroup>
          ))}
        </SidebarContent>

        <SidebarFooter>
          <SidebarMenu>
            <SidebarMenuItem>
              <SidebarMenuButton>
                <div className="flex items-center gap-2">
                  <div className="flex h-6 w-6 items-center justify-center rounded-full bg-muted text-muted-foreground font-mono text-[10px]">
                    A
                  </div>
                  <span className="text-xs text-muted-foreground">admin@acme.co</span>
                </div>
              </SidebarMenuButton>
            </SidebarMenuItem>
          </SidebarMenu>
        </SidebarFooter>

        <SidebarRail />
      </Sidebar>

      <SidebarInset>
        <header className="flex h-12 items-center gap-2 border-b px-4">
          <SidebarTrigger className="-ml-1" />
          <Separator orientation="vertical" className="mr-2 !h-4" />
          <span className="font-mono text-xs text-muted-foreground">freerouter</span>
        </header>
        <main className="flex-1 overflow-auto">
          {children}
        </main>
      </SidebarInset>
    </SidebarProvider>
  )
}
