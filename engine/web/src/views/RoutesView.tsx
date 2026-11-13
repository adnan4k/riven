import { useMemo, useState } from 'react'
import { Search } from 'lucide-react'
import type { Manifest } from '@/types'
import { Link, routePath } from '@/router'
import { short } from '@/format'
import { Methods } from '@/components/Method'
import { Stat, StatGrid } from '@/components/Stat'
import { Badge } from '@/components/ui/badge'
import { Card, CardContent } from '@/components/ui/card'
import { Input } from '@/components/ui/input'
import { Table, TableBody, TableCell, TableHead, TableHeader, TableRow } from '@/components/ui/table'
import { ToggleGroup, ToggleGroupItem } from '@/components/ui/toggle-group'

const METHODS = ['ALL', 'GET', 'POST', 'PUT', 'PATCH', 'DELETE']

export function RoutesView({ manifest }: { manifest: Manifest }) {
  const [query, setQuery] = useState('')
  const [method, setMethod] = useState('ALL')

  const warnings = useMemo(() => {
    const m = new Map<string, number>()
    for (const w of manifest.warnings) m.set(w.route_id, (m.get(w.route_id) ?? 0) + 1)
    return m
  }, [manifest])

  const q = query.trim().toLowerCase()
  const routes = manifest.routes.filter((r) =>
    (method === 'ALL' || r.methods.includes(method)) &&
    (!q || [r.uri, r.name ?? '', r.action.class ?? '', r.action.method ?? '', ...r.middleware].some((s) => s.toLowerCase().includes(q))),
  )

  const withRules = manifest.routes.filter((r) => r.request.rules && Object.keys(r.request.rules).length).length
  const withResource = manifest.routes.filter((r) => r.response.resource).length

  return (
    <>
      <StatGrid>
        <Stat label="Routes" value={manifest.routes.length} />
        <Stat label="With validation rules" value={withRules} />
        <Stat label="With an API Resource" value={withResource} />
        <Stat label="Warnings" value={manifest.warnings.length} tone={manifest.warnings.length ? 'warn' : undefined} />
      </StatGrid>

      <div className="flex flex-wrap items-center gap-3">
        <div className="relative min-w-64 flex-1">
          <Search className="pointer-events-none absolute top-1/2 left-2.5 size-4 -translate-y-1/2 text-muted-foreground" />
          <Input id="route-filter" type="search" className="pl-8" aria-label="Filter routes"
            placeholder="Filter by URI, name, controller or middleware" value={query} onChange={(e) => setQuery(e.target.value)} />
        </div>
        <ToggleGroup type="single" variant="outline" size="sm" value={method} onValueChange={(v) => v && setMethod(v)} aria-label="Method">
          {METHODS.map((m) => <ToggleGroupItem key={m} value={m} className="font-mono text-xs">{m}</ToggleGroupItem>)}
        </ToggleGroup>
      </div>

      <Card className="py-0">
        <CardContent className="px-0">
          <Table>
            <TableHeader>
              <TableRow>
                <TableHead className="pl-4">Method</TableHead><TableHead>URI</TableHead><TableHead>Name</TableHead>
                <TableHead>Action</TableHead><TableHead>Middleware</TableHead><TableHead className="pr-4">Docs</TableHead>
              </TableRow>
            </TableHeader>
            <TableBody>
              {routes.map((r) => (
                <TableRow key={r.id}>
                  <TableCell className="pl-4"><Methods methods={r.methods} /></TableCell>
                  <TableCell className="font-mono font-medium">
                    <Link to={routePath(r.id)} className="hover:underline">/{r.uri.replace(/^\//, '')}</Link>
                  </TableCell>
                  <TableCell className="font-mono text-muted-foreground">{r.name ?? '–'}</TableCell>
                  <TableCell className="font-mono">
                    {r.action.type === 'closure' ? <span className="text-muted-foreground">Closure</span> : `${short(r.action.class)}@${r.action.method}`}
                  </TableCell>
                  <TableCell>
                    <span className="flex flex-wrap gap-1">{r.middleware.map((m) => <Badge key={m} variant="outline" className="font-mono">{m}</Badge>)}</span>
                  </TableCell>
                  <TableCell className="pr-4">
                    <span className="flex flex-wrap gap-1">
                      {r.request.form_request && <Badge variant="secondary" title={r.request.form_request}>rules</Badge>}
                      {r.response.resource && <Badge variant="secondary" title={r.response.resource}>resource</Badge>}
                      {warnings.get(r.id) && <Badge variant="destructive">{warnings.get(r.id)} warning</Badge>}
                    </span>
                  </TableCell>
                </TableRow>
              ))}
            </TableBody>
          </Table>
          {routes.length === 0 && <p className="p-6 text-center text-sm text-muted-foreground">No routes match this filter.</p>}
        </CardContent>
      </Card>
    </>
  )
}
