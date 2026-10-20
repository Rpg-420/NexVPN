import { useState, useEffect } from 'react'
import {
  QrCode,
  Copy,
  Check,
  X,
  Smartphone,
} from 'lucide-react'
import { api } from '../lib/api'
import { t } from '../lib/i18n'
import { generateQRCodeSVG } from '../lib/qrcode'
import type { Profile } from '../types'
import CountryFlag from './CountryFlag'
import { ModalShell } from '../components/ui/modal-shell'

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
    <ModalShell
      isOpen={isOpen}
      onClose={onClose}
      maxWidth="max-w-sm"
    >
        {/* Header */}
        <div className="flex items-center justify-between p-5 border-b border-border bg-background">
          <div className="flex items-center gap-2.5 min-w-0">
            <div className="w-9 h-9 rounded-2xl border border-border bg-muted flex items-center justify-center text-muted-foreground shrink-0">
              <QrCode className="w-5 h-5" />
            </div>
            <div className="min-w-0">
              <h3 className="text-sm font-bold text-foreground truncate flex items-center gap-1.5">
                <CountryFlag codeOrName={profile.country_code || profile.name} className="w-4 h-3 rounded-sm shrink-0" />
                <CountryFlag codeOrName={profile.country_code || profile.name} size="xs" className="shrink-0" />
                <span className="truncate">{profile.name}</span>
              </h3>
              <p className="text-[11px] text-muted-foreground uppercase font-mono tracking-wider">{profile.protocol}</p>
            </div>
          </div>
          <button
            onClick={onClose}
            className="p-1.5 rounded-xl bg-muted hover:bg-muted/90 text-muted-foreground hover:text-foreground transition-all shrink-0"
          >
            <X className="w-4 h-4" />
          </button>
        </div>

        {/* Body */}
        <div className="p-6 flex flex-col items-center text-center space-y-4">
          {isLoading ? (
            <div className="py-12 text-xs text-muted-foreground">{t('generating')}</div>
          ) : shareURI ? (
            <>
              {/* QR Container */}
              <div className="p-3.5 bg-card rounded-2xl shadow-2xl">
                <img
                  src={generateQRCodeSVG(shareURI, 180)}
                  alt="Profile QR Code"
                  className="w-44 h-44 object-contain"
                />
              </div>

              <div className="space-y-1">
                <h4 className="text-xs font-bold text-foreground flex items-center justify-center gap-1.5">
                  <Smartphone className="w-4 h-4 text-primary" />
                  {t('qr_modal_scan_title')}
                </h4>
                <p className="text-[11px] text-muted-foreground max-w-xs mx-auto leading-relaxed">
                  {t('qr_modal_scan_desc')}
                </p>
              </div>

              {/* Copy URI Button */}
              <button
                type="button"
                onClick={handleCopy}
                className="w-full flex items-center justify-center gap-2 py-2.5 px-4 rounded-xl bg-primary/10 hover:bg-primary/20 border border-border text-primary text-xs font-semibold transition"
              >
                {copied ? <Check className="w-4 h-4 text-success" /> : <Copy className="w-4 h-4" />}
                <span>{copied ? t('qr_modal_copied') : t('qr_modal_copy')}</span>
              </button>
            </>
          ) : (
            <div className="py-8 text-xs text-destructive">
              {t('qr_modal_no_uri')}
            </div>
          )}
        </div>

        {/* Footer */}
        <div className="p-4 border-t border-border bg-background flex justify-end">
          <button
            type="button"
            onClick={onClose}
            className="w-full py-2 px-4 rounded-xl bg-muted hover:bg-muted/90 text-xs font-semibold text-foreground transition"
          >
            {t('close')}
          </button>
        </div>
    </ModalShell>
  )
}
