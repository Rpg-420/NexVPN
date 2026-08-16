import { api } from './api'
import type { ConnectionStatus } from '../types'

let lastNotifiedState: string = ''
let lastNotifiedProfileId: string = ''

export async function requestNotificationPermission(): Promise<boolean> {
  return true
}

export function sendDesktopNotification(title: string, body: string, icon: string = '🛡️') {
  try {
    api.sendSystemNotification(`${icon} ${title}`, body)
  } catch (e) {
    console.warn('Native notification error:', e)
  }
}

export function handleGlobalStatusNotification(status: ConnectionStatus) {
  if (!status || !status.state) return

  // Ignore unchanged state
  if (status.state === lastNotifiedState && status.profile_id === lastNotifiedProfileId) {
    return
  }

  const prevState = lastNotifiedState
  lastNotifiedState = status.state
  lastNotifiedProfileId = status.profile_id || ''

  // Only notify on actual state transitions
  if (status.state === 'connected' && prevState !== 'connected') {
    const name = status.profile_name || status.server || 'Server Connected'
    sendDesktopNotification('Connected', `Connected to ${name}`, '🛡️')
  } else if (status.state === 'disconnected' && prevState === 'connected') {
    sendDesktopNotification('Disconnected', 'VPN connection terminated', '🔴')
  } else if (status.state === 'error' && prevState !== 'error') {
    sendDesktopNotification('Connection Error', status.error_msg || 'Failed to connect', '⚠️')
  }
}
