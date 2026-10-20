import { useState, useEffect, useRef, useCallback, useMemo, Suspense, lazy } from 'react'
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
  Gauge,
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
  X,
  History,
  Archive,
  Link2,
  AlertTriangle,
  Search,
  SortAsc,
  Scan,
  ArrowRightLeft,
  ChevronDown,
} from 'lucide-react'
import { api } from '../lib/api'
import { t } from '../lib/i18n'
import { translateBackendError } from '../lib/backendErrors'
import { sound } from '../lib/sound'
import { requestNotificationPermission } from '../lib/notification'
import { generateQRCodeSVG } from '../lib/qrcode'
import type { ConnectionStatus, TrafficStats, Settings, IPInfo, Profile } from '../types'
import {
  cn,
  formatBytes,
  formatSpeed,
  formatDuration,
  getProtocolName,
  getProtocolEmoji,
} from '../lib/utils'
import CountryFlag, { cleanServerName } from '../components/CountryFlag'
import { usePingProgress } from '../hooks/usePingProgress'
import { pinnedFirst } from '../lib/profilePin'

// Lazy-loaded modal components for performance and small bundle size
const SpeedTestModal = lazy(() => import('../components/SpeedTestModal'))
const LANShareModal = lazy(() => import('../components/LANShareModal'))
const RadarModal = lazy(() => import('../components/RadarModal'))
const EmergencyRepairModal = lazy(() => import('../components/EmergencyRepairModal'))
const QuotaManagerModal = lazy(() => import('../components/QuotaManagerModal'))
const CloudSyncModal = lazy(() => import('../components/CloudSyncModal'))
const DNSLeakModal = lazy(() => import('../components/DNSLeakModal'))
const ConnectionHistoryModal = lazy(() => import('../components/ConnectionHistoryModal'))
const FullBackupModal = lazy(() => import('../components/FullBackupModal'))
const OnboardingWizardModal = lazy(() => import('../components/OnboardingWizardModal'))
const ProxyChainModal = lazy(() => import('../components/ProxyChainModal'))
const MobileRemoteModal = lazy(() => import('../components/MobileRemoteModal'))
const FragmentStudioModal = lazy(() => import('../components/FragmentStudioModal'))
const GamingModeModal = lazy(() => import('../components/GamingModeModal'))
const ScreenQRScannerModal = lazy(() => import('../components/ScreenQRScannerModal'))
const ConfigConverterModal = lazy(() => import('../components/ConfigConverterModal'))
const NearbySyncModal = lazy(() => import('../components/NearbySyncModal'))
const ProfileQRModal = lazy(() => import('../components/ProfileQRModal'))
const ServiceUnlockModal = lazy(() => import('../components/ServiceUnlockModal'))
const GlobeWidget = lazy(() => import('../components/GlobeWidget'))

import SmartISPOptimizerCard from '../components/SmartISPOptimizerCard'
import FloatingMiniWidget from '../components/FloatingMiniWidget'
import CyberWaveVisualizer from '../components/CyberWaveVisualizer'
import { ModalShell } from '../components/ui/modal-shell'
import { Button } from '../components/ui/button'
import { Progress } from '../components/ui/progress'
import { useGlobalHotkeys } from '../hooks/useGlobalHotkeys'
import { useTimers } from '../lib/useTimers'
import { Card } from '@/components/ui/card'

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
  const [selectedCountry, setSelectedCountry] = useState<string | null>(null)
  const containerRef = useRef<HTMLDivElement>(null)

  // Extract unique countries from available profiles
  const countries = useMemo(() => {
    const counts: Record<string, number> = {}
    profiles.forEach(p => {
      let code = (p.country_code || '').toUpperCase().trim()
      if (!code || code.length !== 2) {
        const match = p.name.match(/\b([A-Z]{2})\b/)
        if (match) code = match[1].toUpperCase()
      }
      if (code && code.length === 2) {
        counts[code] = (counts[code] || 0) + 1
      }
    })
    return Object.entries(counts)
      .map(([code, count]) => ({ code, count }))
      .sort((a, b) => b.count - a.count)
  }, [profiles])

  const filtered = useMemo(() => {
    let list = [...profiles]
    if (selectedCountry) {
      list = list.filter(p => {
        let code = (p.country_code || '').toUpperCase().trim()
        if (!code || code.length !== 2) {
          const match = p.name.match(/\b([A-Z]{2})\b/)
          if (match) code = match[1].toUpperCase()
        }
        return code === selectedCountry
      })
    }
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
    // Pinned profiles lead here too, so the star set on the list pages is
    // visible in the one place the user picks a server from. Applied last so
    // the chosen sort order survives within each group — see lib/profilePin.ts.
    list.sort(pinnedFirst)
    return list
  }, [profiles, selectedCountry, query, sort])

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
        className="w-full flex items-center gap-2 px-3 py-2 rounded-xl bg-card border border-border hover:border-primary/50 transition text-xs font-semibold text-foreground text-start rtl:text-end"
      >
        <Server size={13} className="text-primary shrink-0" />
        <span className="flex-1 truncate flex items-center gap-2">
          {selected ? (
            <>
              <CountryFlag codeOrName={selected.country_code || selected.name} size="xs" />
              <span className="truncate">{cleanServerName(selected.name)}</span>
              {selected.latency_ms ? <span className="text-[10px] text-muted-foreground font-mono">({selected.latency_ms}ms)</span> : null}
            </>
          ) : (
            t('select_server_prompt')
          )}
        </span>
        <span className="text-muted-foreground text-[9px]">▼</span>
      </button>

      {open && (
        <div className="absolute z-50 bottom-full mb-1.5 w-full rounded-2xl bg-background backdrop-blur-xl border border-border shadow-2xl shadow-black/80 overflow-hidden animate-in fade-in slide-in-from-bottom-2 duration-150">
          {/* Search + sort row */}
          <div className="flex items-center gap-1.5 p-2 border-b border-border">
            <div className="flex items-center gap-1 flex-1 bg-card border border-border rounded-xl px-2 py-1">
              <Search size={12} className="text-muted-foreground shrink-0" />
              <input
                autoFocus
                value={query}
                onChange={e => setQuery(e.target.value)}
                placeholder={t('search') || 'Search profiles...'}
                className="flex-1 bg-transparent text-xs text-foreground outline-none placeholder-muted-foreground"
              />
              {query && (
                <button aria-label="Clear search" onClick={() => setQuery('')} className="text-muted-foreground hover:text-muted-foreground">
                  <X size={11} />
                </button>
              )}
            </div>
            <button
              onClick={() => setSort(s => s === 'latency' ? 'name' : s === 'name' ? 'default' : 'latency')}
              className="p-1.5 rounded-lg bg-muted hover:bg-muted/90 text-muted-foreground hover:text-primary transition text-[9px] font-bold flex items-center gap-1"
              title="Sort: default → latency → name"
            >
              <SortAsc size={12} />
              <span>{sort === 'latency' ? 'ms' : sort === 'name' ? 'A-Z' : '—'}</span>
            </button>
          </div>

          {/* Quick Country Filter Pills */}
          {countries.length > 0 && (
            <div className="flex items-center gap-1 px-2 py-1.5 border-b border-border overflow-x-auto no-scrollbar bg-card">
              <button
                type="button"
                onClick={() => setSelectedCountry(null)}
                className={`px-2 py-0.5 rounded-lg text-[10px] font-bold shrink-0 transition flex items-center gap-1 ${
                  selectedCountry === null
                    ? 'bg-primary text-primary-foreground font-extrabold shadow-sm'
                    : 'bg-muted text-muted-foreground hover:text-foreground border border-border'
                }`}
              >
                <span>{t('all_countries')}</span>
                <span className="opacity-75 text-[9px]">({profiles.length})</span>
              </button>
              {countries.map(({ code, count }) => (
                <button
                  key={code}
                  type="button"
                  onClick={() => setSelectedCountry(selectedCountry === code ? null : code)}
                  className={`flex items-center gap-1 px-2 py-0.5 rounded-lg text-[10px] font-medium shrink-0 transition ${
                    selectedCountry === code
                      ? 'bg-primary text-primary-foreground font-bold shadow-sm'
                      : 'bg-muted text-muted-foreground hover:text-primary border border-border'
                  }`}
                >
                  <CountryFlag codeOrName={code} size="xs" />
                  <span>{code}</span>
                  <span className="opacity-70 text-[9px]">({count})</span>
                </button>
              ))}
            </div>
          )}

          {/* Profile list */}
          <div className="max-h-52 overflow-y-auto">
            {filtered.length === 0 ? (
              <div className="px-3 py-4 text-center text-xs text-muted-foreground">{t('no_results') || 'No matching profiles'}</div>
            ) : (
              filtered.map(p => (
                <button
                  key={p.id}
                  type="button"
                  onClick={() => { onSelect(p.id); setOpen(false); setQuery('') }}
                  className={`w-full flex items-center gap-2 px-3 py-2 text-xs text-start rtl:text-end transition hover:bg-muted ${
                    p.id === selectedProfileId ? 'bg-info/10 text-primary' : 'text-muted-foreground'
                  }`}
                >
                  <CountryFlag codeOrName={p.country_code || p.name} size="xs" className="shrink-0" />
                  <span className="flex-1 truncate font-semibold">{cleanServerName(p.name)}</span>
                  {p.group && <span className="text-[10px] text-muted-foreground truncate max-w-[60px]">{p.group}</span>}
                  {p.latency_ms && p.latency_ms > 0 && (
                    <span className={`text-[10px] font-mono font-bold shrink-0 ${
                      p.latency_ms < 150 ? 'text-success' : p.latency_ms < 300 ? 'text-warning' : 'text-destructive'
                    }`}>{p.latency_ms}ms</span>
                  )}
                </button>
              ))
            )}
          </div>

          {filtered.length < profiles.length && (
            <div className="px-3 py-1.5 text-center text-[10px] text-muted-foreground border-t border-border">
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
}

export default function Dashboard({ status, stats, onOpenDoctor }: Props) {
  const [selectedProfileId, setSelectedProfileId] = useState<string>('')
  const [profiles, setProfiles] = useState<Profile[]>([])
  const [isActioning, setIsActioning] = useState(false)
  const [realDelay, setRealDelay] = useState<number | null>(null)
  const [isTestingDelay, setIsTestingDelay] = useState(false)
  const [proxyMode, setProxyMode] = useState<string>('tun')
  const [gameMode, setGameMode] = useState<boolean>(false)
  const [ipInfo, setIpInfo] = useState<IPInfo | null>(null)
  const [isLoadingIP, setIsLoadingIP] = useState(false)
  const [copiedIP, setCopiedIP] = useState(false)
  const [isPingingAll, setIsPingingAll] = useState(false)
  // The sweep runs over every category at once, so it can take a while and the
  // only feedback used to be the button's spinner.
  const pingProgress = usePingProgress(isPingingAll)
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
  const [showServiceUnlockModal, setShowServiceUnlockModal] = useState(false)
  const [showFloatingWidget, setShowFloatingWidget] = useState(false)
  const [showToolsDropdown, setShowToolsDropdown] = useState(false)
  const [currentSettings, setCurrentSettings] = useState<Settings>({} as Settings)
  const safeTimeout = useTimers()

  const showToast = (msg: string) => {
    setDashboardToast(msg)
    safeTimeout(() => setDashboardToast(null), 4000)
  }

  const handlePanicWipe = async () => {
    if (window.confirm(t('panic_wipe_confirm'))) {
      try {
        await api.panicWipe()
        showToast(t('panic_wipe_success'))
        sound.playDisconnect()
      } catch (err) {
        console.error(err)
      }
    }
  }

  const [connectingElapsed, setConnectingElapsed] = useState(0)

  // Track connection attempt elapsed time
  useEffect(() => {
    let timer: ReturnType<typeof setInterval> | null = null
    if (status.state === 'connecting') {
      setConnectingElapsed(0)
      timer = setInterval(() => {
        setConnectingElapsed(prev => prev + 1)
      }, 1000)
    } else {
      setConnectingElapsed(0)
    }
    return () => {
      if (timer) clearInterval(timer)
    }
  }, [status.state])

  // Listen to global open-modal events dispatched from Command Palette
  useEffect(() => {
    const handleOpenModal = (e: Event) => {
      const modal = (e as CustomEvent).detail
      if (modal === 'speedtest') setShowSpeedTestModal(true)
      else if (modal === 'lanshare') setShowLANShareModal(true)
      else if (modal === 'radar') setShowRadarModal(true)
      else if (modal === 'repair') setShowEmergencyModal(true)
      else if (modal === 'quota') setShowQuotaModal(true)
      else if (modal === 'cloud') setShowCloudModal(true)
      else if (modal === 'dnsleak') setShowDNSLeakModal(true)
      else if (modal === 'history') setShowHistoryModal(true)
      else if (modal === 'backup') setShowBackupModal(true)
      else if (modal === 'chain') setShowChainModal(true)
      else if (modal === 'remote') setShowRemoteModal(true)
      else if (modal === 'fragment') setShowFragmentStudio(true)
      else if (modal === 'gaming') setShowGamingModal(true)
      else if (modal === 'screenqr') setShowScreenQRModal(true)
      else if (modal === 'converter') setShowConverterModal(true)
      else if (modal === 'nearby') setShowNearbyModal(true)
      else if (modal === 'serviceunlock') setShowServiceUnlockModal(true)
    }
    window.addEventListener('nexvpn:open-modal', handleOpenModal)
    return () => window.removeEventListener('nexvpn:open-modal', handleOpenModal)
  }, [])

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
    } catch (err) {
      console.error('Connect toggle error:', err)
      showToast(err instanceof Error ? err.message : t('error'))
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

  const handleCancelPingAll = () => {
    api.cancelPingAll().catch(() => {})
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

  // Shared row styling for the Network Tools menu, so its fourteen entries stay
  // visually identical instead of each carrying its own accent colour.
  const toolItemClass =
    'flex items-center gap-2 rounded-md p-2 text-start text-popover-foreground transition-colors hover:bg-accent hover:text-accent-foreground rtl:text-end'
  const toolIconClass = 'h-3.5 w-3.5 shrink-0 text-muted-foreground'

  return (
    <div className="page space-y-4 animate-fade-in">
      {/* Toast Notification */}
      {dashboardToast && (
        <div className="fixed top-4 left-1/2 -translate-x-1/2 z-50 bg-card border border-primary/50 shadow-2xl text-foreground text-xs px-4 py-2 rounded-xl flex items-center gap-2 backdrop-blur-md animate-fade-in">
          <Sparkles size={14} className="text-primary animate-pulse" />
          <span>{dashboardToast}</span>
        </div>
      )}
      {/* Action toolbar.
          Previously six buttons in four different hues, each filled or tinted:
          amber for Smart Connect, green for Gaming, amber again for Anti-DPI,
          green for Speed Test, cyan for Tools, red for Panic Wipe. Nothing was
          encoded by those colours — the same hue meant different things in
          adjacent buttons. Now only the primary action is filled, utilities are
          neutral, and the one genuinely destructive action is the only one
          that carries a status colour. */}
      <div className="surface-toolbar sticky top-0 z-30 flex flex-wrap items-center justify-between gap-2 px-3 py-2">
        <div className="flex items-center gap-2">
          <span
            aria-hidden="true"
            className={cn(
              'h-2 w-2 shrink-0 rounded-full',
              isConnected
                ? 'bg-success'
                : isConnecting
                ? 'animate-pulse bg-warning'
                : isError
                ? 'bg-destructive'
                : 'bg-muted-foreground/50'
            )}
          />
          <span className="text-xs font-medium text-foreground">{t('control_hub')}</span>
        </div>

        <div className="flex flex-wrap items-center gap-1.5">
          {profiles.length > 0 && !isConnected && (
            <Button size="sm" onClick={handleAutoConnectFastest} title={t('auto_connect_fastest')}>
              <Zap className="h-3.5 w-3.5" />
              <span>{t('smart_connect')}</span>
            </Button>
          )}

          <Button
            size="sm"
            variant="outline"
            onClick={() => setShowGamingModal(true)}
            title="Gaming Accelerator & Anti-Sanction DNS"
            className={gameMode ? 'border-primary bg-primary/10 text-primary' : undefined}
          >
            <Gamepad2 className="h-3.5 w-3.5" />
            <span>{t('gaming')}</span>
          </Button>

          <Button
            size="sm"
            variant="outline"
            onClick={() => setShowFragmentStudio(true)}
            title="DPI Evasion & TLS Fragment Studio"
          >
            <Sparkles className="h-3.5 w-3.5" />
            <span>{t('anti_dpi')}</span>
          </Button>

          <Button
            size="sm"
            variant="outline"
            onClick={() => setShowSpeedTestModal(true)}
            title={t('speed_test_btn')}
          >
            <Gauge className="h-3.5 w-3.5" />
            <span>{t('speed_test')}</span>
          </Button>

          {/* All Network Tools Dropdown Menu */}
          <div className="relative">
            <Button
              size="sm"
              variant="outline"
              onClick={() => setShowToolsDropdown(!showToolsDropdown)}
              title={t('more_tools')}
              aria-expanded={showToolsDropdown}
              aria-haspopup="menu"
              className={showToolsDropdown ? 'border-primary bg-primary/10 text-primary' : undefined}
            >
              <Wrench className="h-3.5 w-3.5" />
              <span>{t('tools')}</span>
              <ChevronDown
                className={cn(
                  'h-3 w-3 opacity-60 transition-transform',
                  showToolsDropdown && 'rotate-180'
                )}
              />
            </Button>

            {showToolsDropdown && (
              <>
                {/* A real <button> rather than a div with role="button": it is
                    reachable by keyboard and Escape/Enter dismiss it. */}
                <button
                  type="button"
                  aria-label={t('more_tools')}
                  className="fixed inset-0 z-30 cursor-default"
                  onClick={() => setShowToolsDropdown(false)}
                />
                <div
                  role="menu"
                  className="absolute z-40 mt-2 grid w-72 grid-cols-2 gap-0.5 rounded-xl border border-border bg-popover p-1.5 text-xs shadow-xl animate-fade-in ltr:end-0 rtl:start-0"
                >
                  {onOpenDoctor && (
                    <button
                      role="menuitem"
                      onClick={() => { setShowToolsDropdown(false); onOpenDoctor() }}
                      className={toolItemClass}
                    >
                      <ShieldCheck className={toolIconClass} />
                      <span className="truncate font-medium">{t('doctor')}</span>
                    </button>
                  )}

                  <button
                    role="menuitem"
                    onClick={() => { setShowToolsDropdown(false); setShowDNSLeakModal(true) }}
                    className={toolItemClass}
                  >
                    <Shield className={toolIconClass} />
                    <span className="truncate font-medium">{t('dns_leak')}</span>
                  </button>

                  <button
                    role="menuitem"
                    onClick={() => { setShowToolsDropdown(false); setShowRadarModal(true) }}
                    className={toolItemClass}
                  >
                    <Radio className={toolIconClass} />
                    <span className="truncate font-medium">{t('radar')}</span>
                  </button>

                  <button
                    role="menuitem"
                    onClick={() => { setShowToolsDropdown(false); setShowLANShareModal(true) }}
                    className={toolItemClass}
                  >
                    <Wifi className={toolIconClass} />
                    <span className="truncate font-medium">{t('wifi_share')}</span>
                  </button>

                  <button
                    role="menuitem"
                    onClick={() => { setShowToolsDropdown(false); setShowRemoteModal(true) }}
                    className={toolItemClass}
                  >
                    <Smartphone className={toolIconClass} />
                    <span className="truncate font-medium">{t('remote_hub')}</span>
                  </button>

                  <button
                    role="menuitem"
                    onClick={() => { setShowToolsDropdown(false); setShowChainModal(true) }}
                    className={toolItemClass}
                  >
                    <Link2 className={toolIconClass} />
                    <span className="truncate font-medium">{t('multi_hop')}</span>
                  </button>

                  <button
                    role="menuitem"
                    onClick={() => { setShowToolsDropdown(false); setShowQuotaModal(true) }}
                    className={toolItemClass}
                  >
                    <PieChart className={toolIconClass} />
                    <span className="truncate font-medium">{t('quota')}</span>
                  </button>

                  <button
                    role="menuitem"
                    onClick={() => { setShowToolsDropdown(false); setShowCloudModal(true) }}
                    className={toolItemClass}
                  >
                    <Cloud className={toolIconClass} />
                    <span className="truncate font-medium">{t('cloud_hub')}</span>
                  </button>

                  <button
                    role="menuitem"
                    onClick={() => { setShowToolsDropdown(false); setShowHistoryModal(true) }}
                    className={toolItemClass}
                  >
                    <History className={toolIconClass} />
                    <span className="truncate font-medium">{t('history')}</span>
                  </button>

                  <button
                    role="menuitem"
                    onClick={() => { setShowToolsDropdown(false); setShowBackupModal(true) }}
                    className={toolItemClass}
                  >
                    <Archive className={toolIconClass} />
                    <span className="truncate font-medium">{t('backup')}</span>
                  </button>

                  <button
                    role="menuitem"
                    onClick={() => { setShowToolsDropdown(false); setShowScreenQRModal(true) }}
                    className={toolItemClass}
                  >
                    <Scan className={toolIconClass} />
                    <span className="truncate font-medium">{t('screen_qr_title') || 'Scan QR'}</span>
                  </button>

                  <button
                    role="menuitem"
                    onClick={() => { setShowToolsDropdown(false); setShowConverterModal(true) }}
                    className={toolItemClass}
                  >
                    <ArrowRightLeft className={toolIconClass} />
                    <span className="truncate font-medium">{t('config_converter_title') || 'Converter'}</span>
                  </button>

                  <button
                    role="menuitem"
                    onClick={() => { setShowToolsDropdown(false); setShowServiceUnlockModal(true) }}
                    className={cn(toolItemClass, 'col-span-2')}
                  >
                    <Activity className={toolIconClass} />
                    <span className="truncate font-medium">{t('service_unblock_btn') || 'AI & Service Unlock Matrix'}</span>
                  </button>

                  <button
                    role="menuitem"
                    onClick={() => { setShowToolsDropdown(false); setShowNearbyModal(true) }}
                    className={cn(toolItemClass, 'col-span-2')}
                  >
                    <Radio className={toolIconClass} />
                    <span className="truncate font-medium">{t('nearby_share_title') || 'Nearby Wi-Fi Share (P2P)'}</span>
                  </button>

                  <button
                    role="menuitem"
                    onClick={() => { setShowToolsDropdown(false); setShowFloatingWidget(prev => !prev) }}
                    className={cn(toolItemClass, 'col-span-2')}
                  >
                    <Zap className={toolIconClass} />
                    <span className="truncate font-medium">{t('floating_desktop_widget')}</span>
                  </button>

                  <button
                    role="menuitem"
                    onClick={() => { setShowToolsDropdown(false); setShowEmergencyModal(true) }}
                    className={cn(
                      toolItemClass,
                      'col-span-2 mt-0.5 border-t border-border text-destructive hover:bg-destructive/10 hover:text-destructive'
                    )}
                  >
                    <Wrench className="h-3.5 w-3.5 shrink-0" />
                    <span className="truncate font-medium">{t('emergency_repair')}</span>
                  </button>
                </div>
              </>
            )}
          </div>

          {/* Panic Wipe — the only toolbar action that keeps a status colour,
              because it is the only one that destroys data. */}
          <Button
            size="sm"
            variant="outline"
            onClick={handlePanicWipe}
            title="Instantly Disconnect, Wipe Sessions & Kill System Proxy"
            className="border-destructive/30 text-destructive hover:bg-destructive/10 hover:text-destructive"
          >
            <AlertTriangle className="h-3.5 w-3.5" />
            <span>{t('panic_wipe')}</span>
          </Button>
        </div>
      </div>

      {/* Hero Layout: Connection Orb on Left, Live Chart & Selector on Right */}
      <div className="grid grid-cols-1 lg:grid-cols-12 gap-5">
        {/* Connection Core Card */}
        <Card className="lg:col-span-5 flex flex-col items-center justify-between p-6 text-center relative">
          {/* Top Info Header */}
          <div className="w-full flex items-center justify-between text-xs text-muted-foreground mb-2">
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
                  size={44}
                  className={`transition-colors duration-300 ${
                    isConnected
                      ? 'text-success'
                      : isConnecting
                      ? 'animate-spin text-warning'
                      : isError
                      ? 'text-destructive'
                      : 'text-muted-foreground'
                  }`}
                />
                <span
                  className={`connect-btn-label text-xs font-medium tracking-wide transition-colors ${
                    isConnected
                      ? 'text-success'
                      : isConnecting
                      ? 'text-warning'
                      : isError
                      ? 'text-destructive'
                      : 'text-muted-foreground'
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
                {isConnecting && (
                  <span className="text-[10px] font-mono font-medium text-warning -mt-0.5 flex items-center gap-1">
                    <span className="w-1.5 h-1.5 rounded-full bg-warning animate-ping inline-block" />
                    <span>{connectingElapsed}s</span>
                  </span>
                )}
                {isConnected && (
                  <span className="text-[10px] font-mono text-success/90 -mt-1 flex items-center gap-1">
                    <span className="w-1.5 h-1.5 rounded-full bg-success animate-ping inline-block" />
                    {t('active_status')}
                  </span>
                )}
              </button>
            </div>
          </div>

          {/* Connection Duration & Real Delay Status */}
          {isConnected ? (
            <div className="w-full flex flex-col items-center gap-2 pt-2 border-t border-border">
              <div className="flex items-center gap-2 font-mono text-xl font-extrabold text-foreground">
                <Clock size={16} className="text-primary" />
                <span>{formatDuration(status.duration || 0)}</span>
              </div>

              <div className="flex items-center gap-2">
                <Button variant="secondary" size="sm" className="text-[11px] py-1 px-2.5"
                  onClick={handleTestRealDelay}
                  disabled={isTestingDelay}
                >
                  <Radio size={12} className="text-success" />
                  {isTestingDelay ? t('testing_delay') : t('real_delay_test')}
                </Button>
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
              <div className="w-full p-3 rounded-xl bg-muted/50 border border-border flex items-center justify-between gap-2 mt-1">
                <div className="flex items-center gap-3 min-w-0 text-start rtl:text-end">
                  <CountryFlag codeOrName={activeProfile?.country_code || activeProfile?.name} size="md" />
                  <div className="min-w-0">
                    <div className="text-xs font-medium text-foreground truncate" title={activeProfile?.name || status.profile_name}>
                      {cleanServerName(activeProfile?.name || status.profile_name) || 'Active Node'}
                    </div>
                    <div className="text-[10px] font-mono text-muted-foreground truncate">
                      {activeProfile?.server || status.server || 'Protected Gateway'}
                    </div>
                  </div>
                </div>
                <span className="text-[10px] font-mono font-medium px-2 py-0.5 rounded-md bg-muted text-muted-foreground border border-border shrink-0">
                  {activeProfile?.protocol ? getProtocolName(activeProfile.protocol) : status.protocol ? getProtocolName(status.protocol) : 'VPN Node'}
                </span>
              </div>

              {healthStatus && (
                <div className="flex items-center gap-2 px-3 py-1.5 rounded-xl segmented-pill text-xs font-mono mt-0.5 shadow-sm">
                  <span
                    className={`w-2 h-2 rounded-full ${
                      healthStatus.quality_score >= 80
                        ? 'bg-success animate-pulse'
                        : healthStatus.quality_score >= 50
                        ? 'bg-warning'
                        : 'bg-destructive'
                    }`}
                  />
                  <span className="text-foreground dark:text-muted-foreground font-bold">Quality: {healthStatus.quality_score}%</span>
                  <span className="text-muted-foreground">•</span>
                  <span className="text-muted-foreground capitalize">{healthStatus.quality_level}</span>
                  {healthStatus.jitter_ms > 0 && (
                    <>
                      <span className="text-muted-foreground">•</span>
                      <span className="text-muted-foreground">Jitter {healthStatus.jitter_ms}ms</span>
                    </>
                  )}
                </div>
              )}
            </div>
          ) : (
            /* Server Selector with Search, 1-Click Ping All and Mobile QR Exporter */
            <div className="w-full space-y-2.5 pt-2 border-t border-border">
              {/* Connection Failure Error Diagnostic Banner */}
              {isError && status.error_msg && (
                <div className="w-full p-3.5 rounded-2xl error-banner-modern text-start rtl:text-end space-y-1.5 animate-in fade-in zoom-in-95 duration-200 shadow-sm">
                  <div className="flex items-center justify-between gap-2">
                    <div className="flex items-center gap-2 text-destructive font-bold text-xs">
                      <AlertTriangle size={15} className="text-destructive shrink-0 animate-pulse" />
                      <span>{t('connection_failed')}</span>
                    </div>
                    <button
                      onClick={() => setShowEmergencyModal(true)}
                      className="text-[10px] text-primary hover:underline flex items-center gap-1 font-semibold"
                    >
                      🩺 {t('open_doctor')}
                    </button>
                  </div>
                  <p
                    className="text-xs text-destructive leading-relaxed font-sans font-medium"
                    title={status.error_msg}
                  >
                    {translateBackendError(status.error_msg)}
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
                <Button variant="secondary" size="sm" className="px-2 text-xs font-bold text-warning border-warning/30 hover:bg-warning/10 flex items-center gap-1 whitespace-nowrap"
                  type="button"
                  onClick={handlePingAllAndPickBest}
                  disabled={isPingingAll || profiles.length === 0}
                  title={t('ping_all_btn')}
                >
                  <Zap size={13} className={isPingingAll ? 'animate-spin text-warning' : 'fill-warning'} />
                  <span>
                    {isPingingAll
                      ? pingProgress
                        ? `${pingProgress.done}/${pingProgress.total}`
                        : '...'
                      : t('ping_all_btn')}
                  </span>
                </Button>

                {isPingingAll && (
                  <Button variant="secondary" size="sm" className="p-1.5 text-muted-foreground hover:text-destructive"
                    type="button"
                    onClick={handleCancelPingAll}
                    title={t('adv_ping_cancel')}
                  >
                    <X size={14} />
                  </Button>
                )}

                <Button variant="secondary" size="sm" className="p-1.5 text-secondary hover:text-primary hover:border-primary/40"
                  type="button"
                  onClick={() => setShowQRModal(true)}
                  disabled={!selectedProfileId}
                  title={t('qr_code_modal_title')}
                >
                  <QrCode size={14} />
                </Button>
              </div>
            </div>
          )}
        </Card>

        {/* Right Side: Speed Graph & Mode Switches */}
        <div className="lg:col-span-7 flex flex-col gap-4">
          {/* Proxy Mode Segmented Bar */}
          <Card className="flex flex-wrap items-center justify-between gap-2 p-3">
            <div className="flex items-center gap-2 px-2 text-xs font-medium text-muted-foreground">
              <Layers size={15} className="text-muted-foreground" />
              <span>{t('proxy_mode_label')}</span>
            </div>

            <div className="segmented-pill flex gap-1">
              <button
                type="button"
                onClick={() => handleModeChange('tun')}
                aria-pressed={proxyMode === 'tun'}
                className={`rounded-md px-3 py-1.5 text-xs font-medium transition-colors ${
                  proxyMode === 'tun'
                    ? 'bg-primary text-primary-foreground'
                    : 'text-muted-foreground hover:text-foreground'
                }`}
              >
                {t('mode_switch_tun')}
              </button>

              <button
                type="button"
                onClick={() => handleModeChange('system_proxy')}
                aria-pressed={proxyMode === 'system_proxy'}
                className={`rounded-md px-3 py-1.5 text-xs font-medium transition-colors ${
                  proxyMode === 'system_proxy'
                    ? 'bg-primary text-primary-foreground'
                    : 'text-muted-foreground hover:text-foreground'
                }`}
              >
                {t('mode_switch_proxy')}
              </button>
            </div>
          </Card>

          {/* Interactive Globe / Network Map Card */}
          <Card className="flex-1 min-h-[220px] relative overflow-hidden flex items-center justify-center">
            <Suspense fallback={
              <div className="flex flex-col items-center justify-center gap-2 text-muted-foreground text-xs">
                <Globe className="w-8 h-8 animate-spin text-primary/50" />
                <span>{t('loading_map')}</span>
              </div>
            }>
              <GlobeWidget 
                connected={isConnected} 
                serverCountry={activeProfile?.country_code || ipInfo?.country_code} 
                serverName={activeProfile?.name || status.profile_name}
                serverIp={activeProfile?.server || status.server || ipInfo?.ip}
                clientCountry={ipInfo?.country_code}
                clientIp={ipInfo?.ip}
              />
            </Suspense>
          </Card>

          {/* Real-time Cyber Traffic & Bandwidth Monitor */}
          <Card className="p-4 flex flex-col gap-3 shrink-0 shadow-md">
            <div className="flex items-center justify-between">
              <div className="flex items-center gap-2 text-xs font-bold text-foreground">
                <Activity size={15} className="text-primary animate-pulse" />
                <span>{t('real_time_traffic')}</span>
              </div>
              <div className="flex items-center gap-4 text-xs font-mono">
                <div className="flex items-center gap-1.5 px-2.5 py-1 rounded-lg border border-border bg-muted text-muted-foreground font-bold">
                  <ArrowDown size={13} className="text-primary" /> 
                  <span>{formatSpeed(stats.speed_download || 0)}</span>
                </div>
                <div className="flex items-center gap-1.5 px-2.5 py-1 rounded-lg bg-success/10 border border-success/20 text-success font-bold">
                  <ArrowUp size={13} className="text-success" /> 
                  <span>{formatSpeed(stats.speed_upload || 0)}</span>
                </div>
              </div>
            </div>

            {/* Live Reactive Cyber Wave Oscilloscope */}
            <div className="h-20 w-full rounded-xl overflow-hidden shadow-inner">
              <CyberWaveVisualizer
                connected={isConnected}
                downloadSpeed={stats.speed_download || 0}
                uploadSpeed={stats.speed_upload || 0}
              />
            </div>
          </Card>
        </div>
      </div>

      {/* Live Real-Time Public IP & Network Security Card */}
      <Card className="p-4 relative overflow-hidden shadow-sm">
        <div className="flex items-center justify-between flex-wrap gap-3">
          <div className="flex items-center gap-3">
            <div className="shrink-0 flex items-center justify-center">
              <CountryFlag codeOrName={ipInfo?.country_code || (isConnected ? activeProfile?.country_code || 'US' : 'IR')} size="md" />
            </div>
            <div>
              <div className="flex items-center gap-2 flex-wrap">
                <span className="text-xs text-secondary font-semibold">
                  {isConnected ? t('safe_ip_label') : t('public_ip_label')}
                </span>
                <span className="font-mono font-extrabold text-sm text-foreground segmented-pill px-2.5 py-0.5 rounded-md flex items-center gap-1.5">
                  {isLoadingIP ? (
                    <span className="text-muted-foreground text-xs flex items-center gap-1">
                      <RefreshCw size={11} className="animate-spin text-primary" /> {t('querying')}
                    </span>
                  ) : ipInfo?.ip ? (
                    <>
                      <span>{ipInfo.ip}</span>
                      <button
                        type="button"
                        onClick={() => {
                          navigator.clipboard.writeText(ipInfo.ip)
                          setCopiedIP(true)
                          safeTimeout(() => setCopiedIP(false), 2000)
                        }}
                        className="text-muted-foreground hover:text-primary ms-1 transition"
                        title="Copy IP"
                      >
                        {copiedIP ? <Check size={12} className="text-success" /> : <Copy size={12} />}
                      </button>
                    </>
                  ) : isConnected ? (
                    <span className="text-success text-xs font-mono">{activeProfile?.server || 'Encrypted Gateway'}</span>
                  ) : (
                    <span className="text-muted-foreground text-xs">-</span>
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

              <div className="text-xs text-muted-foreground mt-1 flex items-center gap-2 flex-wrap">
                {ipInfo?.city && <span>📍 {ipInfo.city}, {ipInfo.country}</span>}
                {ipInfo?.isp && <span>🏢 {ipInfo.isp}</span>}
                {!ipInfo && !isLoadingIP && !isConnected && <span>{t('select_server_prompt')}</span>}
                {isConnected && !ipInfo && !isLoadingIP && (
                  <span className="text-primary text-xs font-medium">🔒 {activeProfile?.name || status.profile_name || 'Protected Node Active'}</span>
                )}
                {isConnected && status.server && ipInfo && (
                  <span className="text-muted-foreground text-[11px] font-mono">({status.server})</span>
                )}
              </div>
            </div>
          </div>

          <div className="flex items-center gap-2">
            <Button variant="secondary" size="sm" className="text-xs flex items-center gap-1.5 border-border hover:border-primary/40 text-muted-foreground hover:text-primary"
              type="button"
              onClick={() => fetchLiveIP()}
              disabled={isLoadingIP}
              title="Refresh IP"
            >
              <RefreshCw size={12} className={isLoadingIP ? 'animate-spin text-primary' : ''} />
              <span>{t('live_ip_query')}</span>
            </Button>
          </div>
        </div>
      </Card>

      {/* 4-Metrics Stats Row + Peak Speed & Quota Progress */}
      <div className="stats-grid">
        <Card className="flex flex-col gap-1 px-4 py-3.5">
          <div className="flex items-center gap-2">
            <ArrowDown size={14} className="text-success" />
            <span className="stat-label">{t('downloaded')}</span>
          </div>
          <div className="stat-value">{formatBytes(stats.bytes_received)}</div>
        </Card>

        <Card className="flex flex-col gap-1 px-4 py-3.5">
          <div className="flex items-center gap-2">
            <ArrowUp size={14} className="text-primary" />
            <span className="stat-label">{t('uploaded')}</span>
          </div>
          <div className="stat-value">{formatBytes(stats.bytes_sent)}</div>
        </Card>

        <Card className="flex flex-col gap-1 px-4 py-3.5">
          <div className="flex items-center gap-2">
            <TrendingUp size={14} className="text-warning" />
            <span className="stat-label">{t('peak_download_speed')}</span>
          </div>
          <div className="stat-value text-warning">{formatSpeed(peakDown)}</div>
        </Card>

        <Card className="flex flex-col gap-1 px-4 py-3.5">
          <div className="flex items-center gap-2">
            <Clock size={14} className="text-primary" />
            <span className="stat-label">{t('duration')}</span>
          </div>
          <div className="stat-value">{formatDuration(status.duration || 0)}</div>
        </Card>
      </div>

      {/* Quota Progress Banner (If configured) */}
      {monthlyQuotaGB > 0 && (
        <Card className="flex flex-col gap-2 p-3">
          <div className="flex items-center justify-between text-xs font-medium text-muted-foreground">
            <span className="flex items-center gap-1.5">
              <PieChart size={14} className="text-muted-foreground" />
              <span>{t('month_quota_usage')} {usedGB} GB / {monthlyQuotaGB} GB</span>
            </span>
            <span className="font-mono font-medium text-foreground">{quotaPercent}%</span>
          </div>
          <Progress
            value={quotaPercent}
            indicatorClassName={
              quotaPercent > 90 ? 'bg-destructive' : quotaPercent > 75 ? 'bg-warning' : 'bg-primary'
            }
          />
        </Card>
      )}

      {/* Smart ISP & Operator AI Recommendation Card (Moved to bottom) */}
      <SmartISPOptimizerCard />

      {/* Mobile QR Code Exporter Modal */}
      {showQRModal && activeProfile && (
        <ModalShell
          isOpen={showQRModal}
          onClose={() => setShowQRModal(false)}
          maxWidth="max-w-sm"
          bodyClassName="p-4 text-center"
        >
            <div className="flex items-center justify-between pb-3 border-b border-border">
              <div className="flex items-center gap-2 font-bold text-foreground text-sm">
                <Smartphone size={16} className="text-primary" />
                <span>{t('qr_code_modal_title')}</span>
              </div>
              <Button
                variant="ghost"
                size="icon"
                className="rounded-md text-muted-foreground"
                onClick={() => setShowQRModal(false)}
              >
                <X size={15} />
              </Button>
            </div>

            <div className="p-4 flex flex-col items-center gap-3">
              <div className="p-3 bg-card rounded-2xl shadow-xl">
                <img
                  src={generateQRCodeSVG(activeProfile.raw_uri || activeProfile.config_content || activeProfile.name, 190)}
                  alt="QR Code"
                  className="w-48 h-48"
                />
              </div>

              <div className="text-xs font-bold text-foreground flex items-center gap-1.5">
                <CountryFlag codeOrName={activeProfile.country_code || activeProfile.name} size="xs" />
                <span>{activeProfile.name}</span>
              </div>

              <p className="text-[11px] text-muted-foreground text-center leading-relaxed">
                {t('qr_code_modal_desc')}
              </p>

              <Button variant="secondary" size="sm" className="w-full flex items-center justify-center gap-2 text-xs text-primary border-primary/30 hover:bg-primary/10"
                type="button"
                onClick={() => {
                  navigator.clipboard.writeText(activeProfile.raw_uri || activeProfile.config_content)
                  setCopiedQRLink(true)
                  safeTimeout(() => setCopiedQRLink(false), 2000)
                }}
              >
                {copiedQRLink ? <Check size={14} className="text-success" /> : <Copy size={14} />}
                <span>{copiedQRLink ? t('link_copied') : t('copy_config_link')}</span>
              </Button>
            </div>
        </ModalShell>
      )}

      {/* Modals - lazy-loaded once, wrapped in Suspense (duplicate renders removed) */}
      <Suspense fallback={null}>
        {showSpeedTestModal && (
          <SpeedTestModal
            isOpen={showSpeedTestModal}
            onClose={() => setShowSpeedTestModal(false)}
          />
        )}
        {showLANShareModal && (
          <LANShareModal
            isOpen={showLANShareModal}
            onClose={() => setShowLANShareModal(false)}
          />
        )}
        {showRadarModal && (
          <RadarModal
            isOpen={showRadarModal}
            onClose={() => setShowRadarModal(false)}
          />
        )}
        {showEmergencyModal && (
          <EmergencyRepairModal
            isOpen={showEmergencyModal}
            onClose={() => setShowEmergencyModal(false)}
          />
        )}
        {showQuotaModal && (
          <QuotaManagerModal
            isOpen={showQuotaModal}
            onClose={() => setShowQuotaModal(false)}
            stats={stats}
          />
        )}
        {showCloudModal && (
          <CloudSyncModal
            isOpen={showCloudModal}
            onClose={() => setShowCloudModal(false)}
          />
        )}
        {showDNSLeakModal && (
          <DNSLeakModal
            isOpen={showDNSLeakModal}
            onClose={() => setShowDNSLeakModal(false)}
          />
        )}
        {showHistoryModal && (
          <ConnectionHistoryModal
            isOpen={showHistoryModal}
            onClose={() => setShowHistoryModal(false)}
          />
        )}
        {showBackupModal && (
          <FullBackupModal
            isOpen={showBackupModal}
            onClose={() => setShowBackupModal(false)}
            onSuccess={() => {
              loadDashboardData()
              showToast(t('backup_restored_success'))
            }}
          />
        )}
        {showOnboarding && (
          <OnboardingWizardModal
            isOpen={showOnboarding}
            onClose={() => setShowOnboarding(false)}
            onCompleted={() => {
              loadDashboardData()
              showToast(t('setup_completed_welcome'))
            }}
          />
        )}
        {showChainModal && (
          <ProxyChainModal
            isOpen={showChainModal}
            onClose={() => setShowChainModal(false)}
            profiles={profiles}
            onSuccess={() => {
              loadDashboardData()
              showToast(t('chain_created_success'))
            }}
          />
        )}
        {showRemoteModal && (
          <MobileRemoteModal
            isOpen={showRemoteModal}
            onClose={() => setShowRemoteModal(false)}
          />
        )}
        {showFragmentStudio && (
          <FragmentStudioModal
            isOpen={showFragmentStudio}
            onClose={() => setShowFragmentStudio(false)}
            settings={currentSettings}
            onSettingsUpdated={updated => {
              setCurrentSettings(updated)
              showToast(t('anti_dpi_saved'))
            }}
          />
        )}
        {showGamingModal && (
          <GamingModeModal
            isOpen={showGamingModal}
            onClose={() => setShowGamingModal(false)}
            settings={currentSettings}
            onSettingsUpdated={updated => {
              setCurrentSettings(updated)
              setGameMode(updated.game_mode_enabled || false)
              showToast(t('gaming_updated'))
            }}
          />
        )}
        {showScreenQRModal && (
          <ScreenQRScannerModal
            isOpen={showScreenQRModal}
            onClose={() => setShowScreenQRModal(false)}
            onProfileImported={loadDashboardData}
          />
        )}
        {showConverterModal && (
          <ConfigConverterModal
            isOpen={showConverterModal}
            onClose={() => setShowConverterModal(false)}
          />
        )}
        {showNearbyModal && (
          <NearbySyncModal
            isOpen={showNearbyModal}
            onClose={() => setShowNearbyModal(false)}
            onProfilesImported={loadDashboardData}
          />
        )}
        {showQRModal && (
          <ProfileQRModal
            profile={profiles.find(p => p.id === selectedProfileId) || (status.profile_id ? profiles.find(p => p.id === status.profile_id) || null : null)}
            isOpen={showQRModal}
            onClose={() => setShowQRModal(false)}
          />
        )}
        {showServiceUnlockModal && (
          <ServiceUnlockModal
            isOpen={showServiceUnlockModal}
            onClose={() => setShowServiceUnlockModal(false)}
          />
        )}
      </Suspense>

      {showFloatingWidget && (
        <FloatingMiniWidget
          status={status}
          stats={stats}
          profiles={profiles}
          onSelectProfile={handleSelectProfile}
          onToggleConnect={handleConnectToggle}
          onConnect={handleConnectToggle}
          onDisconnect={handleConnectToggle}
          onCloseWidget={() => setShowFloatingWidget(false)}
          onHideToTray={() => api.hideWindow()}
          onOpenMainWindow={() => api.showWindow()}
        />
      )}
    </div>
  )
}
