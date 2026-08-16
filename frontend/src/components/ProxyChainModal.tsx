import React, { useState } from 'react'
import {
  Link2,
  Plus,
  Trash2,
  ArrowRight,
  Sparkles,
  X,
  CheckCircle2,
  AlertCircle,
  RefreshCw,
} from 'lucide-react'
import { api } from '../lib/api'
import { t } from '../lib/i18n'
import type { Profile } from '../types'
import { getCountryFlag } from '../lib/utils'

interface Props {
  isOpen: boolean
  onClose: () => void
  profiles: Profile[]
  onSuccess: () => void
}

export default function ProxyChainModal({ isOpen, onClose, profiles, onSuccess }: Props) {
  const [chainName, setChainName] = useState<string>('')
  const [selectedHops, setSelectedHops] = useState<string[]>([])
  const [isCreating, setIsCreating] = useState<boolean>(false)
  const [message, setMessage] = useState<{ text: string; type: 'success' | 'error' } | null>(null)

  if (!isOpen) return null

  const addHop = (profileId: string) => {
    if (selectedHops.length >= 4) {
      setMessage({ text: 'Maximum 4 hops supported in proxy chain', type: 'error' })
      return
    }
    setSelectedHops([...selectedHops, profileId])
    setMessage(null)
  }

  const removeHop = (index: number) => {
    const updated = [...selectedHops]
    updated.splice(index, 1)
    setSelectedHops(updated)
  }

  const handleCreate = async () => {
    if (selectedHops.length < 2) {
      setMessage({ text: t('chain_empty_prompt'), type: 'error' })
      return
    }
    setIsCreating(true)
    setMessage(null)
    try {
      await api.createProxyChain(selectedHops, chainName.trim())
      setMessage({ text: t('chain_success'), type: 'success' })
      onSuccess()
      setTimeout(() => {
        onClose()
      }, 1500)
    } catch (err: any) {
      setMessage({ text: err?.message || 'Failed to create proxy chain', type: 'error' })
    } finally {
      setIsCreating(false)
    }
  }

  return (
    <div className="fixed inset-0 z-50 flex items-center justify-center p-4 bg-black/70 backdrop-blur-md animate-fade-in">
      <div className="relative w-full max-w-2xl bg-neutral-900 border border-neutral-800 rounded-3xl shadow-2xl overflow-hidden flex flex-col max-h-[90vh]">
        {/* Header */}
        <div className="flex items-center justify-between p-6 border-b border-neutral-800 bg-neutral-950/40">
          <div className="flex items-center gap-3">
            <div className="w-10 h-10 rounded-2xl bg-gradient-to-tr from-amber-500/20 to-orange-500/20 border border-amber-500/30 flex items-center justify-center text-amber-400">
              <Link2 className="w-5 h-5" />
            </div>
            <div>
              <h3 className="text-lg font-bold text-white flex items-center gap-2">
                {t('chain_modal_title')}
                <span className="px-2 py-0.5 text-[10px] font-bold rounded-full bg-amber-500/20 text-amber-300 border border-amber-500/30">
                  {t('chain_badge')}
                </span>
              </h3>
              <p className="text-xs text-neutral-400">{t('chain_modal_sub')}</p>
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
        <div className="p-6 overflow-y-auto space-y-5 flex-1">
          {message && (
            <div
              className={`p-3.5 rounded-2xl text-xs flex items-center gap-2.5 border ${
                message.type === 'success'
                  ? 'bg-emerald-500/10 border-emerald-500/20 text-emerald-300'
                  : 'bg-red-500/10 border-red-500/20 text-red-300'
              }`}
            >
              {message.type === 'success' ? (
                <CheckCircle2 className="w-4 h-4 text-emerald-400 flex-shrink-0" />
              ) : (
                <AlertCircle className="w-4 h-4 text-red-400 flex-shrink-0" />
              )}
              <span>{message.text}</span>
            </div>
          )}

          {/* Chain Visual Path */}
          <div className="p-4 rounded-2xl bg-neutral-950/60 border border-neutral-800 space-y-3">
            <div className="text-xs font-semibold text-neutral-400 uppercase tracking-wider flex items-center justify-between">
              <span>{t('chain_pipeline_title')}</span>
              <span className="font-mono text-cyan-400 text-[11px]">{selectedHops.length} / 4</span>
            </div>

            {selectedHops.length === 0 ? (
              <div className="py-6 text-center text-xs text-neutral-500">
                {t('chain_select_prompt')}
              </div>
            ) : (
              <div className="flex items-center flex-wrap gap-2 pt-1">
                <div className="px-3 py-1.5 rounded-xl bg-blue-500/10 border border-blue-500/30 text-blue-300 text-xs font-bold font-mono">
                  💻 Local PC
                </div>
                <ArrowRight className="w-4 h-4 text-neutral-600 rtl:rotate-180" />

                {selectedHops.map((id, index) => {
                  const p = profiles.find(item => item.id === id)
                  return (
                    <React.Fragment key={index}>
                      <div className="flex items-center gap-1.5 px-3 py-1.5 rounded-xl bg-neutral-800 border border-neutral-700 text-xs text-white">
                        <span className="text-[10px] font-bold text-amber-400 font-mono">#{index + 1}</span>
                        <span>{p ? p.name : 'Unknown'}</span>
                        <button
                          type="button"
                          onClick={() => removeHop(index)}
                          className="text-neutral-400 hover:text-red-400 ml-1 transition"
                        >
                          <Trash2 className="w-3 h-3" />
                        </button>
                      </div>
                      <ArrowRight className="w-4 h-4 text-neutral-600 rtl:rotate-180" />
                    </React.Fragment>
                  )
                })}

                <div className="px-3 py-1.5 rounded-xl bg-emerald-500/10 border border-emerald-500/30 text-emerald-300 text-xs font-bold font-mono">
                  🌐 Target Web
                </div>
              </div>
            )}
          </div>

          {/* Chain Name Input */}
          <div className="space-y-1.5">
            <label className="text-xs font-semibold text-neutral-300">{t('chain_custom_name')}</label>
            <input
              type="text"
              value={chainName}
              onChange={e => setChainName(e.target.value)}
              placeholder={t('chain_custom_name_placeholder')}
              className="w-full p-3 rounded-2xl bg-neutral-950/80 border border-neutral-800 text-white font-mono text-xs focus:border-amber-500 focus:outline-none"
            />
          </div>

          {/* Node Selector Table */}
          <div className="space-y-2">
            <label className="text-xs font-semibold text-neutral-400 uppercase tracking-wider">
              {t('chain_available_nodes')}
            </label>
            <div className="divide-y divide-neutral-800/60 rounded-2xl bg-neutral-950/50 border border-neutral-800 max-h-48 overflow-y-auto">
              {profiles.map(p => (
                <div
                  key={p.id}
                  onClick={() => addHop(p.id)}
                  className="p-3 flex items-center justify-between text-xs hover:bg-neutral-800/40 cursor-pointer transition-all"
                >
                  <div className="flex items-center gap-2">
                    <span>{getCountryFlag(p.country_code)}</span>
                    <span className="font-bold text-white">{p.name}</span>
                    <span className="px-1.5 py-0.5 rounded bg-neutral-800 text-[10px] font-mono text-neutral-400 uppercase">
                      {p.protocol}
                    </span>
                  </div>
                  <button
                    type="button"
                    className="p-1 rounded-lg bg-neutral-800 hover:bg-amber-500/20 hover:text-amber-400 text-neutral-400 transition"
                  >
                    <Plus className="w-3.5 h-3.5" />
                  </button>
                </div>
              ))}
            </div>
          </div>
        </div>

        {/* Footer */}
        <div className="p-4 border-t border-neutral-800/80 bg-neutral-950/60 flex items-center justify-between">
          <div className="text-[11px] text-neutral-500 flex items-center gap-1.5">
            <Sparkles className="w-3.5 h-3.5 text-amber-400" />
            <span>{t('chain_kernel_active')}</span>
          </div>
          <button
            onClick={handleCreate}
            disabled={isCreating || selectedHops.length < 2}
            className="px-5 py-2 rounded-xl bg-amber-600 hover:bg-amber-500 text-white font-bold text-xs flex items-center gap-2 transition-all disabled:opacity-50 shadow-lg shadow-amber-500/20"
          >
            {isCreating ? <RefreshCw className="w-4 h-4 animate-spin" /> : <Link2 className="w-4 h-4" />}
            {t('chain_build_btn')}
          </button>
        </div>
      </div>
    </div>
  )
}
