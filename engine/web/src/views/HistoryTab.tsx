import { useEffect, useState } from 'react'
import { getJSON } from '@/api'
import type { ReportSummary, Route, RouteStats, StressReport } from '@/types'
import { ms, pct, when } from '@/format'
import { Link, reportPath } from '@/router'
import { Skeleton } from '@/components/ui/skeleton'
import { Table, TableBody, TableCell, TableHead, TableHeader, TableRow } from '@/components/ui/table'

interface Row { report: ReportSummary; full: StressReport; stats: RouteStats }

// Every saved run that included this route, newest first.
export function HistoryTab({ route, reports }: { route: Route; reports: ReportSummary[] }) {
  const [rows, setRows] = useState<Row[] | null>(null)

  useEffect(() => {
    let cancelled = false
    Promise.all(reports.map((r) => getJSON<StressReport>('/api/reports/' + encodeURIComponent(r.name)).then((full) => ({ report: r, full }))))
      .then((all) => {
        if (cancelled) return
        setRows(all.flatMap(({ report, full }) => {
          const stats = full.routes.find((x) => x.route_id === route.id)
          return stats ? [{ report, full, stats }] : []
        }))
      })
      .catch(() => !cancelled && setRows([]))
    return () => { cancelled = true }
  }, [route.id, reports])

  if (rows === null) return <Skeleton className="h-40 w-full" />
  if (rows.length === 0) {
    return <p className="text-sm text-muted-foreground">No saved stress runs include this route yet. Runs from the Stress test tab are saved automatically.</p>
  }

  return (
    <div className="overflow-hidden rounded-md border">
      <Table>
        <TableHeader>
          <TableRow>
            <TableHead className="pl-4">Run</TableHead><TableHead className="text-right">Users</TableHead>
            <TableHead className="text-right">Requests</TableHead><TableHead className="text-right">Req/s</TableHead>
            <TableHead className="text-right">p50</TableHead><TableHead className="text-right">p95</TableHead>
            <TableHead className="text-right">p99</TableHead><TableHead className="pr-4 text-right">Failed</TableHead>
          </TableRow>
        </TableHeader>
        <TableBody>
          {rows.map(({ report, stats }) => (
            <TableRow key={report.name}>
              <TableCell className="pl-4">
                <Link to={reportPath(report.name)} className="hover:underline">{when(report.started_at)}</Link>
                {report.interrupted && <span className="ml-2 text-xs text-muted-foreground">stopped early</span>}
              </TableCell>
              <TableCell className="text-right font-mono">{report.config.concurrency}</TableCell>
              <TableCell className="text-right font-mono">{stats.requests}</TableCell>
              <TableCell className="text-right font-mono">{stats.rps.toFixed(1)}</TableCell>
              <TableCell className="text-right font-mono">{ms(stats.latency_ms?.p50)}</TableCell>
              <TableCell className="text-right font-mono">{ms(stats.latency_ms?.p95)}</TableCell>
              <TableCell className="text-right font-mono">{ms(stats.latency_ms?.p99)}</TableCell>
              <TableCell className={`pr-4 text-right font-mono ${stats.error_rate > 0 ? 'text-red-600 dark:text-red-400' : ''}`}>{pct(stats.error_rate)}</TableCell>
            </TableRow>
          ))}
        </TableBody>
      </Table>
    </div>
  )
}
