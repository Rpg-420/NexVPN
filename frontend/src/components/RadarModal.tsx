import { useState, useEffect } from 'react'
import { X, Radio, CheckCircle2, AlertTriangle, XCircle, RefreshCw, Globe, Sparkles } from 'lucide-react'
import { api } from '../lib/api'
import { sound } from '../lib/sound'
import { t } from '../lib/i18n'
import type { RadarReport } from '../types'
import { ModalShell } from '../components/ui/modal-shell'
import { Button } from '@/components/ui/button'

interface Props {
  isOpen: boolean
  onClose: () => void
}

export default function RadarModal({ isOpen, onClose }: Props) {
  const [report, setReport] = useState<RadarReport | null>(null)
  const [isLoading, setIsLoading] = useState(false)

  const scanRadar = async () => {
    setIsLoading(true)
    sound.playPing()
    try {
      const res = await api.runProtocolRadar()
      setReport(res)
      sound.playSuccess()
    } catch (err) {
      console.error('Run protocol radar error:', err)
    } finally {
      setIsLoading(false)
    }
  }

  useEffect(() => {
    if (isOpen) {
      scanRadar()
    }
  }, [isOpen])

  if (!isOpen) return null

  const getRecommendation = (rep: RadarReport) => {
    if (rep.censorship_level.toLowerCase().includes('severe')) {
      return t('radar_rec_severe')
    }
    if (rep.censorship_level.toLowerCase().includes('medium')) {
      return t('radar_rec_medium')
    }
    return t('radar_rec_low')
  }

  const getCensorshipLevel = (level: string) => {
    if (level.toLowerCase().includes('severe')) return t('radar_level_severe')
    if (level.toLowerCase().includes('medium')) return t('radar_level_medium')
    return t('radar_level_low')
  }

  return (
    <ModalShell
      isOpen={isOpen}
      onClose={onClose}
      maxWidth="max-w-xl"
      bodyClassName="space-y-4 p-6"
    >
        {/* Header */}
        <div className="flex items-center justify-between border-b border-border pb-3">
          <div className="flex items-center gap-2.5">
            <div className="w-9 h-9 rounded-xl border border-border bg-muted flex items-center justify-center text-muted-foreground shadow-inner">
              <Radio size={18} className={isLoading ? 'animate-spin' : ''} />
            </div>
            <div>
              <h3 className="font-bold text-foreground text-sm flex items-center gap-2">
                <span>{t('radar_modal_title')}</span>
                <span className="text-[10px] font-mono border-border bg-muted text-muted-foreground px-2 py-0.5 rounded-full font-bold">
                  ISP Radar 2.0
                </span>
              </h3>
              <p className="text-xs text-muted-foreground">{t('radar_modal_subtitle')}</p>
            </div>
          </div>
          <button onClick={onClose} className="text-muted-foreground hover:text-foreground p-1 rounded-lg hover:bg-muted transition">
            <X className="w-5 h-5" />
          </button>
        </div>

        {/* ISP Summary Card */}
        {report && (
          <div className="bg-card p-4 rounded-xl border border-border space-y-2">
            <div className="flex items-center justify-between flex-wrap gap-2 text-xs">
              <div className="flex items-center gap-2">
                <Globe size={14} className="text-primary" />
                <span className="text-muted-foreground font-semibold">{t('radar_isp_label')}</span>
                <span className="font-bold text-foreground">{report.isp_name}</span>
                <span className="text-muted-foreground font-mono text-[11px]">({report.ip_address})</span>
              </div>
              <div className="flex items-center gap-1.5">
                <span className="text-muted-foreground">{t('radar_level_label')}</span>
                <span
                  className={`font-bold px-2 py-0.5 rounded text-[11px] ${
                    report.censorship_level.toLowerCase().includes('low')
                      ? 'bg-success/10 text-success border border-success/20'
                      : report.censorship_level.toLowerCase().includes('severe')
                      ? 'bg-destructive/10 text-destructive border border-destructive/20 animate-pulse'
                      : 'bg-warning/10 text-warning border border-warning/20'
                  }`}
                >
                  {getCensorshipLevel(report.censorship_level)}
                </span>
              </div>
            </div>

            <div className="pt-2 border-t border-border text-xs text-muted-foreground flex items-start gap-2">
              <Sparkles size={14} className="text-warning shrink-0 mt-0.5" />
              <div>
                <span className="font-bold text-warning">{t('radar_recommendation_label')} </span>
                <span>{getRecommendation(report)}</span>
              </div>
            </div>
          </div>
        )}

        {/* Protocol Test Results Grid */}
        <div className="space-y-2 max-h-64 overflow-y-auto pe-1">
          {isLoading ? (
            <div className="text-center py-10 space-y-3">
              <div className="relative w-16 h-16 mx-auto">
                <div className="absolute inset-0 rounded-full border-2 border-primary/20 animate-ping" />
                <div className="w-16 h-16 rounded-full border-2 border-primary/40 flex items-center justify-center text-primary animate-pulse">
                  <Radio size={24} />
                </div>
              </div>
              <p className="text-xs text-muted-foreground font-semibold animate-pulse">{t('radar_scanning_label')}</p>
            </div>
          ) : report ? (
            report.protocols.map(p => {
              const isOk = p.status === 'excellent' || p.status === 'good'
              const isThrottled = p.status === 'throttled'

              return (
                <div
                  key={p.protocol}
                  className="bg-background p-3 rounded-xl border border-border flex items-center justify-between gap-3 text-xs"
                >
                  <div className="flex items-center gap-2.5 min-w-0">
                    {isOk && <CheckCircle2 size={16} className="text-success shrink-0" />}
                    {isThrottled && <AlertTriangle size={16} className="text-warning shrink-0" />}
                    {!isOk && !isThrottled && <XCircle size={16} className="text-destructive shrink-0" />}
                    <div className="min-w-0">
                      <div className="font-bold text-foreground truncate">{p.name}</div>
                      <div className="text-[11px] text-muted-foreground truncate">{p.description}</div>
                    </div>
                  </div>

                  <div className="flex items-center gap-2 shrink-0 font-mono">
                    <span
                      className={`px-2 py-0.5 rounded font-bold text-[11px] ${
                        isOk
                          ? 'bg-success/10 text-success'
                          : isThrottled
                          ? 'bg-warning/10 text-warning'
                          : 'bg-destructive/10 text-destructive'
                      }`}
                    >
                      {isOk ? t('radar_status_open') : isThrottled ? t('radar_status_throttled') : t('radar_status_blocked')}
                    </span>
                    <span className="text-muted-foreground font-bold min-w-[50px] text-end">
                      {p.latency_ms > 0 ? `${p.latency_ms} ms` : '---'}
                    </span>
                  </div>
                </div>
              )
            })
          ) : null}
        </div>

        {/* Footer */}
        <div className="flex items-center justify-between pt-2 border-t border-border">
          <Button variant="secondary" size="sm" className="text-xs flex items-center gap-1.5"
            onClick={scanRadar}
            disabled={isLoading}
          >
            <RefreshCw size={13} className={isLoading ? 'animate-spin' : ''} />
            <span>{t('radar_rescan_btn')}</span>
          </Button>

          <Button size="sm" className="text-xs" onClick={onClose}>
            {t('radar_got_it_btn')}
          </Button>
        </div>
      </ModalShell>
  )
}
