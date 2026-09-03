import type { LucideIcon } from "lucide-react"
import { Card, CardContent, CardHeader, CardDescription } from "@/components/ui/card"
import { cn } from "@/lib/utils"

export function MetricCard({
  label,
  value,
  description,
  icon: Icon,
  descriptionClassName,
  className,
}: {
  label: string
  value: string | number
  description?: string
  icon?: LucideIcon
  descriptionClassName?: string
  className?: string
}) {
  return (
    <Card className={cn("glow-box", className)}>
      <CardHeader className="flex flex-row items-center justify-between pb-2">
        <CardDescription className="font-mono text-xs uppercase tracking-wider">
          {label}
        </CardDescription>
        {Icon && <Icon className="h-4 w-4 text-muted-foreground" />}
      </CardHeader>
      <CardContent>
        <p className="text-2xl font-mono font-semibold tabular-nums">{value}</p>
        {description && (
          <p className={cn("text-xs mt-1 font-mono", descriptionClassName ?? "text-muted-foreground")}>
            {description}
          </p>
        )}
      </CardContent>
    </Card>
  )
}
