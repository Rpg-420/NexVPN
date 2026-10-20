import * as React from 'react'

import {
  Dialog as UiDialog,
  DialogContent,
  DialogHeader,
  DialogFooter,
  DialogTitle,
  DialogDescription,
} from '@/components/ui/dialog'
import { cn } from '@/lib/utils'

/**
 * ModalShell — the single dialog primitive every feature modal should use.
 *
 * It wraps the shadcn/Radix Dialog so call sites keep their existing
 * `isOpen` / `onClose` / `title` / `children` / `footer` contract (no rewrite of
 * each of the ~28 modals), while gaining, for free:
 *   - focus trap + focus restoration
 *   - Escape-to-close and overlay-click-to-close
 *   - body scroll lock
 *   - correct `role="dialog"` / `aria-modal` / labelled-by wiring
 *   - portal rendering (no z-index/stacking-context bugs)
 *
 * Props mirror the previous hand-rolled pattern, so migration is a mechanical
 * swap of the outer shell only — the modal body stays untouched.
 */
export interface ModalShellProps {
  isOpen: boolean
  onClose: () => void
  /** Rendered as the dialog heading; omit for a custom header via `children`. */
  title?: React.ReactNode
  /** Optional sub-heading under the title. */
  description?: React.ReactNode
  /** Optional icon shown before the title. */
  icon?: React.ReactNode
  /** Modal body. */
  children?: React.ReactNode
  /** Action row rendered in the footer. */
  footer?: React.ReactNode
  /** Disable close-on-overlay-click (e.g. destructive/confirmation flows). */
  dismissible?: boolean
  /** Tailwind max-width class. Defaults to `max-w-lg`. */
  maxWidth?: string
  /** Extra classes for the content panel. */
  className?: string
  /** Extra classes for the body wrapper. */
  bodyClassName?: string
}

export function ModalShell({
  isOpen,
  onClose,
  title,
  description,
  icon,
  children,
  footer,
  dismissible = true,
  maxWidth = 'max-w-lg',
  className,
  bodyClassName,
}: ModalShellProps) {
  return (
    <UiDialog open={isOpen} onOpenChange={open => { if (!open) onClose() }}>
      <DialogContent
        className={cn(maxWidth, 'p-0 gap-0 overflow-hidden', className)}
        /* Every feature modal renders its own close control in the header, so
         * the corner one is suppressed to avoid two stacked `X` buttons. */
        showClose={false}
        onPointerDownOutside={e => { if (!dismissible) e.preventDefault() }}
        onEscapeKeyDown={e => { if (!dismissible) e.preventDefault() }}
        onInteractOutside={e => { if (!dismissible) e.preventDefault() }}
      >
        {(title || description) && (
          <DialogHeader className="border-b border-border px-6 py-4">
            <DialogTitle className="flex items-center gap-2">
              {icon}
              {title}
            </DialogTitle>
            {description && <DialogDescription>{description}</DialogDescription>}
          </DialogHeader>
        )}

        <div className={cn('px-6 py-4', bodyClassName)}>{children}</div>

        {footer && (
          <DialogFooter className="border-t border-border px-6 py-3">
            {footer}
          </DialogFooter>
        )}
      </DialogContent>
    </UiDialog>
  )
}
