import { useEffect, useState } from "react"
import {
  KeyRound, Plus, Pencil, Trash2, MoreHorizontal,
  ShieldOff, ShieldCheck, Copy, Check,
} from "lucide-react"
import { useApi } from "@/api"
import type { ApiKey, CreateApiKeyRequest, CreateApiKeyResponse, Wallet } from "@/api/types"
import { PageHeader, MetricCard, StatusBadge, ConfirmDialog } from "@/components"
import { MetricCardSkeleton } from "@/components/feedback/skeletons"
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
import {
  Select, SelectContent, SelectItem, SelectTrigger, SelectValue,
} from "@/components/ui/select"
import {
  Table, TableBody, TableCell, TableHead, TableHeader, TableRow,
} from "@/components/ui/table"

export default function ApiKeysPage() {
  const api = useApi()

  const [keys, setKeys] = useState<ApiKey[]>([])
  const [wallets, setWallets] = useState<Wallet[]>([])
  const [loading, setLoading] = useState(true)

  // dialogs
  const [createOpen, setCreateOpen] = useState(false)
  const [editKey, setEditKey] = useState<ApiKey | null>(null)
  const [deleteId, setDeleteId] = useState<string | null>(null)

  // new key secret
  const [newKeyResult, setNewKeyResult] = useState<CreateApiKeyResponse | null>(null)

  async function load() {
    const [res, w] = await Promise.all([
      api.apiKeys.list(),
      api.wallets.list().catch(() => null),
    ])
    setKeys(res.data)
    setWallets(w?.wallets ?? [])
    setLoading(false)
  }

  useEffect(() => { load() }, [api])

  async function handleCreate(req: CreateApiKeyRequest) {
    const res = await api.apiKeys.create(req)
    setNewKeyResult(res)
    setCreateOpen(false)
    load()
  }

  async function handleUpdate(id: string, req: Partial<CreateApiKeyRequest> & { is_active?: boolean }) {
    await api.apiKeys.update(id, req)
    setEditKey(null)
    load()
  }

  async function handleRevoke(id: string) {
    await api.apiKeys.revoke(id)
    load()
  }

  async function handleDelete() {
    if (!deleteId) return
    await api.apiKeys.delete(deleteId)
    setDeleteId(null)
    load()
  }

  const activeCount = keys.filter((k) => k.is_active).length
  const expiredCount = keys.filter(
    (k) => k.expires_at && new Date(k.expires_at) < new Date()
  ).length

  if (loading) {
    return (
      <div className="space-y-6 p-6">
        <PageHeader title="API Keys" description="Manage access keys for the gateway" />
        <MetricCardSkeleton count={3} />
      </div>
    )
  }

  return (
    <div className="space-y-6 p-6">
      <PageHeader
        title="API Keys"
        description="Manage access keys for the gateway"
        actions={
          <Button size="sm" onClick={() => setCreateOpen(true)}>
            <Plus className="h-4 w-4 mr-1" />
            Create Key
          </Button>
        }
      />

      {/* KPIs */}
      <div className="grid grid-cols-1 sm:grid-cols-3 gap-4">
        <MetricCard label="Total Keys" value={keys.length} icon={KeyRound} />
        <MetricCard label="Active" value={activeCount} icon={ShieldCheck} />
        <MetricCard label="Expired" value={expiredCount} icon={ShieldOff} />
      </div>

      {/* New Key Banner */}
      {newKeyResult && (
        <NewKeyBanner result={newKeyResult} onDismiss={() => setNewKeyResult(null)} />
      )}

      {/* Keys Table */}
      <Card>
        <CardHeader>
          <CardTitle className="font-mono text-sm">Keys</CardTitle>
          <CardDescription>{keys.length} API keys</CardDescription>
        </CardHeader>
        <CardContent>
          <Table>
            <TableHeader>
              <TableRow>
                <TableHead className="font-mono text-xs">Name</TableHead>
                <TableHead className="font-mono text-xs">Key Prefix</TableHead>
                <TableHead className="font-mono text-xs">Scopes</TableHead>
                <TableHead className="font-mono text-xs">Models</TableHead>
                <TableHead className="font-mono text-xs">Status</TableHead>
                <TableHead className="font-mono text-xs text-right">Last Used</TableHead>
                <TableHead className="font-mono text-xs w-8" />
              </TableRow>
            </TableHeader>
            <TableBody>
              {keys.map((key) => {
                const isExpired = key.expires_at && new Date(key.expires_at) < new Date()
                return (
                  <TableRow key={key.id}>
                    <TableCell>
                      <div>
                        <p className="font-mono text-sm font-medium">{key.name}</p>
                        <p className="text-[10px] text-muted-foreground mt-0.5 max-w-[200px] truncate">
                          {key.description}
                        </p>
                      </div>
                    </TableCell>
                    <TableCell>
                      <code className="text-xs font-mono bg-muted px-1.5 py-0.5 rounded">{key.key_prefix}...</code>
                    </TableCell>
                    <TableCell>
                      <div className="flex flex-wrap gap-1">
                        {key.scopes.map((s) => (
                          <Badge key={s} variant="outline" className="font-mono text-[10px]">{s}</Badge>
                        ))}
                      </div>
                    </TableCell>
                    <TableCell>
                      {key.allowed_models.length === 0 ? (
                        <span className="text-xs text-muted-foreground">All models</span>
                      ) : (
                        <div className="flex flex-wrap gap-1">
                          {key.allowed_models.slice(0, 2).map((m) => (
                            <Badge key={m} variant="secondary" className="font-mono text-[10px]">{m}</Badge>
                          ))}
                          {key.allowed_models.length > 2 && (
                            <Badge variant="secondary" className="font-mono text-[10px]">
                              +{key.allowed_models.length - 2}
                            </Badge>
                          )}
                        </div>
                      )}
                    </TableCell>
                    <TableCell>
                      {isExpired ? (
                        <Badge variant="destructive" className="font-mono text-[10px]">expired</Badge>
                      ) : (
                        <StatusBadge status={key.is_active ? "active" : "inactive"} />
                      )}
                    </TableCell>
                    <TableCell className="text-right text-xs text-muted-foreground whitespace-nowrap">
                      {key.last_used_at ? timeAgo(key.last_used_at) : "Never"}
                    </TableCell>
                    <TableCell>
                      <DropdownMenu>
                        <DropdownMenuTrigger
                          render={
                            <Button variant="ghost" size="icon" className="h-7 w-7">
                              <MoreHorizontal className="h-4 w-4" />
                            </Button>
                          }
                        />
                        <DropdownMenuContent align="end">
                          <DropdownMenuItem onClick={() => setEditKey(key)}>
                            <Pencil className="h-4 w-4 mr-2" /> Edit
                          </DropdownMenuItem>
                          {key.is_active ? (
                            <DropdownMenuItem onClick={() => handleRevoke(key.id)}>
                              <ShieldOff className="h-4 w-4 mr-2" /> Revoke
                            </DropdownMenuItem>
                          ) : (
                            <DropdownMenuItem onClick={() => handleUpdate(key.id, { is_active: true })}>
                              <ShieldCheck className="h-4 w-4 mr-2" /> Activate
                            </DropdownMenuItem>
                          )}
                          <DropdownMenuSeparator />
                          <DropdownMenuItem className="text-destructive" onClick={() => setDeleteId(key.id)}>
                            <Trash2 className="h-4 w-4 mr-2" /> Delete
                          </DropdownMenuItem>
                        </DropdownMenuContent>
                      </DropdownMenu>
                    </TableCell>
                  </TableRow>
                )
              })}
              {keys.length === 0 && (
                <TableRow>
                  <TableCell colSpan={7} className="text-center text-sm text-muted-foreground py-8">
                    No API keys created
                  </TableCell>
                </TableRow>
              )}
            </TableBody>
          </Table>
        </CardContent>
      </Card>

      {/* Create / Edit Dialog */}
      <ApiKeyFormDialog
        open={createOpen || !!editKey}
        onOpenChange={(v) => { if (!v) { setCreateOpen(false); setEditKey(null) } }}
        apiKey={editKey}
        wallets={wallets}
        onCreate={handleCreate}
        onUpdate={(req) => editKey && handleUpdate(editKey.id, req)}
      />

      {/* Delete Confirm */}
      <ConfirmDialog
        open={!!deleteId}
        onOpenChange={(v: boolean) => { if (!v) setDeleteId(null) }}
        title="Delete API Key"
        description="This key will be permanently deleted and any services using it will lose access."
        onConfirm={handleDelete}
        variant="destructive"
      />
    </div>
  )
}

/* ------------------------------------------------------------------ */
/*  New Key Banner                                                     */
/* ------------------------------------------------------------------ */

function NewKeyBanner({ result, onDismiss }: { result: CreateApiKeyResponse; onDismiss: () => void }) {
  const [copied, setCopied] = useState(false)

  async function copyKey() {
    await navigator.clipboard.writeText(result.secret_key)
    setCopied(true)
    setTimeout(() => setCopied(false), 2000)
  }

  return (
    <div className="rounded-md border border-yellow-500/30 bg-yellow-500/5 p-4">
      <p className="text-sm font-medium mb-1">API Key Created: {result.api_key.name}</p>
      <p className="text-xs text-muted-foreground mb-2">{result.message}</p>
      <div className="flex items-center gap-2">
        <code className="text-xs font-mono bg-muted p-2 rounded break-all flex-1">{result.secret_key}</code>
        <Button size="sm" variant="outline" onClick={copyKey}>
          {copied ? <Check className="h-4 w-4" /> : <Copy className="h-4 w-4" />}
        </Button>
      </div>
      <Button size="sm" variant="outline" className="mt-2" onClick={onDismiss}>
        Dismiss
      </Button>
    </div>
  )
}

/* ------------------------------------------------------------------ */
/*  API Key Form Dialog                                                */
/* ------------------------------------------------------------------ */

function ApiKeyFormDialog({
  open,
  onOpenChange,
  apiKey,
  wallets,
  onCreate,
  onUpdate,
}: {
  open: boolean
  onOpenChange: (v: boolean) => void
  apiKey: ApiKey | null
  wallets: Wallet[]
  onCreate: (req: CreateApiKeyRequest) => void
  onUpdate: (req: Partial<CreateApiKeyRequest>) => void
}) {
  const [name, setName] = useState("")
  const [description, setDescription] = useState("")
  const [scopes, setScopes] = useState("")
  const [allowedModels, setAllowedModels] = useState("")
  const [environment, setEnvironment] = useState("test")
  const [walletId, setWalletId] = useState("none")

  useEffect(() => {
    if (open) {
      if (apiKey) {
        setName(apiKey.name)
        setDescription(apiKey.description)
        setScopes(apiKey.scopes.join(", "))
        setAllowedModels(apiKey.allowed_models.join(", "))
        setEnvironment(apiKey.key_prefix.includes("live") ? "live" : "test")
        setWalletId(apiKey.wallet_id ?? "none")
      } else {
        setName("")
        setDescription("")
        setScopes("gateway:chat")
        setAllowedModels("")
        setEnvironment("test")
        setWalletId("none")
      }
    }
  }, [open, apiKey])

  function handleSubmit() {
    const scopeList = scopes.split(",").map((s) => s.trim()).filter(Boolean)
    const modelList = allowedModels.split(",").map((m) => m.trim()).filter(Boolean)
    const wallet_id = walletId === "none" ? "" : walletId

    if (apiKey) {
      onUpdate({ name, description, scopes: scopeList, allowed_models: modelList, wallet_id })
    } else {
      onCreate({
        name,
        description,
        scopes: scopeList,
        allowed_models: modelList,
        environment,
        ...(wallet_id ? { wallet_id } : {}),
      })
    }
  }

  return (
    <Dialog open={open} onOpenChange={onOpenChange}>
      <DialogContent>
        <DialogHeader>
          <DialogTitle className="font-mono">{apiKey ? "Edit API Key" : "Create API Key"}</DialogTitle>
          <DialogDescription>
            {apiKey ? "Update the API key configuration." : "Generate a new API key for gateway access."}
          </DialogDescription>
        </DialogHeader>
        <div className="space-y-4 py-2">
          <div className="space-y-2">
            <Label className="font-mono text-xs">Name</Label>
            <Input value={name} onChange={(e) => setName(e.target.value)} placeholder="e.g. Production Key" />
          </div>
          <div className="space-y-2">
            <Label className="font-mono text-xs">Description</Label>
            <Input value={description} onChange={(e) => setDescription(e.target.value)} placeholder="e.g. Used by the main backend service" />
          </div>
          {!apiKey && (
            <div className="space-y-2">
              <Label className="font-mono text-xs">Environment</Label>
              <Select value={environment} onValueChange={(v) => v && setEnvironment(v)}>
                <SelectTrigger><SelectValue /></SelectTrigger>
                <SelectContent>
                  <SelectItem value="live">Live</SelectItem>
                  <SelectItem value="test">Test</SelectItem>
                </SelectContent>
              </Select>
            </div>
          )}
          <div className="space-y-2">
            <Label className="font-mono text-xs">Scopes (comma-separated)</Label>
            <Input value={scopes} onChange={(e) => setScopes(e.target.value)} placeholder="gateway:chat, usage:read" />
          </div>
          <div className="space-y-2">
            <Label className="font-mono text-xs">Allowed Models (comma-separated, empty = all)</Label>
            <Input value={allowedModels} onChange={(e) => setAllowedModels(e.target.value)} placeholder="gpt-4o, claude-sonnet-4" />
          </div>
          <div className="space-y-2">
            <Label className="font-mono text-xs">Wallet (usage billed to)</Label>
            <Select value={walletId} onValueChange={(v) => v && setWalletId(v)}>
              <SelectTrigger><SelectValue /></SelectTrigger>
              <SelectContent>
                <SelectItem value="none">Main balance (no wallet)</SelectItem>
                {wallets.map((w) => (
                  <SelectItem key={w.id} value={w.id}>
                    {w.name} (${w.balance.toFixed(2)})
                  </SelectItem>
                ))}
              </SelectContent>
            </Select>
            <p className="text-xs text-muted-foreground">
              When bound to a wallet, requests fail with 402 once the wallet is empty.
            </p>
          </div>
        </div>
        <DialogFooter>
          <Button variant="outline" onClick={() => onOpenChange(false)}>Cancel</Button>
          <Button onClick={handleSubmit} disabled={!name}>
            {apiKey ? "Update" : "Create"}
          </Button>
        </DialogFooter>
      </DialogContent>
    </Dialog>
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
