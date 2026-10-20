import { useState, useEffect } from 'react'
import {
  Smartphone,
  Copy,
  Check,
  X,
  RefreshCw,
  KeyRound,
  ShieldCheck,
  ShieldAlert,
  Lock,
  Unlock,
  AlertTriangle,
  Power,
} from 'lucide-react'
import { api } from '../lib/api'
import { t } from '../lib/i18n'
import { generateQRCodeSVG } from '../lib/qrcode'
import { ModalShell } from '../components/ui/modal-shell'

interface Props {
  isOpen: boolean
  onClose: () => void
}

export default function MobileRemoteModal({ isOpen, onClose }: Props) {
  const [url, setUrl] = useState<string>('')
  const [authUrl, setAuthUrl] = useState<string>('')
  const [pin, setPin] = useState<string>('')
  const [isLoading, setIsLoading] = useState<boolean>(false)
  const [isStopping, setIsStopping] = useState<boolean>(false)
  const [copiedUrl, setCopiedUrl] = useState<boolean>(false)
  const [copiedPin, setCopiedPin] = useState<boolean>(false)
  const [idleMinutes, setIdleMinutes] = useState<number>(30)
  // Real security posture reported by the backend, rather than a hard-coded claim.
  const [tlsActive, setTlsActive] = useState<boolean>(false)
  const [lanOnly, setLanOnly] = useState<boolean>(true)
  const [isRunning, setIsRunning] = useState<boolean>(false)
  const [errorMsg, setErrorMsg] = useState<string | null>(null)

  const initRemote = async () => {
    setIsLoading(true)
    setErrorMsg(null)
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
        setIsRunning(Boolean(info.running))
        setTlsActive(Boolean(info.tls_active))
        setLanOnly(info.lan_only !== false && Boolean(info.lan_only))
        if (info.idle_timeout_sec > 0) {
          setIdleMinutes(Math.round(info.idle_timeout_sec / 60))
        }
      }
    } catch (err) {
      console.error('Failed to start mobile remote:', err)
      setErrorMsg(err instanceof Error ? err.message : 'Failed to start the remote controller')
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

  const handleStop = async () => {
    setIsStopping(true)
    try {
      await api.stopMobileRemote()
      // Re-read rather than assuming: the modal must reflect the server's
      // actual state, and the URL/PIN are invalidated once it stops.
      const info = await api.getMobileRemoteInfo()
      setIsRunning(Boolean(info?.running))
      if (!info?.running) {
        setUrl('')
        setAuthUrl('')
        setPin('')
      }
    } catch (err) {
      console.error('Failed to stop mobile remote:', err)
      setErrorMsg(err instanceof Error ? err.message : 'Failed to stop the remote controller')
    } finally {
      setIsStopping(false)
    }
  }

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
    <ModalShell
      isOpen={isOpen}
      onClose={onClose}
      maxWidth="max-w-md"
    >
        {/* Header */}
        <div className="flex items-center justify-between p-5 border-b border-border bg-background">
          <div className="flex items-center gap-3">
            <div className="w-10 h-10 rounded-2xl border border-border bg-muted flex items-center justify-center text-muted-foreground">
              <Smartphone className="w-5 h-5" />
            </div>
            <div>
              <h3 className="text-base font-bold text-foreground flex items-center gap-2">{t('remote_modal_title') || 'Mobile Remote Controller'}</h3>
              <p className="text-xs text-muted-foreground">{t('remote_modal_sub') || 'Scan QR with your phone on same Wi-Fi'}</p>
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
        <div className="p-5 flex flex-col items-center text-center space-y-4">
          {isLoading ? (
            <div className="py-12 flex flex-col items-center justify-center space-y-3">
              <RefreshCw className="w-8 h-8 text-primary animate-spin" />
              <p className="text-xs text-muted-foreground">Starting secure Wi-Fi controller...</p>
            </div>
          ) : (
            <>
              {/* QR Code Container */}
              <div className="p-3.5 bg-card rounded-2xl shadow-2xl">
                {(authUrl || url) && (
                  <img src={generateQRCodeSVG(authUrl || url, 180)} alt="Scan to Control" className="w-40 h-40" />
                )}
              </div>

              {/* Security PIN Badge */}
              {pin && (
                <div className="w-full flex items-center justify-between px-4 py-2.5 rounded-2xl bg-info/40 border border-border">
                  <div className="flex items-center gap-2">
                    <KeyRound className="w-4 h-4 text-primary shrink-0" />
                    <div className="text-start rtl:text-end">
                      <div className="text-[10px] text-primary font-semibold uppercase tracking-wider">{t('remote_pin_label')}</div>
                      <div className="text-lg font-mono font-black text-foreground tracking-widest">{pin}</div>
                    </div>
                  </div>
                  <button
                    type="button"
                    onClick={handleCopyPin}
                    className="px-3 py-1 rounded-xl bg-info/20 hover:bg-primary/30 text-primary text-xs font-semibold flex items-center gap-1.5 transition"
                  >
                    {copiedPin ? <Check className="w-3.5 h-3.5 text-success" /> : <Copy className="w-3.5 h-3.5" />}
                    <span>{copiedPin ? t('remote_pin_copied') : t('remote_copy_pin')}</span>
                  </button>
                </div>
              )}

              <div className="space-y-1">
                <h4 className="text-xs font-bold text-foreground flex items-center justify-center gap-1.5">
                  <ShieldCheck className="w-4 h-4 text-success" />
                  {t('remote_pair_title')}
                </h4>
                <p className="text-[11px] text-muted-foreground max-w-xs mx-auto leading-relaxed">
                  {t('remote_pair_desc')}
                </p>
              </div>

              {/* Real security posture, read from the backend */}
              {isRunning && (
                <div className="w-full grid grid-cols-2 gap-2">
                  <div className={`flex items-center gap-2 p-2.5 rounded-2xl border text-start rtl:text-end ${
                    tlsActive
                      ? 'bg-success/30 border-success/30'
                      : 'bg-destructive/30 border-destructive/40'
                  }`}>
                    {tlsActive ? (
                      <Lock className="w-4 h-4 text-success shrink-0" />
                    ) : (
                      <Unlock className="w-4 h-4 text-destructive shrink-0" />
                    )}
                    <div className="min-w-0">
                      <div className="text-[10px] font-bold uppercase tracking-wider text-muted-foreground">
                        {t('remote_tls_label')}
                      </div>
                      <div className={`text-[11px] font-semibold ${tlsActive ? 'text-success' : 'text-destructive'}`}>
                        {tlsActive ? t('remote_tls_on') : t('remote_tls_off')}
                      </div>
                    </div>
                  </div>

                  <div className={`flex items-center gap-2 p-2.5 rounded-2xl border text-start rtl:text-end ${
                    lanOnly
                      ? 'bg-success/30 border-success/30'
                      : 'bg-destructive/30 border-destructive/40'
                  }`}>
                    <ShieldAlert className={`w-4 h-4 shrink-0 ${lanOnly ? 'text-success' : 'text-destructive'}`} />
                    <div className="min-w-0">
                      <div className="text-[10px] font-bold uppercase tracking-wider text-muted-foreground">
                        {t('remote_scope_label')}
                      </div>
                      <div className={`text-[11px] font-semibold ${lanOnly ? 'text-success' : 'text-destructive'}`}>
                        {lanOnly ? t('remote_scope_lan') : t('remote_scope_any')}
                      </div>
                    </div>
                  </div>
                </div>
              )}

              {errorMsg && (
                <div className="w-full flex items-start gap-2 p-3 rounded-2xl bg-destructive/30 border border-destructive/40 text-start rtl:text-end">
                  <AlertTriangle className="w-4 h-4 text-destructive shrink-0 mt-0.5" />
                  <p className="text-[11px] text-destructive/90 leading-relaxed">{errorMsg}</p>
                </div>
              )}

              {/* LAN exposure + auto-stop warning */}
              <div className="w-full flex items-start gap-2 p-3 rounded-2xl bg-warning/30 border border-warning/30 text-start rtl:text-end">
                <AlertTriangle className="w-4 h-4 text-warning shrink-0 mt-0.5" />
                <p className="text-[11px] text-warning/90 leading-relaxed">
                  {t('remote_lan_warning')}
                  {' '}
                  {t('remote_auto_stop').replace('{min}', String(idleMinutes))}
                </p>
              </div>

              {/* URL Pill */}
              <div className="w-full flex items-center justify-between p-2.5 rounded-2xl bg-background border border-border text-xs font-mono text-muted-foreground">
                <span className="truncate me-2 ltr text-[11px]">{url}</span>
                <button
                  type="button"
                  onClick={handleCopyUrl}
                  className="px-2.5 py-1 rounded-xl bg-muted hover:bg-muted/90 text-muted-foreground text-xs font-semibold flex items-center gap-1.5 transition"
                >
                  {copiedUrl ? <Check className="w-3.5 h-3.5 text-success" /> : <Copy className="w-3.5 h-3.5" />}
                  <span>{copiedUrl ? (t('remote_copied_btn') || 'Copied') : (t('remote_copy_btn') || 'Copy URL')}</span>
                </button>
              </div>
            </>
          )}
        </div>

        {/* Footer */}
        <div className="p-4 border-t border-border bg-background flex items-center justify-between gap-2">
          {isRunning ? (
            <button
              type="button"
              onClick={handleStop}
              disabled={isStopping}
              className="px-4 py-2 rounded-xl bg-destructive/15 hover:bg-destructive/25 disabled:opacity-50 border border-destructive/40 text-destructive font-semibold text-xs transition-all flex items-center gap-1.5"
            >
              {isStopping ? <RefreshCw className="w-3.5 h-3.5 animate-spin" /> : <Power className="w-3.5 h-3.5" />}
              <span>{isStopping ? t('remote_stopping_btn') : t('remote_stop_btn')}</span>
            </button>
          ) : (
            <span className="text-[11px] text-muted-foreground">{t('remote_stopped_hint')}</span>
          )}
          <button
            onClick={onClose}
            className="px-5 py-2 rounded-xl bg-muted hover:bg-muted/90 text-foreground font-medium text-xs transition-all"
          >
            {t('remote_done_btn') || 'Close'}
          </button>
        </div>
      </ModalShell>
  )
}
