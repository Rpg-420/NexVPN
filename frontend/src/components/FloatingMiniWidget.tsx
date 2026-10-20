import { useState } from 'react'
import {
  Power,
  ArrowDown,
  ArrowUp,
  Minimize2,
  Maximize2,
  X,
  Shield,
  Radio,
} from 'lucide-react'
import { formatSpeed } from '../lib/utils'
import { t } from '../lib/i18n'
import CountryFlag from './CountryFlag'
import type { ConnectionStatus, TrafficStats, Profile } from '../types'

interface Props {
  status: ConnectionStatus
  stats?: TrafficStats
  traffic?: TrafficStats
  profiles?: Profile[]
  onSelectProfile?: (id: string) => void
  onToggleConnect?: () => void
  onConnect?: () => void
  onDisconnect?: () => void
  onOpenMainWindow?: () => void
  onCloseWidget?: () => void
  /** Hide the main window into the tray while keeping this widget visible. */
  onHideToTray?: () => void
}

export default function FloatingMiniWidget({
  status,
  stats,
  traffic,
  profiles = [],
  onSelectProfile,
  onToggleConnect,
  onConnect,
  onDisconnect,
  onOpenMainWindow,
  onCloseWidget,
  onHideToTray,
}: Props) {
  const isConnected = status.state === 'connected'
  const isConnecting = status.state === 'connecting'
  const [isMinimized, setIsMinimized] = useState(false)
  const [showProfileMenu, setShowProfileMenu] = useState(false)

  const activeTraffic = traffic || stats || {
    bytes_received: 0,
    bytes_sent: 0,
    speed_download: 0,
    speed_upload: 0,
    packets_received: 0,
    packets_sent: 0,
  }

  const handleAction = () => {
    if (onToggleConnect) {
      onToggleConnect()
    } else if (isConnected && onDisconnect) {
      onDisconnect()
    } else if (!isConnected && onConnect) {
      onConnect()
    }
  }

  return (
    <div className="fixed bottom-5 start-5 z-50 select-none animate-fade-in font-sans">
      <div
        className={`bg-card/95 backdrop-blur-2xl border border-border rounded-2xl shadow-2xl p-3.5 transition-all duration-300 ${
          isMinimized ? 'w-52' : 'w-80'
        }`}
      >
        {/* Top Header */}
        <div className="flex items-center justify-between pb-2.5 border-b border-border">
          <div className="flex items-center gap-2 min-w-0">
            <CountryFlag
              codeOrName={status.profile_name || 'un'}
              size="xs"
              className="shrink-0"
            />
            <span className="text-xs font-bold text-foreground truncate max-w-[130px]">
              {status.profile_name || 'NexVPN'}
            </span>
          </div>

          <div className="flex items-center gap-1 shrink-0">
            <span
              className={`w-2 h-2 rounded-full ${
                isConnected
                  ? 'bg-success animate-pulse shadow-sm'
                  : isConnecting
                  ? 'bg-warning animate-ping'
                  : 'bg-muted'
              }`}
            />
            <button
              onClick={() => setIsMinimized(!isMinimized)}
              className="p-1 rounded-lg text-muted-foreground hover:text-foreground hover:bg-card transition"
              title={isMinimized ? t('expand') : t('compact')}
            >
              {isMinimized ? <Maximize2 size={12} /> : <Minimize2 size={12} />}
            </button>
            {onCloseWidget && (
              <button
                onClick={onCloseWidget}
                className="p-1 rounded-lg text-muted-foreground hover:text-destructive hover:bg-card transition"
                title={t('close_widget')}
              >
                <X size={12} />
              </button>
            )}
          </div>
        </div>

        {/* Traffic & Controls Body */}
        {!isMinimized && (
          <div className="pt-2.5 space-y-2.5">
            {/* Live Speeds and Latency */}
            <div className="grid grid-cols-2 gap-2 bg-card/70 p-2.5 rounded-xl border border-border text-xs font-mono">
              <div className="flex items-center gap-1.5 text-primary">
                <ArrowDown size={13} className="text-primary" />
                <span className="truncate">{formatSpeed(activeTraffic.speed_download)}</span>
              </div>
              <div className="flex items-center gap-1.5 text-success">
                <ArrowUp size={13} className="text-success" />
                <span className="truncate">{formatSpeed(activeTraffic.speed_upload)}</span>
              </div>
            </div>

            {/* Quick Profile Dropdown */}
            {profiles.length > 0 && (
              <div className="relative">
                <button
                  type="button"
                  onClick={() => setShowProfileMenu(!showProfileMenu)}
                  className="w-full py-1 px-2.5 rounded-lg bg-card/80 hover:bg-card text-[11px] text-foreground flex items-center justify-between border border-border transition"
                >
                  <span className="truncate flex items-center gap-1.5">
                    <Radio size={11} className="text-primary" />
                    <span>{status.profile_name || t('select_server_placeholder')}</span>
                  </span>
                  <span className="text-[9px] text-muted-foreground">▼</span>
                </button>

                {showProfileMenu && (
                  <div className="absolute bottom-full start-0 end-0 mb-1 max-h-36 overflow-y-auto bg-card border border-border rounded-xl shadow-2xl p-1 z-50 space-y-1">
                    {profiles.map(p => (
                      <button
                        key={p.id}
                        type="button"
                        onClick={() => {
                          if (onSelectProfile) onSelectProfile(p.id)
                          setShowProfileMenu(false)
                        }}
                        className={`w-full text-start px-2 py-1 rounded-lg text-xs flex items-center justify-between transition ${
                          p.id === status.profile_id
                            ? 'bg-primary/20 text-primary font-bold'
                            : 'hover:bg-card text-foreground'
                        }`}
                      >
                        <span className="truncate flex items-center gap-1.5">
                          <CountryFlag codeOrName={p.name} size="xs" />
                          <span className="truncate">{p.name}</span>
                        </span>
                        {p.latency_ms && p.latency_ms > 0 ? (
                          <span className="font-mono text-[10px] text-success shrink-0">
                            {p.latency_ms}ms
                          </span>
                        ) : null}
                      </button>
                    ))}
                  </div>
                )}
              </div>
            )}

            {/* Actions Bar */}
            <div className="flex items-center gap-2">
              <button
                type="button"
                onClick={handleAction}
                disabled={isConnecting}
                className={`flex-1 py-1.5 px-3 rounded-xl font-bold text-xs flex items-center justify-center gap-1.5 shadow-lg transition-all active:scale-95 ${
                  isConnected
                    ? 'bg-destructive hover:bg-destructive/90 text-destructive-foreground'
                    : isConnecting
                    ? 'bg-primary text-primary-foreground'
                    : 'bg-primary hover:bg-primary/90 text-primary-foreground'
                }`}
              >
                <Power size={13} />
                <span>{isConnected ? t('disconnect') : isConnecting ? t('connecting') : t('connect')}</span>
              </button>

              {onOpenMainWindow && (
                <button
                  type="button"
                  onClick={onOpenMainWindow}
                  className="p-2 rounded-xl bg-card hover:bg-muted text-foreground transition"
                  title={t('open_main_window')}
                >
                  <Shield size={13} />
                </button>
              )}

              {onHideToTray && (
                <button
                  type="button"
                  onClick={onHideToTray}
                  className="p-2 rounded-xl bg-card hover:bg-muted text-foreground transition"
                  title={t('hide_to_tray')}
                >
                  <Minimize2 size={13} />
                </button>
              )}
            </div>
          </div>
        )}
      </div>
    </div>
  )
}
