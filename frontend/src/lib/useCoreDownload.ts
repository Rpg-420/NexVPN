import { useCallback, useEffect, useRef, useState } from 'react'
import { api } from './api'

export interface CoreDownloadState {
  percent: number
  speed: number
  status: string
}

/**
 * useCoreDownload — one implementation of the "core:download_progress"
 * follower: progress state, an installed? flag, and the delayed progress
 * reset + status re-check at 100%.
 *
 * This pattern was copy-pasted in four pages; every copy since has drifted
 * (Aether/Psiphon used a raw setTimeout that fired after unmount and re-armed
 * a listener that lost its cleanup on early return). Centralizing it means the
 * stuck-timer class of bug gets fixed once.
 *
 * @param coreKey   e.g. 'aether', 'psiphon'
 * @param statusKey key inside checkCoreBinaries() result for this core
 */
export function useCoreDownload(coreKey: string, statusKey: string) {
  const [downloadProgress, setDownloadProgress] = useState<CoreDownloadState | null>(null)
  const [coreInstalled, setCoreInstalled] = useState(false)
  const completionTimer = useRef<ReturnType<typeof setTimeout> | null>(null)

  const checkCoreStatus = useCallback(async () => {
    try {
      const b = await api.checkCoreBinaries()
      const entry = (b as Record<string, { status?: string }> | undefined)?.[statusKey]
      setCoreInstalled(Boolean(entry && entry.status === 'installed'))
    } catch (err) {
      console.error(`Failed to check ${coreKey} core status:`, err)
    }
  }, [coreKey, statusKey])

  useEffect(() => {
    let cancelled = false
    let unsub: (() => void) | null = null

    const attach = () => {
      // Subscribing through api.onEvent gets the runtime-ready gate and the
      // correct late-attach behaviour for free.
      unsub = api.onEvent('core:download_progress', (data: {
        key?: string
        percent?: number
        speed?: number
        status?: string
      }) => {
        if (cancelled || !data || data.key !== coreKey) return
        setDownloadProgress({
          percent: data.percent || 0,
          speed: data.speed || 0,
          status: data.status || 'Downloading...',
        })
        if ((data.percent || 0) >= 100) {
          if (completionTimer.current) clearTimeout(completionTimer.current)
          // The reset timer is tracked and cleared on unmount: the copy-paste
          // versions fired setState + checkCoreStatus after the page was gone.
          completionTimer.current = setTimeout(() => {
            if (cancelled) return
            setDownloadProgress(null)
            void checkCoreStatus()
          }, 1000)
        }
      })
    }
    attach()

    void checkCoreStatus()

    return () => {
      cancelled = true
      if (completionTimer.current) clearTimeout(completionTimer.current)
      unsub?.()
    }
  }, [coreKey, checkCoreStatus])

  return { downloadProgress, setDownloadProgress, coreInstalled, checkCoreStatus }
}
