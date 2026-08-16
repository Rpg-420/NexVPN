import React, { useState, useRef } from 'react'
import {
  Power,
  ArrowDown,
  ArrowUp,
  Minimize2,
  Maximize2,
  Move,
} from 'lucide-react'
import { t } from '../lib/i18n'
import type { ConnectionStatus, TrafficStats } from '../types'

interface FloatingMiniWidgetProps {
  status: ConnectionStatus
  stats: TrafficStats
  onToggleConnect: () => void
}

export const FloatingMiniWidget: React.FC<FloatingMiniWidgetProps> = ({
  status,
  stats,
  onToggleConnect,
}) => {
  const [collapsed, setCollapsed] = useState(false)
  const [position, setPosition] = useState<{ x: number; y: number } | null>(null)
  const isDraggingRef = useRef(false)
  const dragStartRef = useRef<{ startX: number; startY: number; posX: number; posY: number }>({
    startX: 0,
    startY: 0,
    posX: 0,
    posY: 0,
  })

  const isConnected = status.state === 'connected'
  const isConnecting = status.state === 'connecting'

  const formatSpeed = (bytesPerSec: number) => {
    if (!bytesPerSec || bytesPerSec <= 0) return '0 KB/s'
    const kb = bytesPerSec / 1024
    if (kb < 1024) return `${kb.toFixed(1)} KB/s`
    const mb = kb / 1024
    return `${mb.toFixed(2)} MB/s`
  }

  // Handle Dragging
  const handleMouseDown = (e: React.MouseEvent) => {
    if ((e.target as HTMLElement).closest('button')) return
    isDraggingRef.current = true
    const currentX = position ? position.x : window.innerWidth - 300
    const currentY = position ? position.y : window.innerHeight - 200

    dragStartRef.current = {
      startX: e.clientX,
      startY: e.clientY,
      posX: currentX,
      posY: currentY,
    }

    const handleMouseMove = (moveEvent: MouseEvent) => {
      if (!isDraggingRef.current) return
      const deltaX = moveEvent.clientX - dragStartRef.current.startX
      const deltaY = moveEvent.clientY - dragStartRef.current.startY

      const newX = Math.max(10, Math.min(window.innerWidth - 280, dragStartRef.current.posX + deltaX))
      const newY = Math.max(10, Math.min(window.innerHeight - 150, dragStartRef.current.posY + deltaY))

      setPosition({ x: newX, y: newY })
    }

    const handleMouseUp = () => {
      isDraggingRef.current = false
      window.removeEventListener('mousemove', handleMouseMove)
      window.removeEventListener('mouseup', handleMouseUp)
    }

    window.addEventListener('mousemove', handleMouseMove)
    window.addEventListener('mouseup', handleMouseUp)
  }

  const widgetStyle: React.CSSProperties = position
    ? {
        position: 'fixed',
        left: `${position.x}px`,
        top: `${position.y}px`,
        zIndex: 50,
        filter: 'drop-shadow(0 10px 25px rgba(0, 0, 0, 0.5))',
      }
    : {
        position: 'fixed',
        right: '24px',
        bottom: '24px',
        zIndex: 50,
        filter: 'drop-shadow(0 10px 25px rgba(0, 0, 0, 0.5))',
      }

  return (
    <div
      style={widgetStyle}
      onMouseDown={handleMouseDown}
      className="select-none cursor-grab active:cursor-grabbing transition-shadow"
    >
      <div
        className={`rounded-2xl border backdrop-blur-xl transition-all ${
          isConnected
            ? 'bg-slate-900/90 border-emerald-500/40 shadow-xl shadow-emerald-500/15'
            : isConnecting
            ? 'bg-slate-900/90 border-blue-500/40 shadow-xl shadow-blue-500/15'
            : 'bg-slate-900/90 border-white/10 shadow-xl'
        }`}
      >
        {collapsed ? (
          /* Collapsed Pill */
          <div className="flex items-center gap-3 px-3.5 py-2.5">
            <button
              onClick={onToggleConnect}
              className={`p-2 rounded-xl transition-all cursor-pointer ${
                isConnected
                  ? 'bg-emerald-500 text-slate-950 shadow-md shadow-emerald-500/30'
                  : isConnecting
                  ? 'bg-blue-500 text-white animate-pulse'
                  : 'bg-white/10 text-slate-400 hover:text-white'
              }`}
            >
              <Power className="w-4 h-4" />
            </button>

            <div className="flex items-center gap-2 text-xs font-bold text-white">
              {isConnected ? (
                <span className="flex items-center gap-1.5 text-emerald-400">
                  <span className="w-2 h-2 rounded-full bg-emerald-400 animate-ping" />
                  {formatSpeed(stats.speed_download)}
                </span>
              ) : (
                <span className="text-slate-400">{t('widget_disconnected')}</span>
              )}
            </div>

            <button
              onClick={() => setCollapsed(false)}
              className="p-1 rounded-lg text-slate-400 hover:text-white transition-all cursor-pointer"
            >
              <Maximize2 className="w-3.5 h-3.5" />
            </button>
          </div>
        ) : (
          <div className="p-3.5 min-w-[290px] space-y-3">
            {/* Top Bar with Drag Handle */}
            <div className="flex items-center justify-between gap-2 border-b border-white/5 pb-2">
              <div className="flex items-center gap-2">
                <Move className="w-3.5 h-3.5 text-slate-500" />
                <div
                  className={`w-2.5 h-2.5 rounded-full ${
                    isConnected
                      ? 'bg-emerald-400 shadow-sm shadow-emerald-400 animate-pulse'
                      : isConnecting
                      ? 'bg-blue-400 animate-ping'
                      : 'bg-slate-500'
                  }`}
                />
                <span className="text-xs font-bold text-slate-200 truncate max-w-[150px]">
                  {status.profile_name || t('widget_fastest_server')}
                </span>
              </div>

              <button
                onClick={() => setCollapsed(true)}
                className="p-1 rounded-lg text-slate-400 hover:text-white transition-all cursor-pointer"
              >
                <Minimize2 className="w-3.5 h-3.5" />
              </button>
            </div>

            {/* Speeds & Metrics */}
            <div className="grid grid-cols-2 gap-2 text-xs font-mono">
              <div className="p-2 rounded-xl bg-slate-950/60 border border-white/5 flex items-center gap-2">
                <ArrowDown className="w-3.5 h-3.5 text-cyan-400 shrink-0" />
                <div className="min-w-0">
                  <div className="text-[10px] text-slate-400 whitespace-nowrap">{t('widget_download')}</div>
                  <div className="font-bold text-slate-100 text-[11px] whitespace-nowrap truncate">
                    {formatSpeed(stats.speed_download)}
                  </div>
                </div>
              </div>

              <div className="p-2 rounded-xl bg-slate-950/60 border border-white/5 flex items-center gap-2">
                <ArrowUp className="w-3.5 h-3.5 text-purple-400 shrink-0" />
                <div className="min-w-0">
                  <div className="text-[10px] text-slate-400 whitespace-nowrap">{t('widget_upload')}</div>
                  <div className="font-bold text-slate-100 text-[11px] whitespace-nowrap truncate">
                    {formatSpeed(stats.speed_upload)}
                  </div>
                </div>
              </div>
            </div>

            {/* Quick Action Button */}
            <button
              onClick={onToggleConnect}
              className={`w-full py-2 px-3 rounded-xl text-xs font-bold flex items-center justify-center gap-2 transition-all cursor-pointer ${
                isConnected
                  ? 'bg-rose-500/20 text-rose-300 hover:bg-rose-500/30 border border-rose-500/30'
                  : isConnecting
                  ? 'bg-amber-500/20 text-amber-300 border border-amber-500/30 animate-pulse'
                  : 'bg-emerald-500/20 text-emerald-300 hover:bg-emerald-500/30 border border-emerald-500/30'
              }`}
            >
              <Power className="w-3.5 h-3.5" />
              <span>
                {isConnected ? t('widget_disconnect') : isConnecting ? t('connecting') : t('widget_connect')}
              </span>
            </button>
          </div>
        )}
      </div>
    </div>
  )
}
