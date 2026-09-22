import { useState } from "react"
import { Button } from "@/components/ui/button"
import { Badge } from "@/components/ui/badge"
import {
  Card, CardContent, CardDescription, CardHeader, CardTitle,
} from "@/components/ui/card"
import { Input } from "@/components/ui/input"
import { Label } from "@/components/ui/label"
import { Separator } from "@/components/ui/separator"
import { Tabs, TabsContent, TabsList, TabsTrigger } from "@/components/ui/tabs"
import {
  Table, TableBody, TableCell, TableHead, TableHeader, TableRow,
} from "@/components/ui/table"
import {
  Select, SelectContent, SelectItem, SelectTrigger, SelectValue,
} from "@/components/ui/select"
import {
  Dialog, DialogContent, DialogDescription, DialogFooter,
  DialogHeader, DialogTitle, DialogTrigger,
} from "@/components/ui/dialog"
import {
  DropdownMenu, DropdownMenuContent, DropdownMenuItem,
  DropdownMenuLabel, DropdownMenuSeparator, DropdownMenuTrigger,
} from "@/components/ui/dropdown-menu"
import { Avatar, AvatarFallback } from "@/components/ui/avatar"
import { Skeleton } from "@/components/ui/skeleton"
import { Switch } from "@/components/ui/switch"
import { Progress } from "@/components/ui/progress"
import { Tooltip, TooltipContent, TooltipProvider, TooltipTrigger } from "@/components/ui/tooltip"
import {
  Sidebar, SidebarContent, SidebarFooter, SidebarGroup, SidebarGroupLabel,
  SidebarHeader, SidebarMenu, SidebarMenuButton, SidebarMenuItem,
  SidebarProvider, SidebarTrigger, SidebarInset,
} from "@/components/ui/sidebar"
import {
  LayoutDashboard, Server, Boxes, KeyRound, CreditCard, BarChart3,
  Shield, Webhook, Settings, LogOut, MoreHorizontal, Plus, Copy,
  Eye, EyeOff, Trash2, Pencil, ChevronRight, Activity,
  Zap, DollarSign, Clock, AlertTriangle, Check, X,
} from "lucide-react"
import {
  AreaChart, Area, BarChart, Bar, XAxis, YAxis, CartesianGrid,
  Tooltip as RechartsTooltip, ResponsiveContainer, PieChart, Pie, Cell,
} from "recharts"

// ── Chart data ──
const usageData = [
  { date: "Mon", requests: 2400, cost: 12.4 },
  { date: "Tue", requests: 1398, cost: 8.2 },
  { date: "Wed", requests: 4800, cost: 24.1 },
  { date: "Thu", requests: 3908, cost: 18.7 },
  { date: "Fri", requests: 4200, cost: 21.3 },
  { date: "Sat", requests: 2100, cost: 10.5 },
  { date: "Sun", requests: 1800, cost: 9.1 },
]

const modelUsageData = [
  { model: "GPT-4o", requests: 4200, cost: 42.0 },
  { model: "Claude 4", requests: 2800, cost: 38.5 },
  { model: "Gemini 2.5", requests: 1900, cost: 19.0 },
  { model: "GPT-4o Mini", requests: 3100, cost: 4.6 },
  { model: "DeepSeek", requests: 800, cost: 0.4 },
]

const pieData = [
  { name: "OpenAI", value: 45 },
  { name: "Anthropic", value: 28 },
  { name: "Google", value: 15 },
  { name: "DeepSeek", value: 8 },
  { name: "Other", value: 4 },
]

const CHART_COLORS = ["#7b9bae", "#5a7a4a", "#b8a06a", "#a65d4e", "#8a7b9b"]

function Section({ id, title, children }: { id: string; title: string; children: React.ReactNode }) {
  return (
    <section id={id}>
      <h2 className="font-mono text-sm font-semibold uppercase tracking-widest text-primary glow mb-6">
        {`// ${title}`}
      </h2>
      {children}
    </section>
  )
}

function Swatch({ label, bg, border }: { label: string; bg: string; border?: boolean }) {
  return (
    <div className="flex items-center gap-3">
      <div
        className={"h-10 w-10 rounded-sm shrink-0 " + (border ? "border " : "")}
        style={{ backgroundColor: bg }}
      />
      <div>
        <p className="text-sm font-medium">{label}</p>
        <p className="text-xs text-muted-foreground font-mono">{bg}</p>
      </div>
    </div>
  )
}

// ── Sidebar navigation items ──
const NAV = [
  {
    group: "Gateway",
    items: [
      { label: "Dashboard", icon: LayoutDashboard, active: true },
      { label: "Providers", icon: Server },
      { label: "Models", icon: Boxes },
      { label: "API Keys", icon: KeyRound },
    ],
  },
  {
    group: "Operations",
    items: [
      { label: "Billing", icon: CreditCard },
      { label: "Usage", icon: BarChart3 },
      { label: "Guardrails", icon: Shield },
      { label: "Webhooks", icon: Webhook },
    ],
  },
  {
    group: "System",
    items: [{ label: "Settings", icon: Settings }],
  },
]

export default function DesignSystem() {
  const [mode, setMode] = useState<"dark" | "light">("dark")
  const [showKey, setShowKey] = useState(false)

  return (
    <div className={mode === "light" ? "light" : ""}>
      <TooltipProvider>
        <SidebarProvider>
          <div className="min-h-screen bg-background text-foreground flex w-full">

            {/* ── Sidebar ── */}
            <Sidebar>
              <SidebarHeader className="p-4">
                <div className="flex items-center gap-2.5">
                  <div className="h-8 w-8 rounded-md bg-primary flex items-center justify-center text-primary-foreground text-xs font-bold font-mono">
                    FR
                  </div>
                  <div>
                    <p className="text-sm font-semibold font-mono">FreeRouter</p>
                    <p className="text-xs text-muted-foreground">LLM Gateway</p>
                  </div>
                </div>
              </SidebarHeader>

              <SidebarContent>
                {NAV.map((section) => (
                  <SidebarGroup key={section.group}>
                    <SidebarGroupLabel className="font-mono text-[10px] uppercase tracking-widest">
                      {section.group}
                    </SidebarGroupLabel>
                    <SidebarMenu>
                      {section.items.map((item) => (
                        <SidebarMenuItem key={item.label}>
                          <SidebarMenuButton isActive={item.active} tooltip={item.label}>
                            <item.icon className="h-4 w-4" />
                            <span>{item.label}</span>
                          </SidebarMenuButton>
                        </SidebarMenuItem>
                      ))}
                    </SidebarMenu>
                  </SidebarGroup>
                ))}
              </SidebarContent>

              <SidebarFooter className="p-4">
                <DropdownMenu>
                  <DropdownMenuTrigger>
                    <button className="flex items-center gap-2.5 w-full text-left hover:bg-sidebar-accent rounded-md p-1.5 -m-1.5 transition-colors">
                      <Avatar className="h-7 w-7">
                        <AvatarFallback className="text-[10px] font-mono bg-muted">AU</AvatarFallback>
                      </Avatar>
                      <div className="flex-1 min-w-0">
                        <p className="text-sm font-medium truncate">Admin User</p>
                        <p className="text-xs text-muted-foreground truncate">admin@example.com</p>
                      </div>
                    </button>
                  </DropdownMenuTrigger>
                  <DropdownMenuContent align="start" className="w-48">
                    <DropdownMenuLabel className="font-mono text-xs">Account</DropdownMenuLabel>
                    <DropdownMenuSeparator />
                    <DropdownMenuItem><Settings className="h-4 w-4 mr-2" />Settings</DropdownMenuItem>
                    <DropdownMenuSeparator />
                    <DropdownMenuItem className="text-destructive"><LogOut className="h-4 w-4 mr-2" />Sign out</DropdownMenuItem>
                  </DropdownMenuContent>
                </DropdownMenu>
              </SidebarFooter>
            </Sidebar>

            {/* ── Main content ── */}
            <SidebarInset>
              {/* Top bar */}
              <header className="sticky top-0 z-50 border-b bg-background/90 backdrop-blur-sm">
                <div className="px-6 h-12 flex items-center justify-between">
                  <div className="flex items-center gap-3">
                    <SidebarTrigger />
                    <span className="text-sm font-mono text-muted-foreground">Design System</span>
                  </div>
                  <Button size="sm" variant="outline" onClick={() => setMode(mode === "dark" ? "light" : "dark")}>
                    <span className="font-mono text-xs">{mode === "dark" ? "[light]" : "[dark]"}</span>
                  </Button>
                </div>
              </header>

              <main className="px-6 py-10 space-y-14 max-w-6xl">

                {/* ── Colors ── */}
                <Section id="colors" title="colors">
                  <div className="space-y-8">
                    <div>
                      <p className="text-xs text-muted-foreground font-mono mb-4">/* surfaces */</p>
                      <div className="grid grid-cols-2 sm:grid-cols-4 gap-4">
                        <Swatch label="background" bg="var(--background)" border />
                        <Swatch label="card" bg="var(--card)" border />
                        <Swatch label="muted" bg="var(--muted)" />
                        <Swatch label="border" bg="var(--border)" />
                      </div>
                    </div>
                    <div>
                      <p className="text-xs text-muted-foreground font-mono mb-4">/* text */</p>
                      <div className="grid grid-cols-2 sm:grid-cols-4 gap-4">
                        <Swatch label="foreground" bg="var(--foreground)" />
                        <Swatch label="muted-fg" bg="var(--muted-foreground)" />
                        <Swatch label="card-fg" bg="var(--card-foreground)" />
                        <Swatch label="secondary-fg" bg="var(--secondary-foreground)" />
                      </div>
                    </div>
                    <div>
                      <p className="text-xs text-muted-foreground font-mono mb-4">/* signals */</p>
                      <div className="grid grid-cols-2 sm:grid-cols-4 gap-4">
                        <Swatch label="primary" bg="var(--primary)" />
                        <Swatch label="destructive" bg="var(--destructive)" />
                        <Swatch label="success" bg="var(--success)" />
                        <Swatch label="warning" bg="var(--warning)" />
                      </div>
                    </div>
                    <div>
                      <p className="text-xs text-muted-foreground font-mono mb-4">/* data */</p>
                      <div className="flex gap-3">
                        {[1, 2, 3, 4, 5].map((n) => (
                          <div key={n} className="text-center">
                            <div className="h-12 w-12 rounded-sm" style={{ backgroundColor: `var(--chart-${n})` }} />
                            <p className="text-xs text-muted-foreground mt-1.5 font-mono">c{n}</p>
                          </div>
                        ))}
                      </div>
                    </div>
                  </div>
                </Section>

                <Separator />

                {/* ── Typography ── */}
                <Section id="typography" title="typography">
                  <div className="space-y-5">
                    <div className="flex items-baseline gap-4">
                      <span className="text-xs text-muted-foreground font-mono w-14 shrink-0 text-right">h1</span>
                      <span className="font-mono text-3xl font-bold tracking-tight">LLM Gateway</span>
                    </div>
                    <div className="flex items-baseline gap-4">
                      <span className="text-xs text-muted-foreground font-mono w-14 shrink-0 text-right">h2</span>
                      <span className="font-mono text-2xl font-semibold tracking-tight">Provider Config</span>
                    </div>
                    <div className="flex items-baseline gap-4">
                      <span className="text-xs text-muted-foreground font-mono w-14 shrink-0 text-right">h3</span>
                      <span className="font-mono text-xl font-semibold">API Keys</span>
                    </div>
                    <div className="flex items-baseline gap-4">
                      <span className="text-xs text-muted-foreground font-mono w-14 shrink-0 text-right">body</span>
                      <span className="text-base">Route requests across OpenAI, Anthropic, Google, and more.</span>
                    </div>
                    <div className="flex items-baseline gap-4">
                      <span className="text-xs text-muted-foreground font-mono w-14 shrink-0 text-right">sm</span>
                      <span className="text-sm text-muted-foreground">Last updated 2 minutes ago</span>
                    </div>
                    <div className="flex items-baseline gap-4">
                      <span className="text-xs text-muted-foreground font-mono w-14 shrink-0 text-right">code</span>
                      <span className="text-sm font-mono text-primary">fr_live_a1b2c3d4e5f6g7h8</span>
                    </div>
                    <div className="flex items-baseline gap-4">
                      <span className="text-xs text-muted-foreground font-mono w-14 shrink-0 text-right">$$$</span>
                      <span className="text-2xl font-mono font-semibold tabular-nums">$142,847.83</span>
                    </div>
                  </div>
                </Section>

                <Separator />

                {/* ── Buttons ── */}
                <Section id="buttons" title="buttons">
                  <div className="space-y-6">
                    <div>
                      <p className="text-xs text-muted-foreground font-mono mb-3">/* variants */</p>
                      <div className="flex flex-wrap gap-3">
                        <Button>Primary</Button>
                        <Button variant="secondary">Secondary</Button>
                        <Button variant="outline">Outline</Button>
                        <Button variant="ghost">Ghost</Button>
                        <Button variant="link">Link</Button>
                        <Button variant="destructive">Destructive</Button>
                      </div>
                    </div>
                    <div>
                      <p className="text-xs text-muted-foreground font-mono mb-3">/* with icons */</p>
                      <div className="flex flex-wrap gap-3">
                        <Button><Plus className="h-4 w-4 mr-2" />New Provider</Button>
                        <Button variant="outline"><Copy className="h-4 w-4 mr-2" />Copy Key</Button>
                        <Button variant="destructive"><Trash2 className="h-4 w-4 mr-2" />Delete</Button>
                        <Button variant="ghost" size="icon"><Pencil className="h-4 w-4" /></Button>
                        <Button variant="ghost" size="icon"><MoreHorizontal className="h-4 w-4" /></Button>
                      </div>
                    </div>
                    <div>
                      <p className="text-xs text-muted-foreground font-mono mb-3">/* sizes */</p>
                      <div className="flex flex-wrap items-center gap-3">
                        <Button size="sm">Small</Button>
                        <Button size="default">Default</Button>
                        <Button size="lg">Large</Button>
                      </div>
                    </div>
                    <div>
                      <p className="text-xs text-muted-foreground font-mono mb-3">/* states */</p>
                      <div className="flex flex-wrap gap-3">
                        <Button>Enabled</Button>
                        <Button disabled>Disabled</Button>
                      </div>
                    </div>
                  </div>
                </Section>

                <Separator />

                {/* ── Badges ── */}
                <Section id="badges" title="badges">
                  <div className="space-y-4">
                    <div>
                      <p className="text-xs text-muted-foreground font-mono mb-3">/* variants */</p>
                      <div className="flex flex-wrap gap-2">
                        <Badge>Default</Badge>
                        <Badge variant="secondary">Secondary</Badge>
                        <Badge variant="outline">Outline</Badge>
                        <Badge variant="destructive">Destructive</Badge>
                      </div>
                    </div>
                    <div>
                      <p className="text-xs text-muted-foreground font-mono mb-3">/* status indicators */</p>
                      <div className="flex flex-wrap gap-2">
                        <Badge variant="outline" className="gap-1.5 font-mono text-xs"><span className="h-1.5 w-1.5 rounded-full bg-success" />ONLINE</Badge>
                        <Badge variant="outline" className="gap-1.5 font-mono text-xs"><span className="h-1.5 w-1.5 rounded-full bg-warning" />DEGRADED</Badge>
                        <Badge variant="outline" className="gap-1.5 font-mono text-xs"><span className="h-1.5 w-1.5 rounded-full bg-destructive" />DOWN</Badge>
                        <Badge variant="outline" className="gap-1.5 font-mono text-xs"><span className="h-1.5 w-1.5 rounded-full bg-muted-foreground" />OFFLINE</Badge>
                      </div>
                    </div>
                    <div>
                      <p className="text-xs text-muted-foreground font-mono mb-3">/* labels */</p>
                      <div className="flex flex-wrap gap-2">
                        <Badge variant="secondary" className="font-mono text-xs">managed</Badge>
                        <Badge variant="secondary" className="font-mono text-xs">BYOK</Badge>
                        <Badge variant="secondary" className="font-mono text-xs">streaming</Badge>
                        <Badge variant="secondary" className="font-mono text-xs">vision</Badge>
                        <Badge variant="secondary" className="font-mono text-xs">tools</Badge>
                        <Badge variant="secondary" className="font-mono text-xs">reasoning</Badge>
                      </div>
                    </div>
                  </div>
                </Section>

                <Separator />

                {/* ── Metric Cards ── */}
                <Section id="metrics" title="metric_cards">
                  <div className="grid grid-cols-1 sm:grid-cols-2 lg:grid-cols-4 gap-4">
                    <Card className="glow-box">
                      <CardHeader className="flex flex-row items-center justify-between pb-2">
                        <CardDescription className="font-mono text-xs uppercase tracking-wider">Total Spend</CardDescription>
                        <DollarSign className="h-4 w-4 text-muted-foreground" />
                      </CardHeader>
                      <CardContent>
                        <p className="text-2xl font-mono font-semibold tabular-nums">$1,247.83</p>
                        <p className="text-xs text-warning mt-1 font-mono">+12.5% from last month</p>
                      </CardContent>
                    </Card>
                    <Card className="glow-box">
                      <CardHeader className="flex flex-row items-center justify-between pb-2">
                        <CardDescription className="font-mono text-xs uppercase tracking-wider">Requests / 24h</CardDescription>
                        <Activity className="h-4 w-4 text-muted-foreground" />
                      </CardHeader>
                      <CardContent>
                        <p className="text-2xl font-mono font-semibold tabular-nums">12,847</p>
                        <p className="text-xs text-success mt-1 font-mono">99.2% success</p>
                      </CardContent>
                    </Card>
                    <Card className="glow-box">
                      <CardHeader className="flex flex-row items-center justify-between pb-2">
                        <CardDescription className="font-mono text-xs uppercase tracking-wider">Avg Latency</CardDescription>
                        <Clock className="h-4 w-4 text-muted-foreground" />
                      </CardHeader>
                      <CardContent>
                        <p className="text-2xl font-mono font-semibold tabular-nums">342ms</p>
                        <p className="text-xs text-muted-foreground mt-1 font-mono">p95: 890ms</p>
                      </CardContent>
                    </Card>
                    <Card className="glow-box">
                      <CardHeader className="flex flex-row items-center justify-between pb-2">
                        <CardDescription className="font-mono text-xs uppercase tracking-wider">Active Keys</CardDescription>
                        <KeyRound className="h-4 w-4 text-muted-foreground" />
                      </CardHeader>
                      <CardContent>
                        <p className="text-2xl font-mono font-semibold tabular-nums">4</p>
                        <div className="flex items-center gap-1 mt-1">
                          <AlertTriangle className="h-3 w-3 text-warning" />
                          <p className="text-xs text-warning font-mono">1 expiring soon</p>
                        </div>
                      </CardContent>
                    </Card>
                  </div>
                </Section>

                <Separator />

                {/* ── Charts ── */}
                <Section id="charts" title="charts">
                  <div className="grid grid-cols-1 lg:grid-cols-2 gap-4">
                    {/* Area chart */}
                    <Card>
                      <CardHeader>
                        <CardTitle className="font-mono text-sm">Requests Over Time</CardTitle>
                        <CardDescription>Last 7 days</CardDescription>
                      </CardHeader>
                      <CardContent>
                        <ResponsiveContainer width="100%" height={220}>
                          <AreaChart data={usageData}>
                            <defs>
                              <linearGradient id="fillReq" x1="0" y1="0" x2="0" y2="1">
                                <stop offset="0%" stopColor="#7b9bae" stopOpacity={0.3} />
                                <stop offset="100%" stopColor="#7b9bae" stopOpacity={0} />
                              </linearGradient>
                            </defs>
                            <CartesianGrid strokeDasharray="3 3" stroke="#222220" />
                            <XAxis dataKey="date" stroke="#6b675e" fontSize={12} fontFamily="JetBrains Mono Variable" />
                            <YAxis stroke="#6b675e" fontSize={12} fontFamily="JetBrains Mono Variable" />
                            <RechartsTooltip
                              contentStyle={{ backgroundColor: "#131311", border: "1px solid #222220", borderRadius: 4, fontFamily: "JetBrains Mono Variable", fontSize: 12 }}
                              labelStyle={{ color: "#d4d0c8" }}
                            />
                            <Area type="monotone" dataKey="requests" stroke="#7b9bae" fill="url(#fillReq)" strokeWidth={2} />
                          </AreaChart>
                        </ResponsiveContainer>
                      </CardContent>
                    </Card>

                    {/* Bar chart */}
                    <Card>
                      <CardHeader>
                        <CardTitle className="font-mono text-sm">Cost by Model</CardTitle>
                        <CardDescription>Current billing period</CardDescription>
                      </CardHeader>
                      <CardContent>
                        <ResponsiveContainer width="100%" height={220}>
                          <BarChart data={modelUsageData} layout="vertical">
                            <CartesianGrid strokeDasharray="3 3" stroke="#222220" horizontal={false} />
                            <XAxis type="number" stroke="#6b675e" fontSize={12} fontFamily="JetBrains Mono Variable" tickFormatter={(v) => `$${v}`} />
                            <YAxis type="category" dataKey="model" stroke="#6b675e" fontSize={11} fontFamily="JetBrains Mono Variable" width={90} />
                            <RechartsTooltip
                              contentStyle={{ backgroundColor: "#131311", border: "1px solid #222220", borderRadius: 4, fontFamily: "JetBrains Mono Variable", fontSize: 12 }}
                              formatter={(value) => [`$${Number(value).toFixed(2)}`, "Cost"]}
                            />
                            <Bar dataKey="cost" fill="#7b9bae" radius={[0, 2, 2, 0]} />
                          </BarChart>
                        </ResponsiveContainer>
                      </CardContent>
                    </Card>

                    {/* Pie chart */}
                    <Card>
                      <CardHeader>
                        <CardTitle className="font-mono text-sm">Traffic by Provider</CardTitle>
                        <CardDescription>Share of total requests</CardDescription>
                      </CardHeader>
                      <CardContent className="flex items-center gap-6">
                        <ResponsiveContainer width={160} height={160}>
                          <PieChart>
                            <Pie data={pieData} dataKey="value" cx="50%" cy="50%" innerRadius={45} outerRadius={70} strokeWidth={0}>
                              {pieData.map((_, i) => (
                                <Cell key={i} fill={CHART_COLORS[i % CHART_COLORS.length]} />
                              ))}
                            </Pie>
                          </PieChart>
                        </ResponsiveContainer>
                        <div className="space-y-2">
                          {pieData.map((d, i) => (
                            <div key={d.name} className="flex items-center gap-2 text-sm">
                              <span className="h-2.5 w-2.5 rounded-sm shrink-0" style={{ backgroundColor: CHART_COLORS[i] }} />
                              <span className="text-muted-foreground">{d.name}</span>
                              <span className="font-mono text-xs ml-auto tabular-nums">{d.value}%</span>
                            </div>
                          ))}
                        </div>
                      </CardContent>
                    </Card>

                    {/* Progress bars */}
                    <Card>
                      <CardHeader>
                        <CardTitle className="font-mono text-sm">Spending Limits</CardTitle>
                        <CardDescription>Daily and monthly caps</CardDescription>
                      </CardHeader>
                      <CardContent className="space-y-5">
                        <div>
                          <div className="flex justify-between text-sm mb-2">
                            <span className="font-mono text-xs text-muted-foreground">daily</span>
                            <span className="font-mono text-xs tabular-nums">$24.80 / $50.00</span>
                          </div>
                          <Progress value={49.6} />
                        </div>
                        <div>
                          <div className="flex justify-between text-sm mb-2">
                            <span className="font-mono text-xs text-muted-foreground">monthly</span>
                            <span className="font-mono text-xs tabular-nums">$423.50 / $500.00</span>
                          </div>
                          <Progress value={84.7} />
                        </div>
                        <div>
                          <div className="flex justify-between text-sm mb-2">
                            <span className="font-mono text-xs text-muted-foreground">rate_limit</span>
                            <span className="font-mono text-xs tabular-nums">42 / 60 RPM</span>
                          </div>
                          <Progress value={70} />
                        </div>
                      </CardContent>
                    </Card>
                  </div>
                </Section>

                <Separator />

                {/* ── Data Table ── */}
                <Section id="table" title="data_table">
                  <Card>
                    <CardHeader>
                      <div className="flex items-center justify-between">
                        <div>
                          <CardTitle className="font-mono text-sm">Model Mappings</CardTitle>
                          <CardDescription>Provider routes and pricing</CardDescription>
                        </div>
                        <Button size="sm"><Plus className="h-4 w-4 mr-2" />Add Mapping</Button>
                      </div>
                    </CardHeader>
                    <CardContent className="p-0">
                      <Table>
                        <TableHeader>
                          <TableRow>
                            <TableHead className="font-mono text-xs">Provider</TableHead>
                            <TableHead className="font-mono text-xs">Model</TableHead>
                            <TableHead className="text-right font-mono text-xs">In $/1M</TableHead>
                            <TableHead className="text-right font-mono text-xs">Out $/1M</TableHead>
                            <TableHead className="text-right font-mono text-xs">Latency</TableHead>
                            <TableHead className="font-mono text-xs">Capabilities</TableHead>
                            <TableHead className="font-mono text-xs">Status</TableHead>
                            <TableHead className="w-10" />
                          </TableRow>
                        </TableHeader>
                        <TableBody>
                          <TableRow>
                            <TableCell className="font-mono text-sm">OpenAI</TableCell>
                            <TableCell className="text-sm">GPT-4o</TableCell>
                            <TableCell className="text-right font-mono text-sm tabular-nums">$2.50</TableCell>
                            <TableCell className="text-right font-mono text-sm tabular-nums">$10.00</TableCell>
                            <TableCell className="text-right font-mono text-sm tabular-nums">280ms</TableCell>
                            <TableCell>
                              <div className="flex gap-1">
                                <Badge variant="secondary" className="font-mono text-[10px] px-1.5">vision</Badge>
                                <Badge variant="secondary" className="font-mono text-[10px] px-1.5">tools</Badge>
                              </div>
                            </TableCell>
                            <TableCell><Badge variant="outline" className="gap-1.5 font-mono text-xs"><span className="h-1.5 w-1.5 rounded-full bg-success" />ON</Badge></TableCell>
                            <TableCell><Button variant="ghost" size="icon" className="h-8 w-8"><MoreHorizontal className="h-4 w-4" /></Button></TableCell>
                          </TableRow>
                          <TableRow>
                            <TableCell className="font-mono text-sm">Anthropic</TableCell>
                            <TableCell className="text-sm">Claude Sonnet 4</TableCell>
                            <TableCell className="text-right font-mono text-sm tabular-nums">$3.00</TableCell>
                            <TableCell className="text-right font-mono text-sm tabular-nums">$15.00</TableCell>
                            <TableCell className="text-right font-mono text-sm tabular-nums">420ms</TableCell>
                            <TableCell>
                              <div className="flex gap-1">
                                <Badge variant="secondary" className="font-mono text-[10px] px-1.5">vision</Badge>
                                <Badge variant="secondary" className="font-mono text-[10px] px-1.5">tools</Badge>
                              </div>
                            </TableCell>
                            <TableCell><Badge variant="outline" className="gap-1.5 font-mono text-xs"><span className="h-1.5 w-1.5 rounded-full bg-success" />ON</Badge></TableCell>
                            <TableCell><Button variant="ghost" size="icon" className="h-8 w-8"><MoreHorizontal className="h-4 w-4" /></Button></TableCell>
                          </TableRow>
                          <TableRow>
                            <TableCell className="font-mono text-sm">Google</TableCell>
                            <TableCell className="text-sm">Gemini 2.5 Pro</TableCell>
                            <TableCell className="text-right font-mono text-sm tabular-nums">$1.25</TableCell>
                            <TableCell className="text-right font-mono text-sm tabular-nums">$10.00</TableCell>
                            <TableCell className="text-right font-mono text-sm tabular-nums">350ms</TableCell>
                            <TableCell>
                              <div className="flex gap-1">
                                <Badge variant="secondary" className="font-mono text-[10px] px-1.5">vision</Badge>
                                <Badge variant="secondary" className="font-mono text-[10px] px-1.5">reasoning</Badge>
                              </div>
                            </TableCell>
                            <TableCell><Badge variant="outline" className="gap-1.5 font-mono text-xs"><span className="h-1.5 w-1.5 rounded-full bg-warning" />DEG</Badge></TableCell>
                            <TableCell><Button variant="ghost" size="icon" className="h-8 w-8"><MoreHorizontal className="h-4 w-4" /></Button></TableCell>
                          </TableRow>
                          <TableRow>
                            <TableCell className="font-mono text-sm">DeepSeek</TableCell>
                            <TableCell className="text-sm">DeepSeek Chat</TableCell>
                            <TableCell className="text-right font-mono text-sm tabular-nums">$0.14</TableCell>
                            <TableCell className="text-right font-mono text-sm tabular-nums">$0.28</TableCell>
                            <TableCell className="text-right font-mono text-sm tabular-nums">---</TableCell>
                            <TableCell>
                              <div className="flex gap-1">
                                <Badge variant="secondary" className="font-mono text-[10px] px-1.5">tools</Badge>
                              </div>
                            </TableCell>
                            <TableCell><Badge variant="outline" className="gap-1.5 font-mono text-xs"><span className="h-1.5 w-1.5 rounded-full bg-muted-foreground" />OFF</Badge></TableCell>
                            <TableCell><Button variant="ghost" size="icon" className="h-8 w-8"><MoreHorizontal className="h-4 w-4" /></Button></TableCell>
                          </TableRow>
                        </TableBody>
                      </Table>
                    </CardContent>
                  </Card>
                </Section>

                <Separator />

                {/* ── API Key display pattern ── */}
                <Section id="api-key" title="api_key_pattern">
                  <Card className="max-w-lg">
                    <CardHeader>
                      <CardTitle className="font-mono text-sm">Production Key</CardTitle>
                      <CardDescription>Main API key for production workloads</CardDescription>
                    </CardHeader>
                    <CardContent className="space-y-4">
                      <div className="flex items-center gap-2">
                        <div className="flex-1 bg-muted rounded-sm px-3 py-2 font-mono text-sm">
                          {showKey ? "fr_live_a1b2c3d4e5f6g7h8i9j0k1l2m3n4" : "fr_live_a1b2••••••••••••••••••••••••"}
                        </div>
                        <Tooltip>
                          <TooltipTrigger>
                            <Button variant="ghost" size="icon" className="h-8 w-8" onClick={() => setShowKey(!showKey)}>
                              {showKey ? <EyeOff className="h-4 w-4" /> : <Eye className="h-4 w-4" />}
                            </Button>
                          </TooltipTrigger>
                          <TooltipContent>{showKey ? "Hide" : "Reveal"}</TooltipContent>
                        </Tooltip>
                        <Tooltip>
                          <TooltipTrigger>
                            <Button variant="ghost" size="icon" className="h-8 w-8">
                              <Copy className="h-4 w-4" />
                            </Button>
                          </TooltipTrigger>
                          <TooltipContent>Copy</TooltipContent>
                        </Tooltip>
                      </div>
                      <div className="flex flex-wrap gap-1.5">
                        <Badge variant="secondary" className="font-mono text-xs">gateway:chat</Badge>
                        <Badge variant="secondary" className="font-mono text-xs">usage:read</Badge>
                        <Badge variant="secondary" className="font-mono text-xs">billing:read</Badge>
                      </div>
                      <div className="flex items-center justify-between text-xs text-muted-foreground font-mono">
                        <span>Last used: 2 minutes ago</span>
                        <Badge variant="outline" className="gap-1.5 font-mono text-xs"><span className="h-1.5 w-1.5 rounded-full bg-success" />ACTIVE</Badge>
                      </div>
                    </CardContent>
                  </Card>
                </Section>

                <Separator />

                {/* ── Entity Card ── */}
                <Section id="entity" title="entity_card">
                  <div className="grid grid-cols-1 sm:grid-cols-2 lg:grid-cols-3 gap-4">
                    {[
                      { name: "OpenAI", desc: "GPT models and DALL-E", models: 4, keys: 2, status: "ONLINE", statusColor: "bg-success" },
                      { name: "Anthropic", desc: "Claude models", models: 2, keys: 1, status: "ONLINE", statusColor: "bg-success" },
                      { name: "Google", desc: "Gemini models", models: 2, keys: 1, status: "DEGRADED", statusColor: "bg-warning" },
                    ].map((p) => (
                      <Card key={p.name} className="hover:border-primary/30 transition-colors cursor-pointer group">
                        <CardHeader className="pb-3">
                          <div className="flex items-center justify-between">
                            <CardTitle className="font-mono text-sm">{p.name}</CardTitle>
                            <ChevronRight className="h-4 w-4 text-muted-foreground group-hover:text-foreground transition-colors" />
                          </div>
                          <CardDescription>{p.desc}</CardDescription>
                        </CardHeader>
                        <CardContent className="space-y-2">
                          <div className="flex items-center justify-between text-xs">
                            <span className="text-muted-foreground font-mono">models</span>
                            <span className="font-mono tabular-nums">{p.models}</span>
                          </div>
                          <div className="flex items-center justify-between text-xs">
                            <span className="text-muted-foreground font-mono">keys</span>
                            <span className="font-mono tabular-nums">{p.keys}</span>
                          </div>
                          <div className="flex items-center justify-between text-xs">
                            <span className="text-muted-foreground font-mono">status</span>
                            <Badge variant="outline" className="gap-1.5 font-mono text-[10px]"><span className={`h-1.5 w-1.5 rounded-full ${p.statusColor}`} />{p.status}</Badge>
                          </div>
                        </CardContent>
                      </Card>
                    ))}
                  </div>
                </Section>

                <Separator />

                {/* ── Form inputs ── */}
                <Section id="forms" title="form_inputs">
                  <div className="grid grid-cols-1 sm:grid-cols-2 gap-6 max-w-2xl">
                    <div className="space-y-2">
                      <Label htmlFor="name" className="font-mono text-xs">provider_name</Label>
                      <Input id="name" placeholder="e.g. OpenAI" />
                    </div>
                    <div className="space-y-2">
                      <Label htmlFor="key" className="font-mono text-xs">api_key</Label>
                      <Input id="key" type="password" placeholder="sk-..." />
                    </div>
                    <div className="space-y-2">
                      <Label htmlFor="select" className="font-mono text-xs">routing_strategy</Label>
                      <Select>
                        <SelectTrigger>
                          <SelectValue placeholder="Select strategy" />
                        </SelectTrigger>
                        <SelectContent>
                          <SelectItem value="cheapest">Cheapest</SelectItem>
                          <SelectItem value="lowest-latency">Lowest Latency</SelectItem>
                          <SelectItem value="round-robin">Round Robin</SelectItem>
                        </SelectContent>
                      </Select>
                    </div>
                    <div className="space-y-2">
                      <Label htmlFor="url" className="font-mono text-xs">base_url</Label>
                      <Input id="url" placeholder="https://api.openai.com/v1" />
                    </div>
                  </div>
                </Section>

                <Separator />

                {/* ── Switches ── */}
                <Section id="switches" title="toggles">
                  <Card className="max-w-md">
                    <CardContent className="pt-6 space-y-4">
                      {[
                        { label: "Guardrails", desc: "Enable content safety checks", checked: true },
                        { label: "PII Detection", desc: "Auto-redact personal info", checked: true },
                        { label: "Prompt Injection", desc: "Block injection attempts", checked: false },
                        { label: "Request Logging", desc: "Store message content", checked: true },
                      ].map((item) => (
                        <div key={item.label} className="flex items-center justify-between">
                          <div>
                            <p className="text-sm font-medium">{item.label}</p>
                            <p className="text-xs text-muted-foreground">{item.desc}</p>
                          </div>
                          <Switch defaultChecked={item.checked} />
                        </div>
                      ))}
                    </CardContent>
                  </Card>
                </Section>

                <Separator />

                {/* ── Dialog ── */}
                <Section id="dialog" title="dialog">
                  <div className="flex gap-3">
                    <Dialog>
                      <DialogTrigger>
                        <Button><Plus className="h-4 w-4 mr-2" />Create API Key</Button>
                      </DialogTrigger>
                      <DialogContent>
                        <DialogHeader>
                          <DialogTitle className="font-mono">New API Key</DialogTitle>
                          <DialogDescription>Create a new key for gateway access</DialogDescription>
                        </DialogHeader>
                        <div className="space-y-4 py-4">
                          <div className="space-y-2">
                            <Label className="font-mono text-xs">name</Label>
                            <Input placeholder="e.g. Production Key" />
                          </div>
                          <div className="space-y-2">
                            <Label className="font-mono text-xs">scopes</Label>
                            <div className="flex flex-wrap gap-1.5">
                              {["gateway:chat", "usage:read", "billing:read"].map((s) => (
                                <Badge key={s} variant="secondary" className="font-mono text-xs cursor-pointer hover:bg-primary hover:text-primary-foreground transition-colors">{s}</Badge>
                              ))}
                            </div>
                          </div>
                        </div>
                        <DialogFooter>
                          <Button variant="outline">Cancel</Button>
                          <Button>Create</Button>
                        </DialogFooter>
                      </DialogContent>
                    </Dialog>

                    <Dialog>
                      <DialogTrigger>
                        <Button variant="destructive"><Trash2 className="h-4 w-4 mr-2" />Delete</Button>
                      </DialogTrigger>
                      <DialogContent>
                        <DialogHeader>
                          <DialogTitle className="font-mono">Confirm Deletion</DialogTitle>
                          <DialogDescription>This action cannot be undone. The API key will be permanently revoked.</DialogDescription>
                        </DialogHeader>
                        <DialogFooter>
                          <Button variant="outline">Cancel</Button>
                          <Button variant="destructive">Delete Key</Button>
                        </DialogFooter>
                      </DialogContent>
                    </Dialog>
                  </div>
                </Section>

                <Separator />

                {/* ── Tabs ── */}
                <Section id="tabs" title="tabs">
                  <Tabs defaultValue="routing" className="max-w-lg">
                    <TabsList>
                      <TabsTrigger value="routing" className="font-mono text-xs">routing</TabsTrigger>
                      <TabsTrigger value="rate-limits" className="font-mono text-xs">rate_limits</TabsTrigger>
                      <TabsTrigger value="retention" className="font-mono text-xs">retention</TabsTrigger>
                    </TabsList>
                    <TabsContent value="routing" className="mt-4 text-sm text-muted-foreground">
                      Configure routing strategy: cheapest, lowest-latency, or round-robin.
                    </TabsContent>
                    <TabsContent value="rate-limits" className="mt-4 text-sm text-muted-foreground">
                      Set RPM and max concurrent request limits per tenant.
                    </TabsContent>
                    <TabsContent value="retention" className="mt-4 text-sm text-muted-foreground">
                      Control how long usage logs and debug payloads are retained.
                    </TabsContent>
                  </Tabs>
                </Section>

                <Separator />

                {/* ── Skeletons ── */}
                <Section id="loading" title="loading_states">
                  <div className="grid grid-cols-1 sm:grid-cols-2 lg:grid-cols-4 gap-4">
                    {Array.from({ length: 4 }).map((_, i) => (
                      <Card key={i}>
                        <CardHeader className="pb-2">
                          <Skeleton className="h-3 w-24" />
                        </CardHeader>
                        <CardContent className="space-y-2">
                          <Skeleton className="h-7 w-28" />
                          <Skeleton className="h-3 w-32" />
                        </CardContent>
                      </Card>
                    ))}
                  </div>
                  <div className="mt-4 space-y-3">
                    {Array.from({ length: 3 }).map((_, i) => (
                      <div key={i} className="flex items-center gap-4">
                        <Skeleton className="h-10 w-10 rounded-sm" />
                        <div className="space-y-2 flex-1">
                          <Skeleton className="h-4 w-32" />
                          <Skeleton className="h-3 w-48" />
                        </div>
                        <Skeleton className="h-6 w-16 rounded-sm" />
                      </div>
                    ))}
                  </div>
                </Section>

                <Separator />

                {/* ── Status icons ── */}
                <Section id="icons" title="status_indicators">
                  <div className="flex flex-wrap gap-6">
                    <div className="flex items-center gap-2 text-sm">
                      <div className="h-8 w-8 rounded-sm bg-success/10 flex items-center justify-center"><Check className="h-4 w-4 text-success" /></div>
                      <span>Success</span>
                    </div>
                    <div className="flex items-center gap-2 text-sm">
                      <div className="h-8 w-8 rounded-sm bg-destructive/10 flex items-center justify-center"><X className="h-4 w-4 text-destructive" /></div>
                      <span>Error</span>
                    </div>
                    <div className="flex items-center gap-2 text-sm">
                      <div className="h-8 w-8 rounded-sm bg-warning/10 flex items-center justify-center"><AlertTriangle className="h-4 w-4 text-warning" /></div>
                      <span>Warning</span>
                    </div>
                    <div className="flex items-center gap-2 text-sm">
                      <div className="h-8 w-8 rounded-sm bg-primary/10 flex items-center justify-center"><Zap className="h-4 w-4 text-primary" /></div>
                      <span>Info</span>
                    </div>
                    <div className="flex items-center gap-2 text-sm">
                      <div className="h-8 w-8 rounded-sm bg-muted flex items-center justify-center"><Activity className="h-4 w-4 text-muted-foreground" /></div>
                      <span>Neutral</span>
                    </div>
                  </div>
                </Section>

                <div className="h-16" />
              </main>
            </SidebarInset>
          </div>
        </SidebarProvider>
      </TooltipProvider>
    </div>
  )
}
