import { useEffect, useState } from "react"
import {
  Server, ExternalLink, Zap, Plus, Pencil, Trash2, MoreHorizontal,
} from "lucide-react"
import { toast } from "sonner"
import { useApi } from "@/api"
import type { Provider, CreateProviderRequest, UpdateProviderRequest, ProviderProtocol } from "@/api/types"
import { PROVIDER_PROTOCOLS } from "@/api/types"
import { PageHeader, EmptyState, StatusBadge, ConfirmDialog } from "@/components"
import { MetricCardSkeleton } from "@/components/feedback/skeletons"
import { Badge } from "@/components/ui/badge"
import { Button } from "@/components/ui/button"
import { Card, CardContent } from "@/components/ui/card"
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
import {
  Select, SelectContent, SelectItem, SelectTrigger, SelectValue,
} from "@/components/ui/select"
import { Switch } from "@/components/ui/switch"
import { Textarea } from "@/components/ui/textarea"
import {
  Table, TableBody, TableCell, TableHead, TableHeader, TableRow,
} from "@/components/ui/table"

export default function ProvidersPage() {
  const api = useApi()
  const [providers, setProviders] = useState<Provider[]>([])
  const [loading, setLoading] = useState(true)

  const [createOpen, setCreateOpen] = useState(false)
  const [editProvider, setEditProvider] = useState<Provider | null>(null)
  const [deleteId, setDeleteId] = useState<string | null>(null)

  async function load() {
    const res = await api.providers.list({ limit: 100 })
    setProviders(res.items)
    setLoading(false)
  }

  useEffect(() => { load() }, [api])

  async function handleCreate(req: CreateProviderRequest) {
    await api.providers.create(req)
    setCreateOpen(false)
    toast.success("Provider created")
    load()
  }

  async function handleUpdate(id: string, req: UpdateProviderRequest) {
    await api.providers.update(id, req)
    setEditProvider(null)
    toast.success("Provider updated")
    load()
  }

  async function handleDelete() {
    if (!deleteId) return
    await api.providers.delete(deleteId)
    setDeleteId(null)
    toast.success("Provider deleted")
    load()
  }

  if (loading) {
    return (
      <div className="space-y-6 p-6">
        <PageHeader title="Providers" description="Manage upstream LLM providers" />
        <MetricCardSkeleton count={3} />
      </div>
    )
  }

  const active = providers.filter((p) => p.status === "active").length
  const inactive = providers.length - active

  return (
    <div className="space-y-6 p-6">
      <PageHeader
        title="Providers"
        description="Manage upstream LLM providers"
        actions={
          <Button size="sm" onClick={() => setCreateOpen(true)}>
            <Plus className="h-4 w-4" /> New provider
          </Button>
        }
      />

      <div className="flex items-center gap-4">
        <Badge variant="outline" className="font-mono text-xs gap-1.5">
          <span className="h-1.5 w-1.5 rounded-full bg-success" />
          {active} active
        </Badge>
        {inactive > 0 && (
          <Badge variant="outline" className="font-mono text-xs gap-1.5">
            <span className="h-1.5 w-1.5 rounded-full bg-muted-foreground" />
            {inactive} inactive
          </Badge>
        )}
        <span className="text-xs text-muted-foreground font-mono">{providers.length} total</span>
      </div>

      {providers.length === 0 ? (
        <EmptyState
          icon={Server}
          title="No providers"
          description="Add a provider to start routing requests."
          action={
            <Button size="sm" onClick={() => setCreateOpen(true)}>
              <Plus className="h-4 w-4" /> New provider
            </Button>
          }
        />
      ) : (
        <Card>
          <CardContent className="p-0">
            <Table>
              <TableHeader>
                <TableRow>
                  <TableHead className="font-mono text-xs">Name</TableHead>
                  <TableHead className="font-mono text-xs">Protocol</TableHead>
                  <TableHead className="font-mono text-xs">Description</TableHead>
                  <TableHead className="font-mono text-xs">Status</TableHead>
                  <TableHead className="font-mono text-xs">Streaming</TableHead>
                  <TableHead className="font-mono text-xs">Created</TableHead>
                  <TableHead className="font-mono text-xs w-[50px]" />
                </TableRow>
              </TableHeader>
              <TableBody>
                {providers.map((provider) => (
                  <TableRow key={provider.id}>
                    <TableCell>
                      <div className="flex items-center gap-2">
                        <span className="font-mono text-sm font-medium">{provider.name}</span>
                        {provider.website && (
                          <a
                            href={provider.website}
                            target="_blank"
                            rel="noopener noreferrer"
                            className="text-muted-foreground hover:text-foreground transition-colors"
                          >
                            <ExternalLink className="h-3 w-3" />
                          </a>
                        )}
                      </div>
                    </TableCell>
                    <TableCell>
                      <Badge variant="outline" className="font-mono text-[10px]">
                        {PROVIDER_PROTOCOLS.find((p) => p.value === provider.protocol)?.label ?? provider.protocol}
                      </Badge>
                    </TableCell>
                    <TableCell className="text-sm text-muted-foreground max-w-[300px] truncate">
                      {provider.description}
                    </TableCell>
                    <TableCell>
                      <StatusBadge
                        status={provider.status === "active" ? "online" : "offline"}
                        label={provider.status.toUpperCase()}
                      />
                    </TableCell>
                    <TableCell>
                      {provider.streaming ? (
                        <Badge variant="secondary" className="font-mono text-[10px] gap-1">
                          <Zap className="h-3 w-3" /> stream
                        </Badge>
                      ) : (
                        <span className="text-xs text-muted-foreground">-</span>
                      )}
                    </TableCell>
                    <TableCell className="font-mono text-xs text-muted-foreground tabular-nums">
                      {new Date(provider.created_at).toLocaleDateString()}
                    </TableCell>
                    <TableCell>
                      <DropdownMenu>
                        <DropdownMenuTrigger>
                          <Button variant="ghost" size="icon-sm">
                            <MoreHorizontal className="h-4 w-4" />
                          </Button>
                        </DropdownMenuTrigger>
                        <DropdownMenuContent align="end">
                          <DropdownMenuItem onClick={() => setEditProvider(provider)}>
                            <Pencil className="h-3.5 w-3.5" /> Edit
                          </DropdownMenuItem>
                          <DropdownMenuSeparator />
                          <DropdownMenuItem variant="destructive" onClick={() => setDeleteId(provider.id)}>
                            <Trash2 className="h-3.5 w-3.5" /> Delete
                          </DropdownMenuItem>
                        </DropdownMenuContent>
                      </DropdownMenu>
                    </TableCell>
                  </TableRow>
                ))}
              </TableBody>
            </Table>
          </CardContent>
        </Card>
      )}

      <ProviderDialog
        open={createOpen}
        onOpenChange={setCreateOpen}
        onSubmit={handleCreate}
      />
      <ProviderDialog
        provider={editProvider ?? undefined}
        open={!!editProvider}
        onOpenChange={(o) => !o && setEditProvider(null)}
        onSubmit={(req) => editProvider && handleUpdate(editProvider.id, req)}
      />

      <ConfirmDialog
        open={!!deleteId}
        onOpenChange={(o) => !o && setDeleteId(null)}
        title="Delete provider"
        description="This will permanently remove the provider. Existing model mappings and keys referencing it will stop working."
        confirmLabel="Delete"
        onConfirm={handleDelete}
      />
    </div>
  )
}

function ProviderDialog({
  provider,
  open,
  onOpenChange,
  onSubmit,
}: {
  provider?: Provider
  open: boolean
  onOpenChange: (open: boolean) => void
  onSubmit: (req: CreateProviderRequest) => void
}) {
  const [name, setName] = useState("")
  const [protocol, setProtocol] = useState<ProviderProtocol>("openai")
  const [description, setDescription] = useState("")
  const [website, setWebsite] = useState("")
  const [baseUrl, setBaseUrl] = useState("")
  const [streaming, setStreaming] = useState(true)

  useEffect(() => {
    if (open) {
      setName(provider?.name ?? "")
      setProtocol(provider?.protocol ?? "openai")
      setDescription(provider?.description ?? "")
      setWebsite(provider?.website ?? "")
      setBaseUrl(provider?.base_url ?? "")
      setStreaming(provider?.streaming ?? true)
    }
  }, [open, provider])

  function handleSubmit() {
    onSubmit({
      name,
      protocol,
      description,
      website: website || undefined,
      base_url: baseUrl,
      streaming,
    })
  }

  return (
    <Dialog open={open} onOpenChange={onOpenChange}>
      <DialogContent>
        <DialogHeader>
          <DialogTitle className="font-mono">{provider ? "Edit provider" : "New provider"}</DialogTitle>
          <DialogDescription>
            {provider ? "Update the provider configuration." : "Register a new upstream provider."}
          </DialogDescription>
        </DialogHeader>
        <div className="space-y-4">
          <div className="space-y-1.5">
            <Label htmlFor="p-name">Name</Label>
            <Input id="p-name" value={name} onChange={(e) => setName(e.target.value)} />
          </div>
          <div className="space-y-1.5">
            <Label htmlFor="p-protocol">Protocol</Label>
            <Select value={protocol} onValueChange={(v) => setProtocol(v as ProviderProtocol)}>
              <SelectTrigger id="p-protocol">
                <SelectValue />
              </SelectTrigger>
              <SelectContent>
                {PROVIDER_PROTOCOLS.map((p) => (
                  <SelectItem key={p.value} value={p.value}>{p.label}</SelectItem>
                ))}
              </SelectContent>
            </Select>
          </div>
          <div className="space-y-1.5">
            <Label htmlFor="p-desc">Description</Label>
            <Textarea id="p-desc" value={description} onChange={(e) => setDescription(e.target.value)} rows={2} />
          </div>
          <div className="space-y-1.5">
            <Label htmlFor="p-base">Base URL</Label>
            <Input id="p-base" value={baseUrl} onChange={(e) => setBaseUrl(e.target.value)} placeholder="https://api.example.com/v1" />
          </div>
          <div className="space-y-1.5">
            <Label htmlFor="p-website">Website</Label>
            <Input id="p-website" value={website} onChange={(e) => setWebsite(e.target.value)} placeholder="https://example.com" />
          </div>
          <div className="flex items-center justify-between">
            <Label htmlFor="p-streaming">Supports streaming</Label>
            <Switch id="p-streaming" checked={streaming} onCheckedChange={setStreaming} />
          </div>
        </div>
        <DialogFooter>
          <Button variant="outline" onClick={() => onOpenChange(false)}>Cancel</Button>
          <Button onClick={handleSubmit} disabled={!name || !baseUrl}>
            {provider ? "Save" : "Create"}
          </Button>
        </DialogFooter>
      </DialogContent>
    </Dialog>
  )
}
