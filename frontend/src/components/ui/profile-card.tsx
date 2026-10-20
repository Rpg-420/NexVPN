import * as React from 'react'
import { Edit3, Play, RefreshCw, Square, Star, Trash2 } from 'lucide-react'

import { Button } from '@/components/ui/button'
import { Card } from '@/components/ui/card'
import { cn } from '@/lib/utils'

/**
 * ProfileCard — the shared row chrome for every profile list
 * (OpenVPN / V2Ray / WarpWireGuard).
 *
 * All three pages previously hand-rolled the same structure with slightly
 * different markup, so connection state, the connect button and the
 * edit/delete affordances drifted apart. This component fixes the contract in
 * one place:
 *
 *   ┌──────────────────────────────────────────────┐
 *   │ [icon] Name                     [protocol pill]│
 *   │ meta line (server:port, extra badge)          │
 *   ├──────────────────────────────────────────────┤
 *   │ [footnote]            [edit][delete]  [connect]│
 *   └──────────────────────────────────────────────┘
 *
 * Page-specific content is passed through `metaSlot` / `footnoteSlot`, so the
 * cards stay consistent without losing their differences.
 *
 * Connected state is carried by border + a faint primary tint, not by a glow:
 * a list of twenty nodes must not turn into twenty light sources.
 */
export interface ProfileCardProps {
  name: string
  /** Emoji or small icon shown before the name. */
  icon?: React.ReactNode
  /** Small pill on the right of the title row (e.g. "OpenVPN", "VLESS"). */
  protocolLabel: React.ReactNode
  /**
   * Overrides the protocol pill colours. Pages that colour-code protocols
   * (V2Ray renders a different hue per protocol) pass their own classes here.
   */
  protocolClassName?: string
  /** Secondary line (server:port, transport, etc.). */
  metaSlot?: React.ReactNode
  /** Left side of the footer (username, quota, …). */
  footnoteSlot?: React.ReactNode
  /** Extra icon-buttons rendered before edit/delete (ping, QR, copy, …). */
  actionsSlot?: React.ReactNode
  /** Extra badges rendered under the header (fragments, SNI, …). */
  children?: React.ReactNode
  /** Connection state machine for the primary action. */
  state: 'connected' | 'connecting' | 'idle'
  /** Classes for the card container, used for the connected highlight. */
  className?: string
  /** Accent classes for the connect button when idle. */
  connectClassName?: string
  onConnect: () => void
  onDisconnect: () => void
  onEdit?: () => void
  onDelete?: () => void
  /** Localized labels. */
  labels: {
    connect: string
    disconnect: string
    editing?: string
    edit: string
    delete: string
  }
  /** Whether edit/delete are available for this row. */
  canEdit?: boolean
  canDelete?: boolean
  /** Whether the profile is pinned to the top of the list and the tray menu. */
  pinned?: boolean
  /** Toggles the pin. When omitted, no pin control is rendered. */
  onTogglePin?: () => void
  /** Localized labels for the pin control. */
  pinLabels?: { pin: string; unpin: string }
}

export function ProfileCard({
  name,
  icon,
  protocolLabel,
  protocolClassName,
  metaSlot,
  footnoteSlot,
  actionsSlot,
  children,
  state,
  className,
  connectClassName,
  onConnect,
  onDisconnect,
  onEdit,
  onDelete,
  labels,
  canEdit = true,
  canDelete = true,
  pinned = false,
  onTogglePin,
  pinLabels,
}: ProfileCardProps) {
  const isConnected = state === 'connected'
  const isConnecting = state === 'connecting'

  const stop = (e: React.MouseEvent, fn?: () => void) => {
    e.stopPropagation()
    fn?.()
  }

  return (
    <Card
      className={cn(
        'flex flex-col justify-between overflow-hidden p-4 transition-colors',
        isConnected ? 'border-primary bg-primary/5' : 'hover:border-border',
        className
      )}
    >
      <div>
        <div className="mb-2 flex min-w-0 items-center justify-between gap-2">
          <div className="flex min-w-0 flex-1 items-center gap-2">
            {icon && <span className="shrink-0 text-base leading-none">{icon}</span>}
            <h4 className="min-w-0 truncate text-sm font-medium text-foreground" title={name}>
              {name}
            </h4>
          </div>
          {protocolLabel && (
            <span
              className={cn(
                'shrink-0 rounded-md border px-1.5 py-0.5 font-mono text-[10px] font-medium uppercase',
                protocolClassName || 'border-border bg-muted text-muted-foreground'
              )}
            >
              {protocolLabel}
            </span>
          )}
        </div>

        {metaSlot && (
          <div className="mb-3 flex items-center justify-between gap-2 font-mono text-xs text-muted-foreground">
            {metaSlot}
          </div>
        )}

        {children}
      </div>

      <div className="mt-1 flex items-center justify-between gap-1 border-t border-border pt-3">
        <div className="max-w-[140px] truncate font-mono text-xs text-muted-foreground">
          {footnoteSlot}
        </div>

        <div className="flex shrink-0 items-center gap-1">
          {actionsSlot}

          {onTogglePin && (
            <Button
              variant="ghost"
              size="icon-sm"
              title={pinned ? pinLabels?.unpin : pinLabels?.pin}
              aria-label={pinned ? pinLabels?.unpin : pinLabels?.pin}
              aria-pressed={pinned}
              className={cn(
                // Unpinned stays muted so a list of twenty rows is not twenty
                // gold stars; pinned is the only state that draws the eye.
                pinned
                  ? 'text-amber-500 hover:text-amber-600'
                  : 'text-muted-foreground/40 hover:text-amber-500'
              )}
              onClick={e => stop(e, onTogglePin)}
            >
              <Star className={cn('h-3.5 w-3.5', pinned && 'fill-current')} />
            </Button>
          )}

          {canEdit && onEdit && (
            <Button
              variant="ghost"
              size="icon-sm"
              title={labels.edit}
              aria-label={labels.edit}
              onClick={e => stop(e, onEdit)}
            >
              <Edit3 className="h-3.5 w-3.5" />
            </Button>
          )}

          {canDelete && onDelete && (
            <Button
              variant="ghost"
              size="icon-sm"
              title={labels.delete}
              aria-label={labels.delete}
              className="text-muted-foreground hover:bg-destructive/10 hover:text-destructive"
              onClick={e => stop(e, onDelete)}
            >
              <Trash2 className="h-3.5 w-3.5" />
            </Button>
          )}

          <Button
            size="sm"
            variant={isConnected ? 'destructive' : 'default'}
            className={cn('min-w-[72px]', !isConnected && !isConnecting && connectClassName)}
            onClick={e => stop(e, isConnected ? onDisconnect : onConnect)}
            disabled={isConnecting}
          >
            {isConnecting ? (
              <>
                <RefreshCw className="h-3 w-3 animate-spin" />
                <span>…</span>
              </>
            ) : isConnected ? (
              <>
                <Square className="h-3 w-3 fill-current" />
                <span>{labels.disconnect}</span>
              </>
            ) : (
              <>
                <Play className="h-3 w-3 fill-current" />
                <span>{labels.connect}</span>
              </>
            )}
          </Button>
        </div>
      </div>
    </Card>
  )
}
