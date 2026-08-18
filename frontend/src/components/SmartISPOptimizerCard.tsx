import React, { useState, useEffect } from 'react'
import {
  Radio,
  Sparkles,
  Zap,
  RefreshCw,
} from 'lucide-react'
import { api } from '../lib/api'
import { t, getLanguage } from '../lib/i18n'

interface ISPRecommendation {
  isp_name: string
  isp_name_en?: string
  isp_category: string
  ip: string
  country: string
  asn: string
  recommended_protocol: string
  recommended_fragment: string
  recommended_dns: string
  status_score: number
  advice: string
  advice_en?: string
}

export default function SmartISPOptimizerCard() {
  const [ispInfo, setIspInfo] = useState<ISPRecommendation | null>(null)
  const [loading, setLoading] = useState(false)
  const isFa = getLanguage() === 'fa'

  const loadISPInfo = async () => {
    setLoading(true)
    try {
      const res = await (api as any).detectCurrentISP()
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

  if (!ispInfo && !loading) return null

  const displayName = isFa ? (ispInfo?.isp_name || 'در حال شناسایی اپراتور...') : (ispInfo?.isp_name_en || ispInfo?.isp_name || 'Detecting ISP...')
  const displayAdvice = isFa ? ispInfo?.advice : (ispInfo?.advice_en || ispInfo?.advice)

  return (
    <div className="bg-gradient-to-br from-indigo-950/30 via-slate-900/60 to-slate-900 border border-indigo-500/20 rounded-3xl p-4 shadow-xl backdrop-blur-md relative overflow-hidden">
      <div className="flex items-center justify-between gap-3 mb-3">
        <div className="flex items-center gap-2.5">
          <div className="w-8 h-8 rounded-xl bg-indigo-500/20 border border-indigo-500/30 flex items-center justify-center text-indigo-400">
            <Radio className="w-4 h-4" />
          </div>
          <div>
            <h4 className="text-xs font-bold text-slate-100 flex items-center gap-2">
              <span>{displayName}</span>
              {ispInfo?.status_score && (
                <span className="px-2 py-0.5 text-[10px] font-bold rounded-full bg-emerald-500/20 text-emerald-300 border border-emerald-500/30">
                  {ispInfo.status_score}% {isFa ? 'پایداری' : 'Stability'}
                </span>
              )}
            </h4>
            <p className="text-[11px] text-slate-400 font-mono" dir="ltr">
              {ispInfo?.asn ? `${ispInfo.asn} • ${ispInfo.ip}` : (isFa ? 'بهینه‌ساز هوشمند شبکه' : 'Smart Operator Optimizer')}
            </p>
          </div>
        </div>

        <button
          onClick={loadISPInfo}
          disabled={loading}
          className="p-1.5 rounded-lg text-slate-400 hover:text-white hover:bg-slate-800 transition"
          title={isFa ? 'بروزرسانی وضعیت شبکه' : 'Refresh Network Detection'}
        >
          <RefreshCw size={13} className={loading ? 'animate-spin text-indigo-400' : ''} />
        </button>
      </div>

      {ispInfo && (
        <div className="space-y-2 pt-1 border-t border-white/5 text-xs">
          <div className="grid grid-cols-1 sm:grid-cols-2 gap-2">
            <div className="bg-slate-950/60 p-2.5 rounded-xl border border-white/5">
              <span className="text-[10px] text-slate-500 block mb-0.5">
                {isFa ? 'پروتکل پیشنهادی:' : 'Recommended Protocol:'}
              </span>
              <span className="font-bold text-indigo-300 flex items-center gap-1.5" dir="ltr">
                <Zap size={12} className="text-indigo-400" />
                {ispInfo.recommended_protocol}
              </span>
            </div>

            <div className="bg-slate-950/60 p-2.5 rounded-xl border border-white/5">
              <span className="text-[10px] text-slate-500 block mb-0.5">
                {isFa ? 'فرگمنت و DNS بهینه:' : 'Optimal Fragment / DNS:'}
              </span>
              <span className="font-bold text-emerald-300 flex items-center gap-1.5" dir="ltr">
                <Sparkles size={12} className="text-emerald-400" />
                {ispInfo.recommended_fragment}
              </span>
            </div>
          </div>

          <p className="text-[11px] text-slate-300 bg-indigo-950/20 p-2 rounded-xl border border-indigo-500/10 leading-relaxed">
            💡 {displayAdvice}
          </p>
        </div>
      )}
    </div>
  )
}
