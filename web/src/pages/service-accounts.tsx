import { useEffect, useState } from "react"
import {
  Key, Plus, Trash2, Copy, Check,
} from "lucide-react"
import { toast } from "sonner"
import { useApi } from "@/api"
import type { ServiceAccount, CreateServiceAccountRequest, ServiceAccountCredential, Application } from "@/api/types"
import { VALID_PERMISSIONS, DEFAULT_PERMISSIONS } from "@/api/types"
import { PageHeader, EmptyState, ConfirmDialog } from "@/components"
import { MetricCardSkeleton } from "@/components/feedback/skeletons"
import { Badge } from "@/components/ui/badge"
import { Button } from "@/components/ui/button"
import { Card, CardContent } from "@/components/ui/card"
import {
  Dialog, DialogContent, DialogDescription, DialogFooter,
  DialogHeader, DialogTitle,
} from "@/components/ui/dialog"
import { Input } from "@/components/ui/input"
import { Label } from "@/components/ui/label"
import {
  Select, SelectContent, SelectItem, SelectTrigger, SelectValue,
} from "@/components/ui/select"
import {
  Table, TableBody, TableCell, TableHead, TableHeader, TableRow,
} from "@/components/ui/table"

export default function ServiceAccountsPage() {
  const api = useApi()
  const [accounts, setAccounts] = useState<ServiceAccount[]>([])
  const [applications, setApplications] = useState<Application[]>([])
  const [loading, setLoading] = useState(true)
  const [createOpen, setCreateOpen] = useState(false)
  const [revokeId, setRevokeId] = useState<string | null>(null)
  const [newCredential, setNewCredential] = useState<ServiceAccountCredential | null>(null)

  async function load() {
    const [res, apps] = await Promise.all([
      api.serviceAccounts.list(),
      api.serviceAccounts.listApplications(),
    ])
    setAccounts(res)
    setApplications(apps)
    setLoading(false)
  }

  useEffect(() => { load() }, [api])

  async function handleCreate(req: CreateServiceAccountRequest) {
    const cred = await api.serviceAccounts.create(req)
    setNewCredential(cred)
    setCreateOpen(false)
    toast.success("Service account created")
    load()
  }

  async function handleRevoke() {
    if (!revokeId) return
    await api.serviceAccounts.revoke(revokeId)
    setRevokeId(null)
    toast.success("Service account revoked")
    load()
  }

  if (loading) {
    return (
      <div className="space-y-6 p-6">
        <PageHeader title="Service Accounts" description="Manage credentials for gateway and admin API access" />
        <MetricCardSkeleton count={2} />
      </div>
    )
  }

  return (
    <div className="space-y-6 p-6">
      <PageHeader
        title="Service Accounts"
        description="Manage credentials for gateway and admin API access"
        actions={
          <Button size="sm" onClick={() => setCreateOpen(true)}>
            <Plus className="h-4 w-4 mr-1" />
            Create Service Account
          </Button>
        }
      />

      {/* Secret banner — shown once after creation */}
      {newCredential && (
        <SecretBanner credential={newCredential} onDismiss={() => setNewCredential(null)} />
      )}

      {accounts.length === 0 ? (
        <EmptyState
          icon={Key}
          title="No service accounts"
          description="Create a service account to allow applications to call the gateway or admin APIs programmatically."
          action={
            <Button size="sm" onClick={() => setCreateOpen(true)}>
              <Plus className="h-4 w-4 mr-1" />
              Create Service Account
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
                  <TableHead className="font-mono text-xs">Service</TableHead>
                  <TableHead className="font-mono text-xs">Permissions</TableHead>
                  <TableHead className="font-mono text-xs">Expires In</TableHead>
                  <TableHead className="font-mono text-xs w-8" />
                </TableRow>
              </TableHeader>
              <TableBody>
                {accounts.map((a) => (
                  <TableRow key={a.id}>
                    <TableCell className="font-medium">{a.name}</TableCell>
                    <TableCell className="text-xs text-muted-foreground">
                      {applications.find((app) => app.id === a.application_id)?.name || a.application_id}
                    </TableCell>
                    <TableCell>
                      <div className="flex flex-wrap gap-1">
                        {(a.permissions ?? []).map((p) => (
                          <Badge key={p} variant="outline" className="font-mono text-[10px]">{p}</Badge>
                        ))}
                      </div>
                    </TableCell>
                    <TableCell className="text-xs text-muted-foreground">
                      {a.expires_in || "never"}
                    </TableCell>
                    <TableCell>
                      <Button
                        variant="ghost"
                        size="icon-sm"
                        className="text-destructive"
                        onClick={() => setRevokeId(a.id)}
                      >
                        <Trash2 className="h-4 w-4" />
                      </Button>
                    </TableCell>
                  </TableRow>
                ))}
              </TableBody>
            </Table>
          </CardContent>
        </Card>
      )}

      <CreateDialog
        open={createOpen}
        onOpenChange={setCreateOpen}
        applications={applications}
        onCreate={handleCreate}
      />

      <ConfirmDialog
        open={!!revokeId}
        onOpenChange={(open) => { if (!open) setRevokeId(null) }}
        title="Revoke Service Account"
        description="This will permanently revoke the credential. Applications using it will immediately lose access."
        confirmLabel="Revoke"
        variant="destructive"
        onConfirm={handleRevoke}
      />
    </div>
  )
}

// ─── Secret Banner ──────────────────────────────────────────────────────────

function SecretBanner({ credential, onDismiss }: { credential: ServiceAccountCredential; onDismiss: () => void }) {
  const [copied, setCopied] = useState(false)

  function copy() {
    navigator.clipboard.writeText(credential.secret)
    setCopied(true)
    setTimeout(() => setCopied(false), 2000)
  }

  return (
    <div className="rounded-md border border-yellow-500/30 bg-yellow-500/5 p-4">
      <p className="text-sm font-medium mb-1">Service Account Created</p>
      <p className="text-xs text-muted-foreground mb-2">
        Copy this secret now — it won't be shown again. Use it with{" "}
        <code className="text-[10px] bg-muted px-1 rounded">POST /identity/v1/machine-token</code>{" "}
        to exchange for a short-lived JWT.
      </p>
      <div className="flex items-center gap-2">
        <code className="flex-1 text-xs font-mono bg-muted p-2 rounded break-all select-all">
          {credential.secret}
        </code>
        <Button size="sm" variant="outline" onClick={copy}>
          {copied ? <Check className="h-4 w-4" /> : <Copy className="h-4 w-4" />}
        </Button>
      </div>
      <p className="text-xs text-muted-foreground mt-2">
        Expires: {new Date(credential.expires_at).toLocaleString()}
      </p>
      <Button size="sm" variant="outline" className="mt-2" onClick={onDismiss}>
        Dismiss
      </Button>
    </div>
  )
}

// ─── Create Dialog ──────────────────────────────────────────────────────────

function CreateDialog({
  open,
  onOpenChange,
  applications,
  onCreate,
}: {
  open: boolean
  onOpenChange: (open: boolean) => void
  applications: Application[]
  onCreate: (req: CreateServiceAccountRequest) => void
}) {
  const [name, setName] = useState("")
  const [applicationId, setApplicationId] = useState("")
  const [expiresIn, setExpiresIn] = useState("")
  const [selectedPerms, setSelectedPerms] = useState<Set<string>>(new Set(DEFAULT_PERMISSIONS))

  useEffect(() => {
    if (open) {
      setName("")
      setApplicationId("")
      setExpiresIn("")
      setSelectedPerms(new Set(DEFAULT_PERMISSIONS))
    }
  }, [open])

  function toggle(perm: string) {
    setSelectedPerms((prev) => {
      const next = new Set(prev)
      if (next.has(perm)) next.delete(perm)
      else next.add(perm)
      return next
    })
  }

  function handleSubmit() {
    const req: CreateServiceAccountRequest = {
      name,
      permissions: Array.from(selectedPerms),
    }
    if (applicationId) req.application_id = applicationId
    if (expiresIn) req.expires_in = expiresIn
    onCreate(req)
  }

  return (
    <Dialog open={open} onOpenChange={onOpenChange}>
      <DialogContent className="max-w-lg">
        <DialogHeader>
          <DialogTitle>Create Service Account</DialogTitle>
          <DialogDescription>
            Create a credential for programmatic access. Select which service it belongs to and which permissions to grant.
          </DialogDescription>
        </DialogHeader>
        <div className="space-y-4">
          <div className="space-y-2">
            <Label>Name</Label>
            <Input
              value={name}
              onChange={(e) => setName(e.target.value)}
              placeholder="e.g. production-backend"
            />
          </div>

          {applications.length > 0 && (
            <div className="space-y-2">
              <Label>Service</Label>
              <Select value={applicationId} onValueChange={(v) => setApplicationId(v ?? "")}>
                <SelectTrigger className="w-full">
                  <SelectValue placeholder="Default application" />
                </SelectTrigger>
                <SelectContent>
                  {applications.map((app) => (
                    <SelectItem key={app.id} value={app.id}>{app.name}</SelectItem>
                  ))}
                </SelectContent>
              </Select>
              <p className="text-xs text-muted-foreground">
                Which IAMKit application this credential authenticates as. Leave unset to use the default.
              </p>
            </div>
          )}

          <div className="space-y-2">
            <Label>Permissions</Label>
            <div className="grid grid-cols-1 sm:grid-cols-2 gap-x-4 gap-y-1 max-h-48 overflow-y-auto border rounded-md p-2">
              {VALID_PERMISSIONS.map((p) => (
                <label
                  key={p}
                  className="flex items-start gap-2 min-w-0 cursor-pointer text-xs font-mono hover:bg-muted/50 rounded px-1 py-1"
                >
                  <input
                    type="checkbox"
                    checked={selectedPerms.has(p)}
                    onChange={() => toggle(p)}
                    className="mt-0.5 shrink-0 rounded border-muted-foreground/30"
                  />
                  <span className="break-all">{p}</span>
                </label>
              ))}
            </div>
            <p className="text-xs text-muted-foreground">
              Defaults to <code className="text-[10px] bg-muted px-1 rounded">gateway:invoke</code> if none selected.
            </p>
          </div>

          <div className="space-y-2">
            <Label>Expires In (optional)</Label>
            <Input
              value={expiresIn}
              onChange={(e) => setExpiresIn(e.target.value)}
              placeholder="e.g. 8760h (1 year), 720h (30 days)"
            />
            <p className="text-xs text-muted-foreground">
              Go duration format. Leave empty for the IAMKit default.
            </p>
          </div>
        </div>
        <DialogFooter>
          <Button variant="outline" onClick={() => onOpenChange(false)}>Cancel</Button>
          <Button onClick={handleSubmit} disabled={!name.trim()}>Create</Button>
        </DialogFooter>
      </DialogContent>
    </Dialog>
  )
}
