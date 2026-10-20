export type Politeness = 'polite' | 'assertive'

type Listener = (message: string, politeness: Politeness) => void

let listener: Listener | null = null

/** Registered by <AriaLive /> so that announce() can reach the live regions. */
export function setAnnouncer(fn: Listener | null) {
  listener = fn
}

/**
 * Push a message to the app-wide ARIA live region so screen readers announce
 * status changes (connect/disconnect, toasts, background job results) that are
 * otherwise only conveyed visually.
 */
export function announce(message: string, politeness: Politeness = 'polite') {
  if (!message) return
  listener?.(message, politeness)
}
