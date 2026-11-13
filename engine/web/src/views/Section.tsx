import type { ReactNode } from 'react'
import { Card, CardAction, CardContent, CardDescription, CardHeader, CardTitle } from '@/components/ui/card'

// A titled card; `flush` lets a table run edge to edge.
export function Section({ title, description, action, flush, children }: {
  title: string
  description?: ReactNode
  action?: ReactNode
  flush?: boolean
  children: ReactNode
}) {
  return (
    <Card className={flush ? 'pb-0' : undefined}>
      <CardHeader>
        <CardTitle>{title}</CardTitle>
        {description && <CardDescription>{description}</CardDescription>}
        {action && <CardAction>{action}</CardAction>}
      </CardHeader>
      <CardContent className={flush ? 'px-0' : undefined}>{children}</CardContent>
    </Card>
  )
}
