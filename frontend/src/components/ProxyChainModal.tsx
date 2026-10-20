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
import CountryFlag from './CountryFlag'
import { ModalShell } from '../components/ui/modal-shell'

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
    <ModalShell
      isOpen={isOpen}
      onClose={onClose}
      maxWidth="max-w-2xl"
      className="max-h-[90vh]"
    >
        {/* Header */}
        <div className="flex items-center justify-between p-6 border-b border-border bg-background">
          <div className="flex items-center gap-3">
            <div className="bg-warning/20 w-10 h-10 rounded-2xl border border-warning/30 flex items-center justify-center text-warning">
              <Link2 className="w-5 h-5" />
            </div>
            <div>
              <h3 className="text-lg font-bold text-foreground flex items-center gap-2">
                {t('chain_modal_title')}
                <span className="px-2 py-0.5 text-[10px] font-bold rounded-full bg-warning/20 text-warning border border-warning/30">
                  {t('chain_badge')}
                </span>
              </h3>
              <p className="text-xs text-muted-foreground">{t('chain_modal_sub')}</p>
            </div>
          </div>
          <button
            onClick={onClose}
            className="p-2 rounded-xl bg-muted hover:bg-muted/90 text-muted-foreground hover:text-foreground transition-all"
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
                  ? 'bg-success/10 border-success/20 text-success'
                  : 'bg-destructive/10 border-destructive/20 text-destructive'
              }`}
            >
              {message.type === 'success' ? (
                <CheckCircle2 className="w-4 h-4 text-success flex-shrink-0" />
              ) : (
                <AlertCircle className="w-4 h-4 text-destructive flex-shrink-0" />
              )}
              <span>{message.text}</span>
            </div>
          )}

          {/* Chain Visual Path */}
          <div className="p-4 rounded-2xl bg-background border border-border space-y-3">
            <div className="text-xs font-semibold text-muted-foreground uppercase tracking-wider flex items-center justify-between">
              <span>{t('chain_pipeline_title')}</span>
              <span className="font-mono text-primary text-[11px]">{selectedHops.length} / 4</span>
            </div>

            {selectedHops.length === 0 ? (
              <div className="py-6 text-center text-xs text-muted-foreground">
                {t('chain_select_prompt')}
              </div>
            ) : (
              <div className="flex items-center flex-wrap gap-2 pt-1">
                <div className="px-3 py-1.5 rounded-xl border border-border bg-muted text-muted-foreground text-xs font-bold font-mono">
                  💻 Local PC
                </div>
                <ArrowRight className="w-4 h-4 text-muted-foreground rtl:rotate-180" />

                {selectedHops.map((id, index) => {
                  const p = profiles.find(item => item.id === id)
                  return (
                    <React.Fragment key={index}>
                      <div className="flex items-center gap-1.5 px-3 py-1.5 rounded-xl bg-muted border border-border text-xs text-foreground">
                        <span className="text-[10px] font-bold text-warning font-mono">#{index + 1}</span>
                        <span>{p ? p.name : 'Unknown'}</span>
                        <button
                          type="button"
                          onClick={() => removeHop(index)}
                          className="text-muted-foreground hover:text-destructive ms-1 transition"
                        >
                          <Trash2 className="w-3 h-3" />
                        </button>
                      </div>
                      <ArrowRight className="w-4 h-4 text-muted-foreground rtl:rotate-180" />
                    </React.Fragment>
                  )
                })}

                <div className="px-3 py-1.5 rounded-xl bg-success/10 border border-success/30 text-success text-xs font-bold font-mono">
                  🌐 Target Web
                </div>
              </div>
            )}
          </div>

          {/* Chain Name Input */}
          <div className="space-y-1.5">
            <label className="text-xs font-semibold text-muted-foreground">{t('chain_custom_name')}</label>
            <input
              type="text"
              value={chainName}
              onChange={e => setChainName(e.target.value)}
              placeholder={t('chain_custom_name_placeholder')}
              className="w-full p-3 rounded-2xl bg-card/80 border border-border text-foreground font-mono text-xs focus:border-warning focus:outline-none"
            />
          </div>

          {/* Node Selector Table */}
          <div className="space-y-2">
            <label className="text-xs font-semibold text-muted-foreground uppercase tracking-wider">
              {t('chain_available_nodes')}
            </label>
            <div className="divide-y divide-border/60 rounded-2xl bg-background border border-border max-h-48 overflow-y-auto">
              {profiles.map(p => (
                <div
                  key={p.id}
                  onClick={() => addHop(p.id)}
                  className="p-3 flex items-center justify-between text-xs hover:bg-muted cursor-pointer transition-all"
                >
                  <div className="flex items-center gap-2">
                    <CountryFlag codeOrName={p.country_code || p.name} size="xs" />
                    <span className="font-bold text-foreground">{p.name}</span>
                    <span className="px-1.5 py-0.5 rounded bg-muted text-[10px] font-mono text-muted-foreground uppercase">
                      {p.protocol}
                    </span>
                  </div>
                  <button
                    type="button"
                    className="p-1 rounded-lg bg-muted hover:bg-warning/20 hover:text-warning text-muted-foreground transition"
                  >
                    <Plus className="w-3.5 h-3.5" />
                  </button>
                </div>
              ))}
            </div>
          </div>
        </div>

        {/* Footer */}
        <div className="p-4 border-t border-border bg-background flex items-center justify-between">
          <div className="text-[11px] text-muted-foreground flex items-center gap-1.5">
            <Sparkles className="w-3.5 h-3.5 text-warning" />
            <span>{t('chain_kernel_active')}</span>
          </div>
          <button
            onClick={handleCreate}
            disabled={isCreating || selectedHops.length < 2}
            className="px-5 py-2 rounded-xl bg-primary hover:bg-primary/90 text-primary-foreground font-bold text-xs flex items-center gap-2 transition-all disabled:opacity-50 shadow-sm"
          >
            {isCreating ? <RefreshCw className="w-4 h-4 animate-spin" /> : <Link2 className="w-4 h-4" />}
            {t('chain_build_btn')}
          </button>
        </div>
      </ModalShell>
  )
}
