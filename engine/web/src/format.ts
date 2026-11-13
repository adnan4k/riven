export const ms = (v: number | undefined | null) =>
  v == null ? '–' : v >= 1000 ? `${(v / 1000).toFixed(2)}s` : `${v.toFixed(1)}ms`

export const pct = (v: number) => `${(v * 100).toFixed(1)}%`

export const num = (v: number, digits = 1) => v.toLocaleString(undefined, { maximumFractionDigits: digits })

export const when = (iso: string) =>
  new Date(iso).toLocaleString(undefined, { dateStyle: 'medium', timeStyle: 'short' })

// Short class name: App\Http\Controllers\UserController → UserController
export const short = (cls: string | null) => (cls ? cls.slice(cls.lastIndexOf('\\') + 1) : '')

export function ago(iso: string, now = Date.now()) {
  const s = Math.max(0, Math.round((now - new Date(iso).getTime()) / 1000))
  if (s < 60) return 'just now'
  if (s < 3600) return `${Math.round(s / 60)}m ago`
  if (s < 86400) return `${Math.round(s / 3600)}h ago`
  return `${Math.round(s / 86400)}d ago`
}

export const clock = (msTotal: number) => {
  const s = Math.floor(msTotal / 1000)
  return `${Math.floor(s / 60)}:${String(s % 60).padStart(2, '0')}`
}

export const bytes = (n: number) => (n < 1024 ? `${n} B` : `${(n / 1024).toFixed(1)} KB`)
