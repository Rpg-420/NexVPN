import { useState, useEffect } from 'react'
import {
  Shield,
  ShieldCheck,
  Globe,
  Radio,
  Zap,
  Activity,
  Lock,
  RefreshCw,
  Power,
  CheckCircle2,
  AlertTriangle,
  Sparkles,
} from 'lucide-react'
import { api } from '../lib/api'
import { t } from '../lib/i18n'
import { cn } from '@/lib/utils'
import { Alert, AlertDescription, AlertTitle } from '@/components/ui/alert'
import { Button } from '@/components/ui/button'
import { Card } from '@/components/ui/card'
import { Checkbox } from '@/components/ui/checkbox'
import { Input } from '@/components/ui/input'
import { Label } from '@/components/ui/label'
import { Progress } from '@/components/ui/progress'
import CountryFlag from '../components/CountryFlag'
import type { ConnectionStatus, TrafficStats } from '../types'

interface PsiphonPageProps {
  status: ConnectionStatus
  stats: TrafficStats
}

/**
 * Fallback region list, used only until GetPsiphonRegions answers. The backend
 * is the single source of truth: this list must be a subset of
 * psiphonSupportedRegions in app_extra.go, and the page replaces it as soon as
 * the real list arrives.
 */
const PSIPHON_REGIONS_FALLBACK: string[] = [
  'AUTO', 'US', 'DE', 'GB', 'NL', 'CA', 'FR', 'JP', 'SG', 'AT', 'CH', 'IT', 'ES', 'PL', 'SE', 'RO', 'BE', 'IN',
]

/** Maps a region code to its i18n key, falling back to the raw code. */
function regionLabelKey(code: string): string {
  return code === 'AUTO' ? 'psiphon_region_auto' : `country_${code.toLowerCase()}`
}

/**
 * Describes the tunnel protocol the engine is actually configured with.
 * Previously this tile was the hard-coded string "SSH + Fronted Meek", which
 * was wrong whenever the user selected any other protocol.
 */
function describeTunnelProtocol(protocol: string): string {
  switch (protocol) {
    case 'OSSH':
      return 'OSSH (OpenSSH obfuscation)'
    case 'FRONTED-MEEK-OSSH':
      return 'Fronted Meek + OSSH (domain fronting)'
    case 'UNFRONTED-MEEK-OSSH':
      return 'Unfronted Meek + OSSH (direct)'
    case 'QUIC':
      return 'QUIC (UDP transport)'
    default:
      return 'SSH (obfuscated)'
  }
}

export default function PsiphonPage({ status, stats }: PsiphonPageProps) {
  const [selectedRegion, setSelectedRegion] = useState<string>('AUTO')
  const [regions, setRegions] = useState<string[]>(PSIPHON_REGIONS_FALLBACK)
  const [isConnecting, setIsConnecting] = useState(false)
  const [noticeMessage, setNoticeMessage] = useState<string | null>(null)
  const [coreInstalled, setCoreInstalled] = useState(false)
  const [updatingCore, setUpdatingCore] = useState(false)
  const [downloadProgress, setDownloadProgress] = useState<{ percent: number; speed: number; status: string } | null>(null)
  const [showAdvanced, setShowAdvanced] = useState(false)
  const [tunnelProtocol, setTunnelProtocol] = useState<string>('SSH')
  const [socksPort, setSocksPort] = useState(10808)
  const [httpPort, setHttpPort] = useState(10809)
  const [disableSplit, setDisableSplit] = useState(false)
  // True once the backend has answered GetPsiphonOptions. The advanced panel
  // shows the persisted values only after this, so it never briefly displays
  // local defaults that are not what the engine will run.
  const [optionsLoaded, setOptionsLoaded] = useState(false)

  const isPsiphonActive =
    status.state === 'connected' &&
    (status.protocol === 'psiphon' ||
      Boolean(status.profile_name && status.profile_name.toLowerCase().includes('psiphon')))

  const checkCoreStatus = async () => {
    try {
      const b = await api.checkCoreBinaries()
      if (b && b.psiphon && b.psiphon.status === 'installed') {
        setCoreInstalled(true)
      } else {
        setCoreInstalled(false)
      }
    } catch (err) {
      console.error('Failed to check Psiphon core status:', err)
    }
  }

  useEffect(() => {
    checkCoreStatus()

    // The region list and the persisted engine options both come from the
    // backend, so the UI can never drift from what the engine supports.
    void (async () => {
      try {
        const supported = await api.getPsiphonRegions()
        if (Array.isArray(supported) && supported.length > 0) {
          setRegions(supported)
        }
      } catch (err) {
        console.error('Failed to load Psiphon regions:', err)
      }
      try {
        const opts = await api.getPsiphonOptions()
        if (opts) {
          if (opts.tunnel_protocol) setTunnelProtocol(opts.tunnel_protocol)
          if (opts.socks_port) setSocksPort(opts.socks_port)
          if (opts.http_port) setHttpPort(opts.http_port)
          setDisableSplit(Boolean(opts.disable_split))
        }
      } catch (err) {
        console.error('Failed to load Psiphon options:', err)
      } finally {
        setOptionsLoaded(true)
      }
    })()

    // Listen for core download progress
    const win = window as any
    if (win.runtime && win.runtime.EventsOn) {
      const unsub = win.runtime.EventsOn('core:download_progress', (data: any) => {
        if (data && data.key === 'psiphon') {
          setDownloadProgress({
            percent: data.percent || 0,
            speed: data.speed || 0,
            status: data.status || 'Downloading...',
          })
          if (data.percent >= 100) {
            setTimeout(() => {
              setDownloadProgress(null)
              checkCoreStatus()
            }, 1000)
          }
        }
      })
      return () => {
        if (typeof unsub === 'function') unsub()
      }
    }
  }, [])

  const handleInstallCore = async () => {
    setUpdatingCore(true)
    setDownloadProgress({ percent: 1, speed: 0, status: 'Connecting...' })
    try {
      await api.updateCoreEngine('psiphon')
      await checkCoreStatus()
    } catch (err) {
      console.error('Failed to download Psiphon core:', err)
    } finally {
      setUpdatingCore(false)
      setDownloadProgress(null)
    }
  }

  const handleConnect = async (region: string = selectedRegion) => {
    setIsConnecting(true)
    setNoticeMessage(null)
    try {
      if (status.state === 'connected') {
        await api.disconnect()
      }
      // Pass the advanced options through, so the engine runs with exactly what
      // the panel shows rather than its own hard-coded defaults.
      await api.connectPsiphonWithOptions(region, {
        tunnel_protocol: tunnelProtocol,
        socks_port: socksPort,
        http_port: httpPort,
        disable_split: disableSplit,
      })
      setNoticeMessage(region === 'AUTO' ? t('psiphon_connecting_auto') : t('psiphon_connecting_region', { region }))
    } catch (err: unknown) {
      setNoticeMessage(err instanceof Error ? err.message : 'Failed to initialize Psiphon engine')
    } finally {
      setIsConnecting(false)
    }
  }

  const handleDisconnect = async () => {
    try {
      await api.disconnect()
    } catch (err) {
      console.error(err)
    }
  }

  return (
    <div className="space-y-6 animate-fadeIn pb-12">
      {/* Header */}
      <div className="page-header flex items-center justify-between flex-wrap gap-3">
        <div>
          <h1 className="page-title flex items-center gap-2">
            <Shield size={22} className="text-primary" />
            {t('psiphon_title')}
            <span className="rounded border border-border bg-muted px-1.5 py-0.5 font-mono text-[11px] font-medium text-muted-foreground">
              Psiphon Shield
            </span>
          </h1>
          <p className="page-subtitle">{t('psiphon_subtitle')}</p>
        </div>

        <div className="flex items-center gap-2 shrink-0">
          {isPsiphonActive ? (
            <Button
              type="button"
              variant="outline"
              size="sm"
              onClick={handleDisconnect}
              className="border-destructive/30 text-destructive hover:bg-destructive/10 hover:text-destructive"
            >
              <Power className="h-4 w-4" />
              {t('psiphon_disconnect_btn')}
            </Button>
          ) : (
            <Button
              type="button"
              size="sm"
              onClick={() => handleConnect(selectedRegion)}
              disabled={isConnecting}
            >
              {isConnecting ? (
                <>
                  <RefreshCw className="h-4 w-4 animate-spin" />
                  {t('connecting')}
                </>
              ) : (
                <>
                  <Zap className="h-4 w-4" />
                  {t('psiphon_quick_connect')}
                </>
              )}
            </Button>
          )}
        </div>
      </div>

      {/* Core Missing Warning & Download Banner */}
      {!coreInstalled && (
        <Alert variant="warning" className="flex flex-col items-start justify-between gap-4 sm:flex-row sm:items-center">
          <AlertTriangle className="text-warning" />
          <div className="w-full">
            <AlertTitle>{t('psiphon_core_missing_title')}</AlertTitle>
            <AlertDescription>{t('psiphon_core_missing_desc')}</AlertDescription>
          </div>
          <Button
            size="sm"
            onClick={handleInstallCore}
            disabled={updatingCore}
            className="shrink-0"
          >
            <RefreshCw className={cn('h-3.5 w-3.5', updatingCore && 'animate-spin')} />
            {updatingCore ? (downloadProgress?.status || t('psiphon_downloading')) : t('psiphon_download_btn')}
          </Button>
        </Alert>
      )}

      {/* Live Download Progress Bar */}
      {updatingCore && downloadProgress && (
        <div className="animate-in fade-in space-y-2 rounded-lg border border-border bg-card p-4 duration-200">
          <div className="flex items-center justify-between text-xs">
            <span className="flex items-center gap-2 font-medium text-foreground">
              <RefreshCw className="h-3.5 w-3.5 animate-spin text-muted-foreground" />
              <span>
                {downloadProgress.status} ({downloadProgress.percent}%)
              </span>
            </span>
            {downloadProgress.speed > 0 && (
              <span className="font-mono text-[11px] text-muted-foreground">
                {(downloadProgress.speed / (1024 * 1024)).toFixed(2)} MB/s
              </span>
            )}
          </div>
          <Progress value={downloadProgress.percent} />
        </div>
      )}

      {noticeMessage && (
        <Alert variant="warning" className="animate-slideDown flex items-center gap-3">
          <AlertTriangle className="text-warning" />
          <span>{noticeMessage}</span>
        </Alert>
      )}

      {/* Connection State Info Card */}
      {isPsiphonActive && (
        <div className="grid grid-cols-1 md:grid-cols-4 gap-4 rounded-xl border border-border bg-card p-5 shadow-xs">
          <div className="flex items-center gap-3">
            <div className="flex h-10 w-10 items-center justify-center rounded-lg border border-border bg-muted">
              <CheckCircle2 className="h-5 w-5 text-muted-foreground" />
            </div>
            <div>
              <div className="text-xs text-muted-foreground">{t('psiphon_connection_status')}</div>
              <div className="text-sm font-semibold text-success">{t('psiphon_connected_desc')}</div>
            </div>
          </div>

          <div className="flex items-center gap-3">
            <div className="flex h-10 w-10 items-center justify-center rounded-lg border border-border bg-muted">
              <Globe className="h-5 w-5 text-muted-foreground" />
            </div>
            <div>
              <div className="text-xs text-muted-foreground">{t('psiphon_server_country')}</div>
              <div className="text-sm font-semibold text-foreground">{status.profile_name || 'Global Cloud'}</div>
            </div>
          </div>

          <div className="flex items-center gap-3">
            <div className="flex h-10 w-10 items-center justify-center rounded-lg border border-border bg-muted">
              <Activity className="h-5 w-5 text-muted-foreground" />
            </div>
            <div>
              <div className="text-xs text-muted-foreground">{t('psiphon_download_speed')}</div>
              <div className="text-sm font-semibold text-foreground">{(stats.speed_download / 1024).toFixed(1)} KB/s</div>
            </div>
          </div>

          <div className="flex items-center gap-3">
            <div className="flex h-10 w-10 items-center justify-center rounded-lg border border-border bg-muted">
              <Lock className="h-5 w-5 text-muted-foreground" />
            </div>
            <div>
              <div className="text-xs text-muted-foreground">{t('psiphon_enc_tunnel')}</div>
              <div className="text-sm font-semibold text-foreground">{describeTunnelProtocol(tunnelProtocol)}</div>
            </div>
          </div>
        </div>
      )}

      {/* Region Selector Grid */}
      <div className="space-y-4">
        <div className="flex items-center justify-between">
          <div className="flex items-center gap-2">
            <Globe className="h-5 w-5 text-muted-foreground" />
            <h2 className="text-lg font-semibold text-foreground">{t('psiphon_select_region_title')}</h2>
          </div>
          <span className="text-xs text-muted-foreground">{t('psiphon_select_region_hint')}</span>
        </div>

        <div className="grid grid-cols-2 sm:grid-cols-3 md:grid-cols-4 lg:grid-cols-7 gap-2.5">
          {regions.map(code => {
            const isSelected = selectedRegion === code
            const regName = t(regionLabelKey(code))
            return (
              <button
                key={code}
                type="button"
                aria-pressed={isSelected}
                onClick={() => {
                  setSelectedRegion(code)
                  if (isPsiphonActive) {
                    handleConnect(code)
                  }
                }}
                className={`group relative flex flex-col items-center justify-center gap-2 rounded-xl border p-3 text-center transition-colors ${
                  isSelected
                    ? 'border-primary bg-primary/10 font-semibold text-foreground'
                    : 'border-border bg-card text-foreground hover:bg-accent'
                }`}
              >
                {code === 'AUTO' ? (
                  <div className="flex h-5.5 w-8 items-center justify-center rounded-[4px] border border-primary/30 bg-primary/10 text-primary">
                    <Zap size={14} />
                  </div>
                ) : (
                  <CountryFlag codeOrName={code} size="md" />
                )}
                <div className="w-full truncate text-xs font-semibold leading-tight text-foreground">{regName}</div>
                <div className="font-mono text-[10px] tracking-wider text-muted-foreground">{code}</div>
              </button>
            )
          })}
        </div>
      </div>

      {/* Advanced Settings Drawer */}
      <Card className="space-y-4 p-5">
        <div className="flex items-center justify-between">
          <div className="flex items-center gap-2">
            <Radio className="h-5 w-5 text-muted-foreground" />
            <h3 className="text-sm font-semibold text-foreground">{t('psiphon_adv_settings_title')}</h3>
          </div>
          <Button variant="outline" size="sm" onClick={() => setShowAdvanced(!showAdvanced)}>
            {showAdvanced ? t('psiphon_close_settings') : t('psiphon_show_settings')}
          </Button>
        </div>

        {showAdvanced && (
          <div className="animate-fadeIn space-y-4 border-t border-border pt-3">
            <p className="text-[11px] leading-relaxed text-muted-foreground">
              {optionsLoaded ? t('psiphon_adv_settings_desc') : t('psiphon_adv_settings_loading')}
            </p>

            <div className="space-y-1.5">
              <Label htmlFor="psiphon-tunnel">{t('psiphon_tunnel_protocol_label')}</Label>
              <select
                id="psiphon-tunnel"
                value={tunnelProtocol}
                onChange={e => setTunnelProtocol(e.target.value)}
                disabled={isPsiphonActive}
                className="flex h-9 w-full rounded-md border border-input bg-background px-3 py-1 text-xs text-foreground shadow-xs transition-colors focus-visible:outline-none focus-visible:ring-1 focus-visible:ring-ring disabled:cursor-not-allowed disabled:opacity-60"
              >
                <option value="SSH">SSH Obfuscated (Recommended / Standard)</option>
                <option value="OSSH">OSSH (OpenSSH Layered Obfuscation)</option>
                <option value="FRONTED-MEEK-OSSH">FRONTED-MEEK-OSSH (Domain Fronting)</option>
                <option value="UNFRONTED-MEEK-OSSH">UNFRONTED-MEEK-OSSH (Direct Meek)</option>
                <option value="QUIC">QUIC (High-Speed UDP)</option>
              </select>
            </div>

            <div className="grid grid-cols-2 gap-4">
              <div className="space-y-1.5">
                <Label htmlFor="psiphon-socks">{t('psiphon_socks_port_label')}</Label>
                <Input
                  id="psiphon-socks"
                  type="number"
                  min={1024}
                  max={65535}
                  value={socksPort}
                  onChange={e => setSocksPort(Number(e.target.value))}
                  disabled={isPsiphonActive}
                  className="h-9 text-xs"
                />
              </div>
              <div className="space-y-1.5">
                <Label htmlFor="psiphon-http">{t('psiphon_http_port_label')}</Label>
                <Input
                  id="psiphon-http"
                  type="number"
                  min={1024}
                  max={65535}
                  value={httpPort}
                  onChange={e => setHttpPort(Number(e.target.value))}
                  disabled={isPsiphonActive}
                  className="h-9 text-xs"
                />
              </div>
            </div>

            <div className="flex items-center gap-2 pt-1">
              <Checkbox
                id="disableSplit"
                checked={disableSplit}
                onChange={e => setDisableSplit(e.target.checked)}
                disabled={isPsiphonActive}
              />
              <Label htmlFor="disableSplit" className="cursor-pointer select-none text-xs font-medium">
                {t('psiphon_disable_split_label')}
              </Label>
            </div>

            {isPsiphonActive && (
              <p className="text-[11px] leading-relaxed text-muted-foreground">
                {t('psiphon_adv_locked_hint')}
              </p>
            )}
          </div>
        )}
      </Card>

      {/* Technical Highlights / Why Psiphon */}
      <div className="grid grid-cols-1 gap-5 pt-4 md:grid-cols-3">
        {[
          { icon: Lock, title: t('psiphon_feature_1_title'), desc: t('psiphon_feature_1_desc') },
          { icon: Sparkles, title: t('psiphon_feature_2_title'), desc: t('psiphon_feature_2_desc') },
          { icon: ShieldCheck, title: t('psiphon_feature_3_title'), desc: t('psiphon_feature_3_desc') },
        ].map(({ icon: Icon, title, desc }) => (
          <Card key={title} className="space-y-2.5 p-5">
            <div className="flex h-9 w-9 items-center justify-center rounded-lg border border-border bg-muted">
              <Icon className="h-5 w-5 text-muted-foreground" />
            </div>
            <h3 className="text-sm font-semibold text-foreground">{title}</h3>
            <p className="text-xs leading-relaxed text-muted-foreground">{desc}</p>
          </Card>
        ))}
      </div>
    </div>
  )
}
