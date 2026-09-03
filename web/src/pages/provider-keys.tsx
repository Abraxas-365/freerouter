import { useEffect, useState } from "react"
import {
  Key, Plus, MoreHorizontal, Pencil, Trash2, Zap, ZapOff,
  FlaskConical, Check, X, Loader2, Server,
} from "lucide-react"
import { useApi } from "@/api"
import type {
  ProviderKey, CreateProviderKeyRequest, UpdateProviderKeyRequest, Provider, TestKeyResult,
} from "@/api/types"
import { PageHeader, EmptyState, StatusBadge } from "@/components"
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
import {
  Table, TableBody, TableCell, TableHead, TableHeader, TableRow,
} from "@/components/ui/table"

export default function ProviderKeysPage() {
  const api = useApi()
  const [keys, setKeys] = useState<ProviderKey[]>([])
  const [providers, setProviders] = useState<Provider[]>([])
  const [loading, setLoading] = useState(true)
  const [createOpen, setCreateOpen] = useState(false)
  const [editKey, setEditKey] = useState<ProviderKey | null>(null)
  const [testResults, setTestResults] = useState<Record<string, TestKeyResult | "loading">>({})

  async function load() {
    const provRes = await api.providers.list()
    setProviders(provRes.data)
    // load keys from all providers
    const allKeys = await Promise.all(
      provRes.data.map((p) => api.providerKeys.listByProvider(p.id))
    )
    setKeys(allKeys.flatMap((r) => r.data))
    setLoading(false)
  }

  useEffect(() => { load() }, [api])

  const providerMap = Object.fromEntries(providers.map((p) => [p.id, p]))

  async function handleCreate(req: CreateProviderKeyRequest) {
    await api.providerKeys.create(req)
    setCreateOpen(false)
    load()
  }

  async function handleUpdate(id: string, req: UpdateProviderKeyRequest) {
    await api.providerKeys.update(id, req)
    setEditKey(null)
    load()
  }

  async function handleDelete(id: string) {
    await api.providerKeys.delete(id)
    load()
  }

  async function handleToggleStatus(key: ProviderKey) {
    await api.providerKeys.update(key.id, {
      status: key.status === "active" ? "inactive" : "active",
    })
    load()
  }

  async function handleTest(id: string) {
    setTestResults((prev) => ({ ...prev, [id]: "loading" }))
    const result = await api.providerKeys.test(id)
    setTestResults((prev) => ({ ...prev, [id]: result }))
  }

  if (loading) {
    return (
      <div className="space-y-6 p-6">
        <PageHeader title="Provider Keys" description="API keys for connecting to LLM providers" />
        <MetricCardSkeleton count={3} />
      </div>
    )
  }

  const active = keys.filter((k) => k.status === "active").length
  const managed = keys.filter((k) => k.managed).length

  // group by provider
  const byProvider = new Map<string, ProviderKey[]>()
  for (const k of keys) {
    const list = byProvider.get(k.provider_id) ?? []
    list.push(k)
    byProvider.set(k.provider_id, list)
  }

  return (
    <div className="space-y-6 p-6">
      <PageHeader
        title="Provider Keys"
        description="API keys for connecting to LLM providers"
        actions={
          <Button size="sm" onClick={() => setCreateOpen(true)}>
            <Plus className="h-4 w-4 mr-1" /> Add Key
          </Button>
        }
      />

      {/* Summary */}
      <div className="flex items-center gap-4">
        <Badge variant="outline" className="font-mono text-xs gap-1.5">
          <span className="h-1.5 w-1.5 rounded-full bg-success" />
          {active} active
        </Badge>
        {managed > 0 && (
          <Badge variant="outline" className="font-mono text-xs gap-1.5">
            <Key className="h-3 w-3" />
            {managed} managed
          </Badge>
        )}
        <span className="text-xs text-muted-foreground font-mono">{keys.length} total</span>
      </div>

      {keys.length === 0 ? (
        <EmptyState
          icon={Key}
          title="No provider keys"
          description="Add API keys to connect your LLM providers."
          action={
            <Button size="sm" onClick={() => setCreateOpen(true)}>
              <Plus className="h-4 w-4 mr-1" /> Add Key
            </Button>
          }
        />
      ) : (
        <div className="space-y-6">
          {[...byProvider.entries()].map(([providerId, provKeys]) => {
            const provider = providerMap[providerId]
            return (
              <Card key={providerId}>
                <div className="flex items-center gap-2 px-4 pt-4 pb-2">
                  <Server className="h-4 w-4 text-muted-foreground" />
                  <span className="font-mono text-sm font-semibold">
                    {provider?.name ?? providerId}
                  </span>
                  <span className="text-xs text-muted-foreground font-mono">
                    ({provKeys.length} key{provKeys.length !== 1 ? "s" : ""})
                  </span>
                </div>
                <CardContent className="p-0">
                  <Table>
                    <TableHeader>
                      <TableRow>
                        <TableHead className="font-mono text-xs">Name</TableHead>
                        <TableHead className="font-mono text-xs">Token</TableHead>
                        <TableHead className="font-mono text-xs">Base URL</TableHead>
                        <TableHead className="font-mono text-xs">Status</TableHead>
                        <TableHead className="font-mono text-xs">Type</TableHead>
                        <TableHead className="font-mono text-xs">Test</TableHead>
                        <TableHead className="font-mono text-xs w-[50px]" />
                      </TableRow>
                    </TableHeader>
                    <TableBody>
                      {provKeys.map((key) => {
                        const test = testResults[key.id]
                        return (
                          <TableRow key={key.id}>
                            <TableCell>
                              <span className="font-mono text-sm font-medium">{key.name}</span>
                              {key.description && (
                                <p className="text-xs text-muted-foreground mt-0.5 max-w-[200px] truncate">
                                  {key.description}
                                </p>
                              )}
                            </TableCell>
                            <TableCell>
                              <code className="text-xs font-mono bg-muted px-1.5 py-0.5 rounded">
                                {key.token_masked}
                              </code>
                            </TableCell>
                            <TableCell className="font-mono text-xs text-muted-foreground">
                              {key.base_url ?? <span className="text-muted-foreground/50">default</span>}
                            </TableCell>
                            <TableCell>
                              <StatusBadge
                                status={key.status === "active" ? "online" : "offline"}
                                label={key.status.toUpperCase()}
                              />
                            </TableCell>
                            <TableCell>
                              <Badge variant="secondary" className="font-mono text-[10px]">
                                {key.managed ? "managed" : "tenant"}
                              </Badge>
                            </TableCell>
                            <TableCell>
                              {test === "loading" ? (
                                <Loader2 className="h-4 w-4 animate-spin text-muted-foreground" />
                              ) : test ? (
                                <div className="flex items-center gap-1.5">
                                  {test.success ? (
                                    <Check className="h-4 w-4 text-success" />
                                  ) : (
                                    <X className="h-4 w-4 text-destructive" />
                                  )}
                                  <span className="font-mono text-[10px] text-muted-foreground tabular-nums">
                                    {test.latency_ms}ms
                                  </span>
                                </div>
                              ) : (
                                <Button
                                  variant="ghost"
                                  size="icon"
                                  className="h-7 w-7"
                                  onClick={() => handleTest(key.id)}
                                >
                                  <FlaskConical className="h-3.5 w-3.5" />
                                </Button>
                              )}
                            </TableCell>
                            <TableCell>
                              <DropdownMenu>
                                <DropdownMenuTrigger>
                                  <Button variant="ghost" size="icon" className="h-8 w-8">
                                    <MoreHorizontal className="h-4 w-4" />
                                  </Button>
                                </DropdownMenuTrigger>
                                <DropdownMenuContent align="end">
                                  <DropdownMenuItem onClick={() => setEditKey(key)}>
                                    <Pencil className="h-4 w-4 mr-2" /> Edit
                                  </DropdownMenuItem>
                                  <DropdownMenuItem onClick={() => handleTest(key.id)}>
                                    <FlaskConical className="h-4 w-4 mr-2" /> Test
                                  </DropdownMenuItem>
                                  <DropdownMenuItem onClick={() => handleToggleStatus(key)}>
                                    {key.status === "active" ? (
                                      <><ZapOff className="h-4 w-4 mr-2" /> Deactivate</>
                                    ) : (
                                      <><Zap className="h-4 w-4 mr-2" /> Activate</>
                                    )}
                                  </DropdownMenuItem>
                                  <DropdownMenuSeparator />
                                  <DropdownMenuItem
                                    className="text-destructive"
                                    onClick={() => handleDelete(key.id)}
                                  >
                                    <Trash2 className="h-4 w-4 mr-2" /> Delete
                                  </DropdownMenuItem>
                                </DropdownMenuContent>
                              </DropdownMenu>
                            </TableCell>
                          </TableRow>
                        )
                      })}
                    </TableBody>
                  </Table>
                </CardContent>
              </Card>
            )
          })}
        </div>
      )}

      {/* Create dialog */}
      <KeyFormDialog
        open={createOpen}
        onOpenChange={setCreateOpen}
        providers={providers}
        onSubmit={handleCreate}
      />

      {/* Edit dialog */}
      {editKey && (
        <KeyEditDialog
          open={!!editKey}
          onOpenChange={(open) => { if (!open) setEditKey(null) }}
          keyData={editKey}
          onSubmit={(req) => handleUpdate(editKey.id, req)}
        />
      )}
    </div>
  )
}

function KeyFormDialog({
  open,
  onOpenChange,
  providers,
  onSubmit,
}: {
  open: boolean
  onOpenChange: (open: boolean) => void
  providers: Provider[]
  onSubmit: (req: CreateProviderKeyRequest) => void
}) {
  const [providerId, setProviderId] = useState("")
  const [name, setName] = useState("")
  const [description, setDescription] = useState("")
  const [token, setToken] = useState("")
  const [baseUrl, setBaseUrl] = useState("")

  useEffect(() => {
    if (open) {
      setProviderId("")
      setName("")
      setDescription("")
      setToken("")
      setBaseUrl("")
    }
  }, [open])

  function handleSubmit(e: React.FormEvent) {
    e.preventDefault()
    onSubmit({
      provider_id: providerId,
      name,
      description,
      token,
      ...(baseUrl ? { base_url: baseUrl } : {}),
    })
  }

  return (
    <Dialog open={open} onOpenChange={onOpenChange}>
      <DialogContent>
        <DialogHeader>
          <DialogTitle className="font-mono">Add Provider Key</DialogTitle>
          <DialogDescription>Add an API key for a provider.</DialogDescription>
        </DialogHeader>
        <form onSubmit={handleSubmit} className="space-y-4">
          <div className="space-y-2">
            <Label className="font-mono text-xs">Provider</Label>
            <Select value={providerId} onValueChange={(v) => v && setProviderId(v)}>
              <SelectTrigger><SelectValue placeholder="Select provider" /></SelectTrigger>
              <SelectContent>
                {providers.map((p) => (
                  <SelectItem key={p.id} value={p.id}>{p.name}</SelectItem>
                ))}
              </SelectContent>
            </Select>
          </div>
          <div className="space-y-2">
            <Label htmlFor="key-name" className="font-mono text-xs">Name</Label>
            <Input id="key-name" value={name} onChange={(e) => setName(e.target.value)} placeholder="e.g. Production Key" required />
          </div>
          <div className="space-y-2">
            <Label htmlFor="key-desc" className="font-mono text-xs">Description</Label>
            <Input id="key-desc" value={description} onChange={(e) => setDescription(e.target.value)} placeholder="Optional description" />
          </div>
          <div className="space-y-2">
            <Label htmlFor="key-token" className="font-mono text-xs">API Token</Label>
            <Input id="key-token" type="password" value={token} onChange={(e) => setToken(e.target.value)} placeholder="sk-..." required />
          </div>
          <div className="space-y-2">
            <Label htmlFor="key-url" className="font-mono text-xs">Base URL (optional)</Label>
            <Input id="key-url" value={baseUrl} onChange={(e) => setBaseUrl(e.target.value)} placeholder="https://api.openai.com/v1" />
          </div>
          <DialogFooter>
            <Button type="button" variant="outline" onClick={() => onOpenChange(false)}>Cancel</Button>
            <Button type="submit" disabled={!providerId || !name.trim() || !token.trim()}>Add Key</Button>
          </DialogFooter>
        </form>
      </DialogContent>
    </Dialog>
  )
}

function KeyEditDialog({
  open,
  onOpenChange,
  keyData,
  onSubmit,
}: {
  open: boolean
  onOpenChange: (open: boolean) => void
  keyData: ProviderKey
  onSubmit: (req: { name?: string; description?: string; base_url?: string }) => void
}) {
  const [name, setName] = useState(keyData.name)
  const [description, setDescription] = useState(keyData.description)
  const [baseUrl, setBaseUrl] = useState(keyData.base_url ?? "")

  useEffect(() => {
    if (open) {
      setName(keyData.name)
      setDescription(keyData.description)
      setBaseUrl(keyData.base_url ?? "")
    }
  }, [open, keyData])

  function handleSubmit(e: React.FormEvent) {
    e.preventDefault()
    onSubmit({ name, description, base_url: baseUrl || undefined })
  }

  return (
    <Dialog open={open} onOpenChange={onOpenChange}>
      <DialogContent>
        <DialogHeader>
          <DialogTitle className="font-mono">Edit Key</DialogTitle>
          <DialogDescription>Update key "{keyData.name}"</DialogDescription>
        </DialogHeader>
        <form onSubmit={handleSubmit} className="space-y-4">
          <div className="space-y-2">
            <Label htmlFor="edit-name" className="font-mono text-xs">Name</Label>
            <Input id="edit-name" value={name} onChange={(e) => setName(e.target.value)} required />
          </div>
          <div className="space-y-2">
            <Label htmlFor="edit-desc" className="font-mono text-xs">Description</Label>
            <Input id="edit-desc" value={description} onChange={(e) => setDescription(e.target.value)} />
          </div>
          <div className="space-y-2">
            <Label htmlFor="edit-url" className="font-mono text-xs">Base URL</Label>
            <Input id="edit-url" value={baseUrl} onChange={(e) => setBaseUrl(e.target.value)} placeholder="https://..." />
          </div>
          <DialogFooter>
            <Button type="button" variant="outline" onClick={() => onOpenChange(false)}>Cancel</Button>
            <Button type="submit" disabled={!name.trim()}>Save Changes</Button>
          </DialogFooter>
        </form>
      </DialogContent>
    </Dialog>
  )
}

