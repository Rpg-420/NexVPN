import { useState } from 'react'
import { X, Wrench, CheckCircle2, RefreshCw, Terminal, Sparkles, ShieldAlert } from 'lucide-react'
import { api } from '../lib/api'
import { sound } from '../lib/sound'
import { t } from '../lib/i18n'
import type { EmergencyRepairReport } from '../types'
import { ModalShell } from '../components/ui/modal-shell'
import { Button } from '@/components/ui/button'

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
    <ModalShell
      isOpen={isOpen}
      onClose={onClose}
      maxWidth="max-w-xl"
      bodyClassName="space-y-4 p-6"
    >
        {/* Header */}
        <div className="flex items-center justify-between border-b border-border pb-3">
          <div className="flex items-center gap-2.5">
            <div className="w-9 h-9 rounded-xl bg-warning/10 border border-warning/30 flex items-center justify-center text-warning shadow-inner">
              <Wrench size={18} className={isRepairing ? 'animate-spin' : ''} />
            </div>
            <div>
              <h3 className="font-bold text-foreground text-sm flex items-center gap-2">
                <span>{t('emergency_modal_title')}</span>
                <span className="text-[10px] font-mono bg-warning/10 text-warning border border-warning/20 px-2 py-0.5 rounded-full font-bold">
                  Emergency Fixer
                </span>
              </h3>
              <p className="text-xs text-muted-foreground">{t('emergency_modal_subtitle')}</p>
            </div>
          </div>
          <button onClick={onClose} className="text-muted-foreground hover:text-foreground p-1 rounded-lg hover:bg-muted transition">
            <X className="w-5 h-5" />
          </button>
        </div>

        {/* Info Alert Box */}
        <div className="bg-background p-3.5 rounded-xl border border-border text-xs text-muted-foreground space-y-1 leading-relaxed">
          <p className="font-bold text-warning flex items-center gap-1.5">
            <ShieldAlert size={14} />
            {t('emergency_when_to_use')}
          </p>
          <p className="text-[11px] text-muted-foreground">
            {t('emergency_when_desc')}
          </p>
        </div>

        {/* Step Results List */}
        <div className="space-y-2 max-h-64 overflow-y-auto pe-1">
          {report ? (
            report.steps.map((step, idx) => (
              <div
                key={idx}
                className="bg-background p-3 rounded-xl border border-border flex flex-col space-y-1 text-xs"
              >
                <div className="flex items-center justify-between">
                  <div className="flex items-center gap-2 font-bold text-foreground">
                    <CheckCircle2 size={15} className="text-success shrink-0" />
                    <span>{getStepName(step)}</span>
                  </div>
                  <span className="text-[10px] font-mono bg-success/10 text-success border border-success/20 px-2 py-0.5 rounded">
                    {t('emergency_step_success')}
                  </span>
                </div>
                <div className="text-[11px] font-mono text-muted-foreground flex items-center gap-1">
                  <Terminal size={11} />
                  <span>{step.command}</span>
                </div>
                {step.message && (
                  <div className="text-[11px] text-muted-foreground bg-card p-1.5 rounded font-mono mt-1">
                    {getStepMessage(step)}
                  </div>
                )}
              </div>
            ))
          ) : (
            <div className="text-center py-8 space-y-2 text-xs text-muted-foreground">
              <Sparkles size={28} className="mx-auto text-muted-foreground animate-pulse" />
              <p>{t('emergency_idle_hint')}</p>
            </div>
          )}
        </div>

        {/* Action Button */}
        <div className="flex items-center justify-between pt-2 border-t border-border">
          <Button size="sm" className="text-xs flex items-center gap-2 px-4 py-2"
            onClick={handleStartRepair}
            disabled={isRepairing}
          >
            <RefreshCw size={14} className={isRepairing ? 'animate-spin' : ''} />
            <span>{isRepairing ? t('emergency_running_btn') : report ? t('emergency_re_repair_btn') : t('emergency_start_btn')}</span>
          </Button>

          <Button variant="secondary" size="sm" className="text-xs" onClick={onClose}>
            {t('close')}
          </Button>
        </div>
      </ModalShell>
  )
}
