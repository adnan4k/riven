import { useEffect, useRef, useState } from 'react'
import { Activity, RefreshCw, Search } from 'lucide-react'
import { getJSON, postJSON, useJSON } from '@/api'
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

  const [, section, param, tab] = path.split('/')
  const selectedId = section === 'routes' && param ? decodeURIComponent(param) : null

  const latestByRoute = new Map<string, RouteStats>(latest.state === 'ok' ? latest.data.routes.map((r) => [r.route_id, r]) : [])

  async function doRescan() {
    setRescan({ busy: true })
    try {
      await postJSON('/api/rescan', {})
      setRescan({ busy: false })
      setVersion((v) => v + 1)
    } catch (e) {
      setRescan({ busy: false, error: (e as Error).message })
    }
  }

  return (
    <div className="flex h-dvh flex-col">
      <TopBar
        appName={manifest.state === 'ok' ? manifest.data.app.name : null}
        baseUrl={config.state === 'ok' ? config.data.base_url : ''}
        canRescan={config.state === 'ok' && config.data.can_rescan}
        rescanning={rescan.busy}
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
            {rescan.error && (
              <Alert variant="destructive"><AlertTitle>Rescan failed</AlertTitle><AlertDescription>{rescan.error}</AlertDescription></Alert>
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
                  baseUrl={config.state === 'ok' ? config.data.base_url : ''}
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

function TopBar({ appName, baseUrl, canRescan, rescanning, onRescan, query, onQuery }: {
  appName: string | null
  baseUrl: string
  canRescan: boolean
  rescanning: boolean
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
        <Activity className="size-5 text-sky-600" aria-hidden="true" />
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
        {baseUrl && (
          <span className="hidden items-center gap-2 text-sm lg:flex">
            <span className="text-muted-foreground">Target</span>
            <span className="rounded-md border px-2.5 py-1 font-mono text-xs">{baseUrl}</span>
          </span>
        )}
        <Button variant="outline" size="sm" onClick={onRescan} disabled={!canRescan || rescanning}
          title={canRescan ? 'Run php artisan riven:export' : 'Run serve from your Laravel app to enable this'}>
          <RefreshCw className={cn(rescanning && 'animate-spin')} />
          {rescanning ? 'Rescanning…' : 'Rescan routes'}
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

