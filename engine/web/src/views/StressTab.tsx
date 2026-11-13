import { useEffect, useState } from 'react'
import { Loader2, Play, Square, TriangleAlert } from 'lucide-react'
import { getJSON, postJSON } from '@/api'
import type { Route, StressState } from '@/types'
import { clock, ms, pct } from '@/format'
import { mainMethod } from '@/lib/routes'
import { Link, reportPath } from '@/router'
import { cn } from '@/lib/utils'
import { ChartLegend, LatencyChart } from '@/components/LatencyChart'
import { StatusBar } from '@/components/StatusBar'
import { Alert, AlertDescription, AlertTitle } from '@/components/ui/alert'
import { Button } from '@/components/ui/button'
import { Card, CardContent, CardHeader, CardTitle } from '@/components/ui/card'
import { Input } from '@/components/ui/input'
import { Label } from '@/components/ui/label'
import { ToggleGroup, ToggleGroupItem } from '@/components/ui/toggle-group'

// Presets keep the load constant (no ramp-up yet): they differ in users and length.
const PRESETS = {
  smoke: { label: 'Smoke', users: 2, duration: 15 },
  load: { label: 'Load', users: 25, duration: 60 },
  spike: { label: 'Spike', users: 100, duration: 30 },
  soak: { label: 'Soak', users: 20, duration: 600 },
} as const
type Preset = keyof typeof PRESETS

export function StressTab({ route, params, setParams, token, onRunSaved }: {
  route: Route
  params: Record<string, string>
  setParams: (p: Record<string, string>) => void
  token: string
  onRunSaved: () => void
}) {
  const [preset, setPreset] = useState<Preset | ''>('smoke')
  const [users, setUsers] = useState(String(PRESETS.smoke.users))
  const [duration, setDuration] = useState(String(PRESETS.smoke.duration))
  const [threshold, setThreshold] = useState('400')
  const [pool, setPool] = useState('50') // generated bodies to rotate through; '' = off
  const [state, setState] = useState<StressState | null>(null)
  const [error, setError] = useState<string | null>(null)

  const method = mainMethod(route)
  const writes = method !== 'GET'
  const canGenerate = writes && method !== 'DELETE' && !!route.request.form_request
  const missing = route.parameters.filter((p) => p.required && !params[p.name]).map((p) => p.name)
  const running = state?.state === 'running'
  const live = state?.live
  const mine = !!live?.routes.some((r) => r.route_id === route.id)
  const limit = Number(threshold) || undefined

  // Poll while a run is going; once on mount to pick up a run already in progress.
  useEffect(() => {
    let timer: ReturnType<typeof setTimeout>
    let stopped = false
    const tick = async () => {
      try {
        const s = await getJSON<StressState>('/api/stress')
        if (stopped) return
        setState((prev) => {
          if (prev?.state === 'running' && s.state === 'done') onRunSaved()
          return s
        })
        if (s.state === 'running') timer = setTimeout(tick, 500)
      } catch (e) {
        if (!stopped) setError((e as Error).message)
      }
    }
    tick()
    return () => { stopped = true; clearTimeout(timer) }
  }, [state?.id, state?.state === 'running']) // eslint-disable-line react-hooks/exhaustive-deps

  function choose(p: string) {
    if (!p) return
    setPreset(p as Preset)
    setUsers(String(PRESETS[p as Preset].users))
    setDuration(String(PRESETS[p as Preset].duration))
  }

  async function start() {
    setError(null)
    try {
      const s = await postJSON<StressState>('/api/stress', {
        routes: [route.id], params, token, concurrency: Number(users), duration_s: Number(duration),
        fake_bodies: canGenerate ? Number(pool) || 0 : 0,
      })
      setState(s)
    } catch (e) {
      setError((e as Error).message)
    }
  }

  async function stop() {
    try { await postJSON('/api/stress/stop', {}) } catch (e) { setError((e as Error).message) }
  }

  const s = mine ? live!.summary : null
  const peakRps = mine ? Math.max(0, ...live!.timeline.map((p) => p.requests)) : 0
  const overThreshold = !!(s?.latency_ms && limit && s.latency_ms.p95 > limit)

  return (
    <div className="flex flex-col gap-6">
      <Card>
        <CardHeader className="flex flex-row flex-wrap items-center justify-between gap-3">
          <CardTitle>Load profile</CardTitle>
          <ToggleGroup type="single" variant="outline" size="sm" value={preset} onValueChange={choose} aria-label="Preset">
            {Object.entries(PRESETS).map(([k, p]) => <ToggleGroupItem key={k} value={k}>{p.label}</ToggleGroupItem>)}
          </ToggleGroup>
        </CardHeader>
        <CardContent className="flex flex-col gap-4">
          <div className="grid gap-4 sm:grid-cols-2 lg:grid-cols-4">
            <Field id="vus" label="Virtual users" value={users} onChange={(v) => { setUsers(v); setPreset('') }} suffix="" />
            <Field id="duration" label="Duration" value={duration} onChange={(v) => { setDuration(v); setPreset('') }} suffix="s" />
            <Field id="threshold" label="Fail if p95 >" value={threshold} onChange={setThreshold} suffix="ms" />
            {route.parameters.map((p) => (
              <Field key={p.name} id={`stress-${p.name}`} label={`{${p.name}}`} value={params[p.name] ?? ''}
                onChange={(v) => setParams({ ...params, [p.name]: v })} suffix="" mono placeholder={p.required ? 'required' : 'optional'} />
            ))}
          </div>

          {canGenerate && (
            <div className="flex flex-wrap items-center gap-3 text-sm">
              <Label htmlFor="pool" className="text-muted-foreground">Generate test data from the rules</Label>
              <div className="relative w-28">
                <Input id="pool" value={pool} onChange={(e) => setPool(e.target.value)} inputMode="numeric" className="pr-14 font-mono" />
                <span className="pointer-events-none absolute top-1/2 right-3 -translate-y-1/2 font-mono text-xs text-muted-foreground">bodies</span>
              </div>
              <span className="text-xs text-muted-foreground">Requests rotate through them, so unique fields don't repeat. 0 sends no body.</span>
            </div>
          )}

          {writes && (
            <Alert>
              <TriangleAlert />
              <AlertTitle>This sends real {method} requests</AlertTitle>
              <AlertDescription>
                Each one runs against your app's database. Use a local or throwaway database. Rolling back each request in a
                transaction needs a Laravel-side helper that isn't built yet.
              </AlertDescription>
            </Alert>
          )}

          <div className="flex flex-wrap items-center justify-end gap-2">
            <span className="mr-auto text-sm text-muted-foreground">
              {token ? 'Sends your bearer token from Docs & Try it.' : 'No token: routes behind auth will answer 401.'}
            </span>
            <Button variant="outline" onClick={stop} disabled={!running}><Square />Stop</Button>
            <Button onClick={start} disabled={running || missing.length > 0 || !Number(users) || !Number(duration)}>
              {running ? <Loader2 className="animate-spin" /> : <Play />}
              {missing.length ? `Fill in ${missing.join(', ')}` : 'Run stress test'}
            </Button>
          </div>
        </CardContent>
      </Card>

      {error && <Alert variant="destructive"><AlertDescription>{error}</AlertDescription></Alert>}

      {state && state.state !== 'idle' && !mine && (
        <p className="text-sm text-muted-foreground">
          {running ? 'Another stress run is in progress' : 'The latest run'} covers {live?.routes.map((r) => r.route_id).join(', ')}.
        </p>
      )}

      {mine && s && state && (
        <>
          <div className="flex items-center gap-3 text-sm">
            <span className={cn('size-2 rounded-full', running ? 'animate-pulse bg-emerald-500' : 'bg-muted-foreground')} />
            <span className="shrink-0">
              {running ? 'Running' : state.interrupted ? 'Stopped' : 'Finished'} · {clock(state.elapsed_ms)}
              {state.duration_ms > 0 && ` / ${clock(state.duration_ms)}`}
            </span>
            <div className="h-1.5 flex-1 overflow-hidden rounded-full bg-muted">
              <div className="h-full bg-foreground transition-[width]" style={{ width: `${Math.min(100, state.duration_ms ? (state.elapsed_ms / state.duration_ms) * 100 : 100)}%` }} />
            </div>
            <span className="shrink-0 text-muted-foreground">{live!.config.concurrency} users</span>
          </div>

          <div className="grid grid-cols-2 gap-3 md:grid-cols-3 xl:grid-cols-5">
            <Tile label="Throughput" value={`${s.rps.toFixed(0)} rps`} note={`peak ${peakRps}`} />
            <Tile label="p50 latency" value={ms(s.latency_ms?.p50)} note="median" />
            <Tile label="p95 latency" value={ms(s.latency_ms?.p95)} note={limit ? (overThreshold ? `over ${limit}ms threshold` : `under ${limit}ms`) : ''} tone={overThreshold ? 'bad' : undefined} />
            <Tile label="p99 latency" value={ms(s.latency_ms?.p99)} note="tail" />
            <Tile label="Error rate" value={pct(s.error_rate)} note={errorNote(s.statuses, s.errors)} tone={s.error_rate > 0 ? 'warn' : undefined} />
          </div>

          <Card>
            <CardHeader className="flex flex-row flex-wrap items-center gap-4">
              <CardTitle>Latency over time</CardTitle>
              <ChartLegend />
            </CardHeader>
            <CardContent><LatencyChart points={live!.timeline} threshold={limit} /></CardContent>
          </Card>

          <section className="flex flex-col gap-3">
            <h2 className="text-sm font-semibold">Status codes</h2>
            <StatusBar statuses={s.statuses} errors={s.errors} />
          </section>

          {state.report && (
            <p className="text-sm text-muted-foreground">
              Saved as <Link to={reportPath(state.report)} className="font-mono underline-offset-4 hover:underline">{state.report}</Link>.
            </p>
          )}
          {state.error && <Alert variant="destructive"><AlertDescription>{state.error}</AlertDescription></Alert>}
        </>
      )}
    </div>
  )
}

function Field({ id, label, value, onChange, suffix, mono, placeholder }: {
  id: string; label: string; value: string; onChange: (v: string) => void; suffix: string; mono?: boolean; placeholder?: string
}) {
  return (
    <div className="flex flex-col gap-1.5">
      <Label htmlFor={id} className={cn('text-muted-foreground', mono && 'font-mono')}>{label}</Label>
      <div className="relative">
        <Input id={id} value={value} onChange={(e) => onChange(e.target.value)} inputMode={mono ? undefined : 'numeric'}
          placeholder={placeholder} className={cn('font-mono', suffix && 'pr-9')} />
        {suffix && <span className="pointer-events-none absolute top-1/2 right-3 -translate-y-1/2 font-mono text-xs text-muted-foreground">{suffix}</span>}
      </div>
    </div>
  )
}

function Tile({ label, value, note, tone }: { label: string; value: string; note?: string; tone?: 'warn' | 'bad' }) {
  return (
    <Card size="sm">
      <CardContent className="flex flex-col gap-1">
        <span className="text-sm text-muted-foreground">{label}</span>
        <span className={cn('font-mono text-2xl font-medium tabular-nums', tone === 'bad' && 'text-red-600 dark:text-red-400', tone === 'warn' && 'text-amber-600 dark:text-amber-400')}>{value}</span>
        {note && <span className="text-xs text-muted-foreground">{note}</span>}
      </CardContent>
    </Card>
  )
}

function errorNote(statuses: Record<string, number>, errors: Record<string, number>) {
  const fives = Object.entries(statuses).filter(([c]) => c.startsWith('5')).reduce((n, [, c]) => n + c, 0)
  const transport = Object.values(errors).reduce((n, c) => n + c, 0)
  if (fives) return `${fives} × 5xx`
  if (transport) return `${transport} transport errors`
  return 'no 5xx or transport errors'
}
