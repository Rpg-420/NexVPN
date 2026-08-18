import React, { useState } from 'react'
import {
  Power,
  Zap,
  ArrowDown,
  ArrowUp,
  Activity,
  Minimize2,
  Maximize2,
  X,
  Shield,
} from 'lucide-react'
import { formatSpeed } from '../lib/utils'
import type { ConnectionStatus, TrafficStats } from '../types'

interface Props {
  status: ConnectionStatus
  stats?: TrafficStats
  traffic?: TrafficStats
  onToggleConnect?: () => void
  onConnect?: () => void
  onDisconnect?: () => void
  onOpenMainWindow?: () => void
  onCloseWidget?: () => void
}

export default function FloatingMiniWidget({
  status,
  stats,
  traffic,
  onToggleConnect,
  onConnect,
  onDisconnect,
  onOpenMainWindow,
  onCloseWidget,
}: Props) {
  const isConnected = status.state === 'connected'
  const isConnecting = status.state === 'connecting'
  const [isMinimized, setIsMinimized] = useState(false)

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
    <div className="fixed bottom-5 left-5 z-50 select-none animate-fade-in font-sans">
      <div
        className={`bg-slate-900/90 backdrop-blur-xl border border-cyan-500/30 rounded-2xl shadow-2xl shadow-cyan-950/60 p-3 transition-all duration-300 ${
          isMinimized ? 'w-48' : 'w-72'
        }`}
      >
        {/* Top Header */}
        <div className="flex items-center justify-between pb-2 border-b border-white/5">
          <div className="flex items-center gap-2">
            <span
              className={`w-2.5 h-2.5 rounded-full ${
                isConnected
                  ? 'bg-emerald-400 animate-pulse shadow-md shadow-emerald-400/50'
                  : isConnecting
                  ? 'bg-amber-400 animate-ping'
                  : 'bg-slate-600'
              }`}
            />
            <span className="text-xs font-bold text-slate-200 truncate max-w-[120px]">
              {status.profile_name || 'NexVPN'}
            </span>
          </div>

          <div className="flex items-center gap-1">
            <button
              onClick={() => setIsMinimized(!isMinimized)}
              className="p-1 rounded-lg text-slate-400 hover:text-white hover:bg-slate-800 transition"
              title={isMinimized ? 'Expand' : 'Compact'}
            >
              {isMinimized ? <Maximize2 size={12} /> : <Minimize2 size={12} />}
            </button>
            {onCloseWidget && (
              <button
                onClick={onCloseWidget}
                className="p-1 rounded-lg text-slate-400 hover:text-rose-400 hover:bg-slate-800 transition"
                title="Close Widget"
              >
                <X size={12} />
              </button>
            )}
          </div>
        </div>

        {/* Traffic & Controls Body */}
        {!isMinimized && (
          <div className="pt-2.5 space-y-2.5">
            {/* Live Speeds */}
            <div className="grid grid-cols-2 gap-2 bg-slate-950/60 p-2 rounded-xl border border-white/5 text-xs font-mono">
              <div className="flex items-center gap-1.5 text-cyan-300">
                <ArrowDown size={13} className="text-cyan-400" />
                <span className="truncate">{formatSpeed(activeTraffic.speed_download)}</span>
              </div>
              <div className="flex items-center gap-1.5 text-purple-300">
                <ArrowUp size={13} className="text-purple-400" />
                <span className="truncate">{formatSpeed(activeTraffic.speed_upload)}</span>
              </div>
            </div>

            {/* Actions Bar */}
            <div className="flex items-center gap-2">
              <button
                type="button"
                onClick={handleAction}
                disabled={isConnecting}
                className={`flex-1 py-1.5 px-3 rounded-xl font-bold text-xs flex items-center justify-center gap-1.5 shadow-md transition-all active:scale-95 ${
                  isConnected
                    ? 'bg-rose-500 hover:bg-rose-600 text-white shadow-rose-500/20'
                    : 'bg-cyan-500 hover:bg-cyan-400 text-slate-950 shadow-cyan-500/20'
                }`}
              >
                <Power size={13} />
                <span>{isConnected ? 'Disconnect' : isConnecting ? 'Connecting...' : 'Connect'}</span>
              </button>

              {onOpenMainWindow && (
                <button
                  type="button"
                  onClick={onOpenMainWindow}
                  className="p-2 rounded-xl bg-slate-800 hover:bg-slate-700 text-slate-300 hover:text-white transition"
                  title="Open App"
                >
                  <Shield size={13} />
                </button>
              )}
            </div>
          </div>
        )}
      </div>
    </div>
  )
}
