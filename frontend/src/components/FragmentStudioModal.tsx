import { useState, useEffect } from 'react'
import {
  Sparkles,
  Sliders,
  CheckCircle2,
} from 'lucide-react'
import { api } from '../lib/api'
import { sound } from '../lib/sound'
import { t } from '../lib/i18n'
import type { Settings } from '../types'
import { ModalShell } from '@/components/ui/modal-shell'
import { Switch } from '@/components/ui/switch'
import { Button } from '@/components/ui/button'

interface Props {
  isOpen: boolean
  onClose: () => void
  settings: Settings
  onSettingsUpdated: (updated: Settings) => void
}

interface Preset {
  id: string
  nameKey: string
  ispKey: string
  descKey: string
  size: string
  sleep: string
}

const PRESETS: Preset[] = [
  {
    id: 'mci_aggressive',
    nameKey: 'fragment_preset_mci_name',
    ispKey: 'isp_mci',
    descKey: 'fragment_preset_mci_desc',
    size: '10-30',
    sleep: '10-20',
  },
  {
    id: 'irancell_stealth',
    nameKey: 'fragment_preset_irancell_name',
    ispKey: 'isp_irancell',
    descKey: 'fragment_preset_irancell_desc',
    size: '50-100',
    sleep: '5-15',
  },
  {
    id: 'fixed_isp',
    nameKey: 'fragment_preset_fixed_name',
    ispKey: 'isp_fixed',
    descKey: 'fragment_preset_fixed_desc',
    size: '100-200',
    sleep: '5-10',
  },
]

export default function FragmentStudioModal({ isOpen, onClose, settings, onSettingsUpdated }: Props) {
  const [enabled, setEnabled] = useState(settings?.fragment_enabled || false)
  const [size, setSize] = useState(settings?.fragment_size || '100-200')
  const [sleep, setSleep] = useState(settings?.fragment_sleep || '10-20')
  const [selectedPreset, setSelectedPreset] = useState<string | null>(null)
  const [isSaving, setIsSaving] = useState(false)
  const [successMsg, setSuccessMsg] = useState<string | null>(null)

  useEffect(() => {
    if (isOpen) {
      api.getSettings().then(s => {
        if (s) {
          setEnabled(s.fragment_enabled || false)
          setSize(s.fragment_size || '100-200')
          setSleep(s.fragment_sleep || '10-20')
        }
      }).catch(err => console.error('Failed to load fragment settings:', err))
    }
  }, [isOpen])

  if (!isOpen) return null

  const applyPreset = (p: Preset) => {    setSize(p.size)
    setSleep(p.sleep)
    setEnabled(true)
    setSelectedPreset(p.id)
    sound.playSuccess()
  }

  const handleSave = async () => {
    setIsSaving(true)
    setSuccessMsg(null)
    try {
      const current = await api.getSettings()
      const updated: Settings = {
        ...current,
        fragment_enabled: enabled,
        fragment_size: size.trim(),
        fragment_sleep: sleep.trim(),
      }
      await api.saveSettings(updated)
      onSettingsUpdated(updated)
      setSuccessMsg(t('fragment_success'))
      sound.playSuccess()
      setTimeout(() => {
        setSuccessMsg(null)
        onClose()
      }, 1200)
    } catch (err: any) {
      console.error('Save error:', err)
    } finally {
      setIsSaving(false)
    }
  }

  return (
    <ModalShell
      isOpen={isOpen}
      onClose={onClose}
      maxWidth="max-w-2xl"
      icon={<Sparkles className="w-5 h-5 text-warning" />}
      title={
        <>
          {t('fragment_modal_title')}
          <span className="px-2 py-0.5 text-xs font-semibold rounded-full bg-warning/20 text-warning border border-warning/30">
            {t('fragment_badge')}
          </span>
        </>
      }
      description={t('fragment_modal_sub')}
      footer={
        <>
          <Button variant="ghost" size="sm" onClick={onClose}>
            {t('cancel')}
          </Button>
          <Button
            size="sm"
            onClick={handleSave}
            disabled={isSaving}
            className="bg-primary text-primary-foreground hover:opacity-95"
          >
            {isSaving ? t('fragment_applying') : t('fragment_save_btn')}
          </Button>
        </>
      }
    >
      <div className="space-y-6 max-h-[65vh] overflow-y-auto">
          {/* Master Switch */}
          <div className="flex items-center justify-between p-4 rounded-2xl bg-background/50 border border-border">
            <div className="space-y-0.5">
              <div className="text-sm font-bold text-foreground flex items-center gap-2">
                {t('fragment_enable_title')}
              </div>
              <div className="text-xs text-muted-foreground">
                {t('fragment_enable_sub')}
              </div>
            </div>
            <Switch
              checked={enabled}
              onCheckedChange={setEnabled}
              aria-label={t('fragment_enable_title')}
            />
          </div>

          {/* ISP Presets */}
          <div className="space-y-3">
            <div className="text-xs font-bold text-muted-foreground uppercase tracking-wider">
              {t('fragment_presets_title')}
            </div>
            <div className="grid grid-cols-1 md:grid-cols-3 gap-3">
              {PRESETS.map(p => (
                <div
                  key={p.id}
                  onClick={() => applyPreset(p)}
                  className={`p-4 rounded-2xl border transition-all cursor-pointer flex flex-col justify-between ${
                    selectedPreset === p.id
                      ? 'bg-warning/10 border-warning/50 shadow-sm'
                      : 'bg-muted/40 border-border hover:bg-muted/80 hover:border-border'
                  }`}
                >
                  <div className="space-y-1">
                    <div className="flex items-center justify-between">
                      <span className="text-[10px] font-bold px-2 py-0.5 rounded-full bg-muted text-foreground border border-border">
                        {t(p.ispKey)}
                      </span>
                      {selectedPreset === p.id && <CheckCircle2 className="w-3.5 h-3.5 text-warning" />}
                    </div>
                    <div className="text-xs font-bold text-foreground pt-1">{t(p.nameKey)}</div>
                    <p className="text-[11px] text-muted-foreground leading-relaxed">{t(p.descKey)}</p>
                  </div>
                  <div className="mt-3 pt-2 border-t border-border/60 text-[10px] font-mono text-warning flex justify-between">
                    <span>Size: {p.size}</span>
                    <span>Sleep: {p.sleep}ms</span>
                  </div>
                </div>
              ))}
            </div>
          </div>

          {/* Custom Sliders / Inputs */}
          <div className="p-4 rounded-2xl bg-background/40 border border-border space-y-4">
            <div className="text-xs font-bold text-foreground flex items-center gap-1.5">
              <Sliders className="w-4 h-4 text-warning" />
              {t('fragment_custom_title')}
            </div>

            <div className="grid grid-cols-2 gap-4">
              <div className="space-y-1.5">
                <label className="text-xs font-semibold text-muted-foreground">{t('frag_size_input_label')}</label>
                <input
                  type="text"
                  value={size}
                  onChange={e => {
                    setSize(e.target.value)
                    setSelectedPreset(null)
                  }}
                  placeholder="e.g. 100-200 or 10-30"
                  className="w-full bg-background border border-border rounded-xl px-3 py-2 text-xs font-mono text-foreground focus:border-warning focus:outline-none"
                />
                <span className="text-[10px] text-muted-foreground">{t('frag_size_input_sub')}</span>
              </div>

              <div className="space-y-1.5">
                <label className="text-xs font-semibold text-muted-foreground">{t('frag_sleep_input_label')}</label>
                <input
                  type="text"
                  value={sleep}
                  onChange={e => {
                    setSleep(e.target.value)
                    setSelectedPreset(null)
                  }}
                  placeholder="e.g. 10-20"
                  className="w-full bg-background border border-border rounded-xl px-3 py-2 text-xs font-mono text-foreground focus:border-warning focus:outline-none"
                />
                <span className="text-[10px] text-muted-foreground">{t('frag_sleep_input_sub')}</span>
              </div>
            </div>
          </div>

          {successMsg && (
            <div className="p-3 rounded-xl bg-success/10 border border-success/20 text-success text-xs flex items-center gap-2">
              <CheckCircle2 className="w-4 h-4" />
              <span>{successMsg}</span>
            </div>
          )}
      </div>
    </ModalShell>
  )
}
