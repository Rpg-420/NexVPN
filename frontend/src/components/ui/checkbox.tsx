import * as React from 'react'

import { cn } from '@/lib/utils'

/**
 * Checkbox — a thin wrapper over the native control.
 *
 * The visual treatment lives in `styles/main.css` (`input[type='checkbox']`),
 * which means the ~28 pre-existing raw checkboxes in the pages are styled by
 * the same rules as this component. Going native instead of pulling in
 * `@radix-ui/react-checkbox` keeps form participation, keyboard handling and
 * screen-reader semantics for free, and avoids a dependency for a control that
 * has no indeterminate-state requirement anywhere in this app.
 */
const Checkbox = React.forwardRef<HTMLInputElement, React.InputHTMLAttributes<HTMLInputElement>>(
  ({ className, type, ...props }, ref) => (
    <input ref={ref} type="checkbox" className={cn('', className)} {...props} />
  )
)
Checkbox.displayName = 'Checkbox'

export { Checkbox }
