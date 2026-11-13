// Mirrors of the Go JSON (engine/manifest, engine/report). Field names match
// the wire format exactly; null means "unknown" as in the manifest contract.

export interface Manifest {
  schema_version: number
  generated_at: string
  app: { name: string | null; url: string | null; laravel_version: string; php_version: string }
  routes: ManifestRoute[]
  warnings: Warning[]
}

export type Route = ManifestRoute

export interface ManifestRoute {
  id: string
  uri: string
  methods: string[]
  name: string | null
  domain: string | null
  action: { type: 'controller' | 'invokable' | 'closure'; class: string | null; method: string | null }
  middleware: string[]
  middleware_resolved: string[]
  parameters: { name: string; required: boolean; pattern: string | null }[]
  request: { form_request: string | null; rules: Record<string, string[]> | null }
  response: { resource: string | null; collection: boolean | null }
}

export interface Warning {
  route_id: string
  code: string
  message: string
}

export interface Latency {
  min: number; mean: number; p50: number; p90: number; p95: number; p99: number; max: number
}

export interface Stats {
  requests: number
  elapsed_ms: number
  rps: number
  latency_ms: Latency | null
  statuses: Record<string, number>
  errors: Record<string, number>
  error_rate: number
}

export interface RouteStats extends Stats {
  route_id: string
  method: string
  url: string
}

export interface ReportConfig {
  base_url: string
  concurrency: number
  requests: number
  duration_ms: number
  timeout_ms: number
}

export interface StressReport {
  report_version: number
  kind: 'stress'
  started_at: string
  finished_at: string
  interrupted: boolean
  manifest: { generated_at: string; app_name: string | null; app_url: string | null }
  config: ReportConfig
  summary: Stats
  routes: RouteStats[]
  timeline: TimelinePoint[]
}

export interface TimelinePoint {
  second: number
  requests: number
  failed: number
  p50_ms: number
  p95_ms: number
}

export interface ReportSummary {
  name: string
  started_at: string
  interrupted: boolean
  config: ReportConfig
  summary: Stats
  routes: number
}

// The parts of OpenAPI the route view reads.
export type JSONSchema = {
  type?: string | string[]
  format?: string
  properties?: Record<string, JSONSchema>
  required?: string[]
  items?: JSONSchema
  enum?: unknown[]
  pattern?: string
  minimum?: number; maximum?: number
  minLength?: number; maxLength?: number
  minItems?: number; maxItems?: number
  not?: JSONSchema
  'x-laravel-rules'?: string[]
}

export interface Operation {
  operationId: string
  parameters?: { name: string; in: string; required: boolean; schema: JSONSchema }[]
  requestBody?: {
    description?: string
    content: Record<string, { schema: JSONSchema }>
    'x-riven-rules-unknown'?: boolean
    'x-riven-unmapped-rules'?: string[]
  }
  responses: Record<string, { description: string; content?: Record<string, { schema: unknown }> }>
}

export interface OpenAPIDoc {
  paths: Record<string, Record<string, Operation>>
}

export interface AppConfig {
  base_url: string
  can_rescan: boolean
}

export interface TryResponse {
  status: number
  duration_ms: number
  size_bytes: number
  headers: Record<string, string>
  body: string
  truncated: boolean
}

export interface StressState {
  state: 'idle' | 'running' | 'done' | 'failed'
  id?: string
  elapsed_ms: number
  duration_ms: number
  interrupted: boolean
  report?: string
  error?: string
  live?: StressReport
}

export interface FakeResult {
  route_id: string
  form_request: string
  bodies: unknown[]
  invalid: number
  errors: Record<string, string[]>
  warnings: string[]
}
