import { useState } from "react"
import { Badge } from "@/components/ui/badge"
import { Button } from "@/components/ui/button"
import { Tooltip, TooltipContent, TooltipTrigger } from "@/components/ui/tooltip"
import { Copy, Eye, EyeOff, Check } from "lucide-react"
import { StatusBadge, type StatusLevel } from "@/components/data/status"
import { cn } from "@/lib/utils"

export function ApiKeyDisplay({
  keyValue,
  prefix = "fr_live_",
  scopes,
  status = "online",
  lastUsed,
  className,
}: {
  keyValue: string
  prefix?: string
  scopes?: string[]
  status?: StatusLevel
  lastUsed?: string
  className?: string
}) {
  const [revealed, setRevealed] = useState(false)
  const [copied, setCopied] = useState(false)

  const masked = prefix + keyValue.slice(0, 4) + "\u2022".repeat(24)
  const full = prefix + keyValue

  const handleCopy = async () => {
    await navigator.clipboard.writeText(full)
    setCopied(true)
    setTimeout(() => setCopied(false), 2000)
  }

  return (
    <div className={cn("space-y-3", className)}>
      <div className="flex items-center gap-2">
        <div className="flex-1 bg-muted rounded-sm px-3 py-2 font-mono text-sm truncate">
          {revealed ? full : masked}
        </div>
        <Tooltip>
          <TooltipTrigger>
            <Button variant="ghost" size="icon" className="h-8 w-8" onClick={() => setRevealed(!revealed)}>
              {revealed ? <EyeOff className="h-4 w-4" /> : <Eye className="h-4 w-4" />}
            </Button>
          </TooltipTrigger>
          <TooltipContent>{revealed ? "Hide" : "Reveal"}</TooltipContent>
        </Tooltip>
        <Tooltip>
          <TooltipTrigger>
            <Button variant="ghost" size="icon" className="h-8 w-8" onClick={handleCopy}>
              {copied ? <Check className="h-4 w-4 text-success" /> : <Copy className="h-4 w-4" />}
            </Button>
          </TooltipTrigger>
          <TooltipContent>{copied ? "Copied!" : "Copy"}</TooltipContent>
        </Tooltip>
      </div>

      {scopes && scopes.length > 0 && (
        <div className="flex flex-wrap gap-1.5">
          {scopes.map((s) => (
            <Badge key={s} variant="secondary" className="font-mono text-xs">{s}</Badge>
          ))}
        </div>
      )}

      {(lastUsed || status) && (
        <div className="flex items-center justify-between text-xs text-muted-foreground font-mono">
          {lastUsed && <span>Last used: {lastUsed}</span>}
          <StatusBadge status={status} />
        </div>
      )}
    </div>
  )
}
