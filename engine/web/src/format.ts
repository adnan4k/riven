export const ms = (v: number | undefined | null) =>
  v == null ? '–' : v >= 1000 ? `${(v / 1000).toFixed(2)}s` : `${v.toFixed(1)}ms`

export const pct = (v: number) => `${(v * 100).toFixed(1)}%`

export const num = (v: number, digits = 1) => v.toLocaleString(undefined, { maximumFractionDigits: digits })

export const when = (iso: string) =>
  new Date(iso).toLocaleString(undefined, { dateStyle: 'medium', timeStyle: 'short' })

// Short class name: App\Http\Controllers\UserController → UserController
export const short = (cls: string | null) => (cls ? cls.slice(cls.lastIndexOf('\\') + 1) : '')
