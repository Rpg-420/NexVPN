import React, { useState, useEffect } from 'react'
import {
  Smartphone,
  Wifi,
  Copy,
  Check,
  X,
  RefreshCw,
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
  const [isLoading, setIsLoading] = useState<boolean>(false)
  const [copied, setCopied] = useState<boolean>(false)

  const initRemote = async () => {
    setIsLoading(true)
    try {
      const info = await api.getMobileRemoteInfo()
      if (info && info.running && info.url) {
        setUrl(info.url)
      } else {
        const launchedUrl = await api.startMobileRemote(10815)
        setUrl(launchedUrl)
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

  const handleCopy = () => {
    if (!url) return
    navigator.clipboard.writeText(url)
    setCopied(true)
    setTimeout(() => setCopied(false), 2000)
  }

  return (
    <div className="fixed inset-0 z-50 flex items-center justify-center p-4 bg-black/70 backdrop-blur-md animate-fade-in">
      <div className="relative w-full max-w-md bg-neutral-900 border border-neutral-800 rounded-3xl shadow-2xl overflow-hidden flex flex-col">
        {/* Header */}
        <div className="flex items-center justify-between p-6 border-b border-neutral-800 bg-neutral-950/40">
          <div className="flex items-center gap-3">
            <div className="w-10 h-10 rounded-2xl bg-sky-500/10 border border-sky-500/20 flex items-center justify-center text-sky-400">
              <Smartphone className="w-5 h-5" />
            </div>
            <div>
              <h3 className="text-lg font-bold text-white flex items-center gap-2">{t('remote_modal_title')}</h3>
              <p className="text-xs text-neutral-400">{t('remote_modal_sub')}</p>
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
        <div className="p-6 flex flex-col items-center text-center space-y-4">
          {isLoading ? (
            <div className="py-12 flex flex-col items-center justify-center space-y-3">
              <RefreshCw className="w-8 h-8 text-sky-400 animate-spin" />
              <p className="text-xs text-neutral-400">Starting local Wi-Fi controller...</p>
            </div>
          ) : (
            <>
              {/* QR Code Container */}
              <div className="p-4 bg-white rounded-3xl shadow-2xl shadow-sky-500/10">
                {url && (
                  <img src={generateQRCodeSVG(url, 200)} alt="Scan to Control" className="w-44 h-44" />
                )}
              </div>

              <div className="space-y-1">
                <h4 className="text-sm font-bold text-white flex items-center justify-center gap-1.5">
                  <Wifi className="w-4 h-4 text-emerald-400" />
                  {t('remote_scan_title')}
                </h4>
                <p className="text-[11px] text-neutral-400 max-w-xs mx-auto leading-relaxed">
                  {t('remote_scan_sub')}
                </p>
              </div>

              {/* URL Pill */}
              <div className="w-full flex items-center justify-between p-3 rounded-2xl bg-neutral-950/60 border border-neutral-800 text-xs font-mono text-neutral-300">
                <span className="truncate mr-2 ltr">{url}</span>
                <button
                  type="button"
                  onClick={handleCopy}
                  className="px-3 py-1.5 rounded-xl bg-neutral-800 hover:bg-neutral-700 text-sky-300 text-xs font-semibold flex items-center gap-1.5 transition"
                >
                  {copied ? <Check className="w-3.5 h-3.5 text-emerald-400" /> : <Copy className="w-3.5 h-3.5" />}
                  <span>{copied ? t('remote_copied_btn') : t('remote_copy_btn')}</span>
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
            {t('remote_done_btn')}
          </button>
        </div>
      </div>
    </div>
  )
}
