import { Badge } from '@/components/ui/badge'
import { cn } from '@/lib/utils'

// Method colors are placeholders until the color pass.
const methodColor: Record<string, string> = {
  GET: 'text-sky-600 dark:text-sky-400',
  POST: 'text-amber-600 dark:text-amber-400',
  PUT: 'text-violet-600 dark:text-violet-400',
  PATCH: 'text-violet-600 dark:text-violet-400',
  DELETE: 'text-red-600 dark:text-red-400',
}

// MethodText is the plain colored label used in lists ("GET   /api/orders").
export function MethodText({ method, className }: { method: string; className?: string }) {
  return <span className={cn('font-mono text-xs font-medium', methodColor[method] ?? 'text-muted-foreground', className)}>{method}</span>
}

// MethodTag is the boxed version for page headers.
export function MethodTag({ method }: { method: string }) {
  return (
    <span className={cn('rounded-sm bg-muted px-2.5 py-1 font-mono text-sm font-medium', methodColor[method])}>{method}</span>
  )
}

export function Methods({ methods }: { methods: string[] }) {
  return (
    <span className="inline-flex flex-wrap gap-2">
      {methods.filter((m) => m !== 'HEAD').map((m) => <MethodText key={m} method={m} />)}
    </span>
  )
}

// Status codes keep their meaning: 2xx ok, 4xx client error, 5xx/transport errors bad.
export function statusTone(code: string) {
  if (!/^\d/.test(code) || code.startsWith('5')) return 'bad'
  if (code.startsWith('4')) return 'warn'
  return 'good'
}

const toneClass = {
  good: 'bg-emerald-500/10 text-emerald-700 dark:bg-emerald-500/15 dark:text-emerald-300',
  warn: 'bg-amber-500/10 text-amber-700 dark:bg-amber-500/15 dark:text-amber-300',
  bad: 'bg-red-500/10 text-red-700 dark:bg-red-500/15 dark:text-red-300',
}

export function Status({ code, label, count }: { code: string; label?: string; count?: number }) {
  return (
    <Badge variant="outline" className={cn('h-6 border-transparent font-mono', toneClass[statusTone(code)])}>
      {code}{label && ` ${label}`}{count !== undefined && <span className="opacity-70">×{count}</span>}
    </Badge>
  )
}
