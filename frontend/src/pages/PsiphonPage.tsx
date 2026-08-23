import React, { useState, useEffect } from 'react'
import {
  Shield,
  ShieldCheck,
  Globe,
  Radio,
  Zap,
  Activity,
  Server,
  Lock,
  RefreshCw,
  Power,
  CheckCircle2,
  AlertTriangle,
  Flame,
  Sparkles,
} from 'lucide-react'
import { api } from '../lib/api'
import { t, getLanguage } from '../lib/i18n'
import CountryFlag from '../components/CountryFlag'
import type { ConnectionStatus, TrafficStats } from '../types'

interface PsiphonPageProps {
  status: ConnectionStatus
  stats: TrafficStats
}

interface RegionItem {
  code: string
  nameEn: string
  nameFa: string
  flag: string
}

const PSIPHON_REGIONS: RegionItem[] = [
  { code: 'AUTO', nameEn: 'Auto (Fastest Server)', nameFa: 'انتخاب خودکار (سریع‌ترین نود)', flag: '⚡' },
  { code: 'US', nameEn: 'United States', nameFa: 'ایالات متحده آمریکا', flag: '🇺🇸' },
  { code: 'DE', nameEn: 'Germany', nameFa: 'آلمان', flag: '🇩🇪' },
  { code: 'GB', nameEn: 'United Kingdom', nameFa: 'انگلستان', flag: '🇬🇧' },
  { code: 'NL', nameEn: 'Netherlands', nameFa: 'هلند', flag: '🇳🇱' },
  { code: 'CA', nameEn: 'Canada', nameFa: 'کانادا', flag: '🇨🇦' },
  { code: 'FR', nameEn: 'France', nameFa: 'فرانسه', flag: '🇫🇷' },
  { code: 'JP', nameEn: 'Japan', nameFa: 'ژاپن', flag: '🇯🇵' },
  { code: 'SG', nameEn: 'Singapore', nameFa: 'سنگاپور', flag: '🇸🇬' },
  { code: 'CH', nameEn: 'Switzerland', nameFa: 'سوئیس', flag: '🇨🇭' },
  { code: 'IT', nameEn: 'Italy', nameFa: 'ایتالیا', flag: '🇮🇹' },
  { code: 'ES', nameEn: 'Spain', nameFa: 'اسپانیا', flag: '🇪🇸' },
  { code: 'SE', nameEn: 'Sweden', nameFa: 'سوئد', flag: '🇸🇪' },
  { code: 'AT', nameEn: 'Austria', nameFa: 'اتریش', flag: '🇦🇹' },
]

export default function PsiphonPage({ status, stats }: PsiphonPageProps) {
  const [selectedRegion, setSelectedRegion] = useState<string>('AUTO')
  const [isConnecting, setIsConnecting] = useState(false)
  const [noticeMessage, setNoticeMessage] = useState<string | null>(null)
  const [coreInstalled, setCoreInstalled] = useState(false)
  const [updatingCore, setUpdatingCore] = useState(false)
  const [downloadProgress, setDownloadProgress] = useState<{ percent: number; speed: number; status: string } | null>(null)
  const [showAdvanced, setShowAdvanced] = useState(false)
  const [tunnelProtocol, setTunnelProtocol] = useState<string>('SSH')
  const [socksPort, setSocksPort] = useState(10808)
  const [httpPort, setHttpPort] = useState(10809)
  const [upstreamProxy, setUpstreamProxy] = useState('')
  const [splitTunnelIran, setSplitTunnelIran] = useState(true)

  const isLangFa = getLanguage() === 'fa'

  const isPsiphonActive =
    status.state === 'connected' &&
    (status.protocol === 'psiphon' || (status.profile_name && status.profile_name.toLowerCase().includes('psiphon')))

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
      await api.connectPsiphon(region)
      setNoticeMessage(region === 'AUTO' ? (isLangFa ? 'در حال اتصال به نود ابری بهینه سایفون...' : 'Connecting to optimal Psiphon cloud node...') : `Connecting to Psiphon in ${region}...`)
    } catch (err: any) {
      setNoticeMessage(err?.message || 'Failed to initialize Psiphon engine')
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
      {/* Header Banner */}
      <div className="relative overflow-hidden rounded-2xl bg-gradient-to-r from-red-950/40 via-amber-950/30 to-purple-950/40 border border-amber-500/20 p-5 md:p-6 backdrop-blur-xl shadow-2xl">
        <div className="absolute -right-16 -top-16 w-64 h-64 bg-amber-500/10 rounded-full blur-3xl pointer-events-none" />
        <div className="absolute -left-16 -bottom-16 w-64 h-64 bg-red-500/10 rounded-full blur-3xl pointer-events-none" />

        <div className="relative z-10 flex flex-col md:flex-row md:items-center justify-between gap-5">
          <div className="space-y-1.5 min-w-0">
            <div className="inline-flex items-center gap-1.5 px-2.5 py-0.5 rounded-full bg-amber-500/10 border border-amber-500/30 text-amber-300 text-[11px] font-semibold tracking-wide">
              <Flame className="w-3.5 h-3.5 text-amber-400 animate-pulse" />
              <span>{t('psiphon_title')}</span>
            </div>
            <h1 className="text-xl md:text-2xl font-black text-white tracking-tight flex items-center gap-2.5">
              <Shield className="w-7 h-7 text-amber-400 shrink-0" />
              <span>{t('psiphon_title')}</span>
              <span className="text-xs font-mono font-medium px-2 py-0.5 rounded bg-amber-500/10 text-amber-300 border border-amber-500/20">Psiphon Shield</span>
            </h1>
            <p className="text-xs md:text-sm text-slate-300 max-w-2xl leading-relaxed">
              {t('psiphon_subtitle')}
            </p>
          </div>

          <div className="flex items-center gap-3 shrink-0">
            {isPsiphonActive ? (
              <button
                type="button"
                onClick={handleDisconnect}
                className="w-full sm:w-auto px-5 py-2.5 rounded-xl bg-red-500/20 hover:bg-red-500/30 text-red-300 border border-red-500/40 text-xs font-bold transition-all flex items-center justify-center gap-2 shadow-lg shadow-red-950/50 hover:scale-105 active:scale-95"
              >
                <Power className="w-4 h-4 text-red-400" />
                <span>{t('psiphon_disconnect_btn')}</span>
              </button>
            ) : (
              <button
                type="button"
                onClick={() => handleConnect(selectedRegion)}
                disabled={isConnecting}
                className="w-full sm:w-auto px-6 py-2.5 rounded-xl bg-gradient-to-r from-amber-500 to-orange-500 hover:from-amber-400 hover:to-orange-400 text-slate-950 text-xs font-extrabold transition-all flex items-center justify-center gap-2 shadow-lg shadow-amber-500/20 hover:scale-105 active:scale-95"
              >
                {isConnecting ? (
                  <>
                    <RefreshCw className="w-4 h-4 animate-spin" />
                    <span>{t('connecting')}</span>
                  </>
                ) : (
                  <>
                    <Zap className="w-4 h-4 text-slate-950" />
                    <span>{t('psiphon_quick_connect')}</span>
                  </>
                )}
              </button>
            )}
          </div>
        </div>
      </div>

      {/* Core Missing Warning & Download Banner */}
      {!coreInstalled && (
        <div className="p-4 rounded-xl border border-amber-500/30 bg-amber-500/10 flex flex-col sm:flex-row items-start sm:items-center justify-between gap-4">
          <div className="flex items-center gap-3">
            <AlertTriangle className="w-6 h-6 text-amber-400 shrink-0" />
            <div>
              <h3 className="font-bold text-white text-sm">{t('psiphon_core_missing_title')}</h3>
              <p className="text-xs text-amber-200/80 mt-0.5">
                {t('psiphon_core_missing_desc')}
              </p>
            </div>
          </div>
          <button
            onClick={handleInstallCore}
            disabled={updatingCore}
            className="flex items-center gap-2 px-5 py-2.5 bg-amber-500 hover:bg-amber-400 disabled:opacity-50 text-slate-950 font-bold rounded-xl text-xs transition-all shadow-md shrink-0"
          >
            <RefreshCw className={`w-3.5 h-3.5 ${updatingCore ? 'animate-spin' : ''}`} />
            <span>{updatingCore ? (downloadProgress?.status || t('psiphon_downloading')) : t('psiphon_download_btn')}</span>
          </button>
        </div>
      )}

      {/* Live Download Progress Bar */}
      {updatingCore && downloadProgress && (
        <div className="p-4 bg-amber-950/20 border border-amber-500/20 rounded-xl space-y-2 animate-in fade-in duration-200">
          <div className="flex items-center justify-between text-xs text-amber-200">
            <span className="flex items-center gap-2 font-medium">
              <RefreshCw className="w-3.5 h-3.5 text-amber-400 animate-spin" />
              <span>{downloadProgress.status} ({downloadProgress.percent}%)</span>
            </span>
            {downloadProgress.speed > 0 && (
              <span className="font-mono text-gray-400 text-[11px]">
                {(downloadProgress.speed / (1024 * 1024)).toFixed(2)} MB/s
              </span>
            )}
          </div>
          <div className="w-full h-2 bg-gray-800 rounded-full overflow-hidden">
            <div
              className="h-full bg-gradient-to-r from-amber-500 to-orange-400 transition-all duration-200"
              style={{ width: `${downloadProgress.percent}%` }}
            />
          </div>
        </div>
      )}

      {noticeMessage && (
        <div className="p-4 rounded-xl bg-amber-500/10 border border-amber-500/30 text-amber-200 text-sm flex items-center gap-3 animate-slideDown">
          <AlertTriangle className="w-5 h-5 text-amber-400 shrink-0" />
          <span>{noticeMessage}</span>
        </div>
      )}

      {/* Connection State Info Card */}
      {isPsiphonActive && (
        <div className="grid grid-cols-1 md:grid-cols-4 gap-4 p-5 rounded-2xl bg-slate-900/60 border border-emerald-500/30 backdrop-blur-md shadow-xl">
          <div className="flex items-center gap-3">
            <div className="w-10 h-10 rounded-xl bg-emerald-500/20 border border-emerald-500/30 flex items-center justify-center">
              <CheckCircle2 className="w-5 h-5 text-emerald-400" />
            </div>
            <div>
              <div className="text-xs text-slate-400">{t('psiphon_connection_status')}</div>
              <div className="text-sm font-bold text-emerald-300">{t('psiphon_connected_desc')}</div>
            </div>
          </div>

          <div className="flex items-center gap-3">
            <div className="w-10 h-10 rounded-xl bg-cyan-500/20 border border-cyan-500/30 flex items-center justify-center">
              <Globe className="w-5 h-5 text-cyan-400" />
            </div>
            <div>
              <div className="text-xs text-slate-400">{t('psiphon_server_country')}</div>
              <div className="text-sm font-bold text-white">{status.profile_name || 'Global Cloud'}</div>
            </div>
          </div>

          <div className="flex items-center gap-3">
            <div className="w-10 h-10 rounded-xl bg-purple-500/20 border border-purple-500/30 flex items-center justify-center">
              <Activity className="w-5 h-5 text-purple-400" />
            </div>
            <div>
              <div className="text-xs text-slate-400">{t('psiphon_download_speed')}</div>
              <div className="text-sm font-bold text-white">{(stats.speed_download / 1024).toFixed(1)} KB/s</div>
            </div>
          </div>

          <div className="flex items-center gap-3">
            <div className="w-10 h-10 rounded-xl bg-amber-500/20 border border-amber-500/30 flex items-center justify-center">
              <Lock className="w-5 h-5 text-amber-400" />
            </div>
            <div>
              <div className="text-xs text-slate-400">{t('psiphon_enc_tunnel')}</div>
              <div className="text-sm font-bold text-white">SSH + Fronted Meek</div>
            </div>
          </div>
        </div>
      )}

      {/* Region Selector Grid */}
      <div className="space-y-4">
        <div className="flex items-center justify-between">
          <div className="flex items-center gap-2">
            <Globe className="w-5 h-5 text-amber-400" />
            <h2 className="text-lg font-bold text-white">{t('psiphon_select_region_title')}</h2>
          </div>
          <span className="text-xs text-slate-400">{t('psiphon_select_region_hint')}</span>
        </div>

        <div className="grid grid-cols-2 sm:grid-cols-3 md:grid-cols-4 lg:grid-cols-7 gap-2.5">
          {PSIPHON_REGIONS.map(reg => {
            const isSelected = selectedRegion === reg.code
            const regName = isLangFa ? reg.nameFa : reg.nameEn
            return (
              <button
                key={reg.code}
                type="button"
                onClick={() => {
                  setSelectedRegion(reg.code)
                  if (isPsiphonActive) {
                    handleConnect(reg.code)
                  }
                }}
                className={`group relative p-3 rounded-xl border text-center transition-all flex flex-col items-center justify-center gap-1.5 ${
                  isSelected
                    ? 'bg-amber-500/20 border-amber-400 text-white ring-2 ring-amber-400/30 shadow-lg shadow-amber-500/10'
                    : 'bg-slate-900/40 border-slate-800 text-slate-300 hover:bg-slate-800/60 hover:border-slate-700 hover:text-white'
                }`}
              >
                {reg.code === 'AUTO' ? (
                  <div className="w-8 h-8 rounded-full bg-amber-500/20 text-amber-400 flex items-center justify-center text-sm font-black shadow-inner">
                    ⚡
                  </div>
                ) : (
                  <div className="w-8 h-8 rounded-full overflow-hidden shadow-sm flex items-center justify-center ring-1 ring-white/10 bg-slate-800 shrink-0">
                    <CountryFlag codeOrName={reg.code} size="md" />
                  </div>
                )}
                <div className="text-xs font-bold leading-tight truncate w-full">{regName}</div>
                <div className="text-[10px] text-slate-500 font-mono">{reg.code}</div>
              </button>
            )
          })}
        </div>
      </div>

      {/* Advanced Settings Drawer */}
      <div className="rounded-2xl bg-slate-900/60 border border-slate-800 p-5 space-y-4">
        <div className="flex items-center justify-between">
          <div className="flex items-center gap-2">
            <Radio className="w-5 h-5 text-amber-400" />
            <h3 className="text-sm font-bold text-white">{t('psiphon_adv_settings_title')}</h3>
          </div>
          <button
            onClick={() => setShowAdvanced(!showAdvanced)}
            className="text-xs text-amber-400 hover:text-amber-300 font-semibold transition-all px-3 py-1 bg-amber-500/10 rounded-lg border border-amber-500/20"
          >
            {showAdvanced ? t('psiphon_close_settings') : t('psiphon_show_settings')}
          </button>
        </div>

        {showAdvanced && (
          <div className="space-y-4 pt-3 border-t border-slate-800 animate-fadeIn">
            <div className="grid grid-cols-1 md:grid-cols-2 gap-4">
              <div className="space-y-1.5">
                <label className="text-xs font-semibold text-slate-300">{t('psiphon_tunnel_protocol_label')}</label>
                <select
                  value={tunnelProtocol}
                  onChange={e => setTunnelProtocol(e.target.value)}
                  className="w-full px-3 py-2 bg-slate-950 border border-slate-700 rounded-xl text-xs text-white focus:outline-none focus:border-amber-400"
                >
                  <option value="SSH">SSH Obfuscated (Recommended / Standard)</option>
                  <option value="OSSH">OSSH (OpenSSH Layered Obfuscation)</option>
                  <option value="FRONTED-MEEK-OSSH">FRONTED-MEEK-OSSH (Domain Fronting)</option>
                  <option value="UNFRONTED-MEEK-OSSH">UNFRONTED-MEEK-OSSH (Direct Meek)</option>
                  <option value="QUIC">QUIC (High-Speed UDP)</option>
                </select>
              </div>

              <div className="space-y-1.5">
                <label className="text-xs font-semibold text-slate-300">{t('psiphon_upstream_proxy_label')}</label>
                <input
                  type="text"
                  value={upstreamProxy}
                  onChange={e => setUpstreamProxy(e.target.value)}
                  placeholder="e.g. socks5://127.0.0.1:10808"
                  className="w-full px-3 py-2 bg-slate-950 border border-slate-700 rounded-xl text-xs text-white focus:outline-none focus:border-amber-400"
                />
              </div>
            </div>

            <div className="grid grid-cols-2 gap-4">
              <div className="space-y-1.5">
                <label className="text-xs font-semibold text-slate-300">{t('psiphon_socks_port_label')}</label>
                <input
                  type="number"
                  value={socksPort}
                  onChange={e => setSocksPort(Number(e.target.value))}
                  className="w-full px-3 py-2 bg-slate-950 border border-slate-700 rounded-xl text-xs text-white focus:outline-none focus:border-amber-400"
                />
              </div>
              <div className="space-y-1.5">
                <label className="text-xs font-semibold text-slate-300">{t('psiphon_http_port_label')}</label>
                <input
                  type="number"
                  value={httpPort}
                  onChange={e => setHttpPort(Number(e.target.value))}
                  className="w-full px-3 py-2 bg-slate-950 border border-slate-700 rounded-xl text-xs text-white focus:outline-none focus:border-amber-400"
                />
              </div>
            </div>

            <div className="flex items-center gap-2 pt-1">
              <input
                type="checkbox"
                id="splitIran"
                checked={splitTunnelIran}
                onChange={e => setSplitTunnelIran(e.target.checked)}
                className="w-4 h-4 rounded bg-slate-950 border-slate-700 text-amber-500 focus:ring-0"
              />
              <label htmlFor="splitIran" className="text-xs text-slate-300 select-none cursor-pointer">
                {t('psiphon_split_iran_label')}
              </label>
            </div>
          </div>
        )}
      </div>

      {/* Technical Highlights / Why Psiphon */}
      <div className="grid grid-cols-1 md:grid-cols-3 gap-5 pt-4">
        <div className="p-5 rounded-xl bg-slate-900/40 border border-slate-800 space-y-2">
          <div className="w-9 h-9 rounded-lg bg-amber-500/10 border border-amber-500/20 flex items-center justify-center">
            <Lock className="w-5 h-5 text-amber-400" />
          </div>
          <h3 className="text-sm font-bold text-white">{t('psiphon_feature_1_title')}</h3>
          <p className="text-xs text-slate-400 leading-relaxed">
            {t('psiphon_feature_1_desc')}
          </p>
        </div>

        <div className="p-5 rounded-xl bg-slate-900/40 border border-slate-800 space-y-2">
          <div className="w-9 h-9 rounded-lg bg-cyan-500/10 border border-cyan-500/20 flex items-center justify-center">
            <Sparkles className="w-5 h-5 text-cyan-400" />
          </div>
          <h3 className="text-sm font-bold text-white">{t('psiphon_feature_2_title')}</h3>
          <p className="text-xs text-slate-400 leading-relaxed">
            {t('psiphon_feature_2_desc')}
          </p>
        </div>

        <div className="p-5 rounded-xl bg-slate-900/40 border border-slate-800 space-y-2">
          <div className="w-9 h-9 rounded-lg bg-purple-500/10 border border-purple-500/20 flex items-center justify-center">
            <ShieldCheck className="w-5 h-5 text-purple-400" />
          </div>
          <h3 className="text-sm font-bold text-white">{t('psiphon_feature_3_title')}</h3>
          <p className="text-xs text-slate-400 leading-relaxed">
            {t('psiphon_feature_3_desc')}
          </p>
        </div>
      </div>
    </div>
  )
}
