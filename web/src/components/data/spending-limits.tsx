import { Card, CardContent, CardDescription, CardHeader, CardTitle } from "@/components/ui/card"
import { Progress } from "@/components/ui/progress"
import { cn } from "@/lib/utils"

interface LimitItem {
  label: string
  current: number
  max: number
  unit?: string
}

export function SpendingLimits({
  items,
  className,
}: {
  items: LimitItem[]
  className?: string
}) {
  return (
    <Card className={className}>
      <CardHeader>
        <CardTitle className="font-mono text-sm">Spending Limits</CardTitle>
        <CardDescription>Resource usage and caps</CardDescription>
      </CardHeader>
      <CardContent className="space-y-5">
        {items.map((item) => {
          const pct = (item.current / item.max) * 100
          return (
            <div key={item.label}>
              <div className="flex justify-between text-sm mb-2">
                <span className="font-mono text-xs text-muted-foreground">{item.label}</span>
                <span className={cn(
                  "font-mono text-xs tabular-nums",
                  pct > 90 && "text-destructive",
                  pct > 70 && pct <= 90 && "text-warning",
                )}>
                  {item.unit === "$" ? `$${item.current.toFixed(2)} / $${item.max.toFixed(2)}` : `${item.current} / ${item.max}${item.unit ? ` ${item.unit}` : ""}`}
                </span>
              </div>
              <Progress value={pct} />
            </div>
          )
        })}
      </CardContent>
    </Card>
  )
}
