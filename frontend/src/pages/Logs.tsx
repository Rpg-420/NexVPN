import React, { useState, useEffect, useRef, useCallback } from 'react'
import { Trash2, Download, RefreshCw, Filter, Copy, Check, Terminal } from 'lucide-react'
import { api } from '../lib/api'
import { t } from '../lib/i18n'
import type { LogEntry, LogLevel } from '../types'
import { formatTime } from '../lib/utils'

type FilterLevel = 'all' | LogLevel

export default function Logs() {
  const [logs, setLogs] = useState<LogEntry[]>([])
  const [filterLevel, setFilterLevel] = useState<FilterLevel>('all')
  const [autoScroll, setAutoScroll] = useState(true)
  const [isRefreshing, setIsRefreshing] = useState(false)
  const [isCopied, setIsCopied] = useState(false)
  const [search, setSearch] = useState('')
  const logRef = useRef<HTMLDivElement>(null)

  const loadLogs = useCallback(async (showRefreshing = false) => {
    if (showRefreshing) setIsRefreshing(true)
    try {
      const entries = await api.getLogs()
      if (entries && Array.isArray(entries)) setLogs(entries)
    } finally {
      if (showRefreshing) {
        setTimeout(() => setIsRefreshing(false), 300)
      }
    }
  }, [])

  useEffect(() => {
    loadLogs()

    // Real-time log event listener
    const unsub = api.onEvent('log:entry', (entry: LogEntry) => {
      if (entry && entry.message) {
        setLogs(prev => {
          const next = [...prev, entry]
          if (next.length > 1500) return next.slice(-1000)
          return next
        })
      }
    })

    const interval = setInterval(() => loadLogs(), 3000)
    return () => {
      if (typeof unsub === 'function') unsub()
      clearInterval(interval)
    }
  }, [loadLogs])

  useEffect(() => {
    if (autoScroll && logRef.current) {
      logRef.current.scrollTop = logRef.current.scrollHeight
    }
  }, [logs, autoScroll])

  const handleClear = async () => {
    await api.clearLogs()
    setLogs([])
  }

  const handleCopyAll = () => {
    const text = logs
      .map(e => `[${e.timestamp}] [${e.level.toUpperCase()}] [${e.source}] ${e.message}`)
      .join('\n')
    navigator.clipboard.writeText(text)
    setIsCopied(true)
    setTimeout(() => setIsCopied(false), 2000)
  }

  const handleExport = () => {
    const text = logs
      .map(e => `[${e.timestamp}] [${e.level.toUpperCase()}] [${e.source}] ${e.message}`)
      .join('\n')
    const blob = new Blob([text], { type: 'text/plain' })
    const url = URL.createObjectURL(blob)
    const a = document.createElement('a')
    a.href = url
    a.download = `nexvpn-logs-${new Date().toISOString().slice(0, 10)}.txt`
    a.click()
    URL.revokeObjectURL(url)
  }

  const filteredLogs = logs.filter(e => {
    const matchLevel = filterLevel === 'all' || e.level === filterLevel
    const matchSearch =
      !search ||
      e.message.toLowerCase().includes(search.toLowerCase()) ||
      e.source.toLowerCase().includes(search.toLowerCase())
    return matchLevel && matchSearch
  })

  const logCounts = logs.reduce<Record<string, number>>((acc, e) => {
    acc[e.level] = (acc[e.level] || 0) + 1
    return acc
  }, {})

  return (
    <div className="page space-y-4">
      {/* Header */}
      <div className="page-header flex items-center justify-between flex-wrap gap-3">
        <div>
          <h1 className="page-title flex items-center gap-2">
            <Terminal size={22} className="text-accent" />
            {t('logs_title')}
          </h1>
          <p className="page-subtitle">{logs.length} total entries recorded</p>
        </div>

        <div className="flex items-center gap-2">
          <button
            className="btn btn-secondary btn-sm"
            onClick={handleCopyAll}
            title={t('copy_logs')}
          >
            {isCopied ? <Check size={13} className="text-emerald-400" /> : <Copy size={13} />}
            {isCopied ? 'Copied' : t('copy_logs')}
          </button>

          <button
            id="export-logs-btn"
            className="btn btn-secondary btn-sm"
            onClick={handleExport}
            title={t('export_logs')}
          >
            <Download size={13} />
            {t('export_logs')}
          </button>

          <button
            id="clear-logs-btn"
            className="btn btn-secondary btn-sm text-rose-400 hover:bg-rose-500/10"
            onClick={handleClear}
            title={t('clear_logs')}
          >
            <Trash2 size={13} />
            {t('clear_logs')}
          </button>
        </div>
      </div>

      {/* Toolbar with Level Pills and Search */}
      <div className="card p-3 flex items-center justify-between flex-wrap gap-3">
        {/* Level Filter Pills */}
        <div className="flex items-center gap-1.5 flex-wrap">
          {(['all', 'info', 'warning', 'error'] as const).map(lvl => {
            const isSelected = filterLevel === lvl
            const count = lvl === 'all' ? logs.length : logCounts[lvl] || 0
            return (
              <button
                key={lvl}
                onClick={() => setFilterLevel(lvl)}
                className={`px-3 py-1 rounded-lg text-xs font-bold font-mono transition uppercase ${
                  isSelected
                    ? 'bg-accent text-white shadow-md shadow-accent/25'
                    : 'bg-slate-800/60 text-slate-400 hover:text-slate-200'
                }`}
              >
                {lvl} ({count})
              </button>
            )
          })}
        </div>

        {/* Auto Scroll Toggle */}
        <div className="flex items-center gap-2 text-xs text-slate-300">
          <label className="flex items-center gap-2 cursor-pointer">
            <input
              type="checkbox"
              checked={autoScroll}
              onChange={e => setAutoScroll(e.target.checked)}
              className="rounded"
            />
            <span>Auto-scroll</span>
          </label>
        </div>
      </div>

      {/* Terminal Viewport */}
      <div
        ref={logRef}
        className="card p-4 font-mono text-xs overflow-y-auto max-h-[60vh] min-h-[350px] bg-black/70 border-slate-800 shadow-inner rounded-xl space-y-1.5 select-text"
        style={{ direction: 'ltr', textAlign: 'left' }}
      >
        {filteredLogs.length === 0 ? (
          <div className="text-slate-500 text-center py-12">No logs recorded yet.</div>
        ) : (
          filteredLogs.map((e, idx) => (
            <div key={idx} className="flex items-start gap-2.5 hover:bg-white/5 px-2 py-0.5 rounded transition">
              <span className="text-slate-500 flex-shrink-0 text-[11px]">
                {e.timestamp ? new Date(e.timestamp).toLocaleTimeString() : '--:--:--'}
              </span>

              <span
                className={`px-1.5 py-0.2 rounded text-[10px] font-bold uppercase flex-shrink-0 ${
                  e.level === 'error'
                    ? 'bg-rose-500/20 text-rose-400 border border-rose-500/30'
                    : e.level === 'warning'
                    ? 'bg-amber-500/20 text-amber-400 border border-amber-500/30'
                    : 'bg-cyan-500/20 text-cyan-400 border border-cyan-500/30'
                }`}
              >
                {e.level}
              </span>

              <span className="text-slate-400 font-semibold flex-shrink-0">
                [{e.source}]
              </span>

              <span
                className={`break-all ${
                  e.level === 'error'
                    ? 'text-rose-300'
                    : e.level === 'warning'
                    ? 'text-amber-300'
                    : 'text-slate-200'
                }`}
              >
                {e.message}
              </span>
            </div>
          ))
        )}
      </div>
    </div>
  )
}
