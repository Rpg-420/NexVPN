import { useCallback, useEffect, useRef, useState } from 'react'

export type ToastKind = 'info' | 'success' | 'error'

export interface ToastState {
  message: string
  kind: ToastKind
}

/**
 * useToast — single implementation of the toast pattern that was previously
 * copy-pasted in Dashboard, V2RayPage, WarpWireGuardPage and DiscoveryHub with
 * four different timer strategies.
 *
 * It fixes the shared bug class: raw setTimeout that (a) fired after unmount
 * and (b) let a second toast's timer hide the first message early. One timer
 * per hook instance, cleared on unmount and re armed per show.
 *
 * @param durationMs how long a toast stays visible (default 4s, matching the
 *                   longest behaviour in the pages it replaces)
 */
export function useToast(durationMs = 4000) {
  const [toast, setToast] = useState<ToastState | null>(null)
  const timer = useRef<ReturnType<typeof setTimeout> | null>(null)

  const dismiss = useCallback(() => {
    if (timer.current) clearTimeout(timer.current)
    timer.current = null
    setToast(null)
  }, [])

  const showToast = useCallback(
    (message: string, kind: ToastKind = 'info') => {
      if (timer.current) clearTimeout(timer.current)
      setToast({ message, kind })
      timer.current = setTimeout(() => {
        timer.current = null
        setToast(null)
      }, durationMs)
    },
    [durationMs]
  )

  useEffect(
    () => () => {
      if (timer.current) clearTimeout(timer.current)
    },
    []
  )

  return { toast, showToast, dismiss }
}
