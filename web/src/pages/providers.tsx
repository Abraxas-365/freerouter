import { useEffect, useState } from "react"
import {
  Server, ExternalLink, Zap,
} from "lucide-react"
import { useApi } from "@/api"
import type { Provider } from "@/api/types"
import { PageHeader, EmptyState, StatusBadge } from "@/components"
import { MetricCardSkeleton } from "@/components/feedback/skeletons"
import { Badge } from "@/components/ui/badge"
import { Card, CardContent } from "@/components/ui/card"
import {
  Table, TableBody, TableCell, TableHead, TableHeader, TableRow,
} from "@/components/ui/table"

export default function ProvidersPage() {
  const api = useApi()
  const [providers, setProviders] = useState<Provider[]>([])
  const [loading, setLoading] = useState(true)

  async function load() {
    const res = await api.providers.list()
    setProviders(res.data)
    setLoading(false)
  }

  useEffect(() => { load() }, [api])





  if (loading) {
    return (
      <div className="space-y-6 p-6">
        <PageHeader title="Providers" description="Browse the operator-managed provider catalog" />
        <MetricCardSkeleton count={3} />
      </div>
    )
  }

  const active = providers.filter((p) => p.status === "active").length
  const inactive = providers.length - active

  return (
    <div className="space-y-6 p-6">
      <PageHeader
        title="Providers"
        description="Browse the operator-managed provider catalog"
      />

      {/* Summary */}
      <div className="flex items-center gap-4">
        <Badge variant="outline" className="font-mono text-xs gap-1.5">
          <span className="h-1.5 w-1.5 rounded-full bg-success" />
          {active} active
        </Badge>
        {inactive > 0 && (
          <Badge variant="outline" className="font-mono text-xs gap-1.5">
            <span className="h-1.5 w-1.5 rounded-full bg-muted-foreground" />
            {inactive} inactive
          </Badge>
        )}
        <span className="text-xs text-muted-foreground font-mono">{providers.length} total</span>
      </div>

      {providers.length === 0 ? (
        <EmptyState
          icon={Server}
          title="No providers"
          description="No providers have been configured by the operator."
        />
      ) : (
        <Card>
          <CardContent className="p-0">
            <Table>
              <TableHeader>
                <TableRow>
                  <TableHead className="font-mono text-xs">Name</TableHead>
                  <TableHead className="font-mono text-xs">Description</TableHead>
                  <TableHead className="font-mono text-xs">Status</TableHead>
                  <TableHead className="font-mono text-xs">Streaming</TableHead>
                  <TableHead className="font-mono text-xs">Created</TableHead>
                  <TableHead className="font-mono text-xs w-[50px]" />
                </TableRow>
              </TableHeader>
              <TableBody>
                {providers.map((provider) => (
                  <TableRow key={provider.id}>
                    <TableCell>
                      <div className="flex items-center gap-2">
                        <span className="font-mono text-sm font-medium">{provider.name}</span>
                        {provider.website && (
                          <a
                            href={provider.website}
                            target="_blank"
                            rel="noopener noreferrer"
                            className="text-muted-foreground hover:text-foreground transition-colors"
                          >
                            <ExternalLink className="h-3 w-3" />
                          </a>
                        )}
                      </div>
                    </TableCell>
                    <TableCell className="text-sm text-muted-foreground max-w-[300px] truncate">
                      {provider.description}
                    </TableCell>
                    <TableCell>
                      <StatusBadge
                        status={provider.status === "active" ? "online" : "offline"}
                        label={provider.status.toUpperCase()}
                      />
                    </TableCell>
                    <TableCell>
                      {provider.streaming ? (
                        <Badge variant="secondary" className="font-mono text-[10px] gap-1">
                          <Zap className="h-3 w-3" /> stream
                        </Badge>
                      ) : (
                        <span className="text-xs text-muted-foreground">-</span>
                      )}
                    </TableCell>
                    <TableCell className="font-mono text-xs text-muted-foreground tabular-nums">
                      {new Date(provider.created_at).toLocaleDateString()}
                    </TableCell>
                    <TableCell>

                    </TableCell>
                  </TableRow>
                ))}
              </TableBody>
            </Table>
          </CardContent>
        </Card>
      )}


    </div>
  )
}
