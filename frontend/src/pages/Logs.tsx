import { useState, useEffect, useRef, useCallback } from 'react'
import { Trash2, Download, Copy, Check, Terminal, Search, FolderOpen, FileText } from 'lucide-react'
import { api } from '../lib/api'
import { t } from '../lib/i18n'
import type { LogEntry, LogLevel } from '../types'
import { useTimers } from '../lib/useTimers'
import { Button } from '@/components/ui/button'
import { Badge } from '@/components/ui/badge'
import { Input } from '@/components/ui/input'
import { Card } from '@/components/ui/card'
import { Switch } from '@/components/ui/switch'
import { Tabs, TabsList, TabsTrigger } from '@/components/ui/tabs'
import { cn } from '@/lib/utils'

type FilterLevel = 'all' | LogLevel

export default function Logs() {
  const [logs, setLogs] = useState<LogEntry[]>([])
  const safeTimeout = useTimers()
  const [filterLevel, setFilterLevel] = useState<FilterLevel>('all')
  const [autoScroll, setAutoScroll] = useState(true)
  const [isCopied, setIsCopied] = useState(false)
  const [search, setSearch] = useState('')
  // On-disk data directory, so the user can find config.json / history.json.
  const [dataPath, setDataPath] = useState<string>('')
  // The launch diary. Shown next to the data folder because it is the file to
  // attach to a bug report when the app crashes or fails to start.
  const [diaryPath, setDiaryPath] = useState<string>('')
  const [pathCopied, setPathCopied] = useState(false)
  const logRef = useRef<HTMLDivElement>(null)

  const loadLogs = useCallback(async () => {
    try {
      const entries = await api.getLogs()
      if (entries && Array.isArray(entries)) setLogs(entries)
    } catch (err) {
      console.error('Failed to load logs:', err)
    }
  }, [])

  useEffect(() => {
    void api.getAppDataPath().then(p => { if (p) setDataPath(p) }).catch(() => { /* path display is optional */ })
    void api.getStartupDiaryPath().then(p => { if (p) setDiaryPath(p) }).catch(() => { /* path display is optional */ })
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
    safeTimeout(() => setIsCopied(false), 2000)
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

  /**
   * Reveals the on-disk data directory (config, history, logs) in the OS file
   * manager, and reports where it is. Previously the backend exposed both
   * OpenProfilesFolder and GetAppDataPath but no UI ever called them, so users
   * had no way to find their own data.
   */
  const handleRevealDataFolder = async () => {
    try {
      const path = await api.getAppDataPath()
      if (path) {
        setDataPath(path)
      }
      await api.openProfilesFolder()
    } catch (err) {
      console.error('Failed to open the data folder:', err)
    }
  }

  const handleCopyDataPath = () => {
    if (!dataPath) return
    navigator.clipboard.writeText(dataPath)
    setPathCopied(true)
    safeTimeout(() => setPathCopied(false), 2000)
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
            <Terminal size={22} className="text-primary" />
            {t('logs_title')}
          </h1>
          <p className="page-subtitle">{t('logs_subtitle', { count: logs.length })}</p>
        </div>

        <div className="flex items-center gap-2 flex-wrap">
          <Button
            id="open-data-folder-btn"
            variant="secondary"
            size="sm"
            onClick={handleRevealDataFolder}
            title={t('logs_open_data_folder_hint')}
          >
            <FolderOpen size={13} />
            {t('logs_open_data_folder')}
          </Button>

          <Button
            variant="secondary"
            size="sm"
            onClick={handleCopyAll}
            title={t('copy_logs')}
          >
            {isCopied ? <Check size={13} className="text-success" /> : <Copy size={13} />}
            {isCopied ? t('remote_pin_copied') : t('copy_logs')}
          </Button>

          <Button
            id="export-logs-btn"
            variant="secondary"
            size="sm"
            onClick={handleExport}
            title={t('export_logs')}
          >
            <Download size={13} />
            {t('export_logs')}
          </Button>

          <Button
            id="clear-logs-btn"
            variant="secondary"
            size="sm"
            className="text-destructive hover:bg-destructive/10 hover:text-destructive"
            onClick={handleClear}
            title={t('clear_logs')}
          >
            <Trash2 size={13} />
            {t('clear_logs')}
          </Button>
        </div>
      </div>

      {/* Data location, so config.json / history.json / logs can be found */}
      {dataPath && (
        <Card className="flex items-center justify-between gap-3 flex-wrap p-2.5">
          <div className="flex items-center gap-2 min-w-0">
            <FolderOpen size={14} className="text-primary shrink-0" />
            <span className="text-xs text-muted-foreground shrink-0">{t('logs_data_location')}</span>
            <code className="text-xs font-mono text-foreground truncate" dir="ltr">{dataPath}</code>
          </div>
          <Button variant="secondary" size="sm" className="shrink-0" onClick={handleCopyDataPath}>
            {pathCopied ? <Check size={12} className="text-success" /> : <Copy size={12} />}
            {pathCopied ? t('remote_pin_copied') : t('logs_copy_path')}
          </Button>
        </Card>
      )}

      {/*
        Launch diary. This is the file that survives a crash or a hang before the
        window is drawn — the in-memory log above does not — so it is the thing to
        attach when reporting a failure to start.
      */}
      {diaryPath && (
        <Card className="flex items-center justify-between gap-3 flex-wrap p-2.5">
          <div className="flex items-center gap-2 min-w-0">
            <FileText size={14} className="text-primary shrink-0" />
            <span className="text-xs text-muted-foreground shrink-0">{t('logs_startup_diary')}</span>
            <code className="text-xs font-mono text-foreground truncate" dir="ltr">{diaryPath}</code>
          </div>
          <span className="text-xs text-muted-foreground shrink-0">{t('logs_startup_diary_hint')}</span>
        </Card>
      )}

      {/* Toolbar with Level Pills and Search */}
      <Card className="flex items-center justify-between flex-wrap gap-3 p-3">
        {/* Level Filter Pills */}
        <Tabs value={filterLevel} onValueChange={v => setFilterLevel(v as FilterLevel)}>
          <TabsList className="h-9">
            {(['all', 'info', 'warning', 'error'] as const).map(lvl => {
              const count = lvl === 'all' ? logs.length : logCounts[lvl] || 0
              return (
                <TabsTrigger key={lvl} value={lvl} className="font-mono uppercase">
                  {lvl} ({count})
                </TabsTrigger>
              )
            })}
          </TabsList>
        </Tabs>

        {/* Search + Auto Scroll */}
        <div className="flex items-center gap-3 flex-wrap">
          <div className="relative w-[220px] max-w-full">
            <Search size={14} className="pointer-events-none absolute start-3 top-1/2 -translate-y-1/2 text-muted-foreground" />
            <Input
              type="text"
              placeholder={t('search_placeholder')}
              value={search}
              onChange={e => setSearch(e.target.value)}
              className="h-9 ps-9 text-xs"
            />
          </div>

          <label className="flex items-center gap-2 cursor-pointer text-xs text-muted-foreground">
            <Switch
              checked={autoScroll}
              onCheckedChange={setAutoScroll}
              aria-label={t('auto_scroll')}
            />
            <span>{t('auto_scroll')}</span>
          </label>
        </div>
      </Card>

      {/* Terminal Viewport */}
      <Card
        ref={logRef}
        className="p-4 font-mono text-xs overflow-y-auto max-h-[60vh] min-h-[350px] bg-black/70 shadow-inner space-y-1.5 select-text"
        style={{ direction: 'ltr', textAlign: 'left' }}
      >
        {filteredLogs.length === 0 ? (
          <div className="text-muted-foreground text-center py-12">{t('no_logs')}</div>
        ) : (
          filteredLogs.map((e, idx) => (
            <div key={idx} className="flex items-start gap-2.5 hover:bg-muted px-2 py-0.5 rounded transition">
              <span className="text-muted-foreground flex-shrink-0 text-[11px]">
                {e.timestamp ? new Date(e.timestamp).toLocaleTimeString() : '--:--:--'}
              </span>

              <Badge
                variant={e.level === 'error' ? 'destructive' : e.level === 'warning' ? 'warning' : 'info'}
                className="px-1.5 py-0 text-[10px] uppercase flex-shrink-0"
              >
                {e.level}
              </Badge>

              <span className="text-muted-foreground font-semibold flex-shrink-0">
                [{e.source}]
              </span>

              <span
                className={cn(
                  'break-all',
                  e.level === 'error'
                    ? 'text-destructive'
                    : e.level === 'warning'
                    ? 'text-warning'
                    : 'text-foreground'
                )}
              >
                {e.message}
              </span>
            </div>
          ))
        )}
      </Card>
    </div>
  )
}
