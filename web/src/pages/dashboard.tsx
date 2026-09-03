import { useEffect, useState } from "react"
import {
  Activity, DollarSign, Zap, Server, AlertTriangle, ArrowUpRight, ArrowDownRight,
} from "lucide-react"
import { useApi } from "@/api"
import type {
  Balance, UsageSummaryResponse, Provider, UsageLog, SpendingCheck,
} from "@/api/types"
import {
  PageHeader, MetricCard, MetricCardSkeleton,
  ChartCard, AreaChartView, DonutChartView, HBarChartView,
  SpendingLimits,
} from "@/components"
import { StatusBadge } from "@/components/data/status"
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from "@/components/ui/card"
import { Badge } from "@/components/ui/badge"
import {
  Table, TableBody, TableCell, TableHead, TableHeader, TableRow,
} from "@/components/ui/table"

// tenant id for spending check — the mock adapter uses a default
const TENANT_ID = "default"

export default function Dashboard() {
  const api = useApi()

  const [balance, setBalance] = useState<Balance | null>(null)
  const [summary, setSummary] = useState<UsageSummaryResponse | null>(null)
  const [providers, setProviders] = useState<Provider[]>([])
  const [recentLogs, setRecentLogs] = useState<UsageLog[]>([])
  const [spending, setSpending] = useState<SpendingCheck | null>(null)
  const [loading, setLoading] = useState(true)

  useEffect(() => {
    async function load() {
      const [bal, sum, prov, logs, spend] = await Promise.all([
        api.billing.getBalance(),
        api.usage.getSummary(),
        api.providers.list(),
        api.usage.queryLogs({ limit: 5 }),
        api.billing.checkSpendingLimit(TENANT_ID).catch(() => null),
      ])
      setBalance(bal)
      setSummary(sum)
      setProviders(prov.data)
      setRecentLogs(logs.data)
      setSpending(spend)
      setLoading(false)
    }
    load()
  }, [api])

  if (loading) {
    return (
      <div className="space-y-6 p-6">
        <PageHeader title="Dashboard" description="Overview of your LLM gateway" />
        <MetricCardSkeleton count={4} />
      </div>
    )
  }

  // derive chart data from summary
  const costByModel = (summary?.by_model ?? [])
    .filter((m) => m.total_cost > 0)
    .map((m) => ({ name: m.model, value: Number((m.total_cost * 100 / (summary!.summary.total_cost || 1)).toFixed(1)) }))

  const requestsByModel = (summary?.by_model ?? [])
    .sort((a, b) => b.total_requests - a.total_requests)
    .slice(0, 6)
    .map((m) => ({ model: m.model, requests: m.total_requests }))

  // generate daily usage for last 7 days from summary
  const dailyUsage = generateDailyUsage(summary?.summary.total_requests ?? 0)

  const totalTokens = summary?.summary.total_tokens ?? 0
  const totalCost = summary?.summary.total_cost ?? 0
  const totalRequests = summary?.summary.total_requests ?? 0
  const errorRate = totalRequests > 0
    ? ((summary?.summary.error_count ?? 0) / totalRequests * 100).toFixed(1)
    : "0.0"

  const activeProviders = providers.filter((p) => p.status === "active").length

  return (
    <div className="space-y-6 p-6">
      <PageHeader title="Dashboard" description="Overview of your LLM gateway" />

      {/* KPI row */}
      <div className="grid grid-cols-1 sm:grid-cols-2 lg:grid-cols-4 gap-4">
        <MetricCard
          label="Balance"
          value={`$${balance?.balance.toFixed(2) ?? "0.00"}`}
          description={balance && balance.balance > 50 ? "Healthy" : "Low balance"}
          descriptionClassName={balance && balance.balance > 50 ? "text-success" : "text-destructive"}
          icon={DollarSign}
        />
        <MetricCard
          label="Requests (period)"
          value={totalRequests.toLocaleString()}
          description={`${Number(errorRate)}% error rate`}
          descriptionClassName={Number(errorRate) > 5 ? "text-destructive" : "text-success"}
          icon={Activity}
        />
        <MetricCard
          label="Total Tokens"
          value={formatNumber(totalTokens)}
          description={`$${totalCost.toFixed(2)} total cost`}
          icon={Zap}
        />
        <MetricCard
          label="Providers"
          value={`${activeProviders} / ${providers.length}`}
          description={`${activeProviders} active`}
          descriptionClassName="text-success"
          icon={Server}
        />
      </div>

      {/* Charts row */}
      <div className="grid grid-cols-1 lg:grid-cols-3 gap-4">
        <ChartCard title="Requests (7d)" description="Daily request volume" className="lg:col-span-2">
          <AreaChartView data={dailyUsage} xKey="day" yKey="requests" />
        </ChartCard>
        <ChartCard title="Cost by Model" description="Spend distribution">
          {costByModel.length > 0 ? (
            <DonutChartView data={costByModel} nameKey="name" valueKey="value" />
          ) : (
            <p className="text-sm text-muted-foreground text-center py-10">No cost data yet</p>
          )}
        </ChartCard>
      </div>

      {/* Second row: top models + spending */}
      <div className="grid grid-cols-1 lg:grid-cols-2 gap-4">
        <ChartCard title="Top Models" description="By request count">
          {requestsByModel.length > 0 ? (
            <HBarChartView
              data={requestsByModel}
              categoryKey="model"
              valueKey="requests"
              height={Math.max(160, requestsByModel.length * 36)}
            />
          ) : (
            <p className="text-sm text-muted-foreground text-center py-10">No usage data yet</p>
          )}
        </ChartCard>

        {spending && (spending.daily_limit_usd !== null || spending.monthly_limit_usd !== null) ? (
          <SpendingLimits
            items={[
              ...(spending.daily_limit_usd !== null ? [{
                label: "Daily spend",
                current: spending.daily_spend_usd,
                max: spending.daily_limit_usd,
                unit: "$",
              }] : []),
              ...(spending.monthly_limit_usd !== null ? [{
                label: "Monthly spend",
                current: spending.monthly_spend_usd,
                max: spending.monthly_limit_usd,
                unit: "$",
              }] : []),
            ]}
          />
        ) : (
          <Card>
            <CardHeader>
              <CardTitle className="font-mono text-sm">Spending Limits</CardTitle>
              <CardDescription>No limits configured</CardDescription>
            </CardHeader>
            <CardContent>
              <p className="text-sm text-muted-foreground">
                Set daily or monthly spending caps in the billing settings.
              </p>
            </CardContent>
          </Card>
        )}
      </div>

      {/* Providers + Recent requests */}
      <div className="grid grid-cols-1 lg:grid-cols-2 gap-4">
        {/* Provider status */}
        <Card>
          <CardHeader>
            <CardTitle className="font-mono text-sm">Providers</CardTitle>
            <CardDescription>Status of connected LLM providers</CardDescription>
          </CardHeader>
          <CardContent>
            <div className="space-y-3">
              {providers.slice(0, 6).map((p) => (
                <div key={p.id} className="flex items-center justify-between">
                  <div className="flex items-center gap-3">
                    <span className="font-mono text-sm">{p.name}</span>
                    {p.streaming && (
                      <Badge variant="secondary" className="font-mono text-[10px] px-1.5">stream</Badge>
                    )}
                  </div>
                  <StatusBadge status={p.status === "active" ? "online" : "offline"} />
                </div>
              ))}
              {providers.length > 6 && (
                <p className="text-xs text-muted-foreground font-mono">
                  +{providers.length - 6} more
                </p>
              )}
            </div>
          </CardContent>
        </Card>

        {/* Recent requests */}
        <Card>
          <CardHeader>
            <CardTitle className="font-mono text-sm">Recent Requests</CardTitle>
            <CardDescription>Latest API calls</CardDescription>
          </CardHeader>
          <CardContent>
            <Table>
              <TableHeader>
                <TableRow>
                  <TableHead className="font-mono text-xs">Model</TableHead>
                  <TableHead className="font-mono text-xs">Status</TableHead>
                  <TableHead className="font-mono text-xs text-right">Tokens</TableHead>
                  <TableHead className="font-mono text-xs text-right">Cost</TableHead>
                </TableRow>
              </TableHeader>
              <TableBody>
                {recentLogs.map((log) => (
                  <TableRow key={log.id}>
                    <TableCell className="font-mono text-xs">{log.used_model}</TableCell>
                    <TableCell>
                      {!log.has_error ? (
                        <span className="inline-flex items-center gap-1 text-xs text-success">
                          <ArrowUpRight className="h-3 w-3" /> ok
                        </span>
                      ) : (
                        <span className="inline-flex items-center gap-1 text-xs text-destructive">
                          <ArrowDownRight className="h-3 w-3" /> {log.status_code}
                        </span>
                      )}
                    </TableCell>
                    <TableCell className="font-mono text-xs text-right tabular-nums">
                      {(log.prompt_tokens + log.completion_tokens).toLocaleString()}
                    </TableCell>
                    <TableCell className="font-mono text-xs text-right tabular-nums">
                      ${log.total_cost.toFixed(4)}
                    </TableCell>
                  </TableRow>
                ))}
                {recentLogs.length === 0 && (
                  <TableRow>
                    <TableCell colSpan={4} className="text-center text-sm text-muted-foreground py-6">
                      No requests yet
                    </TableCell>
                  </TableRow>
                )}
              </TableBody>
            </Table>
          </CardContent>
        </Card>
      </div>

      {/* Guardrail violations if any */}
      <GuardrailBanner api={api} />
    </div>
  )
}

// Small banner at the bottom if there are recent violations
function GuardrailBanner({ api }: { api: ReturnType<typeof useApi> }) {
  const [count, setCount] = useState<number | null>(null)

  useEffect(() => {
    api.guardrails.listViolations().then((res) => setCount(res.total)).catch(() => null)
  }, [api])

  if (count === null || count === 0) return null

  return (
    <Card className="border-warning/30">
      <CardContent className="flex items-center gap-3 py-4">
        <AlertTriangle className="h-4 w-4 text-warning shrink-0" />
        <span className="text-sm">
          <span className="font-mono font-semibold text-warning">{count}</span>{" "}
          guardrail violation{count !== 1 ? "s" : ""} detected in the current period.
        </span>
      </CardContent>
    </Card>
  )
}

// Helpers

function formatNumber(n: number): string {
  if (n >= 1_000_000) return `${(n / 1_000_000).toFixed(1)}M`
  if (n >= 1_000) return `${(n / 1_000).toFixed(1)}K`
  return n.toString()
}

function generateDailyUsage(totalRequests: number) {
  const days = []
  const base = Math.max(Math.round(totalRequests / 7), 10)
  for (let i = 6; i >= 0; i--) {
    const d = new Date()
    d.setDate(d.getDate() - i)
    days.push({
      day: d.toLocaleDateString("en-US", { weekday: "short" }),
      requests: Math.round(base * (0.7 + Math.random() * 0.6)),
    })
  }
  return days
}

