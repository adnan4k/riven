import type { JSONSchema, Manifest, OpenAPIDoc, Operation, Route } from '@/types'

export const mainMethod = (r: Route) => r.id.split(' ')[0]
export const displayUri = (r: Route) => '/' + r.uri.replace(/^\//, '')

// group is the section a route belongs to in the sidebar, matching the
// OpenAPI tags: the first path segment after "api" and version prefixes.
export function group(r: Route): string {
  for (const seg of r.uri.split('/')) {
    if (seg && seg !== 'api' && !seg.startsWith('{') && !/^v\d+$/.test(seg)) return seg
  }
  return 'web'
}

export function groups(m: Manifest): [string, Route[]][] {
  const map = new Map<string, Route[]>()
  for (const r of m.routes) {
    const g = group(r)
    map.set(g, [...(map.get(g) ?? []), r])
  }
  return [...map.entries()]
}

// The OpenAPI operation for a route (its full path keeps every parameter).
export function operation(doc: OpenAPIDoc | null, r: Route): Operation | undefined {
  if (!doc) return undefined
  const path = '/' + r.uri.replace(/^\//, '').replace(/\?\}/g, '}')
  return doc.paths[path === '//' ? '/' : path]?.[mainMethod(r).toLowerCase()]
}

// fillPath substitutes route parameters; missing optional ones are dropped.
export function fillPath(r: Route, params: Record<string, string>): string {
  const filled = r.uri.replace(/\{(\w+)(\?)?\}/g, (_, name: string, opt?: string) =>
    params[name] ? encodeURIComponent(params[name]) : opt ? '' : `{${name}}`,
  )
  return ('/' + filled.replace(/^\//, '')).replace(/\/{2,}/g, '/').replace(/(.)\/$/, '$1')
}

export interface Field { name: string; required: boolean; schema: JSONSchema }

// flatten turns nested objects/arrays into Laravel-style dotted names: items.*.id
export function flatten(schema: JSONSchema, prefix = ''): Field[] {
  const out: Field[] = []
  for (const [name, s] of Object.entries(schema.properties ?? {})) {
    const full = prefix + name
    out.push({ name: full, required: schema.required?.includes(name) ?? false, schema: s })
    if (s.properties) out.push(...flatten(s, full + '.'))
    if (s.items?.properties) out.push(...flatten(s.items, full + '.*.'))
  }
  return out
}

export function typeOf(s: JSONSchema): string {
  const t = Array.isArray(s.type) ? s.type.filter((x) => x !== 'null').join(' | ') : s.type ?? 'any'
  if (t === 'array' && s.items) return `${typeOf(s.items)}[]`
  return t
}

// example builds a plausible request body from the schema, so Try it starts
// filled in. Name hints only shape string values; types always win.
export function example(s: JSONSchema, name = ''): unknown {
  if (s.enum?.length) return s.enum[0]
  const t = Array.isArray(s.type) ? s.type.find((x) => x !== 'null') : s.type
  if (t === 'object' || s.properties) {
    const out: Record<string, unknown> = {}
    for (const [k, v] of Object.entries(s.properties ?? {})) out[k] = example(v, k)
    return out
  }
  if (t === 'array') return s.items ? [example(s.items, name)] : []
  if (t === 'integer') return Math.max(1, s.minimum ?? 1)
  if (t === 'number') return Math.max(1, s.minimum ?? 1)
  if (t === 'boolean') return true
  if (s.format === 'email' || /email/.test(name)) return 'jane@example.com'
  if (s.format === 'uri') return 'https://example.com'
  if (s.format === 'uuid') return '00000000-0000-4000-8000-000000000000'
  if (s.format === 'date') return '2026-01-01'
  if (/password/.test(name)) return 'Password123!'
  if (/name/.test(name)) return 'Jane Doe'
  const min = s.minLength ?? 0
  const text = 'example'
  return text.length >= min ? text.slice(0, s.maxLength ?? text.length) : text.padEnd(min, 'x')
}

// curl renders a copy-pasteable command; values are single-quoted for a POSIX shell.
export function curl(method: string, url: string, headers: Record<string, string>, body: string): string {
  const q = (v: string) => `'${v.replace(/'/g, `'\\''`)}'`
  const parts = [`curl -X ${method} ${q(url)}`]
  for (const [k, v] of Object.entries(headers)) parts.push(`  -H ${q(`${k}: ${v}`)}`)
  if (body) parts.push(`  --data ${q(body)}`)
  return parts.join(' \\\n')
}
