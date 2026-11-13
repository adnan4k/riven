import type { RouteStats } from '@/types'
import { cn } from '@/lib/utils'

// The sidebar dot: how the route did in the latest stress run.
export function dotFor(s?: RouteStats): { tone: string; label: string } {
  if (!s) return { tone: 'bg-muted-foreground/30', label: 'not stress tested yet' }
  if (s.error_rate > 0) return { tone: 'bg-red-500', label: `${(s.error_rate * 100).toFixed(1)}% failed in the latest run` }
  if (Object.keys(s.statuses).some((c) => c.startsWith('4'))) return { tone: 'bg-amber-500', label: '4xx answers in the latest run' }
  return { tone: 'bg-emerald-500', label: 'healthy in the latest run' }
}

export function StatusDot({ stats }: { stats?: RouteStats }) {
  const { tone, label } = dotFor(stats)
  return <span className={cn('size-2 shrink-0 rounded-full', tone)} title={label} aria-label={label} role="img" />
}
