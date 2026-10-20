import { useState, useEffect } from 'react'
import {
  Radio,
  Sparkles,
  Zap,
  RefreshCw,
} from 'lucide-react'
import { api } from '../lib/api'
import { t, getLanguage } from '../lib/i18n'
import type { ISPRecommendation } from '../types'

export default function SmartISPOptimizerCard() {
  const [ispInfo, setIspInfo] = useState<ISPRecommendation | null>(null)
  const [loading, setLoading] = useState(false)
  const [applied, setApplied] = useState(false)
  const [applying, setApplying] = useState(false)
  const isFa = getLanguage() === 'fa'

  const loadISPInfo = async () => {
    setLoading(true)
    try {
      const res = await api.detectCurrentISP()
      if (res) {
        setIspInfo(res)
      }
    } catch (err) {
      console.error('Failed to detect ISP:', err)
    } finally {
      setLoading(false)
    }
  }

  useEffect(() => {
    loadISPInfo()
  }, [])

  const handleApply = async () => {
    if (!ispInfo) return
    setApplying(true)
    try {
      const size = ispInfo.fragment_size || '50-100'
      const sleep = ispInfo.fragment_sleep || '10-20'
      await api.applyOptimalFragment(size, sleep)
      setApplied(true)
      setTimeout(() => setApplied(false), 2500)
    } catch (err) {
      console.error('Failed to apply ISP settings:', err)
    } finally {
      setApplying(false)
    }
  }

  if (!ispInfo && !loading) return null

  const displayName = (isFa ? ispInfo?.isp_name : (ispInfo?.isp_name_en || ispInfo?.isp_name)) || t('isp_detecting')
  const displayAdvice = isFa ? ispInfo?.advice : (ispInfo?.advice_en || ispInfo?.advice)

  return (
    <div className="bg-primary/30 border border-primary/20 rounded-3xl p-4 shadow-xl backdrop-blur-md relative overflow-hidden">
      <div className="flex items-center justify-between gap-3 mb-3">
        <div className="flex items-center gap-2.5">
          <div className="w-8 h-8 rounded-xl bg-primary/20 border border-primary/30 flex items-center justify-center text-primary">
            <Radio className="w-4 h-4" />
          </div>
          <div>
            <h4 className="text-xs font-bold text-foreground flex items-center gap-2">
              <span>{displayName}</span>
              {ispInfo?.status_score && (
                <span className="px-2 py-0.5 text-[10px] font-bold rounded-full bg-success/20 text-success border border-success/30">
                  {ispInfo.status_score}% {t('isp_stability')}
                </span>
              )}
            </h4>
            <p className="text-[11px] text-muted-foreground font-mono" dir="ltr">
              {ispInfo?.asn ? `${ispInfo.asn} • ${ispInfo.ip}` : (t('isp_optimizer'))}
            </p>
          </div>
        </div>

        <div className="flex items-center gap-1.5">
          {ispInfo && (
            <button
              onClick={handleApply}
              disabled={applying}
              className={`px-2.5 py-1 rounded-xl text-xs font-semibold flex items-center gap-1.5 transition ${
                applied
                  ? 'bg-success/20 border border-success/40 text-success'
                  : 'bg-primary/30 hover:bg-primary/50 border border-primary/30 text-primary'
              }`}
              title={t('isp_apply_title')}
            >
              <Sparkles size={12} className={applied ? 'text-success' : 'text-primary'} />
              <span>{applied ? t('isp_applied') : t('isp_optimize')}</span>
            </button>
          )}

          <button
            onClick={loadISPInfo}
            disabled={loading}
            className="p-1.5 rounded-lg text-muted-foreground hover:text-foreground hover:bg-card transition"
            title={t('isp_refresh')}
          >
            <RefreshCw size={13} className={loading ? 'animate-spin text-primary' : ''} />
          </button>
        </div>
      </div>

      {ispInfo && (
        <div className="space-y-2 pt-1 border-t border-border text-xs">
          <div className="grid grid-cols-1 sm:grid-cols-2 gap-2">
            <div className="bg-card/60 p-2.5 rounded-xl border border-border">
              <span className="text-[10px] text-muted-foreground block mb-0.5">
                {t('isp_recommended_protocol')}
              </span>
              <span className="font-bold text-primary flex items-center gap-1.5" dir="ltr">
                <Zap size={12} className="text-primary" />
                {ispInfo.recommended_protocol}
              </span>
            </div>

            <div className="bg-card/60 p-2.5 rounded-xl border border-border">
              <span className="text-[10px] text-muted-foreground block mb-0.5">
                {t('isp_optimal_fragment')}
              </span>
              <span className="font-bold text-success flex items-center gap-1.5" dir="ltr">
                <Sparkles size={12} className="text-success" />
                {ispInfo.recommended_fragment}
              </span>
            </div>
          </div>

          <p className="text-[11px] text-foreground bg-primary/20 p-2 rounded-xl border border-primary/10 leading-relaxed">
            💡 {displayAdvice}
          </p>
        </div>
      )}
    </div>
  )
}
