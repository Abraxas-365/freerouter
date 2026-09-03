import { useEffect, useState } from "react"
import { useSearchParams } from "react-router-dom"
import { toast } from "sonner"
import {
  Wallet, ArrowUpCircle, ArrowDownCircle, RefreshCw, Wrench,
  Plus, TrendingDown, TrendingUp, Save, Trash2, Loader2, DollarSign,
  ArrowRightCircle, ArrowLeftCircle,
} from "lucide-react"
import { useApi } from "@/api"
import type {
  Balance, Transaction, TransactionType,
  SpendingLimit, SpendingCheck,
  UpsertSpendingLimitRequest,
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
  wallet_fund:     { label: "Wallet Fund",     icon: ArrowRightCircle, color: "text-purple-400" },
  wallet_withdraw: { label: "Wallet Withdraw", icon: ArrowLeftCircle,  color: "text-purple-400" },
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

  // Handle return from Stripe checkout
  const [searchParams, setSearchParams] = useSearchParams()
  useEffect(() => {
    const status = searchParams.get("checkout")
    if (!status) return
    if (status === "success") {
      toast.success("Payment successful", { description: "Your credits will appear shortly." })
    } else if (status === "cancelled") {
      toast.info("Checkout cancelled")
    }
    searchParams.delete("checkout")
    setSearchParams(searchParams, { replace: true })
  }, [])

  async function handleBuyCredits(amountUsd: number) {
    const session = await api.billing.createCheckout({ amount_usd: amountUsd })
    if (session.url.startsWith("http")) {
      // Real Stripe checkout — leave the app
      window.location.href = session.url
      return
    }
    // Mock adapter completes instantly
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
              <Select value={typeFilter} onValueChange={(v) => v && setTypeFilter(v)}>
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

      {/* Buy Credits Dialog */}
      <BuyCreditsDialog
        open={topUpOpen}
        onOpenChange={setTopUpOpen}
        onSubmit={handleBuyCredits}
      />
    </div>
  )
}

/* ------------------------------------------------------------------ */
/*  Buy Credits Dialog                                                 */
/* ------------------------------------------------------------------ */

const PRESET_AMOUNTS = [10, 25, 50, 100]

function BuyCreditsDialog({
  open,
  onOpenChange,
  onSubmit,
}: {
  open: boolean
  onOpenChange: (v: boolean) => void
  onSubmit: (amountUsd: number) => Promise<void>
}) {
  const [amount, setAmount] = useState("")
  const [submitting, setSubmitting] = useState(false)

  useEffect(() => {
    if (open) {
      setAmount("")
      setSubmitting(false)
    }
  }, [open])

  async function submit() {
    setSubmitting(true)
    try {
      await onSubmit(Number(amount))
    } finally {
      setSubmitting(false)
    }
  }

  return (
    <Dialog open={open} onOpenChange={onOpenChange}>
      <DialogContent>
        <DialogHeader>
          <DialogTitle className="font-mono">Buy Credits</DialogTitle>
          <DialogDescription>
            Purchase API credits with a card. You'll be redirected to Stripe to complete the payment.
          </DialogDescription>
        </DialogHeader>
        <div className="space-y-4 py-2">
          <div className="grid grid-cols-4 gap-2">
            {PRESET_AMOUNTS.map((v) => (
              <Button
                key={v}
                type="button"
                variant={amount === String(v) ? "default" : "outline"}
                onClick={() => setAmount(String(v))}
              >
                ${v}
              </Button>
            ))}
          </div>
          <div className="space-y-2">
            <Label htmlFor="buy-amount" className="font-mono text-xs">Custom amount (USD)</Label>
            <Input
              id="buy-amount"
              type="number"
              step="1"
              min="5"
              value={amount}
              onChange={(e) => setAmount(e.target.value)}
              placeholder="e.g. 100"
            />
            <p className="text-xs text-muted-foreground">Minimum $5.00</p>
          </div>
        </div>
        <DialogFooter>
          <Button variant="outline" onClick={() => onOpenChange(false)}>Cancel</Button>
          <Button
            onClick={submit}
            disabled={!amount || Number(amount) < 5 || submitting}
          >
            {submitting
              ? <Loader2 className="h-4 w-4 mr-1 animate-spin" />
              : <DollarSign className="h-4 w-4 mr-1" />}
            Continue to Payment
          </Button>
        </DialogFooter>
      </DialogContent>
    </Dialog>
  )
}


