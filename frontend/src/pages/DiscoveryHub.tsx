import React, { useState, useEffect, useCallback, useMemo } from 'react'
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
  Upload,
  CheckCircle2,
  XCircle,
  Clock,
  Filter,
  Search,
  ExternalLink,
  ShieldCheck,
  Layers,
  Sparkles,
  Sliders,
  Radio,
  Share2,
  CheckSquare,
  Square as SquareOutline,
  X,
  Activity,
  ArrowDownToLine,
  Shield,
  Flame,
  Award,
} from 'lucide-react'
import { api } from '../lib/api'
import { t, getLanguage } from '../lib/i18n'
import { sound } from '../lib/sound'
import { generateQRCodeSVG } from '../lib/qrcode'
import CountryFlag from '../components/CountryFlag'
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
  status: ConnectionStatus
  onStatusChange?: (s: ConnectionStatus) => void
}

type DiscoveryTab = 'telegram' | 'configs' | 'cloudflare' | 'fragment'

const INITIAL_STARTER_PROXIES: TelegramProxy[] = [
  {
    id: 'init-tg-1',
    type: 'mtproto',
    server: '149.154.175.50',
    port: 443,
    secret: 'ee000000000000000000000000000000007777772e676f6f676c652e636f6d',
    country: 'Netherlands',
    country_code: 'NL',
    link: 'tg://proxy?server=149.154.175.50&port=443&secret=ee000000000000000000000000000000007777772e676f6f676c652e636f6d',
    is_alive: true,
    latency_ms: 85,
    source: 'Built-in',
  },
  {
    id: 'init-tg-2',
    type: 'mtproto',
    server: '149.154.167.51',
    port: 443,
    secret: 'ee00000000000000000000000000000000646c2e676f6f676c652e636f6d',
    country: 'Germany',
    country_code: 'DE',
    link: 'tg://proxy?server=149.154.167.51&port=443&secret=ee00000000000000000000000000000000646c2e676f6f676c652e636f6d',
    is_alive: true,
    latency_ms: 120,
    source: 'Built-in',
  },
  {
    id: 'init-tg-3',
    type: 'socks5',
    server: '185.199.229.156',
    port: 1080,
    country: 'Finland',
    country_code: 'FI',
    link: 'tg://socks?server=185.199.229.156&port=1080',
    is_alive: true,
    latency_ms: 145,
    source: 'Built-in',
  },
]

export default function DiscoveryHub({ status }: Props) {
  const [activeTab, setActiveTab] = useState<DiscoveryTab>('telegram')

  // --- Telegram Proxies State ---
  const [tgProxies, setTgProxies] = useState<TelegramProxy[]>(() => {
    try {
      const saved = localStorage.getItem('nexvpn_cached_tg_proxies')
      if (saved) {
        const parsed = JSON.parse(saved)
        if (Array.isArray(parsed) && parsed.length > 0) return parsed
      }
    } catch {}
    return INITIAL_STARTER_PROXIES
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
    } catch {}
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
    } catch {}
    return []
  })
  const [cfProgress, setCfProgress] = useState<CloudflareScanProgress | null>(null)
  const [selectedCleanTarget, setSelectedCleanTarget] = useState<CloudflareTarget | null>(null)
  const [isApplyingWarp, setIsApplyingWarp] = useState(false)
  const [isApplyingV2Ray, setIsApplyingV2Ray] = useState(false)

  // Save states across navigation
  useEffect(() => {
    try {
      if (tgProxies && tgProxies.length > 0) {
        localStorage.setItem('nexvpn_cached_tg_proxies', JSON.stringify(tgProxies))
      }
    } catch {}
  }, [tgProxies])

  useEffect(() => {
    try {
      if (freeConfigs && freeConfigs.length > 0) {
        localStorage.setItem('nexvpn_cached_free_configs', JSON.stringify(freeConfigs))
      }
    } catch {}
  }, [freeConfigs])

  useEffect(() => {
    try {
      if (cfResults && cfResults.length > 0) {
        localStorage.setItem('nexvpn_cached_cf_results', JSON.stringify(cfResults))
      }
    } catch {}
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
      const list = await api.fetchTelegramProxies(customTgSources.length > 0 ? customTgSources : undefined)
      if (Array.isArray(list)) {
        setTgProxies(list)
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
      if (Array.isArray(p?.results)) {
        setCfResults(p.results)
      }
      setIsCfScanning(true)
    })

    const unsubCfComplete = api.onEvent('cf_scanner:complete', (results: CloudflareTarget[]) => {
      setIsCfScanning(false)
      if (Array.isArray(results)) {
        setCfResults(results)
        if (results.length > 0) setSelectedCleanTarget(results[0])
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

    return () => {
      if (typeof unsubCfProg === 'function') unsubCfProg()
      if (typeof unsubCfComplete === 'function') unsubCfComplete()
      if (typeof unsubTgProg === 'function') unsubTgProg()
      if (typeof unsubCfgProg === 'function') unsubCfgProg()
      if (typeof unsubFragProg === 'function') unsubFragProg()
      if (typeof unsubFragComplete === 'function') unsubFragComplete()
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
    <div className="flex flex-col h-full overflow-y-auto px-6 py-5 space-y-5 bg-gradient-to-b from-gray-900/40 via-transparent to-transparent text-gray-100">
      {/* Toast Notification */}
      {toastMessage && (
        <div className="fixed top-6 right-6 z-50 flex items-center gap-2.5 px-4 py-3 bg-indigo-600/90 text-white rounded-xl shadow-2xl backdrop-blur-md border border-indigo-400/30 animate-in fade-in slide-in-from-top-4 duration-300">
          <Sparkles className="w-4 h-4 text-indigo-200 animate-pulse" />
          <span className="text-sm font-medium">{toastMessage}</span>
        </div>
      )}

      {/* Header Bar */}
      <div className="sticky top-0 z-30 flex flex-col lg:flex-row lg:items-center justify-between gap-4 bg-slate-950/90 backdrop-blur-xl -mx-6 -mt-5 px-6 py-4 border-b border-gray-800/80 shadow-md">
        <div className="flex items-center gap-3.5">
          <div className="w-11 h-11 rounded-2xl bg-gradient-to-tr from-indigo-600 to-purple-600 flex items-center justify-center shadow-lg shadow-indigo-500/20 ring-1 ring-white/20 shrink-0">
            <Compass className="w-5 h-5 text-white" />
          </div>
          <div>
            <h1 className="text-xl font-bold tracking-tight bg-gradient-to-r from-white via-gray-100 to-gray-400 bg-clip-text text-transparent">
              {t('discovery_title')}
            </h1>
            <p className="text-xs text-gray-400 mt-0.5">
              {t('discovery_subtitle')}
            </p>
          </div>
        </div>

        {/* 4 Main Sub-Tabs Switcher */}
        <div className="grid grid-cols-2 sm:flex sm:flex-wrap items-center p-1 bg-gray-900/90 rounded-xl border border-gray-800/80 shadow-inner gap-1">
          <button
            onClick={() => setActiveTab('telegram')}
            className={`flex items-center gap-2 px-3.5 py-2 rounded-lg text-xs font-semibold transition-all duration-200 ${
              activeTab === 'telegram'
                ? 'bg-gradient-to-r from-blue-600 to-sky-600 text-white shadow-md shadow-blue-500/20'
                : 'text-gray-400 hover:text-gray-200 hover:bg-gray-800/40'
            }`}
          >
            <Send className="w-3.5 h-3.5" />
            <span>{t('tab_telegram_proxies')}</span>
            {tgProxies.length > 0 && (
              <span className="ml-1 px-1.5 py-0.5 text-[10px] rounded-full bg-blue-500/30 text-blue-200 border border-blue-400/20">
                {tgProxies.length}
              </span>
            )}
          </button>

          <button
            onClick={() => setActiveTab('configs')}
            className={`flex items-center gap-2 px-3.5 py-2 rounded-lg text-xs font-semibold transition-all duration-200 ${
              activeTab === 'configs'
                ? 'bg-gradient-to-r from-indigo-600 to-purple-600 text-white shadow-md shadow-indigo-500/20'
                : 'text-gray-400 hover:text-gray-200 hover:bg-gray-800/40'
            }`}
          >
            <Globe className="w-3.5 h-3.5" />
            <span>{t('tab_free_configs')}</span>
            {freeConfigs.length > 0 && (
              <span className="ml-1 px-1.5 py-0.5 text-[10px] rounded-full bg-indigo-500/30 text-indigo-200 border border-indigo-400/20">
                {freeConfigs.length}
              </span>
            )}
          </button>

          <button
            onClick={() => setActiveTab('cloudflare')}
            className={`flex items-center gap-2 px-3.5 py-2 rounded-lg text-xs font-semibold transition-all duration-200 ${
              activeTab === 'cloudflare'
                ? 'bg-gradient-to-r from-amber-600 to-orange-600 text-white shadow-md shadow-orange-500/20'
                : 'text-gray-400 hover:text-gray-200 hover:bg-gray-800/40'
            }`}
          >
            <Zap className="w-3.5 h-3.5" />
            <span>{t('tab_cloudflare_scanner')}</span>
            {cfResults.length > 0 && (
              <span className="ml-1 px-1.5 py-0.5 text-[10px] rounded-full bg-amber-500/30 text-amber-200 border border-amber-400/20">
                {cfResults.length}
              </span>
            )}
          </button>

          <button
            onClick={() => setActiveTab('fragment')}
            className={`flex items-center gap-2 px-3.5 py-2 rounded-lg text-xs font-semibold transition-all duration-200 ${
              activeTab === 'fragment'
                ? 'bg-gradient-to-r from-rose-600 to-pink-600 text-white shadow-md shadow-rose-500/20'
                : 'text-gray-400 hover:text-gray-200 hover:bg-gray-800/40'
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
          <div className="flex flex-wrap items-center justify-between gap-3 p-3.5 bg-gray-900/60 rounded-2xl border border-gray-800/80 backdrop-blur-md shadow-sm">
            <div className="flex flex-wrap items-center gap-2">
              <button
                onClick={handleFetchTelegramProxies}
                disabled={isFetchingTg || isTestingTg}
                className="flex items-center gap-2 px-3.5 py-2 bg-blue-600 hover:bg-blue-500 disabled:opacity-50 text-white rounded-xl text-xs font-semibold transition-all shadow-md shadow-blue-600/20 active:scale-95"
              >
                <RefreshCw className={`w-3.5 h-3.5 ${isFetchingTg ? 'animate-spin' : ''}`} />
                <span>{isFetchingTg ? t('fetching_proxies') : t('fetch_proxies')}</span>
              </button>

              <button
                onClick={handleTestTelegramProxies}
                disabled={isTestingTg || tgProxies.length === 0}
                className="flex items-center gap-2 px-3.5 py-2 bg-emerald-600 hover:bg-emerald-500 disabled:opacity-50 text-white rounded-xl text-xs font-semibold transition-all shadow-md shadow-emerald-600/20 active:scale-95"
              >
                <Activity className={`w-3.5 h-3.5 ${isTestingTg ? 'animate-pulse' : ''}`} />
                <span>{isTestingTg ? t('testing_proxies') : t('test_proxies')}</span>
              </button>

              {tgProxies.some(x => x.is_alive && x.latency_ms > 0) && (
                <button
                  onClick={handleLaunchFastestTelegram}
                  className="flex items-center gap-1.5 px-3.5 py-2 bg-gradient-to-r from-amber-500 to-orange-500 hover:from-amber-400 hover:to-orange-400 text-white rounded-xl text-xs font-semibold shadow-md shadow-orange-500/20 active:scale-95 transition-all"
                >
                  <Zap className="w-3.5 h-3.5 fill-current" />
                  <span>{t('auto_connect_fastest')}</span>
                </button>
              )}

              <button
                onClick={() => handleOpenSourcesModal('telegram')}
                title={t('custom_sources_modal_title')}
                className="flex items-center gap-1.5 px-3 py-2 bg-gray-800/80 hover:bg-gray-700 text-gray-300 hover:text-white rounded-xl text-xs font-medium transition-all"
              >
                <Sliders className="w-3.5 h-3.5 text-blue-400" />
                <span>{t('candidate_sources')}</span>
                {customTgSources.length > 0 && (
                  <span className="w-2 h-2 rounded-full bg-blue-400" />
                )}
              </button>
            </div>

            <div className="flex flex-wrap items-center gap-2">
              <div className="flex items-center p-0.5 bg-gray-950/60 rounded-lg border border-gray-800/60 text-xs">
                <button
                  type="button"
                  onClick={() => setTgTypeFilter('all')}
                  className={`px-3 py-1 rounded-md transition-all font-semibold ${
                    tgTypeFilter === 'all'
                      ? 'bg-blue-600 text-white shadow-sm'
                      : 'text-gray-400 hover:text-gray-200'
                  }`}
                >
                  {t('select_all') || 'همه'}
                </button>

                <button
                  type="button"
                  onClick={() => setTgTypeFilter('mtproto')}
                  className={`flex items-center gap-1.5 px-3 py-1 rounded-md transition-all font-semibold ${
                    tgTypeFilter === 'mtproto'
                      ? 'bg-blue-600 text-white shadow-sm'
                      : 'text-gray-400 hover:text-gray-200'
                  }`}
                >
                  <Send className="w-3 h-3 text-blue-300" />
                  <span dir="ltr">MTProto</span>
                </button>

                <button
                  type="button"
                  onClick={() => setTgTypeFilter('socks5')}
                  className={`flex items-center gap-1.5 px-3 py-1 rounded-md transition-all font-semibold ${
                    tgTypeFilter === 'socks5'
                      ? 'bg-purple-600 text-white shadow-sm'
                      : 'text-gray-400 hover:text-gray-200'
                  }`}
                >
                  <Globe className="w-3 h-3 text-purple-300" />
                  <span dir="ltr">SOCKS5</span>
                </button>

                <button
                  type="button"
                  onClick={() => setTgTypeFilter('http')}
                  className={`flex items-center gap-1.5 px-3 py-1 rounded-md transition-all font-semibold ${
                    tgTypeFilter === 'http'
                      ? 'bg-amber-600 text-white shadow-sm'
                      : 'text-gray-400 hover:text-gray-200'
                  }`}
                >
                  <Layers className="w-3 h-3 text-amber-300" />
                  <span dir="ltr">HTTP</span>
                </button>
              </div>

              <button
                onClick={() => setTgStatusFilter(tgStatusFilter === 'all' ? 'alive' : 'all')}
                className={`flex items-center gap-1.5 px-3 py-1.5 rounded-lg border text-xs transition-all ${
                  tgStatusFilter === 'alive'
                    ? 'bg-emerald-500/20 border-emerald-500/40 text-emerald-300'
                    : 'bg-gray-950/40 border-gray-800/60 text-gray-400 hover:text-gray-200'
                }`}
              >
                <ShieldCheck className="w-3.5 h-3.5" />
                <span>{t('alive_proxies_found')}</span>
              </button>

              <div className="relative">
                <Search className="w-3.5 h-3.5 text-gray-400 absolute left-3 top-2.5" />
                <input
                  type="text"
                  value={tgSearch}
                  onChange={e => setTgSearch(e.target.value)}
                  placeholder={t('search')}
                  className="pl-8 pr-3 py-1.5 bg-gray-950/60 border border-gray-800/60 rounded-lg text-xs text-gray-200 placeholder-gray-500 focus:outline-none focus:border-blue-500/50 w-44"
                />
              </div>
            </div>
          </div>

          {tgProgress && (
            <div className="p-3 bg-blue-950/30 border border-blue-800/30 rounded-xl space-y-1.5 animate-in fade-in duration-200">
              <div className="flex items-center justify-between text-xs text-blue-200">
                <span className="flex items-center gap-2">
                  <Activity className="w-3.5 h-3.5 text-blue-400 animate-spin" />
                  <span>{t('testing_proxies')} ({tgProgress.tested_count} / {tgProgress.total_count})</span>
                </span>
                <span className="font-mono text-gray-400 text-[11px] truncate max-w-xs">{tgProgress.current}</span>
              </div>
              <div className="w-full h-1.5 bg-gray-800 rounded-full overflow-hidden">
                <div
                  className="h-full bg-gradient-to-r from-blue-500 to-cyan-400 transition-all duration-200"
                  style={{
                    width: `${Math.min(100, Math.round((tgProgress.tested_count / (tgProgress.total_count || 1)) * 100))}%`,
                  }}
                />
              </div>
            </div>
          )}

          {filteredTgProxies.length === 0 ? (
            <div className="flex flex-col items-center justify-center py-16 text-center space-y-3 bg-gray-900/30 rounded-2xl border border-gray-800/40">
              <Send className="w-10 h-10 text-gray-600" />
              <p className="text-sm text-gray-400">{t('empty_discovery_results')}</p>
              <button
                onClick={handleFetchTelegramProxies}
                className="px-4 py-2 bg-blue-600/80 hover:bg-blue-600 text-white rounded-xl text-xs font-semibold"
              >
                {t('fetch_proxies')}
              </button>
            </div>
          ) : (
            <div className="grid grid-cols-1 md:grid-cols-2 lg:grid-cols-3 gap-3.5">
              {filteredTgProxies.map(p => (
                <div
                  key={p.id}
                  className="group relative flex flex-col justify-between p-4 bg-gray-900/60 hover:bg-gray-850/80 rounded-2xl border border-gray-800/80 hover:border-blue-500/40 transition-all duration-200 shadow-sm hover:shadow-lg hover:shadow-blue-500/5 backdrop-blur-sm"
                >
                  <div>
                    <div className="flex items-center justify-between gap-2 mb-2.5">
                      <div className="flex items-center gap-2">
                        <CountryFlag codeOrName={p.country_code || p.country || p.server} size="sm" />
                        <span
                          className={`px-2 py-0.5 text-[10px] font-bold uppercase tracking-wider rounded-md ${
                            p.type === 'mtproto'
                              ? 'bg-blue-500/20 text-blue-300 border border-blue-400/20'
                              : 'bg-purple-500/20 text-purple-300 border border-purple-400/20'
                          }`}
                        >
                          {p.type}
                        </span>
                        {p.country && (
                          <span className="text-xs text-gray-400 truncate max-w-[100px]">{p.country}</span>
                        )}
                      </div>

                      {p.latency_ms > 0 ? (
                        <span
                          className={`flex items-center gap-1 text-xs font-mono font-medium px-2 py-0.5 rounded-full ${
                            p.latency_ms < 300
                              ? 'bg-emerald-500/10 text-emerald-400 border border-emerald-500/20'
                              : p.latency_ms < 700
                              ? 'bg-amber-500/10 text-amber-400 border border-amber-500/20'
                              : 'bg-rose-500/10 text-rose-400 border border-rose-500/20'
                          }`}
                        >
                          <Clock className="w-3 h-3" />
                          {p.latency_ms} ms
                        </span>
                      ) : p.is_alive ? (
                        <span className="flex items-center gap-1 text-[11px] text-emerald-400 bg-emerald-500/10 px-2 py-0.5 rounded-full">
                          <CheckCircle2 className="w-3 h-3" />
                          {t('active_status')}
                        </span>
                      ) : (
                        <span className="text-[11px] text-gray-500 font-mono">-- ms</span>
                      )}
                    </div>

                    <div className="font-mono text-sm font-semibold text-gray-200 truncate flex items-center gap-1 mt-1" dir="ltr">
                      <span className="truncate">{p.server}</span>
                      <span className="text-xs text-blue-400 font-normal shrink-0">:{p.port}</span>
                    </div>

                    {p.secret && (
                      <div className="mt-1 text-[11px] text-gray-500 font-mono truncate">
                        Secret: {p.secret.slice(0, 14)}...
                      </div>
                    )}
                  </div>

                  <div className="flex items-center justify-between gap-1.5 mt-4 pt-3 border-t border-gray-800/60">
                    <button
                      onClick={() => handleLaunchTelegram(p)}
                      className="flex-1 flex items-center justify-center gap-1.5 py-1.5 px-3 bg-gradient-to-r from-blue-600 to-sky-600 hover:from-blue-500 hover:to-sky-500 text-white rounded-xl text-xs font-semibold shadow-sm active:scale-95 transition-all"
                    >
                      <ExternalLink className="w-3.5 h-3.5" />
                      <span>{t('open_in_telegram')}</span>
                    </button>

                    <button
                      onClick={() => copyToClipboard(p.link, p.id, t('proxy_copied'))}
                      title={t('copy_proxy_link')}
                      className="p-2 bg-gray-800/60 hover:bg-gray-800 text-gray-300 hover:text-white rounded-xl transition-all"
                    >
                      {copiedId === p.id ? <Check className="w-3.5 h-3.5 text-emerald-400" /> : <Copy className="w-3.5 h-3.5" />}
                    </button>

                    <button
                      onClick={() => handleShowQrModal(`${p.type.toUpperCase()} - ${p.server}:${p.port}`, p.link)}
                      title={t('show_qr_code')}
                      className="p-2 bg-gray-800/60 hover:bg-gray-800 text-gray-300 hover:text-white rounded-xl transition-all"
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
          <div className="flex flex-col gap-3 p-4 bg-gray-900/80 rounded-2xl border border-gray-800/80 backdrop-blur-md shadow-md">
            <div className="flex flex-wrap items-center justify-between gap-2.5">
              <div className="flex flex-wrap items-center gap-2">
                <button
                  onClick={handleFetchFreeConfigs}
                  disabled={isFetchingConfigs || isTestingConfigs}
                  className="flex items-center gap-2 px-3.5 py-2 bg-indigo-600 hover:bg-indigo-500 disabled:opacity-50 text-white rounded-xl text-xs font-semibold transition-all shadow-md shadow-indigo-600/20 active:scale-95"
                >
                  <RefreshCw className={`w-3.5 h-3.5 ${isFetchingConfigs ? 'animate-spin' : ''}`} />
                  <span>{isFetchingConfigs ? t('fetching_configs') : t('fetch_configs')}</span>
                </button>

                <button
                  onClick={handleTestFreeConfigs}
                  disabled={isTestingConfigs || freeConfigs.length === 0}
                  className="flex items-center gap-2 px-3.5 py-2 bg-emerald-600 hover:bg-emerald-500 disabled:opacity-50 text-white rounded-xl text-xs font-semibold transition-all shadow-md shadow-emerald-600/20 active:scale-95"
                >
                  <Activity className={`w-3.5 h-3.5 ${isTestingConfigs ? 'animate-pulse' : ''}`} />
                  <span>{isTestingConfigs ? t('testing_configs') : t('test_latency') || 'تست تأخیر و سلامت'}</span>
                </button>

                {selectedConfigIds.length > 0 && (
                  <button
                    onClick={() => handleImportSelectedConfigs(false)}
                    disabled={isImportingConfigs}
                    className="flex items-center gap-1.5 px-3 py-2 bg-purple-600 hover:bg-purple-500 text-white rounded-xl text-xs font-semibold shadow-md shadow-purple-600/20 transition-all active:scale-95"
                  >
                    <ArrowDownToLine className="w-3.5 h-3.5" />
                    <span>{t('import_selected_configs')} ({selectedConfigIds.length})</span>
                  </button>
                )}

                <button
                  onClick={() => handleImportSelectedConfigs(true)}
                  disabled={isImportingConfigs || freeConfigs.filter(x => x.is_alive).length === 0}
                  className="flex items-center gap-1.5 px-3 py-2 bg-gray-800 hover:bg-gray-700 text-gray-200 rounded-xl text-xs font-semibold transition-all"
                >
                  <Download className="w-3.5 h-3.5 text-emerald-400" />
                  <span>{t('import_all_alive_configs')}</span>
                </button>

                {freeConfigs.length > 0 && (
                  <button
                    onClick={handleExportSubscriptionLink}
                    title="Export subscription URL"
                    className="flex items-center gap-1.5 px-3 py-2 bg-gray-800/80 hover:bg-gray-700 text-gray-300 hover:text-white rounded-xl text-xs font-medium transition-all"
                  >
                    <Share2 className="w-3.5 h-3.5 text-indigo-400" />
                    <span>{t('subscription')}</span>
                  </button>
                )}

                <button
                  onClick={() => handleOpenSourcesModal('configs')}
                  title={t('custom_sources_modal_title')}
                  className="flex items-center gap-1.5 px-3 py-2 bg-gray-800/80 hover:bg-gray-700 text-gray-300 hover:text-white rounded-xl text-xs font-medium transition-all"
                >
                  <Sliders className="w-3.5 h-3.5 text-indigo-400" />
                  <span>{t('candidate_sources')}</span>
                  {customConfigSources.length > 0 && (
                    <span className="w-2 h-2 rounded-full bg-indigo-400" />
                  )}
                </button>
              </div>
            </div>

            <div className="flex flex-wrap items-center justify-between gap-2.5 pt-2 border-t border-gray-800/60">
              <div className="flex flex-wrap items-center gap-2">
                <button
                  onClick={handleSelectAllConfigs}
                  className="flex items-center gap-1.5 px-3 py-1.5 bg-gray-950/60 border border-gray-800/80 rounded-xl text-xs text-gray-300 hover:text-white transition-all font-medium"
                >
                  {selectedConfigIds.length === filteredConfigs.length && filteredConfigs.length > 0 ? (
                    <CheckSquare className="w-3.5 h-3.5 text-indigo-400" />
                  ) : (
                    <SquareOutline className="w-3.5 h-3.5 text-gray-400" />
                  )}
                  <span>{selectedConfigIds.length === filteredConfigs.length ? t('deselect_all') : t('select_all')}</span>
                </button>

                <div className="flex items-center p-0.5 bg-gray-950/80 rounded-xl border border-gray-800/80 text-xs gap-0.5 flex-wrap">
                  <button
                    onClick={() => setConfigProtoFilter('all')}
                    className={`px-3 py-1 rounded-lg uppercase transition-all font-bold ${
                      configProtoFilter === 'all'
                        ? 'bg-indigo-600 text-white shadow-sm'
                        : 'text-gray-400 hover:text-gray-200'
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
                          ? 'bg-indigo-600 text-white shadow-sm'
                          : 'text-gray-400 hover:text-gray-200'
                      }`}
                    >
                      {proto}
                    </button>
                  ))}
                </div>
              </div>

              <div className="relative">
                <Search className="w-3.5 h-3.5 text-gray-400 absolute left-3 top-2.5" />
                <input
                  type="text"
                  value={configSearch}
                  onChange={e => setConfigSearch(e.target.value)}
                  placeholder={t('search')}
                  className="pl-8 pr-3 py-1.5 bg-gray-950/80 border border-gray-800/80 rounded-xl text-xs text-gray-200 placeholder-gray-500 focus:outline-none focus:border-indigo-500/50 w-48"
                />
              </div>
            </div>
          </div>

          {configsProgress && (
            <div className="p-3 bg-indigo-950/30 border border-indigo-800/30 rounded-xl space-y-1.5 animate-in fade-in duration-200">
              <div className="flex items-center justify-between text-xs text-indigo-200">
                <span className="flex items-center gap-2">
                  <Activity className="w-3.5 h-3.5 text-indigo-400 animate-spin" />
                  <span>{t('testing_configs')} ({configsProgress.tested_count} / {configsProgress.total_count})</span>
                </span>
                <span className="font-mono text-gray-400 text-[11px] truncate max-w-xs">{configsProgress.current}</span>
              </div>
              <div className="w-full h-1.5 bg-gray-800 rounded-full overflow-hidden">
                <div
                  className="h-full bg-gradient-to-r from-indigo-500 to-purple-400 transition-all duration-200"
                  style={{
                    width: `${Math.min(100, Math.round((configsProgress.tested_count / (configsProgress.total_count || 1)) * 100))}%`,
                  }}
                />
              </div>
            </div>
          )}

          {filteredConfigs.length === 0 ? (
            <div className="flex flex-col items-center justify-center py-16 text-center space-y-3 bg-gray-900/30 rounded-2xl border border-gray-800/40">
              <Globe className="w-10 h-10 text-gray-600" />
              <p className="text-sm text-gray-400">{t('empty_discovery_results')}</p>
              <button
                onClick={handleFetchFreeConfigs}
                className="px-4 py-2 bg-indigo-600/80 hover:bg-indigo-600 text-white rounded-xl text-xs font-semibold"
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
                        ? 'bg-indigo-950/40 border-indigo-500/50 shadow-md shadow-indigo-500/5'
                        : 'bg-gray-900/60 hover:bg-gray-850/80 border-gray-800/80'
                    }`}
                  >
                    <div className="flex items-center gap-3 min-w-0">
                      <button
                        type="button"
                        onClick={e => {
                          e.stopPropagation()
                          toggleSelectConfig(c.id)
                        }}
                        className="text-gray-400 hover:text-white"
                      >
                        {isSelected ? (
                          <CheckSquare className="w-4 h-4 text-indigo-400" />
                        ) : (
                          <SquareOutline className="w-4 h-4 text-gray-500" />
                        )}
                      </button>

                      <CountryFlag codeOrName={c.country_code || c.country || c.name || c.server} size="sm" />

                      <span className="px-2 py-0.5 text-[10px] font-bold uppercase rounded-md bg-indigo-500/20 text-indigo-300 border border-indigo-400/20">
                        {itemProto}
                      </span>

                      <div className="min-w-0">
                        <div className="flex items-center gap-2">
                          <span className="text-xs font-semibold text-gray-200 truncate max-w-sm">
                            {c.name || `${c.server}:${c.port}`}
                          </span>
                          {c.country && (
                            <span className="text-[10px] px-1.5 py-0.2 rounded bg-slate-800 text-slate-400 border border-slate-700/50">
                              {c.country}
                            </span>
                          )}
                        </div>
                        <div className="text-[11px] font-mono text-gray-400 truncate max-w-xs">
                          {c.server}:{c.port} {c.source_name ? `• ${c.source_name}` : ''}
                        </div>
                      </div>
                    </div>

                    <div className="flex items-center gap-3">
                      {c.latency_ms > 0 ? (
                        <span
                          className={`flex items-center gap-1 text-xs font-mono font-medium px-2 py-0.5 rounded-full ${
                            c.latency_ms < 300
                              ? 'bg-emerald-500/10 text-emerald-400 border border-emerald-500/20'
                              : c.latency_ms < 700
                              ? 'bg-amber-500/10 text-amber-400 border border-amber-500/20'
                              : 'bg-rose-500/10 text-rose-400 border border-rose-500/20'
                          }`}
                        >
                          <Clock className="w-3 h-3" />
                          {c.latency_ms} ms
                        </span>
                      ) : c.is_alive ? (
                        <span className="flex items-center gap-1 text-[11px] text-emerald-400 bg-emerald-500/10 px-2 py-0.5 rounded-full">
                          <CheckCircle2 className="w-3 h-3" />
                          OK
                        </span>
                      ) : (
                        <span className="text-xs text-gray-500 font-mono">-- ms</span>
                      )}

                      <div className="flex items-center gap-1" onClick={e => e.stopPropagation()}>
                        <button
                          onClick={() => copyToClipboard(c.raw_uri, c.id)}
                          title={t('copy')}
                          className="p-1.5 bg-gray-800/60 hover:bg-gray-800 text-gray-400 hover:text-white rounded-lg transition-all"
                        >
                          {copiedId === c.id ? <Check className="w-3.5 h-3.5 text-emerald-400" /> : <Copy className="w-3.5 h-3.5" />}
                        </button>

                        <button
                          onClick={() => handleShowQrModal(c.name, c.raw_uri)}
                          title={t('show_qr_code')}
                          className="p-1.5 bg-gray-800/60 hover:bg-gray-800 text-gray-400 hover:text-white rounded-lg transition-all"
                        >
                          <QrCode className="w-3.5 h-3.5" />
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
          <div className="flex flex-wrap items-center justify-between gap-3 p-4 bg-gray-900/60 rounded-2xl border border-gray-800/80 backdrop-blur-md shadow-sm">
            <div className="flex flex-wrap items-center gap-2.5">
              {/* Dual Mode Switcher */}
              <div className="flex items-center p-0.5 bg-gray-950/60 rounded-xl border border-gray-800/60 text-xs">
                <button
                  onClick={() => setCfMode('warp')}
                  disabled={isCfScanning}
                  className={`flex items-center gap-1.5 px-3 py-1.5 rounded-lg font-medium transition-all ${
                    cfMode === 'warp'
                      ? 'bg-amber-600 text-white shadow-sm'
                      : 'text-gray-400 hover:text-gray-200'
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
                      ? 'bg-amber-600 text-white shadow-sm'
                      : 'text-gray-400 hover:text-gray-200'
                  }`}
                >
                  <Globe className="w-3.5 h-3.5" />
                  <span>{t('cf_mode_cdn')}</span>
                </button>
              </div>

              {/* ISP Filter Pills */}
              <div className="flex items-center p-0.5 bg-gray-950/60 rounded-xl border border-gray-800/60 text-[11px]">
                {(['all', 'mci', 'mtn', 'rtl'] as const).map(isp => (
                  <button
                    key={isp}
                    onClick={() => setCfIspFilter(isp)}
                    className={`px-2.5 py-1 rounded-lg transition-all ${
                      cfIspFilter === isp
                        ? 'bg-amber-500/20 text-amber-300 font-semibold border border-amber-500/30'
                        : 'text-gray-400 hover:text-gray-200'
                    }`}
                  >
                    {isp === 'all' ? t('isp_filter_all') : isp === 'mci' ? t('isp_filter_mci') : isp === 'mtn' ? t('isp_filter_mtn') : t('isp_filter_rtl')}
                  </button>
                ))}
              </div>

              {!isCfScanning ? (
                <button
                  onClick={handleStartCfScan}
                  className="flex items-center gap-2 px-4 py-2 bg-gradient-to-r from-amber-600 to-orange-600 hover:from-amber-500 hover:to-orange-500 text-white rounded-xl text-xs font-semibold shadow-md shadow-amber-600/20 active:scale-95 transition-all"
                >
                  <Play className="w-3.5 h-3.5 fill-current" />
                  <span>{t('start_scanner')}</span>
                </button>
              ) : (
                <button
                  onClick={handleStopCfScan}
                  className="flex items-center gap-2 px-4 py-2 bg-rose-600 hover:bg-rose-500 text-white rounded-xl text-xs font-semibold shadow-md shadow-rose-600/20 active:scale-95 transition-all"
                >
                  <Square className="w-3.5 h-3.5 fill-current" />
                  <span>{t('stop_scanner')}</span>
                </button>
              )}
            </div>

            <div className="flex items-center gap-3 text-xs">
              <div className="px-3 py-1.5 bg-gray-950/60 rounded-xl border border-gray-800/60">
                <span className="text-gray-400">{t('alive_proxies_found')}: </span>
                <span className="font-bold text-amber-400">{cfResults.length}</span>
              </div>
            </div>
          </div>

          {isCfScanning && cfProgress && (
            <div className="p-3 bg-amber-950/30 border border-amber-800/30 rounded-xl space-y-1.5 animate-in fade-in duration-200">
              <div className="flex items-center justify-between text-xs text-amber-200">
                <span className="flex items-center gap-2">
                  <Activity className="w-3.5 h-3.5 text-amber-400 animate-spin" />
                  <span>{t('scanning')} ({cfProgress.scanned} / {cfProgress.total})</span>
                </span>
                <span className="font-mono text-gray-400 text-[11px]">{cfProgress.current_ip}</span>
              </div>
              <div className="w-full h-1.5 bg-gray-800 rounded-full overflow-hidden">
                <div
                  className="h-full bg-gradient-to-r from-amber-500 to-orange-400 transition-all duration-200"
                  style={{
                    width: `${Math.min(100, Math.round((cfProgress.scanned / (cfProgress.total || 1)) * 100))}%`,
                  }}
                />
              </div>
            </div>
          )}

          {cfResults.length === 0 ? (
            <div className="flex flex-col items-center justify-center py-16 text-center space-y-3 bg-gray-900/30 rounded-2xl border border-gray-800/40">
              <Zap className="w-10 h-10 text-gray-600" />
              <p className="text-sm text-gray-400">{t('empty_discovery_results')}</p>
              <button
                onClick={handleStartCfScan}
                className="px-4 py-2 bg-amber-600/80 hover:bg-amber-600 text-white rounded-xl text-xs font-semibold"
              >
                {t('start_scanner')}
              </button>
            </div>
          ) : (
            <div className="grid grid-cols-1 md:grid-cols-2 lg:grid-cols-3 gap-3.5">
              {cfResults.map((target, idx) => (
                <div
                  key={`${target.ip}:${target.port}-${idx}`}
                  className="group flex flex-col justify-between p-4 bg-gray-900/60 hover:bg-gray-850/80 rounded-2xl border border-gray-800/80 hover:border-amber-500/40 transition-all duration-200 shadow-sm hover:shadow-lg hover:shadow-amber-500/5 backdrop-blur-sm"
                >
                  <div>
                    <div className="flex items-center justify-between mb-2">
                      <span className="px-2 py-0.5 text-[10px] font-bold uppercase rounded-md bg-amber-500/20 text-amber-300 border border-amber-400/20">
                        {target.mode.toUpperCase()}
                      </span>

                      <div className="flex items-center gap-1.5">
                        {target.loss_rate > 0 && (
                          <span className="text-[10px] text-rose-400 font-mono">
                            {target.loss_rate}% loss
                          </span>
                        )}
                        <span
                          className={`flex items-center gap-1 text-xs font-mono font-medium px-2 py-0.5 rounded-full ${
                            target.latency_ms < 200
                              ? 'bg-emerald-500/10 text-emerald-400 border border-emerald-500/20'
                              : target.latency_ms < 500
                              ? 'bg-amber-500/10 text-amber-400 border border-amber-500/20'
                              : 'bg-rose-500/10 text-rose-400 border border-rose-500/20'
                          }`}
                        >
                          <Clock className="w-3 h-3" />
                          {target.latency_ms} ms
                        </span>
                      </div>
                    </div>

                    <div className="font-mono text-sm font-semibold text-gray-200 truncate">
                      {target.ip}
                      <span className="text-xs text-gray-400 font-normal ml-1">:{target.port}</span>
                    </div>

                    {target.location && (
                      <div className="mt-1 text-[11px] text-gray-400 truncate">
                        {target.location}
                      </div>
                    )}
                  </div>

                  <div className="flex items-center justify-between gap-1.5 mt-4 pt-3 border-t border-gray-800/60">
                    {target.mode === 'warp' ? (
                      <>
                        <button
                          type="button"
                          onClick={() => handleGenerateWarpConfig(target)}
                          disabled={isGeneratingWarp === `${target.ip}:${target.port}`}
                          className="flex-1 flex items-center justify-center gap-1.5 py-1.5 px-3 bg-gradient-to-r from-amber-600 to-orange-600 hover:from-amber-500 hover:to-orange-500 text-white rounded-xl text-xs font-bold shadow-md shadow-orange-600/20 active:scale-95 transition-all"
                        >
                          <Zap className={`w-3.5 h-3.5 fill-current ${isGeneratingWarp === `${target.ip}:${target.port}` ? 'animate-spin' : ''}`} />
                          <span>{isGeneratingWarp === `${target.ip}:${target.port}` ? (t('generating') || 'در حال ساخت...') : (t('generate_warp_config') || '⚡ ساخت کانفیگ وارپ')}</span>
                        </button>
                        <button
                          type="button"
                          onClick={() => handleApplyToWarpProfiles(target)}
                          disabled={isApplyingWarp}
                          title={t('apply_to_warp_profiles') || 'اعمال به تمام پروفایل‌های موجود'}
                          className="p-2 bg-gray-800/60 hover:bg-gray-800 text-amber-300 hover:text-white rounded-xl transition-all"
                        >
                          <Sparkles className="w-3.5 h-3.5" />
                        </button>
                        <button
                          type="button"
                          onClick={() => handleApplyToV2RayProfiles(target, true)}
                          disabled={isApplyingV2Ray}
                          title="Apply to CDN V2Ray Profiles"
                          className="p-2 bg-gray-800/60 hover:bg-gray-800 text-amber-300 hover:text-white rounded-xl transition-all"
                        >
                          <Globe className="w-3.5 h-3.5" />
                        </button>
                      </>
                    ) : (
                      <>
                        <button
                          type="button"
                          onClick={() => handleApplyToV2RayProfiles(target, true)}
                          disabled={isApplyingV2Ray}
                          className="flex-1 flex items-center justify-center gap-1.5 py-1.5 px-3 bg-gradient-to-r from-amber-600 to-orange-600 hover:from-amber-500 hover:to-orange-500 text-white rounded-xl text-xs font-bold shadow-md shadow-orange-600/20 active:scale-95 transition-all"
                        >
                          <Globe className="w-3.5 h-3.5" />
                          <span>{t('apply_to_v2ray_profiles')}</span>
                        </button>
                        <button
                          type="button"
                          onClick={() => handleGenerateWarpConfig(target)}
                          disabled={isGeneratingWarp === `${target.ip}:${target.port}`}
                          title={t('generate_warp_config') || 'ساخت کانفیگ جدید وارپ با این IP'}
                          className="p-2 bg-gray-800/60 hover:bg-gray-800 text-amber-300 hover:text-white rounded-xl transition-all"
                        >
                          <Zap className="w-3.5 h-3.5" />
                        </button>
                        <button
                          type="button"
                          onClick={() => handleApplyToWarpProfiles(target)}
                          disabled={isApplyingWarp}
                          title={t('apply_to_warp_profiles') || 'اعمال به تمام پروفایل‌های موجود'}
                          className="p-2 bg-gray-800/60 hover:bg-gray-800 text-amber-300 hover:text-white rounded-xl transition-all"
                        >
                          <Sparkles className="w-3.5 h-3.5" />
                        </button>
                      </>
                    )}

                    <button
                      type="button"
                      onClick={() => copyToClipboard(`${target.ip}:${target.port}`, `${target.ip}:${target.port}`, t('clean_ip_copied'))}
                      title={t('copy_clean_ip')}
                      className="p-2 bg-gray-800/60 hover:bg-gray-800 text-gray-300 hover:text-white rounded-xl transition-all"
                    >
                      {copiedId === `${target.ip}:${target.port}` ? (
                        <Check className="w-3.5 h-3.5 text-emerald-400" />
                      ) : (
                        <Copy className="w-3.5 h-3.5" />
                      )}
                    </button>
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
          <div className="p-4 bg-gray-900/60 rounded-2xl border border-gray-800/80 backdrop-blur-md flex flex-col md:flex-row items-start md:items-center justify-between gap-4">
            <div className="flex items-center gap-3">
              <div className="w-10 h-10 rounded-xl bg-rose-500/20 border border-rose-500/30 flex items-center justify-center text-rose-400">
                <Flame className="w-5 h-5" />
              </div>
              <div>
                <h3 className="text-sm font-bold text-gray-200">{t('tab_fragment_scanner')}</h3>
                <p className="text-xs text-gray-400 mt-0.5">{t('fragment_scanner_desc')}</p>
              </div>
            </div>

            <div className="flex items-center gap-2">
              {!isFragScanning ? (
                <button
                  onClick={handleStartFragScan}
                  className="flex items-center gap-2 px-4 py-2 bg-gradient-to-r from-rose-600 to-pink-600 hover:from-rose-500 hover:to-pink-500 text-white rounded-xl text-xs font-semibold shadow-md shadow-rose-600/20 active:scale-95 transition-all"
                >
                  <Play className="w-3.5 h-3.5 fill-current" />
                  <span>{t('start_scanner')}</span>
                </button>
              ) : (
                <button
                  onClick={handleStopFragScan}
                  className="flex items-center gap-2 px-4 py-2 bg-rose-600 hover:bg-rose-500 text-white rounded-xl text-xs font-semibold shadow-md shadow-rose-600/20 active:scale-95 transition-all"
                >
                  <Square className="w-3.5 h-3.5 fill-current" />
                  <span>{t('stop_scanner')}</span>
                </button>
              )}
            </div>
          </div>

          {/* Real-time Scanning Progress */}
          {isFragScanning && fragProgress && (
            <div className="p-3 bg-rose-950/30 border border-rose-800/30 rounded-xl space-y-1.5 animate-in fade-in duration-200">
              <div className="flex items-center justify-between text-xs text-rose-200">
                <span className="flex items-center gap-2">
                  <Activity className="w-3.5 h-3.5 text-rose-400 animate-spin" />
                  <span>{t('scanning')} ({fragProgress.scanned} / {fragProgress.total})</span>
                </span>
                <span className="font-mono text-gray-400 text-[11px]">{fragProgress.current}</span>
              </div>
              <div className="w-full h-1.5 bg-gray-800 rounded-full overflow-hidden">
                <div
                  className="h-full bg-gradient-to-r from-rose-500 to-pink-400 transition-all duration-200"
                  style={{
                    width: `${Math.min(100, Math.round((fragProgress.scanned / (fragProgress.total || 1)) * 100))}%`,
                  }}
                />
              </div>
            </div>
          )}

          {/* Optimal Setting Spotlight Banner */}
          {bestFragment && (
            <div className="p-4 bg-gradient-to-r from-emerald-950/40 via-gray-900/60 to-rose-950/30 rounded-2xl border border-emerald-500/40 shadow-lg flex flex-col md:flex-row md:items-center justify-between gap-4 animate-in fade-in slide-in-from-top-2 duration-300">
              <div className="flex items-center gap-3">
                <div className="w-10 h-10 rounded-xl bg-emerald-500/20 border border-emerald-500/30 flex items-center justify-center text-emerald-400">
                  <Award className="w-5 h-5" />
                </div>
                <div>
                  <div className="flex items-center gap-2">
                    <span className="text-xs font-bold text-emerald-400 uppercase tracking-wider">
                      {t('optimal_fragment_found')}
                    </span>
                    <span className="px-2 py-0.5 text-[10px] font-bold rounded-full bg-emerald-500/20 text-emerald-300 border border-emerald-500/30">
                      {bestFragment.success_rate}% {t('success_rate')}
                    </span>
                  </div>
                  <div className="text-xs text-gray-300 font-mono mt-1">
                    {t('chunk_size')}: <span className="text-white font-bold">{bestFragment.size_range}</span> | {t('delay_sleep')}: <span className="text-white font-bold">{bestFragment.sleep_range} ms</span> ({t('latency')}: {bestFragment.avg_latency} ms)
                  </div>
                </div>
              </div>

              <button
                onClick={() => handleApplyOptimalFragment(bestFragment)}
                disabled={isApplyingFrag}
                className="flex items-center gap-2 px-4 py-2 bg-gradient-to-r from-emerald-600 to-teal-600 hover:from-emerald-500 hover:to-teal-500 text-white rounded-xl text-xs font-semibold shadow-md shadow-emerald-600/20 active:scale-95 transition-all shrink-0"
              >
                <Sparkles className="w-3.5 h-3.5" />
                <span>{t('apply_optimal_fragment')}</span>
              </button>
            </div>
          )}

          {/* Candidates List Table */}
          {fragResults.length === 0 ? (
            <div className="flex flex-col items-center justify-center py-16 text-center space-y-3 bg-gray-900/30 rounded-2xl border border-gray-800/40">
              <Flame className="w-10 h-10 text-gray-600" />
              <p className="text-sm text-gray-400">{t('empty_discovery_results')}</p>
              <button
                onClick={handleStartFragScan}
                className="px-4 py-2 bg-rose-600/80 hover:bg-rose-600 text-white rounded-xl text-xs font-semibold"
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
                      ? 'bg-emerald-950/30 border-emerald-500/50 shadow-md shadow-emerald-500/5'
                      : cand.status === 'optimal'
                      ? 'bg-gray-900/60 border-gray-800 hover:border-gray-700'
                      : 'bg-gray-950/40 border-gray-900 opacity-70'
                  }`}
                >
                  <div className="flex items-center gap-3">
                    <div
                      className={`w-8 h-8 rounded-lg flex items-center justify-center font-mono font-bold text-xs ${
                        cand.success_rate >= 80
                          ? 'bg-emerald-500/20 text-emerald-300'
                          : cand.success_rate >= 50
                          ? 'bg-amber-500/20 text-amber-300'
                          : 'bg-rose-500/20 text-rose-400'
                      }`}
                    >
                      {cand.success_rate}%
                    </div>

                    <div>
                      <div className="text-xs font-bold text-gray-200 flex items-center gap-2">
                        <span>Size: <span className="font-mono text-indigo-300">{cand.size_range}</span></span>
                        <span className="text-gray-500">•</span>
                        <span>Sleep: <span className="font-mono text-purple-300">{cand.sleep_range} ms</span></span>
                        {cand.recommended && (
                          <span className="px-2 py-0.5 text-[10px] bg-emerald-500/20 text-emerald-300 border border-emerald-500/30 rounded-md font-bold">
                            TOP 1
                          </span>
                        )}
                      </div>
                      <div className="text-[11px] text-gray-400 font-mono mt-0.5">
                        Score: {cand.score}/100 • Latency: {cand.avg_latency > 0 ? `${cand.avg_latency} ms` : '--'}
                      </div>
                    </div>
                  </div>

                  <button
                    onClick={() => handleApplyOptimalFragment(cand)}
                    disabled={isApplyingFrag}
                    className="flex items-center gap-1.5 px-3 py-1.5 bg-gray-800 hover:bg-gray-700 text-gray-200 hover:text-white rounded-lg text-xs font-semibold transition-all active:scale-95"
                  >
                    <Sparkles className="w-3 h-3 text-emerald-400" />
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
        <div className="fixed inset-0 z-50 flex items-center justify-center p-4 bg-black/75 backdrop-blur-sm animate-in fade-in duration-200">
          <div className="bg-gray-900 border border-gray-800 rounded-3xl p-6 max-w-lg w-full shadow-2xl space-y-4">
            <div className="flex items-center justify-between">
              <div className="flex items-center gap-2">
                <Sliders className="w-4 h-4 text-indigo-400" />
                <h3 className="text-sm font-bold text-gray-100">
                  {t('custom_sources_modal_title')} ({sourcesModalTab === 'telegram' ? 'Telegram' : 'Free Configs'})
                </h3>
              </div>
              <button
                onClick={() => setSourcesModalTab(null)}
                className="p-1 text-gray-400 hover:text-white rounded-lg"
              >
                <X className="w-4 h-4" />
              </button>
            </div>

            <p className="text-xs text-gray-400">
              {t('custom_sources_desc')}
            </p>

            <textarea
              rows={6}
              value={sourcesInputText}
              onChange={e => setSourcesInputText(e.target.value)}
              placeholder="https://raw.githubusercontent.com/user/repo/main/list.txt&#10;https://example.com/sub/configs"
              className="w-full p-3 bg-gray-950/80 border border-gray-800/80 rounded-xl text-xs font-mono text-gray-200 placeholder-gray-600 focus:outline-none focus:border-indigo-500/50 resize-y"
            />

            <div className="flex items-center justify-between gap-3 pt-2">
              <button
                type="button"
                onClick={() => setSourcesInputText('')}
                className="text-xs text-gray-400 hover:text-rose-400 transition"
              >
                {t('clear_all')}
              </button>

              <div className="flex items-center gap-2">
                <button
                  onClick={() => setSourcesModalTab(null)}
                  className="px-4 py-2 bg-gray-800 hover:bg-gray-700 text-gray-300 rounded-xl text-xs font-semibold"
                >
                  {t('cancel')}
                </button>
                <button
                  onClick={handleSaveCustomSources}
                  className="px-4 py-2 bg-indigo-600 hover:bg-indigo-500 text-white rounded-xl text-xs font-semibold shadow-md shadow-indigo-600/20"
                >
                  {t('custom_sources_save')}
                </button>
              </div>
            </div>
          </div>
        </div>
      )}

      {/* ======================================================== */}
      {/* QR Code Modal                                            */}
      {/* ======================================================== */}
      {qrModalData && (
        <div className="fixed inset-0 z-50 flex items-center justify-center p-4 bg-black/70 backdrop-blur-sm animate-in fade-in duration-200">
          <div className="bg-gray-900 border border-gray-800 rounded-3xl p-6 max-w-sm w-full shadow-2xl space-y-4 text-center">
            <div className="flex items-center justify-between">
              <h3 className="text-sm font-semibold text-gray-200 truncate">{qrModalData.title}</h3>
              <button
                onClick={() => setQrModalData(null)}
                className="p-1 text-gray-400 hover:text-white rounded-lg"
              >
                <X className="w-4 h-4" />
              </button>
            </div>

            <div
              className="p-4 bg-white rounded-2xl flex items-center justify-center shadow-inner"
              dangerouslySetInnerHTML={{ __html: qrModalData.qrSvg }}
            />

            <div className="p-2.5 bg-gray-950/60 rounded-xl text-left border border-gray-800/60">
              <p className="font-mono text-[11px] text-gray-400 break-all select-all line-clamp-3">
                {qrModalData.link}
              </p>
            </div>

            <div className="flex gap-2">
              <button
                onClick={() => copyToClipboard(qrModalData.link, 'modal_qr')}
                className="flex-1 py-2 px-3 bg-blue-600 hover:bg-blue-500 text-white rounded-xl text-xs font-semibold"
              >
                {t('copy')}
              </button>
              <button
                onClick={() => setQrModalData(null)}
                className="py-2 px-4 bg-gray-800 hover:bg-gray-700 text-gray-300 rounded-xl text-xs font-semibold"
              >
                {t('close')}
              </button>
            </div>
          </div>
        </div>
      )}
    </div>
  )
}
