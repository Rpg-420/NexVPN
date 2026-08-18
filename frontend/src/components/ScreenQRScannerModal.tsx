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
    <div className="fixed inset-0 z-50 flex items-center justify-center p-4 bg-black/80 backdrop-blur-md animate-fade-in">
      <div className="relative w-full max-w-xl bg-slate-900 border border-slate-700/80 rounded-3xl shadow-2xl overflow-hidden flex flex-col max-h-[90vh]">
        {/* Header */}
        <div className="flex items-center justify-between p-5 border-b border-slate-800 bg-slate-950/40">
          <div className="flex items-center gap-3">
            <div className="w-10 h-10 rounded-2xl bg-cyan-500/20 border border-cyan-500/30 flex items-center justify-center text-cyan-400">
              <Scan className="w-5 h-5" />
            </div>
            <div>
              <h3 className="text-base font-bold text-white flex items-center gap-2">
                <span>{t('screen_qr_title')}</span>
                <span className="px-2 py-0.5 text-[10px] font-bold rounded-full bg-cyan-500/20 text-cyan-300 border border-cyan-500/30">
                  AI Scanner
                </span>
              </h3>
              <p className="text-xs text-slate-400">{t('screen_qr_subtitle')}</p>
            </div>
          </div>
          <button
            onClick={onClose}
            className="p-1.5 rounded-xl bg-slate-800/60 hover:bg-slate-700 text-slate-400 hover:text-white transition-all"
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
              className="p-4 rounded-2xl bg-gradient-to-b from-slate-800/80 to-slate-900/90 border border-slate-700/80 hover:border-cyan-500/50 hover:bg-slate-800 transition flex flex-col items-center justify-center text-center gap-2 group active:scale-95"
            >
              <div className="w-10 h-10 rounded-xl bg-cyan-500/10 group-hover:bg-cyan-500/20 border border-cyan-500/30 flex items-center justify-center text-cyan-400 transition">
                <Monitor className="w-5 h-5" />
              </div>
              <span className="text-xs font-bold text-slate-200">{t('scan_from_screen')}</span>
              <span className="text-[10px] text-slate-400 leading-tight">Capture window or desktop</span>
            </button>

            {/* Paste from Clipboard Tile */}
            <div className="p-4 rounded-2xl bg-gradient-to-b from-slate-800/80 to-slate-900/90 border border-slate-700/80 flex flex-col items-center justify-center text-center gap-2">
              <div className="w-10 h-10 rounded-xl bg-purple-500/10 border border-purple-500/30 flex items-center justify-center text-purple-400">
                <Clipboard className="w-5 h-5" />
              </div>
              <span className="text-xs font-bold text-slate-200">{t('paste_from_clipboard')}</span>
              <span className="text-[10px] text-purple-300 font-mono bg-purple-500/10 px-2 py-0.5 rounded border border-purple-500/20">
                Press Ctrl + V
              </span>
            </div>

            {/* Select Image File Button */}
            <button
              type="button"
              onClick={() => fileInputRef.current?.click()}
              className="p-4 rounded-2xl bg-gradient-to-b from-slate-800/80 to-slate-900/90 border border-slate-700/80 hover:border-emerald-500/50 hover:bg-slate-800 transition flex flex-col items-center justify-center text-center gap-2 group active:scale-95"
            >
              <div className="w-10 h-10 rounded-xl bg-emerald-500/10 group-hover:bg-emerald-500/20 border border-emerald-500/30 flex items-center justify-center text-emerald-400 transition">
                <Upload className="w-5 h-5" />
              </div>
              <span className="text-xs font-bold text-slate-200">{t('select_image_file')}</span>
              <span className="text-[10px] text-slate-400 leading-tight">PNG, JPG, WebP</span>
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
            <div className="p-3.5 rounded-2xl bg-cyan-950/40 border border-cyan-500/40 flex items-center gap-3 animate-pulse">
              <RefreshCw className="w-4 h-4 text-cyan-400 animate-spin" />
              <span className="text-xs text-cyan-200">{t('capturing_screen')}</span>
            </div>
          )}

          {/* Error Message */}
          {errorMsg && (
            <div className="p-3.5 rounded-2xl bg-rose-950/40 border border-rose-500/40 flex items-center gap-3 text-rose-300 text-xs">
              <AlertCircle className="w-4 h-4 text-rose-400 shrink-0" />
              <span>{errorMsg}</span>
            </div>
          )}

          {/* Decoded QR Content Box */}
          {decodedText && (
            <div className="p-4 rounded-2xl bg-slate-950/80 border border-slate-700/80 space-y-3 animate-in fade-in duration-200">
              <div className="flex items-center justify-between">
                <div className="flex items-center gap-2 text-xs font-bold text-emerald-400">
                  <CheckCircle2 className="w-4 h-4" />
                  <span>{t('qr_decoded_success')}</span>
                </div>
                <span className="text-[10px] font-mono text-slate-400 bg-slate-800 px-2 py-0.5 rounded">
                  {decodedText.slice(0, 15)}...
                </span>
              </div>

              <div className="p-2.5 bg-slate-900 rounded-xl font-mono text-[11px] text-slate-300 break-all select-all max-h-24 overflow-y-auto border border-white/5">
                {decodedText}
              </div>

              <button
                type="button"
                onClick={handleImport}
                disabled={isImporting || importedSuccess}
                className="w-full py-2.5 px-4 bg-gradient-to-r from-cyan-600 to-blue-600 hover:from-cyan-500 hover:to-blue-500 text-white rounded-xl text-xs font-bold shadow-md shadow-cyan-600/20 flex items-center justify-center gap-2 active:scale-95 transition-all"
              >
                {isImporting ? (
                  <RefreshCw className="w-3.5 h-3.5 animate-spin" />
                ) : importedSuccess ? (
                  <CheckCircle2 className="w-3.5 h-3.5 text-emerald-300" />
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
        <div className="flex items-center justify-end p-4 border-t border-slate-800 bg-slate-950/40">
          <button
            type="button"
            onClick={onClose}
            className="px-4 py-2 bg-slate-800 hover:bg-slate-700 text-slate-300 rounded-xl text-xs font-semibold"
          >
            {t('close')}
          </button>
        </div>
      </div>
    </div>
  )
}
