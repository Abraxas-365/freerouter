import { Card, CardContent, CardDescription, CardHeader, CardTitle } from "@/components/ui/card"
import {
  AreaChart, Area, BarChart, Bar, XAxis, YAxis, CartesianGrid,
  Tooltip, ResponsiveContainer, PieChart, Pie, Cell,
} from "recharts"

const GRID_COLOR = "#222220"
const AXIS_COLOR = "#6b675e"
const MONO_FONT = "JetBrains Mono Variable"

const tooltipStyle = {
  backgroundColor: "#131311",
  border: "1px solid #222220",
  borderRadius: 4,
  fontFamily: MONO_FONT,
  fontSize: 12,
}

export const CHART_COLORS = ["#7b9bae", "#5a7a4a", "#b8a06a", "#a65d4e", "#8a7b9b"]

export function ChartCard({
  title,
  description,
  children,
  className,
}: {
  title: string
  description?: string
  children: React.ReactNode
  className?: string
}) {
  return (
    <Card className={className}>
      <CardHeader>
        <CardTitle className="font-mono text-sm">{title}</CardTitle>
        {description && <CardDescription>{description}</CardDescription>}
      </CardHeader>
      <CardContent>{children}</CardContent>
    </Card>
  )
}

export function AreaChartView({
  data,
  xKey,
  yKey,
  color = CHART_COLORS[0],
  height = 220,
}: {
  data: Record<string, unknown>[]
  xKey: string
  yKey: string
  color?: string
  height?: number
}) {
  const gradientId = `fill-${yKey}`
  return (
    <ResponsiveContainer width="100%" height={height}>
      <AreaChart data={data}>
        <defs>
          <linearGradient id={gradientId} x1="0" y1="0" x2="0" y2="1">
            <stop offset="0%" stopColor={color} stopOpacity={0.3} />
            <stop offset="100%" stopColor={color} stopOpacity={0} />
          </linearGradient>
        </defs>
        <CartesianGrid strokeDasharray="3 3" stroke={GRID_COLOR} />
        <XAxis dataKey={xKey} stroke={AXIS_COLOR} fontSize={12} fontFamily={MONO_FONT} />
        <YAxis stroke={AXIS_COLOR} fontSize={12} fontFamily={MONO_FONT} />
        <Tooltip contentStyle={tooltipStyle} labelStyle={{ color: "#d4d0c8" }} />
        <Area type="monotone" dataKey={yKey} stroke={color} fill={`url(#${gradientId})`} strokeWidth={2} />
      </AreaChart>
    </ResponsiveContainer>
  )
}

export function HBarChartView({
  data,
  categoryKey,
  valueKey,
  color = CHART_COLORS[0],
  height = 220,
  formatValue,
}: {
  data: Record<string, unknown>[]
  categoryKey: string
  valueKey: string
  color?: string
  height?: number
  formatValue?: (v: number) => string
}) {
  const fmt = formatValue ?? ((v: number) => String(v))
  return (
    <ResponsiveContainer width="100%" height={height}>
      <BarChart data={data} layout="vertical">
        <CartesianGrid strokeDasharray="3 3" stroke={GRID_COLOR} horizontal={false} />
        <XAxis type="number" stroke={AXIS_COLOR} fontSize={12} fontFamily={MONO_FONT} tickFormatter={(v) => fmt(v)} />
        <YAxis type="category" dataKey={categoryKey} stroke={AXIS_COLOR} fontSize={11} fontFamily={MONO_FONT} width={90} />
        <Tooltip contentStyle={tooltipStyle} formatter={(value) => [fmt(Number(value)), valueKey]} />
        <Bar dataKey={valueKey} fill={color} radius={[0, 2, 2, 0]} />
      </BarChart>
    </ResponsiveContainer>
  )
}

export function DonutChartView({
  data,
  nameKey,
  valueKey,
  colors = CHART_COLORS,
  size = 160,
}: {
  data: Record<string, unknown>[]
  nameKey: string
  valueKey: string
  colors?: string[]
  size?: number
}) {
  return (
    <div className="flex items-center gap-6">
      <ResponsiveContainer width={size} height={size}>
        <PieChart>
          <Pie
            data={data}
            dataKey={valueKey}
            nameKey={nameKey}
            cx="50%"
            cy="50%"
            innerRadius={size * 0.28}
            outerRadius={size * 0.44}
            strokeWidth={0}
          >
            {data.map((_, i) => (
              <Cell key={i} fill={colors[i % colors.length]} />
            ))}
          </Pie>
        </PieChart>
      </ResponsiveContainer>
      <div className="space-y-2">
        {data.map((d, i) => (
          <div key={String(d[nameKey])} className="flex items-center gap-2 text-sm">
            <span className="h-2.5 w-2.5 rounded-sm shrink-0" style={{ backgroundColor: colors[i % colors.length] }} />
            <span className="text-muted-foreground">{String(d[nameKey])}</span>
            <span className="font-mono text-xs ml-auto tabular-nums">{String(d[valueKey])}%</span>
          </div>
        ))}
      </div>
    </div>
  )
}
