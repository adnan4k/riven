import { Gauge } from 'lucide-react'
import type { Loaded } from '@/api'
import type { ReportSummary } from '@/types'
import { Link, reportPath } from '@/router'
import { ms, pct, when } from '@/format'
import { Alert, AlertDescription } from '@/components/ui/alert'
import { Badge } from '@/components/ui/badge'
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from '@/components/ui/card'
import { Skeleton } from '@/components/ui/skeleton'
import { Table, TableBody, TableCell, TableHead, TableHeader, TableRow } from '@/components/ui/table'

export function ReportsView({ reports }: { reports: Loaded<ReportSummary[]> }) {
  if (reports.state === 'loading') return <Skeleton className="h-64 w-full" />
  if (reports.state === 'error') return <Alert variant="destructive"><AlertDescription>{reports.message}</AlertDescription></Alert>

  if (reports.data.length === 0) {
    return (
      <Card className="mx-auto w-full max-w-xl text-center">
        <CardHeader>
          <Gauge className="mx-auto size-8 text-muted-foreground" />
          <CardTitle>No stress reports yet</CardTitle>
          <CardDescription>Run a stress test with <code className="font-mono">--save</code>, then reload this page.</CardDescription>
        </CardHeader>
        <CardContent>
          <pre className="rounded-md border bg-muted px-4 py-3 text-left font-mono text-sm">riven-engine stress -d 30s -c 10 --save</pre>
          <p className="mt-3 text-xs text-muted-foreground">Reports are read from <code className="font-mono">riven-reports/</code> (change it with <code className="font-mono">serve --reports</code>).</p>
        </CardContent>
      </Card>
    )
  }

  return (
    <Card className="py-0">
      <CardContent className="px-0">
        <Table>
          <TableHeader>
            <TableRow>
              <TableHead className="pl-4">Run</TableHead><TableHead>Target</TableHead>
              <TableHead className="text-right">Workers</TableHead><TableHead className="text-right">Requests</TableHead>
              <TableHead className="text-right">Req/s</TableHead><TableHead className="text-right">p50</TableHead>
              <TableHead className="text-right">p95</TableHead><TableHead className="text-right">p99</TableHead>
              <TableHead className="pr-4 text-right">Failed</TableHead>
            </TableRow>
          </TableHeader>
          <TableBody>
            {reports.data.map((r) => (
              <TableRow key={r.name}>
                <TableCell className="pl-4 font-medium">
                  <Link to={reportPath(r.name)} className="hover:underline">{when(r.started_at)}</Link>
                  {r.interrupted && <Badge variant="outline" className="ml-2">interrupted</Badge>}
                </TableCell>
                <TableCell className="font-mono text-xs">{r.config.base_url} <span className="text-muted-foreground">· {r.routes} routes</span></TableCell>
                <TableCell className="text-right tabular-nums">{r.config.concurrency}</TableCell>
                <TableCell className="text-right tabular-nums">{r.summary.requests}</TableCell>
                <TableCell className="text-right tabular-nums">{r.summary.rps.toFixed(1)}</TableCell>
                <TableCell className="text-right tabular-nums">{ms(r.summary.latency_ms?.p50)}</TableCell>
                <TableCell className="text-right tabular-nums">{ms(r.summary.latency_ms?.p95)}</TableCell>
                <TableCell className="text-right tabular-nums">{ms(r.summary.latency_ms?.p99)}</TableCell>
                <TableCell className={`pr-4 text-right tabular-nums ${r.summary.error_rate > 0 ? 'font-medium text-destructive' : ''}`}>{pct(r.summary.error_rate)}</TableCell>
              </TableRow>
            ))}
          </TableBody>
        </Table>
      </CardContent>
    </Card>
  )
}
