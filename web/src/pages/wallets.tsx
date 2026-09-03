import { useEffect, useState } from "react"
import {
  Wallet as WalletIcon, Plus, Pencil, Trash2, MoreHorizontal,
  ArrowRightCircle, ArrowLeftCircle, Loader2, PiggyBank, Landmark, KeyRound,
} from "lucide-react"
import { toast } from "sonner"
import { useApi } from "@/api"
import type { Wallet, Balance, ApiKey } from "@/api/types"
import { PageHeader, EmptyState, MetricCard } from "@/components"
import { MetricCardSkeleton } from "@/components/feedback/skeletons"
import { ConfirmDialog } from "@/components/feedback/confirm-dialog"
import { Badge } from "@/components/ui/badge"
import { Button } from "@/components/ui/button"
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from "@/components/ui/card"
import {
  Dialog, DialogContent, DialogDescription, DialogFooter,
  DialogHeader, DialogTitle,
} from "@/components/ui/dialog"
import {
  DropdownMenu, DropdownMenuContent, DropdownMenuItem, DropdownMenuTrigger,
} from "@/components/ui/dropdown-menu"
import { Input } from "@/components/ui/input"
import { Label } from "@/components/ui/label"
import {
  Table, TableBody, TableCell, TableHead, TableHeader, TableRow,
} from "@/components/ui/table"

export default function WalletsPage() {
  const api = useApi()

  const [wallets, setWallets] = useState<Wallet[]>([])
  const [balance, setBalance] = useState<Balance | null>(null)
  const [apiKeys, setApiKeys] = useState<ApiKey[]>([])
  const [loading, setLoading] = useState(true)

  // dialogs
  const [createOpen, setCreateOpen] = useState(false)
  const [editWallet, setEditWallet] = useState<Wallet | null>(null)
  const [transferWallet, setTransferWallet] = useState<{ wallet: Wallet; mode: "fund" | "withdraw" } | null>(null)
  const [deleteId, setDeleteId] = useState<string | null>(null)

  async function load() {
    const [w, bal, keys] = await Promise.all([
      api.wallets.list(),
      api.billing.getBalance(),
      api.apiKeys.list().catch(() => null),
    ])
    setWallets(w.wallets)
    setBalance(bal)
    setApiKeys(keys?.data ?? [])
    setLoading(false)
  }

  useEffect(() => { load() }, [api])

  const keysForWallet = (walletId: string) =>
    apiKeys.filter((k) => k.wallet_id === walletId)

  async function handleCreate(name: string, description: string) {
    try {
      await api.wallets.create({ name, description })
      setCreateOpen(false)
      load()
    } catch (e) {
      toast.error("Failed to create wallet", { description: e instanceof Error ? e.message : undefined })
    }
  }

  async function handleUpdate(id: string, name: string, description: string) {
    try {
      await api.wallets.update(id, { name, description })
      setEditWallet(null)
      load()
    } catch (e) {
      toast.error("Failed to update wallet", { description: e instanceof Error ? e.message : undefined })
    }
  }

  async function handleTransfer(amount: number, description: string) {
    if (!transferWallet) return
    const { wallet, mode } = transferWallet
    try {
      const res = mode === "fund"
        ? await api.wallets.fund(wallet.id, { amount, description: description || undefined })
        : await api.wallets.withdraw(wallet.id, { amount, description: description || undefined })
      setBalance(res.main_balance)
      setTransferWallet(null)
      toast.success(mode === "fund" ? "Wallet funded" : "Funds withdrawn")
      load()
    } catch (e) {
      toast.error(mode === "fund" ? "Failed to fund wallet" : "Failed to withdraw", {
        description: e instanceof Error ? e.message : undefined,
      })
    }
  }

  async function handleDelete() {
    if (!deleteId) return
    try {
      await api.wallets.delete(deleteId)
      setDeleteId(null)
      load()
    } catch (e) {
      setDeleteId(null)
      toast.error("Failed to delete wallet", { description: e instanceof Error ? e.message : undefined })
    }
  }

  const totalAllocated = wallets.reduce((sum, w) => sum + w.balance, 0)

  if (loading) {
    return (
      <div className="space-y-6 p-6">
        <PageHeader title="Wallets" description="Split your credits into isolated budgets" />
        <MetricCardSkeleton count={3} />
      </div>
    )
  }

  return (
    <div className="space-y-6 p-6">
      <PageHeader
        title="Wallets"
        description="Split your credits into isolated budgets"
        actions={
          <Button size="sm" onClick={() => setCreateOpen(true)}>
            <Plus className="h-4 w-4 mr-1" />
            New Wallet
          </Button>
        }
      />

      {/* KPIs */}
      <div className="grid grid-cols-1 sm:grid-cols-3 gap-4">
        <MetricCard
          label="Unallocated Balance"
          value={`$${(balance?.balance ?? 0).toFixed(2)}`}
          description="Main balance available to fund wallets"
          icon={Landmark}
        />
        <MetricCard
          label="Allocated to Wallets"
          value={`$${totalAllocated.toFixed(2)}`}
          description={`Across ${wallets.length} wallet${wallets.length === 1 ? "" : "s"}`}
          icon={PiggyBank}
        />
        <MetricCard
          label="Total Credits"
          value={`$${((balance?.balance ?? 0) + totalAllocated).toFixed(2)}`}
          description="Main balance + all wallets"
          icon={WalletIcon}
        />
      </div>

      {/* Wallets table */}
      <Card>
        <CardHeader>
          <CardTitle className="font-mono text-sm">Wallets</CardTitle>
          <CardDescription>
            Fund wallets from your main balance and bind API keys to them for hard budget isolation
          </CardDescription>
        </CardHeader>
        <CardContent>
          {wallets.length === 0 ? (
            <EmptyState
              icon={WalletIcon}
              title="No wallets yet"
              description="Create a wallet to allocate a budget for a customer, team, or environment."
              action={
                <Button size="sm" onClick={() => setCreateOpen(true)}>
                  <Plus className="h-4 w-4 mr-1" />
                  New Wallet
                </Button>
              }
            />
          ) : (
            <Table>
              <TableHeader>
                <TableRow>
                  <TableHead className="font-mono text-xs">Name</TableHead>
                  <TableHead className="font-mono text-xs">Description</TableHead>
                  <TableHead className="font-mono text-xs text-right">Balance</TableHead>
                  <TableHead className="font-mono text-xs">API Keys</TableHead>
                  <TableHead className="font-mono text-xs text-right">Created</TableHead>
                  <TableHead className="w-10" />
                </TableRow>
              </TableHeader>
              <TableBody>
                {wallets.map((w) => {
                  const boundKeys = keysForWallet(w.id)
                  return (
                    <TableRow key={w.id}>
                      <TableCell className="font-mono text-sm">{w.name}</TableCell>
                      <TableCell className="text-sm text-muted-foreground max-w-[240px] truncate">
                        {w.description || "—"}
                      </TableCell>
                      <TableCell className={`text-right font-mono tabular-nums text-sm ${
                        w.balance === 0 ? "text-red-400" : ""
                      }`}>
                        ${w.balance.toFixed(2)}
                      </TableCell>
                      <TableCell>
                        {boundKeys.length > 0 ? (
                          <Badge variant="outline" className="font-mono text-[10px] gap-1">
                            <KeyRound className="h-3 w-3" />
                            {boundKeys.length} key{boundKeys.length === 1 ? "" : "s"}
                          </Badge>
                        ) : (
                          <span className="text-xs text-muted-foreground">unbound</span>
                        )}
                      </TableCell>
                      <TableCell className="text-right text-xs text-muted-foreground whitespace-nowrap">
                        {new Date(w.created_at).toLocaleDateString()}
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
                            <DropdownMenuItem onClick={() => setTransferWallet({ wallet: w, mode: "fund" })}>
                              <ArrowRightCircle className="h-4 w-4 mr-2" /> Fund
                            </DropdownMenuItem>
                            <DropdownMenuItem
                              onClick={() => setTransferWallet({ wallet: w, mode: "withdraw" })}
                              disabled={w.balance === 0}
                            >
                              <ArrowLeftCircle className="h-4 w-4 mr-2" /> Withdraw
                            </DropdownMenuItem>
                            <DropdownMenuItem onClick={() => setEditWallet(w)}>
                              <Pencil className="h-4 w-4 mr-2" /> Edit
                            </DropdownMenuItem>
                            <DropdownMenuItem
                              className="text-destructive"
                              onClick={() => setDeleteId(w.id)}
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
          )}
        </CardContent>
      </Card>

      {/* Create / Edit dialog */}
      <WalletFormDialog
        open={createOpen || !!editWallet}
        wallet={editWallet}
        onOpenChange={(v) => { if (!v) { setCreateOpen(false); setEditWallet(null) } }}
        onSubmit={(name, description) =>
          editWallet ? handleUpdate(editWallet.id, name, description) : handleCreate(name, description)
        }
      />

      {/* Fund / Withdraw dialog */}
      <TransferDialog
        transfer={transferWallet}
        mainBalance={balance?.balance ?? 0}
        onOpenChange={(v) => { if (!v) setTransferWallet(null) }}
        onSubmit={handleTransfer}
      />

      {/* Delete confirm */}
      <ConfirmDialog
        open={!!deleteId}
        onOpenChange={(v) => { if (!v) setDeleteId(null) }}
        title="Delete Wallet"
        description="The wallet must be empty and unbound from all API keys. This cannot be undone."
        onConfirm={handleDelete}
        variant="destructive"
      />
    </div>
  )
}

/* ------------------------------------------------------------------ */
/*  Create / Edit dialog                                               */
/* ------------------------------------------------------------------ */

function WalletFormDialog({
  open,
  wallet,
  onOpenChange,
  onSubmit,
}: {
  open: boolean
  wallet: Wallet | null
  onOpenChange: (v: boolean) => void
  onSubmit: (name: string, description: string) => Promise<void>
}) {
  const [name, setName] = useState("")
  const [description, setDescription] = useState("")
  const [submitting, setSubmitting] = useState(false)

  useEffect(() => {
    if (open) {
      setName(wallet?.name ?? "")
      setDescription(wallet?.description ?? "")
      setSubmitting(false)
    }
  }, [open, wallet])

  async function submit() {
    setSubmitting(true)
    try {
      await onSubmit(name.trim(), description.trim())
    } finally {
      setSubmitting(false)
    }
  }

  return (
    <Dialog open={open} onOpenChange={onOpenChange}>
      <DialogContent>
        <DialogHeader>
          <DialogTitle className="font-mono">{wallet ? "Edit Wallet" : "New Wallet"}</DialogTitle>
          <DialogDescription>
            {wallet
              ? "Rename the wallet or update its description."
              : "A wallet is an isolated budget you fund from your main balance."}
          </DialogDescription>
        </DialogHeader>
        <div className="space-y-4 py-2">
          <div className="space-y-2">
            <Label htmlFor="wallet-name" className="font-mono text-xs">Name</Label>
            <Input
              id="wallet-name"
              value={name}
              onChange={(e) => setName(e.target.value)}
              placeholder="e.g. customer-acme"
            />
          </div>
          <div className="space-y-2">
            <Label htmlFor="wallet-desc" className="font-mono text-xs">Description</Label>
            <Input
              id="wallet-desc"
              value={description}
              onChange={(e) => setDescription(e.target.value)}
              placeholder="e.g. Credits allocated to ACME Corp"
            />
          </div>
        </div>
        <DialogFooter>
          <Button variant="outline" onClick={() => onOpenChange(false)}>Cancel</Button>
          <Button onClick={submit} disabled={!name.trim() || submitting}>
            {submitting
              ? <Loader2 className="h-4 w-4 mr-1 animate-spin" />
              : <Plus className="h-4 w-4 mr-1" />}
            {wallet ? "Save" : "Create"}
          </Button>
        </DialogFooter>
      </DialogContent>
    </Dialog>
  )
}

/* ------------------------------------------------------------------ */
/*  Fund / Withdraw dialog                                             */
/* ------------------------------------------------------------------ */

function TransferDialog({
  transfer,
  mainBalance,
  onOpenChange,
  onSubmit,
}: {
  transfer: { wallet: Wallet; mode: "fund" | "withdraw" } | null
  mainBalance: number
  onOpenChange: (v: boolean) => void
  onSubmit: (amount: number, description: string) => Promise<void>
}) {
  const [amount, setAmount] = useState("")
  const [description, setDescription] = useState("")
  const [submitting, setSubmitting] = useState(false)

  useEffect(() => {
    if (transfer) {
      setAmount("")
      setDescription("")
      setSubmitting(false)
    }
  }, [transfer])

  const isFund = transfer?.mode === "fund"
  const max = isFund ? mainBalance : transfer?.wallet.balance ?? 0
  const amountNum = Number(amount)
  const invalid = !amount || amountNum <= 0 || amountNum > max

  async function submit() {
    setSubmitting(true)
    try {
      await onSubmit(amountNum, description.trim())
    } finally {
      setSubmitting(false)
    }
  }

  return (
    <Dialog open={!!transfer} onOpenChange={onOpenChange}>
      <DialogContent>
        <DialogHeader>
          <DialogTitle className="font-mono">
            {isFund ? "Fund Wallet" : "Withdraw from Wallet"}
          </DialogTitle>
          <DialogDescription>
            {isFund
              ? `Move credits from your main balance into "${transfer?.wallet.name}".`
              : `Return credits from "${transfer?.wallet.name}" to your main balance.`}
          </DialogDescription>
        </DialogHeader>
        <div className="space-y-4 py-2">
          <div className="space-y-2">
            <Label htmlFor="transfer-amount" className="font-mono text-xs">Amount (USD)</Label>
            <Input
              id="transfer-amount"
              type="number"
              step="0.01"
              min="0"
              value={amount}
              onChange={(e) => setAmount(e.target.value)}
              placeholder="e.g. 25"
            />
            <p className="text-xs text-muted-foreground">
              Available: ${max.toFixed(2)} {isFund ? "in main balance" : "in wallet"}
            </p>
          </div>
          <div className="space-y-2">
            <Label htmlFor="transfer-desc" className="font-mono text-xs">Note (optional)</Label>
            <Input
              id="transfer-desc"
              value={description}
              onChange={(e) => setDescription(e.target.value)}
              placeholder="e.g. Q3 budget allocation"
            />
          </div>
        </div>
        <DialogFooter>
          <Button variant="outline" onClick={() => onOpenChange(false)}>Cancel</Button>
          <Button onClick={submit} disabled={invalid || submitting}>
            {submitting
              ? <Loader2 className="h-4 w-4 mr-1 animate-spin" />
              : isFund
                ? <ArrowRightCircle className="h-4 w-4 mr-1" />
                : <ArrowLeftCircle className="h-4 w-4 mr-1" />}
            {isFund ? "Fund" : "Withdraw"}
          </Button>
        </DialogFooter>
      </DialogContent>
    </Dialog>
  )
}
