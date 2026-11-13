import type { TimelinePoint } from '@/types'

// Latency over time: p95 and p50 lines (left axis, ms), throughput dashed
// (right axis, req/s), and an optional threshold line. Plain SVG, one scale per axis.
export function LatencyChart({ points, threshold }: { points: TimelinePoint[]; threshold?: number }) {
  const W = 640, H = 220, L = 40, R = 40, T = 12, B = 26
  const w = W - L - R, h = H - T - B

  const n = Math.max(points.length, 2)
  const maxMs = niceMax(Math.max(threshold ?? 0, ...points.map((p) => p.p95_ms), 1))
  const maxRps = niceMax(Math.max(...points.map((p) => p.requests), 1))
  const x = (i: number) => L + (i / (n - 1)) * w
  const y = (v: number) => T + h - (v / maxMs) * h
  const yr = (v: number) => T + h - (v / maxRps) * h
  const line = (f: (p: TimelinePoint) => number, scale: (v: number) => number) =>
    points.map((p, i) => `${i ? 'L' : 'M'}${x(i).toFixed(1)},${scale(f(p)).toFixed(1)}`).join('')

  const ticks = [0, maxMs / 2, maxMs]
  const last = points.length ? points[points.length - 1].second : 0

  return (
    <svg viewBox={`0 0 ${W} ${H}`} className="w-full text-muted-foreground" role="img"
      aria-label={`Latency over ${last + 1} seconds; p95 peak ${Math.max(0, ...points.map((p) => p.p95_ms)).toFixed(0)} ms`}>
      {ticks.map((t) => (
        <g key={t}>
          <line x1={L} x2={W - R} y1={y(t)} y2={y(t)} stroke="currentColor" strokeOpacity={0.15} />
          <text x={L - 6} y={y(t) + 3} textAnchor="end" fontSize="10" fill="currentColor" className="font-mono">{Math.round(t)}</text>
        </g>
      ))}
      <text x={W - R + 6} y={yr(maxRps) + 3} fontSize="10" fill="currentColor" className="font-mono">{Math.round(maxRps)}/s</text>
      {[0, Math.floor((n - 1) / 2), n - 1].map((i) => (
        <text key={i} x={x(i)} y={H - 8} textAnchor="middle" fontSize="10" fill="currentColor" className="font-mono">
          {fmt(points[i]?.second ?? i)}
        </text>
      ))}
      {threshold ? (
        <g className="text-red-500">
          <line x1={L} x2={W - R} y1={y(threshold)} y2={y(threshold)} stroke="currentColor" strokeDasharray="2 3" />
          <text x={W - R - 4} y={y(threshold) - 4} textAnchor="end" fontSize="10" fill="currentColor" className="font-mono">threshold {threshold}ms</text>
        </g>
      ) : null}
      {points.length > 1 && (
        <>
          <path d={line((p) => p.requests, yr)} fill="none" stroke="currentColor" strokeOpacity={0.6} strokeDasharray="4 3" className="text-amber-500" />
          <path d={line((p) => p.p50_ms, y)} fill="none" stroke="currentColor" strokeWidth={1.5} strokeOpacity={0.55} />
          <path d={line((p) => p.p95_ms, y)} fill="none" stroke="currentColor" strokeWidth={2} className="text-foreground" />
        </>
      )}
    </svg>
  )
}

export function ChartLegend() {
  return (
    <span className="flex items-center gap-4 text-xs text-muted-foreground">
      <span className="flex items-center gap-1.5"><i className="h-0.5 w-4 bg-foreground" />p95</span>
      <span className="flex items-center gap-1.5"><i className="h-0.5 w-4 bg-muted-foreground/60" />p50</span>
      <span className="flex items-center gap-1.5"><i className="w-4 border-t border-dashed border-amber-500" />req/s</span>
    </span>
  )
}

function niceMax(v: number) {
  const step = 10 ** Math.floor(Math.log10(v))
  return Math.ceil(v / step) * step
}

const fmt = (s: number) => `${Math.floor(s / 60)}:${String(s % 60).padStart(2, '0')}`
