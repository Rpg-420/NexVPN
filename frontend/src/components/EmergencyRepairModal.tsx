import React, { useState } from 'react'
import { X, Wrench, CheckCircle2, RefreshCw, Terminal, Sparkles, ShieldAlert } from 'lucide-react'
import { api } from '../lib/api'
import { sound } from '../lib/sound'
import { t } from '../lib/i18n'
import type { EmergencyRepairReport } from '../types'

interface Props {
  isOpen: boolean
  onClose: () => void
}

export default function EmergencyRepairModal({ isOpen, onClose }: Props) {
  const [isRepairing, setIsRepairing] = useState(false)
  const [report, setReport] = useState<EmergencyRepairReport | null>(null)

  const handleStartRepair = async () => {
    setIsRepairing(true)
    sound.playPing()
    try {
      const res = await api.runEmergencyNetworkRepair()
      setReport(res)
      sound.playSuccess()
    } catch (err) {
      console.error('Emergency repair error:', err)
    } finally {
      setIsRepairing(false)
    }
  }

  if (!isOpen) return null

  const getStepName = (step: { name: string; command: string }) => {
    if (step.command.includes('flushdns')) return t('emergency_step_flushdns')
    if (step.command.includes('winsock')) return t('emergency_step_winsock')
    if (step.command.includes('int ip')) return t('emergency_step_ipreset')
    if (step.command.includes('arp')) return t('emergency_step_arp')
    if (step.command.includes('ProxyEnable')) return t('emergency_step_proxy')
    return step.name
  }

  const getStepMessage = (step: { name: string; command: string; message: string }) => {
    if (step.command.includes('ProxyEnable')) return t('emergency_msg_proxy')
    return step.message
  }

  return (
    <div className="fixed inset-0 z-50 flex items-center justify-center bg-black/80 backdrop-blur-md p-4 animate-fade-in">
      <div className="bg-slate-900 border border-slate-700/80 rounded-2xl w-full max-w-xl overflow-hidden shadow-2xl flex flex-col space-y-4 p-6">
        {/* Header */}
        <div className="flex items-center justify-between border-b border-slate-800 pb-3">
          <div className="flex items-center gap-2.5">
            <div className="w-9 h-9 rounded-xl bg-amber-500/10 border border-amber-500/30 flex items-center justify-center text-amber-400 shadow-inner">
              <Wrench size={18} className={isRepairing ? 'animate-spin' : ''} />
            </div>
            <div>
              <h3 className="font-bold text-slate-100 text-sm flex items-center gap-2">
                <span>{t('emergency_modal_title')}</span>
                <span className="text-[10px] font-mono bg-amber-500/10 text-amber-400 border border-amber-500/20 px-2 py-0.5 rounded-full font-bold">
                  Emergency Fixer
                </span>
              </h3>
              <p className="text-xs text-slate-400">{t('emergency_modal_subtitle')}</p>
            </div>
          </div>
          <button onClick={onClose} className="text-slate-400 hover:text-slate-200 p-1 rounded-lg hover:bg-slate-800 transition">
            <X className="w-5 h-5" />
          </button>
        </div>

        {/* Info Alert Box */}
        <div className="bg-slate-950/60 p-3.5 rounded-xl border border-white/5 text-xs text-slate-300 space-y-1 leading-relaxed">
          <p className="font-bold text-amber-400 flex items-center gap-1.5">
            <ShieldAlert size={14} />
            {t('emergency_when_to_use')}
          </p>
          <p className="text-[11px] text-slate-400">
            {t('emergency_when_desc')}
          </p>
        </div>

        {/* Step Results List */}
        <div className="space-y-2 max-h-64 overflow-y-auto pr-1">
          {report ? (
            report.steps.map((step, idx) => (
              <div
                key={idx}
                className="bg-slate-950/70 p-3 rounded-xl border border-white/5 flex flex-col space-y-1 text-xs"
              >
                <div className="flex items-center justify-between">
                  <div className="flex items-center gap-2 font-bold text-slate-200">
                    <CheckCircle2 size={15} className="text-emerald-400 shrink-0" />
                    <span>{getStepName(step)}</span>
                  </div>
                  <span className="text-[10px] font-mono bg-emerald-500/10 text-emerald-400 border border-emerald-500/20 px-2 py-0.5 rounded">
                    {t('emergency_step_success')}
                  </span>
                </div>
                <div className="text-[11px] font-mono text-slate-500 flex items-center gap-1">
                  <Terminal size={11} />
                  <span>{step.command}</span>
                </div>
                {step.message && (
                  <div className="text-[11px] text-slate-400 bg-slate-900/80 p-1.5 rounded font-mono mt-1">
                    {getStepMessage(step)}
                  </div>
                )}
              </div>
            ))
          ) : (
            <div className="text-center py-8 space-y-2 text-xs text-slate-500">
              <Sparkles size={28} className="mx-auto text-slate-600 animate-pulse" />
              <p>{t('emergency_idle_hint')}</p>
            </div>
          )}
        </div>

        {/* Action Button */}
        <div className="flex items-center justify-between pt-2 border-t border-slate-800">
          <button
            className="btn btn-primary btn-sm text-xs flex items-center gap-2 px-4 py-2"
            onClick={handleStartRepair}
            disabled={isRepairing}
          >
            <RefreshCw size={14} className={isRepairing ? 'animate-spin' : ''} />
            <span>{isRepairing ? t('emergency_running_btn') : report ? t('emergency_re_repair_btn') : t('emergency_start_btn')}</span>
          </button>

          <button className="btn btn-secondary btn-sm text-xs" onClick={onClose}>
            {t('close')}
          </button>
        </div>
      </div>
    </div>
  )
}
