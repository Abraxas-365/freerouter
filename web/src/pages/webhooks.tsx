import { useEffect, useState } from "react"
import {
  Webhook, Plus, Pencil, Trash2, MoreHorizontal,
  Send, Loader2, CheckCircle2, XCircle, Clock,
  Zap, ZapOff,
} from "lucide-react"
import { useApi } from "@/api"
import type {
  WebhookConfig, WebhookDelivery, CreateWebhookRequest,
} from "@/api/types"
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
  Table, TableBody, TableCell, TableHead, TableHeader, TableRow,
} from "@/components/ui/table"

const DELIVERY_ICON = {
  success: CheckCircle2,
  failed: XCircle,
  pending: Clock,
}
const DELIVERY_COLOR = {
  success: "text-green-500",
  failed: "text-red-400",
  pending: "text-yellow-400",
}

export default function WebhooksPage() {
  const api = useApi()

  const [webhooks, setWebhooks] = useState<WebhookConfig[]>([])
  const [events, setEvents] = useState<string[]>([])
  const [loading, setLoading] = useState(true)

  // dialogs
  const [createOpen, setCreateOpen] = useState(false)
  const [editWebhook, setEditWebhook] = useState<WebhookConfig | null>(null)
  const [deleteId, setDeleteId] = useState<string | null>(null)

  // deliveries panel
  const [deliveriesWebhookId, setDeliveriesWebhookId] = useState<string | null>(null)
  const [deliveries, setDeliveries] = useState<WebhookDelivery[]>([])
  const [deliveriesLoading, setDeliveriesLoading] = useState(false)

  // secret display
  const [newSecret, setNewSecret] = useState<string | null>(null)

  // test
  const [testingId, setTestingId] = useState<string | null>(null)
  const [testMsg, setTestMsg] = useState<string | null>(null)

  async function load() {
    const [wh, ev] = await Promise.all([
      api.webhooks.list(),
      api.webhooks.listEvents(),
    ])
    setWebhooks(wh.data)
    setEvents(ev)
    setLoading(false)
  }

  useEffect(() => { load() }, [api])

  async function handleCreate(req: CreateWebhookRequest) {
    const res = await api.webhooks.create(req)
    setNewSecret(res.secret)
    setCreateOpen(false)
    load()
  }

  async function handleUpdate(id: string, req: Partial<CreateWebhookRequest> & { enabled?: boolean }) {
    await api.webhooks.update(id, req)
    setEditWebhook(null)
    load()
  }

  async function handleDelete() {
    if (!deleteId) return
    await api.webhooks.delete(deleteId)
    setDeleteId(null)
    if (deliveriesWebhookId === deleteId) setDeliveriesWebhookId(null)
    load()
  }

  async function handleTest(id: string) {
    setTestingId(id)
    setTestMsg(null)
    const res = await api.webhooks.test(id)
    setTestMsg(res.message)
    setTestingId(null)
    setTimeout(() => setTestMsg(null), 3000)
  }

  async function loadDeliveries(webhookId: string) {
    setDeliveriesWebhookId(webhookId)
    setDeliveriesLoading(true)
    const res = await api.webhooks.listDeliveries(webhookId)
    setDeliveries(res.data)
    setDeliveriesLoading(false)
  }

  const activeCount = webhooks.filter((w) => w.enabled).length

  if (loading) {
    return (
      <div className="space-y-6 p-6">
        <PageHeader title="Webhooks" description="Event notifications and delivery management" />
        <MetricCardSkeleton count={3} />
      </div>
    )
  }

  return (
    <div className="space-y-6 p-6">
      <PageHeader
        title="Webhooks"
        description="Event notifications and delivery management"
        actions={
          <Button size="sm" onClick={() => setCreateOpen(true)}>
            <Plus className="h-4 w-4 mr-1" />
            Add Webhook
          </Button>
        }
      />

      {/* KPIs */}
      <div className="grid grid-cols-1 sm:grid-cols-3 gap-4">
        <MetricCard label="Total Webhooks" value={webhooks.length} icon={Webhook} />
        <MetricCard label="Active" value={activeCount} icon={Zap} />
        <MetricCard label="Available Events" value={events.length} icon={Send} />
      </div>

      {/* Secret Banner */}
      {newSecret && (
        <div className="rounded-md border border-yellow-500/30 bg-yellow-500/5 p-4">
          <p className="text-sm font-medium mb-1">Webhook Secret Created</p>
          <p className="text-xs text-muted-foreground mb-2">Copy this secret now. It won't be shown again.</p>
          <code className="block text-xs font-mono bg-muted p-2 rounded break-all">{newSecret}</code>
          <Button size="sm" variant="outline" className="mt-2" onClick={() => setNewSecret(null)}>
            Dismiss
          </Button>
        </div>
      )}

      {/* Webhooks List */}
      <Card>
        <CardHeader>
          <CardTitle className="font-mono text-sm">Endpoints</CardTitle>
          <CardDescription>{webhooks.length} webhook endpoints configured</CardDescription>
        </CardHeader>
        <CardContent>
          <Table>
            <TableHeader>
              <TableRow>
                <TableHead className="font-mono text-xs">URL</TableHead>
                <TableHead className="font-mono text-xs">Events</TableHead>
                <TableHead className="font-mono text-xs">Status</TableHead>
                <TableHead className="font-mono text-xs text-right">Created</TableHead>
                <TableHead className="font-mono text-xs w-8" />
              </TableRow>
            </TableHeader>
            <TableBody>
              {webhooks.map((wh) => (
                <TableRow
                  key={wh.id}
                  className="cursor-pointer"
                  onClick={() => loadDeliveries(wh.id)}
                >
                  <TableCell className="font-mono text-sm max-w-[300px] truncate">{wh.url}</TableCell>
                  <TableCell>
                    <div className="flex flex-wrap gap-1">
                      {wh.events.map((e) => (
                        <Badge key={e} variant="outline" className="font-mono text-[10px]">{e}</Badge>
                      ))}
                    </div>
                  </TableCell>
                  <TableCell>
                    <StatusBadge status={wh.enabled ? "active" : "inactive"} />
                  </TableCell>
                  <TableCell className="text-right text-xs text-muted-foreground whitespace-nowrap">
                    {new Date(wh.created_at).toLocaleDateString()}
                  </TableCell>
                  <TableCell onClick={(e) => e.stopPropagation()}>
                    <DropdownMenu>
                      <DropdownMenuTrigger asChild>
                        <Button variant="ghost" size="icon" className="h-7 w-7">
                          <MoreHorizontal className="h-4 w-4" />
                        </Button>
                      </DropdownMenuTrigger>
                      <DropdownMenuContent align="end">
                        <DropdownMenuItem onClick={() => handleTest(wh.id)}>
                          <Send className="h-4 w-4 mr-2" /> Test
                        </DropdownMenuItem>
                        <DropdownMenuItem onClick={() => setEditWebhook(wh)}>
                          <Pencil className="h-4 w-4 mr-2" /> Edit
                        </DropdownMenuItem>
                        <DropdownMenuItem
                          onClick={() => handleUpdate(wh.id, { enabled: !wh.enabled })}
                        >
                          {wh.enabled
                            ? <><ZapOff className="h-4 w-4 mr-2" /> Disable</>
                            : <><Zap className="h-4 w-4 mr-2" /> Enable</>}
                        </DropdownMenuItem>
                        <DropdownMenuSeparator />
                        <DropdownMenuItem className="text-destructive" onClick={() => setDeleteId(wh.id)}>
                          <Trash2 className="h-4 w-4 mr-2" /> Delete
                        </DropdownMenuItem>
                      </DropdownMenuContent>
                    </DropdownMenu>
                  </TableCell>
                </TableRow>
              ))}
              {webhooks.length === 0 && (
                <TableRow>
                  <TableCell colSpan={5} className="text-center text-sm text-muted-foreground py-8">
                    No webhooks configured
                  </TableCell>
                </TableRow>
              )}
            </TableBody>
          </Table>
          {testMsg && (
            <p className="text-xs text-green-500 font-mono mt-2">{testMsg}</p>
          )}
        </CardContent>
      </Card>

      {/* Deliveries Panel */}
      {deliveriesWebhookId && (
        <Card>
          <CardHeader>
            <div className="flex items-center justify-between">
              <div>
                <CardTitle className="font-mono text-sm">Deliveries</CardTitle>
                <CardDescription>
                  {webhooks.find((w) => w.id === deliveriesWebhookId)?.url}
                </CardDescription>
              </div>
              <Button size="sm" variant="ghost" onClick={() => setDeliveriesWebhookId(null)}>
                Close
              </Button>
            </div>
          </CardHeader>
          <CardContent>
            {deliveriesLoading ? (
              <div className="flex items-center justify-center py-8">
                <Loader2 className="h-5 w-5 animate-spin text-muted-foreground" />
              </div>
            ) : (
              <Table>
                <TableHeader>
                  <TableRow>
                    <TableHead className="font-mono text-xs">Status</TableHead>
                    <TableHead className="font-mono text-xs">Event</TableHead>
                    <TableHead className="font-mono text-xs">Response</TableHead>
                    <TableHead className="font-mono text-xs">Attempts</TableHead>
                    <TableHead className="font-mono text-xs">Error</TableHead>
                    <TableHead className="font-mono text-xs text-right">Time</TableHead>
                  </TableRow>
                </TableHeader>
                <TableBody>
                  {deliveries.map((d) => {
                    const StatusIcon = DELIVERY_ICON[d.status]
                    return (
                      <TableRow key={d.id}>
                        <TableCell>
                          <div className="flex items-center gap-1.5">
                            <StatusIcon className={`h-4 w-4 ${DELIVERY_COLOR[d.status]}`} />
                            <span className="font-mono text-xs">{d.status}</span>
                          </div>
                        </TableCell>
                        <TableCell>
                          <Badge variant="outline" className="font-mono text-[10px]">{d.event_type}</Badge>
                        </TableCell>
                        <TableCell className="font-mono text-xs text-muted-foreground">
                          {d.status_code ?? "—"}
                        </TableCell>
                        <TableCell className="font-mono text-xs">{d.attempts}</TableCell>
                        <TableCell className="text-xs text-muted-foreground max-w-[200px] truncate">
                          {d.last_error ?? "—"}
                        </TableCell>
                        <TableCell className="text-right text-xs text-muted-foreground whitespace-nowrap">
                          {timeAgo(d.created_at)}
                        </TableCell>
                      </TableRow>
                    )
                  })}
                  {deliveries.length === 0 && (
                    <TableRow>
                      <TableCell colSpan={6} className="text-center text-sm text-muted-foreground py-8">
                        No deliveries yet
                      </TableCell>
                    </TableRow>
                  )}
                </TableBody>
              </Table>
            )}
          </CardContent>
        </Card>
      )}

      {/* Create / Edit Dialog */}
      <WebhookFormDialog
        open={createOpen || !!editWebhook}
        onOpenChange={(v) => { if (!v) { setCreateOpen(false); setEditWebhook(null) } }}
        webhook={editWebhook}
        availableEvents={events}
        onCreate={handleCreate}
        onUpdate={(req) => editWebhook && handleUpdate(editWebhook.id, req)}
      />

      {/* Delete Confirm */}
      <ConfirmDialog
        open={!!deleteId}
        onOpenChange={(v) => { if (!v) setDeleteId(null) }}
        title="Delete Webhook"
        description="This webhook and all its delivery history will be permanently deleted."
        onConfirm={handleDelete}
        variant="destructive"
      />
    </div>
  )
}

/* ------------------------------------------------------------------ */
/*  Webhook Form Dialog                                                */
/* ------------------------------------------------------------------ */

function WebhookFormDialog({
  open,
  onOpenChange,
  webhook,
  availableEvents,
  onCreate,
  onUpdate,
}: {
  open: boolean
  onOpenChange: (v: boolean) => void
  webhook: WebhookConfig | null
  availableEvents: string[]
  onCreate: (req: CreateWebhookRequest) => void
  onUpdate: (req: Partial<CreateWebhookRequest>) => void
}) {
  const [url, setUrl] = useState("")
  const [selectedEvents, setSelectedEvents] = useState<string[]>([])

  useEffect(() => {
    if (open) {
      if (webhook) {
        setUrl(webhook.url)
        setSelectedEvents([...webhook.events])
      } else {
        setUrl("")
        setSelectedEvents([])
      }
    }
  }, [open, webhook])

  function toggleEvent(event: string) {
    setSelectedEvents((prev) =>
      prev.includes(event) ? prev.filter((e) => e !== event) : [...prev, event]
    )
  }

  function handleSubmit() {
    if (webhook) {
      onUpdate({ url, events: selectedEvents })
    } else {
      onCreate({ url, events: selectedEvents })
    }
  }

  return (
    <Dialog open={open} onOpenChange={onOpenChange}>
      <DialogContent>
        <DialogHeader>
          <DialogTitle className="font-mono">{webhook ? "Edit Webhook" : "Add Webhook"}</DialogTitle>
          <DialogDescription>
            {webhook ? "Update the webhook endpoint configuration." : "Create a new webhook endpoint to receive event notifications."}
          </DialogDescription>
        </DialogHeader>
        <div className="space-y-4 py-2">
          <div className="space-y-2">
            <Label className="font-mono text-xs">Endpoint URL</Label>
            <Input
              value={url}
              onChange={(e) => setUrl(e.target.value)}
              placeholder="https://example.com/webhooks"
            />
          </div>
          <div className="space-y-2">
            <Label className="font-mono text-xs">Events</Label>
            <div className="flex flex-wrap gap-2">
              {availableEvents.map((event) => (
                <Badge
                  key={event}
                  variant={selectedEvents.includes(event) ? "default" : "outline"}
                  className="cursor-pointer font-mono text-[10px]"
                  onClick={() => toggleEvent(event)}
                >
                  {event}
                </Badge>
              ))}
            </div>
          </div>
        </div>
        <DialogFooter>
          <Button variant="outline" onClick={() => onOpenChange(false)}>Cancel</Button>
          <Button onClick={handleSubmit} disabled={!url || selectedEvents.length === 0}>
            {webhook ? "Update" : "Create"}
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
