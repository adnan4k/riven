import type { Manifest, RouteStats } from '@/types'
import { Link, routePath } from '@/router'
import { displayUri, groups, mainMethod } from '@/lib/routes'
import { MethodText } from '@/components/Method'
import { StatusDot } from '@/components/StatusDot'
import { Card, CardContent } from '@/components/ui/card'

// The start page: what the app has, and (on small screens, where the sidebar
// is hidden) the route list itself.
export function Overview({ manifest, latestByRoute }: { manifest: Manifest; latestByRoute: Map<string, RouteStats> }) {
  const withRules = manifest.routes.filter((r) => r.request.rules && Object.keys(r.request.rules).length).length
  const withResource = manifest.routes.filter((r) => r.response.resource).length
  const tested = manifest.routes.filter((r) => latestByRoute.has(r.id)).length

  return (
    <div className="flex flex-col gap-6">
      <div>
        <h1 className="text-2xl font-semibold tracking-tight">{manifest.app.name ?? 'Your Laravel app'}</h1>
        <p className="text-sm text-muted-foreground">Laravel {manifest.app.laravel_version} · PHP {manifest.app.php_version}. Pick a route to read its docs, try it, or stress test it.</p>
      </div>

      <div className="grid grid-cols-2 gap-3 md:grid-cols-4">
        <Tile label="Routes" value={manifest.routes.length} />
        <Tile label="With validation rules" value={withRules} />
        <Tile label="With an API Resource" value={withResource} />
        <Tile label="In the latest stress run" value={tested} />
      </div>

      {manifest.warnings.length > 0 && (
        <section className="flex flex-col gap-2">
          <h2 className="text-sm font-semibold">Scan warnings</h2>
          {manifest.warnings.map((w) => (
            <Link key={w.route_id + w.code} to={routePath(w.route_id)} className="rounded-md border px-3 py-2 text-sm hover:bg-accent">
              <span className="font-mono">{w.route_id}</span> <span className="text-muted-foreground">· {w.message}</span>
            </Link>
          ))}
        </section>
      )}

      <section className="flex flex-col gap-4 md:hidden">
        {groups(manifest).map(([g, routes]) => (
          <div key={g} className="flex flex-col gap-1">
            <h2 className="label-mono">{g}</h2>
            {routes.map((r) => (
              <Link key={r.id} to={routePath(r.id)} className="flex items-center gap-3 rounded-md px-2 py-1.5 hover:bg-accent">
                <MethodText method={mainMethod(r)} className="w-14" />
                <span className="flex-1 truncate font-mono text-sm">{displayUri(r)}</span>
                <StatusDot stats={latestByRoute.get(r.id)} />
              </Link>
            ))}
          </div>
        ))}
      </section>
    </div>
  )
}

function Tile({ label, value }: { label: string; value: number }) {
  return (
    <Card size="sm">
      <CardContent className="flex flex-col gap-1">
        <span className="text-sm text-muted-foreground">{label}</span>
        <span className="font-mono text-2xl font-medium tabular-nums">{value}</span>
      </CardContent>
    </Card>
  )
}
