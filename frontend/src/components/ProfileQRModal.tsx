import React, { useState, useEffect } from 'react'
import {
  QrCode,
  Copy,
  Check,
  X,
  Share2,
  Server,
  Smartphone,
  Layers,
} from 'lucide-react'
import { api } from '../lib/api'
import { t } from '../lib/i18n'
import { generateQRCodeSVG } from '../lib/qrcode'
import type { Profile } from '../types'
import CountryFlag from './CountryFlag'

interface Props {
  profile: Profile | null
  isOpen: boolean
  onClose: () => void
}

export default function ProfileQRModal({ profile, isOpen, onClose }: Props) {
  const [shareURI, setShareURI] = useState<string>('')
  const [copied, setCopied] = useState<boolean>(false)
  const [isLoading, setIsLoading] = useState<boolean>(false)

  useEffect(() => {
    if (isOpen && profile) {
      setIsLoading(true)
      api.getProfileShareURI(profile.id)
        .then(uri => {
          setShareURI(uri || profile.raw_uri || profile.config_content || '')
        })
        .catch(() => {
          setShareURI(profile.raw_uri || profile.config_content || '')
        })
        .finally(() => {
          setIsLoading(false)
        })
    } else {
      setShareURI('')
    }
  }, [isOpen, profile])

  if (!isOpen || !profile) return null

  const handleCopy = () => {
    if (!shareURI) return
    navigator.clipboard.writeText(shareURI)
    setCopied(true)
    setTimeout(() => setCopied(false), 2000)
  }

  return (
    <div className="fixed inset-0 z-50 flex items-center justify-center p-4 bg-black/75 backdrop-blur-md animate-fade-in">
      <div className="relative w-full max-w-sm bg-neutral-900 border border-neutral-800 rounded-3xl shadow-2xl overflow-hidden flex flex-col">
        {/* Header */}
        <div className="flex items-center justify-between p-5 border-b border-neutral-800 bg-neutral-950/40">
          <div className="flex items-center gap-2.5 min-w-0">
            <div className="w-9 h-9 rounded-2xl bg-cyan-500/10 border border-cyan-500/20 flex items-center justify-center text-cyan-400 shrink-0">
              <QrCode className="w-5 h-5" />
            </div>
            <div className="min-w-0">
              <h3 className="text-sm font-bold text-white truncate flex items-center gap-1.5">
                <CountryFlag codeOrName={profile.country_code || profile.name} className="w-4 h-3 rounded-sm shrink-0" />
                <span className="truncate">{profile.name}</span>
              </h3>
              <p className="text-[11px] text-neutral-400 uppercase font-mono tracking-wider">{profile.protocol}</p>
            </div>
          </div>
          <button
            onClick={onClose}
            className="p-1.5 rounded-xl bg-neutral-800/60 hover:bg-neutral-700 text-neutral-400 hover:text-white transition-all shrink-0"
          >
            <X className="w-4 h-4" />
          </button>
        </div>

        {/* Body */}
        <div className="p-6 flex flex-col items-center text-center space-y-4">
          {isLoading ? (
            <div className="py-12 text-xs text-neutral-400">{t('generating')}</div>
          ) : shareURI ? (
            <>
              {/* QR Container */}
              <div className="p-3.5 bg-white rounded-2xl shadow-2xl shadow-cyan-500/10">
                <img
                  src={generateQRCodeSVG(shareURI, 180)}
                  alt="Profile QR Code"
                  className="w-44 h-44 object-contain"
                />
              </div>

              <div className="space-y-1">
                <h4 className="text-xs font-bold text-white flex items-center justify-center gap-1.5">
                  <Smartphone className="w-4 h-4 text-cyan-400" />
                  {t('qr_modal_scan_title')}
                </h4>
                <p className="text-[11px] text-neutral-400 max-w-xs mx-auto leading-relaxed">
                  {t('qr_modal_scan_desc')}
                </p>
              </div>

              {/* Copy URI Button */}
              <button
                type="button"
                onClick={handleCopy}
                className="w-full flex items-center justify-center gap-2 py-2.5 px-4 rounded-xl bg-cyan-500/10 hover:bg-cyan-500/20 border border-cyan-500/30 text-cyan-300 text-xs font-semibold transition"
              >
                {copied ? <Check className="w-4 h-4 text-emerald-400" /> : <Copy className="w-4 h-4" />}
                <span>{copied ? t('qr_modal_copied') : t('qr_modal_copy')}</span>
              </button>
            </>
          ) : (
            <div className="py-8 text-xs text-rose-400">
              {t('qr_modal_no_uri')}
            </div>
          )}
        </div>

        {/* Footer */}
        <div className="p-4 border-t border-neutral-800 bg-neutral-950/20 flex justify-end">
          <button
            type="button"
            onClick={onClose}
            className="w-full py-2 px-4 rounded-xl bg-neutral-800 hover:bg-neutral-700 text-xs font-semibold text-neutral-200 transition"
          >
            {t('close')}
          </button>
        </div>
      </div>
    </div>
  )
}
