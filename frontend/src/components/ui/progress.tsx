import * as React from 'react'

import { cn } from '@/lib/utils'

export interface ProgressProps extends React.HTMLAttributes<HTMLDivElement> {
  /** Completion percentage, clamped to 0–100. */
  value?: number
  /** Extra classes for the filled indicator (e.g. a status colour). */
  indicatorClassName?: string
}

/**
 * Progress — determinate bar.
 *
 * The indicator is sized with `width`, not `translateX`. A transform-based
 * indicator has to be mirrored by hand for RTL, and this app renders in RTL by
 * default; a width change is direction-agnostic because the layout handles the
 * start edge on its own.
 */
const Progress = React.forwardRef<HTMLDivElement, ProgressProps>(
  ({ className, value = 0, indicatorClassName, ...props }, ref) => {
    const pct = Math.min(100, Math.max(0, Number.isFinite(value) ? value : 0))
    return (
      <div
        ref={ref}
        role="progressbar"
        aria-valuemin={0}
        aria-valuemax={100}
        aria-valuenow={pct}
        className={cn('relative h-1.5 w-full overflow-hidden rounded-full bg-muted', className)}
        {...props}
      >
        <div
          className={cn('h-full rounded-full bg-primary transition-[width] duration-300 ease-out', indicatorClassName)}
          style={{ width: `${pct}%` }}
        />
      </div>
    )
  }
)
Progress.displayName = 'Progress'

export { Progress }
