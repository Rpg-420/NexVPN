import { useState, useEffect } from 'react'
import { X, PieChart, ArrowDown, ArrowUp, Sparkles, Check, Info } from 'lucide-react'
import { api } from '../lib/api'
import { sound } from '../lib/sound'
import { formatBytes } from '../lib/utils'
import { t } from '../lib/i18n'
import type { TrafficStats, UsageReport } from '../types'
import { ModalShell } from '../components/ui/modal-shell'
import { Button } from '@/components/ui/button'

interface Props {
  isOpen: boolean
  onClose: () => void
  stats: TrafficStats
}

export default function QuotaManagerModal({ isOpen, onClose, stats }: Props) {
  const [quotaGB, setQuotaGB] = useState<number>(50)
  const [alertPercentage, setAlertPercentage] = useState<number>(80)
  const [isSaved, setIsSaved] = useState(false)
  // Real month-to-date usage from the connection history. This replaces the
  // previous behaviour of comparing the live session counters against a
  // monthly allowance, which always under-reported badly.
  const [usage, setUsage] = useState<UsageReport | null>(null)
  const [usageError, setUsageError] = useState<string | null>(null)

  const loadUsage = async () => {
    try {
      const report = await api.getUsageReport()
      setUsage(report)
      setUsageError(null)
    } catch (err) {
      console.error('Get usage report error:', err)
      setUsageError(err instanceof Error ? err.message : 'Could not read the usage history')
    }
  }

  useEffect(() => {
    if (isOpen) {
      api.getSettings().then(s => {
        if (s) {
          if (s.monthly_quota_gb) setQuotaGB(s.monthly_quota_gb)
          if (s.quota_alert_percentage) setAlertPercentage(s.quota_alert_percentage)
        }
      }).catch(err => console.error('Get settings error:', err))
      void loadUsage()
    }
  }, [isOpen])

  if (!isOpen) return null

  // Prefer the backend's aggregated figure. Fall back to the session counters
  // only if the report could not be read, and say so rather than pretending.
  const usingRealReport = usage !== null
  const totalUsedBytes = usingRealReport ? usage!.total_bytes : (stats.bytes_received || 0) + (stats.bytes_sent || 0)
  const quotaBytes = (quotaGB || 50) * 1024 * 1024 * 1024
  const usedPercentage = Math.min(100, Math.round((totalUsedBytes / (quotaBytes || 1)) * 100))
  const remainingBytes = Math.max(0, quotaBytes - totalUsedBytes)

  const handleSave = async () => {
    try {
      const s = await api.getSettings()
      await api.saveSettings({
        ...s,
        monthly_quota_gb: quotaGB,
        quota_alert_percentage: alertPercentage,
      })
      sound.playSuccess()
      setIsSaved(true)
      // Re-read so the bar reflects the new allowance immediately.
      await loadUsage()
      setTimeout(() => setIsSaved(false), 2000)
    } catch (err) {
      console.error('Save quota error:', err)
    }
  }

  return (
    <ModalShell
      isOpen={isOpen}
      onClose={onClose}
      maxWidth="max-w-lg"
      bodyClassName="space-y-4 p-6"
    >
        {/* Header */}
        <div className="flex items-center justify-between border-b border-border pb-3">
          <div className="flex items-center gap-2.5">
            <div className="w-9 h-9 rounded-xl bg-success/10 border border-success/30 flex items-center justify-center text-success shadow-inner">
              <PieChart size={18} />
            </div>
            <div>
              <h3 className="font-bold text-foreground text-sm">{t('quota_modal_title')}</h3>
              <p className="text-xs text-muted-foreground">{t('quota_modal_subtitle')}</p>
            </div>
          </div>
          <button onClick={onClose} className="text-muted-foreground hover:text-foreground p-1 rounded-lg hover:bg-muted transition">
            <X className="w-5 h-5" />
          </button>
        </div>

        {/* Visual Progress Bar Card */}
        <div className="bg-background p-4 rounded-xl border border-border space-y-3">
          <div className="flex items-center justify-between text-xs">
            <span className="text-muted-foreground">
              {usingRealReport ? t('quota_used_this_month') : t('quota_used_session')}
            </span>
            <span className="font-mono font-bold text-foreground">
              {formatBytes(totalUsedBytes)} {t('quota_of_gb', { gb: quotaGB })}
            </span>
          </div>

          {/* Progress Track */}
          <div className="w-full h-3 bg-muted rounded-full overflow-hidden p-0.5 border border-border">
            <div
              className={`h-full rounded-full transition-all duration-500 ${
                usedPercentage > 85
                  ? 'bg-destructive shadow-sm'
                  : usedPercentage > 60
                  ? 'bg-warning shadow-sm'
                  : 'bg-success shadow-sm'
              }`}
              style={{ width: `${Math.max(4, usedPercentage)}%` }}
            />
          </div>

          <div className="flex items-center justify-between text-[11px] text-muted-foreground">
            <span>{t('quota_used_pct')} <b className="text-foreground font-mono">{usedPercentage}%</b></span>
            <span>{t('quota_remaining')} <b className="text-success font-mono">{formatBytes(remainingBytes)}</b></span>
          </div>

          {/* Show what the number actually means, using the aggregated report. */}
          {usingRealReport && (
            <div className="flex items-start gap-2 pt-1 text-[10px] text-muted-foreground leading-relaxed">
              <Info size={11} className="shrink-0 mt-0.5" />
              <span>
                {t('quota_window_hint')
                  .replace('{sessions}', String(usage!.session_count))
                  .replace('{start}', usage!.window_start.slice(0, 10))}
                {!usage!.history_available && ` ${t('quota_no_history_hint')}`}
              </span>
            </div>
          )}
          {!usingRealReport && usageError && (
            <div className="flex items-start gap-2 pt-1 text-[10px] text-warning/90 leading-relaxed">
              <Info size={11} className="shrink-0 mt-0.5" />
              <span>{t('quota_report_unavailable')}</span>
            </div>
          )}
        </div>

        {/* Traffic Breakdown Stats */}
        <div className="grid grid-cols-2 gap-2 text-xs">
          <div className="bg-background p-2.5 rounded-lg border border-border flex items-center justify-between">
            <span className="flex items-center gap-1.5 text-success">
              <ArrowDown size={13} />
              <span>{t('quota_session_dl')}</span>
            </span>
            <span className="font-mono font-bold text-foreground">{formatBytes(stats.bytes_received)}</span>
          </div>

          <div className="bg-background p-2.5 rounded-lg border border-border flex items-center justify-between">
            <span className="flex items-center gap-1.5 text-primary">
              <ArrowUp size={13} />
              <span>{t('quota_session_ul')}</span>
            </span>
            <span className="font-mono font-bold text-foreground">{formatBytes(stats.bytes_sent)}</span>
          </div>
        </div>

        {/* Quota Settings Form */}
        <div className="grid grid-cols-2 gap-3 text-xs">
          <div className="space-y-1.5">
            <label className="text-muted-foreground font-semibold block">{t('quota_limit_label')}</label>
            <input
              type="number"
              className="form-control font-mono text-xs w-full"
              value={quotaGB}
              onChange={e => setQuotaGB(parseInt(e.target.value) || 10)}
              min={1}
              max={1000}
            />
          </div>

          <div className="space-y-1.5">
            <label className="text-muted-foreground font-semibold block">{t('quota_alert_label')}</label>
            <select
              className="form-control text-xs w-full"
              value={alertPercentage}
              onChange={e => setAlertPercentage(parseInt(e.target.value))}
            >
              <option value={70}>{t('quota_alert_70')}</option>
              <option value={80}>{t('quota_alert_80')}</option>
              <option value={90}>{t('quota_alert_90')}</option>
              <option value={100}>{t('quota_alert_100')}</option>
            </select>
          </div>
        </div>

        {/* Traffic Breakdown Stats */}
        <div className="grid grid-cols-2 gap-2 text-xs">
          <div className="bg-background p-2.5 rounded-lg border border-border flex items-center justify-between">
            <span className="flex items-center gap-1.5 text-success">
              <ArrowDown size={13} />
              <span>{t('quota_total_dl')}</span>
            </span>
            <span className="font-mono font-bold text-foreground">{formatBytes(stats.bytes_received)}</span>
          </div>

          <div className="bg-background p-2.5 rounded-lg border border-border flex items-center justify-between">
            <span className="flex items-center gap-1.5 text-primary">
              <ArrowUp size={13} />
              <span>{t('quota_total_ul')}</span>
            </span>
            <span className="font-mono font-bold text-foreground">{formatBytes(stats.bytes_sent)}</span>
          </div>
        </div>

        {/* Footer */}
        <div className="flex items-center justify-between pt-2 border-t border-border">
          <Button size="sm" className="text-xs flex items-center gap-1.5" onClick={handleSave}>
            {isSaved ? <Check size={13} className="text-success" /> : <Sparkles size={13} />}
            <span>{isSaved ? t('quota_saved') : t('quota_save_btn')}</span>
          </Button>

          <Button variant="secondary" size="sm" className="text-xs" onClick={onClose}>
            {t('close')}
          </Button>
        </div>
      </ModalShell>
  )
}
