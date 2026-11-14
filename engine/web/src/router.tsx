import { useEffect, useState, type ComponentProps, type MouseEvent } from 'react'

// A tiny pushState router: three views don't need a router library.
// The server answers index.html for unknown paths, so reloads work.
//
// Paths inside the app ("/routes/…") are relative to where the dashboard is
// mounted: <base href> is "/" under riven-engine serve and "/riven/" in Laravel.
const BASE = new URL(document.baseURI).pathname.replace(/\/$/, '')

const current = () => window.location.pathname.slice(BASE.length) || '/'

export function usePath(): string {
  const [path, setPath] = useState(current)
  useEffect(() => {
    const onPop = () => setPath(current())
    window.addEventListener('popstate', onPop)
    window.addEventListener('riven:navigate', onPop)
    return () => {
      window.removeEventListener('popstate', onPop)
      window.removeEventListener('riven:navigate', onPop)
    }
  }, [])
  return path
}

export function navigate(to: string) {
  window.history.pushState(null, '', BASE + to)
  window.dispatchEvent(new Event('riven:navigate'))
  window.scrollTo(0, 0)
}

// Link passes other props through, so shadcn components can render it via asChild.
export function Link({ to, onClick, ...props }: ComponentProps<'a'> & { to: string }) {
  const handle = (e: MouseEvent<HTMLAnchorElement>) => {
    onClick?.(e)
    if (e.defaultPrevented || e.metaKey || e.ctrlKey || e.shiftKey || e.button !== 0) return // new tab etc.
    e.preventDefault()
    navigate(to)
  }
  return <a href={BASE + to} onClick={handle} {...props} />
}

export const routePath = (id: string, tab?: string) => '/routes/' + encodeURIComponent(id) + (tab ? '/' + tab : '')
export const reportPath = (name: string) => '/reports/' + encodeURIComponent(name)
