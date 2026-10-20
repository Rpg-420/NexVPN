import { useState, useEffect } from 'react'
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
  Sparkles,
  X,
  ArrowDownToLine,
  AlertCircle,
  Tag,
  Loader2,
} from 'lucide-react'
import { api } from '../lib/api'
import { t } from '../lib/i18n'
import { ModalShell } from '../components/ui/modal-shell'
import { Button } from '@/components/ui/button'

interface Props {
  isOpen: boolean
  onClose: () => void
}

interface UpdateResult {
  has_update: boolean
  latest_version: string
  current_version: string
  release_url: string
  release_notes: string
  error?: string
}

export default function AboutModal({ isOpen, onClose }: Props) {
  const [isCheckingUpdate, setIsCheckingUpdate] = useState(false)
  const [updateResult, setUpdateResult] = useState<UpdateResult | null>(null)
  // Start empty rather than with a hard-coded number: showing a version the
  // binary did not report is worse than showing nothing briefly.
  const [appVersion, setAppVersion] = useState<string>('')
  const [sysInfo, setSysInfo] = useState<Record<string, string>>({})
  const [binaries, setBinaries] = useState<Record<string, any>>({})

  useEffect(() => {
    if (isOpen) {
      api.getVersion().then(v => { if (v) setAppVersion(v) }).catch(() => {})
      api.getSystemInfo().then(info => { if (info) setSysInfo(info) }).catch(() => {})
      api.checkCoreBinaries().then(b => { if (b) setBinaries(b) }).catch(() => {})
    }
  }, [isOpen])

  if (!isOpen) return null

  const handleOpenLink = (url: string) => {
    try {
      api.openBrowserURL(url)
    } catch {
      window.open(url, '_blank')
    }
  }

  const handleCheckUpdate = async () => {
    setIsCheckingUpdate(true)
    setUpdateResult(null)
    try {
      const result = await api.checkForUpdates() as UpdateResult
      setUpdateResult(result)
    } catch {
      setUpdateResult({
        has_update: false,
        latest_version: appVersion,
        current_version: appVersion,
        release_url: 'https://github.com/Rpg-420/NexVPN/releases',
        release_notes: '',
        error: 'Connection failed – check your internet',
      })
    } finally {
      setIsCheckingUpdate(false)
    }
  }

  return (
    <ModalShell
      isOpen={isOpen}
      onClose={onClose}
      maxWidth="max-w-lg"
      className="max-h-[90vh]"
    >
        {/* Modal Header */}
        <div className="flex items-center justify-between px-5 py-4 border-b border-border shrink-0">
          <div className="flex items-center gap-2.5 font-bold text-foreground text-sm">
            <div className="w-8 h-8 rounded-lg border border-border bg-muted flex items-center justify-center text-muted-foreground">
              <Shield size={16} />
            </div>
            <span>{t('about_title')}</span>
          </div>
          <button onClick={onClose} className="text-muted-foreground hover:text-foreground p-1 rounded-lg hover:bg-muted transition">
            <X size={18} />
          </button>
        </div>

        {/* Modal Body */}
        <div className="p-5 space-y-5 overflow-y-auto flex-1 custom-scrollbar">
          {/* App Brand Banner */}
          <div className="bg-card/90 flex flex-col items-center text-center p-4 rounded-2xl border border-border relative overflow-hidden">
            <div className="w-16 h-16 rounded-2xl overflow-hidden shadow-xl mb-3 ring-1 ring-border">
              <img src="/appicon.png" alt="NexVPN" className="w-full h-full object-cover" />
            </div>
            <h2 className="text-lg font-extrabold text-foreground flex items-center gap-2">
              <span>NexVPN Pro Client</span>
              <span className="badge badge-info text-[10px] font-mono font-bold">{appVersion ? `v${appVersion}` : '...'}</span>
            </h2>
            <p className="text-xs text-muted-foreground mt-1 max-w-sm leading-relaxed">
              {t('about_description')}
            </p>
            {/* Build metadata, read from the running binary via GetSystemInfo */}
            {(sysInfo.platform || sysInfo.arch || sysInfo.go_version) && (
              <div className="mt-2.5 flex flex-wrap items-center justify-center gap-1.5 text-[10px] font-mono text-muted-foreground">
                {sysInfo.platform && (
                  <span className="px-2 py-0.5 rounded bg-muted border border-border">
                    {sysInfo.platform}/{sysInfo.arch}
                  </span>
                )}
                {sysInfo.go_version && (
                  <span className="px-2 py-0.5 rounded bg-muted border border-border">
                    {sysInfo.go_version}
                  </span>
                )}
              </div>
            )}
          </div>

          {/* Core Engines Badge Grid */}
          <div className="space-y-2">
            <h3 className="text-xs font-bold text-muted-foreground flex items-center gap-1.5">
              <Cpu size={14} className="text-primary" />
              <span>{t('cores_status_title')}</span>
            </h3>

            <div className="grid grid-cols-2 sm:grid-cols-3 gap-2 text-center text-[11px] font-mono">
              <div className="p-2.5 rounded-xl bg-black/40 border border-border flex flex-col items-center gap-1">
                <span className="text-primary font-bold">Sing-Box</span>
                <span className="text-[10px] text-muted-foreground">{binaries.singbox?.version || 'v1.11.3 (TUN)'}</span>
              </div>
              <div className="p-2.5 rounded-xl bg-black/40 border border-border flex flex-col items-center gap-1">
                <span className="text-primary font-bold">Xray-Core</span>
                <span className="text-[10px] text-muted-foreground">{binaries.xray?.version || 'v1.8.24 (V2Ray)'}</span>
              </div>
              <div className="p-2.5 rounded-xl bg-black/40 border border-border flex flex-col items-center gap-1">
                <span className="text-warning font-bold">Psiphon Shield</span>
                <span className="text-[10px] text-muted-foreground">{binaries.psiphon?.version || 'v2.0.40 (Core)'}</span>
              </div>
              <div className="p-2.5 rounded-xl bg-black/40 border border-border flex flex-col items-center gap-1">
                <span className="text-success font-bold">Aether Engine</span>
                <span className="text-[10px] text-muted-foreground">{binaries.aether?.version || 'v1.7.0 (MASQUE)'}</span>
              </div>
              <div className="p-2.5 rounded-xl bg-black/40 border border-border flex flex-col items-center gap-1">
                <span className="text-warning font-bold">OpenVPN</span>
                <span className="text-[10px] text-muted-foreground">{binaries.openvpn?.version || 'v2.6.9 (TAP)'}</span>
              </div>
              <div className="p-2.5 rounded-xl bg-black/40 border border-border flex flex-col items-center gap-1">
                <span className="text-success font-bold">Wintun Adapter</span>
                <span className="text-[10px] text-muted-foreground">{binaries.wireguard?.version || 'v0.14.1 (Driver)'}</span>
              </div>
            </div>
          </div>

          {/* Official Community & Links */}
          <div className="space-y-2">
            <h3 className="text-xs font-bold text-muted-foreground flex items-center gap-1.5">
              <Sparkles size={14} className="text-warning" />
              <span>{t('support_feedback')}</span>
            </h3>

            <div className="grid grid-cols-1 sm:grid-cols-2 gap-2">
              <button
                type="button"
                onClick={() => handleOpenLink('https://t.me/nexvpnpro')}
                className="p-2.5 rounded-xl bg-card hover:bg-muted border border-border hover:border-primary/40 transition flex items-center justify-between text-xs text-foreground"
              >
                <div className="flex items-center gap-2">
                  <Send size={15} className="text-primary" />
                  <span className="font-semibold">{t('telegram_channel')}</span>
                </div>
                <ExternalLink size={13} className="text-muted-foreground" />
              </button>

              <button
                type="button"
                onClick={() => handleOpenLink('https://github.com/Rpg-420/NexVPN')}
                className="p-2.5 rounded-xl bg-card hover:bg-muted border border-border hover:border-primary/40 transition flex items-center justify-between text-xs text-foreground"
              >
                <div className="flex items-center gap-2">
                  <Github size={15} className="text-primary" />
                  <span className="font-semibold">{t('github_source')}</span>
                </div>
                <ExternalLink size={13} className="text-muted-foreground" />
              </button>

              <button
                type="button"
                onClick={() => handleOpenLink('https://github.com/Rpg-420/NexVPN/releases')}
                className="p-2.5 rounded-xl bg-card hover:bg-muted border border-border hover:border-success/40 transition flex items-center justify-between text-xs text-foreground"
              >
                <div className="flex items-center gap-2">
                  <Globe size={15} className="text-success" />
                  <span className="font-semibold">{t('official_website')}</span>
                </div>
                <ExternalLink size={13} className="text-muted-foreground" />
              </button>

              <button
                type="button"
                onClick={() => handleOpenLink('https://t.me/nexvpnpro')}
                className="p-2.5 rounded-xl bg-card hover:bg-muted border border-border hover:border-warning/40 transition flex items-center justify-between text-xs text-foreground"
              >
                <div className="flex items-center gap-2">
                  <MessageCircle size={15} className="text-warning" />
                  <span className="font-semibold">{t('support_feedback')}</span>
                </div>
                <ExternalLink size={13} className="text-muted-foreground" />
              </button>
            </div>
          </div>

          {/* Update Checker Button */}
          <div className="pt-2 border-t border-border flex flex-col items-center gap-3">
            <Button variant="secondary" size="sm" className="w-full flex items-center justify-center gap-2 text-xs font-bold text-foreground border-border hover:border-primary/40"
              type="button"
              onClick={handleCheckUpdate}
              disabled={isCheckingUpdate}
            >
              {isCheckingUpdate ? (
                <Loader2 size={13} className="animate-spin text-primary" />
              ) : (
                <RefreshCw size={13} />
              )}
              <span>{isCheckingUpdate ? t('checking_updates') : t('check_updates')}</span>
            </Button>

            {/* Update Result Panel */}
            {updateResult && (
              <div className={`w-full rounded-xl border p-3 text-xs space-y-2 animate-fade-in ${
                updateResult.error
                  ? 'bg-destructive/30 border-destructive/40 text-destructive'
                  : updateResult.has_update
                  ? 'bg-warning/30 border-warning/50 text-warning'
                  : 'bg-success/30 border-success/40 text-success'
              }`}>
                {updateResult.error ? (
                  <div className="flex items-center gap-2">
                    <AlertCircle size={14} className="text-destructive shrink-0" />
                    <span>{updateResult.error}</span>
                  </div>
                ) : updateResult.has_update ? (
                  <>
                    <div className="flex items-center justify-between">
                      <div className="flex items-center gap-2">
                        <ArrowDownToLine size={14} className="text-warning shrink-0" />
                        <span className="font-bold">{t('update_new_available')}</span>
                      </div>
                      <span className="font-mono text-[10px] bg-warning/20 px-2 py-0.5 rounded-full border border-warning/30">
                        v{updateResult.latest_version}
                      </span>
                    </div>
                    <div className="flex items-center justify-between text-[11px] text-warning/70">
                      <span>{t('current_version')}: v{updateResult.current_version}</span>
                    </div>
                    {updateResult.release_notes && (
                      <p className="text-[11px] text-muted-foreground leading-relaxed border-t border-warning/30 pt-2">
                        {updateResult.release_notes}
                      </p>
                    )}
                    <button
                      onClick={() => handleOpenLink(updateResult.release_url)}
                      className="flex items-center gap-1.5 px-3 py-1.5 bg-primary hover:bg-primary/90 text-primary-foreground rounded-lg text-[11px] font-bold transition-all w-full justify-center"
                    >
                      <ArrowDownToLine size={12} />
                      <span>{t('download_version')} v{updateResult.latest_version}</span>
                    </button>
                  </>
                ) : (
                  <div className="flex items-center gap-2">
                    <CheckCircle2 size={14} className="text-success shrink-0" />
                    <div>
                      <span className="font-bold">{t('update_latest_notice')}</span>
                      <div className="flex items-center gap-1 mt-0.5 text-[10px] text-success/70">
                        <Tag size={10} />
                        <span>v{updateResult.current_version}</span>
                      </div>
                    </div>
                  </div>
                )}
              </div>
            )}
          </div>

          {/* License & Copyright */}
          <p className="text-[10px] text-muted-foreground text-center leading-relaxed">
            {t('about_license')}
          </p>
        </div>
      </ModalShell>
  )
}
