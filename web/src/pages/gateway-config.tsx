import { useEffect, useState } from "react"
import {
  Gauge, Save, RotateCcw, DollarSign, Loader2,
  ArrowDownUp, Shuffle, TrendingDown, Zap,
} from "lucide-react"
import { useApi } from "@/api"
import type {
  RateLimitConfig, RoutingConfig, RoutingStrategy,
} from "@/api/types"
import { PageHeader, MetricCard } from "@/components"
import { MetricCardSkeleton } from "@/components/feedback/skeletons"
import { Badge } from "@/components/ui/badge"
import { Button } from "@/components/ui/button"
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from "@/components/ui/card"
import { Input } from "@/components/ui/input"
import { Label } from "@/components/ui/label"

const TENANT_ID = "default"

const STRATEGY_META: Record<RoutingStrategy, { label: string; description: string; icon: typeof Zap }> = {
  cheapest: {
    label: "Cheapest",
    description: "Routes to the provider with the lowest output price per token.",
    icon: DollarSign,
  },
  "lowest-latency": {
    label: "Lowest Latency",
    description: "Routes to the provider with the best recent average response time.",
    icon: TrendingDown,
  },
  "round-robin": {
    label: "Round Robin",
    description: "Distributes requests evenly across all available providers.",
    icon: Shuffle,
  },
}

export default function GatewayConfigPage() {
  const api = useApi()

  const [rateLimit, setRateLimit] = useState<RateLimitConfig | null>(null)
  const [routing, setRouting] = useState<RoutingConfig | null>(null)
  const [loading, setLoading] = useState(true)

  // rate limit form
  const [rpm, setRpm] = useState("")
  const [maxConcurrent, setMaxConcurrent] = useState("")
  const [rlSaving, setRlSaving] = useState(false)
  const [rlDirty, setRlDirty] = useState(false)

  // routing form
  const [strategy, setStrategy] = useState<RoutingStrategy | null>(null)
  const [rtSaving, setRtSaving] = useState(false)
  const [rtDirty, setRtDirty] = useState(false)

  // cache
  const [cacheInvalidating, setCacheInvalidating] = useState(false)
  const [cacheMsg, setCacheMsg] = useState("")

  async function load() {
    const [rl, rt] = await Promise.all([
      api.gatewayConfig.getRateLimit(TENANT_ID).catch(() => null),
      api.gatewayConfig.getRouting(TENANT_ID).catch(() => null),
    ])
    setRateLimit(rl)
    setRouting(rt)
    if (rl) {
      setRpm(String(rl.rpm))
      setMaxConcurrent(String(rl.max_concurrent))
    }
    if (rt) {
      setStrategy(rt.strategy)
    }
    setLoading(false)
  }

  useEffect(() => { load() }, [api])

  async function saveRateLimit() {
    setRlSaving(true)
    const saved = await api.gatewayConfig.upsertRateLimit(TENANT_ID, {
      rpm: Number(rpm) || undefined,
      max_concurrent: Number(maxConcurrent) || undefined,
    })
    setRateLimit(saved)
    setRlDirty(false)
    setRlSaving(false)
  }

  async function saveRouting() {
    setRtSaving(true)
    if (strategy) {
      const saved = await api.gatewayConfig.upsertRouting(TENANT_ID, { strategy })
      setRouting(saved)
    } else {
      await api.gatewayConfig.deleteRouting(TENANT_ID).catch(() => {})
      setRouting(null)
    }
    setRtDirty(false)
    setRtSaving(false)
  }

  async function invalidateCache() {
    setCacheInvalidating(true)
    await api.gatewayConfig.invalidateCache(TENANT_ID)
    setCacheMsg("Cache invalidated")
    setCacheInvalidating(false)
    setTimeout(() => setCacheMsg(""), 3000)
  }

  if (loading) {
    return (
      <div className="space-y-6 p-6">
        <PageHeader title="Gateway Config" description="Rate limits, routing strategy, and cache management" />
        <MetricCardSkeleton count={3} />
      </div>
    )
  }

  return (
    <div className="space-y-6 p-6">
      <PageHeader
        title="Gateway Config"
        description="Rate limits, routing strategy, and cache management"
      />

      {/* KPIs */}
      <div className="grid grid-cols-1 sm:grid-cols-2 gap-4">
        <MetricCard
          label="Rate Limit"
          value={rateLimit ? `${rateLimit.rpm} RPM` : "No limit"}
          description={rateLimit ? `${rateLimit.max_concurrent} max concurrent` : "Not configured"}
          icon={Gauge}
        />
        <MetricCard
          label="Routing Strategy"
          value={routing ? STRATEGY_META[routing.strategy].label : "Not configured"}
          description={routing ? `Updated ${new Date(routing.updated_at).toLocaleDateString()}` : "Uses cheapest by default"}
          icon={ArrowDownUp}
        />
      </div>

      <div className="grid grid-cols-1 lg:grid-cols-2 gap-6">
        {/* Rate Limits */}
        <Card>
          <CardHeader>
            <CardTitle className="font-mono text-sm">Rate Limits</CardTitle>
            <CardDescription>
              Control request throughput per tenant
            </CardDescription>
          </CardHeader>
          <CardContent className="space-y-4">
            <div className="space-y-2">
              <Label htmlFor="rpm" className="font-mono text-xs">Requests per minute (RPM)</Label>
              <Input
                id="rpm"
                type="number"
                value={rpm}
                onChange={(e) => { setRpm(e.target.value); setRlDirty(true) }}
                placeholder="e.g. 60"
              />
            </div>
            <div className="space-y-2">
              <Label htmlFor="concurrent" className="font-mono text-xs">Max concurrent requests</Label>
              <Input
                id="concurrent"
                type="number"
                value={maxConcurrent}
                onChange={(e) => { setMaxConcurrent(e.target.value); setRlDirty(true) }}
                placeholder="e.g. 10"
              />
            </div>
            <Button
              size="sm"
              onClick={saveRateLimit}
              disabled={!rlDirty || rlSaving}
            >
              {rlSaving ? <Loader2 className="h-4 w-4 mr-1 animate-spin" /> : <Save className="h-4 w-4 mr-1" />}
              Save
            </Button>
          </CardContent>
        </Card>

        {/* Routing Strategy */}
        <Card>
          <CardHeader>
            <CardTitle className="font-mono text-sm">Routing Strategy</CardTitle>
            <CardDescription>
              How requests are routed when multiple providers are available
            </CardDescription>
          </CardHeader>
          <CardContent className="space-y-4">
            <div className="space-y-3">
              {(Object.entries(STRATEGY_META) as [RoutingStrategy, typeof STRATEGY_META["cheapest"]][]).map(([key, meta]) => {
                const Icon = meta.icon
                const isSaved = routing?.strategy === key
                const selected = strategy === key
                return (
                  <div
                    key={key}
                    className={`flex items-start gap-3 p-3 rounded-md border cursor-pointer transition-colors ${
                      selected
                        ? "border-primary/50 bg-primary/5"
                        : isSaved
                          ? "border-primary/30 bg-primary/[0.02]"
                          : "border-border hover:border-primary/20"
                    }`}
                    onClick={() => {
                      const next = strategy === key ? null : key
                      setStrategy(next)
                      setRtDirty(next !== (routing?.strategy ?? null))
                    }}
                  >
                    <div className={`flex h-8 w-8 shrink-0 items-center justify-center rounded-sm ${
                      selected ? "bg-primary/10 text-primary" : isSaved ? "bg-primary/5 text-primary/60" : "bg-muted text-muted-foreground"
                    }`}>
                      <Icon className="h-4 w-4" />
                    </div>
                    <div>
                      <div className="flex items-center gap-2">
                        <span className="font-mono text-sm font-medium">{meta.label}</span>
                        {isSaved && <Badge variant="secondary" className="font-mono text-[10px]">active</Badge>}
                      </div>
                      <p className="text-xs text-muted-foreground mt-0.5">{meta.description}</p>
                    </div>
                  </div>
                )
              })}
            </div>
            <Button
              size="sm"
              onClick={saveRouting}
              disabled={!rtDirty || rtSaving}
            >
              {rtSaving ? <Loader2 className="h-4 w-4 mr-1 animate-spin" /> : <Save className="h-4 w-4 mr-1" />}
              Save
            </Button>
          </CardContent>
        </Card>
      </div>

      <div className="grid grid-cols-1 gap-6">
        {/* Cache */}
        <Card>
          <CardHeader>
            <CardTitle className="font-mono text-sm">Cache</CardTitle>
            <CardDescription>
              Invalidate cached routing and config data
            </CardDescription>
          </CardHeader>
          <CardContent>
            <p className="text-sm text-muted-foreground mb-4">
              The gateway caches routing strategies and provider key health.
              Invalidate if you've changed configuration and need it applied immediately.
            </p>
            <div className="flex items-center gap-3">
              <Button
                size="sm"
                variant="outline"
                onClick={invalidateCache}
                disabled={cacheInvalidating}
              >
                {cacheInvalidating ? (
                  <Loader2 className="h-4 w-4 mr-1 animate-spin" />
                ) : (
                  <RotateCcw className="h-4 w-4 mr-1" />
                )}
                Invalidate Cache
              </Button>
              {cacheMsg && (
                <span className="text-xs text-success font-mono">{cacheMsg}</span>
              )}
            </div>
          </CardContent>
        </Card>
      </div>
    </div>
  )
}
