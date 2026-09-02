import type { LucideIcon } from "lucide-react"
import { Badge } from "@/components/ui/badge"
import { cn } from "@/lib/utils"

// ── Status Badge ──
// Usage: <StatusBadge status="online" />

type StatusLevel = "online" | "degraded" | "down" | "offline"

const STATUS_CONFIG: Record<StatusLevel, { label: string; color: string }> = {
  online:   { label: "ONLINE",   color: "bg-success" },
  degraded: { label: "DEGRADED", color: "bg-warning" },
  down:     { label: "DOWN",     color: "bg-destructive" },
  offline:  { label: "OFFLINE",  color: "bg-muted-foreground" },
}

export function StatusBadge({
  status,
  label,
  className,
}: {
  status: StatusLevel
  label?: string
  className?: string
}) {
  const cfg = STATUS_CONFIG[status]
  return (
    <Badge variant="outline" className={cn("gap-1.5 font-mono text-xs", className)}>
      <span className={cn("h-1.5 w-1.5 rounded-full", cfg.color)} />
      {label ?? cfg.label}
    </Badge>
  )
}

// ── Capability Badge ──
// Usage: <CapabilityBadge>vision</CapabilityBadge>

export function CapabilityBadge({
  children,
  className,
}: {
  children: React.ReactNode
  className?: string
}) {
  return (
    <Badge variant="secondary" className={cn("font-mono text-[10px] px-1.5", className)}>
      {children}
    </Badge>
  )
}

// ── Status Icon ──
// Usage: <StatusIcon icon={Check} variant="success" />

type StatusVariant = "success" | "error" | "warning" | "info" | "neutral"

const VARIANT_STYLES: Record<StatusVariant, { bg: string; fg: string }> = {
  success: { bg: "bg-success/10", fg: "text-success" },
  error:   { bg: "bg-destructive/10", fg: "text-destructive" },
  warning: { bg: "bg-warning/10", fg: "text-warning" },
  info:    { bg: "bg-primary/10", fg: "text-primary" },
  neutral: { bg: "bg-muted", fg: "text-muted-foreground" },
}

export function StatusIcon({
  icon: Icon,
  variant = "neutral",
  size = "md",
  className,
}: {
  icon: LucideIcon
  variant?: StatusVariant
  size?: "sm" | "md"
  className?: string
}) {
  const styles = VARIANT_STYLES[variant]
  const dims = size === "sm" ? "h-6 w-6" : "h-8 w-8"
  const iconSize = size === "sm" ? "h-3 w-3" : "h-4 w-4"
  return (
    <div className={cn(dims, "rounded-sm flex items-center justify-center", styles.bg, className)}>
      <Icon className={cn(iconSize, styles.fg)} />
    </div>
  )
}

export type { StatusLevel, StatusVariant }
