import { useEffect, useState } from "react"
import {
  Activity, BarChart3, Clock, AlertTriangle, Search,
  ChevronRight, Loader2, X,
} from "lucide-react"
import { useApi } from "@/api"
import type {
  UsageLog, UsageLogDetail, UsageSummaryResponse, UsageQuery,
} from "@/api/types"
import {
  PageHeader, MetricCard, ChartCard, DonutChartView,
  HBarChartView,
} from "@/components"
import { MetricCardSkeleton } from "@/components/feedback/skeletons"
import { Badge } from "@/components/ui/badge"
import { Button } from "@/components/ui/button"
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from "@/components/ui/card"
import {
  Dialog, DialogContent, DialogHeader, DialogTitle,
} from "@/components/ui/dialog"
import { Input } from "@/components/ui/input"
import {
  Select, SelectContent, SelectItem, SelectTrigger, SelectValue,
} from "@/components/ui/select"
import {
  Table, TableBody, TableCell, TableHead, TableHeader, TableRow,
} from "@/components/ui/table"

export default function UsagePage() {
  const api = useApi()

  const [summary, setSummary] = useState<UsageSummaryResponse | null>(null)
  const [logs, setLogs] = useState<UsageLog[]>([])
  const [totalLogs, setTotalLogs] = useState(0)
  const [loading, setLoading] = useState(true)

  // filters
  const [modelFilter, setModelFilter] = useState("")
  const [providerFilter, setProviderFilter] = useState("")
  const [statusFilter, setStatusFilter] = useState<string>("all")

  // detail dialog
  const [detailLog, setDetailLog] = useState<UsageLogDetail | null>(null)
  const [detailOpen, setDetailOpen] = useState(false)
  const [detailLoading, setDetailLoading] = useState(false)

  async function load() {
    const query: UsageQuery = { limit: 50 }
    if (modelFilter) query.model = modelFilter
    if (providerFilter) query.provider = providerFilter

    const [sum, res] = await Promise.all([
      api.usage.getSummary(),
      api.usage.queryLogs(query),
    ])
    setSummary(sum)

    const filtered = statusFilter === "all"
      ? res.data
      : statusFilter === "error"
        ? res.data.filter((l) => l.has_error)
        : res.data.filter((l) => !l.has_error)

    setLogs(filtered)
    setTotalLogs(res.total)
    setLoading(false)
  }

  useEffect(() => { load() }, [api, modelFilter, providerFilter, statusFilter])

  async function openDetail(id: string) {
    setDetailLoading(true)
    setDetailOpen(true)
    const detail = await api.usage.getLog(id)
    setDetailLog(detail)
    setDetailLoading(false)
  }

  // chart data
  const totalCost = summary?.by_model?.reduce((s, m) => s + m.total_cost, 0) ?? 0
  const costByModel = (summary?.by_model ?? [])
    .filter((m) => m.total_cost > 0)
    .sort((a, b) => b.total_cost - a.total_cost)
    .map((m) => ({
      model: m.model,
      cost: Number(m.total_cost.toFixed(4)),
      pct: totalCost > 0 ? Number(((m.total_cost / totalCost) * 100).toFixed(1)) : 0,
    }))

  const requestsByModel = (summary?.by_model ?? [])
    .sort((a, b) => b.total_requests - a.total_requests)
    .map((m) => ({
      model: m.model,
      requests: m.total_requests,
    }))

  const avgLatency = logs.length
    ? Math.round(logs.reduce((s, l) => s + l.duration_ms, 0) / logs.length)
    : 0

  const errorRate = summary?.summary
    ? ((summary.summary.error_count / summary.summary.total_requests) * 100).toFixed(1)
    : "0"

  const hasFilters = modelFilter || providerFilter || statusFilter !== "all"

  if (loading && !summary) {
    return (
      <div className="space-y-6 p-6">
        <PageHeader title="Usage" description="Request logs, token usage, and cost analytics" />
        <MetricCardSkeleton count={4} />
      </div>
    )
  }

  return (
    <div className="space-y-6 p-6">
      <PageHeader
        title="Usage"
        description="Request logs, token usage, and cost analytics"
      />

      {/* KPIs */}
      <div className="grid grid-cols-1 sm:grid-cols-2 lg:grid-cols-4 gap-4">
        <MetricCard
          label="Total Requests"
          value={summary?.summary.total_requests.toLocaleString() ?? "0"}
          icon={Activity}
        />
        <MetricCard
          label="Total Cost"
          value={`$${(summary?.summary.total_cost ?? 0).toFixed(4)}`}
          icon={BarChart3}
        />
        <MetricCard
          label="Avg Latency"
          value={`${avgLatency}ms`}
          icon={Clock}
        />
        <MetricCard
          label="Error Rate"
          value={`${errorRate}%`}
          description={`${summary?.summary.error_count ?? 0} errors`}
          icon={AlertTriangle}
        />
      </div>

      {/* Charts */}
      <div className="grid grid-cols-1 lg:grid-cols-2 gap-6">
        <ChartCard title="Cost by Model">
          {costByModel.length > 0 ? (
            <DonutChartView data={costByModel} nameKey="model" valueKey="pct" />
          ) : (
            <p className="text-sm text-muted-foreground text-center py-8">No cost data</p>
          )}
        </ChartCard>
        <ChartCard title="Requests by Model">
          {requestsByModel.length > 0 ? (
            <HBarChartView data={requestsByModel} categoryKey="model" valueKey="requests" />
          ) : (
            <p className="text-sm text-muted-foreground text-center py-8">No request data</p>
          )}
        </ChartCard>
      </div>

      {/* Logs */}
      <Card>
        <CardHeader>
          <div className="flex items-center justify-between">
            <div>
              <CardTitle className="font-mono text-sm">Request Logs</CardTitle>
              <CardDescription>{totalLogs} total requests</CardDescription>
            </div>
          </div>
          {/* Filters */}
          <div className="flex flex-wrap gap-3 pt-2">
            <div className="flex items-center gap-2">
              <Search className="h-4 w-4 text-muted-foreground" />
              <Input
                placeholder="Filter by model..."
                value={modelFilter}
                onChange={(e) => setModelFilter(e.target.value)}
                className="h-8 w-[160px]"
              />
            </div>
            <Input
              placeholder="Filter by provider..."
              value={providerFilter}
              onChange={(e) => setProviderFilter(e.target.value)}
              className="h-8 w-[160px]"
            />
            <Select value={statusFilter} onValueChange={(v) => v && setStatusFilter(v)}>
              <SelectTrigger className="h-8 w-[120px]">
                <SelectValue />
              </SelectTrigger>
              <SelectContent>
                <SelectItem value="all">All status</SelectItem>
                <SelectItem value="success">Success</SelectItem>
                <SelectItem value="error">Errors</SelectItem>
              </SelectContent>
            </Select>
            {hasFilters && (
              <Button
                variant="ghost"
                size="sm"
                className="h-8"
                onClick={() => {
                  setModelFilter("")
                  setProviderFilter("")
                  setStatusFilter("all")
                }}
              >
                <X className="h-3 w-3 mr-1" />
                Clear
              </Button>
            )}
          </div>
        </CardHeader>
        <CardContent>
          <Table>
            <TableHeader>
              <TableRow>
                <TableHead className="font-mono text-xs">Model</TableHead>
                <TableHead className="font-mono text-xs">Provider</TableHead>
                <TableHead className="font-mono text-xs text-right">Tokens</TableHead>
                <TableHead className="font-mono text-xs text-right">Cost</TableHead>
                <TableHead className="font-mono text-xs text-right">Latency</TableHead>
                <TableHead className="font-mono text-xs">Status</TableHead>
                <TableHead className="font-mono text-xs text-right">Time</TableHead>
                <TableHead className="font-mono text-xs w-8" />
              </TableRow>
            </TableHeader>
            <TableBody>
              {logs.map((log) => (
                <TableRow
                  key={log.id}
                  className="cursor-pointer"
                  onClick={() => openDetail(log.id)}
                >
                  <TableCell>
                    <div className="space-y-0.5">
                      <span className="font-mono text-sm">{log.used_model}</span>
                      {log.requested_model !== log.used_model && (
                        <p className="text-[10px] text-muted-foreground">
                          requested: {log.requested_model}
                        </p>
                      )}
                    </div>
                  </TableCell>
                  <TableCell>
                    <Badge variant="outline" className="font-mono text-[10px]">
                      {log.used_provider}
                    </Badge>
                  </TableCell>
                  <TableCell className="text-right font-mono tabular-nums text-sm">
                    {log.total_tokens.toLocaleString()}
                  </TableCell>
                  <TableCell className="text-right font-mono tabular-nums text-sm">
                    ${log.total_cost.toFixed(4)}
                  </TableCell>
                  <TableCell className="text-right font-mono tabular-nums text-sm text-muted-foreground">
                    {log.duration_ms}ms
                  </TableCell>
                  <TableCell>
                    {log.has_error ? (
                      <Badge variant="destructive" className="font-mono text-[10px]">
                        {log.status_code}
                      </Badge>
                    ) : (
                      <Badge variant="secondary" className="font-mono text-[10px]">
                        {log.status_code}
                      </Badge>
                    )}
                    {log.streamed && (
                      <Badge variant="outline" className="font-mono text-[10px] ml-1">
                        stream
                      </Badge>
                    )}
                  </TableCell>
                  <TableCell className="text-right text-xs text-muted-foreground whitespace-nowrap">
                    {timeAgo(log.created_at)}
                  </TableCell>
                  <TableCell>
                    <ChevronRight className="h-4 w-4 text-muted-foreground" />
                  </TableCell>
                </TableRow>
              ))}
              {logs.length === 0 && (
                <TableRow>
                  <TableCell colSpan={8} className="text-center text-sm text-muted-foreground py-8">
                    No logs found
                  </TableCell>
                </TableRow>
              )}
            </TableBody>
          </Table>
        </CardContent>
      </Card>

      {/* Detail Dialog */}
      <Dialog open={detailOpen} onOpenChange={(v) => { setDetailOpen(v); if (!v) setDetailLog(null) }}>
        <DialogContent className="max-w-2xl max-h-[80vh] overflow-y-auto">
          <DialogHeader>
            <DialogTitle className="font-mono">Request Detail</DialogTitle>
          </DialogHeader>
          {detailLoading ? (
            <div className="flex items-center justify-center py-12">
              <Loader2 className="h-6 w-6 animate-spin text-muted-foreground" />
            </div>
          ) : detailLog ? (
            <div className="space-y-4">
              {/* Overview grid */}
              <div className="grid grid-cols-2 gap-3 text-sm">
                <DetailField label="Request ID" value={detailLog.id} />
                <DetailField label="Status" value={
                  detailLog.has_error
                    ? `${detailLog.status_code} (error)`
                    : `${detailLog.status_code} (ok)`
                } />
                <DetailField label="Requested Model" value={detailLog.requested_model} />
                <DetailField label="Used Model" value={detailLog.used_model} />
                <DetailField label="Provider" value={detailLog.used_provider} />
                <DetailField label="Streamed" value={detailLog.streamed ? "Yes" : "No"} />
                <DetailField label="Duration" value={`${detailLog.duration_ms}ms`} />
                <DetailField label="Time" value={new Date(detailLog.created_at).toLocaleString()} />
              </div>

              {/* Tokens & cost */}
              <div className="border rounded-md p-3">
                <p className="font-mono text-xs text-muted-foreground mb-2">Tokens & Cost</p>
                <div className="grid grid-cols-3 gap-3 text-sm">
                  <DetailField label="Prompt" value={detailLog.prompt_tokens.toLocaleString()} />
                  <DetailField label="Completion" value={detailLog.completion_tokens.toLocaleString()} />
                  <DetailField label="Total" value={detailLog.total_tokens.toLocaleString()} />
                  <DetailField label="Input Cost" value={`$${detailLog.input_cost.toFixed(6)}`} />
                  <DetailField label="Output Cost" value={`$${detailLog.output_cost.toFixed(6)}`} />
                  <DetailField label="Total Cost" value={`$${detailLog.total_cost.toFixed(6)}`} />
                </div>
              </div>

              {/* Messages */}
              {detailLog.messages ? (
                <JsonBlock label="Messages" data={detailLog.messages} />
              ) : null}
              {detailLog.response_body ? (
                <JsonBlock label="Response Body" data={detailLog.response_body} />
              ) : null}
            </div>
          ) : null}
        </DialogContent>
      </Dialog>
    </div>
  )
}

/* ------------------------------------------------------------------ */
/*  Helpers                                                            */
/* ------------------------------------------------------------------ */

function DetailField({ label, value }: { label: string; value: string }) {
  return (
    <div>
      <p className="font-mono text-[10px] text-muted-foreground">{label}</p>
      <p className="font-mono text-sm">{value}</p>
    </div>
  )
}

function JsonBlock({ label, data }: { label: string; data: unknown }) {
  return (
    <div>
      <p className="font-mono text-xs text-muted-foreground mb-1">{label}</p>
      <pre className="text-xs bg-muted/50 rounded-md p-3 overflow-x-auto max-h-[200px] overflow-y-auto font-mono">
        {JSON.stringify(data, null, 2)}
      </pre>
    </div>
  )
}

function timeAgo(dateStr: string): string {
  const diff = Date.now() - new Date(dateStr).getTime()
  const mins = Math.floor(diff / 60000)
  if (mins < 1) return "just now"
  if (mins < 60) return `${mins}m ago`
  const hours = Math.floor(mins / 60)
  if (hours < 24) return `${hours}h ago`
  const days = Math.floor(hours / 24)
  return `${days}d ago`
}
