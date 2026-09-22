import { useEffect, useState } from "react"
import {
  Box, Server,
} from "lucide-react"
import { useApi } from "@/api"
import type {
  Model, ModelProviderMapping, Provider,
} from "@/api/types"
import { PageHeader, EmptyState, CapabilityBadge } from "@/components"
import { MetricCardSkeleton } from "@/components/feedback/skeletons"
import { StatusBadge } from "@/components/data/status"
import { Badge } from "@/components/ui/badge"
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from "@/components/ui/card"
import {
  Dialog, DialogContent, DialogDescription,
  DialogHeader, DialogTitle,
} from "@/components/ui/dialog"
import { Separator } from "@/components/ui/separator"
import { fetchAllPages } from "@/lib/utils"

interface ModelWithMappings {
  model: Model
  mappings: ModelProviderMapping[]
}

export default function ModelsPage() {
  const api = useApi()
  const [modelsWithMappings, setModelsWithMappings] = useState<ModelWithMappings[]>([])
  const [providers, setProviders] = useState<Provider[]>([])
  const [loading, setLoading] = useState(true)
  const [detailModel, setDetailModel] = useState<ModelWithMappings | null>(null)

  async function load() {
    const [modelList, provList, mappingItems] = await Promise.all([
      fetchAllPages((p) => api.models.list(p)),
      api.providers.list({ limit: 100 }),
      fetchAllPages((p) => api.mappings.list(p)),
    ])
    setProviders(provList.items)
    const mappingsByModel = new Map<string, ModelProviderMapping[]>()
    for (const m of mappingItems) {
      const list = mappingsByModel.get(m.model_id) ?? []
      list.push(m)
      mappingsByModel.set(m.model_id, list)
    }
    setModelsWithMappings(
      modelList.map((model) => ({ model, mappings: mappingsByModel.get(model.id) ?? [] }))
    )
    setLoading(false)
  }

  useEffect(() => { load() }, [api])

  const providerMap = Object.fromEntries(providers.map((p) => [p.id, p]))

  if (loading) {
    return (
      <div className="space-y-6 p-6">
        <PageHeader title="Models" description="Browse the operator-managed model catalog" />
        <MetricCardSkeleton count={3} />
      </div>
    )
  }

  const models = modelsWithMappings.map((m) => m.model)
  const families = [...new Set(models.map((m) => m.family))]
  const active = models.filter((m) => m.status === "active").length

  return (
    <div className="space-y-6 p-6">
      <PageHeader
        title="Models"
        description="Browse the operator-managed model catalog"
      />

      <div className="flex items-center gap-4 flex-wrap">
        <Badge variant="outline" className="font-mono text-xs gap-1.5">
          <span className="h-1.5 w-1.5 rounded-full bg-success" />
          {active} active
        </Badge>
        <span className="text-xs text-muted-foreground font-mono">{models.length} total</span>
        <Separator orientation="vertical" className="!h-3" />
        {families.map((f) => (
          <Badge key={f} variant="secondary" className="font-mono text-[10px]">{f}</Badge>
        ))}
      </div>

      {models.length === 0 ? (
        <EmptyState
          icon={Box}
          title="No models"
          description="No models have been configured by the operator."
        />
      ) : (
        <div className="space-y-3">
          {modelsWithMappings.map(({ model, mappings }) => {
            const caps = {
              streaming: mappings.some((m) => m.streaming),
              vision: mappings.some((m) => m.vision),
              reasoning: mappings.some((m) => m.reasoning),
              tools: mappings.some((m) => m.tools),
              json_output: mappings.some((m) => m.json_output),
            }
            const cheapestInput = mappings
              .filter((m) => m.input_price !== undefined)
              .sort((a, b) => a.input_price! - b.input_price!)[0]
            const maxCtx = Math.max(0, ...mappings.map((m) => m.context_size ?? 0))

            return (
              <Card
                key={model.id}
                className="hover:border-primary/30 transition-colors cursor-pointer group"
                onClick={() => setDetailModel({ model, mappings })}
              >
                <CardContent className="p-4">
                  <div className="flex items-start justify-between gap-4">
                    <div className="flex-1 min-w-0">
                      <div className="flex items-center gap-2 flex-wrap">
                        <span className="font-mono text-sm font-semibold">{model.name}</span>
                        <Badge variant="secondary" className="font-mono text-[10px]">{model.family}</Badge>
                        <StabilityBadge stability={model.stability} />
                        {model.free && <Badge variant="secondary" className="font-mono text-[10px] px-1.5">free</Badge>}
                        <StatusBadge
                          status={model.status === "active" ? "online" : "offline"}
                          label={model.status.toUpperCase()}
                        />
                      </div>
                      <p className="text-xs text-muted-foreground mt-1 line-clamp-1">
                        {model.description}
                      </p>

                      <div className="flex items-center gap-3 mt-3">
                        <div className="flex items-center gap-1 text-xs text-muted-foreground">
                          <Server className="h-3 w-3" />
                          <span className="font-mono">{mappings.length} provider{mappings.length !== 1 ? "s" : ""}</span>
                        </div>
                        <Separator orientation="vertical" className="!h-3" />
                        <div className="flex gap-1.5 flex-wrap">
                          {mappings.map((m) => (
                            <Badge key={m.id} variant="outline" className="font-mono text-[10px] px-1.5 gap-1">
                              <span className={`h-1.5 w-1.5 rounded-full ${m.status === "active" ? "bg-success" : "bg-muted-foreground"}`} />
                              {providerMap[m.provider_id]?.name ?? m.provider_id}
                            </Badge>
                          ))}
                        </div>
                      </div>

                      <div className="flex items-center gap-3 mt-2 flex-wrap">
                        <div className="flex gap-1 flex-wrap">
                          {caps.streaming && <CapabilityBadge>stream</CapabilityBadge>}
                          {caps.vision && <CapabilityBadge>vision</CapabilityBadge>}
                          {caps.reasoning && <CapabilityBadge>reasoning</CapabilityBadge>}
                          {caps.tools && <CapabilityBadge>tools</CapabilityBadge>}
                          {caps.json_output && <CapabilityBadge>json</CapabilityBadge>}
                        </div>
                        <Separator orientation="vertical" className="!h-3" />
                        <div className="flex gap-3 text-xs text-muted-foreground font-mono">
                          {cheapestInput && (
                            <span>from ${cheapestInput.input_price}/M in</span>
                          )}
                          {maxCtx > 0 && (
                            <span>{(maxCtx / 1000).toFixed(0)}K ctx</span>
                          )}
                        </div>
                      </div>
                    </div>
                  </div>
                </CardContent>
              </Card>
            )
          })}
        </div>
      )}

      {detailModel && (
        <ModelDetailDialog
          data={detailModel}
          providerMap={providerMap}
          open={!!detailModel}
          onOpenChange={(open) => { if (!open) setDetailModel(null) }}
        />
      )}
    </div>
  )
}

function StabilityBadge({ stability }: { stability: string }) {
  const colors: Record<string, string> = {
    stable: "text-success border-success/30",
    beta: "text-warning border-warning/30",
    experimental: "text-destructive border-destructive/30",
  }
  return (
    <Badge variant="outline" className={`font-mono text-[10px] ${colors[stability] ?? ""}`}>
      {stability}
    </Badge>
  )
}

function ModelDetailDialog({
  data,
  providerMap,
  open,
  onOpenChange,
}: {
  data: ModelWithMappings
  providerMap: Record<string, Provider>
  open: boolean
  onOpenChange: (open: boolean) => void
}) {
  const { model, mappings } = data

  return (
    <Dialog open={open} onOpenChange={onOpenChange}>
      <DialogContent className="max-w-2xl">
        <DialogHeader>
          <DialogTitle className="font-mono flex items-center gap-2">
            {model.name}
            <StabilityBadge stability={model.stability} />
            {model.free && <Badge variant="secondary" className="font-mono text-[10px]">free</Badge>}
          </DialogTitle>
          <DialogDescription>{model.description}</DialogDescription>
        </DialogHeader>

        <div className="space-y-1 text-sm">
          <div className="flex justify-between">
            <span className="text-muted-foreground font-mono text-xs">Family</span>
            <span className="font-mono text-xs">{model.family}</span>
          </div>
          <div className="flex justify-between">
            <span className="text-muted-foreground font-mono text-xs">Status</span>
            <StatusBadge
              status={model.status === "active" ? "online" : "offline"}
              label={model.status.toUpperCase()}
            />
          </div>
        </div>

        <Separator />

        <div>
          <h4 className="font-mono text-sm font-semibold mb-3">
            Providers ({mappings.length})
          </h4>
          {mappings.length === 0 ? (
            <p className="text-sm text-muted-foreground text-center py-6">
              No providers connected to this model.
            </p>
          ) : (
            <div className="space-y-3">
              {mappings.map((m) => {
                const provider = providerMap[m.provider_id]
                return (
                  <Card key={m.id}>
                    <CardHeader className="pb-2">
                      <div className="flex items-center justify-between">
                        <CardTitle className="font-mono text-xs">
                          {provider?.name ?? m.provider_id}
                        </CardTitle>
                        <StatusBadge
                          status={m.status === "active" ? "online" : "offline"}
                          label={m.status.toUpperCase()}
                        />
                      </div>
                      <CardDescription className="font-mono text-[10px]">
                        {m.external_id}
                      </CardDescription>
                    </CardHeader>
                    <CardContent className="space-y-2">
                      <div className="grid grid-cols-2 sm:grid-cols-4 gap-2 text-xs">
                        <PriceCell label="Input" value={m.input_price} />
                        <PriceCell label="Output" value={m.output_price} />
                        <PriceCell label="Cached" value={m.cached_input_price} />
                        <PriceCell label="Image" value={m.image_input_price} />
                      </div>

                      <div className="flex gap-4 text-xs">
                        {m.context_size && (
                          <span className="text-muted-foreground font-mono">
                            ctx: {(m.context_size / 1000).toFixed(0)}K
                          </span>
                        )}
                        {m.max_output && (
                          <span className="text-muted-foreground font-mono">
                            max out: {(m.max_output / 1000).toFixed(0)}K
                          </span>
                        )}
                      </div>

                      <div className="flex gap-1.5 flex-wrap">
                        {m.streaming && <CapabilityBadge>stream</CapabilityBadge>}
                        {m.vision && <CapabilityBadge>vision</CapabilityBadge>}
                        {m.reasoning && <CapabilityBadge>reasoning</CapabilityBadge>}
                        {m.tools && <CapabilityBadge>tools</CapabilityBadge>}
                        {m.json_output && <CapabilityBadge>json</CapabilityBadge>}
                      </div>
                    </CardContent>
                  </Card>
                )
              })}
            </div>
          )}
        </div>
      </DialogContent>
    </Dialog>
  )
}

function PriceCell({ label, value }: { label: string; value?: number }) {
  return (
    <div>
      <span className="text-muted-foreground font-mono">{label}</span>
      <p className="font-mono tabular-nums">
        {value !== undefined ? `$${value}/M` : "-"}
      </p>
    </div>
  )
}
