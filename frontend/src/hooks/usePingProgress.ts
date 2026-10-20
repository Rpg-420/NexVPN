import { useEffect, useState } from 'react'

import { api } from '../lib/api'
import type { ProfilePingProgress } from '../types'

/**
 * Subscribe to the backend's bulk-ping progress stream.
 *
 * The sweep runs inside a single blocking binding call, so the only way to show
 * how far along it is, is to listen to the events Go emits while it works. This
 * hook owns that subscription so the three list pages do not each re-implement
 * it — and so that a page which forgets to unsubscribe cannot leak a listener
 * per mount.
 *
 * `active` clears the readout when the sweep ends. It is a separate effect
 * rather than part of the subscription because the last progress event arrives
 * *before* the call returns, so clearing on completion has to be driven by the
 * caller's own state.
 */
export function usePingProgress(active: boolean): { done: number; total: number } | null {
  const [progress, setProgress] = useState<{ done: number; total: number } | null>(null)

  useEffect(() => {
    const unsubscribe = api.onEvent('profiles:ping_progress', (p: ProfilePingProgress) => {
      if (!p || typeof p.total_count !== 'number') return
      setProgress({ done: p.tested_count, total: p.total_count })
    })
    return () => unsubscribe()
  }, [])

  useEffect(() => {
    if (!active) setProgress(null)
  }, [active])

  return progress
}
