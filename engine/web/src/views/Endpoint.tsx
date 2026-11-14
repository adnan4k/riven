import { useState } from 'react'
import { TriangleAlert } from 'lucide-react'
import type { Manifest, OpenAPIDoc, ReportSummary } from '@/types'
import { Link, navigate, routePath } from '@/router'
import { displayUri, mainMethod, operation } from '@/lib/routes'
import { MethodTag } from '@/components/Method'
import { Alert, AlertDescription, AlertTitle } from '@/components/ui/alert'
import { Badge } from '@/components/ui/badge'
import { Tabs, TabsContent, TabsList, TabsTrigger } from '@/components/ui/tabs'
import { DocsTab } from './DocsTab'
import { StressTab } from './StressTab'
import { HistoryTab } from './HistoryTab'

const TABS = ['docs', 'stress', 'tests', 'history'] as const

export function Endpoint({ manifest, id, tab, openapi, baseUrl, reports, onRunSaved }: {
  manifest: Manifest
  id: string
  tab?: string
  openapi: OpenAPIDoc | null
  baseUrl: string
  reports: ReportSummary[]
  onRunSaved: () => void
}) {
  const route = manifest.routes.find((r) => r.id === id)
  // Shared by Docs (Try it) and Stress: fill route parameters and the token once.
  const [params, setParams] = useState<Record<string, string>>({})
  const [token, setToken] = useState(() => readToken())

  if (!route) {
    return (
      <Alert>
        <AlertTitle>No route {id}</AlertTitle>
        <AlertDescription>It may have been removed since the last scan. <Link to="/" className="underline">Back to all routes</Link></AlertDescription>
      </Alert>
    )
  }

  const current = TABS.includes(tab as (typeof TABS)[number]) ? tab! : 'docs'
  const op = operation(openapi, route)
  const warnings = manifest.warnings.filter((w) => w.route_id === id)
  const updateToken = (t: string) => {
    setToken(t)
    try { sessionStorage.setItem('riven:token', t) } catch { /* storage may be blocked */ }
  }

  return (
    <article className="flex flex-col gap-6">
      <header className="flex flex-col gap-3">
        <div className="flex flex-wrap items-center gap-3">
          <MethodTag method={mainMethod(route)} />
          <h1 className="font-mono text-2xl font-medium tracking-tight md:text-3xl">{displayUri(route)}</h1>
          {route.name && <span className="text-sm text-muted-foreground">{route.name}</span>}
          {route.methods.filter((m) => m !== 'HEAD' && m !== mainMethod(route)).map((m) => (
            <Badge key={m} variant="outline" className="font-mono">also {m}</Badge>
          ))}
        </div>
      </header>

      {warnings.map((w) => (
        <Alert key={w.code + w.message}>
          <TriangleAlert />
          <AlertTitle className="font-mono">{w.code}</AlertTitle>
          <AlertDescription>{w.message}</AlertDescription>
        </Alert>
      ))}

      <Tabs value={current} onValueChange={(t) => navigate(routePath(id, t === 'docs' ? undefined : t))}>
        <TabsList variant="line" className="w-full justify-start border-b">
          <TabsTrigger value="docs">Docs &amp; Try it</TabsTrigger>
          <TabsTrigger value="stress">Stress test</TabsTrigger>
          <TabsTrigger value="tests" disabled title="Planned with the AI features">Generated tests</TabsTrigger>
          <TabsTrigger value="history">History</TabsTrigger>
        </TabsList>

        <TabsContent value="docs" className="pt-4">
          <DocsTab route={route} op={op} baseUrl={baseUrl} params={params} setParams={setParams} token={token} setToken={updateToken} />
        </TabsContent>
        <TabsContent value="stress" className="pt-4">
          <StressTab route={route} params={params} setParams={setParams} token={token} onRunSaved={onRunSaved} />
        </TabsContent>
        <TabsContent value="history" className="pt-4">
          <HistoryTab route={route} reports={reports} />
        </TabsContent>
      </Tabs>
    </article>
  )
}

// The token lives in sessionStorage: kept across routes and reloads in this
// tab, gone when the browser tab closes.
function readToken() {
  try { return sessionStorage.getItem('riven:token') ?? '' } catch { return '' }
}
