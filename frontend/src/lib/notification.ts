import { api } from './api'
import { t } from './i18n'
import { translateBackendError } from './backendErrors'
import { announce } from './announcer'
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
    const name = status.profile_name || status.server || t('notif_unknown_server')
    announce(t('notif_title_connected') + ' — ' + t('notif_body_connected', { name }))
    sendDesktopNotification(t('notif_title_connected'), t('notif_body_connected', { name }), '🛡️')
  } else if (status.state === 'disconnected' && prevState === 'connected') {
    announce(t('notif_title_disconnected'))
    sendDesktopNotification(t('notif_title_disconnected'), t('notif_body_disconnected'), '🔴')
  } else if (status.state === 'error' && prevState !== 'error') {
    const reason = translateBackendError(status.error_msg) || t('notif_body_error')
    announce(t('notif_title_error') + ' — ' + reason, 'assertive')
    sendDesktopNotification(t('notif_title_error'), reason, '⚠️')
  }
}
