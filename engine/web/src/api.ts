import { useEffect, useState } from 'react'

export type Loaded<T> =
  | { state: 'loading' }
  | { state: 'error'; message: string }
  | { state: 'ok'; data: T }

async function getJSON<T>(path: string): Promise<T> {
  const res = await fetch(path)
  const body = await res.json().catch(() => null)
  if (!res.ok) {
    throw new Error(body?.error ?? `${path} answered ${res.status}`)
  }
  return body as T
}

// useJSON fetches an API path; changing `path` refetches.
export function useJSON<T>(path: string): Loaded<T> {
  const [result, setResult] = useState<Loaded<T>>({ state: 'loading' })

  useEffect(() => {
    let cancelled = false
    setResult({ state: 'loading' })
    getJSON<T>(path).then(
      (data) => !cancelled && setResult({ state: 'ok', data }),
      (err: Error) => !cancelled && setResult({ state: 'error', message: err.message }),
    )
    return () => {
      cancelled = true
    }
  }, [path])

  return result
}
