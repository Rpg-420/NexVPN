import { t } from './i18n'

/**
 * Translation for error strings produced by the Go backend.
 *
 * The engine builds its error messages in English — they are also what lands in
 * the launch diary and in bug reports, so they must stay English — but they were
 * being rendered verbatim into a Persian, right-to-left interface. A user whose
 * connection failed was shown a paragraph of English explaining what to do.
 *
 * Each entry matches a stable prefix: the part of the message the backend does
 * not interpolate. Anything after the prefix (a server name, a core error, a
 * process name) is not translated, so the mapping stays valid when the dynamic
 * part changes.
 */
const ERROR_PATTERNS: Array<{ prefix: string; key: string }> = [
  // engine.go — tunnel verification
  { prefix: 'Server is not reachable through the tunnel', key: 'err_server_unreachable' },
  { prefix: 'Tunnel verification failed through the core', key: 'err_server_unreachable' },
  // engine.go — local readiness
  { prefix: 'proxy port 10808 did not become available', key: 'err_proxy_port_unavailable' },
  { prefix: 'local proxy port 10808 is still in use', key: 'err_proxy_port_busy' },
  // engine.go — core lifecycle
  { prefix: 'Core process crashed', key: 'err_core_crashed' },
  { prefix: 'Core process exited unexpectedly', key: 'err_core_crashed' },
  { prefix: 'Connection failed on', key: 'err_connection_failed' },
  { prefix: 'TUN Adapter startup failed', key: 'err_tun_failed' },
  // killswitch.go
  { prefix: 'Failed to enable Kill Switch', key: 'err_killswitch_failed' },
  { prefix: 'Kill Switch configuration failed', key: 'err_killswitch_failed' },
  { prefix: 'Failed to disable Kill Switch', key: 'err_killswitch_failed' },
]

/**
 * Returns a localised rendering of a backend error message.
 *
 * Messages with no matching pattern are returned unchanged. That is deliberate:
 * some backend errors are already localised (the TUN-needs-Administrator text is
 * written in Persian), and a passthrough is better than replacing a specific
 * explanation with a generic "an error occurred".
 */
export function translateBackendError(message?: string | null): string {
  if (!message) return ''
  const trimmed = message.trim()
  if (trimmed === '') return ''

  for (const { prefix, key } of ERROR_PATTERNS) {
    if (trimmed.startsWith(prefix)) {
      const translated = t(key)
      // t() returns the key itself when no translation exists. Showing the user
      // "err_core_crashed" would be worse than showing the original English.
      return translated === key ? trimmed : translated
    }
  }
  return trimmed
}
