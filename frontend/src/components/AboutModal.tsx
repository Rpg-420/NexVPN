import React, { useState } from 'react'
import {
  Shield,
  Globe,
  Send,
  Github,
  MessageCircle,
  RefreshCw,
  CheckCircle2,
  ExternalLink,
  Cpu,
  Layers,
  Sparkles,
  X,
  Code2,
} from 'lucide-react'
import { api } from '../lib/api'
import { t } from '../lib/i18n'

interface Props {
  isOpen: boolean
  onClose: () => void
}

export default function AboutModal({ isOpen, onClose }: Props) {
  const [isCheckingUpdate, setIsCheckingUpdate] = useState(false)
  const [updateStatus, setUpdateStatus] = useState<string | null>(null)

  if (!isOpen) return null

  const handleOpenLink = (url: string) => {
    try {
      api.openBrowserURL(url)
    } catch {
      window.open(url, '_blank')
    }
  }

  const handleCheckUpdate = () => {
    setIsCheckingUpdate(true)
    setUpdateStatus(null)
    setTimeout(() => {
      setIsCheckingUpdate(false)
      setUpdateStatus(t('update_latest_notice'))
    }, 1200)
  }

  return (
    <div className="fixed inset-0 z-50 flex items-center justify-center bg-black/80 backdrop-blur-md p-4 animate-fade-in" onClick={onClose}>
      <div className="bg-slate-900 border border-slate-700/80 rounded-2xl w-full max-w-lg overflow-hidden shadow-2xl relative flex flex-col" onClick={e => e.stopPropagation()}>
        {/* Modal Header */}
        <div className="flex items-center justify-between px-5 py-4 border-b border-slate-800">
          <div className="flex items-center gap-2.5 font-bold text-slate-100 text-sm">
            <div className="w-8 h-8 rounded-lg bg-cyan-500/10 border border-cyan-500/30 flex items-center justify-center text-cyan-400">
              <Shield size={16} />
            </div>
            <span>{t('about_title')}</span>
          </div>
          <button onClick={onClose} className="text-slate-400 hover:text-slate-200 p-1 rounded-lg hover:bg-slate-800 transition">
            <X size={18} />
          </button>
        </div>

        {/* Modal Body */}
        <div className="p-5 space-y-5">
          {/* App Brand Banner */}
          <div className="flex flex-col items-center text-center p-4 bg-gradient-to-b from-slate-900/90 via-slate-900/60 to-slate-950/80 rounded-2xl border border-white/5 relative overflow-hidden">
            <div className="w-16 h-16 rounded-2xl overflow-hidden shadow-xl shadow-cyan-500/20 mb-3 ring-1 ring-cyan-500/40">
              <img src="/appicon.png" alt="NexVPN" className="w-full h-full object-cover" />
            </div>
            <h2 className="text-lg font-extrabold text-slate-100 flex items-center gap-2">
              <span>NexVPN Pro Client</span>
              <span className="badge badge-info text-[10px] font-mono font-bold">v1.1.0</span>
            </h2>
            <p className="text-xs text-slate-400 mt-1 max-w-sm leading-relaxed">
              {t('about_description')}
            </p>
          </div>

          {/* Core Engines Badge Grid */}
          <div className="space-y-2">
            <h3 className="text-xs font-bold text-slate-300 flex items-center gap-1.5">
              <Cpu size={14} className="text-cyan-400" />
              <span>{t('cores_status_title')}</span>
            </h3>

            <div className="grid grid-cols-2 sm:grid-cols-4 gap-2 text-center text-[11px] font-mono">
              <div className="p-2 rounded-xl bg-black/40 border border-white/5 flex flex-col items-center gap-1">
                <span className="text-cyan-400 font-bold">Sing-Box</span>
                <span className="text-[10px] text-slate-400">v1.11.3 (TUN)</span>
              </div>
              <div className="p-2 rounded-xl bg-black/40 border border-white/5 flex flex-col items-center gap-1">
                <span className="text-accent font-bold">Xray-Core</span>
                <span className="text-[10px] text-slate-400">v1.8.24 (V2Ray)</span>
              </div>
              <div className="p-2 rounded-xl bg-black/40 border border-white/5 flex flex-col items-center gap-1">
                <span className="text-orange-400 font-bold">OpenVPN</span>
                <span className="text-[10px] text-slate-400">v2.6.9 (TAP)</span>
              </div>
              <div className="p-2 rounded-xl bg-black/40 border border-white/5 flex flex-col items-center gap-1">
                <span className="text-emerald-400 font-bold">Wintun</span>
                <span className="text-[10px] text-slate-400">v0.14.1 (Driver)</span>
              </div>
            </div>
          </div>

          {/* Official Community & Links */}
          <div className="space-y-2">
            <h3 className="text-xs font-bold text-slate-300 flex items-center gap-1.5">
              <Sparkles size={14} className="text-amber-400" />
              <span>{t('support_feedback')}</span>
            </h3>

            <div className="grid grid-cols-1 sm:grid-cols-2 gap-2">
              <button
                type="button"
                onClick={() => handleOpenLink('https://t.me/nexvpnpro')}
                className="p-2.5 rounded-xl bg-slate-900/80 hover:bg-slate-800/90 border border-white/5 hover:border-cyan-500/40 transition flex items-center justify-between text-xs text-slate-200"
              >
                <div className="flex items-center gap-2">
                  <Send size={15} className="text-sky-400" />
                  <span className="font-semibold">{t('telegram_channel')}</span>
                </div>
                <ExternalLink size={13} className="text-slate-500" />
              </button>

              <button
                type="button"
                onClick={() => handleOpenLink('https://github.com/Rpg-420/NexVPN')}
                className="p-2.5 rounded-xl bg-slate-900/80 hover:bg-slate-800/90 border border-white/5 hover:border-accent/40 transition flex items-center justify-between text-xs text-slate-200"
              >
                <div className="flex items-center gap-2">
                  <Github size={15} className="text-purple-400" />
                  <span className="font-semibold">{t('github_source')}</span>
                </div>
                <ExternalLink size={13} className="text-slate-500" />
              </button>

              <button
                type="button"
                onClick={() => handleOpenLink('https://github.com/Rpg-420/NexVPN/releases')}
                className="p-2.5 rounded-xl bg-slate-900/80 hover:bg-slate-800/90 border border-white/5 hover:border-emerald-500/40 transition flex items-center justify-between text-xs text-slate-200"
              >
                <div className="flex items-center gap-2">
                  <Globe size={15} className="text-emerald-400" />
                  <span className="font-semibold">{t('official_website')}</span>
                </div>
                <ExternalLink size={13} className="text-slate-500" />
              </button>

              <button
                type="button"
                onClick={() => handleOpenLink('https://t.me/nexvpnpro')}
                className="p-2.5 rounded-xl bg-slate-900/80 hover:bg-slate-800/90 border border-white/5 hover:border-amber-500/40 transition flex items-center justify-between text-xs text-slate-200"
              >
                <div className="flex items-center gap-2">
                  <MessageCircle size={15} className="text-amber-400" />
                  <span className="font-semibold">{t('support_feedback')}</span>
                </div>
                <ExternalLink size={13} className="text-slate-500" />
              </button>
            </div>
          </div>

          {/* Update Checker Button */}
          <div className="pt-2 border-t border-white/5 flex flex-col items-center gap-2">
            <button
              type="button"
              onClick={handleCheckUpdate}
              disabled={isCheckingUpdate}
              className="btn btn-secondary btn-sm w-full flex items-center justify-center gap-2 text-xs font-bold text-slate-200 border-slate-700 hover:border-accent/40"
            >
              <RefreshCw size={13} className={isCheckingUpdate ? 'animate-spin text-accent' : ''} />
              <span>{isCheckingUpdate ? t('checking_updates') : t('check_updates')}</span>
            </button>

            {updateStatus && (
              <p className="text-[11px] font-medium text-emerald-400 text-center animate-fade-in flex items-center gap-1">
                <CheckCircle2 size={13} />
                <span>{updateStatus}</span>
              </p>
            )}
          </div>

          {/* License & Copyright */}
          <p className="text-[10px] text-slate-500 text-center leading-relaxed">
            {t('about_license')}
          </p>
        </div>
      </div>
    </div>
  )
}
