import { useEffect, useState, type ComponentProps, type MouseEvent } from 'react'

// A tiny pushState router: three views don't need a router library.
// The Go server answers index.html for unknown paths, so reloads work.

export function usePath(): string {
  const [path, setPath] = useState(window.location.pathname)
  useEffect(() => {
    const onPop = () => setPath(window.location.pathname)
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
  window.history.pushState(null, '', to)
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
  return <a href={to} onClick={handle} {...props} />
}

export const routePath = (id: string, tab?: string) => '/routes/' + encodeURIComponent(id) + (tab ? '/' + tab : '')
export const reportPath = (name: string) => '/reports/' + encodeURIComponent(name)
