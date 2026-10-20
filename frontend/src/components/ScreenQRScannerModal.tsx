import React, { useState, useEffect, useRef } from 'react'
import {
  Scan,
  Monitor,
  Clipboard,
  Upload,
  X,
  CheckCircle2,
  AlertCircle,
  Sparkles,
  RefreshCw,
} from 'lucide-react'
import jsQR from 'jsqr'
import { api } from '../lib/api'
import { sound } from '../lib/sound'
import { t } from '../lib/i18n'
import { ModalShell } from '../components/ui/modal-shell'

interface Props {
  isOpen: boolean
  onClose: () => void
  onProfileImported?: () => void
}

export default function ScreenQRScannerModal({ isOpen, onClose, onProfileImported }: Props) {
  const [isScanningScreen, setIsScanningScreen] = useState(false)
  const [decodedText, setDecodedText] = useState<string | null>(null)
  const [errorMsg, setErrorMsg] = useState<string | null>(null)
  const [isImporting, setIsImporting] = useState(false)
  const [importedSuccess, setImportedSuccess] = useState(false)
  const fileInputRef = useRef<HTMLInputElement>(null)

  // Listen to paste events (Ctrl+V) when modal is open
  useEffect(() => {
    if (!isOpen) return

    const handlePaste = (e: ClipboardEvent) => {
      const items = e.clipboardData?.items
      if (!items) return

      for (let i = 0; i < items.length; i++) {
        if (items[i].type.indexOf('image') !== -1) {
          const blob = items[i].getAsFile()
          if (blob) {
            decodeImageBlob(blob)
            e.preventDefault()
            break
          }
        }
      }
    }

    window.addEventListener('paste', handlePaste)
    return () => window.removeEventListener('paste', handlePaste)
  }, [isOpen])

  if (!isOpen) return null

  const decodeImageBlob = (blob: Blob) => {
    setErrorMsg(null)
    setDecodedText(null)
    const reader = new FileReader()
    reader.onload = e => {
      const img = new Image()
      img.onload = () => {
        const canvas = document.createElement('canvas')
        canvas.width = img.width
        canvas.height = img.height
        const ctx = canvas.getContext('2d')
        if (!ctx) {
          setErrorMsg('Failed to process image canvas')
          return
        }
        ctx.drawImage(img, 0, 0)
        const imageData = ctx.getImageData(0, 0, canvas.width, canvas.height)
        const code = jsQR(imageData.data, imageData.width, imageData.height, {
          inversionAttempts: 'attemptBoth',
        })

        if (code && code.data) {
          setDecodedText(code.data.trim())
          sound.playSuccess()
        } else {
          setErrorMsg(t('qr_not_found') || 'No valid QR code found in the image. Please try again.')
        }
      }
      img.src = e.target?.result as string
    }
    reader.readAsDataURL(blob)
  }

  const handleCaptureScreen = async () => {
    setErrorMsg(null)
    setDecodedText(null)
    setIsScanningScreen(true)

    try {
      const stream = await navigator.mediaDevices.getDisplayMedia({
        video: { cursor: 'always' } as any,
        audio: false,
      })

      const video = document.createElement('video')
      video.srcObject = stream
      await video.play()

      setTimeout(() => {
        const canvas = document.createElement('canvas')
        canvas.width = video.videoWidth
        canvas.height = video.videoHeight
        const ctx = canvas.getContext('2d')
        if (ctx) {
          ctx.drawImage(video, 0, 0, canvas.width, canvas.height)
          const imageData = ctx.getImageData(0, 0, canvas.width, canvas.height)
          const code = jsQR(imageData.data, imageData.width, imageData.height, {
            inversionAttempts: 'attemptBoth',
          })

          if (code && code.data) {
            setDecodedText(code.data.trim())
            sound.playSuccess()
          } else {
            setErrorMsg(t('qr_not_found_screen') || 'No QR code detected on the selected screen area.')
          }
        }

        stream.getTracks().forEach(tr => tr.stop())
        setIsScanningScreen(false)
      }, 500)
    } catch (err: any) {
      setIsScanningScreen(false)
      if (err?.name !== 'NotAllowedError') {
        setErrorMsg('Screen capture failed: ' + (err?.message || 'Permission denied'))
      }
    }
  }

  const handleFileChange = (e: React.ChangeEvent<HTMLInputElement>) => {
    const file = e.target.files?.[0]
    if (file) {
      decodeImageBlob(file)
    }
  }

  const handleImport = async () => {
    if (!decodedText) return
    setIsImporting(true)
    setErrorMsg(null)

    try {
      const res = await api.importURI(decodedText)
      if (res) {
        setImportedSuccess(true)
        sound.playSuccess()
        if (onProfileImported) onProfileImported()
        setTimeout(() => {
          setImportedSuccess(false)
          onClose()
        }, 1200)
      } else {
        setErrorMsg('Import failed. Unrecognized profile format.')
      }
    } catch (err: any) {
      setErrorMsg(err?.message || 'Import failed')
    } finally {
      setIsImporting(false)
    }
  }

  return (
    <ModalShell
      isOpen={isOpen}
      onClose={onClose}
      maxWidth="max-w-xl"
      className="max-h-[90vh]"
    >
        {/* Header */}
        <div className="flex items-center justify-between p-5 border-b border-border bg-background">
          <div className="flex items-center gap-3">
            <div className="w-10 h-10 rounded-2xl bg-info/20 border border-border flex items-center justify-center text-primary">
              <Scan className="w-5 h-5" />
            </div>
            <div>
              <h3 className="text-base font-bold text-foreground flex items-center gap-2">
                <span>{t('screen_qr_title')}</span>
                <span className="px-2 py-0.5 text-[10px] font-bold rounded-full bg-primary/20 text-primary border border-primary/30">
                  AI Scanner
                </span>
              </h3>
              <p className="text-xs text-muted-foreground">{t('screen_qr_subtitle')}</p>
            </div>
          </div>
          <button
            onClick={onClose}
            className="p-1.5 rounded-xl bg-muted hover:bg-muted/90 text-muted-foreground hover:text-foreground transition-all"
          >
            <X className="w-4 h-4" />
          </button>
        </div>

        {/* Body */}
        <div className="p-5 space-y-4 overflow-y-auto flex-1">
          {/* Action Tiles Grid */}
          <div className="grid grid-cols-1 sm:grid-cols-3 gap-3">
            {/* Capture Screen Button */}
            <button
              type="button"
              onClick={handleCaptureScreen}
              disabled={isScanningScreen}
              className="bg-card/80 p-4 rounded-2xl border border-border hover:border-primary/50 transition flex flex-col items-center justify-center text-center gap-2 group active:scale-95"
            >
              <div className="w-10 h-10 rounded-xl bg-info/10 group-hover:bg-primary/20 border border-border flex items-center justify-center text-primary transition">
                <Monitor className="w-5 h-5" />
              </div>
              <span className="text-xs font-bold text-foreground">{t('scan_from_screen')}</span>
              <span className="text-[10px] text-muted-foreground leading-tight">Capture window or desktop</span>
            </button>

            {/* Paste from Clipboard Tile */}
            <div className="bg-card/80 p-4 rounded-2xl border border-border flex flex-col items-center justify-center text-center gap-2">
              <div className="w-10 h-10 rounded-xl bg-primary/10 border border-primary/30 flex items-center justify-center text-primary">
                <Clipboard className="w-5 h-5" />
              </div>
              <span className="text-xs font-bold text-foreground">{t('paste_from_clipboard')}</span>
              <span className="text-[10px] text-primary font-mono bg-primary/10 px-2 py-0.5 rounded border border-primary/20">
                Press Ctrl + V
              </span>
            </div>

            {/* Select Image File Button */}
            <button
              type="button"
              onClick={() => fileInputRef.current?.click()}
              className="bg-card/80 p-4 rounded-2xl border border-border hover:border-success/50 transition flex flex-col items-center justify-center text-center gap-2 group active:scale-95"
            >
              <div className="w-10 h-10 rounded-xl bg-success/10 group-hover:bg-success/20 border border-success/30 flex items-center justify-center text-success transition">
                <Upload className="w-5 h-5" />
              </div>
              <span className="text-xs font-bold text-foreground">{t('select_image_file')}</span>
              <span className="text-[10px] text-muted-foreground leading-tight">PNG, JPG, WebP</span>
            </button>
            <input
              ref={fileInputRef}
              type="file"
              accept="image/*"
              className="hidden"
              onChange={handleFileChange}
            />
          </div>

          {/* Status / Scanning Indicator */}
          {isScanningScreen && (
            <div className="p-3.5 rounded-2xl bg-info/40 border border-primary/40 flex items-center gap-3 animate-pulse">
              <RefreshCw className="w-4 h-4 text-primary animate-spin" />
              <span className="text-xs text-primary">{t('capturing_screen')}</span>
            </div>
          )}

          {/* Error Message */}
          {errorMsg && (
            <div className="p-3.5 rounded-2xl bg-destructive/40 border border-destructive/40 flex items-center gap-3 text-destructive text-xs">
              <AlertCircle className="w-4 h-4 text-destructive shrink-0" />
              <span>{errorMsg}</span>
            </div>
          )}

          {/* Decoded QR Content Box */}
          {decodedText && (
            <div className="p-4 rounded-2xl bg-background border border-border space-y-3 animate-in fade-in duration-200">
              <div className="flex items-center justify-between">
                <div className="flex items-center gap-2 text-xs font-bold text-success">
                  <CheckCircle2 className="w-4 h-4" />
                  <span>{t('qr_decoded_success')}</span>
                </div>
                <span className="text-[10px] font-mono text-muted-foreground bg-muted px-2 py-0.5 rounded">
                  {decodedText.slice(0, 15)}...
                </span>
              </div>

              <div className="p-2.5 bg-card rounded-xl font-mono text-[11px] text-muted-foreground break-all select-all max-h-24 overflow-y-auto border border-border">
                {decodedText}
              </div>

              <button
                type="button"
                onClick={handleImport}
                disabled={isImporting || importedSuccess}
                className="bg-primary w-full py-2.5 px-4 text-primary-foreground rounded-xl text-xs font-bold shadow-sm flex items-center justify-center gap-2 active:scale-95 transition-all"
              >
                {isImporting ? (
                  <RefreshCw className="w-3.5 h-3.5 animate-spin" />
                ) : importedSuccess ? (
                  <CheckCircle2 className="w-3.5 h-3.5 text-success" />
                ) : (
                  <Sparkles className="w-3.5 h-3.5" />
                )}
                <span>
                  {importedSuccess
                    ? t('imported_successfully')
                    : isImporting
                    ? t('importing')
                    : t('add_to_profiles_btn')}
                </span>
              </button>
            </div>
          )}
        </div>

        {/* Footer */}
        <div className="flex items-center justify-end p-4 border-t border-border bg-background">
          <button
            type="button"
            onClick={onClose}
            className="px-4 py-2 bg-muted hover:bg-muted/90 text-muted-foreground rounded-xl text-xs font-semibold"
          >
            {t('close')}
          </button>
        </div>
      </ModalShell>
  )
}
