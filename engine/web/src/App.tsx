import { Gauge, Waypoints } from 'lucide-react'
import { useJSON } from '@/api'
import { Link, usePath } from '@/router'
import type { Manifest, OpenAPIDoc, ReportSummary } from '@/types'
import { when } from '@/format'
import { Alert, AlertDescription, AlertTitle } from '@/components/ui/alert'
import { Separator } from '@/components/ui/separator'
import { Skeleton } from '@/components/ui/skeleton'
import { TooltipProvider } from '@/components/ui/tooltip'
import {
  Sidebar, SidebarContent, SidebarFooter, SidebarGroup, SidebarGroupContent, SidebarGroupLabel,
  SidebarHeader, SidebarInset, SidebarMenu, SidebarMenuBadge, SidebarMenuButton, SidebarMenuItem,
  SidebarProvider, SidebarTrigger,
} from '@/components/ui/sidebar'
import { RoutesView } from '@/views/RoutesView'
import { RouteDetail } from '@/views/RouteDetail'
import { ReportsView } from '@/views/ReportsView'
import { ReportDetail } from '@/views/ReportDetail'

export function App() {
  const path = usePath()
  const manifest = useJSON<Manifest>('/api/manifest')
  const openapi = useJSON<OpenAPIDoc>('/api/openapi')
  const reports = useJSON<ReportSummary[]>('/api/reports')

  const [, section, param] = path.split('/')
  const onReports = section === 'reports'
  const title = onReports ? (param ? 'Stress run' : 'Stress reports') : param ? 'Route' : 'Routes'

  return (
    <TooltipProvider>
      <SidebarProvider>
        <Sidebar collapsible="icon">
          <SidebarHeader>
            <SidebarMenu>
              <SidebarMenuItem>
                <SidebarMenuButton size="lg" asChild>
                  <Link to="/">
                    <span className="flex size-8 items-center justify-center rounded-lg bg-primary font-bold text-primary-foreground">R</span>
                    <span className="flex flex-col leading-tight">
                      <span className="font-semibold">Riven</span>
                      <span className="text-xs text-muted-foreground">{manifest.state === 'ok' ? manifest.data.app.name ?? 'Laravel app' : '…'}</span>
                    </span>
                  </Link>
                </SidebarMenuButton>
              </SidebarMenuItem>
            </SidebarMenu>
          </SidebarHeader>

          <SidebarContent>
            <SidebarGroup>
              <SidebarGroupLabel>Explore</SidebarGroupLabel>
              <SidebarGroupContent>
                <SidebarMenu>
                  <SidebarMenuItem>
                    <SidebarMenuButton asChild isActive={!onReports} tooltip="Routes">
                      <Link to="/"><Waypoints /><span>Routes</span></Link>
                    </SidebarMenuButton>
                    {manifest.state === 'ok' && <SidebarMenuBadge>{manifest.data.routes.length}</SidebarMenuBadge>}
                  </SidebarMenuItem>
                  <SidebarMenuItem>
                    <SidebarMenuButton asChild isActive={onReports} tooltip="Stress reports">
                      <Link to="/reports"><Gauge /><span>Stress reports</span></Link>
                    </SidebarMenuButton>
                    {reports.state === 'ok' && <SidebarMenuBadge>{reports.data.length}</SidebarMenuBadge>}
                  </SidebarMenuItem>
                </SidebarMenu>
              </SidebarGroupContent>
            </SidebarGroup>
          </SidebarContent>

          {manifest.state === 'ok' && (
            <SidebarFooter className="group-data-[collapsible=icon]:hidden">
              <dl className="grid grid-cols-[auto_1fr] gap-x-3 gap-y-1 px-2 pb-1 text-xs">
                <dt className="text-muted-foreground">Laravel</dt><dd className="font-mono">{manifest.data.app.laravel_version}</dd>
                <dt className="text-muted-foreground">PHP</dt><dd className="font-mono">{manifest.data.app.php_version}</dd>
                <dt className="text-muted-foreground">Exported</dt><dd title="When php artisan riven:export ran">{when(manifest.data.generated_at)}</dd>
              </dl>
            </SidebarFooter>
          )}
        </Sidebar>

        <SidebarInset>
          <header className="flex h-14 shrink-0 items-center gap-2 border-b px-4">
            <SidebarTrigger className="-ml-1" />
            <Separator orientation="vertical" className="mr-2 data-[orientation=vertical]:h-4" />
            <h1 className="text-sm font-medium">{title}</h1>
          </header>

          <main className="mx-auto flex w-full max-w-7xl flex-col gap-6 p-4 md:p-6">
            {manifest.state === 'loading' && (
              <div className="flex flex-col gap-3">
                <Skeleton className="h-20 w-full" />
                <Skeleton className="h-64 w-full" />
              </div>
            )}
            {manifest.state === 'error' && (
              <Alert variant="destructive">
                <AlertTitle>Can't read the manifest</AlertTitle>
                <AlertDescription>{manifest.message}</AlertDescription>
              </Alert>
            )}
            {manifest.state === 'ok' && (
              <View section={section} param={param} manifest={manifest.data} openapi={openapi} reports={reports} />
            )}
          </main>
        </SidebarInset>
      </SidebarProvider>
    </TooltipProvider>
  )
}

function View({ section, param, manifest, openapi, reports }: {
  section: string
  param?: string
  manifest: Manifest
  openapi: ReturnType<typeof useJSON<OpenAPIDoc>>
  reports: ReturnType<typeof useJSON<ReportSummary[]>>
}) {
  const id = param ? decodeURIComponent(param) : ''
  const latest = reports.state === 'ok' ? reports.data[0]?.name : undefined

  if (section === 'routes' && id) {
    return <RouteDetail manifest={manifest} id={id} openapi={openapi.state === 'ok' ? openapi.data : null} latestReport={latest} />
  }
  if (section === 'reports' && id) return <ReportDetail name={id} />
  if (section === 'reports') return <ReportsView reports={reports} />
  return <RoutesView manifest={manifest} />
}
