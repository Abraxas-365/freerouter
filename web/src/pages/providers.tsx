import { useEffect, useState } from "react"
import {
  Server, Plus, ExternalLink, MoreHorizontal,
  Pencil, Trash2, Zap, ZapOff,
} from "lucide-react"
import { useApi } from "@/api"
import type { Provider, CreateProviderRequest } from "@/api/types"
import { PageHeader, EmptyState, StatusBadge, ConfirmDialog } from "@/components"
import { MetricCardSkeleton } from "@/components/feedback/skeletons"
import { Badge } from "@/components/ui/badge"
import { Button } from "@/components/ui/button"
import { Card, CardContent, CardHeader, CardTitle } from "@/components/ui/card"
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
import {
  Table, TableBody, TableCell, TableHead, TableHeader, TableRow,
} from "@/components/ui/table"

export default function ProvidersPage() {
  const api = useApi()
  const [providers, setProviders] = useState<Provider[]>([])
  const [loading, setLoading] = useState(true)
  const [createOpen, setCreateOpen] = useState(false)
  const [editProvider, setEditProvider] = useState<Provider | null>(null)

  async function load() {
    const res = await api.providers.list()
    setProviders(res.data)
    setLoading(false)
  }

  useEffect(() => { load() }, [api])

  async function handleCreate(req: CreateProviderRequest) {
    await api.providers.create(req)
    setCreateOpen(false)
    load()
  }

  async function handleUpdate(id: string, req: Partial<CreateProviderRequest> & { status?: string }) {
    await api.providers.update(id, req)
    setEditProvider(null)
    load()
  }

  async function handleDelete(id: string) {
    await api.providers.delete(id)
    load()
  }

  async function handleToggleStatus(provider: Provider) {
    const newStatus = provider.status === "active" ? "inactive" : "active"
    await api.providers.update(provider.id, { status: newStatus })
    load()
  }

  if (loading) {
    return (
      <div className="space-y-6 p-6">
        <PageHeader title="Providers" description="Manage LLM providers connected to your gateway" />
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
        description="Manage LLM providers connected to your gateway"
        actions={
          <Button size="sm" onClick={() => setCreateOpen(true)}>
            <Plus className="h-4 w-4 mr-1" /> Add Provider
          </Button>
        }
      />

      {/* Summary */}
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
          description="Add your first LLM provider to start routing requests."
          action={
            <Button size="sm" onClick={() => setCreateOpen(true)}>
              <Plus className="h-4 w-4 mr-1" /> Add Provider
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
                          <Button variant="ghost" size="icon" className="h-8 w-8">
                            <MoreHorizontal className="h-4 w-4" />
                          </Button>
                        </DropdownMenuTrigger>
                        <DropdownMenuContent align="end">
                          <DropdownMenuItem onClick={() => setEditProvider(provider)}>
                            <Pencil className="h-4 w-4 mr-2" /> Edit
                          </DropdownMenuItem>
                          <DropdownMenuItem onClick={() => handleToggleStatus(provider)}>
                            {provider.status === "active" ? (
                              <><ZapOff className="h-4 w-4 mr-2" /> Deactivate</>
                            ) : (
                              <><Zap className="h-4 w-4 mr-2" /> Activate</>
                            )}
                          </DropdownMenuItem>
                          <DropdownMenuSeparator />
                          <DropdownMenuItem
                            className="text-destructive"
                            onClick={() => handleDelete(provider.id)}
                          >
                            <Trash2 className="h-4 w-4 mr-2" /> Delete
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

      {/* Create dialog */}
      <ProviderFormDialog
        open={createOpen}
        onOpenChange={setCreateOpen}
        title="Add Provider"
        description="Connect a new LLM provider to your gateway."
        onSubmit={handleCreate}
      />

      {/* Edit dialog */}
      {editProvider && (
        <ProviderFormDialog
          open={!!editProvider}
          onOpenChange={(open) => { if (!open) setEditProvider(null) }}
          title="Edit Provider"
          description={`Update ${editProvider.name} configuration.`}
          defaults={editProvider}
          onSubmit={(req) => handleUpdate(editProvider.id, req)}
        />
      )}
    </div>
  )
}

function ProviderFormDialog({
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
  defaults?: Provider
  onSubmit: (req: CreateProviderRequest) => void
}) {
  const [name, setName] = useState(defaults?.name ?? "")
  const [desc, setDesc] = useState(defaults?.description ?? "")
  const [website, setWebsite] = useState(defaults?.website ?? "")
  const [streaming, setStreaming] = useState(defaults?.streaming ?? true)

  useEffect(() => {
    if (open) {
      setName(defaults?.name ?? "")
      setDesc(defaults?.description ?? "")
      setWebsite(defaults?.website ?? "")
      setStreaming(defaults?.streaming ?? true)
    }
  }, [open, defaults])

  function handleSubmit(e: React.FormEvent) {
    e.preventDefault()
    onSubmit({ name, description: desc, website, streaming })
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
            <Input
              id="name"
              value={name}
              onChange={(e) => setName(e.target.value)}
              placeholder="e.g. OpenAI"
              required
            />
          </div>
          <div className="space-y-2">
            <Label htmlFor="description" className="font-mono text-xs">Description</Label>
            <Input
              id="description"
              value={desc}
              onChange={(e) => setDesc(e.target.value)}
              placeholder="Brief description"
              required
            />
          </div>
          <div className="space-y-2">
            <Label htmlFor="website" className="font-mono text-xs">Website</Label>
            <Input
              id="website"
              value={website}
              onChange={(e) => setWebsite(e.target.value)}
              placeholder="https://..."
              required
            />
          </div>
          <div className="flex items-center justify-between">
            <Label htmlFor="streaming" className="font-mono text-xs">Streaming support</Label>
            <Switch
              id="streaming"
              checked={streaming}
              onCheckedChange={setStreaming}
            />
          </div>
          <DialogFooter>
            <Button type="button" variant="outline" onClick={() => onOpenChange(false)}>
              Cancel
            </Button>
            <Button type="submit" disabled={!name.trim()}>
              {defaults ? "Save Changes" : "Add Provider"}
            </Button>
          </DialogFooter>
        </form>
      </DialogContent>
    </Dialog>
  )
}
