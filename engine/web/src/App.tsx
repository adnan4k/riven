import { useEffect, useRef, useState } from 'react'
import { Activity, Pencil, RefreshCw, RotateCcw, Search } from 'lucide-react'
import { apiURL, postJSON, useJSON } from '@/api'
import { Link, routePath, usePath } from '@/router'
import type { AppConfig, Manifest, OpenAPIDoc, ReportSummary, RouteStats, StressReport } from '@/types'
import { ago } from '@/format'
import { displayUri, groups, mainMethod } from '@/lib/routes'
import { cn } from '@/lib/utils'
import { MethodText } from '@/components/Method'
import { StatusDot } from '@/components/StatusDot'
import { Alert, AlertDescription, AlertTitle } from '@/components/ui/alert'
import { Badge } from '@/components/ui/badge'
import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'
import { Label } from '@/components/ui/label'
import { Skeleton } from '@/components/ui/skeleton'
import { Overview } from '@/views/Overview'
import { Endpoint } from '@/views/Endpoint'
import { ReportsView } from '@/views/ReportsView'
import { ReportDetail } from '@/views/ReportDetail'

export function App() {
  const path = usePath()
  const [version, setVersion] = useState(0) // bump to refetch after a rescan or a run
  const manifest = useJSON<Manifest>('/api/manifest', version)
  const openapi = useJSON<OpenAPIDoc>('/api/openapi', version)
  const config = useJSON<AppConfig>('/api/config', version)
  const reports = useJSON<ReportSummary[]>('/api/reports', version)
  const latestName = reports.state === 'ok' ? reports.data[0]?.name : undefined
  const latest = useJSON<StressReport>(latestName ? '/api/reports/' + encodeURIComponent(latestName) : null, version)

  const [query, setQuery] = useState('')
  const [rescan, setRescan] = useState<{ busy: boolean; error?: string }>({ busy: false })
  // Wait for the config: inside Laravel there is no event stream to open.
  const live = config.state === 'ok' && config.data.live_updates !== false
  const scan = useLiveUpdates(() => setVersion((v) => v + 1), live)
  const defaultTarget = config.state === 'ok' ? config.data.base_url : ''
  const [target, setTarget] = useTarget(defaultTarget)

  const [, section, param, tab] = path.split('/')
  const selectedId = section === 'routes' && param ? decodeURIComponent(param) : null

  const latestByRoute = new Map<string, RouteStats>(latest.state === 'ok' ? latest.data.routes.map((r) => [r.route_id, r]) : [])

  async function doRescan() {
    setRescan({ busy: true })
    try {
      await postJSON('/api/rescan', {})
      setRescan({ busy: false })
      if (!live) setVersion((v) => v + 1) // with live updates, the "manifest" event reloads
    } catch (e) {
      setRescan({ busy: false, error: (e as Error).message })
    }
  }

  return (
    <div className="flex h-dvh flex-col">
      <TopBar
        appName={manifest.state === 'ok' ? manifest.data.app.name : null}
        target={target}
        defaultTarget={defaultTarget}
        onTarget={setTarget}
        canRescan={config.state === 'ok' && config.data.can_rescan}
        rescanning={rescan.busy || scan.state === 'running'}
        scanNote={scan.state === 'done' ? 'Routes updated' : undefined}
        onRescan={doRescan}
        query={query}
        onQuery={setQuery}
      />

      <div className="flex min-h-0 flex-1">
        <aside className="hidden w-80 shrink-0 overflow-y-auto border-r bg-sidebar md:block">
          {manifest.state === 'ok' ? (
            <RouteList manifest={manifest.data} query={query} selectedId={selectedId} latestByRoute={latestByRoute} />
          ) : (
            <div className="flex flex-col gap-2 p-4">{[1, 2, 3, 4].map((i) => <Skeleton key={i} className="h-7" />)}</div>
          )}
        </aside>

        <main className="min-w-0 flex-1 overflow-y-auto">
          <div className="mx-auto flex w-full max-w-6xl flex-col gap-6 p-4 md:p-8">
            {(rescan.error || scan.state === 'failed') && (
              <Alert variant="destructive">
                <AlertTitle>Couldn't rescan the routes</AlertTitle>
                <AlertDescription><pre className="font-mono text-xs whitespace-pre-wrap">{rescan.error ?? scan.message}</pre></AlertDescription>
              </Alert>
            )}
            {manifest.state === 'loading' && <Skeleton className="h-64 w-full" />}
            {manifest.state === 'error' && (
              <Alert variant="destructive"><AlertTitle>Can't read the manifest</AlertTitle><AlertDescription>{manifest.message}</AlertDescription></Alert>
            )}
            {manifest.state === 'ok' && (
              section === 'routes' && selectedId ? (
                <Endpoint
                  key={selectedId}
                  manifest={manifest.data}
                  id={selectedId}
                  tab={tab}
                  openapi={openapi.state === 'ok' ? openapi.data : null}
                  baseUrl={target}
                  reports={reports.state === 'ok' ? reports.data : []}
                  onRunSaved={() => setVersion((v) => v + 1)}
                />
              ) : section === 'reports' && param ? (
                <ReportDetail name={decodeURIComponent(param)} />
              ) : section === 'reports' ? (
                <ReportsView reports={reports} />
              ) : (
                <Overview manifest={manifest.data} latestByRoute={latestByRoute} />
              )
            )}
          </div>
        </main>
      </div>
    </div>
  )
}

// useTarget is the URL Try it and stress runs are sent to. It starts as the
// server's default (APP_URL) and can be changed in the top bar; the choice is
// kept in this browser only, and clearing it goes back to the default.
function useTarget(fallback: string): [string, (t: string) => void] {
  const [chosen, setChosen] = useState(() => {
    try { return localStorage.getItem('riven:target') ?? '' } catch { return '' }
  })
  const set = (t: string) => {
    const value = t.trim().replace(/\/+$/, '')
    setChosen(value)
    try { value ? localStorage.setItem('riven:target', value) : localStorage.removeItem('riven:target') } catch { /* storage blocked */ }
  }
  return [chosen || fallback, set]
}

// useLiveUpdates listens to /api/events: the engine watches the app's code,
// re-exports on change, and tells us to reload. EventSource reconnects by itself.
// Without live updates (inside Laravel, the manifest is built on every
// request) it reloads when you come back to the tab instead.
function useLiveUpdates(onManifest: () => void, enabled: boolean) {
  const [scan, setScan] = useState<{ state: 'idle' | 'running' | 'done' | 'failed'; message?: string }>({ state: 'idle' })
  const reload = useRef(onManifest)
  reload.current = onManifest

  useEffect(() => {
    if (!enabled) {
      const onVisible = () => document.visibilityState === 'visible' && reload.current()
      document.addEventListener('visibilitychange', onVisible)
      return () => document.removeEventListener('visibilitychange', onVisible)
    }
    const es = new EventSource(apiURL('/api/events'))
    let clear: ReturnType<typeof setTimeout>
    es.addEventListener('manifest', () => reload.current())
    es.addEventListener('scan', (msg) => {
      const e = JSON.parse((msg as MessageEvent).data) as { state: 'running' | 'done' | 'failed'; message?: string }
      clearTimeout(clear)
      setScan({ state: e.state, message: e.message })
      if (e.state === 'done') clear = setTimeout(() => setScan({ state: 'idle' }), 2500)
    })
    return () => { clearTimeout(clear); es.close() }
  }, [enabled])

  return scan
}

function TopBar({ appName, target, defaultTarget, onTarget, canRescan, rescanning, scanNote, onRescan, query, onQuery }: {
  appName: string | null
  target: string
  defaultTarget: string
  onTarget: (t: string) => void
  canRescan: boolean
  rescanning: boolean
  scanNote?: string
  onRescan: () => void
  query: string
  onQuery: (q: string) => void
}) {
  const search = useRef<HTMLInputElement>(null)

  // ⌘K / Ctrl+K focuses the route search, like most developer tools.
  useEffect(() => {
    const onKey = (e: KeyboardEvent) => {
      if ((e.metaKey || e.ctrlKey) && e.key.toLowerCase() === 'k') {
        e.preventDefault()
        search.current?.focus()
      }
    }
    window.addEventListener('keydown', onKey)
    return () => window.removeEventListener('keydown', onKey)
  }, [])

  return (
    <header className="flex h-14 shrink-0 items-center gap-3 border-b px-4">
      <Link to="/" className="flex items-center gap-2 font-semibold">
        <Activity className="size-5 text-sky-600 dark:text-sky-400" aria-hidden="true" />
        Riven
      </Link>
      {appName && <Badge variant="outline" className="hidden rounded-full font-normal sm:inline-flex">{appName}</Badge>}

      <div className="relative mx-2 max-w-2xl flex-1">
        <Search className="pointer-events-none absolute top-1/2 left-2.5 size-4 -translate-y-1/2 text-muted-foreground" />
        <Input ref={search} id="route-search" type="search" value={query} onChange={(e) => onQuery(e.target.value)}
          className="pl-8" aria-label="Search routes" placeholder="Search routes, controllers, names…" />
        <kbd className="pointer-events-none absolute top-1/2 right-2.5 hidden -translate-y-1/2 font-mono text-xs text-muted-foreground sm:block">⌘K</kbd>
      </div>

      <div className="ml-auto flex items-center gap-2">
        {scanNote && <span className="hidden text-xs text-emerald-600 sm:inline dark:text-emerald-400" role="status">{scanNote}</span>}
        <TargetEditor target={target} defaultTarget={defaultTarget} onChange={onTarget} />
        <Button variant="outline" size="sm" onClick={onRescan} disabled={!canRescan || rescanning}
          title={canRescan ? 'Scan the routes again' : 'Run serve from your Laravel app to enable this'}>
          <RefreshCw className={cn(rescanning && 'animate-spin')} />
          {rescanning ? 'Scanning…' : 'Rescan routes'}
        </Button>
      </div>
    </header>
  )
}

function RouteList({ manifest, query, selectedId, latestByRoute }: {
  manifest: Manifest
  query: string
  selectedId: string | null
  latestByRoute: Map<string, RouteStats>
}) {
  const q = query.trim().toLowerCase()
  const all = groups(manifest)
  const shown = all
    .map(([g, routes]) => [g, routes.filter((r) =>
      !q || [r.uri, r.name ?? '', r.action.class ?? '', r.action.method ?? '', mainMethod(r)].some((s) => s.toLowerCase().includes(q)),
    )] as const)
    .filter(([, routes]) => routes.length)

  return (
    <nav aria-label="Routes" className="flex flex-col gap-5 p-3">
      <p className="flex justify-between px-2 pt-1 text-xs text-muted-foreground">
        <span>{manifest.routes.length} routes · {all.length} groups</span>
        <span title={manifest.generated_at}>scanned {ago(manifest.generated_at)}</span>
      </p>
      {shown.map(([g, routes]) => (
        <div key={g} className="flex flex-col gap-0.5">
          <h2 className="label-mono px-2 pb-1">{g}</h2>
          {routes.map((r) => (
            <Link key={r.id} to={routePath(r.id)}
              aria-current={r.id === selectedId ? 'page' : undefined}
              className={cn('flex items-center gap-3 rounded-md px-2 py-1.5 hover:bg-accent',
                r.id === selectedId && 'bg-accent')}>
              <MethodText method={mainMethod(r)} className="w-14 shrink-0" />
              <span className="min-w-0 flex-1 truncate font-mono text-sm" title={displayUri(r)}>{displayUri(r)}</span>
              <StatusDot stats={latestByRoute.get(r.id)} />
            </Link>
          ))}
        </div>
      ))}
      {shown.length === 0 && <p className="px-2 text-sm text-muted-foreground">No routes match “{query}”.</p>}
      <Link to="/reports" className="px-2 text-xs text-muted-foreground hover:underline">All stress runs →</Link>
    </nav>
  )
}

// TargetEditor shows where requests go and lets you point them elsewhere
// (another port, a staging URL, a Docker service name).
function TargetEditor({ target, defaultTarget, onChange }: { target: string; defaultTarget: string; onChange: (t: string) => void }) {
  const [editing, setEditing] = useState(false)
  const [draft, setDraft] = useState(target)
  const valid = /^https?:\/\/[^/\s]+/.test(draft.trim())
  const changed = !!defaultTarget && target !== defaultTarget

  function save() {
    if (!valid) return
    onChange(draft.trim() === defaultTarget ? '' : draft)
    setEditing(false)
  }

  if (editing) {
    return (
      <form className="flex items-center gap-2" onSubmit={(e) => { e.preventDefault(); save() }}>
        <Label htmlFor="target-url" className="text-sm text-muted-foreground">Target</Label>
        <Input id="target-url" autoFocus value={draft} onChange={(e) => setDraft(e.target.value)}
          onKeyDown={(e) => e.key === 'Escape' && setEditing(false)}
          aria-invalid={!valid} placeholder="http://localhost:8000" className="h-8 w-72 font-mono text-xs" />
        <Button type="submit" size="sm" disabled={!valid}>Save</Button>
        <Button type="button" size="sm" variant="ghost" onClick={() => setEditing(false)}>Cancel</Button>
      </form>
    )
  }

  return (
    <span className="flex items-center gap-1 text-sm">
      <span className="hidden text-muted-foreground sm:inline">Target</span>
      <Button variant="outline" size="sm" className="max-w-64 font-mono text-xs" title="Change where requests are sent"
        onClick={() => { setDraft(target); setEditing(true) }}>
        <span className="truncate">{target || 'set a target URL'}</span>
        <Pencil className="text-muted-foreground" />
      </Button>
      {changed && (
        <Button variant="ghost" size="icon" className="size-8" title={`Back to ${defaultTarget}`} aria-label="Reset target" onClick={() => onChange('')}>
          <RotateCcw />
        </Button>
      )}
    </span>
  )
}
