import React, { useState, useEffect, useRef, useCallback, useMemo } from 'react'
import {
  Power,
  Shield,
  ShieldCheck,
  ShieldAlert,
  ArrowDown,
  ArrowUp,
  Clock,
  Globe,
  Zap,
  Activity,
  Server,
  Radio,
  Sparkles,
  Layers,
  MapPin,
  Cpu,
  Gauge,
  CheckCircle2,
  RefreshCw,
  Wifi,
  Wrench,
  PieChart,
  Cloud,
  Copy,
  Check,
  QrCode,
  Gamepad2,
  TrendingUp,
  Smartphone,
  Share2,
  X,
  History,
  Archive,
  Link2,
  AlertTriangle,
  Search,
  SortAsc,
  Scan,
  ArrowRightLeft,
} from 'lucide-react'
import { AreaChart, Area, XAxis, YAxis, Tooltip, ResponsiveContainer } from 'recharts'
import { api } from '../lib/api'
import { t } from '../lib/i18n'
import { sound } from '../lib/sound'
import { sendDesktopNotification, requestNotificationPermission } from '../lib/notification'
import { generateQRCodeSVG } from '../lib/qrcode'
import type { ConnectionStatus, TrafficStats, Settings, IPInfo, Profile } from '../types'
import {
  formatBytes,
  formatSpeed,
  formatDuration,
  getProtocolName,
  getProtocolEmoji,
  getCountryFlag,
} from '../lib/utils'
import CountryFlag from '../components/CountryFlag'
import SpeedTestModal from '../components/SpeedTestModal'
import LANShareModal from '../components/LANShareModal'
import RadarModal from '../components/RadarModal'
import EmergencyRepairModal from '../components/EmergencyRepairModal'
import QuotaManagerModal from '../components/QuotaManagerModal'
import CloudSyncModal from '../components/CloudSyncModal'
import DNSLeakModal from '../components/DNSLeakModal'
import ConnectionHistoryModal from '../components/ConnectionHistoryModal'
import FullBackupModal from '../components/FullBackupModal'
import OnboardingWizardModal from '../components/OnboardingWizardModal'
import ProxyChainModal from '../components/ProxyChainModal'
import MobileRemoteModal from '../components/MobileRemoteModal'
import FragmentStudioModal from '../components/FragmentStudioModal'
import GamingModeModal from '../components/GamingModeModal'
import ScreenQRScannerModal from '../components/ScreenQRScannerModal'
import ConfigConverterModal from '../components/ConfigConverterModal'
import SmartISPOptimizerCard from '../components/SmartISPOptimizerCard'
import NearbySyncModal from '../components/NearbySyncModal'
import FloatingMiniWidget from '../components/FloatingMiniWidget'
import { useGlobalHotkeys } from '../hooks/useGlobalHotkeys'

// ─── Profile Search Selector ─────────────────────────────────────────────────
// Searchable dropdown with latency sorting and group display
type SortMode = 'default' | 'latency' | 'name'

function ProfileSearchSelector({
  profiles,
  selectedProfileId,
  onSelect,
}: {
  profiles: Profile[]
  selectedProfileId: string
  onSelect: (id: string) => void
}) {
  const [query, setQuery] = useState('')
  const [sort, setSort] = useState<SortMode>('default')
  const [open, setOpen] = useState(false)
  const containerRef = useRef<HTMLDivElement>(null)

  const filtered = useMemo(() => {
    let list = [...profiles]
    if (query.trim()) {
      const q = query.toLowerCase()
      list = list.filter(
        p =>
          p.name.toLowerCase().includes(q) ||
          (p.server || '').toLowerCase().includes(q) ||
          (p.group || '').toLowerCase().includes(q)
      )
    }
    if (sort === 'latency') {
      list.sort((a, b) => {
        const la = a.latency_ms && a.latency_ms > 0 ? a.latency_ms : 99999
        const lb = b.latency_ms && b.latency_ms > 0 ? b.latency_ms : 99999
        return la - lb
      })
    } else if (sort === 'name') {
      list.sort((a, b) => a.name.localeCompare(b.name))
    }
    return list
  }, [profiles, query, sort])

  const selected = profiles.find(p => p.id === selectedProfileId)

  // Close on outside click
  useEffect(() => {
    const handler = (e: MouseEvent) => {
      if (containerRef.current && !containerRef.current.contains(e.target as Node)) {
        setOpen(false)
      }
    }
    document.addEventListener('mousedown', handler)
    return () => document.removeEventListener('mousedown', handler)
  }, [])

  return (
    <div ref={containerRef} className="relative w-full">
      {/* Trigger button */}
      <button
        id="dashboard-profile-select"
        type="button"
        onClick={() => setOpen(o => !o)}
        className="w-full flex items-center gap-2 px-3 py-2 rounded-xl bg-slate-900/90 border border-slate-700/60 hover:border-cyan-500/50 transition text-xs font-semibold text-slate-200 text-left rtl:text-right"
      >
        <Server size={13} className="text-cyan-400 shrink-0" />
        <span className="flex-1 truncate">
          {selected
            ? `${getCountryFlag(selected.country_code)} ${selected.name}${selected.latency_ms ? ` (${selected.latency_ms}ms)` : ''}`
            : t('select_server_prompt')}
        </span>
        <span className="text-slate-500 text-[9px]">▼</span>
      </button>

      {open && (
        <div className="absolute z-50 bottom-full mb-1.5 w-full rounded-2xl bg-slate-950/95 backdrop-blur-xl border border-slate-700 shadow-2xl shadow-black/80 overflow-hidden animate-in fade-in slide-in-from-bottom-2 duration-150">
          {/* Search + sort row */}
          <div className="flex items-center gap-1.5 p-2 border-b border-slate-800">
            <div className="flex items-center gap-1 flex-1 bg-slate-900 border border-slate-700 rounded-xl px-2 py-1">
              <Search size={12} className="text-slate-500 shrink-0" />
              <input
                autoFocus
                value={query}
                onChange={e => setQuery(e.target.value)}
                placeholder={t('search') || 'Search profiles...'}
                className="flex-1 bg-transparent text-xs text-slate-200 outline-none placeholder-slate-600"
              />
              {query && (
                <button onClick={() => setQuery('')} className="text-slate-500 hover:text-slate-300">
                  <X size={11} />
                </button>
              )}
            </div>
            <button
              onClick={() => setSort(s => s === 'latency' ? 'name' : s === 'name' ? 'default' : 'latency')}
              className="p-1.5 rounded-lg bg-slate-800 hover:bg-slate-700 text-slate-400 hover:text-cyan-400 transition text-[9px] font-bold flex items-center gap-1"
              title="Sort: default → latency → name"
            >
              <SortAsc size={12} />
              <span>{sort === 'latency' ? 'ms' : sort === 'name' ? 'A-Z' : '—'}</span>
            </button>
          </div>

          {/* Profile list */}
          <div className="max-h-52 overflow-y-auto">
            {filtered.length === 0 ? (
              <div className="px-3 py-4 text-center text-xs text-slate-500">{t('no_results') || 'No matching profiles'}</div>
            ) : (
              filtered.map(p => (
                <button
                  key={p.id}
                  type="button"
                  onClick={() => { onSelect(p.id); setOpen(false); setQuery('') }}
                  className={`w-full flex items-center gap-2 px-3 py-2 text-xs text-left rtl:text-right transition hover:bg-slate-800 ${
                    p.id === selectedProfileId ? 'bg-cyan-500/10 text-cyan-300' : 'text-slate-300'
                  }`}
                >
                  <span className="text-base leading-none shrink-0">{getCountryFlag(p.country_code)}</span>
                  <span className="flex-1 truncate font-semibold">{p.name}</span>
                  {p.group && <span className="text-[10px] text-slate-500 truncate max-w-[60px]">{p.group}</span>}
                  {p.latency_ms && p.latency_ms > 0 && (
                    <span className={`text-[10px] font-mono font-bold shrink-0 ${
                      p.latency_ms < 150 ? 'text-emerald-400' : p.latency_ms < 300 ? 'text-amber-400' : 'text-rose-400'
                    }`}>{p.latency_ms}ms</span>
                  )}
                </button>
              ))
            )}
          </div>

          {filtered.length < profiles.length && (
            <div className="px-3 py-1.5 text-center text-[10px] text-slate-600 border-t border-slate-800">
              {filtered.length} / {profiles.length} profiles
            </div>
          )}
        </div>
      )}
    </div>
  )
}
// ─────────────────────────────────────────────────────────────────────────────

interface Props {
  status: ConnectionStatus
  stats: TrafficStats
  onOpenDoctor?: () => void
  onOpenAppSelector?: () => void
}

interface SpeedPoint {
  t: number
  down: number
  up: number
}

const MAX_POINTS = 25

export default function Dashboard({ status, stats, onOpenDoctor, onOpenAppSelector }: Props) {
  const [selectedProfileId, setSelectedProfileId] = useState<string>('')
  const [profiles, setProfiles] = useState<Profile[]>([])
  const [speedHistory, setSpeedHistory] = useState<SpeedPoint[]>([])
  const [isActioning, setIsActioning] = useState(false)
  const [realDelay, setRealDelay] = useState<number | null>(null)
  const [isTestingDelay, setIsTestingDelay] = useState(false)
  const [routingMode, setRoutingMode] = useState<string>('bypass_iran')
  const [proxyMode, setProxyMode] = useState<string>('tun')
  const [gameMode, setGameMode] = useState<boolean>(false)
  const [ipInfo, setIpInfo] = useState<IPInfo | null>(null)
  const [isLoadingIP, setIsLoadingIP] = useState(false)
  const [copiedIP, setCopiedIP] = useState(false)
  const [isPingingAll, setIsPingingAll] = useState(false)
  const [dashboardToast, setDashboardToast] = useState<string | null>(null)
  const [showQRModal, setShowQRModal] = useState(false)
  const [copiedQRLink, setCopiedQRLink] = useState(false)
  const [peakDown, setPeakDown] = useState(0)
  const [peakUp, setPeakUp] = useState(0)
  const [monthlyQuotaGB, setMonthlyQuotaGB] = useState<number>(0)
  const [healthStatus, setHealthStatus] = useState<import('../types').HealthStatus | null>(null)

  // Modals
  const [showSpeedTestModal, setShowSpeedTestModal] = useState(false)
  const [showLANShareModal, setShowLANShareModal] = useState(false)
  const [showRadarModal, setShowRadarModal] = useState(false)
  const [showEmergencyModal, setShowEmergencyModal] = useState(false)
  const [showQuotaModal, setShowQuotaModal] = useState(false)
  const [showCloudModal, setShowCloudModal] = useState(false)
  const [showDNSLeakModal, setShowDNSLeakModal] = useState(false)
  const [showHistoryModal, setShowHistoryModal] = useState(false)
  const [showBackupModal, setShowBackupModal] = useState(false)
  const [showOnboarding, setShowOnboarding] = useState(false)
  const [showChainModal, setShowChainModal] = useState(false)
  const [showRemoteModal, setShowRemoteModal] = useState(false)
  const [showFragmentStudio, setShowFragmentStudio] = useState(false)
  const [showGamingModal, setShowGamingModal] = useState(false)
  const [showScreenQRModal, setShowScreenQRModal] = useState(false)
  const [showConverterModal, setShowConverterModal] = useState(false)
  const [showNearbyModal, setShowNearbyModal] = useState(false)
  const [showToolsDropdown, setShowToolsDropdown] = useState(false)
  const [currentSettings, setCurrentSettings] = useState<Settings>({} as Settings)
  const tickRef = useRef(0)
  const lastNotifiedStateRef = useRef<string>('')

  const showToast = (msg: string) => {
    setDashboardToast(msg)
    setTimeout(() => setDashboardToast(null), 4000)
  }

  const handlePanicWipe = async () => {
    if (window.confirm('⚠️ PANIC ACTION: Are you sure you want to disconnect, clear all proxies, and wipe session logs immediately?')) {
      try {
        await api.panicWipe()
        showToast('Panic Wipe Completed: System restored to clean direct state.')
        sound.playDisconnect()
      } catch (err) {
        console.error(err)
      }
    }
  }

  // Request desktop notification permission on mount
  useEffect(() => {
    requestNotificationPermission()

    const unsubHealth = api.onEvent('health:update', (data: any) => {
      if (data) setHealthStatus(data)
    })

    return () => {
      if (typeof unsubHealth === 'function') unsubHealth()
    }
  }, [])

  // Track Peak Speeds
  useEffect(() => {
    if (stats.speed_download > peakDown) setPeakDown(stats.speed_download)
    if (stats.speed_upload > peakUp) setPeakUp(stats.speed_upload)
  }, [stats.speed_download, stats.speed_upload, peakDown, peakUp])

  // Fetch current live public IP
  const fetchLiveIP = useCallback(async () => {
    setIsLoadingIP(true)
    try {
      const info = await api.lookupExternalIP()
      if (info && info.ip) {
        setIpInfo(info)
      }
    } catch (err) {
      console.warn('IP lookup notice:', err)
    } finally {
      setIsLoadingIP(false)
    }
  }, [])

  const handleSelectProfile = (id: string) => {
    setSelectedProfileId(id)
    // NOTE: last_profile_id is persisted in backend settings on Connect.
    // We do NOT duplicate it in localStorage to avoid inconsistency.
  }

  const loadDashboardData = useCallback(async () => {
    try {
      const [pList, s] = await Promise.all([api.getProfiles(), api.getSettings()])
      if (pList && Array.isArray(pList)) {
        setProfiles(pList)
        const savedLocal = localStorage.getItem('nexvpn_last_selected_profile_id')
        let targetId = ''
        if (s?.last_profile_id && pList.some(p => p.id === s.last_profile_id)) {
          targetId = s.last_profile_id
        } else if (savedLocal && pList.some(p => p.id === savedLocal)) {
          // Legacy: still read from localStorage once, then rely on backend
          targetId = savedLocal
          localStorage.removeItem('nexvpn_last_selected_profile_id')
        } else if (pList.length > 0) {
          targetId = pList[0].id
        }
        if (targetId) setSelectedProfileId(targetId)
        if (pList.length === 0 && localStorage.getItem('nexvpn_onboarding_completed') !== 'true') {
          setShowOnboarding(true)
        }
      }
      if (s) {
        if (s.routing_mode) setRoutingMode(s.routing_mode)
        if (s.proxy_mode) setProxyMode(s.proxy_mode)
        if (s.game_mode_enabled !== undefined) setGameMode(s.game_mode_enabled)
        if (s.monthly_quota_gb) setMonthlyQuotaGB(s.monthly_quota_gb)
      }
    } catch (err) {
      console.error('Failed to load dashboard data:', err)
    }
  }, [])

  useEffect(() => {
    loadDashboardData()
  }, [loadDashboardData])

  // Update speed history on stats change
  useEffect(() => {
    setSpeedHistory(prev => {
      const next = [
        ...prev,
        {
          t: tickRef.current++,
          down: stats.speed_download,
          up: stats.speed_upload,
        },
      ]
      return next.slice(-MAX_POINTS)
    })
  }, [stats.speed_download, stats.speed_upload])

  // When connection state changes, re-fetch live IP
  useEffect(() => {
    if (status.state === 'connected') {
      const timer = setTimeout(() => {
        fetchLiveIP()
      }, 1500)
      return () => clearTimeout(timer)
    } else if (status.state === 'disconnected') {
      setRealDelay(null)
      const timer = setTimeout(() => {
        fetchLiveIP()
      }, 500)
      return () => clearTimeout(timer)
    }
  }, [status.state, fetchLiveIP])

  const handleConnectToggle = useCallback(async () => {
    if (isActioning) return
    setIsActioning(true)
    try {
      const idToConnect = selectedProfileId || (profiles.length > 0 ? profiles[0].id : '')
      if (status.state === 'connected' && idToConnect && idToConnect !== status.profile_id) {
        // Seamlessly switch to newly selected profile
        sound.playConnect()
        handleSelectProfile(idToConnect)
        await api.connect(idToConnect)
      } else if (status.state === 'connected' || status.state === 'connecting') {
        sound.playDisconnect()
        await api.disconnect()
        setRealDelay(null)
      } else {
        if (idToConnect) {
          sound.playConnect()
          handleSelectProfile(idToConnect)
          await api.connect(idToConnect)
        }
      }
    } catch (err: any) {
      console.error('Connect toggle error:', err)
      showToast(err?.message || t('error'))
    } finally {
      setIsActioning(false)
    }
  }, [isActioning, status.state, status.profile_id, selectedProfileId, profiles])

  const handleTestRealDelay = async () => {
    if (isTestingDelay) return
    setIsTestingDelay(true)
    try {
      const delay = await api.realPingActiveProfile()
      setRealDelay(delay)
    } catch {
      setRealDelay(-1)
    } finally {
      setIsTestingDelay(false)
    }
  }

  const handleModeChange = async (newMode: string) => {
    setProxyMode(newMode)
    try {
      await api.setProxyMode(newMode)
    } catch (err) {
      console.error('Failed to change proxy mode:', err)
    }
  }

  const handleRoutingChange = async (newMode: string) => {
    setRoutingMode(newMode)
    try {
      const s = await api.getSettings()
      await api.saveSettings({ ...s, routing_mode: newMode })
    } catch (err) {
      console.error('Failed to change routing mode:', err)
    }
  }

  const handleToggleGameMode = async () => {
    const next = !gameMode
    setGameMode(next)
    try {
      const s = await api.getSettings()
      await api.saveSettings({ ...s, game_mode_enabled: next })
      showToast(next ? t('game_mode_active') : t('game_mode_inactive'))
    } catch (err) {
      console.error('Failed to toggle game mode:', err)
    }
  }

  const handlePingAllAndPickBest = async () => {
    if (isPingingAll || profiles.length === 0) return
    setIsPingingAll(true)
    try {
      const updated = await api.pingAllProfiles()
      if (updated && Array.isArray(updated) && updated.length > 0) {
        setProfiles(updated)
        // Find fastest healthy profile
        const sorted = [...updated].sort((a, b) => {
          const lA = a.latency_ms && a.latency_ms > 0 ? a.latency_ms : 9999
          const lB = b.latency_ms && b.latency_ms > 0 ? b.latency_ms : 9999
          return lA - lB
        })
        const fastest = sorted[0]
        if (fastest && fastest.latency_ms && fastest.latency_ms > 0 && fastest.latency_ms < 9999) {
          handleSelectProfile(fastest.id)
          showToast(`${t('ping_all_selected')} ${fastest.name} (${fastest.latency_ms}ms)`)
        } else {
          showToast(t('saved_successfully'))
        }
      }
    } catch (err) {
      console.error('Ping all error:', err)
    } finally {
      setIsPingingAll(false)
    }
  }

  const handleAutoConnectFastest = async () => {
    if (profiles.length === 0) return
    const sorted = [...profiles].sort((a, b) => {
      const pA = a.latency_ms && a.latency_ms > 0 ? a.latency_ms : 9999
      const pB = b.latency_ms && b.latency_ms > 0 ? b.latency_ms : 9999
      return pA - pB
    })
    const fastest = sorted[0]
    if (fastest) {
      handleSelectProfile(fastest.id)
      sound.playConnect()
      await api.connect(fastest.id)
    }
  }

  const isConnected = status.state === 'connected'
  const isConnecting = status.state === 'connecting'
  const isDisconnected = status.state === 'disconnected'
  const isError = status.state === 'error'

  const activeProfile = profiles.find(p => p.id === (status.profile_id || selectedProfileId))

  useGlobalHotkeys({
    onToggleConnect: handleConnectToggle,
    onOpenSpeedTest: () => setShowSpeedTestModal(true),
    onOpenRadar: () => setShowRadarModal(true),
    onOpenLANShare: () => setShowLANShareModal(true),
    onOpenEmergencyFixer: () => setShowEmergencyModal(true),
    onOpenQuotaManager: () => setShowQuotaModal(true),
    onOpenDNSLeak: () => setShowDNSLeakModal(true),
    onOpenHistory: () => setShowHistoryModal(true),
    onOpenBackup: () => setShowBackupModal(true),
    onPingAll: handlePingAllAndPickBest,
  })


  const totalUsedBytes = stats.bytes_received + stats.bytes_sent
  const usedGB = (totalUsedBytes / (1024 * 1024 * 1024)).toFixed(2)
  const quotaPercent = monthlyQuotaGB > 0 ? Math.min(100, Math.round((parseFloat(usedGB) / monthlyQuotaGB) * 100)) : 0

  return (
    <div className="page space-y-4 animate-fade-in">
      {/* Toast Notification */}
      {dashboardToast && (
        <div className="fixed top-4 left-1/2 -translate-x-1/2 z-50 bg-slate-900/95 border border-cyan-500/50 shadow-2xl shadow-cyan-500/20 text-slate-100 text-xs px-4 py-2 rounded-xl flex items-center gap-2 backdrop-blur-md animate-fade-in">
          <Sparkles size={14} className="text-cyan-400 animate-pulse" />
          <span>{dashboardToast}</span>
        </div>
      )}
      {/* Sleek Professional Sticky Action Toolbar */}
      <div className="sticky top-0 z-30 flex items-center justify-between flex-wrap gap-2 py-2.5 px-3 bg-slate-950/85 backdrop-blur-xl border border-slate-800/80 rounded-2xl shadow-lg shadow-black/30">
        {/* Left Side: Real-Time Server / Network Status Tag */}
        <div className="flex items-center gap-2">
          <span className="text-xs font-bold text-slate-300 flex items-center gap-1.5">
            <span className="w-2 h-2 rounded-full bg-cyan-400 animate-pulse" />
            {t('control_hub')}
          </span>
        </div>

        {/* Right Side: Clean Organized Button Bar */}
        <div className="flex items-center gap-2 flex-wrap">
          {profiles.length > 0 && !isConnected && (
            <button
              onClick={handleAutoConnectFastest}
              className="btn btn-secondary btn-sm flex items-center gap-1.5 text-xs font-bold text-orange-400 border-orange-500/30 hover:bg-orange-500/10 shadow-sm"
              title={t('auto_connect_fastest')}
            >
              <Zap size={13} className="fill-orange-400 text-orange-400" />
              <span>{t('smart_connect')}</span>
            </button>
          )}

          <button
            onClick={() => setShowGamingModal(true)}
            className={`btn btn-sm flex items-center gap-1.5 text-xs font-bold transition ${
              gameMode
                ? 'bg-emerald-500/20 text-emerald-300 border-emerald-500/50 shadow-lg shadow-emerald-500/10'
                : 'btn-secondary text-slate-300 border-slate-700/60 hover:text-emerald-300'
            }`}
            title="Gaming Accelerator & Anti-Sanction DNS"
          >
            <Gamepad2 size={13} className={gameMode ? 'text-emerald-400 animate-pulse' : 'text-emerald-400'} />
            <span>{t('gaming')}</span>
          </button>

          <button
            onClick={() => setShowFragmentStudio(true)}
            className="btn btn-secondary btn-sm flex items-center gap-1.5 text-xs font-bold text-amber-400 border-amber-500/30 hover:bg-amber-500/10"
            title="DPI Evasion & TLS Fragment Studio"
          >
            <Sparkles size={13} className="text-amber-400" />
            <span>{t('anti_dpi')}</span>
          </button>

          <button
            onClick={() => setShowSpeedTestModal(true)}
            className="btn btn-secondary btn-sm flex items-center gap-1.5 text-xs font-bold text-emerald-400 border-emerald-500/30 hover:bg-emerald-500/10"
            title={t('speed_test_btn')}
          >
            <Gauge size={13} />
            <span>{t('speed_test')}</span>
          </button>

          {/* All Network Tools Dropdown Menu */}
          <div className="relative">
            <button
              onClick={() => setShowToolsDropdown(!showToolsDropdown)}
              className={`btn btn-secondary btn-sm flex items-center gap-1.5 text-xs font-bold transition ${
                showToolsDropdown ? 'bg-cyan-500/20 text-cyan-300 border-cyan-500/50' : 'text-slate-300 border-slate-700/60 hover:text-cyan-300'
              }`}
              title="More Network Tools & Utilities"
            >
              <Wrench size={13} className="text-cyan-400" />
              <span>{t('tools')}</span>
              <span className="text-[9px] opacity-70">▼</span>
            </button>

            {showToolsDropdown && (
              <>
                <div className="fixed inset-0 z-30" onClick={() => setShowToolsDropdown(false)} />
                <div className="absolute ltr:right-0 rtl:left-0 mt-2 w-72 rounded-2xl bg-slate-900 border border-slate-700 shadow-2xl p-2 z-40 animate-fade-in grid grid-cols-2 gap-1.5 text-xs">
                  {onOpenDoctor && (
                    <button
                      onClick={() => { setShowToolsDropdown(false); onOpenDoctor() }}
                      className="flex items-center gap-2 p-2 rounded-xl hover:bg-slate-800 text-slate-200 text-left rtl:text-right transition"
                    >
                      <ShieldCheck size={14} className="text-blue-400 shrink-0" />
                      <span className="truncate">{t('doctor')}</span>
                    </button>
                  )}

                  <button
                    onClick={() => { setShowToolsDropdown(false); setShowDNSLeakModal(true) }}
                    className="flex items-center gap-2 p-2 rounded-xl hover:bg-slate-800 text-slate-200 text-left rtl:text-right transition"
                  >
                    <Shield size={14} className="text-cyan-400 shrink-0" />
                    <span className="truncate">{t('dns_leak')}</span>
                  </button>

                  <button
                    onClick={() => { setShowToolsDropdown(false); setShowRadarModal(true) }}
                    className="flex items-center gap-2 p-2 rounded-xl hover:bg-slate-800 text-slate-200 text-left rtl:text-right transition"
                  >
                    <Radio size={14} className="text-pink-400 shrink-0" />
                    <span className="truncate">{t('radar')}</span>
                  </button>

                  <button
                    onClick={() => { setShowToolsDropdown(false); setShowLANShareModal(true) }}
                    className="flex items-center gap-2 p-2 rounded-xl hover:bg-slate-800 text-slate-200 text-left rtl:text-right transition"
                  >
                    <Wifi size={14} className="text-purple-400 shrink-0" />
                    <span className="truncate">{t('wifi_share')}</span>
                  </button>

                  <button
                    onClick={() => { setShowToolsDropdown(false); setShowRemoteModal(true) }}
                    className="flex items-center gap-2 p-2 rounded-xl hover:bg-slate-800 text-slate-200 text-left rtl:text-right transition"
                  >
                    <Smartphone size={14} className="text-sky-400 shrink-0" />
                    <span className="truncate">{t('remote_hub')}</span>
                  </button>

                  <button
                    onClick={() => { setShowToolsDropdown(false); setShowChainModal(true) }}
                    className="flex items-center gap-2 p-2 rounded-xl hover:bg-slate-800 text-slate-200 text-left rtl:text-right transition"
                  >
                    <Link2 size={14} className="text-amber-400 shrink-0" />
                    <span className="truncate">{t('multi_hop')}</span>
                  </button>

                  <button
                    onClick={() => { setShowToolsDropdown(false); setShowQuotaModal(true) }}
                    className="flex items-center gap-2 p-2 rounded-xl hover:bg-slate-800 text-slate-200 text-left rtl:text-right transition"
                  >
                    <PieChart size={14} className="text-emerald-400 shrink-0" />
                    <span className="truncate">{t('quota')}</span>
                  </button>

                  <button
                    onClick={() => { setShowToolsDropdown(false); setShowCloudModal(true) }}
                    className="flex items-center gap-2 p-2 rounded-xl hover:bg-slate-800 text-slate-200 text-left rtl:text-right transition"
                  >
                    <Cloud size={14} className="text-sky-400 shrink-0" />
                    <span className="truncate">{t('cloud_hub')}</span>
                  </button>

                  <button
                    onClick={() => { setShowToolsDropdown(false); setShowHistoryModal(true) }}
                    className="flex items-center gap-2 p-2 rounded-xl hover:bg-slate-800 text-slate-200 text-left rtl:text-right transition"
                  >
                    <History size={14} className="text-indigo-400 shrink-0" />
                    <span className="truncate">{t('history')}</span>
                  </button>

                  <button
                    onClick={() => { setShowToolsDropdown(false); setShowBackupModal(true) }}
                    className="flex items-center gap-2 p-2 rounded-xl hover:bg-slate-800 text-slate-200 text-left rtl:text-right transition"
                  >
                    <Archive size={14} className="text-blue-400 shrink-0" />
                    <span className="truncate">{t('backup')}</span>
                  </button>

                  <button
                    onClick={() => { setShowToolsDropdown(false); setShowScreenQRModal(true) }}
                    className="flex items-center gap-2 p-2 rounded-xl hover:bg-slate-800 text-slate-200 text-left rtl:text-right transition"
                  >
                    <Scan size={14} className="text-cyan-400 shrink-0" />
                    <span className="truncate">{t('screen_qr_title') || 'Scan QR'}</span>
                  </button>

                  <button
                    onClick={() => { setShowToolsDropdown(false); setShowConverterModal(true) }}
                    className="flex items-center gap-2 p-2 rounded-xl hover:bg-slate-800 text-slate-200 text-left rtl:text-right transition"
                  >
                    <ArrowRightLeft size={14} className="text-amber-400 shrink-0" />
                    <span className="truncate">{t('config_converter_title') || 'Converter'}</span>
                  </button>

                  <button
                    onClick={() => { setShowToolsDropdown(false); setShowNearbyModal(true) }}
                    className="col-span-2 flex items-center gap-2 p-2 rounded-xl hover:bg-slate-800 text-purple-300 text-left rtl:text-right transition"
                  >
                    <Radio size={14} className="text-purple-400 shrink-0" />
                    <span className="truncate">{t('nearby_share_title') || 'Nearby Wi-Fi Share (P2P)'}</span>
                  </button>

                  <button
                    onClick={() => { setShowToolsDropdown(false); setShowEmergencyModal(true) }}
                    className="col-span-2 flex items-center gap-2 p-2 rounded-xl hover:bg-rose-950/40 text-rose-300 text-left rtl:text-right transition border-t border-slate-800 mt-1"
                  >
                    <Wrench size={14} className="text-rose-400 shrink-0" />
                    <span className="truncate">{t('emergency_repair')}</span>
                  </button>
                </div>
              </>
            )}
          </div>

          {/* Panic Wipe Emergency Button */}
          <button
            onClick={handlePanicWipe}
            className="btn btn-sm flex items-center gap-1 text-xs font-bold text-rose-400 bg-rose-500/10 border border-rose-500/30 hover:bg-rose-500/25 transition"
            title="Instantly Disconnect, Wipe Sessions & Kill System Proxy"
          >
            <AlertTriangle size={13} className="text-rose-400" />
            <span>{t('panic_wipe')}</span>
          </button>
        </div>
      </div>

      {/* Hero Layout: Connection Orb on Left, Live Chart & Selector on Right */}
      <div className="grid grid-cols-1 lg:grid-cols-12 gap-5">
        {/* Connection Core Card */}
        <div className="lg:col-span-5 card flex flex-col items-center justify-between p-6 text-center relative">
          {/* Top Info Header */}
          <div className="w-full flex items-center justify-between text-xs text-muted mb-2">
            <span className="font-semibold uppercase tracking-wider">{t('protocol')}</span>
            <span className="badge badge-muted text-[11px] font-mono">
              {activeProfile ? `${getProtocolEmoji(activeProfile.protocol)} ${getProtocolName(activeProfile.protocol)}` : 'Smart Dual Core'}
            </span>
          </div>

          {/* Smart Cybernetic Glowing Orb Button */}
          <div className="hero-orb-container my-3">
            <div className={`hero-orb-wrapper ${isConnected ? 'connected' : isConnecting ? 'connecting' : isError ? 'error' : ''}`}>
              <div className="hero-orb-ring" />
              <button
                id="main-connect-btn"
                className={`hero-orb-btn ${
                  isConnected ? 'connected' : isConnecting ? 'connecting' : isError ? 'error' : ''
                }`}
                onClick={handleConnectToggle}
                disabled={isActioning}
                title={isConnected ? t('click_to_disconnect') : t('click_to_connect')}
              >
                <Power
                  size={46}
                  className={`transition-all duration-300 ${
                    isConnected
                      ? 'text-emerald-400 drop-shadow-[0_0_15px_rgba(16,185,129,0.9)] scale-110'
                      : isConnecting
                      ? 'text-amber-400 animate-spin'
                      : isError
                      ? 'text-rose-400'
                      : 'text-slate-300 hover:text-cyan-300'
                  }`}
                />
                <span
                  className={`connect-btn-label font-bold text-xs tracking-wide transition-colors ${
                    isConnected
                      ? 'text-emerald-400'
                      : isConnecting
                      ? 'text-amber-400'
                      : isError
                      ? 'text-rose-400'
                      : 'text-slate-300'
                  }`}
                >
                  {isConnected
                    ? t('connected')
                    : isConnecting
                    ? t('connecting')
                    : isError
                    ? t('error')
                    : t('connect')}
                </span>
                {isConnected && (
                  <span className="text-[10px] font-mono text-emerald-400/90 -mt-1 flex items-center gap-1">
                    <span className="w-1.5 h-1.5 rounded-full bg-emerald-400 animate-ping inline-block" />
                    {t('active_status')}
                  </span>
                )}
              </button>
            </div>
          </div>

          {/* Connection Duration & Real Delay Status */}
          {isConnected ? (
            <div className="w-full flex flex-col items-center gap-2 pt-2 border-t border-white/5">
              <div className="flex items-center gap-2 font-mono text-xl font-extrabold text-slate-100">
                <Clock size={16} className="text-accent" />
                <span>{formatDuration(status.duration || 0)}</span>
              </div>

              <div className="flex items-center gap-2">
                <button
                  className="btn btn-secondary btn-sm text-[11px] py-1 px-2.5"
                  onClick={handleTestRealDelay}
                  disabled={isTestingDelay}
                >
                  <Radio size={12} className="text-emerald-400" />
                  {isTestingDelay ? t('testing_delay') : t('real_delay_test')}
                </button>
                {realDelay !== null && (
                  <span
                    className={`badge font-mono text-xs ${
                      realDelay > 0 && realDelay < 300
                        ? 'badge-success'
                        : realDelay >= 300
                        ? 'badge-warning'
                        : 'badge-error'
                    }`}
                  >
                    {realDelay > 0 ? `${realDelay} ms` : 'Timeout'}
                  </span>
                )}
              </div>

              {/* Active Connected Server Info Pill */}
              <div className="w-full p-2.5 rounded-2xl bg-slate-900/80 border border-cyan-500/20 flex items-center justify-between gap-2 mt-1">
                <div className="flex items-center gap-2.5 min-w-0 text-left rtl:text-right">
                  <CountryFlag codeOrName={activeProfile?.country_code || activeProfile?.name} size="lg" />
                  <div className="min-w-0">
                    <div className="text-xs font-bold text-slate-100 truncate" title={activeProfile?.name || status.profile_name}>
                      {activeProfile?.name || status.profile_name || 'Active Node'}
                    </div>
                    <div className="text-[10px] font-mono text-cyan-400/90 truncate">
                      {activeProfile?.server || status.server || 'Protected Gateway'}
                    </div>
                  </div>
                </div>
                <span className="text-[10px] font-mono font-bold px-2 py-0.5 rounded-lg bg-cyan-500/10 text-cyan-300 border border-cyan-500/30 shrink-0">
                  {activeProfile?.protocol ? getProtocolName(activeProfile.protocol) : status.protocol ? getProtocolName(status.protocol) : 'VPN Node'}
                </span>
              </div>

              {healthStatus && (
                <div className="flex items-center gap-2 px-3 py-1 rounded-xl bg-black/40 border border-white/10 text-xs font-mono mt-0.5">
                  <span
                    className={`w-2 h-2 rounded-full ${
                      healthStatus.quality_score >= 80
                        ? 'bg-emerald-400 animate-pulse'
                        : healthStatus.quality_score >= 50
                        ? 'bg-amber-400'
                        : 'bg-red-400'
                    }`}
                  />
                  <span className="text-slate-300 font-bold">Quality: {healthStatus.quality_score}%</span>
                  <span className="text-slate-500">•</span>
                  <span className="text-slate-400 capitalize">{healthStatus.quality_level}</span>
                  {healthStatus.jitter_ms > 0 && (
                    <>
                      <span className="text-slate-500">•</span>
                      <span className="text-slate-400">Jitter {healthStatus.jitter_ms}ms</span>
                    </>
                  )}
                </div>
              )}
            </div>
          ) : (
            /* Server Selector with Search, 1-Click Ping All and Mobile QR Exporter */
            <div className="w-full space-y-2.5 pt-2 border-t border-white/5">
              {/* Connection Failure Error Diagnostic Banner */}
              {isError && status.error_msg && (
                <div className="w-full p-3 rounded-2xl bg-rose-950/60 border border-rose-500/40 text-left rtl:text-right space-y-1.5 animate-in fade-in zoom-in-95 duration-200">
                  <div className="flex items-center justify-between gap-2">
                    <div className="flex items-center gap-2 text-rose-300 font-bold text-xs">
                      <AlertTriangle size={14} className="text-rose-400 shrink-0 animate-pulse" />
                      <span>{t('connection_failed') || 'خطای اتصال به سرور'}</span>
                    </div>
                    <button
                      onClick={() => setShowEmergencyModal(true)}
                      className="text-[10px] text-cyan-400 hover:underline flex items-center gap-1"
                    >
                      🩺 {t('open_doctor') || 'پزشک هوشمند'}
                    </button>
                  </div>
                  <p className="text-[11px] text-rose-200/90 leading-relaxed font-sans font-medium">
                    {status.error_msg}
                  </p>
                </div>
              )}

              {/* Search filter input */}
              <ProfileSearchSelector
                profiles={profiles}
                selectedProfileId={selectedProfileId}
                onSelect={handleSelectProfile}
              />
              <div className="flex items-center gap-1.5">
                <button
                  type="button"
                  onClick={handlePingAllAndPickBest}
                  disabled={isPingingAll || profiles.length === 0}
                  className="btn btn-secondary btn-sm px-2 text-xs font-bold text-amber-400 border-amber-500/30 hover:bg-amber-500/10 flex items-center gap-1 whitespace-nowrap"
                  title={t('ping_all_btn')}
                >
                  <Zap size={13} className={isPingingAll ? 'animate-spin text-amber-400' : 'fill-amber-400'} />
                  <span>{isPingingAll ? '...' : t('ping_all_btn')}</span>
                </button>

                <button
                  type="button"
                  onClick={() => setShowQRModal(true)}
                  disabled={!selectedProfileId}
                  className="btn btn-secondary btn-sm p-1.5 text-slate-300 hover:text-cyan-400 border-slate-700 hover:border-cyan-500/40"
                  title={t('qr_code_modal_title')}
                >
                  <QrCode size={14} />
                </button>
              </div>
            </div>
          )}
        </div>

        {/* Right Side: Speed Graph & Mode Switches */}
        <div className="lg:col-span-7 flex flex-col gap-4">
          {/* Proxy Mode Segmented Bar */}
          <div className="card p-3 flex items-center justify-between flex-wrap gap-2">
            <div className="flex items-center gap-2 text-xs font-bold text-slate-300 px-2">
              <Layers size={15} className="text-accent" />
              <span>{t('proxy_mode_label')}</span>
            </div>

            <div className="flex bg-black/40 p-1 rounded-xl border border-white/5 gap-1">
              <button
                type="button"
                onClick={() => handleModeChange('tun')}
                className={`px-3 py-1.5 rounded-lg text-xs font-bold transition ${
                  proxyMode === 'tun'
                    ? 'bg-accent text-white shadow-md shadow-accent/25'
                    : 'text-slate-400 hover:text-slate-200'
                }`}
              >
                {t('mode_switch_tun')}
              </button>

              <button
                type="button"
                onClick={() => handleModeChange('system_proxy')}
                className={`px-3 py-1.5 rounded-lg text-xs font-bold transition ${
                  proxyMode === 'system_proxy'
                    ? 'bg-accent text-white shadow-md shadow-accent/25'
                    : 'text-slate-400 hover:text-slate-200'
                }`}
              >
                {t('mode_switch_proxy')}
              </button>
            </div>
          </div>

          {/* Real-time Traffic Graph Card */}
          <div className="card p-4 flex-1 flex flex-col justify-between">
            <div className="flex items-center justify-between mb-2">
              <div className="flex items-center gap-2 text-xs font-bold text-slate-200">
                <Activity size={15} className="text-emerald-400" />
                <span>{t('real_time_traffic')}</span>
              </div>
              <div className="flex items-center gap-4 text-xs font-mono">
                <span className="flex items-center gap-1.5 text-emerald-400 font-bold">
                  <ArrowDown size={13} /> {formatSpeed(stats.speed_download)}
                </span>
                <span className="flex items-center gap-1.5 text-blue-400 font-bold">
                  <ArrowUp size={13} /> {formatSpeed(stats.speed_upload)}
                </span>
              </div>
            </div>

            <div style={{ width: '100%', height: '140px' }}>
              <ResponsiveContainer width="100%" height="100%">
                <AreaChart data={speedHistory} margin={{ top: 5, right: 5, left: 0, bottom: 0 }}>
                  <defs>
                    <linearGradient id="downGrad" x1="0" y1="0" x2="0" y2="1">
                      <stop offset="5%" stopColor="#10b981" stopOpacity={0.4} />
                      <stop offset="95%" stopColor="#10b981" stopOpacity={0.0} />
                    </linearGradient>
                    <linearGradient id="upGrad" x1="0" y1="0" x2="0" y2="1">
                      <stop offset="5%" stopColor="#3b82f6" stopOpacity={0.4} />
                      <stop offset="95%" stopColor="#3b82f6" stopOpacity={0.0} />
                    </linearGradient>
                  </defs>
                  <XAxis dataKey="t" hide />
                  <YAxis hide domain={[0, 'auto']} />
                  <Tooltip
                    contentStyle={{
                      backgroundColor: 'rgba(10, 14, 23, 0.95)',
                      border: '1px solid rgba(255, 255, 255, 0.1)',
                      borderRadius: '8px',
                      fontSize: '11px',
                      fontFamily: 'var(--font-mono)',
                    }}
                    formatter={(value: any) => [formatSpeed(Number(value)), '']}
                  />
                  <Area
                    type="monotone"
                    dataKey="down"
                    stroke="#10b981"
                    strokeWidth={2}
                    fillOpacity={1}
                    fill="url(#downGrad)"
                    isAnimationActive={false}
                  />
                  <Area
                    type="monotone"
                    dataKey="up"
                    stroke="#3b82f6"
                    strokeWidth={2}
                    fillOpacity={1}
                    fill="url(#upGrad)"
                    isAnimationActive={false}
                  />
                </AreaChart>
              </ResponsiveContainer>
            </div>
          </div>
        </div>
      </div>

      {/* Live Real-Time Public IP & Network Security Card */}
      <div className="card p-4 bg-gradient-to-r from-slate-900/90 via-slate-900/60 to-slate-950/80 border-slate-700/40 relative overflow-hidden">
        <div className="flex items-center justify-between flex-wrap gap-3">
          <div className="flex items-center gap-3">
            <div className="shrink-0 flex items-center justify-center">
              <CountryFlag codeOrName={ipInfo?.country_code || (isConnected ? activeProfile?.country_code || 'US' : 'IR')} size="lg" />
            </div>
            <div>
              <div className="flex items-center gap-2 flex-wrap">
                <span className="text-xs text-slate-400 font-semibold">
                  {isConnected ? t('safe_ip_label') : t('public_ip_label')}
                </span>
                <span className="font-mono font-extrabold text-sm text-slate-100 bg-black/40 px-2 py-0.5 rounded-md border border-white/10 flex items-center gap-1.5">
                  {isLoadingIP ? (
                    <span className="text-slate-400 text-xs flex items-center gap-1">
                      <RefreshCw size={11} className="animate-spin text-cyan-400" /> {t('querying')}
                    </span>
                  ) : ipInfo?.ip ? (
                    <>
                      <span>{ipInfo.ip}</span>
                      <button
                        type="button"
                        onClick={() => {
                          navigator.clipboard.writeText(ipInfo.ip)
                          setCopiedIP(true)
                          setTimeout(() => setCopiedIP(false), 2000)
                        }}
                        className="text-slate-400 hover:text-cyan-300 ml-1 transition"
                        title="Copy IP"
                      >
                        {copiedIP ? <Check size={12} className="text-emerald-400" /> : <Copy size={12} />}
                      </button>
                    </>
                  ) : isConnected ? (
                    <span className="text-emerald-400 text-xs font-mono">{activeProfile?.server || 'Encrypted Gateway'}</span>
                  ) : (
                    <span className="text-slate-500 text-xs">-</span>
                  )}
                </span>

                <span
                  className={`badge text-[10px] font-bold ${
                    isConnected
                      ? 'badge-success flex items-center gap-1'
                      : 'badge-warning flex items-center gap-1'
                  }`}
                >
                  {isConnected ? (
                    <>
                      <ShieldCheck size={11} /> {t('protected_badge')}
                    </>
                  ) : (
                    <>
                      <ShieldAlert size={11} /> {t('exposed_badge')}
                    </>
                  )}
                </span>
              </div>

              <div className="text-xs text-slate-400 mt-1 flex items-center gap-2 flex-wrap">
                {ipInfo?.city && <span>📍 {ipInfo.city}, {ipInfo.country}</span>}
                {ipInfo?.isp && <span>🏢 {ipInfo.isp}</span>}
                {!ipInfo && !isLoadingIP && !isConnected && <span>{t('select_server_prompt')}</span>}
                {isConnected && !ipInfo && !isLoadingIP && (
                  <span className="text-cyan-400 text-xs font-medium">🔒 {activeProfile?.name || status.profile_name || 'Protected Node Active'}</span>
                )}
                {isConnected && status.server && ipInfo && (
                  <span className="text-slate-500 text-[11px] font-mono">({status.server})</span>
                )}
              </div>
            </div>
          </div>

          <div className="flex items-center gap-2">
            <button
              type="button"
              onClick={() => fetchLiveIP()}
              disabled={isLoadingIP}
              className="btn btn-secondary btn-sm text-xs flex items-center gap-1.5 border-slate-700 hover:border-cyan-500/40 text-slate-300 hover:text-cyan-300"
              title="Refresh IP"
            >
              <RefreshCw size={12} className={isLoadingIP ? 'animate-spin text-cyan-400' : ''} />
              <span>{t('live_ip_query')}</span>
            </button>
          </div>
        </div>
      </div>

      {/* 4-Metrics Stats Row + Peak Speed & Quota Progress */}
      <div className="stats-grid">
        <div className="stat-card">
          <div className="flex items-center gap-2">
            <ArrowDown size={14} className="text-emerald-400" />
            <span className="stat-label">{t('downloaded')}</span>
          </div>
          <div className="stat-value">{formatBytes(stats.bytes_received)}</div>
        </div>

        <div className="stat-card">
          <div className="flex items-center gap-2">
            <ArrowUp size={14} className="text-blue-400" />
            <span className="stat-label">{t('uploaded')}</span>
          </div>
          <div className="stat-value">{formatBytes(stats.bytes_sent)}</div>
        </div>

        <div className="stat-card">
          <div className="flex items-center gap-2">
            <TrendingUp size={14} className="text-amber-400" />
            <span className="stat-label">{t('peak_download_speed')}</span>
          </div>
          <div className="stat-value text-amber-300">{formatSpeed(peakDown)}</div>
        </div>

        <div className="stat-card">
          <div className="flex items-center gap-2">
            <Clock size={14} className="text-accent" />
            <span className="stat-label">{t('duration')}</span>
          </div>
          <div className="stat-value">{formatDuration(status.duration || 0)}</div>
        </div>
      </div>

      {/* Quota Progress Banner (If configured) */}
      {monthlyQuotaGB > 0 && (
        <div className="card p-3 bg-slate-900/80 border-slate-700/50 flex flex-col gap-2">
          <div className="flex items-center justify-between text-xs font-semibold text-slate-300">
            <span className="flex items-center gap-1.5">
              <PieChart size={14} className="text-emerald-400" />
              <span>{t('month_quota_usage')} {usedGB} GB / {monthlyQuotaGB} GB</span>
            </span>
            <span className="font-mono text-emerald-400 font-bold">{quotaPercent}%</span>
          </div>
          <div className="w-full bg-slate-800 rounded-full h-2 overflow-hidden">
            <div
              className={`h-full transition-all duration-500 rounded-full ${
                quotaPercent > 90 ? 'bg-rose-500' : quotaPercent > 75 ? 'bg-amber-500' : 'bg-emerald-500'
              }`}
              style={{ width: `${quotaPercent}%` }}
            />
          </div>
        </div>
      )}

      {/* Smart ISP & Operator AI Recommendation Card (Moved to bottom) */}
      <SmartISPOptimizerCard />

      {/* Mobile QR Code Exporter Modal */}
      {showQRModal && activeProfile && (
        <div className="modal-backdrop animate-fade-in" onClick={() => setShowQRModal(false)}>
          <div className="modal modal-sm text-center" onClick={e => e.stopPropagation()}>
            <div className="modal-header flex items-center justify-between pb-3 border-b border-white/5">
              <div className="flex items-center gap-2 font-bold text-slate-100 text-sm">
                <Smartphone size={16} className="text-cyan-400" />
                <span>{t('qr_code_modal_title')}</span>
              </div>
              <button onClick={() => setShowQRModal(false)} className="btn-icon">
                <X size={15} />
              </button>
            </div>

            <div className="p-4 flex flex-col items-center gap-3">
              <div className="p-3 bg-white rounded-2xl shadow-xl shadow-cyan-500/10">
                <img
                  src={generateQRCodeSVG(activeProfile.raw_uri || activeProfile.config_content || activeProfile.name, 190)}
                  alt="QR Code"
                  className="w-48 h-48"
                />
              </div>

              <div className="text-xs font-bold text-slate-200 flex items-center gap-1.5">
                <span>{getCountryFlag(activeProfile.country_code)}</span>
                <span>{activeProfile.name}</span>
              </div>

              <p className="text-[11px] text-slate-400 text-center leading-relaxed">
                {t('qr_code_modal_desc')}
              </p>

              <button
                type="button"
                onClick={() => {
                  navigator.clipboard.writeText(activeProfile.raw_uri || activeProfile.config_content)
                  setCopiedQRLink(true)
                  setTimeout(() => setCopiedQRLink(false), 2000)
                }}
                className="btn btn-secondary btn-sm w-full flex items-center justify-center gap-2 text-xs text-cyan-300 border-cyan-500/30 hover:bg-cyan-500/10"
              >
                {copiedQRLink ? <Check size={14} className="text-emerald-400" /> : <Copy size={14} />}
                <span>{copiedQRLink ? t('link_copied') : t('copy_config_link')}</span>
              </button>
            </div>
          </div>
        </div>
      )}

      {/* Modals */}
      <SpeedTestModal
        isOpen={showSpeedTestModal}
        onClose={() => setShowSpeedTestModal(false)}
      />

      <LANShareModal
        isOpen={showLANShareModal}
        onClose={() => setShowLANShareModal(false)}
      />

      <RadarModal
        isOpen={showRadarModal}
        onClose={() => setShowRadarModal(false)}
      />

      <EmergencyRepairModal
        isOpen={showEmergencyModal}
        onClose={() => setShowEmergencyModal(false)}
      />

      <QuotaManagerModal
        isOpen={showQuotaModal}
        onClose={() => setShowQuotaModal(false)}
        stats={stats}
      />

      <CloudSyncModal
        isOpen={showCloudModal}
        onClose={() => setShowCloudModal(false)}
      />

      <DNSLeakModal
        isOpen={showDNSLeakModal}
        onClose={() => setShowDNSLeakModal(false)}
      />

      <ConnectionHistoryModal
        isOpen={showHistoryModal}
        onClose={() => setShowHistoryModal(false)}
      />

      <FullBackupModal
        isOpen={showBackupModal}
        onClose={() => setShowBackupModal(false)}
        onSuccess={() => {
          loadDashboardData()
          showToast('Backup restored successfully!')
        }}
      />

      <OnboardingWizardModal
        isOpen={showOnboarding}
        onClose={() => setShowOnboarding(false)}
        onCompleted={() => {
          loadDashboardData()
          showToast('Setup completed! Welcome to NexVPN.')
        }}
      />

      <ProxyChainModal
        isOpen={showChainModal}
        onClose={() => setShowChainModal(false)}
        profiles={profiles}
        onSuccess={() => {
          loadDashboardData()
          showToast('Chained node created successfully!')
        }}
      />

      <MobileRemoteModal
        isOpen={showRemoteModal}
        onClose={() => setShowRemoteModal(false)}
      />

      <FragmentStudioModal
        isOpen={showFragmentStudio}
        onClose={() => setShowFragmentStudio(false)}
        settings={currentSettings}
        onSettingsUpdated={updated => {
          setCurrentSettings(updated)
          showToast('Anti-DPI settings saved!')
        }}
      />

      <GamingModeModal
        isOpen={showGamingModal}
        onClose={() => setShowGamingModal(false)}
        settings={currentSettings}
        onSettingsUpdated={updated => {
          setCurrentSettings(updated)
          setGameMode(updated.game_mode_enabled || false)
          showToast('Gaming Accelerator updated!')
        }}
      />

      <ScreenQRScannerModal
        isOpen={showScreenQRModal}
        onClose={() => setShowScreenQRModal(false)}
        onProfileImported={loadDashboardData}
      />

      <ConfigConverterModal
        isOpen={showConverterModal}
        onClose={() => setShowConverterModal(false)}
      />

      <NearbySyncModal
        isOpen={showNearbyModal}
        onClose={() => setShowNearbyModal(false)}
        onProfilesImported={loadDashboardData}
      />
    </div>
  )
}
