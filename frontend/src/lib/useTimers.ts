import { useCallback, useEffect, useRef } from 'react'

/**
 * A `setTimeout` that is cancelled automatically when the component unmounts.
 *
 * A plain `setTimeout` started from an event handler keeps running after the
 * route/component is torn down, so its callback can call `setState` on an
 * unmounted component (React warning, and in some cases a real leak when the
 * callback closes over large objects).
 *
 * Usage:
 *   const safeTimeout = useTimers()
 *   safeTimeout(() => setToast(null), 4000)
 */
export function useTimers() {
  const timers = useRef<Set<ReturnType<typeof setTimeout>>>(new Set())
  const mounted = useRef(true)

  useEffect(() => {
    mounted.current = true
    return () => {
      mounted.current = false
      timers.current.forEach(clearTimeout)
      timers.current.clear()
    }
  }, [])

  return useCallback((fn: () => void, ms: number) => {
    const id = setTimeout(() => {
      timers.current.delete(id)
      if (mounted.current) {
        fn()
      }
    }, ms)
    timers.current.add(id)
    return id
  }, [])
}
