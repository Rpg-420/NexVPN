import { useEffect } from 'react'

interface HotkeyHandlers {
  onToggleConnect?: () => void
  onOpenSpeedTest?: () => void
  onOpenRadar?: () => void
  onOpenLANShare?: () => void
  onOpenEmergencyFixer?: () => void
  onOpenQuotaManager?: () => void
}

export function useGlobalHotkeys(handlers: HotkeyHandlers) {
  useEffect(() => {
    const handleKeyDown = (e: KeyboardEvent) => {
      // Check for Ctrl + Shift combinations
      if (e.ctrlKey && e.shiftKey) {
        const key = e.key.toUpperCase()

        if (key === 'V' && handlers.onToggleConnect) {
          e.preventDefault()
          handlers.onToggleConnect()
        } else if (key === 'S' && handlers.onOpenSpeedTest) {
          e.preventDefault()
          handlers.onOpenSpeedTest()
        } else if (key === 'R' && handlers.onOpenRadar) {
          e.preventDefault()
          handlers.onOpenRadar()
        } else if (key === 'L' && handlers.onOpenLANShare) {
          e.preventDefault()
          handlers.onOpenLANShare()
        } else if (key === 'E' && handlers.onOpenEmergencyFixer) {
          e.preventDefault()
          handlers.onOpenEmergencyFixer()
        } else if (key === 'Q' && handlers.onOpenQuotaManager) {
          e.preventDefault()
          handlers.onOpenQuotaManager()
        }
      }
    }

    window.addEventListener('keydown', handleKeyDown)
    return () => window.removeEventListener('keydown', handleKeyDown)
  }, [handlers])
}
