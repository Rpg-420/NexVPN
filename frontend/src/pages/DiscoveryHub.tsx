import { useState, useEffect, useMemo } from 'react'
import {
  Compass,
  Send,
  Globe,
  Zap,
  RefreshCw,
  Play,
  Square,
  Copy,
  Check,
  QrCode,
  Download,
  CheckCircle2,
  XCircle,
  Clock,
  Search,
  ExternalLink,
  ShieldCheck,
  Layers,
  Sparkles,
  Sliders,
  Share2,
  CheckSquare,
  Square as SquareOutline,
  X,
  Activity,
  ArrowDownToLine,
  Shield,
  Flame,
  Award,
  Trash2,
} from 'lucide-react'
import { api } from '../lib/api'
import { t, getLanguage } from '../lib/i18n'
import { sound } from '../lib/sound'
import { generateQRCodeSVG } from '../lib/qrcode'
import CountryFlag, { cleanServerName } from '../components/CountryFlag'
import { ModalShell } from '../components/ui/modal-shell'
import type {
  TelegramProxy,
  TelegramTestProgress,
  FreeConfigItem,
  FreeConfigTestProgress,
  CloudflareTarget,
  CloudflareScanProgress,
  FragmentCandidate,
  FragmentScanProgress,
  ConnectionStatus,
} from '../types'

interface Props {
  onStatusChange?: (s: ConnectionStatus) => void
}

type DiscoveryTab = 'telegram' | 'configs' | 'cloudflare' | 'fragment'

export default function DiscoveryHub(_props: Props) {
  const [activeTab, setActiveTab] = useState<DiscoveryTab>('telegram')

  // --- Telegram Proxies State ---
  const [tgProxies, setTgProxies] = useState<TelegramProxy[]>(() => {
    try {
      const saved = localStorage.getItem('nexvpn_cached_tg_proxies')
      if (saved) {
        const parsed = JSON.parse(saved)
        if (Array.isArray(parsed)) return parsed
      }
    } catch {
      // Corrupt cache is non-fatal; fall through to the empty default.
    }
    return []
  })
  const [isFetchingTg, setIsFetchingTg] = useState(false)
  const [isTestingTg, setIsTestingTg] = useState(false)
  const [tgProgress, setTgProgress] = useState<TelegramTestProgress | null>(null)
  const [tgSearch, setTgSearch] = useState('')
  const [tgTypeFilter, setTgTypeFilter] = useState<'all' | 'mtproto' | 'socks5' | 'http'>('all')
  const [tgStatusFilter, setTgStatusFilter] = useState<'all' | 'alive'>('all')
  const [customTgSources, setCustomTgSources] = useState<string[]>(() => {
    try {
      const saved = localStorage.getItem('nexvpn_custom_tg_sources')
      return saved ? JSON.parse(saved) : []
    } catch {
      return []
    }
  })

  // --- Free Configs State ---
  const [freeConfigs, setFreeConfigs] = useState<FreeConfigItem[]>(() => {
    try {
      const saved = localStorage.getItem('nexvpn_cached_free_configs')
      if (saved) {
        const parsed = JSON.parse(saved)
        if (Array.isArray(parsed) && parsed.length > 0) return parsed
      }
    } catch {
      // Corrupt cache is non-fatal; fall through to the empty default.
    }
    return []
  })
  const [isFetchingConfigs, setIsFetchingConfigs] = useState(false)
  const [isTestingConfigs, setIsTestingConfigs] = useState(false)
  const [configsProgress, setConfigsProgress] = useState<FreeConfigTestProgress | null>(null)
  const [configSearch, setConfigSearch] = useState('')
  const [configProtoFilter, setConfigProtoFilter] = useState<string>('all')
  const [selectedConfigIds, setSelectedConfigIds] = useState<string[]>([])
  const [isImportingConfigs, setIsImportingConfigs] = useState(false)
  const [customConfigSources, setCustomConfigSources] = useState<string[]>(() => {
    try {
      const saved = localStorage.getItem('nexvpn_custom_cfg_sources')
      return saved ? JSON.parse(saved) : []
    } catch {
      return []
    }
  })

  // --- Cloudflare Scanner State ---
  const [cfMode, setCfMode] = useState<'warp' | 'cdn'>('warp')
  const [cfIspFilter, setCfIspFilter] = useState<'all' | 'mci' | 'mtn' | 'rtl' | 'fixed'>('all')
  const [isCfScanning, setIsCfScanning] = useState(false)
  const [cfResults, setCfResults] = useState<CloudflareTarget[]>(() => {
    try {
      const saved = localStorage.getItem('nexvpn_cached_cf_results')
      if (saved) {
        const parsed = JSON.parse(saved)
        if (Array.isArray(parsed) && parsed.length > 0) return parsed
      }
    } catch {
      // Corrupt cache is non-fatal; fall through to the empty default.
    }
    return []
  })
  const [cfProgress, setCfProgress] = useState<CloudflareScanProgress | null>(null)
  const [isApplyingWarp, setIsApplyingWarp] = useState(false)
  const [isApplyingV2Ray, setIsApplyingV2Ray] = useState(false)

  // Save states across navigation
  useEffect(() => {
    try {
      localStorage.setItem('nexvpn_cached_tg_proxies', JSON.stringify(tgProxies))
    } catch {
      // Persisting the cache is best-effort (storage quota / private mode).
    }
  }, [tgProxies])

  useEffect(() => {
    try {
      localStorage.setItem('nexvpn_cached_free_configs', JSON.stringify(freeConfigs))
    } catch {
      // Persisting the cache is best-effort (storage quota / private mode).
    }
  }, [freeConfigs])

  useEffect(() => {
    try {
      localStorage.setItem('nexvpn_cached_cf_results', JSON.stringify(cfResults))
    } catch {
      // Persisting the cache is best-effort (storage quota / private mode).
    }
  }, [cfResults])

  // --- Fragment Scanner State ---
  const [isFragScanning, setIsFragScanning] = useState(false)
  const [fragProgress, setFragProgress] = useState<FragmentScanProgress | null>(null)
  const [fragResults, setFragResults] = useState<FragmentCandidate[]>([])
  const [bestFragment, setBestFragment] = useState<FragmentCandidate | null>(null)
  const [isApplyingFrag, setIsApplyingFrag] = useState(false)

  // --- Shared Modals & Toast State ---
  const [qrModalData, setQrModalData] = useState<{ title: string; link: string; qrSvg: string } | null>(null)
  const [sourcesModalTab, setSourcesModalTab] = useState<'telegram' | 'configs' | null>(null)
  const [sourcesInputText, setSourcesInputText] = useState('')
  const [toastMessage, setToastMessage] = useState<string | null>(null)
  const [copiedId, setCopiedId] = useState<string | null>(null)

  const showToast = (msg: string) => {
    setToastMessage(msg)
    setTimeout(() => setToastMessage(null), 3000)
  }

  const copyToClipboard = (text: string, idKey: string, toastTxt?: string) => {
    navigator.clipboard.writeText(text)
    setCopiedId(idKey)
    showToast(toastTxt || t('copied_to_clipboard'))
    setTimeout(() => setCopiedId(null), 2000)
  }

  // ==========================================
  // 1. Telegram Proxies Handlers
  // ==========================================
  const handleFetchTelegramProxies = async () => {
    setIsFetchingTg(true)
    try {
      const list = await api.fetchTelegramProxies(customTgSources)
      if (Array.isArray(list)) {
        setTgProxies(prev => {
          const existingMap = new Set(prev.map(p => `${p.server}:${p.port}`))
          const newOnes = list.filter(p => !existingMap.has(`${p.server}:${p.port}`))
          return newOnes.length > 0 ? [...prev, ...newOnes] : prev
        })
        showToast(t('alive_proxies_found') + `: ${list.length}`)
      }
    } catch (err) {
      console.error('Fetch TG error:', err)
      showToast(t('error'))
    } finally {
      setIsFetchingTg(false)
    }
  }

  const handleTestTelegramProxies = async () => {
    if (tgProxies.length === 0) return
    setIsTestingTg(true)
    setTgProgress({ tested_count: 0, total_count: tgProxies.length, current: '' })

    try {
      const results = await api.testTelegramProxies(tgProxies)
      if (Array.isArray(results)) {
        setTgProxies(results)
        const aliveCount = results.filter(x => x.is_alive).length
        showToast(t('alive_proxies_found') + `: ${aliveCount}`)
      }
    } catch (err) {
      console.error('Test TG error:', err)
    } finally {
      setIsTestingTg(false)
      setTgProgress(null)
    }
  }

  const handleLaunchTelegram = async (p: TelegramProxy) => {
    try {
      await api.launchTelegramProxyLink(p.link)
      showToast(t('open_in_telegram'))
    } catch (err) {
      console.error('Launch TG error:', err)
    }
  }

  const handleLaunchFastestTelegram = async () => {
    const alive = tgProxies.filter(x => x.is_alive && x.latency_ms > 0)
    if (alive.length === 0) {
      showToast(t('no_results'))
      return
    }
    const fastest = alive.reduce((prev, curr) => (prev.latency_ms < curr.latency_ms ? prev : curr))
    await handleLaunchTelegram(fastest)
  }

  // Open Telegram through this app's own local proxy (127.0.0.1 socks/http)
  // instead of a third-party proxy link. Requires an active tunnel.
  const handleUseBuiltInProxy = async () => {
    try {
      await api.openTelegramProxy()
      showToast(t('open_in_telegram'))
    } catch (err) {
      console.error('Built-in TG proxy error:', err)
      showToast(t('error'))
    }
  }

  const handleShowQrModal = (title: string, link: string) => {
    const qrSvg = generateQRCodeSVG(link)
    setQrModalData({ title, link, qrSvg })
  }

  // Filtered Telegram Proxies
  const filteredTgProxies = useMemo(() => {
    return tgProxies.filter(p => {
      if (tgTypeFilter !== 'all' && p.type !== tgTypeFilter) return false
      if (tgStatusFilter === 'alive' && !p.is_alive) return false
      if (tgSearch) {
        const q = tgSearch.toLowerCase()
        return (
          p.server.toLowerCase().includes(q) ||
          String(p.port).includes(q) ||
          (p.country && p.country.toLowerCase().includes(q))
        )
      }
      return true
    })
  }, [tgProxies, tgTypeFilter, tgStatusFilter, tgSearch])

  // ==========================================
  // 2. Free Configs Handlers
  // ==========================================
  const handleFetchFreeConfigs = async () => {
    setIsFetchingConfigs(true)
    try {
      const list = await api.fetchFreeConfigs(customConfigSources.length > 0 ? customConfigSources : undefined)
      if (Array.isArray(list)) {
        setFreeConfigs(list)
        showToast(`${list.length} ` + t('fetch_configs'))
      }
    } catch (err) {
      console.error('Fetch configs error:', err)
      showToast(t('error'))
    } finally {
      setIsFetchingConfigs(false)
    }
  }

  const handleTestFreeConfigs = async () => {
    if (freeConfigs.length === 0) return
    setIsTestingConfigs(true)
    setConfigsProgress({ tested_count: 0, total_count: freeConfigs.length, current: '' })

    try {
      const results = await api.testFreeConfigs(freeConfigs)
      if (Array.isArray(results)) {
        // Auto-sort by status (alive first) and lowest latency ascending
        const sorted = [...results].sort((a, b) => {
          if (a.is_alive !== b.is_alive) return a.is_alive ? -1 : 1
          const latA = a.latency_ms > 0 ? a.latency_ms : 99999
          const latB = b.latency_ms > 0 ? b.latency_ms : 99999
          return latA - latB
        })
        setFreeConfigs(sorted)
        const alive = sorted.filter(x => x.is_alive).length
        showToast(`${alive} ` + t('active_status'))
      }
    } catch (err) {
      console.error('Test configs error:', err)
    } finally {
      setIsTestingConfigs(false)
      setConfigsProgress(null)
    }
  }

  const toggleSelectConfig = (id: string) => {
    if (selectedConfigIds.includes(id)) {
      setSelectedConfigIds(selectedConfigIds.filter(x => x !== id))
    } else {
      setSelectedConfigIds([...selectedConfigIds, id])
    }
  }

  const handleSelectAllConfigs = () => {
    if (selectedConfigIds.length === filteredConfigs.length) {
      setSelectedConfigIds([])
    } else {
      setSelectedConfigIds(filteredConfigs.map(x => x.id))
    }
  }

  const handleImportSelectedConfigs = async (onlyAlive = false) => {
    const toImport = freeConfigs.filter(x =>
      onlyAlive ? x.is_alive : selectedConfigIds.includes(x.id)
    )
    if (toImport.length === 0) {
      showToast(t('no_results'))
      return
    }

    setIsImportingConfigs(true)
    try {
      const count = await api.importFreeConfigs(toImport, 'Free Discovery')
      showToast(t('configs_imported_success', { count }))
      setSelectedConfigIds([])
    } catch (err) {
      console.error('Import configs error:', err)
      showToast(t('error'))
    } finally {
      setIsImportingConfigs(false)
    }
  }

  const handleExportSubscriptionLink = () => {
    const alive = freeConfigs.filter(x => x.is_alive)
    const list = alive.length > 0 ? alive : freeConfigs
    if (list.length === 0) {
      showToast(t('no_results'))
      return
    }
    const rawAll = list.map(x => x.raw_uri).join('\n')
    try {
      const b64 = btoa(unescape(encodeURIComponent(rawAll)))
      copyToClipboard(b64, 'sub_link', t('copied_to_clipboard'))
    } catch {
      copyToClipboard(rawAll, 'sub_link', t('copied_to_clipboard'))
    }
  }

  const handleOpenSourcesModal = (tab: 'telegram' | 'configs') => {
    setSourcesModalTab(tab)
    const list = tab === 'telegram' ? customTgSources : customConfigSources
    setSourcesInputText(list.join('\n'))
  }

  const handleSaveCustomSources = () => {
    if (!sourcesModalTab) return
    const lines = sourcesInputText
      .split('\n')
      .map(s => s.trim())
      .filter(s => s.startsWith('http://') || s.startsWith('https://'))

    if (sourcesModalTab === 'telegram') {
      setCustomTgSources(lines)
      localStorage.setItem('nexvpn_custom_tg_sources', JSON.stringify(lines))
      showToast(t('custom_sources_saved') || 'Custom sources saved!')
      handleFetchTelegramProxies()
    } else {
      setCustomConfigSources(lines)
      localStorage.setItem('nexvpn_custom_cfg_sources', JSON.stringify(lines))
      showToast(t('custom_sources_saved') || 'Custom sources saved!')
      handleFetchFreeConfigs()
    }
    setSourcesModalTab(null)
  }

  const getProtocol = (c: FreeConfigItem) => {
    if (c.raw_uri) {
      const u = c.raw_uri.toLowerCase()
      if (u.startsWith('vless://')) return 'vless'
      if (u.startsWith('vmess://')) return 'vmess'
      if (u.startsWith('trojan://')) return 'trojan'
      if (u.startsWith('ss://') || u.startsWith('shadowsocks://')) return 'shadowsocks'
      if (u.startsWith('hy2://') || u.startsWith('hysteria2://')) return 'hysteria2'
      if (u.startsWith('tuic://')) return 'tuic'
    }
    const s = (c.protocol || '').toLowerCase().trim()
    if (s === 'ss') return 'shadowsocks'
    if (s === 'hy2') return 'hysteria2'
    return s || 'vless'
  }

  // Filtered Free Configs
  const filteredConfigs = useMemo(() => {
    const targetP = (configProtoFilter || 'all').toLowerCase().trim()
    return freeConfigs.filter(c => {
      const actualP = getProtocol(c)
      if (targetP !== 'all' && actualP !== targetP) return false
      if (configSearch) {
        const q = configSearch.toLowerCase()
        return (
          (c.name && c.name.toLowerCase().includes(q)) ||
          (c.server && c.server.toLowerCase().includes(q)) ||
          actualP.includes(q) ||
          (c.country && c.country.toLowerCase().includes(q))
        )
      }
      return true
    })
  }, [freeConfigs, configProtoFilter, configSearch])

  const availableProtocols = useMemo(() => {
    const set = new Set<string>()
    freeConfigs.forEach(c => {
      const p = getProtocol(c)
      if (p) set.add(p)
    })
    return Array.from(set)
  }, [freeConfigs])

  // ==========================================
  // 3. Cloudflare & WARP Scanner Handlers
  // ==========================================
  const handleStartCfScan = async () => {
    setIsCfScanning(true)
    setCfResults([])
    setCfProgress({ total: 100, scanned: 0, found: 0, current_ip: 'Initializing Anycast feed...', mode: cfMode, results: [] })
    await api.startCloudflareDiscoveryScan(cfMode, cfIspFilter)
  }

  const handleStopCfScan = async () => {
    setIsCfScanning(false)
    await api.stopCloudflareDiscoveryScan()
  }

  const [isGeneratingWarp, setIsGeneratingWarp] = useState<string | null>(null)

  const handleGenerateWarpConfig = async (target: CloudflareTarget) => {
    const key = `${target.ip}:${target.port}`
    setIsGeneratingWarp(key)
    try {
      await api.generateWarpProfile(target.ip, target.port)
      const isFa = getLanguage() === 'fa'
      showToast(isFa ? `کانفیگ جدید وارپ با اندپوینت ${target.ip}:${target.port} ساخته شد! ✨` : `New WARP profile created with endpoint ${target.ip}:${target.port}! ✨`)
    } catch (err: any) {
      console.error('Generate WARP error:', err)
      showToast(err?.message || t('error'))
    } finally {
      setIsGeneratingWarp(null)
    }
  }

  const handleApplyToWarpProfiles = async (target: CloudflareTarget) => {
    setIsApplyingWarp(true)
    try {
      const count = await api.applyCleanIPToWARPProfiles(target.ip, target.port)
      showToast(t('warp_profiles_updated', { count }))
    } catch (err) {
      console.error('Apply WARP error:', err)
      showToast(t('error'))
    } finally {
      setIsApplyingWarp(false)
    }
  }

  const handleApplyToV2RayProfiles = async (target: CloudflareTarget, onlyCDN = true) => {
    setIsApplyingV2Ray(true)
    try {
      const count = await api.applyCleanIPToV2RayProfiles(target.ip, target.port, onlyCDN)
      showToast(t('v2ray_profiles_updated', { count }))
    } catch (err) {
      console.error('Apply V2Ray error:', err)
      showToast(t('error'))
    } finally {
      setIsApplyingV2Ray(false)
    }
  }

  // ==========================================
  // 4. Fragment Scanner Handlers
  // ==========================================
  const handleStartFragScan = async () => {
    setIsFragScanning(true)
    setFragResults([])
    setBestFragment(null)
    setFragProgress({ total: 10, scanned: 0, current: 'Starting TLS ClientHello probes...', results: [] })
    await api.startFragmentDiscoveryScan()
  }

  const handleStopFragScan = async () => {
    setIsFragScanning(false)
    await api.stopFragmentDiscoveryScan()
  }

  const handleApplyOptimalFragment = async (candidate: FragmentCandidate) => {
    setIsApplyingFrag(true)
    try {
      await api.applyOptimalFragment(candidate.size_range, candidate.sleep_range)
      showToast(t('fragment_applied_success'))
      sound?.playSuccess?.()
    } catch (err) {
      console.error('Apply fragment error:', err)
      showToast(t('error'))
    } finally {
      setIsApplyingFrag(false)
    }
  }

  // Event Listeners for All Discovery Streams
  useEffect(() => {
    const unsubCfProg = api.onEvent('cf_scanner:progress', (p: CloudflareScanProgress) => {
      setCfProgress(p)
      if (Array.isArray(p?.results) && p.results.length > 0) {
        setCfResults(prev => {
          const existingMap = new Set(prev.map(x => `${x.ip}:${x.port}`))
          const newOnes = p.results.filter(x => !existingMap.has(`${x.ip}:${x.port}`))
          return newOnes.length > 0 ? [...prev, ...newOnes] : prev
        })
      }
      setIsCfScanning(true)
    })

    const unsubCfComplete = api.onEvent('cf_scanner:complete', (results: CloudflareTarget[]) => {
      setIsCfScanning(false)
      if (Array.isArray(results) && results.length > 0) {
        setCfResults(prev => {
          const existingMap = new Set(prev.map(x => `${x.ip}:${x.port}`))
          const newOnes = results.filter(x => !existingMap.has(`${x.ip}:${x.port}`))
          return newOnes.length > 0 ? [...prev, ...newOnes] : prev
        })
      }
    })

    const unsubTgProg = api.onEvent('telegram:test_progress', (p: TelegramTestProgress) => {
      setTgProgress(p)
    })

    const unsubCfgProg = api.onEvent('free_configs:test_progress', (p: FreeConfigTestProgress) => {
      setConfigsProgress(p)
    })

    const unsubFragProg = api.onEvent('frag_scanner:progress', (p: FragmentScanProgress) => {
      setFragProgress(p)
      if (Array.isArray(p?.results)) {
        setFragResults(p.results)
      }
      if (p?.best_setting) {
        setBestFragment(p.best_setting)
      }
      setIsFragScanning(true)
    })

    const unsubFragComplete = api.onEvent('frag_scanner:complete', (data: { results: FragmentCandidate[]; best: FragmentCandidate }) => {
      setIsFragScanning(false)
      if (Array.isArray(data?.results)) {
        setFragResults(data.results)
      }
      if (data?.best) {
        setBestFragment(data.best)
      }
    })

    // Terminal error events. The backend emits `complete` on success and
    // `error` otherwise, but only `complete` was subscribed to here — so a
    // failed (or user-stopped) scan left the spinner running with no
    // explanation, and the only way out was to close the page.
    const unsubCfError = api.onEvent('cf_scanner:error', (msg: string) => {
      setIsCfScanning(false)
      showToast(typeof msg === 'string' && msg.trim() !== '' ? msg : t('error'))
    })

    const unsubFragError = api.onEvent('frag_scanner:error', (msg: string) => {
      setIsFragScanning(false)
      showToast(typeof msg === 'string' && msg.trim() !== '' ? msg : t('error'))
    })

    return () => {
      if (typeof unsubCfProg === 'function') unsubCfProg()
      if (typeof unsubCfComplete === 'function') unsubCfComplete()
      if (typeof unsubTgProg === 'function') unsubTgProg()
      if (typeof unsubCfgProg === 'function') unsubCfgProg()
      if (typeof unsubFragProg === 'function') unsubFragProg()
      if (typeof unsubFragComplete === 'function') unsubFragComplete()
      if (typeof unsubCfError === 'function') unsubCfError()
      if (typeof unsubFragError === 'function') unsubFragError()
    }
  }, [])

  // Load initial feeds safely (only on initial app launch or 45-minute interval)
  useEffect(() => {
    const lastTgFetch = Number(sessionStorage.getItem('nexvpn_last_tg_fetch') || 0)
    const now = Date.now()
    if (now - lastTgFetch > 45 * 60 * 1000) {
      sessionStorage.setItem('nexvpn_last_tg_fetch', String(now))
      const timer = setTimeout(() => {
        handleFetchTelegramProxies().catch(() => {})
      }, 300)
      return () => clearTimeout(timer)
    }
  }, [])

  return (
    <div className="flex flex-col h-full overflow-y-auto px-6 py-5 space-y-5 bg-gradient-to-b from-card/40 via-transparent to-transparent text-foreground">
      {/* Toast Notification */}
      {toastMessage && (
        <div className="fixed top-6 end-6 z-50 flex items-center gap-2.5 px-4 py-3 bg-primary/90 text-primary-foreground rounded-xl shadow-2xl backdrop-blur-md border border-primary/30 animate-in fade-in slide-in-from-top-4 duration-300">
          <Sparkles className="w-4 h-4 text-primary animate-pulse" />
          <span className="text-sm font-medium">{toastMessage}</span>
        </div>
      )}

      {/* Header Bar */}
      <div className="sticky top-0 z-30 flex flex-col lg:flex-row lg:items-center justify-between gap-4 bg-card/85 dark:bg-background backdrop-blur-xl -mx-6 -mt-5 px-6 py-4 border-b border-border/80 shadow-sm">
        <div className="flex items-center gap-3.5">
          <div className="bg-primary w-11 h-11 rounded-2xl flex items-center justify-center shadow-sm ring-1 ring-white/20 shrink-0">
            <Compass className="w-5 h-5 text-foreground" />
          </div>
          <div>
            <h1 className="text-xl font-bold tracking-tight text-foreground">
              {t('discovery_title')}
            </h1>
            <p className="text-xs text-muted-foreground mt-0.5">
              {t('discovery_subtitle')}
            </p>
          </div>
        </div>

        {/* 4 Main Sub-Tabs Switcher */}
        <div className="grid grid-cols-2 sm:flex sm:flex-wrap items-center p-1 bg-muted dark:bg-card/90 rounded-xl border border-border/80 shadow-inner gap-1">
          <button
            onClick={() => setActiveTab('telegram')}
            className={`flex items-center gap-2 px-3.5 py-2 rounded-lg text-xs font-semibold transition-all duration-200 ${
              activeTab === 'telegram'
                ? 'bg-primary text-primary-foreground shadow-sm'
                : 'text-muted-foreground hover:text-foreground dark:hover:text-foreground hover:bg-card/60 dark:hover:bg-card/40'
            }`}
          >
            <Send className="w-3.5 h-3.5" />
            <span>{t('tab_telegram_proxies')}</span>
            {tgProxies.length > 0 && (
              <span className="ms-1 px-1.5 py-0.5 text-[10px] rounded-full bg-info/30 text-primary border border-primary/20 font-bold">
                {tgProxies.length}
              </span>
            )}
          </button>

          <button
            onClick={() => setActiveTab('configs')}
            className={`flex items-center gap-2 px-3.5 py-2 rounded-lg text-xs font-semibold transition-all duration-200 ${
              activeTab === 'configs'
                ? 'bg-primary text-primary-foreground shadow-sm'
                : 'text-muted-foreground hover:text-foreground dark:hover:text-foreground hover:bg-card/60 dark:hover:bg-card/40'
            }`}
          >
            <Globe className="w-3.5 h-3.5" />
            <span>{t('tab_free_configs')}</span>
            {freeConfigs.length > 0 && (
              <span className="ms-1 px-1.5 py-0.5 text-[10px] rounded-full bg-primary/30 text-primary border border-primary/20 font-bold">
                {freeConfigs.length}
              </span>
            )}
          </button>

          <button
            onClick={() => setActiveTab('cloudflare')}
            className={`flex items-center gap-2 px-3.5 py-2 rounded-lg text-xs font-semibold transition-all duration-200 ${
              activeTab === 'cloudflare'
                ? 'bg-primary text-primary-foreground shadow-sm'
                : 'text-muted-foreground hover:text-foreground dark:hover:text-foreground hover:bg-card/60 dark:hover:bg-card/40'
            }`}
          >
            <Zap className="w-3.5 h-3.5" />
            <span>{t('tab_cloudflare_scanner')}</span>
            {cfResults.length > 0 && (
              <span className="ms-1 px-1.5 py-0.5 text-[10px] rounded-full bg-warning/30 text-warning border border-warning/20 font-bold">
                {cfResults.length}
              </span>
            )}
          </button>

          <button
            onClick={() => setActiveTab('fragment')}
            className={`flex items-center gap-2 px-3.5 py-2 rounded-lg text-xs font-semibold transition-all duration-200 ${
              activeTab === 'fragment'
                ? 'bg-destructive to-pink-600 text-destructive-foreground shadow-sm'
                : 'text-muted-foreground hover:text-foreground dark:hover:text-foreground hover:bg-card/60 dark:hover:bg-card/40'
            }`}
          >
            <Shield className="w-3.5 h-3.5" />
            <span>{t('tab_fragment_scanner')}</span>
          </button>
        </div>
      </div>

      {/* ======================================================== */}
      {/* TAB 1: TELEGRAM PROXIES                                 */}
      {/* ======================================================== */}
      {activeTab === 'telegram' && (
        <div className="space-y-4 animate-in fade-in duration-300">
          <div className="flex flex-wrap items-center justify-between gap-3 p-3.5 bg-card/80 dark:bg-card/60 rounded-2xl border border-border/80 backdrop-blur-md shadow-sm">
            <div className="flex flex-wrap items-center gap-2">
              <button
                onClick={handleFetchTelegramProxies}
                disabled={isFetchingTg || isTestingTg}
                className="flex items-center gap-2 px-3.5 py-2 bg-primary hover:bg-primary/90 disabled:opacity-50 text-primary-foreground rounded-xl text-xs font-semibold transition-all shadow-sm active:scale-95"
              >
                <RefreshCw className={`w-3.5 h-3.5 ${isFetchingTg ? 'animate-spin' : ''}`} />
                <span>{isFetchingTg ? t('fetching_proxies') : t('fetch_proxies')}</span>
              </button>

              <button
                onClick={handleTestTelegramProxies}
                disabled={isTestingTg || tgProxies.length === 0}
                className="flex items-center gap-2 px-3.5 py-2 bg-primary hover:bg-primary/90 disabled:opacity-50 text-primary-foreground rounded-xl text-xs font-semibold transition-all shadow-sm active:scale-95"
              >
                <Activity className={`w-3.5 h-3.5 ${isTestingTg ? 'animate-pulse' : ''}`} />
                <span>{isTestingTg ? t('testing_proxies') : t('test_proxies')}</span>
              </button>

              {tgProxies.some(x => x.is_alive && x.latency_ms > 0) && (
                <button
                  onClick={handleLaunchFastestTelegram}
                  className="bg-primary flex items-center gap-1.5 px-3.5 py-2 text-primary-foreground rounded-xl text-xs font-semibold shadow-sm active:scale-95 transition-all"
                >
                  <Zap className="w-3.5 h-3.5 fill-current" />
                  <span>{t('auto_connect_fastest')}</span>
                </button>
              )}

              <button
                onClick={handleUseBuiltInProxy}
                title={t('tg_builtin_proxy_hint')}
                className="flex items-center gap-1.5 px-3 py-2 bg-muted hover:bg-muted/90 dark:bg-card/80 dark:hover:bg-muted text-foreground dark:hover:text-foreground rounded-xl text-xs font-medium transition-all"
              >
                <Send className="w-3.5 h-3.5 text-primary" />
                <span>{t('tg_builtin_proxy')}</span>
              </button>

              <button
                onClick={() => handleOpenSourcesModal('telegram')}
                title={t('custom_sources_modal_title')}
                className="flex items-center gap-1.5 px-3 py-2 bg-muted hover:bg-muted/90 dark:bg-card/80 dark:hover:bg-muted text-foreground dark:hover:text-foreground rounded-xl text-xs font-medium transition-all"
              >
                <Sliders className="w-3.5 h-3.5 text-primary" />
                <span>{t('candidate_sources')}</span>
                {customTgSources.length > 0 && (
                  <span className="w-2 h-2 rounded-full bg-info" />
                )}
              </button>

              {tgProxies.some(x => !x.is_alive) && (
                <button
                  onClick={() => setTgProxies(prev => prev.filter(x => x.is_alive))}
                  className="flex items-center gap-1.5 px-3 py-2 bg-destructive/10 hover:bg-destructive/20 border border-destructive/30 text-destructive rounded-xl text-xs font-semibold transition-all active:scale-95"
                  title={t('remove_dead')}
                >
                  <Trash2 className="w-3.5 h-3.5" />
                  <span>{t('remove_dead')}</span>
                </button>
              )}

              {tgProxies.length > 0 && (
                <button
                  onClick={() => setTgProxies([])}
                  className="flex items-center gap-1.5 px-3 py-2 bg-muted hover:bg-destructive/10 dark:bg-card/60 dark:hover:bg-destructive/60 border border-border/40 text-muted-foreground hover:text-destructive dark:text-muted-foreground dark:hover:text-destructive rounded-xl text-xs font-semibold transition-all active:scale-95"
                  title={t('clear_all_proxies')}
                >
                  <XCircle className="w-3.5 h-3.5" />
                  <span>{t('clear_all_proxies')}</span>
                </button>
              )}
            </div>

            <div className="flex flex-wrap items-center gap-2">
              <div className="flex items-center p-0.5 bg-muted dark:bg-card/60 rounded-xl border border-border/60 text-xs">
                <button
                  type="button"
                  onClick={() => setTgTypeFilter('all')}
                  className={`px-3 py-1 rounded-lg transition-all font-bold ${
                    tgTypeFilter === 'all'
                      ? 'bg-primary text-primary-foreground shadow-sm'
                      : 'text-muted-foreground hover:text-foreground dark:hover:text-foreground'
                  }`}
                >
                  {t('filter_all')}
                </button>

                <button
                  type="button"
                  onClick={() => setTgTypeFilter('mtproto')}
                  className={`flex items-center gap-1.5 px-3 py-1 rounded-lg transition-all font-semibold ${
                    tgTypeFilter === 'mtproto'
                      ? 'bg-primary text-primary-foreground shadow-sm'
                      : 'text-muted-foreground hover:text-foreground dark:hover:text-foreground'
                  }`}
                >
                  <Send className="w-3 h-3 text-primary" />
                  <span dir="ltr">MTProto</span>
                </button>

                <button
                  type="button"
                  onClick={() => setTgTypeFilter('socks5')}
                  className={`flex items-center gap-1.5 px-3 py-1 rounded-lg transition-all font-semibold ${
                    tgTypeFilter === 'socks5'
                      ? 'bg-primary text-primary-foreground shadow-sm'
                      : 'text-muted-foreground hover:text-foreground dark:hover:text-foreground'
                  }`}
                >
                  <Globe className="w-3 h-3 text-primary" />
                  <span dir="ltr">SOCKS5</span>
                </button>

                <button
                  type="button"
                  onClick={() => setTgTypeFilter('http')}
                  className={`flex items-center gap-1.5 px-3 py-1 rounded-lg transition-all font-semibold ${
                    tgTypeFilter === 'http'
                      ? 'bg-primary text-primary-foreground shadow-sm'
                      : 'text-muted-foreground hover:text-foreground dark:hover:text-foreground'
                  }`}
                >
                  <Layers className="w-3 h-3 text-warning" />
                  <span dir="ltr">HTTP</span>
                </button>
              </div>

              <button
                onClick={() => setTgStatusFilter(tgStatusFilter === 'all' ? 'alive' : 'all')}
                className={`flex items-center gap-1.5 px-3 py-1.5 rounded-xl border text-xs font-semibold transition-all ${
                  tgStatusFilter === 'alive'
                    ? 'bg-success/20 border-success/40 text-success'
                    : 'bg-card/40 border-border/60 text-foreground dark:text-muted-foreground hover:text-foreground dark:hover:text-foreground shadow-sm'
                }`}
              >
                <ShieldCheck className="w-3.5 h-3.5 text-success" />
                <span>{t('alive_proxies_found')}</span>
              </button>

              <div className="relative">
                <Search className="w-3.5 h-3.5 text-muted-foreground absolute start-3 top-2.5" />
                <input
                  type="text"
                  value={tgSearch}
                  onChange={e => setTgSearch(e.target.value)}
                  placeholder={t('search')}
                  className="ps-8 pe-3 py-1.5 bg-card/60 border border-border/60 rounded-xl text-xs text-foreground placeholder:text-muted-foreground dark:placeholder-muted-foreground focus:outline-none focus:border-primary/50 w-44 shadow-sm"
                />
              </div>
            </div>
          </div>

          {tgProgress && (
            <div className="p-3 border border-border bg-muted rounded-xl space-y-1.5 animate-in fade-in duration-200">
              <div className="flex items-center justify-between text-xs text-primary font-semibold">
                <span className="flex items-center gap-2">
                  <Activity className="w-3.5 h-3.5 text-primary animate-spin" />
                  <span>{t('testing_proxies')} ({tgProgress.tested_count} / {tgProgress.total_count})</span>
                </span>
                <span className="font-mono text-muted-foreground text-[11px] truncate max-w-xs">{tgProgress.current}</span>
              </div>
              <div className="w-full h-1.5 bg-muted dark:bg-card rounded-full overflow-hidden">
                <div
                  className="bg-info h-full transition-all duration-200"
                  style={{
                    width: `${Math.min(100, Math.round((tgProgress.tested_count / (tgProgress.total_count || 1)) * 100))}%`,
                  }}
                />
              </div>
            </div>
          )}

          {filteredTgProxies.length === 0 ? (
            <div className="flex flex-col items-center justify-center py-16 text-center space-y-3 bg-card/70 dark:bg-card/30 rounded-2xl border border-border/40 shadow-sm">
              <Send className="w-10 h-10 text-muted-foreground" />
              <p className="text-sm text-muted-foreground">{t('empty_discovery_results')}</p>
              <button
                onClick={handleFetchTelegramProxies}
                className="px-4 py-2 bg-primary hover:bg-primary/90 text-primary-foreground rounded-xl text-xs font-semibold shadow-sm"
              >
                {t('fetch_proxies')}
              </button>
            </div>
          ) : (
            <div className="grid grid-cols-1 md:grid-cols-2 lg:grid-cols-3 gap-3.5">
              {filteredTgProxies.map(p => (
                <div
                  key={p.id}
                  className="group relative flex flex-col justify-between p-4 bg-card/60 hover:bg-muted/90 dark:hover:bg-card/80 rounded-2xl border border-border/80 hover:border-primary/40 transition-all duration-200 shadow-sm hover:shadow-md backdrop-blur-sm"
                >
                  <div>
                    <div className="flex items-center justify-between mb-2">
                      <div className="flex items-center gap-2">
                        <CountryFlag codeOrName={p.country_code || p.country || p.server} size="sm" />
                        <span className="px-2 py-0.5 text-[10px] font-bold uppercase rounded-md border-border bg-muted text-muted-foreground">
                          {p.type.toUpperCase()}
                        </span>
                      </div>

                      <div className="flex items-center gap-1.5">
                        {p.latency_ms > 0 ? (
                          <span
                            className={`flex items-center gap-1 text-xs font-mono font-medium px-2 py-0.5 rounded-full ${
                              p.latency_ms < 300
                                ? 'bg-success/10 text-success border border-success/20 font-bold'
                                : p.latency_ms < 700
                                ? 'bg-warning/10 text-warning border border-warning/20'
                                : 'bg-destructive/10 text-destructive border border-destructive/20'
                            }`}
                          >
                            <Clock className="w-3 h-3" />
                            {p.latency_ms} ms
                          </span>
                        ) : p.is_alive ? (
                          <span className="flex items-center gap-1 text-[11px] text-success bg-success/10 px-2 py-0.5 rounded-full font-medium">
                            <CheckCircle2 className="w-3 h-3" />
                            {t('active_status')}
                          </span>
                        ) : (
                          <span className="text-[11px] text-muted-foreground font-mono">-- ms</span>
                        )}

                        <button
                          type="button"
                          onClick={() => setTgProxies(prev => prev.filter(x => x.id !== p.id))}
                          title={t('delete')}
                          className="p-1 text-muted-foreground hover:text-destructive rounded-lg transition-all"
                        >
                          <Trash2 className="w-3.5 h-3.5" />
                        </button>
                      </div>
                    </div>

                    <div className="font-mono text-sm font-bold text-foreground truncate flex items-center justify-between gap-1 mt-1" dir="ltr">
                      <div className="truncate">
                        <span className="truncate">{p.server}</span>
                        <span className="text-xs text-primary font-semibold shrink-0 ms-0.5">:{p.port}</span>
                      </div>
                    </div>

                    {p.secret && (
                      <div className="mt-1 text-[11px] text-muted-foreground font-mono truncate" dir="ltr">
                        Secret: {p.secret.slice(0, 16)}...
                      </div>
                    )}
                  </div>

                  <div className="flex items-center gap-1.5 mt-3 pt-2.5 border-t border-border/60">
                    <button
                      type="button"
                      onClick={() => handleLaunchTelegram(p)}
                      className="bg-primary flex-1 flex items-center justify-center gap-1.5 py-1.5 px-3 text-primary-foreground rounded-xl text-xs font-semibold shadow-sm active:scale-95 transition-all whitespace-nowrap"
                    >
                      <ExternalLink className="w-3.5 h-3.5 shrink-0" />
                      <span>{t('open_in_telegram')}</span>
                    </button>

                    <button
                      type="button"
                      onClick={() => copyToClipboard(p.link, p.id, t('proxy_copied'))}
                      title={t('copy_proxy_link')}
                      className="p-2 bg-muted hover:bg-muted/90 dark:bg-card/60 dark:hover:bg-card text-foreground rounded-xl transition-all shrink-0"
                    >
                      {copiedId === p.id ? <Check className="w-3.5 h-3.5 text-success" /> : <Copy className="w-3.5 h-3.5" />}
                    </button>

                    <button
                      type="button"
                      onClick={() => handleShowQrModal(`${p.type.toUpperCase()} - ${p.server}:${p.port}`, p.link)}
                      title={t('show_qr_code')}
                      className="p-2 bg-muted hover:bg-muted/90 dark:bg-card/60 dark:hover:bg-card text-foreground rounded-xl transition-all shrink-0"
                    >
                      <QrCode className="w-3.5 h-3.5" />
                    </button>
                  </div>
                </div>
              ))}
            </div>
          )}
        </div>
      )}

      {/* ======================================================== */}
      {/* TAB 2: FREE CONFIGS & SERVERS                           */}
      {/* ======================================================== */}
      {activeTab === 'configs' && (
        <div className="space-y-4 animate-in fade-in duration-300">
          <div className="flex flex-col gap-3 p-4 bg-card/85 dark:bg-card/80 rounded-2xl border border-border/80 backdrop-blur-md shadow-sm">
            <div className="flex flex-wrap items-center justify-between gap-2.5">
              <div className="flex flex-wrap items-center gap-2">
                <button
                  onClick={handleFetchFreeConfigs}
                  disabled={isFetchingConfigs || isTestingConfigs}
                  className="flex items-center gap-2 px-3.5 py-2 bg-primary hover:bg-primary/90 disabled:opacity-50 text-primary-foreground rounded-xl text-xs font-semibold transition-all shadow-sm active:scale-95"
                >
                  <RefreshCw className={`w-3.5 h-3.5 ${isFetchingConfigs ? 'animate-spin' : ''}`} />
                  <span>{isFetchingConfigs ? t('fetching_configs') : t('fetch_configs')}</span>
                </button>

                <button
                  onClick={handleTestFreeConfigs}
                  disabled={isTestingConfigs || freeConfigs.length === 0}
                  className="flex items-center gap-2 px-3.5 py-2 bg-primary hover:bg-primary/90 disabled:opacity-50 text-primary-foreground rounded-xl text-xs font-semibold transition-all shadow-sm active:scale-95"
                >
                  <Activity className={`w-3.5 h-3.5 ${isTestingConfigs ? 'animate-pulse' : ''}`} />
                  <span>{isTestingConfigs ? t('testing_configs') : t('test_latency')}</span>
                </button>

                {selectedConfigIds.length > 0 && (
                  <button
                    onClick={() => handleImportSelectedConfigs(false)}
                    disabled={isImportingConfigs}
                    className="flex items-center gap-1.5 px-3 py-2 bg-primary hover:bg-primary/90 text-primary-foreground rounded-xl text-xs font-semibold shadow-sm transition-all active:scale-95"
                  >
                    <ArrowDownToLine className="w-3.5 h-3.5" />
                    <span>{t('import_selected_configs')} ({selectedConfigIds.length})</span>
                  </button>
                )}

                <button
                  onClick={() => handleImportSelectedConfigs(true)}
                  disabled={isImportingConfigs || freeConfigs.filter(x => x.is_alive).length === 0}
                  className="flex items-center gap-1.5 px-3 py-2 bg-muted hover:bg-muted/90 dark:bg-card dark:hover:bg-muted text-foreground rounded-xl text-xs font-semibold transition-all"
                >
                  <Download className="w-3.5 h-3.5 text-success" />
                  <span>{t('import_all_alive_configs')}</span>
                </button>

                {freeConfigs.length > 0 && (
                  <button
                    onClick={handleExportSubscriptionLink}
                    title="Export subscription URL"
                    className="flex items-center gap-1.5 px-3 py-2 bg-muted hover:bg-muted/90 dark:bg-card/80 dark:hover:bg-muted text-foreground dark:hover:text-foreground rounded-xl text-xs font-medium transition-all"
                  >
                    <Share2 className="w-3.5 h-3.5 text-primary" />
                    <span>{t('subscription')}</span>
                  </button>
                )}

                <button
                  onClick={() => handleOpenSourcesModal('configs')}
                  title={t('custom_sources_modal_title')}
                  className="flex items-center gap-1.5 px-3 py-2 bg-muted hover:bg-muted/90 dark:bg-card/80 dark:hover:bg-muted text-foreground dark:hover:text-foreground rounded-xl text-xs font-medium transition-all"
                >
                  <Sliders className="w-3.5 h-3.5 text-primary" />
                  <span>{t('candidate_sources')}</span>
                  {customConfigSources.length > 0 && (
                    <span className="w-2 h-2 rounded-full bg-primary" />
                  )}
                </button>

                {freeConfigs.some(x => !x.is_alive) && (
                  <button
                    onClick={() => setFreeConfigs(prev => prev.filter(x => x.is_alive))}
                    className="flex items-center gap-1.5 px-3 py-2 bg-destructive/30 hover:bg-destructive/50 border border-destructive/30 text-destructive rounded-xl text-xs font-semibold transition-all active:scale-95"
                    title={t('remove_dead')}
                  >
                    <Trash2 className="w-3.5 h-3.5" />
                    <span>{t('remove_dead')}</span>
                  </button>
                )}

                {freeConfigs.length > 0 && (
                  <button
                    onClick={() => setFreeConfigs([])}
                    className="flex items-center gap-1.5 px-3 py-2 bg-card/60 hover:bg-destructive/60 border border-border/40 hover:border-destructive/40 text-muted-foreground hover:text-destructive rounded-xl text-xs font-semibold transition-all active:scale-95"
                    title={t('clear_all_proxies')}
                  >
                    <XCircle className="w-3.5 h-3.5" />
                    <span>{t('clear_all_proxies')}</span>
                  </button>
                )}
              </div>
            </div>

            <div className="flex flex-wrap items-center justify-between gap-2.5 pt-2 border-t border-border/60">
              <div className="flex flex-wrap items-center gap-2">
                <button
                  onClick={handleSelectAllConfigs}
                  className="flex items-center gap-1.5 px-3 py-1.5 bg-card/60 border border-border/80 rounded-xl text-xs text-foreground transition-all font-medium"
                >
                  {selectedConfigIds.length === filteredConfigs.length && filteredConfigs.length > 0 ? (
                    <CheckSquare className="w-3.5 h-3.5 text-primary" />
                  ) : (
                    <SquareOutline className="w-3.5 h-3.5 text-muted-foreground" />
                  )}
                  <span>{selectedConfigIds.length === filteredConfigs.length ? t('deselect_all') : t('select_all')}</span>
                </button>

                <div className="flex items-center p-0.5 bg-card/80 rounded-xl border border-border/80 text-xs gap-0.5 flex-wrap">
                  <button
                    onClick={() => setConfigProtoFilter('all')}
                    className={`px-3 py-1 rounded-lg uppercase transition-all font-bold ${
                      configProtoFilter === 'all'
                        ? 'bg-primary text-primary-foreground shadow-sm'
                        : 'text-muted-foreground hover:text-foreground'
                    }`}
                  >
                    {getLanguage() === 'fa' ? 'همه' : 'ALL'}
                  </button>
                  {availableProtocols.map(proto => (
                    <button
                      key={proto}
                      onClick={() => setConfigProtoFilter(proto)}
                      className={`px-3 py-1 rounded-lg uppercase text-[11px] transition-all font-bold ${
                        configProtoFilter.toLowerCase() === proto.toLowerCase()
                          ? 'bg-primary text-primary-foreground shadow-sm'
                          : 'text-muted-foreground hover:text-foreground'
                      }`}
                    >
                      {proto}
                    </button>
                  ))}
                </div>
              </div>

              <div className="relative">
                <Search className="w-3.5 h-3.5 text-muted-foreground absolute start-3 top-2.5" />
                <input
                  type="text"
                  value={configSearch}
                  onChange={e => setConfigSearch(e.target.value)}
                  placeholder={t('search')}
                  className="ps-8 pe-3 py-1.5 bg-card/80 border border-border/80 rounded-xl text-xs text-foreground placeholder-muted-foreground focus:outline-none focus:border-primary/50 w-48"
                />
              </div>
            </div>
          </div>

          {configsProgress && (
            <div className="p-3 bg-primary/30 border border-primary/30 rounded-xl space-y-1.5 animate-in fade-in duration-200">
              <div className="flex items-center justify-between text-xs text-primary">
                <span className="flex items-center gap-2">
                  <Activity className="w-3.5 h-3.5 text-primary animate-spin" />
                  <span>{t('testing_configs')} ({configsProgress.tested_count} / {configsProgress.total_count})</span>
                </span>
                <span className="font-mono text-muted-foreground text-[11px] truncate max-w-xs">{configsProgress.current}</span>
              </div>
              <div className="w-full h-1.5 bg-card rounded-full overflow-hidden">
                <div
                  className="bg-primary h-full transition-all duration-200"
                  style={{
                    width: `${Math.min(100, Math.round((configsProgress.tested_count / (configsProgress.total_count || 1)) * 100))}%`,
                  }}
                />
              </div>
            </div>
          )}

          {filteredConfigs.length === 0 ? (
            <div className="flex flex-col items-center justify-center py-16 text-center space-y-3 bg-card/30 rounded-2xl border border-border/40">
              <Globe className="w-10 h-10 text-muted-foreground" />
              <p className="text-sm text-muted-foreground">{t('empty_discovery_results')}</p>
              <button
                onClick={handleFetchFreeConfigs}
                className="px-4 py-2 bg-primary/80 hover:bg-primary text-primary-foreground rounded-xl text-xs font-semibold"
              >
                {t('fetch_configs')}
              </button>
            </div>
          ) : (
            <div className="space-y-2">
              {filteredConfigs.map(c => {
                const isSelected = selectedConfigIds.includes(c.id)
                const itemProto = getProtocol(c)
                return (
                  <div
                    key={c.id}
                    onClick={() => toggleSelectConfig(c.id)}
                    className={`flex items-center justify-between p-3 rounded-xl border transition-all cursor-pointer ${
                      isSelected
                        ? 'bg-primary/40 border-primary/50 shadow-sm'
                        : 'bg-card/60 hover:bg-muted dark:hover:bg-card/80 border-border/80 shadow-sm'
                    }`}
                  >
                    <div className="flex items-center gap-3 min-w-0">
                      <button
                        type="button"
                        onClick={e => {
                          e.stopPropagation()
                          toggleSelectConfig(c.id)
                        }}
                        className="text-muted-foreground hover:text-foreground dark:text-muted-foreground dark:hover:text-foreground"
                      >
                        {isSelected ? (
                          <CheckSquare className="w-4 h-4 text-primary" />
                        ) : (
                          <SquareOutline className="w-4 h-4 text-muted-foreground" />
                        )}
                      </button>

                      <CountryFlag codeOrName={c.country_code || c.country || c.name || c.server} size="sm" />

                      <span className="px-2 py-0.5 text-[10px] font-bold uppercase rounded-md bg-primary/20 text-primary border border-primary/20">
                        {itemProto}
                      </span>

                      <div className="min-w-0">
                        <div className="flex items-center gap-2">
                          <span className="text-xs font-semibold text-foreground truncate max-w-sm">
                            {cleanServerName(c.name) || `${c.server}:${c.port}`}
                          </span>
                          {c.country && (
                            <span className="text-[10px] px-1.5 py-0.2 rounded bg-muted text-muted-foreground border border-border">
                              {c.country}
                            </span>
                          )}
                        </div>
                        <div className="text-[11px] font-mono text-muted-foreground truncate max-w-xs">
                          {c.server}:{c.port} {c.source_name ? `• ${c.source_name}` : ''}
                        </div>
                      </div>
                    </div>

                    <div className="flex items-center gap-3">
                      {c.latency_ms > 0 ? (
                        <span
                          className={`flex items-center gap-1 text-xs font-mono font-medium px-2 py-0.5 rounded-full ${
                            c.latency_ms < 300
                              ? 'bg-success/10 text-success border border-success/20'
                              : c.latency_ms < 700
                              ? 'bg-warning/10 text-warning border border-warning/20'
                              : 'bg-destructive/10 text-destructive border border-destructive/20'
                          }`}
                        >
                          <Clock className="w-3 h-3" />
                          {c.latency_ms} ms
                        </span>
                      ) : c.is_alive ? (
                        <span className="flex items-center gap-1 text-[11px] text-success bg-success/10 px-2 py-0.5 rounded-full">
                          <CheckCircle2 className="w-3 h-3" />
                          OK
                        </span>
                      ) : (
                        <span className="text-xs text-muted-foreground font-mono">-- ms</span>
                      )}

                      <div className="flex items-center gap-1" role="dialog" onClick={e => e.stopPropagation()}>
                        <button
                          onClick={() => copyToClipboard(c.raw_uri, c.id)}
                          title={t('copy')}
                          className="p-1.5 bg-muted hover:bg-muted/90 text-muted-foreground hover:text-foreground dark:bg-card/60 dark:hover:bg-card dark:text-muted-foreground dark:hover:text-foreground rounded-lg transition-all"
                        >
                          {copiedId === c.id ? <Check className="w-3.5 h-3.5 text-success" /> : <Copy className="w-3.5 h-3.5" />}
                        </button>

                        <button
                          onClick={() => handleShowQrModal(c.name, c.raw_uri)}
                          title={t('show_qr_code')}
                          className="p-1.5 bg-muted hover:bg-muted/90 text-muted-foreground hover:text-foreground dark:bg-card/60 dark:hover:bg-card dark:text-muted-foreground dark:hover:text-foreground rounded-lg transition-all"
                        >
                          <QrCode className="w-3.5 h-3.5" />
                        </button>

                        <button
                          onClick={() => setFreeConfigs(prev => prev.filter(x => x.id !== c.id))}
                          title={t('delete')}
                          className="p-1.5 bg-muted hover:bg-destructive text-muted-foreground hover:text-destructive dark:bg-card/60 dark:hover:bg-destructive/60 dark:text-muted-foreground dark:hover:text-destructive rounded-lg transition-all"
                        >
                          <Trash2 className="w-3.5 h-3.5" />
                        </button>
                      </div>
                    </div>
                  </div>
                )
              })}
            </div>
          )}
        </div>
      )}

      {/* ======================================================== */}
      {/* TAB 3: CLOUDFLARE & WARP SCANNER                        */}
      {/* ======================================================== */}
      {activeTab === 'cloudflare' && (
        <div className="space-y-4 animate-in fade-in duration-300">
          <div className="flex flex-wrap items-center justify-between gap-3 p-4 bg-card/60 rounded-2xl border border-border/80 backdrop-blur-md shadow-sm">
            <div className="flex flex-wrap items-center gap-2.5">
              {/* Dual Mode Switcher */}
              <div className="flex items-center p-0.5 bg-muted dark:bg-card/60 rounded-xl border border-border/60 text-xs">
                <button
                  onClick={() => setCfMode('warp')}
                  disabled={isCfScanning}
                  className={`flex items-center gap-1.5 px-3 py-1.5 rounded-lg font-medium transition-all ${
                    cfMode === 'warp'
                      ? 'bg-primary text-primary-foreground shadow-sm'
                      : 'text-muted-foreground hover:text-foreground dark:text-muted-foreground dark:hover:text-foreground'
                  }`}
                >
                  <Zap className="w-3.5 h-3.5" />
                  <span>{t('cf_mode_warp')}</span>
                </button>

                <button
                  onClick={() => setCfMode('cdn')}
                  disabled={isCfScanning}
                  className={`flex items-center gap-1.5 px-3 py-1.5 rounded-lg font-medium transition-all ${
                    cfMode === 'cdn'
                      ? 'bg-primary text-primary-foreground shadow-sm'
                      : 'text-muted-foreground hover:text-foreground dark:text-muted-foreground dark:hover:text-foreground'
                  }`}
                >
                  <Globe className="w-3.5 h-3.5" />
                  <span>{t('cf_mode_cdn')}</span>
                </button>
              </div>

              {/* ISP Filter Pills */}
              <div className="flex items-center p-0.5 bg-muted dark:bg-card/60 rounded-xl border border-border/60 text-[11px]">
                {(['all', 'mci', 'mtn', 'rtl'] as const).map(isp => (
                  <button
                    key={isp}
                    onClick={() => setCfIspFilter(isp)}
                    className={`px-2.5 py-1 rounded-lg transition-all ${
                      cfIspFilter === isp
                        ? 'bg-warning/20 text-warning font-semibold border border-warning/30'
                        : 'text-muted-foreground hover:text-foreground dark:text-muted-foreground dark:hover:text-foreground'
                    }`}
                  >
                    {isp === 'all' ? t('isp_filter_all') : isp === 'mci' ? t('isp_filter_mci') : isp === 'mtn' ? t('isp_filter_mtn') : t('isp_filter_rtl')}
                  </button>
                ))}
              </div>

              {!isCfScanning ? (
                <button
                  onClick={handleStartCfScan}
                  className="bg-primary flex items-center gap-2 px-4 py-2 text-primary-foreground rounded-xl text-xs font-semibold shadow-sm active:scale-95 transition-all"
                >
                  <Play className="w-3.5 h-3.5 fill-current" />
                  <span>{t('start_scanner')}</span>
                </button>
              ) : (
                <button
                  onClick={handleStopCfScan}
                  className="flex items-center gap-2 px-4 py-2 bg-destructive hover:bg-destructive/90 text-destructive-foreground rounded-xl text-xs font-semibold shadow-sm active:scale-95 transition-all"
                >
                  <Square className="w-3.5 h-3.5 fill-current" />
                  <span>{t('stop_scanner')}</span>
                </button>
              )}

              {cfResults.some(x => x.loss_rate > 0) && (
                <button
                  onClick={() => setCfResults(prev => prev.filter(x => x.loss_rate === 0))}
                  className="flex items-center gap-1.5 px-3 py-2 bg-destructive hover:bg-destructive/30 dark:hover:bg-destructive/50 border border-destructive/30 text-destructive rounded-xl text-xs font-semibold transition-all active:scale-95"
                  title={t('remove_unstable')}
                >
                  <Trash2 className="w-3.5 h-3.5" />
                  <span>{t('remove_unstable')}</span>
                </button>
              )}

              {cfResults.length > 0 && (
                <button
                  onClick={() => setCfResults([])}
                  className="flex items-center gap-1.5 px-3 py-2 bg-muted hover:bg-destructive border border-border hover:border-destructive dark:bg-card/60 dark:hover:bg-destructive/60 dark:border-border/40 dark:hover:border-destructive/40 text-muted-foreground hover:text-destructive dark:text-muted-foreground dark:hover:text-destructive rounded-xl text-xs font-semibold transition-all active:scale-95"
                  title={t('clear_all_proxies')}
                >
                  <XCircle className="w-3.5 h-3.5" />
                  <span>{t('clear_all_proxies')}</span>
                </button>
              )}
            </div>

            <div className="flex items-center gap-3 text-xs">
              <div className="px-3 py-1.5 bg-muted dark:bg-card/60 rounded-xl border border-border/60">
                <span className="text-muted-foreground">{t('alive_proxies_found')}: </span>
                <span className="font-bold text-warning">{cfResults.length}</span>
              </div>
            </div>
          </div>

          {isCfScanning && cfProgress && (
            <div className="p-3 bg-warning/10 border border-warning/20 rounded-xl space-y-1.5 animate-in fade-in duration-200">
              <div className="flex items-center justify-between text-xs text-warning">
                <span className="flex items-center gap-2">
                  <Activity className="w-3.5 h-3.5 text-warning animate-spin" />
                  <span>{t('scanning')} ({cfProgress.scanned} / {cfProgress.total})</span>
                </span>
                <span className="font-mono text-muted-foreground text-[11px]">{cfProgress.current_ip}</span>
              </div>
              <div className="w-full h-1.5 bg-muted dark:bg-card rounded-full overflow-hidden">
                <div
                  className="bg-warning h-full transition-all duration-200"
                  style={{
                    width: `${Math.min(100, Math.round((cfProgress.scanned / (cfProgress.total || 1)) * 100))}%`,
                  }}
                />
              </div>
            </div>
          )}

          {cfResults.length === 0 ? (
            <div className="flex flex-col items-center justify-center py-16 text-center space-y-3 bg-card/30 rounded-2xl border border-border/80 dark:border-border/40 shadow-sm">
              <Zap className="w-10 h-10 text-muted-foreground" />
              <p className="text-sm text-muted-foreground">{t('empty_discovery_results')}</p>
              <button
                onClick={handleStartCfScan}
                className="px-4 py-2 bg-primary hover:bg-primary/90 text-primary-foreground rounded-xl text-xs font-semibold shadow-sm"
              >
                {t('start_scanner')}
              </button>
            </div>
          ) : (
            <div className="grid grid-cols-1 md:grid-cols-2 lg:grid-cols-3 gap-3.5">
              {cfResults.map((target, idx) => (
                <div
                  key={`${target.ip}:${target.port}-${idx}`}
                  className="group flex flex-col justify-between p-4 bg-card/60 hover:bg-muted dark:hover:bg-card/80 rounded-2xl border border-border/80 hover:border-warning/40 transition-all duration-200 shadow-sm hover:shadow-sm backdrop-blur-sm"
                >
                  <div>
                    <div className="flex items-center justify-between mb-2">
                      <span className={`px-2 py-0.5 text-[10px] font-bold uppercase rounded-md ${
                        target.mode === 'cdn'
                          ? 'bg-success/20 text-success border border-success/20'
                          : 'bg-warning/20 text-warning border border-warning/20'
                      }`}>
                        {target.mode === 'cdn' ? '🌐 CDN Clean IP' : 'WARP Endpoint'}
                      </span>

                      <div className="flex items-center gap-1.5">
                        {target.loss_rate > 0 && (
                          <span className="text-[10px] text-destructive font-mono">
                            {target.loss_rate}% loss
                          </span>
                        )}
                        <span
                          className={`flex items-center gap-1 text-xs font-mono font-medium px-2 py-0.5 rounded-full ${
                            target.latency_ms < 200
                              ? 'bg-success/10 text-success border border-success/20'
                              : target.latency_ms < 500
                              ? 'bg-warning/10 text-warning border border-warning/20'
                              : 'bg-destructive/10 text-destructive border border-destructive/20'
                          }`}
                        >
                          <Clock className="w-3 h-3" />
                          {target.latency_ms} ms
                        </span>

                        <button
                          type="button"
                          onClick={() => setCfResults(prev => prev.filter(x => !(x.ip === target.ip && x.port === target.port)))}
                          title={t('delete')}
                          className="p-1 text-muted-foreground hover:text-destructive hover:bg-destructive dark:text-muted-foreground dark:hover:text-destructive dark:hover:bg-destructive/40 rounded-lg transition-all"
                        >
                          <Trash2 className="w-3.5 h-3.5" />
                        </button>
                      </div>
                    </div>

                    <div className="font-mono text-sm font-semibold text-foreground truncate flex items-center justify-between gap-1" dir="ltr">
                      <div className="truncate">
                        <span>{target.ip}</span>
                        <span className={`text-xs font-normal ms-0.5 ${target.mode === 'cdn' ? 'text-success' : 'text-warning'}`}>:{target.port}</span>
                      </div>
                      <button
                        type="button"
                        onClick={() => copyToClipboard(`${target.ip}:${target.port}`, `${target.ip}:${target.port}`, t('clean_ip_copied'))}
                        title={t('copy_clean_ip')}
                        className="p-1 text-muted-foreground hover:text-foreground dark:text-muted-foreground dark:hover:text-foreground rounded transition-all shrink-0"
                      >
                        {copiedId === `${target.ip}:${target.port}` ? (
                          <Check className="w-3.5 h-3.5 text-success" />
                        ) : (
                          <Copy className="w-3.5 h-3.5" />
                        )}
                      </button>
                    </div>

                    {target.location && (
                      <div className="mt-1 text-[11px] text-muted-foreground truncate">
                        {target.location}
                      </div>
                    )}
                  </div>

                  <div className="mt-3 pt-2.5 border-t border-border/60">
                    {target.mode === 'warp' ? (
                      <div className="grid grid-cols-2 gap-1.5 w-full">
                        <button
                          type="button"
                          onClick={() => handleGenerateWarpConfig(target)}
                          disabled={isGeneratingWarp === `${target.ip}:${target.port}`}
                          className="bg-warning/20 hover:bg-warning/30 flex items-center justify-center gap-1 py-2 px-2 text-warning hover:text-warning border border-warning/35 rounded-xl text-xs font-bold shadow-sm active:scale-95 transition-all truncate"
                          title={t('generate_warp_config')}
                        >
                          <Zap className={`w-3.5 h-3.5 text-warning shrink-0 ${isGeneratingWarp === `${target.ip}:${target.port}` ? 'animate-spin' : ''}`} />
                          <span className="truncate">{isGeneratingWarp === `${target.ip}:${target.port}` ? t('generating') : t('create_warp_btn')}</span>
                        </button>
                        <button
                          type="button"
                          onClick={() => handleApplyToWarpProfiles(target)}
                          disabled={isApplyingWarp}
                          title={t('apply_to_warp_profiles')}
                          className="flex items-center justify-center gap-1 py-2 px-2 bg-muted hover:bg-muted/90 text-foreground border border-border dark:bg-card/70 dark:hover:bg-muted dark:text-foreground dark:hover:text-foreground dark:border-border/50 rounded-xl text-xs font-semibold transition-all active:scale-95 truncate"
                        >
                          <Sparkles className="w-3.5 h-3.5 text-warning shrink-0" />
                          <span className="truncate">{t('apply_to_warp_btn')}</span>
                        </button>
                      </div>
                    ) : (
                      <div className="w-full">
                        <button
                          type="button"
                          onClick={() => handleApplyToV2RayProfiles(target, true)}
                          disabled={isApplyingV2Ray}
                          className="bg-success/20 hover:bg-success/30 w-full flex items-center justify-center gap-1.5 py-2 px-3 text-success hover:text-success border border-success/35 rounded-xl text-xs font-bold shadow-sm active:scale-95 transition-all"
                          title={t('apply_to_v2ray_profiles')}
                        >
                          <Globe className="w-3.5 h-3.5 text-success shrink-0" />
                          <span>{t('apply_to_v2ray_btn')}</span>
                        </button>
                      </div>
                    )}
                  </div>
                </div>
              ))}
            </div>
          )}
        </div>
      )}

      {/* ======================================================== */}
      {/* TAB 4: FRAGMENT SCANNER (DPI BYPASS)                    */}
      {/* ======================================================== */}
      {activeTab === 'fragment' && (
        <div className="space-y-4 animate-in fade-in duration-300">
          <div className="p-4 bg-card/60 rounded-2xl border border-border/80 backdrop-blur-md shadow-sm flex flex-col md:flex-row items-start md:items-center justify-between gap-4">
            <div className="flex items-center gap-3">
              <div className="w-10 h-10 rounded-xl bg-destructive/10 border border-destructive/20 flex items-center justify-center text-destructive">
                <Flame className="w-5 h-5" />
              </div>
              <div>
                <h3 className="text-sm font-bold text-foreground">{t('tab_fragment_scanner')}</h3>
                <p className="text-xs text-muted-foreground mt-0.5">{t('fragment_scanner_desc')}</p>
              </div>
            </div>

            <div className="flex items-center gap-2">
              {!isFragScanning ? (
                <button
                  onClick={handleStartFragScan}
                  className="bg-destructive flex items-center gap-2 px-4 py-2 to-pink-600 hover:to-pink-500 text-destructive-foreground rounded-xl text-xs font-semibold shadow-sm active:scale-95 transition-all"
                >
                  <Play className="w-3.5 h-3.5 fill-current" />
                  <span>{t('start_scanner')}</span>
                </button>
              ) : (
                <button
                  onClick={handleStopFragScan}
                  className="flex items-center gap-2 px-4 py-2 bg-destructive hover:bg-destructive/90 text-destructive-foreground rounded-xl text-xs font-semibold shadow-sm active:scale-95 transition-all"
                >
                  <Square className="w-3.5 h-3.5 fill-current" />
                  <span>{t('stop_scanner')}</span>
                </button>
              )}
            </div>
          </div>

          {/* Real-time Scanning Progress */}
          {isFragScanning && fragProgress && (
            <div className="p-3 bg-destructive/10 border border-destructive/20 rounded-xl space-y-1.5 animate-in fade-in duration-200">
              <div className="flex items-center justify-between text-xs text-destructive">
                <span className="flex items-center gap-2">
                  <Activity className="w-3.5 h-3.5 text-destructive animate-spin" />
                  <span>{t('scanning')} ({fragProgress.scanned} / {fragProgress.total})</span>
                </span>
                <span className="font-mono text-muted-foreground text-[11px]">{fragProgress.current}</span>
              </div>
              <div className="w-full h-1.5 bg-muted dark:bg-card rounded-full overflow-hidden">
                <div
                  className="bg-destructive h-full to-pink-400 transition-all duration-200"
                  style={{
                    width: `${Math.min(100, Math.round((fragProgress.scanned / (fragProgress.total || 1)) * 100))}%`,
                  }}
                />
              </div>
            </div>
          )}

          {/* Optimal Setting Spotlight Banner */}
          {bestFragment && (
            <div className="bg-success/10 p-4 rounded-2xl border border-success/30 shadow-lg flex flex-col md:flex-row md:items-center justify-between gap-4 animate-in fade-in slide-in-from-top-2 duration-300">
              <div className="flex items-center gap-3">
                <div className="w-10 h-10 rounded-xl bg-success/20 border border-success/30 flex items-center justify-center text-success">
                  <Award className="w-5 h-5" />
                </div>
                <div>
                  <div className="flex items-center gap-2">
                    <span className="text-xs font-bold text-success uppercase tracking-wider">
                      {t('optimal_fragment_found')}
                    </span>
                    <span className="px-2 py-0.5 text-[10px] font-bold rounded-full bg-success/20 text-success border border-success/30">
                      {bestFragment.success_rate}% {t('success_rate')}
                    </span>
                  </div>
                  <div className="text-xs text-foreground font-mono mt-1">
                    {t('chunk_size')}: <span className="text-foreground font-bold">{bestFragment.size_range}</span> | {t('delay_sleep')}: <span className="text-foreground font-bold">{bestFragment.sleep_range} ms</span> ({t('latency')}: {bestFragment.avg_latency} ms)
                  </div>
                </div>
              </div>

              <button
                onClick={() => handleApplyOptimalFragment(bestFragment)}
                disabled={isApplyingFrag}
                className="bg-primary flex items-center gap-2 px-4 py-2 text-primary-foreground rounded-xl text-xs font-semibold shadow-sm active:scale-95 transition-all shrink-0"
              >
                <Sparkles className="w-3.5 h-3.5" />
                <span>{t('apply_optimal_fragment')}</span>
              </button>
            </div>
          )}

          {/* Candidates List Table */}
          {fragResults.length === 0 ? (
            <div className="flex flex-col items-center justify-center py-16 text-center space-y-3 bg-card/30 rounded-2xl border border-border/80 dark:border-border/40 shadow-sm">
              <Flame className="w-10 h-10 text-muted-foreground" />
              <p className="text-sm text-muted-foreground">{t('empty_discovery_results')}</p>
              <button
                onClick={handleStartFragScan}
                className="px-4 py-2 bg-destructive hover:bg-destructive/90 text-destructive-foreground rounded-xl text-xs font-semibold shadow-sm"
              >
                {t('start_scanner')}
              </button>
            </div>
          ) : (
            <div className="space-y-2">
              {fragResults.map(cand => (
                <div
                  key={cand.id}
                  className={`flex items-center justify-between p-3.5 rounded-xl border transition-all ${
                    cand.recommended
                      ? 'bg-success/70 dark:bg-success/30 border-success/50 shadow-sm'
                      : cand.status === 'optimal'
                      ? 'bg-card/60 border-border/80 dark:border-border hover:border-border dark:hover:border-border shadow-sm'
                      : 'bg-muted dark:bg-card/40 border-border opacity-80'
                  }`}
                >
                  <div className="flex items-center gap-3">
                    <div
                      className={`w-8 h-8 rounded-lg flex items-center justify-center font-mono font-bold text-xs ${
                        cand.success_rate >= 80
                          ? 'bg-success/20 text-success'
                          : cand.success_rate >= 50
                          ? 'bg-warning/20 text-warning'
                          : 'bg-destructive/20 text-destructive'
                      }`}
                    >
                      {cand.success_rate}%
                    </div>

                    <div>
                      <div className="text-xs font-bold text-foreground flex items-center gap-2">
                        <span>Size: <span className="font-mono text-primary">{cand.size_range}</span></span>
                        <span className="text-muted-foreground">•</span>
                        <span>Sleep: <span className="font-mono text-primary">{cand.sleep_range} ms</span></span>
                        {cand.recommended && (
                          <span className="px-2 py-0.5 text-[10px] bg-success/20 text-success border border-success/30 rounded-md font-bold">
                            TOP 1
                          </span>
                        )}
                      </div>
                      <div className="text-[11px] text-muted-foreground font-mono mt-0.5">
                        Score: {cand.score}/100 • Latency: {cand.avg_latency > 0 ? `${cand.avg_latency} ms` : '--'}
                      </div>
                    </div>
                  </div>

                  <button
                    onClick={() => handleApplyOptimalFragment(cand)}
                    disabled={isApplyingFrag}
                    className="flex items-center gap-1.5 px-3 py-1.5 bg-muted hover:bg-muted/90 text-foreground border border-border dark:bg-card dark:hover:bg-muted dark:text-foreground dark:hover:text-foreground dark:border-transparent rounded-lg text-xs font-semibold transition-all active:scale-95"
                  >
                    <Sparkles className="w-3 h-3 text-success" />
                    <span>{t('apply')}</span>
                  </button>
                </div>
              ))}
            </div>
          )}
        </div>
      )}

      {/* ======================================================== */}
      {/* Custom Sources Management Modal                          */}
      {/* ======================================================== */}
      {sourcesModalTab && (
        <ModalShell
          isOpen={true}
          onClose={() => setSourcesModalTab(null)}
          maxWidth="max-w-lg"
          bodyClassName="p-6 space-y-4"
        >
            <div className="flex items-center justify-between">
              <div className="flex items-center gap-2">
                <Sliders className="w-4 h-4 text-primary" />
                <h3 className="text-sm font-bold text-foreground">
                  {t('custom_sources_modal_title')} ({sourcesModalTab === 'telegram' ? 'Telegram' : 'Free Configs'})
                </h3>
              </div>
              <button
                onClick={() => setSourcesModalTab(null)}
                className="p-1 text-muted-foreground hover:text-foreground dark:text-muted-foreground dark:hover:text-foreground rounded-lg"
              >
                <X className="w-4 h-4" />
              </button>
            </div>

            <p className="text-xs text-muted-foreground">
              {t('custom_sources_desc')}
            </p>

            <textarea
              rows={6}
              value={sourcesInputText}
              onChange={e => setSourcesInputText(e.target.value)}
              placeholder="https://raw.githubusercontent.com/user/repo/main/list.txt&#10;https://example.com/sub/configs"
              className="w-full p-3 bg-muted dark:bg-card/80 border border-border/80 rounded-xl text-xs font-mono text-foreground placeholder:text-muted-foreground dark:placeholder-muted-foreground focus:outline-none focus:border-primary/50 resize-y"
            />

            <div className="flex items-center justify-between gap-3 pt-2">
              <button
                type="button"
                onClick={() => setSourcesInputText('')}
                className="text-xs text-muted-foreground hover:text-destructive dark:text-muted-foreground dark:hover:text-destructive transition"
              >
                {t('clear_all')}
              </button>

              <div className="flex items-center gap-2">
                <button
                  onClick={() => setSourcesModalTab(null)}
                  className="px-4 py-2 bg-muted hover:bg-muted/90 text-foreground dark:bg-card dark:hover:bg-muted dark:text-foreground rounded-xl text-xs font-semibold"
                >
                  {t('cancel')}
                </button>
                <button
                  onClick={handleSaveCustomSources}
                  className="px-4 py-2 bg-primary hover:bg-primary/90 text-primary-foreground rounded-xl text-xs font-semibold shadow-sm"
                >
                  {t('custom_sources_save')}
                </button>
              </div>
            </div>
        </ModalShell>
      )}

      {/* ======================================================== */}
      {/* QR Code Modal                                            */}
      {/* ======================================================== */}
      {qrModalData && (
        <ModalShell
          isOpen={true}
          onClose={() => setQrModalData(null)}
          maxWidth="max-w-sm"
          bodyClassName="p-6 space-y-4 text-center"
        >
            <div className="flex items-center justify-between">
              <h3 className="text-sm font-semibold text-foreground truncate">{qrModalData.title}</h3>
              <button
                onClick={() => setQrModalData(null)}
                className="p-1 text-muted-foreground hover:text-foreground dark:text-muted-foreground dark:hover:text-foreground rounded-lg"
              >
                <X className="w-4 h-4" />
              </button>
            </div>

            <div className="bg-card p-3 rounded-xl mx-auto inline-block shadow-lg border border-border">
              <img src={qrModalData.qrSvg} alt="QR Code" className="w-[220px] h-[220px]" />
            </div>

            <div className="p-2.5 bg-muted dark:bg-card/60 rounded-xl text-start border border-border/60">
              <p className="font-mono text-[11px] text-muted-foreground break-all select-all line-clamp-3">
                {qrModalData.link}
              </p>
            </div>

            <div className="flex gap-2">
              <button
                onClick={() => copyToClipboard(qrModalData.link, 'modal_qr')}
                className="flex-1 py-2 px-3 bg-primary hover:bg-primary/90 text-primary-foreground rounded-xl text-xs font-semibold shadow-sm"
              >
                {t('copy')}
              </button>
              <button
                onClick={() => setQrModalData(null)}
                className="py-2 px-4 bg-muted hover:bg-muted/90 text-foreground dark:bg-card dark:hover:bg-muted dark:text-foreground rounded-xl text-xs font-semibold"
              >
                {t('close')}
              </button>
            </div>
        </ModalShell>
      )}
    </div>
  )
}
