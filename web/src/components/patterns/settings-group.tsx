import { Switch } from "@/components/ui/switch"
import { Card, CardContent } from "@/components/ui/card"
import { cn } from "@/lib/utils"

interface SettingToggle {
  id: string
  label: string
  description?: string
  checked: boolean
  onCheckedChange: (checked: boolean) => void
}

export function SettingsGroup({
  items,
  className,
}: {
  items: SettingToggle[]
  className?: string
}) {
  return (
    <Card className={cn(className)}>
      <CardContent className="pt-6 space-y-4">
        {items.map((item) => (
          <div key={item.id} className="flex items-center justify-between">
            <div>
              <p className="text-sm font-medium">{item.label}</p>
              {item.description && (
                <p className="text-xs text-muted-foreground">{item.description}</p>
              )}
            </div>
            <Switch checked={item.checked} onCheckedChange={item.onCheckedChange} />
          </div>
        ))}
      </CardContent>
    </Card>
  )
}
