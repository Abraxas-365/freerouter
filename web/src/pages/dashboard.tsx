import { useEffect, useState } from "react"
import {
  Activity, Zap, Server, AlertTriangle, Box, ShieldAlert,
} from "lucide-react"
import { useApi } from "@/api"
import type {
  UsageSummaryResponse, Provider, UsageLog, GuardrailViolation,
} from "@/api/types"
import {
  PageHeader, MetricCard, MetricCardSkeleton,
  ChartCard, HBarChartView,
} from "@/components"
import { StatusBadge } from "@/components/data/status"
import { Card, CardContent } from "@/components/ui/card"
import { Badge } from "@/components/ui/badge"
import {
  Table, TableBody, TableCell, TableHead, TableHeader, TableRow,
} from "@/components/ui/table"

export default function Dashboard() {
  const api = useApi()

  const [summary, setSummary] = useState<UsageSummaryResponse | null>(null)
  const [providers, setProviders] = useState<Provider[]>([])
  const [modelCount, setModelCount] = useState(0)
  const [recentLogs, setRecentLogs] = useState<UsageLog[]>([])
  const [violations, setViolations] = useState<GuardrailViolation[]>([])
  const [loading, setLoading] = useState(true)

  useEffect(() => {
    async function load() {
      const [sum, prov, models, logs, viols] = await Promise.all([
        api.usage.getSummary(),
        api.providers.list({ limit: 100 }),
        api.models.list({ limit: 1 }),
        api.usage.list({ limit: 5 }),
        api.guardrails.listViolations({ limit: 100 }).catch(() => null),
      ])
      setSummary(sum)
      setProviders(prov.items)
      setModelCount(models.page.total)
      setRecentLogs(logs.items)
      setViolations(viols?.items ?? [])
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

  const requestsByModel = (summary?.by_model ?? [])
    .sort((a, b) => b.total_requests - a.total_requests)
    .slice(0, 6)
    .map((m) => ({ model: m.model, requests: m.total_requests }))

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
          label="Requests"
          value={formatNumber(totalRequests)}
          description={`${errorRate}% error rate`}
          descriptionClassName={Number(errorRate) > 5 ? "text-destructive" : undefined}
          icon={Activity}
        />
        <MetricCard
          label="Tokens"
          value={formatNumber(totalTokens)}
          icon={Zap}
        />
        <MetricCard
          label="Total Cost"
          value={`$${totalCost.toFixed(2)}`}
          icon={Box}
        />
        <MetricCard
          label="Providers"
          value={`${activeProviders}/${providers.length}`}
          description={`${modelCount} models`}
          icon={Server}
        />
      </div>

      {violations.length > 0 && (
        <Card className="border-warning/30 bg-warning/5">
          <CardContent className="flex items-center gap-3 py-4">
            <ShieldAlert className="h-5 w-5 text-warning shrink-0" />
            <span className="text-sm">
              <span className="font-mono font-semibold text-warning">{violations.length}</span>{" "}
              guardrail violation{violations.length !== 1 ? "s" : ""} detected in the current period.
            </span>
          </CardContent>
        </Card>
      )}

      <div className="grid grid-cols-1 lg:grid-cols-2 gap-4">
        <ChartCard title="Requests by Model" description="Top 6 models by request volume">
          {requestsByModel.length > 0 ? (
            <HBarChartView data={requestsByModel} categoryKey="model" valueKey="requests" />
          ) : (
            <p className="text-sm text-muted-foreground py-8 text-center">No usage data yet</p>
          )}
        </ChartCard>

        <Card>
          <CardContent className="p-0">
            <Table>
              <TableHeader>
                <TableRow>
                  <TableHead className="font-mono text-xs">Model</TableHead>
                  <TableHead className="font-mono text-xs">Tokens</TableHead>
                  <TableHead className="font-mono text-xs">Cost</TableHead>
                  <TableHead className="font-mono text-xs">Status</TableHead>
                </TableRow>
              </TableHeader>
              <TableBody>
                {recentLogs.map((log) => (
                  <TableRow key={log.id}>
                    <TableCell className="font-mono text-xs">
                      <div className="flex items-center gap-1.5">
                        {log.used_model}
                        {log.is_fallback && (
                          <Badge variant="secondary" className="font-mono text-[9px]">fallback</Badge>
                        )}
                      </div>
                    </TableCell>
                    <TableCell className="font-mono text-xs tabular-nums">{log.total_tokens}</TableCell>
                    <TableCell className="font-mono text-xs tabular-nums">${log.total_cost.toFixed(4)}</TableCell>
                    <TableCell>
                      {log.has_error ? (
                        <StatusBadge status="down" label="ERROR" />
                      ) : (
                        <StatusBadge status="online" label="OK" />
                      )}
                    </TableCell>
                  </TableRow>
                ))}
                {recentLogs.length === 0 && (
                  <TableRow>
                    <TableCell colSpan={4} className="text-center text-sm text-muted-foreground py-8">
                      <div className="flex flex-col items-center gap-2">
                        <AlertTriangle className="h-4 w-4" />
                        No requests yet
                      </div>
                    </TableCell>
                  </TableRow>
                )}
              </TableBody>
            </Table>
          </CardContent>
        </Card>
      </div>
    </div>
  )
}

function formatNumber(n: number): string {
  if (n >= 1_000_000) return `${(n / 1_000_000).toFixed(1)}M`
  if (n >= 1_000) return `${(n / 1_000).toFixed(1)}K`
  return n.toString()
}
