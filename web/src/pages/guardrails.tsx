import { useEffect, useState } from "react"
import {
  Shield, ShieldCheck, ShieldAlert, ShieldOff,
  Plus, Pencil, Trash2, MoreHorizontal,
} from "lucide-react"
import { toast } from "sonner"
import { useApi } from "@/api"
import type {
  GuardrailConfig, GuardrailRule, GuardrailViolation,
  GuardrailAction, GuardrailRuleType, SystemRuleConfig,
  CreateGuardrailRuleRequest, UpdateGuardrailRuleRequest,
  BlockedTermsConfig, CustomRegexConfig,
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
  Select, SelectContent, SelectItem, SelectTrigger, SelectValue,
} from "@/components/ui/select"
import { Switch } from "@/components/ui/switch"
import {
  Table, TableBody, TableCell, TableHead, TableHeader, TableRow,
} from "@/components/ui/table"
import { Textarea } from "@/components/ui/textarea"

const ACTION_COLORS: Record<string, "destructive" | "default" | "secondary" | "outline"> = {
  block: "destructive",
  redact: "default",
  warn: "secondary",
}

const SYSTEM_RULE_LABELS: Record<string, { label: string; description: string }> = {
  prompt_injection: { label: "Prompt Injection", description: "Detects attempts to override system instructions" },
  jailbreak: { label: "Jailbreak", description: "Detects attempts to bypass safety guidelines" },
  pii_detection: { label: "PII Detection", description: "Detects personal information like names, emails, addresses" },
  secrets: { label: "Secrets", description: "Detects API keys, passwords, and credentials" },
  document_leakage: { label: "Document Leakage", description: "Detects classification markers like CONFIDENTIAL or TOP SECRET" },
}

export default function GuardrailsPage() {
  const api = useApi()

  const [config, setConfig] = useState<GuardrailConfig | null>(null)
  const [rules, setRules] = useState<GuardrailRule[]>([])
  const [violations, setViolations] = useState<GuardrailViolation[]>([])
  const [loading, setLoading] = useState(true)

  // dialogs
  const [ruleDialogOpen, setRuleDialogOpen] = useState(false)
  const [editingRule, setEditingRule] = useState<GuardrailRule | null>(null)
  const [deleteRuleId, setDeleteRuleId] = useState<string | null>(null)

  // saving state
  const [configSaving, setConfigSaving] = useState(false)

  async function load() {
    const [cfg, rls, viols] = await Promise.all([
      api.guardrails.getConfig().catch(() => null),
      api.guardrails.listRules(),
      api.guardrails.listViolations({ limit: 50 }),
    ])
    setConfig(cfg)
    setRules(rls)
    setViolations(viols.items)
    setLoading(false)
  }

  useEffect(() => { load() }, [api])

  async function toggleEnabled(enabled: boolean) {
    setConfigSaving(true)
    const updated = await api.guardrails.upsertConfig({ enabled })
    setConfig(updated)
    setConfigSaving(false)
  }

  async function updateSystemRule(key: keyof GuardrailConfig["system_rules"], update: Partial<SystemRuleConfig>) {
    if (!config) return
    const newRules = { ...config.system_rules, [key]: { ...config.system_rules[key], ...update } }
    const updated = await api.guardrails.upsertConfig({ system_rules: newRules })
    setConfig(updated)
  }

  async function handleCreateRule(req: CreateGuardrailRuleRequest) {
    await api.guardrails.createRule(req)
    setRuleDialogOpen(false)
    toast.success("Rule created")
    load()
  }

  async function handleUpdateRule(id: string, req: UpdateGuardrailRuleRequest) {
    await api.guardrails.updateRule(id, req)
    setEditingRule(null)
    toast.success("Rule updated")
    load()
  }

  async function handleDeleteRule() {
    if (!deleteRuleId) return
    await api.guardrails.deleteRule(deleteRuleId)
    setDeleteRuleId(null)
    toast.success("Rule deleted")
    load()
  }

  const activeRules = rules.filter((r) => r.enabled).length
  const recentViolations = violations.filter(
    (v) => Date.now() - new Date(v.created_at).getTime() < 24 * 60 * 60 * 1000
  ).length

  if (loading) {
    return (
      <div className="space-y-6 p-6">
        <PageHeader title="Guardrails" description="Content safety rules and violation tracking" />
        <MetricCardSkeleton count={4} />
      </div>
    )
  }

  return (
    <div className="space-y-6 p-6">
      <PageHeader
        title="Guardrails"
        description="Content safety rules and violation tracking"
        actions={
          <Button size="sm" onClick={() => { setEditingRule(null); setRuleDialogOpen(true) }}>
            <Plus className="h-4 w-4" /> Add Rule
          </Button>
        }
      />

      {/* KPIs */}
      <div className="grid grid-cols-1 sm:grid-cols-2 lg:grid-cols-4 gap-4">
        <MetricCard
          label="Status"
          value={config?.enabled ? "Enabled" : "Disabled"}
          icon={config?.enabled ? ShieldCheck : ShieldOff}
        />
        <MetricCard
          label="Custom Rules"
          value={rules.length}
          description={`${activeRules} active`}
          icon={Shield}
        />
        <MetricCard
          label="Violations (24h)"
          value={recentViolations}
          icon={ShieldAlert}
        />
        <MetricCard
          label="Total Violations"
          value={violations.length}
          icon={ShieldAlert}
        />
      </div>

      {/* Master toggle + System Rules */}
      <Card>
        <CardHeader>
          <div className="flex items-center justify-between">
            <div>
              <CardTitle className="font-mono text-sm">System Rules</CardTitle>
              <CardDescription>Built-in safety checks applied to all requests</CardDescription>
            </div>
            <div className="flex items-center gap-2">
              <Label className="text-xs text-muted-foreground font-mono">
                {config?.enabled ? "Enabled" : "Disabled"}
              </Label>
              <Switch
                checked={config?.enabled ?? false}
                onCheckedChange={toggleEnabled}
                disabled={configSaving}
              />
            </div>
          </div>
        </CardHeader>
        <CardContent>
          <div className="space-y-3">
            {config && (Object.entries(config.system_rules) as [keyof GuardrailConfig["system_rules"], SystemRuleConfig][]).map(([key, rule]) => {
              const meta = SYSTEM_RULE_LABELS[key]
              return (
                <div key={key} className="flex items-center justify-between py-2 border-b last:border-0">
                  <div className="flex items-center gap-3">
                    <Switch
                      checked={rule.enabled}
                      onCheckedChange={(checked) => updateSystemRule(key, { enabled: checked })}
                    />
                    <div>
                      <p className="text-sm font-medium">{meta?.label ?? key}</p>
                      <p className="text-xs text-muted-foreground">{meta?.description}</p>
                    </div>
                  </div>
                  <Select
                    value={rule.action}
                    onValueChange={(v) => v && updateSystemRule(key, { action: v as GuardrailAction })}
                  >
                    <SelectTrigger className="w-[100px] h-7">
                      <SelectValue />
                    </SelectTrigger>
                    <SelectContent>
                      <SelectItem value="block">Block</SelectItem>
                      <SelectItem value="redact">Redact</SelectItem>
                      <SelectItem value="warn">Warn</SelectItem>
                    </SelectContent>
                  </Select>
                </div>
              )
            })}
          </div>
        </CardContent>
      </Card>

      {/* Custom Rules */}
      <Card>
        <CardHeader>
          <CardTitle className="font-mono text-sm">Custom Rules</CardTitle>
          <CardDescription>
            Tenant-specific content filtering rules
          </CardDescription>
        </CardHeader>
        <CardContent>
          <Table>
            <TableHeader>
              <TableRow>
                <TableHead className="font-mono text-xs">Name</TableHead>
                <TableHead className="font-mono text-xs">Type</TableHead>
                <TableHead className="font-mono text-xs">Action</TableHead>
                <TableHead className="font-mono text-xs">Priority</TableHead>
                <TableHead className="font-mono text-xs">Status</TableHead>
                <TableHead className="font-mono text-xs w-8" />
              </TableRow>
            </TableHeader>
            <TableBody>
              {rules.map((rule) => (
                <TableRow key={rule.id}>
                  <TableCell>
                    <div>
                      <p className="font-mono text-sm">{rule.name}</p>
                      <p className="text-[10px] text-muted-foreground mt-0.5">
                        {rule.type === "blocked_terms"
                          ? `${((rule.config as BlockedTermsConfig).terms ?? []).length} terms`
                          : (rule.config as CustomRegexConfig).pattern}
                      </p>
                    </div>
                  </TableCell>
                  <TableCell>
                    <Badge variant="outline" className="font-mono text-[10px]">
                      {rule.type === "blocked_terms" ? "Terms" : "Regex"}
                    </Badge>
                  </TableCell>
                  <TableCell>
                    <Badge variant={ACTION_COLORS[rule.action]} className="font-mono text-[10px]">
                      {rule.action}
                    </Badge>
                  </TableCell>
                  <TableCell className="font-mono text-sm">{rule.priority}</TableCell>
                  <TableCell>
                    <StatusBadge status={rule.enabled ? "active" : "inactive"} />
                  </TableCell>
                  <TableCell>
                    <DropdownMenu>
                      <DropdownMenuTrigger>
                        <Button variant="ghost" size="icon-sm">
                          <MoreHorizontal className="h-4 w-4" />
                        </Button>
                      </DropdownMenuTrigger>
                      <DropdownMenuContent align="end">
                        <DropdownMenuItem onClick={() => { setEditingRule(rule); setRuleDialogOpen(true) }}>
                          <Pencil className="h-3.5 w-3.5" /> Edit
                        </DropdownMenuItem>
                        <DropdownMenuItem
                          onClick={() => handleUpdateRule(rule.id, { enabled: !rule.enabled })}
                        >
                          {rule.enabled
                            ? <><ShieldOff className="h-3.5 w-3.5" /> Disable</>
                            : <><ShieldCheck className="h-3.5 w-3.5" /> Enable</>}
                        </DropdownMenuItem>
                        <DropdownMenuSeparator />
                        <DropdownMenuItem variant="destructive" onClick={() => setDeleteRuleId(rule.id)}>
                          <Trash2 className="h-3.5 w-3.5" /> Delete
                        </DropdownMenuItem>
                      </DropdownMenuContent>
                    </DropdownMenu>
                  </TableCell>
                </TableRow>
              ))}
              {rules.length === 0 && (
                <TableRow>
                  <TableCell colSpan={6} className="text-center text-sm text-muted-foreground py-8">
                    No custom rules configured
                  </TableCell>
                </TableRow>
              )}
            </TableBody>
          </Table>
        </CardContent>
      </Card>

      {/* Violations */}
      <Card>
        <CardHeader>
          <CardTitle className="font-mono text-sm">Recent Violations</CardTitle>
          <CardDescription>{violations.length} total violations</CardDescription>
        </CardHeader>
        <CardContent>
          <Table>
            <TableHeader>
              <TableRow>
                <TableHead className="font-mono text-xs">Rule</TableHead>
                <TableHead className="font-mono text-xs">Category</TableHead>
                <TableHead className="font-mono text-xs">Action</TableHead>
                <TableHead className="font-mono text-xs">Model</TableHead>
                <TableHead className="font-mono text-xs">Matched</TableHead>
                <TableHead className="font-mono text-xs text-right">Time</TableHead>
              </TableRow>
            </TableHeader>
            <TableBody>
              {violations.map((v) => (
                <TableRow key={v.id}>
                  <TableCell className="font-mono text-sm">{v.rule_name}</TableCell>
                  <TableCell>
                    <Badge variant="outline" className="font-mono text-[10px]">{v.category}</Badge>
                  </TableCell>
                  <TableCell>
                    <Badge
                      variant={ACTION_COLORS[v.action_taken] ?? "outline"}
                      className="font-mono text-[10px]"
                    >
                      {v.action_taken}
                    </Badge>
                  </TableCell>
                  <TableCell className="font-mono text-sm text-muted-foreground">{v.model ?? "—"}</TableCell>
                  <TableCell className="text-sm max-w-[200px] truncate text-muted-foreground">
                    {v.matched_content ?? v.matched_pattern ?? "—"}
                  </TableCell>
                  <TableCell className="text-right text-xs text-muted-foreground whitespace-nowrap">
                    {timeAgo(v.created_at)}
                  </TableCell>
                </TableRow>
              ))}
              {violations.length === 0 && (
                <TableRow>
                  <TableCell colSpan={6} className="text-center text-sm text-muted-foreground py-8">
                    No violations recorded
                  </TableCell>
                </TableRow>
              )}
            </TableBody>
          </Table>
        </CardContent>
      </Card>

      {/* Rule Dialog */}
      <RuleFormDialog
        open={ruleDialogOpen}
        onOpenChange={(v) => { setRuleDialogOpen(v); if (!v) setEditingRule(null) }}
        rule={editingRule}
        onCreate={handleCreateRule}
        onUpdate={(req) => editingRule && handleUpdateRule(editingRule.id, req)}
      />

      {/* Delete Confirm */}
      <ConfirmDialog
        open={!!deleteRuleId}
        onOpenChange={(v: boolean) => { if (!v) setDeleteRuleId(null) }}
        title="Delete Rule"
        description="This rule will be permanently deleted. This cannot be undone."
        onConfirm={handleDeleteRule}
        variant="destructive"
      />
    </div>
  )
}

/* ------------------------------------------------------------------ */
/*  Rule Form Dialog                                                   */
/* ------------------------------------------------------------------ */

function RuleFormDialog({
  open,
  onOpenChange,
  rule,
  onCreate,
  onUpdate,
}: {
  open: boolean
  onOpenChange: (v: boolean) => void
  rule: GuardrailRule | null
  onCreate: (req: CreateGuardrailRuleRequest) => void
  onUpdate: (req: UpdateGuardrailRuleRequest) => void
}) {
  const [name, setName] = useState("")
  const [type, setType] = useState<GuardrailRuleType>("blocked_terms")
  const [action, setAction] = useState<GuardrailAction>("block")
  const [terms, setTerms] = useState("")
  const [pattern, setPattern] = useState("")
  const [priority, setPriority] = useState("")

  useEffect(() => {
    if (open) {
      if (rule) {
        setName(rule.name)
        setType(rule.type)
        setAction(rule.action)
        setPriority(String(rule.priority))
        if (rule.type === "blocked_terms") {
          setTerms(((rule.config as BlockedTermsConfig).terms ?? []).join(", "))
        } else {
          setPattern((rule.config as CustomRegexConfig).pattern ?? "")
        }
      } else {
        setName("")
        setType("blocked_terms")
        setAction("block")
        setTerms("")
        setPattern("")
        setPriority("")
      }
    }
  }, [open, rule])

  function handleSubmit() {
    const config: BlockedTermsConfig | CustomRegexConfig = type === "blocked_terms"
      ? { terms: terms.split(",").map((t) => t.trim()).filter(Boolean), match_type: "contains", case_sensitive: false }
      : { pattern }

    if (rule) {
      onUpdate({ name, config, action, priority: priority ? Number(priority) : undefined })
    } else {
      onCreate({ name, type, config, action, priority: priority ? Number(priority) : undefined })
    }
  }

  return (
    <Dialog open={open} onOpenChange={onOpenChange}>
      <DialogContent>
        <DialogHeader>
          <DialogTitle className="font-mono">{rule ? "Edit Rule" : "Create Rule"}</DialogTitle>
          <DialogDescription>
            {rule ? "Update the custom guardrail rule." : "Add a new custom content filtering rule."}
          </DialogDescription>
        </DialogHeader>
        <div className="space-y-4 py-2">
          <div className="space-y-2">
            <Label className="font-mono text-xs">Name</Label>
            <Input value={name} onChange={(e) => setName(e.target.value)} placeholder="e.g. Block competitor mentions" />
          </div>
          {!rule && (
            <div className="space-y-2">
              <Label className="font-mono text-xs">Type</Label>
              <Select value={type} onValueChange={(v) => v && setType(v as GuardrailRuleType)}>
                <SelectTrigger><SelectValue /></SelectTrigger>
                <SelectContent>
                  <SelectItem value="blocked_terms">Blocked Terms</SelectItem>
                  <SelectItem value="custom_regex">Custom Regex</SelectItem>
                </SelectContent>
              </Select>
            </div>
          )}
          {type === "blocked_terms" ? (
            <div className="space-y-2">
              <Label className="font-mono text-xs">Terms (comma-separated)</Label>
              <Textarea value={terms} onChange={(e) => setTerms(e.target.value)} placeholder="term1, term2, term3" rows={3} />
            </div>
          ) : (
            <div className="space-y-2">
              <Label className="font-mono text-xs">Regex Pattern</Label>
              <Input value={pattern} onChange={(e) => setPattern(e.target.value)} placeholder="e.g. \b\d{4}-\d{4}\b" />
            </div>
          )}
          <div className="grid grid-cols-2 gap-4">
            <div className="space-y-2">
              <Label className="font-mono text-xs">Action</Label>
              <Select value={action} onValueChange={(v) => v && setAction(v as GuardrailAction)}>
                <SelectTrigger><SelectValue /></SelectTrigger>
                <SelectContent>
                  <SelectItem value="block">Block</SelectItem>
                  <SelectItem value="redact">Redact</SelectItem>
                  <SelectItem value="warn">Warn</SelectItem>
                </SelectContent>
              </Select>
            </div>
            <div className="space-y-2">
              <Label className="font-mono text-xs">Priority</Label>
              <Input type="number" value={priority} onChange={(e) => setPriority(e.target.value)} placeholder="e.g. 1" />
            </div>
          </div>
        </div>
        <DialogFooter>
          <Button variant="outline" onClick={() => onOpenChange(false)}>Cancel</Button>
          <Button onClick={handleSubmit} disabled={!name}>
            {rule ? "Update" : "Create"}
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
