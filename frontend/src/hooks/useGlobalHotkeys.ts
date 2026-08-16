import { useEffect, useRef } from 'react'

interface HotkeyHandlers {
  onToggleConnect?: () => void
  onOpenSpeedTest?: () => void
  onOpenRadar?: () => void
  onOpenLANShare?: () => void
  onOpenEmergencyFixer?: () => void
  onOpenQuotaManager?: () => void
  onOpenDNSLeak?: () => void
  onOpenHistory?: () => void
  onOpenBackup?: () => void
  onPingAll?: () => void
}

/**
 * Global keyboard shortcuts for NexVPN Pro.
 *
 * All shortcuts use Ctrl+Shift to avoid conflicts with OS / browser hotkeys:
 *   Ctrl+Shift+V — Toggle VPN connect/disconnect
 *   Ctrl+Shift+S — Speed Test
 *   Ctrl+Shift+R — ISP Radar
 *   Ctrl+Shift+L — LAN Share
 *   Ctrl+Shift+E — Emergency Repair
 *   Ctrl+Shift+Q — Data Quota
 *   Ctrl+Shift+D — DNS Leak Test
 *   Ctrl+Shift+H — Connection History
 *   Ctrl+Shift+B — Backup & Restore
 *   Ctrl+Shift+P — Ping All Profiles
 */
export function useGlobalHotkeys(handlers: HotkeyHandlers) {
  // Use a ref so handler changes don't require re-registering the listener
  const handlersRef = useRef(handlers)
  handlersRef.current = handlers

  useEffect(() => {
    const handleKeyDown = (e: KeyboardEvent) => {
      // Skip if user is typing in an input/textarea
      const target = e.target as HTMLElement
      if (target.tagName === 'INPUT' || target.tagName === 'TEXTAREA' || target.isContentEditable) return

      if (e.ctrlKey && e.shiftKey) {
        const key = e.key.toUpperCase()
        const h = handlersRef.current

        const actions: Record<string, (() => void) | undefined> = {
          V: h.onToggleConnect,
          S: h.onOpenSpeedTest,
          R: h.onOpenRadar,
          L: h.onOpenLANShare,
          E: h.onOpenEmergencyFixer,
          Q: h.onOpenQuotaManager,
          D: h.onOpenDNSLeak,
          H: h.onOpenHistory,
          B: h.onOpenBackup,
          P: h.onPingAll,
        }

        if (actions[key]) {
          e.preventDefault()
          actions[key]!()
        }
      }
    }

    window.addEventListener('keydown', handleKeyDown)
    return () => window.removeEventListener('keydown', handleKeyDown)
  }, []) // empty — handlersRef keeps latest handlers without re-registration
}
