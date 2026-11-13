import { statusTone } from './Method'

const barTone = { good: 'bg-emerald-500', warn: 'bg-amber-500', bad: 'bg-red-500' }

// A single stacked bar of status codes (and transport errors) by share of requests.
export function StatusBar({ statuses, errors }: { statuses: Record<string, number>; errors: Record<string, number> }) {
  const entries = [...Object.entries(statuses), ...Object.entries(errors)].sort(([a], [b]) => a.localeCompare(b))
  const total = entries.reduce((n, [, c]) => n + c, 0)
  if (!total) return <p className="text-sm text-muted-foreground">No requests yet.</p>

  return (
    <div className="flex flex-col gap-2">
      <div className="flex h-2.5 overflow-hidden rounded-full bg-muted" role="img"
        aria-label={entries.map(([c, n]) => `${c}: ${n}`).join(', ')}>
        {entries.map(([code, n]) => (
          <span key={code} className={barTone[statusTone(code)]} style={{ width: `${(n / total) * 100}%` }} />
        ))}
      </div>
      <p className="flex flex-wrap gap-x-4 font-mono text-sm">
        {entries.map(([code, n]) => <span key={code}>{code} · {((n / total) * 100).toFixed(n / total < 0.1 ? 1 : 0)}%</span>)}
      </p>
    </div>
  )
}
