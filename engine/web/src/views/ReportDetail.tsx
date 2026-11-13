import { TriangleAlert } from 'lucide-react'
import { useJSON } from '@/api'
import type { Stats, StressReport } from '@/types'
import { Link, routePath } from '@/router'
import { ms, pct, when } from '@/format'
import { Status } from '@/components/Method'
import { Stat, StatGrid } from '@/components/Stat'
import { Section } from './Section'
import { Alert, AlertDescription } from '@/components/ui/alert'
import { Skeleton } from '@/components/ui/skeleton'
import { Table, TableBody, TableCell, TableHead, TableHeader, TableRow } from '@/components/ui/table'

export function ReportDetail({ name }: { name: string }) {
  const rep = useJSON<StressReport>('/api/reports/' + encodeURIComponent(name))
  if (rep.state === 'loading') return <Skeleton className="h-64 w-full" />
  if (rep.state === 'error') return <Alert variant="destructive"><AlertDescription>{rep.message}</AlertDescription></Alert>

  const r = rep.data
  // One scale for every bar: the slowest p99 in the run.
  const scale = Math.max(1, ...r.routes.map((x) => x.latency_ms?.p99 ?? 0))
  const limit = [r.config.requests ? `${r.config.requests} requests` : '', r.config.duration_ms ? `${r.config.duration_ms / 1000}s` : '']
    .filter(Boolean).join(' or ')
  const s = r.summary

  return (
    <>
      <div className="flex flex-col gap-3">
        <p className="text-sm text-muted-foreground"><Link to="/reports" className="hover:underline">Stress reports</Link> / {when(r.started_at)}</p>
        <h2 className="text-2xl font-semibold tracking-tight">Stress run</h2>
        <dl className="flex flex-wrap gap-x-8 gap-y-2 text-sm">
          <Fact label="Target" value={r.config.base_url} mono />
          <Fact label="Workers" value={String(r.config.concurrency)} />
          <Fact label="Limit" value={limit} />
          <Fact label="Manifest exported" value={when(r.manifest.generated_at)} />
        </dl>
      </div>

      {r.interrupted && <Alert><TriangleAlert /><AlertDescription>This run was stopped early with Ctrl-C.</AlertDescription></Alert>}

      <StatGrid>
        <Stat label={`Requests in ${ms(s.elapsed_ms)}`} value={s.requests} />
        <Stat label="Req/s" value={s.rps.toFixed(1)} />
        <Stat label="p50" value={ms(s.latency_ms?.p50)} />
        <Stat label="p95" value={ms(s.latency_ms?.p95)} />
        <Stat label="p99" value={ms(s.latency_ms?.p99)} />
        <Stat label="Failed" value={pct(s.error_rate)} tone={s.error_rate > 0 ? 'bad' : undefined} />
      </StatGrid>

      <Section
        title="Per route"
        description="Failed = transport errors + 5xx. Latency only counts requests that got a response."
        action={<Legend scale={scale} />}
        flush
      >
        <Table>
          <TableHeader>
            <TableRow>
              <TableHead className="pl-4">Route</TableHead><TableHead className="text-right">Req/s</TableHead>
              <TableHead className="w-1/3 min-w-40">Latency</TableHead><TableHead className="text-right">p95</TableHead>
              <TableHead className="text-right">Failed</TableHead><TableHead className="pr-4">Statuses</TableHead>
            </TableRow>
          </TableHeader>
          <TableBody>
            {r.routes.map((x) => (
              <TableRow key={x.route_id}>
                <TableCell className="pl-4 font-mono"><Link to={routePath(x.route_id)} className="hover:underline">{x.route_id}</Link></TableCell>
                <TableCell className="text-right tabular-nums">{x.rps.toFixed(1)}</TableCell>
                <TableCell><LatencyBar s={x} scale={scale} /></TableCell>
                <TableCell className="text-right tabular-nums">{ms(x.latency_ms?.p95)}</TableCell>
                <TableCell className={`text-right tabular-nums ${x.error_rate > 0 ? 'font-medium text-destructive' : ''}`}>{pct(x.error_rate)}</TableCell>
                <TableCell className="pr-4">
                  <span className="flex flex-wrap gap-1">
                    {Object.entries(x.statuses).map(([code, n]) => <Status key={code} code={code} count={n} />)}
                    {Object.entries(x.errors).map(([kind, n]) => <Status key={kind} code={kind} count={n} />)}
                  </span>
                </TableCell>
              </TableRow>
            ))}
          </TableBody>
        </Table>
      </Section>
    </>
  )
}

function Fact({ label, value, mono }: { label: string; value: string; mono?: boolean }) {
  return (
    <div className="min-w-0">
      <dt className="label-mono">{label}</dt>
      <dd className={mono ? 'font-mono break-all' : undefined}>{value}</dd>
    </div>
  )
}

function Legend({ scale }: { scale: number }) {
  return (
    <div className="flex flex-wrap items-center gap-3 text-xs text-muted-foreground">
      <span className="flex items-center gap-1"><i className="size-2.5 rounded-sm bg-foreground" />p50</span>
      <span className="flex items-center gap-1"><i className="size-2.5 rounded-sm bg-foreground/45" />p95</span>
      <span className="flex items-center gap-1"><i className="size-2.5 rounded-sm bg-foreground/15" />p99</span>
      <span>0 – {ms(scale)}</span>
    </div>
  )
}

function LatencyBar({ s, scale }: { s: Stats; scale: number }) {
  const l = s.latency_ms
  if (!l) return <span className="text-xs text-muted-foreground">no responses</span>
  const w = (v: number) => `${Math.min(100, (v / scale) * 100)}%`
  return (
    <div className="relative h-2.5 rounded-full bg-muted" role="img" aria-label={`p50 ${ms(l.p50)}, p95 ${ms(l.p95)}, p99 ${ms(l.p99)}`}>
      <span className="absolute inset-y-0 left-0 rounded-full bg-foreground/15" style={{ width: w(l.p99) }} />
      <span className="absolute inset-y-0 left-0 rounded-full bg-foreground/45" style={{ width: w(l.p95) }} />
      <span className="absolute inset-y-0 left-0 rounded-full bg-foreground" style={{ width: w(l.p50) }} />
    </div>
  )
}
