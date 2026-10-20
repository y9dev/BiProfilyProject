// Small data-loading hook: { status, data, error, reload }.
// status: 'loading' | 'ready' | 'error'. `load` receives an AbortSignal.
import { useCallback, useEffect, useRef, useState } from 'react'

export function useAsync(load, deps = []) {
  const [state, setState] = useState({ status: 'loading', data: null, error: null })
  const [tick, setTick] = useState(0)
  const loadRef = useRef(load)
  useEffect(() => {
    loadRef.current = load
  })

  useEffect(() => {
    const ctrl = new AbortController()
    setState((s) => (s.status === 'loading' ? s : { status: 'loading', data: s.data, error: null }))
    loadRef
      .current(ctrl.signal)
      .then((data) => {
        if (!ctrl.signal.aborted) setState({ status: 'ready', data, error: null })
      })
      .catch((error) => {
        if (!ctrl.signal.aborted) setState({ status: 'error', data: null, error })
      })
    return () => ctrl.abort()
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [tick, ...deps])

  const reload = useCallback(() => setTick((n) => n + 1), [])
  return { ...state, reload }
}
