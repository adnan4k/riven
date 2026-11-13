import { useEffect, useState } from 'react'

export type Loaded<T> =
  | { state: 'loading' }
  | { state: 'error'; message: string }
  | { state: 'ok'; data: T }

async function parse<T>(res: Response, path: string): Promise<T> {
  const body = await res.json().catch(() => null)
  if (!res.ok) throw new Error(body?.error ?? `${path} answered ${res.status}`)
  return body as T
}

export const getJSON = <T,>(path: string) => fetch(path).then((r) => parse<T>(r, path))

// postJSON always sends JSON: the server refuses anything else (see server/actions.go guard).
export const postJSON = <T,>(path: string, body: unknown) =>
  fetch(path, { method: 'POST', headers: { 'Content-Type': 'application/json' }, body: JSON.stringify(body) })
    .then((r) => parse<T>(r, path))

// useJSON fetches an API path; changing `path` or `version` refetches.
// Data already shown stays visible while a refetch runs.
export function useJSON<T>(path: string | null, version = 0): Loaded<T> {
  const [result, setResult] = useState<Loaded<T>>({ state: 'loading' })

  useEffect(() => {
    if (!path) return
    let cancelled = false
    getJSON<T>(path).then(
      (data) => !cancelled && setResult({ state: 'ok', data }),
      (err: Error) => !cancelled && setResult({ state: 'error', message: err.message }),
    )
    return () => {
      cancelled = true
    }
  }, [path, version])

  return result
}
