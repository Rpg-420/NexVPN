import React, { useState, useEffect } from 'react'
import {
  Sparkles,
  Sliders,
  CheckCircle2,
  X,
} from 'lucide-react'
import { api } from '../lib/api'
import { sound } from '../lib/sound'
import { t } from '../lib/i18n'
import type { Settings } from '../types'

interface Props {
  isOpen: boolean
  onClose: () => void
  settings: Settings
  onSettingsUpdated: (updated: Settings) => void
}

interface Preset {
  id: string
  nameKey: string
  isp: string
  descKey: string
  size: string
  sleep: string
}

const PRESETS: Preset[] = [
  {
    id: 'mci_aggressive',
    nameKey: 'fragment_preset_mci_name',
    isp: 'همراه اول',
    descKey: 'fragment_preset_mci_desc',
    size: '10-30',
    sleep: '10-20',
  },
  {
    id: 'irancell_stealth',
    nameKey: 'fragment_preset_irancell_name',
    isp: 'ایرانسل',
    descKey: 'fragment_preset_irancell_desc',
    size: '50-100',
    sleep: '5-15',
  },
  {
    id: 'fixed_isp',
    nameKey: 'fragment_preset_fixed_name',
    isp: 'اینترنت ثابت',
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

  const applyPreset = (p: Preset) => {
    setSize(p.size)
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
    <div className="fixed inset-0 z-50 flex items-center justify-center p-4 bg-black/75 backdrop-blur-md animate-fade-in">
      <div className="relative w-full max-w-2xl bg-neutral-900 border border-neutral-800 rounded-3xl shadow-2xl overflow-hidden flex flex-col max-h-[90vh]">
        {/* Header */}
        <div className="flex items-center justify-between p-6 border-b border-neutral-800/80 bg-neutral-950/40">
          <div className="flex items-center gap-3">
            <div className="w-10 h-10 rounded-2xl bg-gradient-to-tr from-amber-500/20 to-rose-500/20 border border-amber-500/30 flex items-center justify-center text-amber-400">
              <Sparkles className="w-5 h-5" />
            </div>
            <div>
              <h3 className="text-lg font-bold text-white flex items-center gap-2">
                {t('fragment_modal_title')}
                <span className="px-2 py-0.5 text-xs font-semibold rounded-full bg-amber-500/20 text-amber-300 border border-amber-500/30">
                  {t('fragment_badge')}
                </span>
              </h3>
              <p className="text-xs text-neutral-400">{t('fragment_modal_sub')}</p>
            </div>
          </div>
          <button
            onClick={onClose}
            className="p-2 rounded-xl bg-neutral-800/60 hover:bg-neutral-700 text-neutral-400 hover:text-white transition-all"
          >
            <X className="w-4 h-4" />
          </button>
        </div>

        {/* Body */}
        <div className="p-6 overflow-y-auto space-y-6 flex-1">
          {/* Master Switch */}
          <div className="flex items-center justify-between p-4 rounded-2xl bg-neutral-950/50 border border-neutral-800">
            <div className="space-y-0.5">
              <div className="text-sm font-bold text-white flex items-center gap-2">
                {t('fragment_enable_title')}
              </div>
              <div className="text-xs text-neutral-400">
                {t('fragment_enable_sub')}
              </div>
            </div>
            <button
              onClick={() => setEnabled(!enabled)}
              className={`relative inline-flex h-6 w-11 items-center rounded-full transition-colors ${
                enabled ? 'bg-amber-500' : 'bg-neutral-700'
              }`}
            >
              <span
                className={`inline-block h-4 w-4 transform rounded-full bg-white transition-transform ${
                  enabled ? 'translate-x-6' : 'translate-x-1'
                }`}
              />
            </button>
          </div>

          {/* ISP Presets */}
          <div className="space-y-3">
            <div className="text-xs font-bold text-neutral-400 uppercase tracking-wider">
              {t('fragment_presets_title')}
            </div>
            <div className="grid grid-cols-1 md:grid-cols-3 gap-3">
              {PRESETS.map(p => (
                <div
                  key={p.id}
                  onClick={() => applyPreset(p)}
                  className={`p-4 rounded-2xl border transition-all cursor-pointer flex flex-col justify-between ${
                    selectedPreset === p.id
                      ? 'bg-amber-500/10 border-amber-500/50 shadow-lg shadow-amber-500/10'
                      : 'bg-neutral-800/40 border-neutral-800 hover:bg-neutral-800/80 hover:border-neutral-700'
                  }`}
                >
                  <div className="space-y-1">
                    <div className="flex items-center justify-between">
                      <span className="text-[10px] font-bold px-2 py-0.5 rounded-full bg-neutral-800 text-neutral-300 border border-neutral-700">
                        {p.isp}
                      </span>
                      {selectedPreset === p.id && <CheckCircle2 className="w-3.5 h-3.5 text-amber-400" />}
                    </div>
                    <div className="text-xs font-bold text-white pt-1">{t(p.nameKey)}</div>
                    <p className="text-[11px] text-neutral-400 leading-relaxed">{t(p.descKey)}</p>
                  </div>
                  <div className="mt-3 pt-2 border-t border-neutral-800/60 text-[10px] font-mono text-amber-300 flex justify-between">
                    <span>Size: {p.size}</span>
                    <span>Sleep: {p.sleep}ms</span>
                  </div>
                </div>
              ))}
            </div>
          </div>

          {/* Custom Sliders / Inputs */}
          <div className="p-4 rounded-2xl bg-neutral-950/40 border border-neutral-800 space-y-4">
            <div className="text-xs font-bold text-neutral-300 flex items-center gap-1.5">
              <Sliders className="w-4 h-4 text-amber-400" />
              {t('fragment_custom_title')}
            </div>

            <div className="grid grid-cols-2 gap-4">
              <div className="space-y-1.5">
                <label className="text-xs font-semibold text-neutral-400">{t('frag_size_input_label')}</label>
                <input
                  type="text"
                  value={size}
                  onChange={e => {
                    setSize(e.target.value)
                    setSelectedPreset(null)
                  }}
                  placeholder="e.g. 100-200 or 10-30"
                  className="w-full bg-neutral-900 border border-neutral-800 rounded-xl px-3 py-2 text-xs font-mono text-white focus:border-amber-500 focus:outline-none"
                />
                <span className="text-[10px] text-neutral-500">{t('frag_size_input_sub')}</span>
              </div>

              <div className="space-y-1.5">
                <label className="text-xs font-semibold text-neutral-400">{t('frag_sleep_input_label')}</label>
                <input
                  type="text"
                  value={sleep}
                  onChange={e => {
                    setSleep(e.target.value)
                    setSelectedPreset(null)
                  }}
                  placeholder="e.g. 10-20"
                  className="w-full bg-neutral-900 border border-neutral-800 rounded-xl px-3 py-2 text-xs font-mono text-white focus:border-amber-500 focus:outline-none"
                />
                <span className="text-[10px] text-neutral-500">{t('frag_sleep_input_sub')}</span>
              </div>
            </div>
          </div>

          {successMsg && (
            <div className="p-3 rounded-xl bg-emerald-500/10 border border-emerald-500/20 text-emerald-400 text-xs flex items-center gap-2">
              <CheckCircle2 className="w-4 h-4" />
              <span>{successMsg}</span>
            </div>
          )}
        </div>

        {/* Footer */}
        <div className="flex items-center justify-end gap-3 p-5 border-t border-neutral-800/80 bg-neutral-950/40">
          <button
            onClick={onClose}
            className="px-4 py-2 rounded-xl text-xs font-semibold text-neutral-400 hover:text-white transition"
          >
            {t('cancel')}
          </button>
          <button
            onClick={handleSave}
            disabled={isSaving}
            className="px-6 py-2.5 rounded-xl bg-gradient-to-r from-amber-500 to-orange-500 text-neutral-950 text-xs font-bold shadow-lg shadow-amber-500/20 hover:opacity-95 transition disabled:opacity-50"
          >
            {isSaving ? t('fragment_applying') : t('fragment_save_btn')}
          </button>
        </div>
      </div>
    </div>
  )
}
