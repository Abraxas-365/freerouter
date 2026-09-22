import { useEffect, useState } from "react"
import { Gauge, Plus, Pencil, Trash2, MoreHorizontal } from "lucide-react"
import { toast } from "sonner"
import { useApi } from "@/api"
import type {
  RateLimitConfig, CreateRateLimitRequest, UpdateRateLimitRequest,
  AccessUser, ServiceAccount,
} from "@/api/types"
import { PageHeader, EmptyState, ConfirmDialog } from "@/components"
import { MetricCardSkeleton } from "@/components/feedback/skeletons"
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
  Select, SelectContent, SelectGroup, SelectItem, SelectLabel, SelectTrigger, SelectValue,
} from "@/components/ui/select"
import {
  Table, TableBody, TableCell, TableHead, TableHeader, TableRow,
} from "@/components/ui/table"

const CUSTOM_SUBJECT = "__custom__"

export default function RateLimitsPage() {
  const api = useApi()
  const [configs, setConfigs] = useState<RateLimitConfig[]>([])
  const [users, setUsers] = useState<AccessUser[]>([])
  const [serviceAccounts, setServiceAccounts] = useState<ServiceAccount[]>([])
  const [loading, setLoading] = useState(true)

  const [createOpen, setCreateOpen] = useState(false)
  const [editConfig, setEditConfig] = useState<RateLimitConfig | null>(null)
  const [deleteId, setDeleteId] = useState<string | null>(null)

  async function load() {
    const [res, u, sa] = await Promise.all([
      api.rateLimits.list({ limit: 100 }),
      api.access.listUsers().catch(() => []),
      api.serviceAccounts.list().catch(() => []),
    ])
    setConfigs(res.items)
    setUsers(u)
    setServiceAccounts(sa)
    setLoading(false)
  }

  useEffect(() => { load() }, [api])

  const userMap = Object.fromEntries(users.map((u) => [u.id, u]))
  const serviceAccountMap = Object.fromEntries(serviceAccounts.map((a) => [a.id, a]))

  function subjectLabel(subjectId: string) {
    if (subjectId === "default") return "default"
    return userMap[subjectId]?.email ?? serviceAccountMap[subjectId]?.name ?? subjectId
  }

  async function handleCreate(req: CreateRateLimitRequest) {
    await api.rateLimits.create(req)
    setCreateOpen(false)
    toast.success("Rate limit created")
    load()
  }

  async function handleUpdate(id: string, req: UpdateRateLimitRequest) {
    await api.rateLimits.update(id, req)
    setEditConfig(null)
    toast.success("Rate limit updated")
    load()
  }

  async function handleDelete() {
    if (!deleteId) return
    await api.rateLimits.delete(deleteId)
    setDeleteId(null)
    toast.success("Rate limit deleted")
    load()
  }

  if (loading) {
    return (
      <div className="space-y-6 p-6">
        <PageHeader title="Rate Limits" description="Per-subject request rate and concurrency limits" />
        <MetricCardSkeleton count={3} />
      </div>
    )
  }

  return (
    <div className="space-y-6 p-6">
      <PageHeader
        title="Rate Limits"
        description="Per-subject request rate and concurrency limits"
        actions={
          <Button size="sm" onClick={() => setCreateOpen(true)}>
            <Plus className="h-4 w-4" /> New limit
          </Button>
        }
      />

      <p className="text-xs text-muted-foreground font-mono">
        Configs match by <code>subject_id</code> (the JWT subject making the request). Use{" "}
        <code>default</code> as the subject to set a fallback that applies to everyone else.
      </p>

      {configs.length === 0 ? (
        <EmptyState
          icon={Gauge}
          title="No rate limits configured"
          description="Requests are unrestricted until you add a rate limit config."
          action={
            <Button size="sm" onClick={() => setCreateOpen(true)}>
              <Plus className="h-4 w-4" /> New limit
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
                  <TableHead className="font-mono text-xs">Subject</TableHead>
                  <TableHead className="font-mono text-xs">RPM</TableHead>
                  <TableHead className="font-mono text-xs">Max concurrent</TableHead>
                  <TableHead className="font-mono text-xs">Updated</TableHead>
                  <TableHead className="font-mono text-xs w-[50px]" />
                </TableRow>
              </TableHeader>
              <TableBody>
                {configs.map((cfg) => (
                  <TableRow key={cfg.id}>
                    <TableCell className="font-mono text-sm font-medium">{cfg.name}</TableCell>
                    <TableCell>
                      <code className="text-xs font-mono bg-muted px-1.5 py-0.5 rounded">
                        {subjectLabel(cfg.subject_id)}
                      </code>
                    </TableCell>
                    <TableCell className="font-mono text-xs tabular-nums">
                      {cfg.rpm === 0 ? "unlimited" : cfg.rpm}
                    </TableCell>
                    <TableCell className="font-mono text-xs tabular-nums">
                      {cfg.max_concurrent === 0 ? "unlimited" : cfg.max_concurrent}
                    </TableCell>
                    <TableCell className="font-mono text-xs text-muted-foreground tabular-nums">
                      {new Date(cfg.updated_at).toLocaleDateString()}
                    </TableCell>
                    <TableCell>
                      <DropdownMenu>
                        <DropdownMenuTrigger>
                          <Button variant="ghost" size="icon-sm">
                            <MoreHorizontal className="h-4 w-4" />
                          </Button>
                        </DropdownMenuTrigger>
                        <DropdownMenuContent align="end">
                          <DropdownMenuItem onClick={() => setEditConfig(cfg)}>
                            <Pencil className="h-3.5 w-3.5" /> Edit
                          </DropdownMenuItem>
                          <DropdownMenuSeparator />
                          <DropdownMenuItem variant="destructive" onClick={() => setDeleteId(cfg.id)}>
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

      <RateLimitDialog
        open={createOpen}
        onOpenChange={setCreateOpen}
        onSubmit={handleCreate}
        users={users}
        serviceAccounts={serviceAccounts}
      />
      <RateLimitDialog
        config={editConfig ?? undefined}
        open={!!editConfig}
        onOpenChange={(o) => !o && setEditConfig(null)}
        onSubmit={(req) => editConfig && handleUpdate(editConfig.id, req)}
        users={users}
        serviceAccounts={serviceAccounts}
      />

      <ConfirmDialog
        open={!!deleteId}
        onOpenChange={(o) => !o && setDeleteId(null)}
        title="Delete rate limit"
        description="This will remove the rate limit config. The subject will fall back to the default limit (or be unrestricted if none exists)."
        confirmLabel="Delete"
        onConfirm={handleDelete}
      />
    </div>
  )
}

function RateLimitDialog({
  config,
  open,
  onOpenChange,
  onSubmit,
  users,
  serviceAccounts,
}: {
  config?: RateLimitConfig
  open: boolean
  onOpenChange: (open: boolean) => void
  onSubmit: (req: CreateRateLimitRequest) => void
  users: AccessUser[]
  serviceAccounts: ServiceAccount[]
}) {
  const [name, setName] = useState("")
  const [subjectId, setSubjectId] = useState("")
  const [customSubjectId, setCustomSubjectId] = useState("")
  const [rpm, setRpm] = useState("60")
  const [maxConcurrent, setMaxConcurrent] = useState("10")

  const knownSubjectIds = new Set([
    "default",
    ...users.map((u) => u.id),
    ...serviceAccounts.map((a) => a.id),
  ])

  useEffect(() => {
    if (open) {
      const existing = config?.subject_id ?? ""
      setName(config?.name ?? "")
      if (existing && !knownSubjectIds.has(existing)) {
        setSubjectId(CUSTOM_SUBJECT)
        setCustomSubjectId(existing)
      } else {
        setSubjectId(existing)
        setCustomSubjectId("")
      }
      setRpm(String(config?.rpm ?? 60))
      setMaxConcurrent(String(config?.max_concurrent ?? 10))
    }
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [open, config])

  const effectiveSubjectId = subjectId === CUSTOM_SUBJECT ? customSubjectId.trim() : subjectId

  function handleSubmit() {
    onSubmit({
      name,
      subject_id: effectiveSubjectId,
      rpm: Number(rpm) || 0,
      max_concurrent: Number(maxConcurrent) || 0,
    })
  }

  return (
    <Dialog open={open} onOpenChange={onOpenChange}>
      <DialogContent>
        <DialogHeader>
          <DialogTitle className="font-mono">{config ? "Edit rate limit" : "New rate limit"}</DialogTitle>
          <DialogDescription>
            {config
              ? "Update the rate limit configuration."
              : "Limit requests-per-minute and concurrency for a JWT subject. Use 0 for unlimited."}
          </DialogDescription>
        </DialogHeader>
        <div className="space-y-4">
          <div className="space-y-1.5">
            <Label htmlFor="rl-name">Name</Label>
            <Input id="rl-name" value={name} onChange={(e) => setName(e.target.value)} />
          </div>
          <div className="space-y-1.5">
            <Label>Subject</Label>
            <Select
              value={subjectId}
              onValueChange={(v) => v && setSubjectId(v)}
              disabled={!!config}
            >
              <SelectTrigger className="w-full">
                <SelectValue placeholder="Select a subject">
                  {(value: string) => {
                    if (value === CUSTOM_SUBJECT) return "Custom subject ID"
                    if (value === "default") return "default (fallback for everyone else)"
                    return users.find((u) => u.id === value)?.email
                      ?? serviceAccounts.find((a) => a.id === value)?.name
                      ?? value
                  }}
                </SelectValue>
              </SelectTrigger>
              <SelectContent>
                <SelectItem value="default">default (fallback for everyone else)</SelectItem>
                {users.length > 0 && (
                  <SelectGroup>
                    <SelectLabel>Users</SelectLabel>
                    {users.map((u) => (
                      <SelectItem key={u.id} value={u.id}>{u.email}</SelectItem>
                    ))}
                  </SelectGroup>
                )}
                {serviceAccounts.length > 0 && (
                  <SelectGroup>
                    <SelectLabel>Service Accounts</SelectLabel>
                    {serviceAccounts.map((a) => (
                      <SelectItem key={a.id} value={a.id}>{a.name}</SelectItem>
                    ))}
                  </SelectGroup>
                )}
                <SelectItem value={CUSTOM_SUBJECT}>Custom subject ID…</SelectItem>
              </SelectContent>
            </Select>
            {subjectId === CUSTOM_SUBJECT && (
              <Input
                value={customSubjectId}
                onChange={(e) => setCustomSubjectId(e.target.value)}
                placeholder="JWT subject UUID"
                className="mt-2"
              />
            )}
          </div>
          <div className="grid grid-cols-2 gap-3">
            <div className="space-y-1.5">
              <Label htmlFor="rl-rpm">Requests / min</Label>
              <Input id="rl-rpm" type="number" min={0} value={rpm} onChange={(e) => setRpm(e.target.value)} />
            </div>
            <div className="space-y-1.5">
              <Label htmlFor="rl-concurrent">Max concurrent</Label>
              <Input id="rl-concurrent" type="number" min={0} value={maxConcurrent} onChange={(e) => setMaxConcurrent(e.target.value)} />
            </div>
          </div>
        </div>
        <DialogFooter>
          <Button variant="outline" onClick={() => onOpenChange(false)}>Cancel</Button>
          <Button onClick={handleSubmit} disabled={!name || !effectiveSubjectId}>
            {config ? "Save" : "Create"}
          </Button>
        </DialogFooter>
      </DialogContent>
    </Dialog>
  )
}
