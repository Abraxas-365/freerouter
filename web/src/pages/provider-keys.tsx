import { useEffect, useState } from "react"
import {
  Key, Plus, MoreHorizontal, Pencil, Trash2, Zap, ZapOff, Server, Shield,
} from "lucide-react"
import { toast } from "sonner"
import { useApi } from "@/api"
import type {
  ProviderKey, CreateProviderKeyRequest, UpdateProviderKeyRequest, Provider,
  KeyType,
} from "@/api/types"
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
import {
  Table, TableBody, TableCell, TableHead, TableHeader, TableRow,
} from "@/components/ui/table"
import { fetchAllPages } from "@/lib/utils"

export default function ProviderKeysPage() {
  const api = useApi()
  const [keys, setKeys] = useState<ProviderKey[]>([])
  const [providers, setProviders] = useState<Provider[]>([])
  const [loading, setLoading] = useState(true)
  const [createOpen, setCreateOpen] = useState(false)
  const [editKey, setEditKey] = useState<ProviderKey | null>(null)
  const [deleteId, setDeleteId] = useState<string | null>(null)

  async function load() {
    const [keyItems, provRes] = await Promise.all([
      fetchAllPages((p) => api.providerKeys.list(p)),
      api.providers.list({ limit: 100 }),
    ])
    setKeys(keyItems)
    setProviders(provRes.items)
    setLoading(false)
  }

  useEffect(() => { load() }, [api])

  const providerMap = Object.fromEntries(providers.map((p) => [p.id, p]))

  async function handleCreate(req: CreateProviderKeyRequest) {
    await api.providerKeys.create(req)
    setCreateOpen(false)
    toast.success("Provider key added")
    load()
  }

  async function handleUpdate(id: string, req: UpdateProviderKeyRequest) {
    await api.providerKeys.update(id, req)
    setEditKey(null)
    toast.success("Provider key updated")
    load()
  }

  async function handleDelete() {
    if (!deleteId) return
    await api.providerKeys.delete(deleteId)
    setDeleteId(null)
    toast.success("Provider key deleted")
    load()
  }

  async function handleToggleStatus(key: ProviderKey) {
    await api.providerKeys.update(key.id, {
      status: key.status === "active" ? "inactive" : "active",
    })
    load()
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
            <Plus className="h-4 w-4" /> Add Key
          </Button>
        }
      />

      {/* Summary */}
      <div className="flex items-center gap-4">
        <Badge variant="outline" className="font-mono text-xs gap-1.5">
          <span className="h-1.5 w-1.5 rounded-full bg-success" />
          {active} active
        </Badge>
        <span className="text-xs text-muted-foreground font-mono">{keys.length} total</span>
      </div>

      {keys.length === 0 ? (
        <EmptyState
          icon={Key}
          title="No provider keys"
          description="Add API keys to connect your LLM providers."
          action={
            <Button size="sm" onClick={() => setCreateOpen(true)}>
              <Plus className="h-4 w-4" /> Add Key
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
                        <TableHead className="font-mono text-xs">Type</TableHead>
                        <TableHead className="font-mono text-xs">Token</TableHead>
                        <TableHead className="font-mono text-xs">Base URL</TableHead>
                        <TableHead className="font-mono text-xs">Status</TableHead>
                        <TableHead className="font-mono text-xs w-[50px]" />
                      </TableRow>
                    </TableHeader>
                    <TableBody>
                      {provKeys.map((key) => (
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
                            <Badge variant={key.key_type === "oauth" ? "default" : "outline"} className="font-mono text-[10px] gap-1">
                              {key.key_type === "oauth" && <Shield className="h-3 w-3" />}
                              {key.key_type === "oauth" ? "OAuth" : "API Key"}
                            </Badge>
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
                            <DropdownMenu>
                              <DropdownMenuTrigger>
                                <Button variant="ghost" size="icon-sm">
                                  <MoreHorizontal className="h-4 w-4" />
                                </Button>
                              </DropdownMenuTrigger>
                              <DropdownMenuContent align="end">
                                <DropdownMenuItem onClick={() => setEditKey(key)}>
                                  <Pencil className="h-3.5 w-3.5" /> Edit
                                </DropdownMenuItem>
                                <DropdownMenuItem onClick={() => handleToggleStatus(key)}>
                                  {key.status === "active" ? (
                                    <><ZapOff className="h-3.5 w-3.5" /> Deactivate</>
                                  ) : (
                                    <><Zap className="h-3.5 w-3.5" /> Activate</>
                                  )}
                                </DropdownMenuItem>
                                <DropdownMenuSeparator />
                                <DropdownMenuItem
                                  variant="destructive"
                                  onClick={() => setDeleteId(key.id)}
                                >
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

      <ConfirmDialog
        open={!!deleteId}
        onOpenChange={(o) => !o && setDeleteId(null)}
        title="Delete provider key"
        description="This will permanently remove the key. Requests routed through it will fail until another key is available."
        confirmLabel="Delete"
        onConfirm={handleDelete}
      />
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
  const [keyType, setKeyType] = useState<KeyType>("api_key")
  const [name, setName] = useState("")
  const [description, setDescription] = useState("")
  const [token, setToken] = useState("")
  const [accessToken, setAccessToken] = useState("")
  const [refreshToken, setRefreshToken] = useState("")
  const [expiresAt, setExpiresAt] = useState("")
  const [baseUrl, setBaseUrl] = useState("")

  useEffect(() => {
    if (open) {
      setProviderId("")
      setKeyType("api_key")
      setName("")
      setDescription("")
      setToken("")
      setAccessToken("")
      setRefreshToken("")
      setExpiresAt("")
      setBaseUrl("")
    }
  }, [open])

  const isValid = keyType === "api_key"
    ? !!(providerId && name.trim() && token.trim())
    : !!(providerId && name.trim() && accessToken.trim() && refreshToken.trim() && expiresAt)

  function handleSubmit(e: React.FormEvent) {
    e.preventDefault()
    const req: CreateProviderKeyRequest = {
      provider_id: providerId,
      key_type: keyType,
      name,
      description,
      ...(baseUrl ? { base_url: baseUrl } : {}),
    }
    if (keyType === "api_key") {
      req.token = token
    } else {
      req.oauth_data = {
        access_token: accessToken,
        refresh_token: refreshToken,
        expires_at: new Date(expiresAt).toISOString(),
      }
    }
    onSubmit(req)
  }

  return (
    <Dialog open={open} onOpenChange={onOpenChange}>
      <DialogContent>
        <DialogHeader>
          <DialogTitle className="font-mono">Add Provider Key</DialogTitle>
          <DialogDescription>Add an API key or OAuth credential for a provider.</DialogDescription>
        </DialogHeader>
        <form onSubmit={handleSubmit} className="space-y-4">
          <div className="space-y-2">
            <Label className="font-mono text-xs">Provider</Label>
            <Select value={providerId} onValueChange={(v) => v && setProviderId(v)}>
              <SelectTrigger>
                <SelectValue placeholder="Select provider">
                  {(value: string) => providers.find((p) => p.id === value)?.name ?? value}
                </SelectValue>
              </SelectTrigger>
              <SelectContent>
                {providers.map((p) => (
                  <SelectItem key={p.id} value={p.id}>{p.name}</SelectItem>
                ))}
              </SelectContent>
            </Select>
          </div>
          <div className="space-y-2">
            <Label className="font-mono text-xs">Key Type</Label>
            <Select value={keyType} onValueChange={(v) => setKeyType(v as KeyType)}>
              <SelectTrigger>
                <SelectValue />
              </SelectTrigger>
              <SelectContent>
                <SelectItem value="api_key">API Key</SelectItem>
                <SelectItem value="oauth">OAuth Token</SelectItem>
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

          {keyType === "api_key" ? (
            <div className="space-y-2">
              <Label htmlFor="key-token" className="font-mono text-xs">API Token</Label>
              <Input id="key-token" type="password" value={token} onChange={(e) => setToken(e.target.value)} placeholder="sk-..." required />
            </div>
          ) : (
            <>
              <div className="space-y-2">
                <Label htmlFor="key-access" className="font-mono text-xs">Access Token</Label>
                <Input id="key-access" type="password" value={accessToken} onChange={(e) => setAccessToken(e.target.value)} placeholder="sk-ant-oat01-..." required />
              </div>
              <div className="space-y-2">
                <Label htmlFor="key-refresh" className="font-mono text-xs">Refresh Token</Label>
                <Input id="key-refresh" type="password" value={refreshToken} onChange={(e) => setRefreshToken(e.target.value)} placeholder="sk-ant-ort01-..." required />
              </div>
              <div className="space-y-2">
                <Label htmlFor="key-expires" className="font-mono text-xs">Expires At</Label>
                <Input id="key-expires" type="datetime-local" value={expiresAt} onChange={(e) => setExpiresAt(e.target.value)} required />
              </div>
            </>
          )}

          <div className="space-y-2">
            <Label htmlFor="key-url" className="font-mono text-xs">Base URL (optional)</Label>
            <Input id="key-url" value={baseUrl} onChange={(e) => setBaseUrl(e.target.value)} placeholder="https://api.openai.com/v1" />
          </div>
          <DialogFooter>
            <Button type="button" variant="outline" onClick={() => onOpenChange(false)}>Cancel</Button>
            <Button type="submit" disabled={!isValid}>Add Key</Button>
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
  onSubmit: (req: UpdateProviderKeyRequest) => void
}) {
  const [name, setName] = useState(keyData.name)
  const [description, setDescription] = useState(keyData.description)
  const [baseUrl, setBaseUrl] = useState(keyData.base_url ?? "")
  const [token, setToken] = useState("")
  const [accessToken, setAccessToken] = useState("")
  const [refreshToken, setRefreshToken] = useState("")
  const [expiresAt, setExpiresAt] = useState("")

  const isOAuth = keyData.key_type === "oauth"

  useEffect(() => {
    if (open) {
      setName(keyData.name)
      setDescription(keyData.description)
      setBaseUrl(keyData.base_url ?? "")
      setToken("")
      setAccessToken("")
      setRefreshToken("")
      setExpiresAt("")
    }
  }, [open, keyData])

  function handleSubmit(e: React.FormEvent) {
    e.preventDefault()
    const req: UpdateProviderKeyRequest = {
      name,
      description,
      base_url: baseUrl || undefined,
    }
    if (isOAuth && accessToken && refreshToken && expiresAt) {
      req.oauth_data = {
        access_token: accessToken,
        refresh_token: refreshToken,
        expires_at: new Date(expiresAt).toISOString(),
      }
    } else if (!isOAuth && token) {
      req.token = token
    }
    onSubmit(req)
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

          {isOAuth ? (
            <>
              <p className="text-xs text-muted-foreground font-mono">Replace OAuth tokens (leave all blank to keep current)</p>
              <div className="space-y-2">
                <Label htmlFor="edit-access" className="font-mono text-xs">Access Token</Label>
                <Input id="edit-access" type="password" value={accessToken} onChange={(e) => setAccessToken(e.target.value)} placeholder="Leave blank to keep current" />
              </div>
              <div className="space-y-2">
                <Label htmlFor="edit-refresh" className="font-mono text-xs">Refresh Token</Label>
                <Input id="edit-refresh" type="password" value={refreshToken} onChange={(e) => setRefreshToken(e.target.value)} placeholder="Leave blank to keep current" />
              </div>
              <div className="space-y-2">
                <Label htmlFor="edit-expires" className="font-mono text-xs">Expires At</Label>
                <Input id="edit-expires" type="datetime-local" value={expiresAt} onChange={(e) => setExpiresAt(e.target.value)} />
              </div>
            </>
          ) : (
            <div className="space-y-2">
              <Label htmlFor="edit-token" className="font-mono text-xs">Replace token (optional)</Label>
              <Input id="edit-token" type="password" value={token} onChange={(e) => setToken(e.target.value)} placeholder="Leave blank to keep current token" />
            </div>
          )}

          <DialogFooter>
            <Button type="button" variant="outline" onClick={() => onOpenChange(false)}>Cancel</Button>
            <Button type="submit" disabled={!name.trim()}>Save Changes</Button>
          </DialogFooter>
        </form>
      </DialogContent>
    </Dialog>
  )
}
