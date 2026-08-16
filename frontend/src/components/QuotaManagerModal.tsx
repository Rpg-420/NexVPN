import React, { useState, useEffect } from 'react'
import { X, PieChart, ArrowDown, ArrowUp, Sparkles, Check } from 'lucide-react'
import { api } from '../lib/api'
import { sound } from '../lib/sound'
import { formatBytes } from '../lib/utils'
import { t } from '../lib/i18n'
import type { TrafficStats } from '../types'

interface Props {
  isOpen: boolean
  onClose: () => void
  stats: TrafficStats
}

export default function QuotaManagerModal({ isOpen, onClose, stats }: Props) {
  const [quotaGB, setQuotaGB] = useState<number>(50)
  const [alertPercentage, setAlertPercentage] = useState<number>(80)
  const [isSaved, setIsSaved] = useState(false)

  useEffect(() => {
    if (isOpen) {
      api.getSettings().then(s => {
        if (s) {
          if (s.monthly_quota_gb) setQuotaGB(s.monthly_quota_gb)
          if (s.quota_alert_percentage) setAlertPercentage(s.quota_alert_percentage)
        }
      }).catch(err => console.error('Get settings error:', err))
    }
  }, [isOpen])

  if (!isOpen) return null

  const totalUsedBytes = (stats.bytes_received || 0) + (stats.bytes_sent || 0)
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
      setTimeout(() => setIsSaved(false), 2000)
    } catch (err) {
      console.error('Save quota error:', err)
    }
  }

  return (
    <div className="fixed inset-0 z-50 flex items-center justify-center bg-black/80 backdrop-blur-md p-4 animate-fade-in">
      <div className="bg-slate-900 border border-slate-700/80 rounded-2xl w-full max-w-lg overflow-hidden shadow-2xl flex flex-col space-y-4 p-6">
        {/* Header */}
        <div className="flex items-center justify-between border-b border-slate-800 pb-3">
          <div className="flex items-center gap-2.5">
            <div className="w-9 h-9 rounded-xl bg-emerald-500/10 border border-emerald-500/30 flex items-center justify-center text-emerald-400 shadow-inner">
              <PieChart size={18} />
            </div>
            <div>
              <h3 className="font-bold text-slate-100 text-sm">{t('quota_modal_title')}</h3>
              <p className="text-xs text-slate-400">{t('quota_modal_subtitle')}</p>
            </div>
          </div>
          <button onClick={onClose} className="text-slate-400 hover:text-slate-200 p-1 rounded-lg hover:bg-slate-800 transition">
            <X className="w-5 h-5" />
          </button>
        </div>

        {/* Visual Progress Bar Card */}
        <div className="bg-slate-950/70 p-4 rounded-xl border border-white/5 space-y-3">
          <div className="flex items-center justify-between text-xs">
            <span className="text-slate-400">{t('quota_total_used')}</span>
            <span className="font-mono font-bold text-slate-200">
              {formatBytes(totalUsedBytes)} {t('quota_of_gb', { gb: quotaGB })}
            </span>
          </div>

          {/* Progress Track */}
          <div className="w-full h-3 bg-slate-800 rounded-full overflow-hidden p-0.5 border border-white/5">
            <div
              className={`h-full rounded-full transition-all duration-500 ${
                usedPercentage > 85
                  ? 'bg-rose-500 shadow-lg shadow-rose-500/30'
                  : usedPercentage > 60
                  ? 'bg-amber-500 shadow-lg shadow-amber-500/30'
                  : 'bg-emerald-500 shadow-lg shadow-emerald-500/30'
              }`}
              style={{ width: `${Math.max(4, usedPercentage)}%` }}
            />
          </div>

          <div className="flex items-center justify-between text-[11px] text-slate-400">
            <span>{t('quota_used_pct')} <b className="text-slate-200 font-mono">{usedPercentage}%</b></span>
            <span>{t('quota_remaining')} <b className="text-emerald-400 font-mono">{formatBytes(remainingBytes)}</b></span>
          </div>
        </div>

        {/* Quota Settings Form */}
        <div className="grid grid-cols-2 gap-3 text-xs">
          <div className="space-y-1.5">
            <label className="text-slate-300 font-semibold block">{t('quota_limit_label')}</label>
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
            <label className="text-slate-300 font-semibold block">{t('quota_alert_label')}</label>
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
          <div className="bg-slate-950/40 p-2.5 rounded-lg border border-white/5 flex items-center justify-between">
            <span className="flex items-center gap-1.5 text-emerald-400">
              <ArrowDown size={13} />
              <span>{t('quota_total_dl')}</span>
            </span>
            <span className="font-mono font-bold text-slate-200">{formatBytes(stats.bytes_received)}</span>
          </div>

          <div className="bg-slate-950/40 p-2.5 rounded-lg border border-white/5 flex items-center justify-between">
            <span className="flex items-center gap-1.5 text-blue-400">
              <ArrowUp size={13} />
              <span>{t('quota_total_ul')}</span>
            </span>
            <span className="font-mono font-bold text-slate-200">{formatBytes(stats.bytes_sent)}</span>
          </div>
        </div>

        {/* Footer */}
        <div className="flex items-center justify-between pt-2 border-t border-slate-800">
          <button className="btn btn-primary btn-sm text-xs flex items-center gap-1.5" onClick={handleSave}>
            {isSaved ? <Check size={13} className="text-emerald-400" /> : <Sparkles size={13} />}
            <span>{isSaved ? t('quota_saved') : t('quota_save_btn')}</span>
          </button>

          <button className="btn btn-secondary btn-sm text-xs" onClick={onClose}>
            {t('close')}
          </button>
        </div>
      </div>
    </div>
  )
}
