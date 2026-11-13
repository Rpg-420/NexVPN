import type { ProfileReliability } from '../../types'

const STYLE: Record<string, { cls: string; label: string }> = {
  excellent: { cls: 'text-success bg-success/10 border-success/30', label: '✓' },
  good: { cls: 'text-success bg-success/5 border-success/20', label: '✓' },
  fair: { cls: 'text-amber-500 bg-amber-500/10 border-amber-500/30', label: '!' },
  poor: { cls: 'text-destructive bg-destructive/10 border-destructive/30', label: '✕' },
}

/**
 * ReliabilityBadge — a small derived-health pill for the profile cards.
 *
 * Rendered only when the backend has an actual signal (failures or cooling):
 * a server the engine has never measured shows nothing rather than a
 * misleading "100%".
 */
export function ReliabilityBadge({ r }: { r: ProfileReliability | undefined }) {
  if (!r || r.state === 'unknown') return null
  const style = STYLE[r.state] ?? STYLE.fair
  const title = `${r.score}%${r.failures > 0 ? ` — ${r.failures} failure(s)` : ''}${
    r.last_failure_at ? ` — last: ${r.last_failure_at}` : ''
  }${r.last_failure ? ` (${r.last_failure})` : ''}${r.cooling ? ' — cooling down' : ''}`
  return (
    <span
      title={title}
      className={`shrink-0 text-[10px] px-1.5 py-0.5 rounded border font-mono flex items-center gap-0.5 ${style.cls}`}
    >
      {style.label}
      <span>{r.score}%</span>
      {r.cooling && <span>·⏳</span>}
    </span>
  )
}
