import { Card, CardDescription, CardHeader, CardTitle } from '@/components/ui/card'
import { cn } from '@/lib/utils'

export function Stat({ label, value, tone }: { label: string; value: string | number; tone?: 'warn' | 'bad' }) {
  return (
    <Card size="sm" className={cn(tone === 'bad' && 'ring-destructive/40', tone === 'warn' && 'ring-amber-500/40')}>
      <CardHeader>
        <CardDescription className="label-mono">{label}</CardDescription>
        <CardTitle className={cn('font-mono text-2xl font-medium tabular-nums', tone === 'bad' && 'text-destructive')}>{value}</CardTitle>
      </CardHeader>
    </Card>
  )
}

export function StatGrid({ children }: { children: React.ReactNode }) {
  return <div className="grid grid-cols-2 gap-3 md:grid-cols-3 xl:grid-cols-6">{children}</div>
}
