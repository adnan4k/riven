import { TriangleAlert } from 'lucide-react'
import type { JSONSchema, Manifest, OpenAPIDoc, Operation, StressReport } from '@/types'
import { useJSON } from '@/api'
import { Link, reportPath } from '@/router'
import { ms, pct, short, when } from '@/format'
import { Methods, Status } from '@/components/Method'
import { Stat } from '@/components/Stat'
import { Section } from './Section'
import { Alert, AlertDescription, AlertTitle } from '@/components/ui/alert'
import { Badge } from '@/components/ui/badge'
import { Table, TableBody, TableCell, TableHead, TableHeader, TableRow } from '@/components/ui/table'

type Route = Manifest['routes'][number]

export function RouteDetail({ manifest, id, openapi, latestReport }: {
  manifest: Manifest
  id: string
  openapi: OpenAPIDoc | null
  latestReport?: string
}) {
  const route = manifest.routes.find((r) => r.id === id)
  if (!route) {
    return (
      <Alert>
        <AlertTitle>No route {id}</AlertTitle>
        <AlertDescription>It may have been removed since the last export. <Link to="/" className="underline">Back to routes</Link></AlertDescription>
      </Alert>
    )
  }

  const method = id.split(' ')[0]
  const op = findOperation(openapi, route.uri, method)
  const warnings = manifest.warnings.filter((w) => w.route_id === id)
  const body = op?.requestBody
  const bodyType = body ? Object.keys(body.content)[0] : null
  const fields = body && bodyType ? flatten(body.content[bodyType].schema) : []
  const query: Field[] = (op?.parameters ?? []).filter((p) => p.in === 'query').map((p) => ({ name: p.name, required: p.required, schema: p.schema }))

  return (
    <>
      <div className="flex flex-col gap-3">
        <p className="text-sm text-muted-foreground"><Link to="/" className="hover:underline">Routes</Link> / <span className="font-mono">{id}</span></p>
        <h2 className="flex flex-wrap items-center gap-2 text-2xl font-semibold tracking-tight">
          <Methods methods={route.methods} />
          <span className="font-mono">/{route.uri.replace(/^\//, '')}</span>
        </h2>
        <dl className="flex flex-wrap gap-x-8 gap-y-2 text-sm">
          <Fact label="Name" value={route.name ?? '–'} />
          <Fact label="Action" value={route.action.type === 'closure' ? 'Closure' : `${route.action.class}@${route.action.method}`} />
          {route.domain && <Fact label="Domain" value={route.domain} />}
          {op && <Fact label="operationId" value={op.operationId} />}
        </dl>
      </div>

      {warnings.map((w) => (
        <Alert key={w.code + w.message}>
          <TriangleAlert />
          <AlertTitle className="font-mono">{w.code}</AlertTitle>
          <AlertDescription>{w.message}</AlertDescription>
        </Alert>
      ))}

      <div className="grid gap-6 lg:grid-cols-2">
        <Section title="Middleware" description="As declared, and the classes they resolve to" flush>
          <Table>
            <TableHeader><TableRow><TableHead className="pl-4">Declared</TableHead><TableHead className="pr-4">Resolved</TableHead></TableRow></TableHeader>
            <TableBody>
              <TableRow>
                <TableCell className="pl-4 align-top">
                  <span className="flex flex-wrap gap-1">{route.middleware.length ? route.middleware.map((m) => <Badge key={m} variant="outline" className="font-mono">{m}</Badge>) : '–'}</span>
                </TableCell>
                <TableCell className="pr-4 font-mono text-xs whitespace-normal">
                  {route.middleware_resolved.length ? route.middleware_resolved.map((m) => <div key={m}>{m}</div>) : '–'}
                </TableCell>
              </TableRow>
            </TableBody>
          </Table>
        </Section>

        <Section title="Route parameters" description={route.parameters.length ? undefined : 'This route has none.'} flush={route.parameters.length > 0}>
          {route.parameters.length > 0 && (
            <Table>
              <TableHeader><TableRow><TableHead className="pl-4">Name</TableHead><TableHead>Required</TableHead><TableHead className="pr-4">Pattern</TableHead></TableRow></TableHeader>
              <TableBody>
                {route.parameters.map((p) => (
                  <TableRow key={p.name}>
                    <TableCell className="pl-4 font-mono">{p.name}</TableCell>
                    <TableCell>{p.required ? 'yes' : 'optional'}</TableCell>
                    <TableCell className="pr-4 font-mono">{p.pattern ?? <span className="text-muted-foreground">any</span>}</TableCell>
                  </TableRow>
                ))}
              </TableBody>
            </Table>
          )}
        </Section>
      </div>

      {route.request.form_request && (
        <Section
          title="Request"
          description={<>Validated by <span className="font-mono">{route.request.form_request}</span></>}
          action={bodyType && <Badge variant="outline" className="font-mono">{bodyType}</Badge>}
          flush={fields.length + query.length > 0}
        >
          {route.request.rules === null && (
            <Alert><TriangleAlert /><AlertDescription>These rules need a live request (route parameters, the user…), so the fields aren't known statically.</AlertDescription></Alert>
          )}
          {fields.length > 0 && <FieldTable fields={fields} />}
          {query.length > 0 && <FieldTable fields={query} label="Query parameter" />}
          {body?.['x-riven-unmapped-rules'] && (
            <p className="flex flex-wrap items-center gap-1 border-t px-4 py-3 text-xs text-muted-foreground">
              Not expressible in the schema:
              {body['x-riven-unmapped-rules'].map((r) => <Badge key={r} variant="outline" className="font-mono">{r}</Badge>)}
            </p>
          )}
        </Section>
      )}

      <Section title="Responses" flush>
        <Table>
          <TableHeader><TableRow><TableHead className="pl-4">Status</TableHead><TableHead>Meaning</TableHead><TableHead className="pr-4">Body</TableHead></TableRow></TableHeader>
          <TableBody>
            {op ? Object.entries(op.responses).map(([code, r]) => (
              <TableRow key={code}>
                <TableCell className="pl-4"><Status code={code} /></TableCell>
                <TableCell>{r.description}</TableCell>
                <TableCell className="pr-4 font-mono text-xs">{code.startsWith('2') ? responseBody(route.response) : r.content ? 'JSON' : '–'}</TableCell>
              </TableRow>
            )) : (
              <TableRow><TableCell colSpan={3} className="pl-4 text-muted-foreground">OpenAPI document unavailable.</TableCell></TableRow>
            )}
          </TableBody>
        </Table>
      </Section>

      {latestReport && <LatestStress reportName={latestReport} routeId={id} />}
    </>
  )
}

function Fact({ label, value }: { label: string; value: string }) {
  return (
    <div className="min-w-0">
      <dt className="text-xs text-muted-foreground">{label}</dt>
      <dd className="font-mono break-all">{value}</dd>
    </div>
  )
}

function responseBody(r: Route['response']) {
  if (!r.resource) return 'unknown'
  return r.collection ? `{ data: ${short(r.resource)}[] }` : `{ data: ${short(r.resource)} }`
}

// The OpenAPI path for a route: the full form keeps every parameter.
function findOperation(doc: OpenAPIDoc | null, uri: string, method: string): Operation | undefined {
  if (!doc) return undefined
  const path = '/' + uri.replace(/^\//, '').replace(/\?\}/g, '}')
  return doc.paths[path === '//' ? '/' : path]?.[method.toLowerCase()]
}

interface Field { name: string; required: boolean; schema: JSONSchema }

// flatten turns nested objects/arrays into Laravel-style dotted names: items.*.id
function flatten(schema: JSONSchema, prefix = ''): Field[] {
  const out: Field[] = []
  for (const [name, s] of Object.entries(schema.properties ?? {})) {
    const full = prefix + name
    out.push({ name: full, required: schema.required?.includes(name) ?? false, schema: s })
    if (s.properties) out.push(...flatten(s, full + '.'))
    if (s.items?.properties) out.push(...flatten(s.items, full + '.*.'))
  }
  return out
}

function FieldTable({ fields, label = 'Field' }: { fields: Field[]; label?: string }) {
  return (
    <Table>
      <TableHeader>
        <TableRow><TableHead className="pl-4">{label}</TableHead><TableHead>Type</TableHead><TableHead>Constraints</TableHead><TableHead className="pr-4">Laravel rules</TableHead></TableRow>
      </TableHeader>
      <TableBody>
        {fields.map((f) => (
          <TableRow key={f.name}>
            <TableCell className="pl-4 font-mono">
              {f.name}{f.required && <span className="ml-0.5 text-destructive" title="required">*</span>}
            </TableCell>
            <TableCell className="font-mono">{typeOf(f.schema)}</TableCell>
            <TableCell className="text-muted-foreground">{constraints(f.schema).join(' · ') || '–'}</TableCell>
            <TableCell className="pr-4">
              <span className="flex flex-wrap gap-1">
                {(f.schema['x-laravel-rules'] ?? []).map((r) => <Badge key={r} variant="secondary" className="font-mono">{short(r) || r}</Badge>)}
              </span>
            </TableCell>
          </TableRow>
        ))}
      </TableBody>
    </Table>
  )
}

function typeOf(s: JSONSchema): string {
  const t = Array.isArray(s.type) ? s.type.join(' | ') : s.type ?? 'any'
  if (t === 'array' && s.items) return `${typeOf(s.items)}[]`
  return s.format ? `${t} (${s.format})` : t
}

function constraints(s: JSONSchema): string[] {
  const c: string[] = []
  if (s.minLength != null) c.push(`≥ ${s.minLength} chars`)
  if (s.maxLength != null) c.push(`≤ ${s.maxLength} chars`)
  if (s.minimum != null) c.push(`≥ ${s.minimum}`)
  if (s.maximum != null) c.push(`≤ ${s.maximum}`)
  if (s.minItems != null) c.push(`≥ ${s.minItems} items`)
  if (s.maxItems != null) c.push(`≤ ${s.maxItems} items`)
  if (s.enum) c.push(`one of ${s.enum.map(String).join(', ')}`)
  if (s.not?.enum) c.push(`not ${s.not.enum.map(String).join(', ')}`)
  if (s.pattern) c.push(`matches ${s.pattern}`)
  return c
}

function LatestStress({ reportName, routeId }: { reportName: string; routeId: string }) {
  const rep = useJSON<StressReport>('/api/reports/' + encodeURIComponent(reportName))
  if (rep.state !== 'ok') return null
  const s = rep.data.routes.find((r) => r.route_id === routeId)

  return (
    <Section
      title="Latest stress run"
      description={<><Link to={reportPath(reportName)} className="underline-offset-4 hover:underline">{when(rep.data.started_at)}</Link>, {rep.data.config.concurrency} workers</>}
    >
      {s ? (
        <div className="grid grid-cols-2 gap-3 md:grid-cols-5">
          <Stat label="Requests" value={s.requests} />
          <Stat label="Req/s" value={s.rps.toFixed(1)} />
          <Stat label="p50" value={ms(s.latency_ms?.p50)} />
          <Stat label="p95" value={ms(s.latency_ms?.p95)} />
          <Stat label="Failed" value={pct(s.error_rate)} tone={s.error_rate > 0 ? 'bad' : undefined} />
        </div>
      ) : (
        <p className="text-sm text-muted-foreground">This route wasn't part of the latest run.</p>
      )}
    </Section>
  )
}
