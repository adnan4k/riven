import { useMemo, useState } from 'react'
import { Check, Copy, Loader2, Send, Sparkles } from 'lucide-react'
import { postJSON } from '@/api'
import type { FakeResult, Operation, Route, TryResponse } from '@/types'
import { bytes, ms, short } from '@/format'
import { curl, example, fillPath, flatten, mainMethod, typeOf, type Field } from '@/lib/routes'
import { Status } from '@/components/Method'
import { Alert, AlertDescription } from '@/components/ui/alert'
import { Badge } from '@/components/ui/badge'
import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'
import { Label } from '@/components/ui/label'
import { Table, TableBody, TableCell, TableHead, TableHeader, TableRow } from '@/components/ui/table'
import { Textarea } from '@/components/ui/textarea'

const responseLabel: Record<string, string> = {
  '200': 'OK', '201': 'Created', '204': 'No Content', '401': 'Unauthenticated', '403': 'Forbidden',
  '404': 'Not Found', '422': 'Validation', '429': 'Throttled',
}

export function DocsTab({ route, op, baseUrl, params, setParams, token, setToken }: {
  route: Route
  op?: Operation
  baseUrl: string
  params: Record<string, string>
  setParams: (p: Record<string, string>) => void
  token: string
  setToken: (t: string) => void
}) {
  const method = mainMethod(route)
  const body = op?.requestBody
  const bodyType = body ? Object.keys(body.content)[0] : null
  const schema = body && bodyType ? body.content[bodyType].schema : null
  const fields = schema ? flatten(schema) : []
  const query: Field[] = (op?.parameters ?? []).filter((p) => p.in === 'query').map((p) => ({ name: p.name, required: p.required, schema: p.schema }))

  return (
    <div className="grid gap-8 xl:grid-cols-[minmax(0,1fr)_minmax(0,1fr)]">
      <div className="flex flex-col gap-6">
        {route.request.form_request ? (
          <section className="flex flex-col gap-3">
            <h2 className="text-sm font-semibold">
              {fields.length || route.request.rules === null ? 'Body parameters' : 'Query parameters'}
              <span className="font-normal text-muted-foreground"> · from {short(route.request.form_request)}::rules()</span>
            </h2>
            {route.request.rules === null ? (
              <Alert><AlertDescription>These rules need a live request (route parameters, the current user…), so the fields can't be read statically.</AlertDescription></Alert>
            ) : (
              <ParamTable fields={fields.length ? fields : query} />
            )}
          </section>
        ) : (
          <p className="text-sm text-muted-foreground">No FormRequest: this route doesn't validate input with declared rules.</p>
        )}

        {route.parameters.length > 0 && (
          <section className="flex flex-col gap-3">
            <h2 className="text-sm font-semibold">Route parameters</h2>
            <div className="overflow-hidden rounded-md border">
              <Table>
                <TableHeader><TableRow><TableHead className="pl-4">Name</TableHead><TableHead>Required</TableHead><TableHead className="pr-4">Pattern</TableHead></TableRow></TableHeader>
                <TableBody>
                  {route.parameters.map((p) => (
                    <TableRow key={p.name}>
                      <TableCell className="pl-4 font-mono">{p.name}</TableCell>
                      <TableCell>{p.required ? 'yes' : 'optional'}</TableCell>
                      <TableCell className="pr-4 font-mono text-muted-foreground">{p.pattern ?? 'any'}</TableCell>
                    </TableRow>
                  ))}
                </TableBody>
              </Table>
            </div>
          </section>
        )}

        <section className="flex flex-col gap-3">
          <h2 className="text-sm font-semibold">Responses</h2>
          <div className="flex flex-wrap gap-2">
            {op ? Object.keys(op.responses).map((code) => <Status key={code} code={code} label={responseLabel[code]} />) : <span className="text-sm text-muted-foreground">Unknown</span>}
          </div>
          {route.response.resource && (
            <p className="text-sm text-muted-foreground">
              Success body: <code className="font-mono text-foreground">{`{ data: ${short(route.response.resource)}${route.response.collection ? '[]' : ''} }`}</code>
            </p>
          )}
        </section>
      </div>

      <TryIt route={route} method={method} baseUrl={baseUrl} schema={schema} params={params} setParams={setParams} token={token} setToken={setToken} />
    </div>
  )
}

function ParamTable({ fields }: { fields: Field[] }) {
  return (
    <div className="overflow-hidden rounded-md border">
      <Table>
        <TableHeader><TableRow><TableHead className="pl-4">Field</TableHead><TableHead>Type</TableHead><TableHead className="pr-4">Rules</TableHead></TableRow></TableHeader>
        <TableBody>
          {fields.map((f) => (
            <TableRow key={f.name}>
              <TableCell className="pl-4 font-mono">{f.name}{f.required && <span className="ml-1 text-amber-600 dark:text-amber-400" title="required">*</span>}</TableCell>
              <TableCell>{typeOf(f.schema)}</TableCell>
              <TableCell className="pr-4 font-mono text-xs whitespace-normal text-muted-foreground">
                {(f.schema['x-laravel-rules'] ?? []).map((r) => short(r) || r).join('|') || '–'}
              </TableCell>
            </TableRow>
          ))}
        </TableBody>
      </Table>
    </div>
  )
}

function TryIt({ route, method, baseUrl, schema, params, setParams, token, setToken }: {
  route: Route
  method: string
  baseUrl: string
  schema: Parameters<typeof example>[0] | null
  params: Record<string, string>
  setParams: (p: Record<string, string>) => void
  token: string
  setToken: (t: string) => void
}) {
  const initialBody = useMemo(() => (schema ? JSON.stringify(example(schema), null, 2) : ''), [schema])
  const [body, setBody] = useState(initialBody)
  const [result, setResult] = useState<{ state: 'idle' } | { state: 'sending' } | { state: 'ok'; res: TryResponse } | { state: 'error'; message: string }>({ state: 'idle' })
  const [copied, setCopied] = useState(false)
  const [gen, setGen] = useState<{ busy: boolean; notes: string[] }>({ busy: false, notes: [] })

  const sendsBody = method !== 'GET' && method !== 'HEAD' && method !== 'DELETE'
  const path = fillPath(route, params)
  const missing = route.parameters.filter((p) => p.required && !params[p.name]).map((p) => p.name)
  const headers: Record<string, string> = { Accept: 'application/json' }
  if (sendsBody && body.trim()) headers['Content-Type'] = 'application/json'
  if (token) headers.Authorization = `Bearer ${token}`
  const bodyError = sendsBody && body.trim() ? jsonError(body) : null

  async function send() {
    setResult({ state: 'sending' })
    try {
      const payload = sendsBody ? body : ''
      const res = sameOrigin(baseUrl)
        ? await sendDirect(method, baseUrl.replace(/\/$/, '') + path, headers, payload)
        : await postJSON<TryResponse>('/api/try', { base_url: baseUrl, method, path, headers, body: payload })
      setResult({ state: 'ok', res })
    } catch (e) {
      setResult({ state: 'error', message: (e as Error).message })
    }
  }

  // Generates a body from the route's rules in the app (riven:fake-body):
  // Faker values, real rows for exists:, fresh values for unique:.
  async function generate() {
    setGen({ busy: true, notes: [] })
    try {
      const res = await postJSON<FakeResult>('/api/fake', { route_id: route.id, count: 1 })
      setBody(JSON.stringify(res.bodies[0] ?? {}, null, 2))
      const failed = Object.entries(res.errors ?? {}).map(([f, m]) => `${f}: ${m[0]}`)
      setGen({ busy: false, notes: [...res.warnings, ...failed] })
    } catch (e) {
      setGen({ busy: false, notes: [(e as Error).message] })
    }
  }

  async function copyCurl() {
    try {
      await navigator.clipboard.writeText(curl(method, baseUrl.replace(/\/$/, '') + path, headers, sendsBody ? body : ''))
      setCopied(true)
      setTimeout(() => setCopied(false), 1500)
    } catch { /* clipboard blocked: nothing to do */ }
  }

  return (
    <section className="flex flex-col gap-4">
      <div className="flex items-center justify-between gap-2">
        <h2 className="text-sm font-semibold">Request</h2>
        <div className="flex gap-2">
          {sendsBody && route.request.form_request && (
            <Button variant="outline" size="sm" onClick={generate} disabled={gen.busy}>
              {gen.busy ? <Loader2 className="animate-spin" /> : <Sparkles />}Generate test data
            </Button>
          )}
          <Button variant="outline" size="sm" onClick={copyCurl}>{copied ? <Check /> : <Copy />}{copied ? 'Copied' : 'Copy cURL'}</Button>
        </div>
      </div>

      <div className="grid grid-cols-[auto_1fr] items-center gap-x-3 gap-y-2">
        <Label htmlFor="try-token" className="text-muted-foreground">Auth</Label>
        <Input id="try-token" type="password" autoComplete="off" value={token} onChange={(e) => setToken(e.target.value)}
          placeholder="Bearer token (e.g. a Sanctum token), optional" className="font-mono" />
        {route.parameters.map((p) => (
          <ParamInput key={p.name} name={p.name} required={p.required} value={params[p.name] ?? ''}
            onChange={(v) => setParams({ ...params, [p.name]: v })} />
        ))}
      </div>

      <p className="truncate font-mono text-xs text-muted-foreground" title={baseUrl + path}>
        {method} {baseUrl.replace(/\/$/, '')}{path}
      </p>

      {sendsBody && (
        <div className="flex flex-col gap-1">
          <Textarea id="try-body" aria-label="Request body (JSON)" value={body} onChange={(e) => setBody(e.target.value)}
            spellCheck={false} className="min-h-56 font-mono text-sm" placeholder="{ }" />
          {bodyError && <p className="text-xs text-red-600 dark:text-red-400">Invalid JSON: {bodyError}</p>}
          {gen.notes.length > 0 && (
            <ul className="flex flex-col gap-0.5 text-xs text-amber-700 dark:text-amber-400">
              {gen.notes.map((n) => <li key={n}>{n}</li>)}
            </ul>
          )}
        </div>
      )}

      <Button size="lg" onClick={send} disabled={result.state === 'sending' || missing.length > 0 || !!bodyError || !baseUrl}>
        {result.state === 'sending' ? <Loader2 className="animate-spin" /> : <Send />}
        {missing.length ? `Fill in ${missing.join(', ')}` : 'Send request'}
      </Button>

      {result.state === 'error' && <Alert variant="destructive"><AlertDescription>{result.message}</AlertDescription></Alert>}
      {result.state === 'ok' && <ResponseView res={result.res} />}
    </section>
  )
}

function ParamInput({ name, required, value, onChange }: { name: string; required: boolean; value: string; onChange: (v: string) => void }) {
  const id = `param-${name}`
  return (
    <>
      <Label htmlFor={id} className="font-mono text-muted-foreground">{`{${name}}`}</Label>
      <Input id={id} value={value} onChange={(e) => onChange(e.target.value)} placeholder={required ? 'required' : 'optional'} className="font-mono" />
    </>
  )
}

function ResponseView({ res }: { res: TryResponse }) {
  let pretty = res.body
  try { pretty = JSON.stringify(JSON.parse(res.body), null, 2) } catch { /* not JSON: show as is */ }

  return (
    <div className="flex flex-col gap-2">
      <div className="flex flex-wrap items-center gap-3">
        <h3 className="text-sm font-semibold">Response</h3>
        <Status code={String(res.status)} />
        <span className="text-sm text-muted-foreground">{ms(res.duration_ms)} · {bytes(res.size_bytes)}</span>
        {res.headers['Content-Type'] && <Badge variant="outline" className="font-mono">{res.headers['Content-Type'].split(';')[0]}</Badge>}
      </div>
      <pre className="max-h-96 overflow-auto rounded-md border bg-muted/40 p-4 font-mono text-sm">{pretty || '(empty body)'}</pre>
      {res.truncated && <p className="text-xs text-muted-foreground">Body cut at 1 MB.</p>}
    </div>
  )
}

// When the target is the page's own origin (the dashboard served by Laravel
// at /riven), the browser sends the request itself: no proxy, so it works
// even when the server can't reach its own public URL (Docker), and it
// carries your session cookie plus Laravel's XSRF token, so web routes work
// while you're logged in. Other targets go through the server (CORS).
function sameOrigin(target: string): boolean {
  try { return new URL(target).origin === window.location.origin } catch { return false }
}

const BODY_LIMIT = 1 << 20

async function sendDirect(method: string, url: string, headers: Record<string, string>, body: string): Promise<TryResponse> {
  const xsrf = document.cookie.split('; ').find((c) => c.startsWith('XSRF-TOKEN='))
  const h = { ...headers }
  if (xsrf) h['X-XSRF-TOKEN'] = decodeURIComponent(xsrf.slice('XSRF-TOKEN='.length))

  const start = performance.now()
  const res = await fetch(url, { method, headers: h, body: body || undefined, credentials: 'same-origin', redirect: 'manual' })
  const text = method === 'HEAD' ? '' : await res.text()
  const duration = performance.now() - start

  const out: Record<string, string> = {}
  res.headers.forEach((v, k) => { out[k.replace(/(^|-)\w/g, (c) => c.toUpperCase())] = v })
  const size = new TextEncoder().encode(text).length
  return {
    // A redirect is hidden from scripts (opaque); say so instead of "0".
    status: res.type === 'opaqueredirect' ? 302 : res.status,
    duration_ms: Math.round(duration * 10) / 10,
    size_bytes: size,
    headers: out,
    body: res.type === 'opaqueredirect' ? '(redirect: the browser hides where to)' : text.slice(0, BODY_LIMIT),
    truncated: text.length > BODY_LIMIT,
  }
}

function jsonError(text: string): string | null {
  try { JSON.parse(text); return null } catch (e) { return (e as Error).message }
}
