import { useEffect, useState } from "react"
import {
  Box, Plus, MoreHorizontal, Pencil, Trash2, Server,
} from "lucide-react"
import { useApi } from "@/api"
import type {
  Model, CreateModelRequest, UpdateModelRequest, ModelWithMappings, Provider,
} from "@/api/types"
import { PageHeader, EmptyState, CapabilityBadge } from "@/components"
import { MetricCardSkeleton } from "@/components/feedback/skeletons"
import { StatusBadge } from "@/components/data/status"
import { Badge } from "@/components/ui/badge"
import { Button } from "@/components/ui/button"
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from "@/components/ui/card"
import {
  Dialog, DialogContent, DialogDescription, DialogFooter,
  DialogHeader, DialogTitle,
} from "@/components/ui/dialog"
import {
  DropdownMenu, DropdownMenuContent, DropdownMenuItem,
  DropdownMenuSeparator, DropdownMenuTrigger,
} from "@/components/ui/dropdown-menu"
import { Input } from "@/components/ui/input"
import { Label } from "@/components/ui/label"
import { Switch } from "@/components/ui/switch"
import { Separator } from "@/components/ui/separator"

export default function ModelsPage() {
  const api = useApi()
  const [modelsWithMappings, setModelsWithMappings] = useState<ModelWithMappings[]>([])
  const [providers, setProviders] = useState<Provider[]>([])
  const [loading, setLoading] = useState(true)
  const [createOpen, setCreateOpen] = useState(false)
  const [editModel, setEditModel] = useState<Model | null>(null)
  const [detailModel, setDetailModel] = useState<ModelWithMappings | null>(null)

  async function load() {
    const [modelList, provList] = await Promise.all([
      api.models.list(),
      api.providers.list(),
    ])
    setProviders(provList.data)
    const details = await Promise.all(
      modelList.data.map((m) => api.models.getWithMappings(m.id))
    )
    setModelsWithMappings(details)
    setLoading(false)
  }

  useEffect(() => { load() }, [api])

  const providerMap = Object.fromEntries(providers.map((p) => [p.id, p]))

  async function handleCreate(req: CreateModelRequest) {
    await api.models.create(req)
    setCreateOpen(false)
    load()
  }

  async function handleUpdate(id: string, req: UpdateModelRequest) {
    await api.models.update(id, req)
    setEditModel(null)
    load()
  }

  async function handleDelete(id: string) {
    await api.models.delete(id)
    load()
  }

  if (loading) {
    return (
      <div className="space-y-6 p-6">
        <PageHeader title="Models" description="Manage virtual models and provider mappings" />
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
        description="Manage virtual models and provider mappings"
        actions={
          <Button size="sm" onClick={() => setCreateOpen(true)}>
            <Plus className="h-4 w-4 mr-1" /> Add Model
          </Button>
        }
      />

      {/* Summary */}
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
          description="Add your first virtual model to start routing requests."
          action={
            <Button size="sm" onClick={() => setCreateOpen(true)}>
              <Plus className="h-4 w-4 mr-1" /> Add Model
            </Button>
          }
        />
      ) : (
        <div className="space-y-3">
          {modelsWithMappings.map(({ model, mappings }) => {
            // aggregate capabilities from all mappings
            const caps = {
              streaming: mappings.some((m) => m.streaming),
              vision: mappings.some((m) => m.vision),
              reasoning: mappings.some((m) => m.reasoning),
              tools: mappings.some((m) => m.tools),
              json_output: mappings.some((m) => m.json_output),
            }
            // cheapest input price
            const cheapestInput = mappings
              .filter((m) => m.input_price !== null)
              .sort((a, b) => a.input_price! - b.input_price!)[0]
            // max context
            const maxCtx = Math.max(...mappings.map((m) => m.context_size ?? 0))

            return (
              <Card
                key={model.id}
                className="hover:border-primary/30 transition-colors cursor-pointer group"
                onClick={() => setDetailModel({ model, mappings })}
              >
                <CardContent className="p-4">
                  <div className="flex items-start justify-between gap-4">
                    {/* Left: name + meta */}
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

                      {/* Providers row */}
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

                      {/* Capabilities + pricing row */}
                      <div className="flex items-center gap-3 mt-2 flex-wrap">
                        {/* Capabilities */}
                        <div className="flex gap-1 flex-wrap">
                          {caps.streaming && <CapabilityBadge>stream</CapabilityBadge>}
                          {caps.vision && <CapabilityBadge>vision</CapabilityBadge>}
                          {caps.reasoning && <CapabilityBadge>reasoning</CapabilityBadge>}
                          {caps.tools && <CapabilityBadge>tools</CapabilityBadge>}
                          {caps.json_output && <CapabilityBadge>json</CapabilityBadge>}
                        </div>
                        <Separator orientation="vertical" className="!h-3" />
                        {/* Pricing + context */}
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

                    {/* Right: actions */}
                    <div onClick={(e) => e.stopPropagation()}>
                      <DropdownMenu>
                        <DropdownMenuTrigger>
                          <Button variant="ghost" size="icon" className="h-8 w-8">
                            <MoreHorizontal className="h-4 w-4" />
                          </Button>
                        </DropdownMenuTrigger>
                        <DropdownMenuContent align="end">
                          <DropdownMenuItem onClick={() => setEditModel(model)}>
                            <Pencil className="h-4 w-4 mr-2" /> Edit
                          </DropdownMenuItem>
                          <DropdownMenuSeparator />
                          <DropdownMenuItem
                            className="text-destructive"
                            onClick={() => handleDelete(model.id)}
                          >
                            <Trash2 className="h-4 w-4 mr-2" /> Delete
                          </DropdownMenuItem>
                        </DropdownMenuContent>
                      </DropdownMenu>
                    </div>
                  </div>
                </CardContent>
              </Card>
            )
          })}
        </div>
      )}

      {/* Create dialog */}
      <ModelFormDialog
        open={createOpen}
        onOpenChange={setCreateOpen}
        title="Add Model"
        description="Create a new virtual model."
        onSubmit={handleCreate}
      />

      {/* Edit dialog */}
      {editModel && (
        <ModelFormDialog
          open={!!editModel}
          onOpenChange={(open) => { if (!open) setEditModel(null) }}
          title="Edit Model"
          description={`Update ${editModel.name} configuration.`}
          defaults={editModel}
          onSubmit={(req) => handleUpdate(editModel.id, req)}
        />
      )}

      {/* Detail dialog */}
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

// ── Subcomponents ──

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

function ModelFormDialog({
  open,
  onOpenChange,
  title,
  description,
  defaults,
  onSubmit,
}: {
  open: boolean
  onOpenChange: (open: boolean) => void
  title: string
  description: string
  defaults?: Model
  onSubmit: (req: CreateModelRequest) => void
}) {
  const [name, setName] = useState(defaults?.name ?? "")
  const [desc, setDesc] = useState(defaults?.description ?? "")
  const [family, setFamily] = useState(defaults?.family ?? "")
  const [free, setFree] = useState(defaults?.free ?? false)

  useEffect(() => {
    if (open) {
      setName(defaults?.name ?? "")
      setDesc(defaults?.description ?? "")
      setFamily(defaults?.family ?? "")
      setFree(defaults?.free ?? false)
    }
  }, [open, defaults])

  function handleSubmit(e: React.FormEvent) {
    e.preventDefault()
    onSubmit({ name, description: desc, family, free })
  }

  return (
    <Dialog open={open} onOpenChange={onOpenChange}>
      <DialogContent>
        <DialogHeader>
          <DialogTitle className="font-mono">{title}</DialogTitle>
          <DialogDescription>{description}</DialogDescription>
        </DialogHeader>
        <form onSubmit={handleSubmit} className="space-y-4">
          <div className="space-y-2">
            <Label htmlFor="name" className="font-mono text-xs">Name</Label>
            <Input id="name" value={name} onChange={(e) => setName(e.target.value)} placeholder="e.g. GPT-4o" required />
          </div>
          <div className="space-y-2">
            <Label htmlFor="description" className="font-mono text-xs">Description</Label>
            <Input id="description" value={desc} onChange={(e) => setDesc(e.target.value)} placeholder="Brief description" required />
          </div>
          <div className="space-y-2">
            <Label htmlFor="family" className="font-mono text-xs">Family</Label>
            <Input id="family" value={family} onChange={(e) => setFamily(e.target.value)} placeholder="e.g. GPT, Claude, Gemini" required />
          </div>
          <div className="flex items-center justify-between">
            <Label htmlFor="free" className="font-mono text-xs">Free model</Label>
            <Switch id="free" checked={free} onCheckedChange={setFree} />
          </div>
          <DialogFooter>
            <Button type="button" variant="outline" onClick={() => onOpenChange(false)}>Cancel</Button>
            <Button type="submit" disabled={!name.trim()}>{defaults ? "Save Changes" : "Add Model"}</Button>
          </DialogFooter>
        </form>
      </DialogContent>
    </Dialog>
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
                      {/* Pricing */}
                      <div className="grid grid-cols-2 sm:grid-cols-4 gap-2 text-xs">
                        <PriceCell label="Input" value={m.input_price} />
                        <PriceCell label="Output" value={m.output_price} />
                        <PriceCell label="Cached" value={m.cached_input_price} />
                        <PriceCell label="Image" value={m.image_input_price} />
                      </div>

                      {/* Limits */}
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

                      {/* Capabilities */}
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

function PriceCell({ label, value }: { label: string; value: number | null }) {
  return (
    <div>
      <span className="text-muted-foreground font-mono">{label}</span>
      <p className="font-mono tabular-nums">
        {value !== null ? `$${value}/M` : "-"}
      </p>
    </div>
  )
}

