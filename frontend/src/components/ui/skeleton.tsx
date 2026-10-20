import * as React from 'react'

import { cn } from '@/lib/utils'

/**
 * Skeleton — placeholder block for a loading region.
 *
 * Replaces the ad-hoc `animate-pulse bg-…` divs so loading states read the same
 * everywhere. Pair it with `aria-busy` on the container being loaded.
 */
function Skeleton({ className, ...props }: React.HTMLAttributes<HTMLDivElement>) {
  return (
    <div
      aria-hidden="true"
      className={cn('animate-pulse rounded-md bg-muted', className)}
      {...props}
    />
  )
}

export { Skeleton }
