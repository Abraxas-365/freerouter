import { useEffect, useState } from "react"
import {
  Wallet, ArrowUpCircle, ArrowDownCircle, RefreshCw, Wrench,
  Plus, TrendingDown, TrendingUp, Save, Trash2, Loader2, DollarSign,
} from "lucide-react"
import { useApi } from "@/api"
import type {
  Balance, Transaction, TransactionType,
  SpendingLimit, SpendingCheck,
  TopUpRequest, UpsertSpendingLimitRequest,
} from "@/api/types"
import { PageHeader, MetricCard } from "@/components"
import { MetricCardSkeleton } from "@/components/feedback/skeletons"
import { Badge } from "@/components/ui/badge"
import { Button } from "@/components/ui/button"
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from "@/components/ui/card"
import {
  Dialog, DialogContent, DialogDescription, DialogFooter,
  DialogHeader, DialogTitle,
} from "@/components/ui/dialog"
import { Input } from "@/components/ui/input"
import { Label } from "@/components/ui/label"
import {
  Table, TableBody, TableCell, TableHead, TableHeader, TableRow,
} from "@/components/ui/table"
import {
  Select, SelectContent, SelectItem, SelectTrigger, SelectValue,
} from "@/components/ui/select"

const TENANT_ID = "default"

const TXN_META: Record<TransactionType, { label: string; icon: typeof Wallet; color: string }> = {
  top_up:  { label: "Top Up",     icon: ArrowUpCircle,   color: "text-green-500" },
  usage:   { label: "Usage",      icon: ArrowDownCircle, color: "text-red-400" },
  refund:  { label: "Refund",     icon: RefreshCw,       color: "text-blue-400" },
  adjust:  { label: "Adjustment", icon: Wrench,          color: "text-yellow-400" },
}

export default function BillingPage() {
  const api = useApi()

  const [balance, setBalance] = useState<Balance | null>(null)
  const [transactions, setTransactions] = useState<Transaction[]>([])
  const [spendingLimit, setSpendingLimit] = useState<SpendingLimit | null>(null)
  const [spendingCheck, setSpendingCheck] = useState<SpendingCheck | null>(null)
  const [loading, setLoading] = useState(true)

  // filters
  const [typeFilter, setTypeFilter] = useState<string>("all")

  // top-up dialog
  const [topUpOpen, setTopUpOpen] = useState(false)

  // spending limit form
  const [dailyLimit, setDailyLimit] = useState("")
  const [monthlyLimit, setMonthlyLimit] = useState("")
  const [slDirty, setSlDirty] = useState(false)
  const [slSaving, setSlSaving] = useState(false)

  async function load() {
    const [bal, txns, sl, sc] = await Promise.all([
      api.billing.getBalance(),
      api.billing.listTransactions({ limit: 50 }),
      api.billing.getSpendingLimit(TENANT_ID).catch(() => null),
      api.billing.checkSpendingLimit(TENANT_ID).catch(() => null),
    ])
    setBalance(bal)
    setTransactions(txns.data)
    setSpendingLimit(sl)
    setSpendingCheck(sc)
    if (sl) {
      setDailyLimit(sl.daily_limit_usd != null ? String(sl.daily_limit_usd) : "")
      setMonthlyLimit(sl.monthly_limit_usd != null ? String(sl.monthly_limit_usd) : "")
    }
    setLoading(false)
  }

  useEffect(() => { load() }, [api])

  async function handleTopUp(req: TopUpRequest) {
    const res = await api.billing.topUp(req)
    setBalance(res.balance)
    setTopUpOpen(false)
    load()
  }

  async function saveSpendingLimit() {
    setSlSaving(true)
    const req: UpsertSpendingLimitRequest = {}
    if (dailyLimit) req.daily_limit_usd = Number(dailyLimit)
    if (monthlyLimit) req.monthly_limit_usd = Number(monthlyLimit)
    const saved = await api.billing.upsertSpendingLimit(TENANT_ID, req)
    setSpendingLimit(saved)
    setSlDirty(false)
    setSlSaving(false)
    // refresh spending check
    const sc = await api.billing.checkSpendingLimit(TENANT_ID).catch(() => null)
    setSpendingCheck(sc)
  }

  async function deleteSpendingLimit() {
    await api.billing.deleteSpendingLimit(TENANT_ID)
    setSpendingLimit(null)
    setDailyLimit("")
    setMonthlyLimit("")
    setSlDirty(false)
    const sc = await api.billing.checkSpendingLimit(TENANT_ID).catch(() => null)
    setSpendingCheck(sc)
  }

  // computed
  const totalCredits = transactions
    .filter((t) => t.amount > 0)
    .reduce((sum, t) => sum + t.amount, 0)
  const totalSpent = Math.abs(
    transactions
      .filter((t) => t.type === "usage")
      .reduce((sum, t) => sum + t.amount, 0)
  )

  const filtered = typeFilter === "all"
    ? transactions
    : transactions.filter((t) => t.type === typeFilter)

  if (loading) {
    return (
      <div className="space-y-6 p-6">
        <PageHeader title="Billing" description="Balance, transactions, and spending limits" />
        <MetricCardSkeleton count={4} />
      </div>
    )
  }

  return (
    <div className="space-y-6 p-6">
      <PageHeader
        title="Billing"
        description="Balance, transactions, and spending limits"
        actions={
          <Button size="sm" onClick={() => setTopUpOpen(true)}>
            <Plus className="h-4 w-4 mr-1" />
            Top Up
          </Button>
        }
      />

      {/* KPIs */}
      <div className="grid grid-cols-1 sm:grid-cols-2 lg:grid-cols-4 gap-4">
        <MetricCard
          label="Current Balance"
          value={`$${(balance?.balance ?? 0).toFixed(2)}`}
          icon={Wallet}
        />
        <MetricCard
          label="Total Credits"
          value={`$${totalCredits.toFixed(2)}`}
          description="All-time top-ups & refunds"
          icon={TrendingUp}
        />
        <MetricCard
          label="Total Spent"
          value={`$${totalSpent.toFixed(2)}`}
          description="All-time usage charges"
          icon={TrendingDown}
        />
        <MetricCard
          label="Spending Status"
          value={spendingCheck?.allowed ? "Within Limits" : spendingCheck ? "Exceeded" : "No limits"}
          description={
            spendingCheck
              ? `$${spendingCheck.daily_spend_usd.toFixed(2)} today / $${spendingCheck.monthly_spend_usd.toFixed(2)} this month`
              : "No spending limits configured"
          }
          icon={DollarSign}
        />
      </div>

      <div className="grid grid-cols-1 lg:grid-cols-3 gap-6">
        {/* Transactions */}
        <Card className="lg:col-span-2">
          <CardHeader>
            <div className="flex items-center justify-between">
              <div>
                <CardTitle className="font-mono text-sm">Transactions</CardTitle>
                <CardDescription>{transactions.length} total transactions</CardDescription>
              </div>
              <Select value={typeFilter} onValueChange={setTypeFilter}>
                <SelectTrigger className="w-[140px]">
                  <SelectValue />
                </SelectTrigger>
                <SelectContent>
                  <SelectItem value="all">All types</SelectItem>
                  <SelectItem value="top_up">Top Up</SelectItem>
                  <SelectItem value="usage">Usage</SelectItem>
                  <SelectItem value="refund">Refund</SelectItem>
                  <SelectItem value="adjust">Adjustment</SelectItem>
                </SelectContent>
              </Select>
            </div>
          </CardHeader>
          <CardContent>
            <Table>
              <TableHeader>
                <TableRow>
                  <TableHead className="font-mono text-xs">Type</TableHead>
                  <TableHead className="font-mono text-xs">Description</TableHead>
                  <TableHead className="font-mono text-xs text-right">Amount</TableHead>
                  <TableHead className="font-mono text-xs text-right">Balance</TableHead>
                  <TableHead className="font-mono text-xs text-right">Date</TableHead>
                </TableRow>
              </TableHeader>
              <TableBody>
                {filtered.map((txn) => {
                  const meta = TXN_META[txn.type]
                  const Icon = meta.icon
                  return (
                    <TableRow key={txn.id}>
                      <TableCell>
                        <div className="flex items-center gap-2">
                          <Icon className={`h-4 w-4 ${meta.color}`} />
                          <Badge variant="outline" className="font-mono text-[10px]">
                            {meta.label}
                          </Badge>
                        </div>
                      </TableCell>
                      <TableCell className="text-sm max-w-[200px] truncate">
                        {txn.description}
                      </TableCell>
                      <TableCell className={`text-right font-mono tabular-nums text-sm ${
                        txn.amount > 0 ? "text-green-500" : "text-red-400"
                      }`}>
                        {txn.amount > 0 ? "+" : ""}{txn.amount.toFixed(4)}
                      </TableCell>
                      <TableCell className="text-right font-mono tabular-nums text-sm text-muted-foreground">
                        ${txn.balance_after.toFixed(2)}
                      </TableCell>
                      <TableCell className="text-right text-xs text-muted-foreground whitespace-nowrap">
                        {new Date(txn.created_at).toLocaleDateString()}
                      </TableCell>
                    </TableRow>
                  )
                })}
                {filtered.length === 0 && (
                  <TableRow>
                    <TableCell colSpan={5} className="text-center text-sm text-muted-foreground py-8">
                      No transactions found
                    </TableCell>
                  </TableRow>
                )}
              </TableBody>
            </Table>
          </CardContent>
        </Card>

        {/* Spending Limits */}
        <Card>
          <CardHeader>
            <CardTitle className="font-mono text-sm">Spending Limits</CardTitle>
            <CardDescription>
              Set daily and monthly caps to control costs
            </CardDescription>
          </CardHeader>
          <CardContent className="space-y-4">
            {/* Current usage bars */}
            {spendingCheck && (spendingCheck.daily_limit_usd != null || spendingCheck.monthly_limit_usd != null) && (
              <div className="space-y-3 pb-4 border-b">
                {spendingCheck.daily_limit_usd != null && (
                  <div className="space-y-1">
                    <div className="flex justify-between text-xs font-mono">
                      <span className="text-muted-foreground">Daily</span>
                      <span>
                        ${spendingCheck.daily_spend_usd.toFixed(2)} / ${spendingCheck.daily_limit_usd.toFixed(2)}
                      </span>
                    </div>
                    <div className="h-2 rounded-full bg-muted overflow-hidden">
                      <div
                        className={`h-full rounded-full transition-all ${
                          spendingCheck.daily_spend_usd / spendingCheck.daily_limit_usd > 0.9
                            ? "bg-red-500"
                            : spendingCheck.daily_spend_usd / spendingCheck.daily_limit_usd > 0.7
                              ? "bg-yellow-500"
                              : "bg-primary"
                        }`}
                        style={{ width: `${Math.min(100, (spendingCheck.daily_spend_usd / spendingCheck.daily_limit_usd) * 100)}%` }}
                      />
                    </div>
                  </div>
                )}
                {spendingCheck.monthly_limit_usd != null && (
                  <div className="space-y-1">
                    <div className="flex justify-between text-xs font-mono">
                      <span className="text-muted-foreground">Monthly</span>
                      <span>
                        ${spendingCheck.monthly_spend_usd.toFixed(2)} / ${spendingCheck.monthly_limit_usd.toFixed(2)}
                      </span>
                    </div>
                    <div className="h-2 rounded-full bg-muted overflow-hidden">
                      <div
                        className={`h-full rounded-full transition-all ${
                          spendingCheck.monthly_spend_usd / spendingCheck.monthly_limit_usd > 0.9
                            ? "bg-red-500"
                            : spendingCheck.monthly_spend_usd / spendingCheck.monthly_limit_usd > 0.7
                              ? "bg-yellow-500"
                              : "bg-primary"
                        }`}
                        style={{ width: `${Math.min(100, (spendingCheck.monthly_spend_usd / spendingCheck.monthly_limit_usd) * 100)}%` }}
                      />
                    </div>
                  </div>
                )}
              </div>
            )}

            {/* Limit inputs */}
            <div className="space-y-2">
              <Label htmlFor="daily-limit" className="font-mono text-xs">Daily limit (USD)</Label>
              <Input
                id="daily-limit"
                type="number"
                value={dailyLimit}
                onChange={(e) => { setDailyLimit(e.target.value); setSlDirty(true) }}
                placeholder="e.g. 50"
              />
            </div>
            <div className="space-y-2">
              <Label htmlFor="monthly-limit" className="font-mono text-xs">Monthly limit (USD)</Label>
              <Input
                id="monthly-limit"
                type="number"
                value={monthlyLimit}
                onChange={(e) => { setMonthlyLimit(e.target.value); setSlDirty(true) }}
                placeholder="e.g. 500"
              />
            </div>
            <div className="flex gap-2">
              <Button
                size="sm"
                onClick={saveSpendingLimit}
                disabled={!slDirty || slSaving}
              >
                {slSaving ? <Loader2 className="h-4 w-4 mr-1 animate-spin" /> : <Save className="h-4 w-4 mr-1" />}
                Save
              </Button>
              {spendingLimit && (
                <Button
                  size="sm"
                  variant="outline"
                  onClick={deleteSpendingLimit}
                >
                  <Trash2 className="h-4 w-4 mr-1" />
                  Remove
                </Button>
              )}
            </div>
          </CardContent>
        </Card>
      </div>

      {/* Top Up Dialog */}
      <TopUpDialog
        open={topUpOpen}
        onOpenChange={setTopUpOpen}
        onSubmit={handleTopUp}
      />
    </div>
  )
}

/* ------------------------------------------------------------------ */
/*  Top Up Dialog                                                      */
/* ------------------------------------------------------------------ */

function TopUpDialog({
  open,
  onOpenChange,
  onSubmit,
}: {
  open: boolean
  onOpenChange: (v: boolean) => void
  onSubmit: (req: TopUpRequest) => void
}) {
  const [amount, setAmount] = useState("")
  const [description, setDescription] = useState("")
  const [referenceId, setReferenceId] = useState("")

  useEffect(() => {
    if (open) {
      setAmount("")
      setDescription("")
      setReferenceId("")
    }
  }, [open])

  return (
    <Dialog open={open} onOpenChange={onOpenChange}>
      <DialogContent>
        <DialogHeader>
          <DialogTitle className="font-mono">Add Credit</DialogTitle>
          <DialogDescription>
            Top up the tenant balance with a credit amount.
          </DialogDescription>
        </DialogHeader>
        <div className="space-y-4 py-2">
          <div className="space-y-2">
            <Label htmlFor="topup-amount" className="font-mono text-xs">Amount (USD)</Label>
            <Input
              id="topup-amount"
              type="number"
              step="0.01"
              min="0"
              value={amount}
              onChange={(e) => setAmount(e.target.value)}
              placeholder="e.g. 100"
            />
          </div>
          <div className="space-y-2">
            <Label htmlFor="topup-desc" className="font-mono text-xs">Description</Label>
            <Input
              id="topup-desc"
              value={description}
              onChange={(e) => setDescription(e.target.value)}
              placeholder="e.g. Monthly credit top-up"
            />
          </div>
          <div className="space-y-2">
            <Label htmlFor="topup-ref" className="font-mono text-xs">Reference ID</Label>
            <Input
              id="topup-ref"
              value={referenceId}
              onChange={(e) => setReferenceId(e.target.value)}
              placeholder="e.g. inv-2024-001"
            />
          </div>
        </div>
        <DialogFooter>
          <Button variant="outline" onClick={() => onOpenChange(false)}>Cancel</Button>
          <Button
            onClick={() => onSubmit({
              amount: Number(amount),
              description,
              reference_id: referenceId,
            })}
            disabled={!amount || Number(amount) <= 0 || !description}
          >
            <Plus className="h-4 w-4 mr-1" />
            Add Credit
          </Button>
        </DialogFooter>
      </DialogContent>
    </Dialog>
  )
}
