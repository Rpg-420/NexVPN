import React, { useState, useEffect } from 'react'
import {
  Smartphone,
  Wifi,
  Copy,
  Check,
  X,
  RefreshCw,
  KeyRound,
  ShieldCheck,
} from 'lucide-react'
import { api } from '../lib/api'
import { t } from '../lib/i18n'
import { generateQRCodeSVG } from '../lib/qrcode'

interface Props {
  isOpen: boolean
  onClose: () => void
}

export default function MobileRemoteModal({ isOpen, onClose }: Props) {
  const [url, setUrl] = useState<string>('')
  const [authUrl, setAuthUrl] = useState<string>('')
  const [pin, setPin] = useState<string>('')
  const [isLoading, setIsLoading] = useState<boolean>(false)
  const [copiedUrl, setCopiedUrl] = useState<boolean>(false)
  const [copiedPin, setCopiedPin] = useState<boolean>(false)

  const initRemote = async () => {
    setIsLoading(true)
    try {
      let info = await api.getMobileRemoteInfo()
      if (!info || !info.running) {
        await api.startMobileRemote(10815)
        info = await api.getMobileRemoteInfo()
      }
      if (info) {
        setUrl(info.url || '')
        setAuthUrl(info.auth_url || info.url || '')
        setPin(info.pin || '')
      }
    } catch (err) {
      console.error('Failed to start mobile remote:', err)
    } finally {
      setIsLoading(false)
    }
  }

  useEffect(() => {
    if (isOpen) {
      initRemote()
    }
  }, [isOpen])

  if (!isOpen) return null

  const handleCopyUrl = () => {
    if (!url) return
    navigator.clipboard.writeText(url)
    setCopiedUrl(true)
    setTimeout(() => setCopiedUrl(false), 2000)
  }

  const handleCopyPin = () => {
    if (!pin) return
    navigator.clipboard.writeText(pin)
    setCopiedPin(true)
    setTimeout(() => setCopiedPin(false), 2000)
  }

  return (
    <div className="fixed inset-0 z-50 flex items-center justify-center p-4 bg-black/70 backdrop-blur-md animate-fade-in">
      <div className="relative w-full max-w-md bg-neutral-900 border border-neutral-800 rounded-3xl shadow-2xl overflow-hidden flex flex-col">
        {/* Header */}
        <div className="flex items-center justify-between p-5 border-b border-neutral-800 bg-neutral-950/40">
          <div className="flex items-center gap-3">
            <div className="w-10 h-10 rounded-2xl bg-sky-500/10 border border-sky-500/20 flex items-center justify-center text-sky-400">
              <Smartphone className="w-5 h-5" />
            </div>
            <div>
              <h3 className="text-base font-bold text-white flex items-center gap-2">{t('remote_modal_title') || 'Mobile Remote Controller'}</h3>
              <p className="text-xs text-neutral-400">{t('remote_modal_sub') || 'Scan QR with your phone on same Wi-Fi'}</p>
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
        <div className="p-5 flex flex-col items-center text-center space-y-4">
          {isLoading ? (
            <div className="py-12 flex flex-col items-center justify-center space-y-3">
              <RefreshCw className="w-8 h-8 text-sky-400 animate-spin" />
              <p className="text-xs text-neutral-400">Starting secure Wi-Fi controller...</p>
            </div>
          ) : (
            <>
              {/* QR Code Container */}
              <div className="p-3.5 bg-white rounded-2xl shadow-2xl shadow-sky-500/10">
                {(authUrl || url) && (
                  <img src={generateQRCodeSVG(authUrl || url, 180)} alt="Scan to Control" className="w-40 h-40" />
                )}
              </div>

              {/* Security PIN Badge */}
              {pin && (
                <div className="w-full flex items-center justify-between px-4 py-2.5 rounded-2xl bg-sky-950/40 border border-sky-500/30">
                  <div className="flex items-center gap-2">
                    <KeyRound className="w-4 h-4 text-sky-400 shrink-0" />
                    <div className="text-left rtl:text-right">
                      <div className="text-[10px] text-sky-300 font-semibold uppercase tracking-wider">{t('remote_pin_label')}</div>
                      <div className="text-lg font-mono font-black text-white tracking-widest">{pin}</div>
                    </div>
                  </div>
                  <button
                    type="button"
                    onClick={handleCopyPin}
                    className="px-3 py-1 rounded-xl bg-sky-500/20 hover:bg-sky-500/30 text-sky-300 text-xs font-semibold flex items-center gap-1.5 transition"
                  >
                    {copiedPin ? <Check className="w-3.5 h-3.5 text-emerald-400" /> : <Copy className="w-3.5 h-3.5" />}
                    <span>{copiedPin ? t('remote_pin_copied') : t('remote_copy_pin')}</span>
                  </button>
                </div>
              )}

              <div className="space-y-1">
                <h4 className="text-xs font-bold text-white flex items-center justify-center gap-1.5">
                  <ShieldCheck className="w-4 h-4 text-emerald-400" />
                  {t('remote_pair_title')}
                </h4>
                <p className="text-[11px] text-neutral-400 max-w-xs mx-auto leading-relaxed">
                  {t('remote_pair_desc')}
                </p>
              </div>

              {/* URL Pill */}
              <div className="w-full flex items-center justify-between p-2.5 rounded-2xl bg-neutral-950/60 border border-neutral-800 text-xs font-mono text-neutral-300">
                <span className="truncate mr-2 ltr text-[11px]">{url}</span>
                <button
                  type="button"
                  onClick={handleCopyUrl}
                  className="px-2.5 py-1 rounded-xl bg-neutral-800 hover:bg-neutral-700 text-sky-300 text-xs font-semibold flex items-center gap-1.5 transition"
                >
                  {copiedUrl ? <Check className="w-3.5 h-3.5 text-emerald-400" /> : <Copy className="w-3.5 h-3.5" />}
                  <span>{copiedUrl ? (t('remote_copied_btn') || 'Copied') : (t('remote_copy_btn') || 'Copy URL')}</span>
                </button>
              </div>
            </>
          )}
        </div>

        {/* Footer */}
        <div className="p-4 border-t border-neutral-800/80 bg-neutral-950/60 flex items-center justify-end">
          <button
            onClick={onClose}
            className="px-5 py-2 rounded-xl bg-neutral-800 hover:bg-neutral-700 text-white font-medium text-xs transition-all"
          >
            {t('remote_done_btn') || 'Close'}
          </button>
        </div>
      </div>
    </div>
  )
}
