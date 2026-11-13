import { Badge } from '@/components/ui/badge'

// HTTP methods as badges, using theme variants only: writes stand out,
// DELETE is destructive.
const variant = {
  GET: 'secondary',
  POST: 'default',
  PUT: 'outline',
  PATCH: 'outline',
  DELETE: 'destructive',
} as const

export function Method({ method }: { method: string }) {
  const v = variant[method as keyof typeof variant] ?? 'outline'
  return <Badge variant={v} className="font-mono text-[11px] tracking-wide">{method}</Badge>
}

export function Methods({ methods }: { methods: string[] }) {
  return (
    <span className="inline-flex flex-wrap gap-1">
      {methods.filter((m) => m !== 'HEAD').map((m) => <Method key={m} method={m} />)}
    </span>
  )
}

// Status codes keep their meaning: 5xx and transport errors are destructive.
export function Status({ code, count }: { code: string; count?: number }) {
  const bad = code === 'error' || code.startsWith('5') || !/^\d/.test(code)
  const v = bad ? 'destructive' : code.startsWith('4') ? 'outline' : 'secondary'
  return (
    <Badge variant={v} className="font-mono">
      {code}{count !== undefined && <span className="opacity-70">×{count}</span>}
    </Badge>
  )
}
