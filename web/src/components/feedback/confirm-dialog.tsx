import {
  Dialog, DialogContent, DialogDescription, DialogFooter,
  DialogHeader, DialogTitle, DialogTrigger,
} from "@/components/ui/dialog"
import { Button } from "@/components/ui/button"

export function ConfirmDialog({
  trigger,
  title,
  description,
  confirmLabel = "Confirm",
  cancelLabel = "Cancel",
  variant = "destructive",
  onConfirm,
}: {
  trigger: React.ReactNode
  title: string
  description: string
  confirmLabel?: string
  cancelLabel?: string
  variant?: "default" | "destructive"
  onConfirm: () => void
}) {
  return (
    <Dialog>
      <DialogTrigger>{trigger}</DialogTrigger>
      <DialogContent>
        <DialogHeader>
          <DialogTitle className="font-mono">{title}</DialogTitle>
          <DialogDescription>{description}</DialogDescription>
        </DialogHeader>
        <DialogFooter>
          <Button variant="outline">{cancelLabel}</Button>
          <Button variant={variant} onClick={onConfirm}>{confirmLabel}</Button>
        </DialogFooter>
      </DialogContent>
    </Dialog>
  )
}
