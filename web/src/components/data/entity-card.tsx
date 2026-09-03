import { Card, CardContent, CardDescription, CardHeader, CardTitle } from "@/components/ui/card"
import { ChevronRight } from "lucide-react"
import { cn } from "@/lib/utils"

interface KeyValue {
  label: string
  value: React.ReactNode
}

export function EntityCard({
  title,
  description,
  rows,
  onClick,
  className,
}: {
  title: string
  description?: string
  rows?: KeyValue[]
  onClick?: () => void
  className?: string
}) {
  const interactive = !!onClick
  return (
    <Card
      className={cn(
        interactive && "hover:border-primary/30 transition-colors cursor-pointer group",
        className,
      )}
      onClick={onClick}
    >
      <CardHeader className="pb-3">
        <div className="flex items-center justify-between">
          <CardTitle className="font-mono text-sm">{title}</CardTitle>
          {interactive && (
            <ChevronRight className="h-4 w-4 text-muted-foreground group-hover:text-foreground transition-colors" />
          )}
        </div>
        {description && <CardDescription>{description}</CardDescription>}
      </CardHeader>
      {rows && rows.length > 0 && (
        <CardContent className="space-y-2">
          {rows.map((row) => (
            <div key={row.label} className="flex items-center justify-between text-xs">
              <span className="text-muted-foreground font-mono">{row.label}</span>
              <span className="font-mono tabular-nums">{row.value}</span>
            </div>
          ))}
        </CardContent>
      )}
    </Card>
  )
}
