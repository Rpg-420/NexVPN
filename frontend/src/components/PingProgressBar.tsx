import { RefreshCw, Square } from 'lucide-react'

import { Button } from './ui/button'
import { Card } from './ui/card'
import { t } from '../lib/i18n'

export interface PingProgress {
  done: number
  total: number
}

interface PingProgressBarProps {
  progress: PingProgress | null
  /**
   * Shown only when the caller can actually abort the sweep. The sweep itself
   * is a single blocking Go call, so cancelling is cooperative: the button asks
   * the backend to stop, and the in-flight call returns whatever it has.
   */
  onCancel?: () => void
}

/**
 * Shared progress readout for the bulk reachability sweep.
 *
 * Every list page can start a sweep over its own category, and the backend
 * streams `profiles:ping_progress` while it works. Before this component each
 * page either rendered its own copy of the same card or showed nothing at all,
 * which made the sweep look frozen on a slow subscription.
 */
export function PingProgressBar({ progress, onCancel }: PingProgressBarProps) {
  if (!progress) return null

  const pct = Math.min(
    100,
    Math.round((progress.done / (progress.total || 1)) * 100)
  )

  return (
    <Card className="p-3 rounded-2xl border border-primary/30 bg-primary/5 space-y-1.5">
      <div className="flex items-center justify-between text-xs text-primary font-medium">
        <span className="flex items-center gap-2">
          <RefreshCw size={13} className="animate-spin" />
          <span>
            {t('ping_all_progress', {
              done: progress.done,
              total: progress.total,
            })}
          </span>
        </span>
        <span className="flex items-center gap-2">
          <span className="font-mono text-[11px] text-muted-foreground">{pct}%</span>
          {onCancel && (
            <Button
              variant="ghost"
              size="sm"
              className="h-6 px-2 text-[11px] text-muted-foreground hover:text-destructive"
              onClick={onCancel}
              title={t('adv_ping_cancel')}
            >
              <Square size={11} className="fill-current" />
              <span>{t('adv_ping_cancel')}</span>
            </Button>
          )}
        </span>
      </div>
      <div className="w-full h-1.5 bg-muted rounded-full overflow-hidden">
        <div
          className="bg-primary h-full transition-all duration-200"
          style={{ width: `${pct}%` }}
        />
      </div>
    </Card>
  )
}
