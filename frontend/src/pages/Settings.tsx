import React, { useState, useEffect, useRef } from 'react'
import {
  Shield,
  Wifi,
  RefreshCw,
  Globe,
  Sparkles,
  Radio,
  Eraser,
  Minimize2,
  Bell,
  Terminal,
  AlertTriangle,
  Download,
  CheckCircle2,
  AlertCircle,
  Cpu,
  Layers,
  Zap,
  Split,
  Clock,
  Link,
  Plus,
  X,
  Palette,
  Gamepad2,
  Volume2,
  Keyboard,
  Network,
  Lock,
  Upload,
  Activity,
  Wrench,
  Trash2,
  Settings as SettingsIcon,
  Search,
  SlidersHorizontal,
} from 'lucide-react'
import { api } from '../lib/api'
import AutomationPanel from '../components/AutomationPanel'
import DiagnosticsPanel from '../components/DiagnosticsPanel'
import { t, setLanguage, getLanguage, type Language } from '../lib/i18n'
import { THEMES, getTheme, applyTheme, Theme } from '../lib/theme'
import { sound } from '../lib/sound'
import { reportFailure } from '../lib/utils'
import { useTimers } from '../lib/useTimers'
import type {
  Settings,
  BinaryInfo,
  DiagnosticReport,
  DiagnosticRepairResult,
  SNIPoolItem,
  QuotaAlertInfo,
  DNSScanResult,
  ConnectionState,
} from '../types'
import { Button } from '@/components/ui/button'
import { Card } from '@/components/ui/card'

function errorMessage(err: unknown, fallback: string): string {
  if (err instanceof Error) return err.message
  if (typeof err === 'string' && err) return err
  return fallback
}

const defaultSettings: Settings = {
  auto_connect: false,
  start_minimized: false,
  start_on_boot: false,
  kill_switch_enabled: false,
  dns_server: '1.1.1.1',
  dns_fallback: '8.8.8.8',
  language: 'fa',
  theme: 'cyber-dark',
  routing_mode: 'bypass_iran',
  proxy_mode: 'tun',
  tun_system_proxy: 'dual',
  real_delay_url: 'http://cp.cloudflare.com/generate_204',
  last_profile_id: '',
  log_level: 'info',
  max_log_entries: 1000,
  auto_reconnect: true,
  reconnect_attempts: 3,
  warp_clean_ip: '162.159.192.1',
  warp_port: 2408,
  fragment_enabled: false,
  fragment_size: '100-200',
  fragment_sleep: '10-20',
  app_routing_mode: 'all',
  selected_apps: ['telegram.exe', 'discord.exe', 'chrome.exe'],
  auto_failover: true,
  auto_failover_enabled: true,
  subscription_auto_update_hours: 12,
  multi_hop_warp: false,
  show_mini_widget: false,
  v2ray_core: 'auto',
  custom_sni_pool: [],
}

const popularApps = [
  { name: 'Telegram', exe: 'telegram.exe' },
  { name: 'Discord', exe: 'discord.exe' },
  { name: 'Chrome', exe: 'chrome.exe' },
  { name: 'Spotify', exe: 'spotify.exe' },
  { name: 'Steam', exe: 'steam.exe' },
  { name: 'IDM', exe: 'idman.exe' },
]

type SettingsTab = 'network' | 'antidpi' | 'apps' | 'appearance' | 'system' | 'automation' | 'diagnostics'

interface SettingsPageProps {
  onSettingsSaved?: (s: Settings) => void
  /**
   * Live connection state. Only the diagnostics tab needs it, to explain that a
   * leak test on an untunnelled connection proves nothing.
   */
  connectionState?: ConnectionState
}

/**
 * Lets every SettingRow report whether it currently matches the search query,
 * so the page can show a "no results" hint without duplicating the label text
 * in a separate registry.
 */
const SettingMatchContext = React.createContext<((id: string, match: boolean) => void) | null>(null)

export default function SettingsPage({ onSettingsSaved, connectionState }: SettingsPageProps = {}) {
  const [settings, setSettings] = useState<Settings>(defaultSettings)
  const safeTimeout = useTimers()
  const [activeTab, setActiveTab] = useState<SettingsTab>('network')
  const [selectedTheme, setSelectedTheme] = useState<Theme>(getTheme())
  const [isSaving, setIsSaving] = useState(false)
  const [saved, setSaved] = useState(false)
  const [query, setQuery] = useState('')
  const [matchCount, setMatchCount] = useState(0)
  const matchedIds = React.useRef<Set<string>>(new Set())

  const reportSettingMatch = React.useCallback((id: string, match: boolean) => {
    const set = matchedIds.current
    if (match) set.add(id)
    else set.delete(id)
    setMatchCount(set.size)
  }, [])

  // Each tab renders a different set of rows, so reset the registry when the
  // view changes; rows re-report on mount.
  useEffect(() => {
    matchedIds.current = new Set()
    setMatchCount(0)
  }, [activeTab])
  const [clearingProxy, setClearingProxy] = useState(false)
  const [proxyCleared, setProxyCleared] = useState(false)
  const [binaries, setBinaries] = useState<Record<string, BinaryInfo>>({})
  const [isInstallingCores, setIsInstallingCores] = useState(false)
  const [updatingCore, setUpdatingCore] = useState<string | null>(null)
  const [downloadProgress, setDownloadProgress] = useState<{ key: string; percent: number; speed: number; status: string } | null>(null)
  const [updatingGeo, setUpdatingGeo] = useState(false)
  const [geoUpdated, setGeoUpdated] = useState(false)
  const [newAppInput, setNewAppInput] = useState('')

  // Diagnostics & Network Recovery
  const [diagnosticReport, setDiagnosticReport] = useState<DiagnosticReport | null>(null)
  const [isRunningDiagnostics, setIsRunningDiagnostics] = useState(false)
  const [isRepairing, setIsRepairing] = useState(false)
  const [repairResult, setRepairResult] = useState<DiagnosticRepairResult | null>(null)
  const [flushingDNS, setFlushingDNS] = useState(false)
  const [dnsFlushed, setDnsFlushed] = useState(false)

  // Smart DNS Scanner
  const [isScanningDNS, setIsScanningDNS] = useState(false)
  const [dnsScanResult, setDnsScanResult] = useState<DNSScanResult[] | null>(null)

  // Encrypted Backup & Restore
  const [backupPassword, setBackupPassword] = useState('')
  const [isExportingBackup, setIsExportingBackup] = useState(false)
  const [importBackupString, setImportBackupString] = useState('')
  const [importPassword, setImportPassword] = useState('')
  const [isImportingBackup, setIsImportingBackup] = useState(false)
  const [backupError, setBackupError] = useState<string | null>(null)
  const [backupSuccess, setBackupSuccess] = useState<string | null>(null)

  // Dynamic SNI Pool & MTU & Quota States
  const [sniPool, setSniPool] = useState<SNIPoolItem[]>([])
  const [isAuditingSNI, setIsAuditingSNI] = useState(false)
  const [optimizingMTU, setOptimizingMTU] = useState(false)
  const [optimizedMTUResult, setOptimizedMTUResult] = useState<number | null>(null)
  const [checkingQuota, setCheckingQuota] = useState(false)
  const [quotaResult, setQuotaResult] = useState<QuotaAlertInfo | null>(null)
  const [quotaCheckUrl, setQuotaCheckUrl] = useState('')

  useEffect(() => {
    api.getSettings().then(s => {
      if (s) {
        setSettings(s)
        if (s.theme) {
          setSelectedTheme(s.theme as Theme)
          applyTheme(s.theme as Theme)
        }
      }
    }).catch(err => console.error('Failed to load settings:', err))
    api.checkCoreBinaries().then(b => {
      if (b) setBinaries(b)
    }).catch(err => console.error('Failed to check binaries:', err))
    api.getSNIPoolList().then(p => {
      if (p && p.length > 0) setSniPool(p)
    }).catch(() => { })

    // Listen for core download progress
    const win = window as any
    if (win.runtime && win.runtime.EventsOn) {
      const unsub = win.runtime.EventsOn('core:download_progress', (data: any) => {
        if (data && data.key) {
          setDownloadProgress({
            key: data.key,
            percent: data.percent || 0,
            speed: data.speed || 0,
            status: data.status || 'Downloading...',
          })
          if (data.percent >= 100) {
            safeTimeout(() => {
              setDownloadProgress(null)
              api.checkCoreBinaries().then(b => { if (b) setBinaries(b) }).catch(reportFailure('refresh binaries'))
            }, 1000)
          }
        }
      })
      return () => {
        if (typeof unsub === 'function') unsub()
      }
    }
  }, [])

  const [downloadError, setDownloadError] = useState<{ key: string; msg: string } | null>(null)

  const handleUpdateCore = async (key: string) => {
    setUpdatingCore(key)
    setDownloadError(null)
    setDownloadProgress({ key, percent: 1, speed: 0, status: t('settings_status_connecting') })
    try {
      await api.updateCoreEngine(key)
      const updated = await api.checkCoreBinaries()
      setBinaries(updated)
    } catch (err) {
      const msg = errorMessage(err, t('settings_unknown_error'))
      setDownloadError({ key, msg })
      safeTimeout(() => setDownloadError(null), 8000)
      console.error(`Update ${key} error:`, err)
    } finally {
      setUpdatingCore(null)
      setDownloadProgress(null)
    }
  }

  const handleUpdateGeoData = async () => {
    setUpdatingGeo(true)
    try {
      await api.updateGeoData()
      setGeoUpdated(true)
      safeTimeout(() => setGeoUpdated(false), 3000)
    } catch (err) {
      console.error('Update geo data error:', err)
    } finally {
      setUpdatingGeo(false)
    }
  }

  // Auto-failover is toggled via a dedicated backend method so the engine can
  // arm/disarm its health monitor immediately, then we mirror it into settings.
  const handleToggleFailover = async (enabled: boolean) => {
    if (!settings) return
    setSettings({ ...settings, auto_failover_enabled: enabled })
    try {
      await api.toggleAutoFailover(enabled)
    } catch (err) {
      console.error('Toggle auto-failover error:', err)
      // Roll the local state back so the switch never lies about reality.
      setSettings({ ...settings, auto_failover_enabled: !enabled })
    }
  }

  // Debounced auto-persist.
  //
  // Every keystroke in a text field used to call SaveSettings with the whole
  // Settings object, which the backend writes as a ~176 KB config.json — one
  // full-file write per character typed, each one racing the last, with no
  // ordering guarantee about which lands. Waiting for a short pause collapses a
  // burst of typing into a single write, and resetting the timer means the
  // latest value is always the one written.
  const saveTimer = useRef<number | null>(null)
  const pendingSave = useRef<Settings | null>(null)
  const savedCallbackRef = useRef(onSettingsSaved)

  useEffect(() => {
    savedCallbackRef.current = onSettingsSaved
  }, [onSettingsSaved])

  const flushPendingSave = () => {
    if (saveTimer.current !== null) {
      window.clearTimeout(saveTimer.current)
      saveTimer.current = null
    }
    const pending = pendingSave.current
    pendingSave.current = null
    if (!pending) return
    api.saveSettings(pending)
      .then(() => savedCallbackRef.current?.(pending))
      .catch(err => console.error('Auto save settings error:', err))
  }

  // Flush on unmount, so a change made just before navigating away still lands.
  useEffect(() => flushPendingSave, [])

  const update = <K extends keyof Settings>(key: K, value: Settings[K]) => {
    if (!settings) return
    const updated = { ...settings, [key]: value }
    setSettings(updated)

    if (key === 'language') {
      setLanguage(value as Language)
    }

    // Auto-persist to storage so settings never revert when navigating away
    pendingSave.current = updated
    if (saveTimer.current !== null) window.clearTimeout(saveTimer.current)
    saveTimer.current = window.setTimeout(flushPendingSave, 500)
  }

  const handleThemeSelect = (themeId: Theme) => {
    setSelectedTheme(themeId)
    applyTheme(themeId)
    update('theme', themeId)
  }

  const handleSave = async () => {
    if (!settings) return
    // Cancel any debounced write first: it holds an older snapshot of `settings`,
    // and letting it land after this one would silently revert the save.
    if (saveTimer.current !== null) {
      window.clearTimeout(saveTimer.current)
      saveTimer.current = null
    }
    pendingSave.current = null
    setIsSaving(true)
    try {
      await api.saveSettings(settings)
      if (onSettingsSaved) {
        onSettingsSaved(settings)
      }
      setSaved(true)
      safeTimeout(() => setSaved(false), 2000)
    } catch (err) {
      console.error('Save settings error:', err)
    } finally {
      setIsSaving(false)
    }
  }

  const handleClearStaleProxy = async () => {
    setClearingProxy(true)
    try {
      await api.clearStaleProxy()
      setProxyCleared(true)
      safeTimeout(() => setProxyCleared(false), 2500)
    } catch (err) {
      console.error('Clear proxy error:', err)
    } finally {
      setClearingProxy(false)
    }
  }

  const handleAutoInstallMissingCores = async () => {
    setIsInstallingCores(true)
    try {
      for (const [key, info] of Object.entries(binaries)) {
        if (info.status === 'missing') {
          await api.installCoreBinary(key)
        }
      }
      const updated = await api.checkCoreBinaries()
      setBinaries(updated)
    } catch (err) {
      console.error('Install cores error:', err)
    } finally {
      setIsInstallingCores(false)
    }
  }

  const handleRunDiagnostics = async () => {
    setIsRunningDiagnostics(true)
    setRepairResult(null)
    try {
      const rep = await api.runDiagnosticAudit()
      setDiagnosticReport(rep)
    } catch (err) {
      console.error('Diagnostics error:', err)
    } finally {
      setIsRunningDiagnostics(false)
    }
  }

  const handleRepairDiagnostics = async () => {
    setIsRepairing(true)
    try {
      const res = await api.repairDiagnosticIssues()
      setRepairResult(res)
      if (res?.post_repair_health) {
        setDiagnosticReport(res.post_repair_health)
      }
    } catch (err) {
      console.error('Repair diagnostics error:', err)
    } finally {
      setIsRepairing(false)
    }
  }

  const handleFlushDNS = async () => {
    setFlushingDNS(true)
    try {
      await api.flushWindowsDNS()
      setDnsFlushed(true)
      safeTimeout(() => setDnsFlushed(false), 2500)
    } catch (err) {
      console.error('Flush DNS error:', err)
    } finally {
      setFlushingDNS(false)
    }
  }

  const handleScanDNS = async () => {
    setIsScanningDNS(true)
    setDnsScanResult(null)
    try {
      const results = await api.scanAndSelectBestDNS(true) // prefer DoH
      if (results && results.length > 0) {
        setDnsScanResult(results)
        // Find best successful result
        const best = results.find((r) => r.is_success)
        if (best) {
          update('dns_server', best.raw_value)
        }
      }
    } catch (err) {
      console.error('DNS Scan error:', err)
    } finally {
      setIsScanningDNS(false)
    }
  }

  const handleExportBackup = async () => {
    setIsExportingBackup(true)
    setBackupError(null)
    setBackupSuccess(null)
    try {
      const data = await api.exportEncryptedBackup(backupPassword)
      const blob = new Blob([data], { type: 'application/json' })
      const url = URL.createObjectURL(blob)
      const a = document.createElement('a')
      a.href = url
      a.download = `NexVPN_Backup_${new Date().toISOString().slice(0, 10)}.json`
      document.body.appendChild(a)
      a.click()
      document.body.removeChild(a)
      URL.revokeObjectURL(url)
      setBackupSuccess(t('settings_backup_downloaded'))
      safeTimeout(() => setBackupSuccess(null), 3000)
    } catch (err) {
      setBackupError(errorMessage(err, t('settings_backup_create_failed')))
    } finally {
      setIsExportingBackup(false)
    }
  }

  const handleImportBackup = async () => {
    if (!importBackupString.trim()) return
    setIsImportingBackup(true)
    setBackupError(null)
    setBackupSuccess(null)
    try {
      const summary = await api.importEncryptedBackup(importBackupString, importPassword, true)
      setBackupSuccess(t('settings_restore_success').replace('{n}', String(summary.profiles_imported)))
      setImportBackupString('')
      const s = await api.getSettings()
      if (s) setSettings(s)
    } catch (err) {
      setBackupError(errorMessage(err, t('settings_restore_failed')))
    } finally {
      setIsImportingBackup(false)
    }
  }

  const [newCustomSNI, setNewCustomSNI] = useState('')

  const handleAddCustomSNI = async () => {
    const domain = newCustomSNI.trim().toLowerCase()
    if (!domain) return
    try {
      await api.addCustomSNI(domain)
      setNewCustomSNI('')
      const pool = await api.getSNIPoolList()
      if (pool) setSniPool(pool)
    } catch (err) {
      console.error('Failed to add custom SNI:', err)
    }
  }

  const handleRemoveCustomSNI = async (domain: string) => {
    try {
      await api.removeCustomSNI(domain)
      const pool = await api.getSNIPoolList()
      if (pool) setSniPool(pool)
    } catch (err) {
      console.error('Failed to remove custom SNI:', err)
    }
  }

  const handleAuditSNIPool = async () => {
    setIsAuditingSNI(true)
    try {
      const items = await api.auditSNIPool()
      if (items) setSniPool(items)
    } catch (err) {
      console.error('Audit SNI pool error:', err)
    } finally {
      setIsAuditingSNI(false)
    }
  }

  const handleRunAutoMTU = async () => {
    setOptimizingMTU(true)
    try {
      const mtu = await api.runAutoMTUOptimization()
      setOptimizedMTUResult(mtu)
      update('custom_mtu', mtu)
      safeTimeout(() => setOptimizedMTUResult(null), 4000)
    } catch (err) {
      console.error('MTU optimization error:', err)
    } finally {
      setOptimizingMTU(false)
    }
  }

  const handleCheckQuota = async () => {
    if (!quotaCheckUrl.trim()) return
    setCheckingQuota(true)
    try {
      const res = await api.checkSubscriptionQuotas(quotaCheckUrl)
      setQuotaResult(res)
    } catch (err) {
      console.error('Check quota error:', err)
    } finally {
      setCheckingQuota(false)
    }
  }

  const toggleApp = (exe: string) => {
    const list = settings.selected_apps || []
    if (list.includes(exe)) {
      update('selected_apps', list.filter(item => item !== exe))
    } else {
      update('selected_apps', [...list, exe])
    }
  }

  const addCustomApp = () => {
    const trimmed = newAppInput.trim().toLowerCase()
    if (!trimmed) return
    const exe = trimmed.endsWith('.exe') ? trimmed : trimmed + '.exe'
    const list = settings.selected_apps || []
    if (!list.includes(exe)) {
      update('selected_apps', [...list, exe])
    }
    setNewAppInput('')
  }

  const [newProxyDomain, setNewProxyDomain] = useState('')
  const [newDirectDomain, setNewDirectDomain] = useState('')
  const [newBlockDomain, setNewBlockDomain] = useState('')

  const addDomainRule = (type: 'proxy' | 'direct' | 'block') => {
    if (type === 'proxy') {
      const val = newProxyDomain.trim().toLowerCase()
      if (!val) return
      const list = settings.custom_proxy_domains || []
      if (!list.includes(val)) update('custom_proxy_domains', [...list, val])
      setNewProxyDomain('')
    } else if (type === 'direct') {
      const val = newDirectDomain.trim().toLowerCase()
      if (!val) return
      const list = settings.custom_direct_domains || []
      if (!list.includes(val)) update('custom_direct_domains', [...list, val])
      setNewDirectDomain('')
    } else if (type === 'block') {
      const val = newBlockDomain.trim().toLowerCase()
      if (!val) return
      const list = settings.custom_block_domains || []
      if (!list.includes(val)) update('custom_block_domains', [...list, val])
      setNewBlockDomain('')
    }
  }

  const removeDomainRule = (type: 'proxy' | 'direct' | 'block', domain: string) => {
    if (type === 'proxy') {
      const list = settings.custom_proxy_domains || []
      update('custom_proxy_domains', list.filter(d => d !== domain))
    } else if (type === 'direct') {
      const list = settings.custom_direct_domains || []
      update('custom_direct_domains', list.filter(d => d !== domain))
    } else if (type === 'block') {
      const list = settings.custom_block_domains || []
      update('custom_block_domains', list.filter(d => d !== domain))
    }
  }

  const currentLang = getLanguage()

  return (
    <div className="page space-y-4">
      {/* Header & Save Action */}
      <div className="page-header flex items-center justify-between flex-wrap gap-3">
        <div>
          <h1 className="page-title">{t('settings_title')}</h1>
          <p className="page-subtitle">{t('settings_subtitle')}</p>
        </div>
        <div className="flex items-center gap-2">
          <div className="relative">
            <Search
              size={14}
              className="pointer-events-none absolute start-3 top-1/2 -translate-y-1/2 text-muted-foreground"
            />
            <input
              type="search"
              id="settings-search"
              value={query}
              onChange={e => setQuery(e.target.value)}
              placeholder={t('settings_search_placeholder')}
              aria-label={t('settings_search_placeholder')}
              className="form-control ps-9 w-56 text-xs"
              style={{ fontSize: '12px' }}
            />
            {query && (
              <button
                type="button"
                onClick={() => setQuery('')}
                aria-label={t('clear')}
                className="absolute end-2 top-1/2 -translate-y-1/2 p-1 rounded-lg text-muted-foreground hover:text-foreground"
              >
                <X size={13} />
              </button>
            )}
          </div>
          <Button
            id="save-settings-btn"
            variant={saved ? 'secondary' : 'default'}
            onClick={handleSave}
            disabled={isSaving}
          >
            {isSaving ? (
              <>
                <div className="spinner" style={{ width: 14, height: 14 }} /> ...
              </>
            ) : saved ? (
              t('saved_successfully')
            ) : (
              t('save_settings')
            )}
          </Button>
        </div>
      </div>

      {/* Modern Categorized Tab Navigation */}
      <div className="flex items-center gap-2 overflow-x-auto pb-1 text-xs no-scrollbar">
        <button
          type="button"
          onClick={() => setActiveTab('network')}
          className={`flex items-center gap-2 px-3.5 py-2 rounded-xl font-bold whitespace-nowrap transition-all ${activeTab === 'network'
            ? 'bg-primary text-primary-foreground shadow-sm'
            : "bg-card/80 text-muted-foreground hover:text-foreground border border-border"
            }`}
        >
          <Network size={14} />
          <span>{t('sec_routing_cores')}</span>
        </button>

        <button
          type="button"
          onClick={() => setActiveTab('antidpi')}
          className={`flex items-center gap-2 px-3.5 py-2 rounded-xl font-bold whitespace-nowrap transition-all ${activeTab === 'antidpi'
            ? 'bg-primary text-primary-foreground shadow-sm'
            : "bg-card/80 text-muted-foreground hover:text-foreground border border-border"
            }`}
        >
          <Zap size={14} />
          <span>{t('sec_anti_dpi')}</span>
        </button>

        <button
          type="button"
          onClick={() => setActiveTab('apps')}
          className={`flex items-center gap-2 px-3.5 py-2 rounded-xl font-bold whitespace-nowrap transition-all ${activeTab === 'apps'
            ? 'bg-primary text-primary-foreground shadow-sm'
            : "bg-card/80 text-muted-foreground hover:text-foreground border border-border"
            }`}
        >
          <Split size={14} />
          <span>{t('sec_split_tunneling')}</span>
        </button>

        <button
          type="button"
          onClick={() => setActiveTab('appearance')}
          className={`flex items-center gap-2 px-3.5 py-2 rounded-xl font-bold whitespace-nowrap transition-all ${activeTab === 'appearance'
            ? 'bg-primary text-primary-foreground shadow-sm'
            : "bg-card/80 text-muted-foreground hover:text-foreground border border-border"
            }`}
        >
          <Palette size={14} />
          <span>{t('sec_appearance')}</span>
        </button>

        <button
          type="button"
          onClick={() => setActiveTab('system')}
          className={`flex items-center gap-2 px-3.5 py-2 rounded-xl font-bold whitespace-nowrap transition-all ${activeTab === 'system'
            ? 'bg-primary text-primary-foreground shadow-sm'
            : "bg-card/80 text-muted-foreground hover:text-foreground border border-border"
            }`}
        >
          <Cpu size={14} />
          <span>{t('sec_system')}</span>
        </button>

        <button
          type="button"
          onClick={() => setActiveTab('automation')}
          className={`flex items-center gap-2 px-3.5 py-2 rounded-xl font-bold whitespace-nowrap transition-all ${activeTab === 'automation'
            ? 'bg-primary text-primary-foreground shadow-sm'
            : "bg-card/80 text-muted-foreground hover:text-foreground border border-border"
            }`}
        >
          <SlidersHorizontal size={14} />
          <span>{t('adv_automation')}</span>
        </button>

        <button
          type="button"
          onClick={() => setActiveTab('diagnostics')}
          className={`flex items-center gap-2 px-3.5 py-2 rounded-xl font-bold whitespace-nowrap transition-all ${activeTab === 'diagnostics'
            ? 'bg-primary text-primary-foreground shadow-sm'
            : "bg-card/80 text-muted-foreground hover:text-foreground border border-border"
            }`}
        >
          <Wrench size={14} />
          <span>{t('adv_diag')}</span>
        </button>
      </div>

      {/* Search feedback — the active tab's rows filter themselves; this tells
          the user when the whole current view came up empty. */}
      {query.trim() && matchCount === 0 && (
        <Card className="p-6 text-center text-sm text-muted-foreground">
          <Search size={18} className="mx-auto mb-2 opacity-60" />
          {t('settings_no_results')}
        </Card>
      )}

      {/* TAB 1: Network & Cores */}
      <SettingMatchContext.Provider value={reportSettingMatch}>      {activeTab === 'network' && (
        <div className="space-y-4">
          <div className="section">
            <div className="section-title">
              <Wifi size={13} style={{ display: 'inline', marginInlineEnd: 6 }} />
              {t('sec_routing_cores')}
            </div>
            <Card >
              <div style={{ padding: 0 }}>
                {/* Proxy Mode */}
                <SettingRow
                  id="proxy-mode-row"
                  icon={<Layers size={16} />}
                  label={t('vpn_proxy_mode_label')}
                  description={t('vpn_proxy_mode_desc')}
                  query={query}
                  control={
                    <select
                      id="proxy-mode-select"
                      className="form-control"
                      style={{ width: '250px', fontSize: '12px' }}
                      value={settings.proxy_mode || 'tun'}
                      onChange={e => update('proxy_mode', e.target.value)}
                    >
                      <option value="tun">🚀 {t('mode_switch_tun')}</option>
                      <option value="system_proxy">🌐 {t('mode_switch_proxy')}</option>
                    </select>
                  }
                />

                {/* System Proxy inside TUN mode */}
                <SettingRow
                  id="tun-system-proxy-row"
                  icon={<Network size={16} />}
                  label={t('vpn_tun_proxy_label')}
                  description={t('vpn_tun_proxy_desc')}
                  query={query}
                  control={
                    <select
                      id="tun-system-proxy-select"
                      className="form-control"
                      style={{ width: '250px', fontSize: '12px' }}
                      value={settings.tun_system_proxy || 'dual'}
                      onChange={e => update('tun_system_proxy', e.target.value)}
                    >
                      <option value="dual">🔀 {t('vpn_tun_proxy_dual')}</option>
                      <option value="tun_only">🛡️ {t('vpn_tun_proxy_only')}</option>
                    </select>
                  }
                />

                {/* V2Ray Core Selector */}
                <SettingRow
                  id="v2ray-core-select-row"
                  icon={<Cpu size={16} />}
                  label={t('v2ray_core_label')}
                  description={t('v2ray_core_desc')}
                  query={query}
                  control={
                    <select
                      id="v2ray-core-select"
                      className="form-control"
                      style={{ width: '250px', fontSize: '12px' }}
                      value={settings.v2ray_core || 'auto'}
                      onChange={e => update('v2ray_core', e.target.value)}
                    >
                      <option value="auto">⚡ {t('core_auto')}</option>
                      <option value="xray">🔮 {t('core_xray')}</option>
                      <option value="singbox">📦 {t('core_singbox')}</option>
                    </select>
                  }
                />

                {/* Smart Routing Mode */}
                <SettingRow
                  id="routing-mode-row"
                  icon={<Sparkles size={16} />}
                  label={t('smart_routing_label')}
                  description={t('smart_routing_desc')}
                  query={query}
                  control={
                    <select
                      id="routing-mode-select"
                      className="form-control"
                      style={{ width: '250px', fontSize: '12px' }}
                      value={settings.routing_mode || 'bypass_iran'}
                      onChange={e => update('routing_mode', e.target.value)}
                    >
                      <option value="bypass_iran">{t('mode_bypass_iran')}</option>
                      <option value="direct_iran_block_ads">{t('mode_direct_iran_block_ads')}</option>
                      <option value="global">{t('mode_global')}</option>
                    </select>
                  }
                />

                {/* Ultra-Low Latency Game Mode */}
                <SettingRow
                  id="game-mode-row"
                  icon={<Gamepad2 size={16} />}
                  label={t('game_mode_label')}
                  description={t('game_mode_desc')}
                  query={query}
                  control={
                    <label className="toggle">
                      <input
                        type="checkbox"
                        checked={!!settings.game_mode_enabled}
                        onChange={e => update('game_mode_enabled', e.target.checked)}
                      />
                      <span className="toggle-slider" />
                    </label>
                  }
                />

                {/* Allow LAN Sharing */}
                <SettingRow
                  id="allow-lan-row"
                  icon={<Wifi size={16} />}
                  label={t('allow_lan_label')}
                  description={t('allow_lan_desc')}
                  query={query}
                  control={
                    <label className="toggle">
                      <input
                        type="checkbox"
                        checked={!!settings.allow_lan}
                        onChange={e => update('allow_lan', e.target.checked)}
                      />
                      <span className="toggle-slider" />
                    </label>
                  }
                />

                {/* Bypass LAN / Local IP Ranges */}
                <SettingRow
                  id="bypass-lan-row"
                  icon={<Layers size={16} />}
                  label={t('bypass_lan_label')}
                  description={t('bypass_lan_desc')}
                  query={query}
                  control={
                    <label className="toggle">
                      <input
                        type="checkbox"
                        checked={settings.bypass_lan !== false}
                        onChange={e => update('bypass_lan', e.target.checked)}
                      />
                      <span className="toggle-slider" />
                    </label>
                  }
                />

                {/* Custom Direct Domains */}
                <SettingRow
                  id="custom-direct-domains-row"
                  icon={<Globe size={16} />}
                  label={t('custom_direct_domains_label')}
                  description={t('custom_direct_domains_desc')}
                  query={query}
                  control={
                    <input
                      type="text"
                      className="form-control font-mono text-xs"
                      style={{ width: '280px', fontSize: '11px' }}
                      placeholder="e.g. example.com, mybank.ir"
                      value={(settings.custom_direct_domains || []).join(', ')}
                      onChange={e => {
                        const arr = e.target.value.split(',').map(s => s.trim()).filter(Boolean)
                        update('custom_direct_domains', arr)
                      }}
                    />
                  }
                />

                {/* Custom Proxy Domains */}
                <SettingRow
                  id="custom-proxy-domains-row"
                  icon={<Shield size={16} />}
                  label={t('custom_proxy_domains_label')}
                  description={t('custom_proxy_domains_desc')}
                  query={query}
                  control={
                    <input
                      type="text"
                      className="form-control font-mono text-xs"
                      style={{ width: '280px', fontSize: '11px' }}
                      placeholder="e.g. youtube.com, openai.com"
                      value={(settings.custom_proxy_domains || []).join(', ')}
                      onChange={e => {
                        const arr = e.target.value.split(',').map(s => s.trim()).filter(Boolean)
                        update('custom_proxy_domains', arr)
                      }}
                    />
                  }
                />

                {/* Primary DNS Engine */}
                <SettingRow
                  id="dns-server-row"
                  icon={<Globe size={16} />}
                  label={t('dns_engine_label')}
                  description={t('dns_engine_desc')}
                  query={query}
                  control={
                    <div className="flex flex-col gap-2 items-end">
                      <select
                        className="form-control"
                        style={{ width: '250px', fontSize: '12px' }}
                        value={
                          ['https://1.1.1.1/dns-query', 'https://dns.google/dns-query', 'https://dns.quad9.net/dns-query', '1.1.1.1', '8.8.8.8', '9.9.9.9'].includes(settings.dns_server)
                            ? settings.dns_server
                            : 'custom'
                        }
                        onChange={e => {
                          if (e.target.value !== 'custom') {
                            update('dns_server', e.target.value)
                          }
                        }}
                      >
                        <option value="https://1.1.1.1/dns-query">Cloudflare DoH (1.1.1.1)</option>
                        <option value="https://dns.google/dns-query">Google DoH (8.8.8.8)</option>
                        <option value="https://dns.quad9.net/dns-query">Quad9 Secure DoH</option>
                        <option value="1.1.1.1">Cloudflare UDP (1.1.1.1)</option>
                        <option value="8.8.8.8">Google UDP (8.8.8.8)</option>
                        <option value="9.9.9.9">Quad9 UDP (9.9.9.9)</option>
                        <option value="custom">⚙️ Custom DNS / DoH URL...</option>
                      </select>
                      <input
                        type="text"
                        placeholder={t('custom_dns_placeholder')}
                        className="form-control font-mono"
                        style={{ width: '250px', fontSize: '11px' }}
                        value={settings.dns_server || ''}
                        onChange={e => update('dns_server', e.target.value)}
                      />

                      <Button variant="secondary" size="sm" className="mt-1 w-[250px] flex justify-center items-center gap-2"
                        type="button"
                        onClick={handleScanDNS}
                        disabled={isScanningDNS}
                      >
                        {isScanningDNS ? (
                          <>
                            <div className="spinner" style={{ width: 12, height: 12 }} />
                            <span>{t('settings_scanning')}</span>
                          </>
                        ) : (
                          <>
                            <Activity size={14} />
                            <span>{t('settings_find_fastest_dns')}</span>
                          </>
                        )}
                      </Button>

                      {dnsScanResult && dnsScanResult.length > 0 && (
                        <div className="mt-2 text-xs bg-card/50 p-2 rounded-lg border border-border w-[250px]">
                          <div className="text-success font-bold mb-1">
                            {t('settings_best_dns_found')}
                          </div>
                          {dnsScanResult.filter((r) => r.is_success).slice(0, 3).map((r, idx: number) => (
                            <div key={idx} className="flex justify-between items-center py-1 border-b border-border last:border-0">
                              <span className="text-foreground font-mono text-[10px]" title={r.raw_value}>{r.server_name}</span>
                              <span className={`font-mono text-[10px] ${idx === 0 ? 'text-success font-bold' : 'text-muted-foreground'}`}>
                                {r.latency}ms
                              </span>
                            </div>
                          ))}
                        </div>
                      )}
                    </div>
                  }
                />

                {/* Fallback DNS */}
                <SettingRow
                  id="dns-fallback-row"
                  icon={<Radio size={16} />}
                  label={t('dns_fallback_label')}
                  description={t('dns_fallback_desc')}
                  query={query}
                  control={
                    <input
                      type="text"
                      className="form-control font-mono"
                      style={{ width: '250px', fontSize: '12px' }}
                      value={settings.dns_fallback || '8.8.8.8'}
                      onChange={e => update('dns_fallback', e.target.value)}
                      placeholder="8.8.8.8"
                    />
                  }
                />

                {/* IPv4 / IPv6 Strategy */}
                <SettingRow
                  id="ip-strategy-row"
                  icon={<Globe size={16} />}
                  label={t('ip_strategy_label')}
                  description={t('ip_strategy_desc')}
                  query={query}
                  control={
                    <select
                      className="form-control"
                      style={{ width: '250px', fontSize: '12px' }}
                      value={settings.ip_strategy || 'prefer_ipv4'}
                      onChange={e => update('ip_strategy', e.target.value)}
                    >
                      <option value="prefer_ipv4">{t('prefer_ipv4')}</option>
                      <option value="prefer_ipv6">{t('prefer_ipv6')}</option>
                      <option value="ipv4_only">{t('ipv4_only')}</option>
                      <option value="ipv6_only">{t('ipv6_only')}</option>
                    </select>
                  }
                />

                {/* TUN MTU Size & Adaptive MTU Prober */}
                <SettingRow
                  id="tun-mtu-row"
                  icon={<Layers size={16} />}
                  label={t('settings_mtu_label')}
                  description={t('settings_mtu_desc')}
                  query={query}
                  control={
                    <div className="flex flex-col gap-2 items-end">
                      <div className="flex items-center gap-2">
                        <select
                          className="form-control font-mono"
                          style={{ width: '160px', fontSize: '12px' }}
                          value={settings.custom_mtu ? `${settings.custom_mtu}` : '1380'}
                          onChange={e => update('custom_mtu', parseInt(e.target.value))}
                        >
                          <option value="1280">{t('settings_mtu_1280')}</option>
                          <option value="1380">{t('settings_mtu_1380')}</option>
                          <option value="1420">1420 B (WireGuard/ADSL)</option>
                          <option value="1500">1500 B (Ethernet/FTTH)</option>
                        </select>
                        <Button variant="secondary" size="sm" className="text-[11px]"
                          onClick={handleRunAutoMTU}
                          disabled={optimizingMTU}
                        >
                          {optimizingMTU ? t('settings_testing') : optimizedMTUResult ? `✓ ${optimizedMTUResult} B` : t('settings_auto_detect')}
                        </Button>
                      </div>
                      <label className="flex items-center gap-2 text-[11px] text-muted-foreground cursor-pointer">
                        <input
                          type="checkbox"
                          checked={settings.auto_mtu_enabled !== false}
                          onChange={e => update('auto_mtu_enabled', e.target.checked)}
                        />
                        <span>{t('settings_auto_mtu_each_connect')}</span>
                      </label>
                    </div>
                  }
                />

                {/* Real Delay Test URL */}
                <SettingRow
                  id="real-delay-url-row"
                  icon={<Radio size={16} />}
                  label={t('real_delay_url_label')}
                  description={t('real_delay_url_desc')}
                  query={query}
                  control={
                    <input
                      type="text"
                      className="form-control font-mono"
                      style={{ width: '250px', fontSize: '12px' }}
                      value={settings.real_delay_url || 'http://cp.cloudflare.com/generate_204'}
                      onChange={e => update('real_delay_url', e.target.value)}
                    />
                  }
                />
              </div>
            </Card>
          </div>

          {/* Visual Custom Domain Rules Manager */}
          <div className="section">
            <div className="section-title">
              <Shield size={13} style={{ display: 'inline', marginInlineEnd: 6, color: '#06b6d4' }} />
              {t('custom_domain_rules_title')}
            </div>
            <Card className="p-4 space-y-4">
              {/* Always Proxy Domains */}
              <div className="space-y-2">
                <div className="flex items-center justify-between text-xs font-bold text-foreground">
                  <span className="flex items-center gap-1.5 text-primary">
                    <span>🌐</span> {t('always_proxy_domains_label')}
                  </span>
                  <span className="text-[11px] font-normal text-muted-foreground">{t('always_proxy_domains_desc')}</span>
                </div>
                <div className="flex items-center gap-2">
                  <input
                    type="text"
                    placeholder={t('domain_placeholder_proxy')}
                    className="form-control text-xs flex-1 font-mono"
                    value={newProxyDomain}
                    onChange={e => setNewProxyDomain(e.target.value)}
                    onKeyDown={e => e.key === 'Enter' && addDomainRule('proxy')}
                  />
                  <Button variant="secondary" size="sm" className="text-xs px-3" type="button" onClick={() => addDomainRule('proxy')}>
                    {t('add')}
                  </Button>
                </div>
                <div className="flex items-center gap-1.5 flex-wrap">
                  {(settings.custom_proxy_domains || []).map(d => (
                    <span key={d} className="inline-flex items-center gap-1 text-xs px-2.5 py-1 rounded-lg border-border bg-muted text-muted-foreground font-mono">
                      {d}
                      <button type="button" onClick={() => removeDomainRule('proxy', d)} className="text-muted-foreground hover:text-destructive ms-1">
                        <X size={11} />
                      </button>
                    </span>
                  ))}
                  {(!settings.custom_proxy_domains || settings.custom_proxy_domains.length === 0) && (
                    <span className="text-[11px] text-muted-foreground italic">{t('no_custom_domains')}</span>
                  )}
                </div>
              </div>

              {/* Always Direct Domains */}
              <div className="space-y-2 pt-3 border-t border-border">
                <div className="flex items-center justify-between text-xs font-bold text-foreground">
                  <span className="flex items-center gap-1.5 text-success">
                    <span>⚡</span> {t('always_direct_domains_label')}
                  </span>
                  <span className="text-[11px] font-normal text-muted-foreground">{t('always_direct_domains_desc')}</span>
                </div>
                <div className="flex items-center gap-2">
                  <input
                    type="text"
                    placeholder={t('domain_placeholder_direct')}
                    className="form-control text-xs flex-1 font-mono"
                    value={newDirectDomain}
                    onChange={e => setNewDirectDomain(e.target.value)}
                    onKeyDown={e => e.key === 'Enter' && addDomainRule('direct')}
                  />
                  <Button variant="secondary" size="sm" className="text-xs px-3" type="button" onClick={() => addDomainRule('direct')}>
                    {t('add')}
                  </Button>
                </div>
                <div className="flex items-center gap-1.5 flex-wrap">
                  {(settings.custom_direct_domains || []).map(d => (
                    <span key={d} className="inline-flex items-center gap-1 text-xs px-2.5 py-1 rounded-lg bg-success/10 text-success border border-success/20 font-mono">
                      {d}
                      <button type="button" onClick={() => removeDomainRule('direct', d)} className="text-muted-foreground hover:text-destructive ms-1">
                        <X size={11} />
                      </button>
                    </span>
                  ))}
                  {(!settings.custom_direct_domains || settings.custom_direct_domains.length === 0) && (
                    <span className="text-[11px] text-muted-foreground italic">{t('no_custom_domains')}</span>
                  )}
                </div>
              </div>

              {/* Blocked Domains */}
              <div className="space-y-2 pt-3 border-t border-border">
                <div className="flex items-center justify-between text-xs font-bold text-foreground">
                  <span className="flex items-center gap-1.5 text-destructive">
                    <span>🛑</span> {t('blocked_domains_label')}
                  </span>
                  <span className="text-[11px] font-normal text-muted-foreground">{t('blocked_domains_desc')}</span>
                </div>
                <div className="flex items-center gap-2">
                  <input
                    type="text"
                    placeholder={t('domain_placeholder_block')}
                    className="form-control text-xs flex-1 font-mono"
                    value={newBlockDomain}
                    onChange={e => setNewBlockDomain(e.target.value)}
                    onKeyDown={e => e.key === 'Enter' && addDomainRule('block')}
                  />
                  <Button variant="secondary" size="sm" className="text-xs px-3" type="button" onClick={() => addDomainRule('block')}>
                    {t('add')}
                  </Button>
                </div>
                <div className="flex items-center gap-1.5 flex-wrap">
                  {(settings.custom_block_domains || []).map(d => (
                    <span key={d} className="inline-flex items-center gap-1 text-xs px-2.5 py-1 rounded-lg bg-destructive/10 text-destructive border border-destructive/20 font-mono">
                      {d}
                      <button type="button" onClick={() => removeDomainRule('block', d)} className="text-muted-foreground hover:text-destructive ms-1">
                        <X size={11} />
                      </button>
                    </span>
                  ))}
                  {(!settings.custom_block_domains || settings.custom_block_domains.length === 0) && (
                    <span className="text-[11px] text-muted-foreground italic">{t('no_custom_domains')}</span>
                  )}
                </div>
              </div>
            </Card>
          </div>
        </div>
      )}

        {/* TAB 2: Anti-DPI & Censorship Resistance */}
        {activeTab === 'antidpi' && (
          <div className="space-y-4">
            <div className="section">
              <div className="section-title">
                <Zap size={13} style={{ display: 'inline', marginInlineEnd: 6, color: '#f59e0b' }} />
                {t('sec_anti_dpi')}
              </div>
              <Card >
                <div style={{ padding: 0 }}>
                  {/* TLS Fragment */}
                  <SettingRow
                    id="fragment-toggle"
                    icon={<Zap size={16} />}
                    label={t('anti_dpi_fragment_label')}
                    description={t('anti_dpi_fragment_desc')}
                    query={query}
                    control={
                      <label className="toggle">
                        <input
                          type="checkbox"
                          checked={!!settings.fragment_enabled}
                          onChange={e => update('fragment_enabled', e.target.checked)}
                        />
                        <span className="toggle-slider" />
                      </label>
                    }
                  />

                  {settings.fragment_enabled && (
                    <>
                      <SettingRow
                        id="fragment-size"
                        icon={<Sparkles size={16} />}
                        label={t('fragment_size_label')}
                        description={t('fragment_size_desc')}
                        query={query}
                        control={
                          <input
                            type="text"
                            className="form-control font-mono"
                            style={{ width: '180px', fontSize: '12px' }}
                            value={settings.fragment_size || '100-200'}
                            onChange={e => update('fragment_size', e.target.value)}
                          />
                        }
                      />
                      <SettingRow
                        id="fragment-sleep"
                        icon={<Clock size={16} />}
                        label={t('fragment_sleep_label')}
                        description={t('fragment_sleep_desc')}
                        query={query}
                        control={
                          <input
                            type="text"
                            className="form-control font-mono"
                            style={{ width: '180px', fontSize: '12px' }}
                            value={settings.fragment_sleep || '10-20'}
                            onChange={e => update('fragment_sleep', e.target.value)}
                          />
                        }
                      />
                    </>
                  )}

                  {/* Multi Hop WARP */}
                  <SettingRow
                    id="multi-hop-warp"
                    icon={<Link size={16} />}
                    label={t('multi_hop_warp_label')}
                    description={t('multi_hop_warp_desc')}
                    query={query}
                    control={
                      <label className="toggle">
                        <input
                          type="checkbox"
                          checked={!!settings.multi_hop_warp}
                          onChange={e => update('multi_hop_warp', e.target.checked)}
                        />
                        <span className="toggle-slider" />
                      </label>
                    }
                  />

                  {/* Auto reconnect on unexpected drop */}
                  <SettingRow
                    id="auto-reconnect-row"
                    icon={<RefreshCw size={16} />}
                    label={t('auto_reconnect_label')}
                    description={t('auto_reconnect_desc')}
                    query={query}
                    control={
                      <label className="toggle">
                        <input
                          type="checkbox"
                          checked={!!settings.auto_reconnect}
                          onChange={e => update('auto_reconnect', e.target.checked)}
                        />
                        <span className="toggle-slider" />
                      </label>
                    }
                  />

                  {/* Domestic traffic routing */}
                  <SettingRow
                    id="domestic-routing-row"
                    icon={<Globe size={16} />}
                    label={t('domestic_routing_label')}
                    description={t('domestic_routing_desc')}
                    query={query}
                    control={
                      <select
                        className="form-control text-xs"
                        value={settings.domestic_routing_mode || 'direct'}
                        onChange={e => update('domestic_routing_mode', e.target.value)}
                      >
                        <option value="direct">{t('domestic_routing_direct')}</option>
                        <option value="proxy">{t('domestic_routing_proxy')}</option>
                      </select>
                    }
                  />
                </div>
              </Card>
            </div>

            {/* AmneziaWG Settings */}
            <div className="section">
              <div className="section-title">
                <Shield size={13} style={{ display: 'inline', marginInlineEnd: 6, color: 'hsl(var(--success))' }} />
                {t('amnezia_section_title')}
              </div>
              <Card >
                <div style={{ padding: 0 }}>
                  <SettingRow
                    id="amnezia-jc"
                    icon={<Sparkles size={16} />}
                    label={t('amnezia_jc_label')}
                    description={t('amnezia_jc_desc')}
                    query={query}
                    control={
                      <input
                        type="number"
                        className="form-control font-mono"
                        style={{ width: '120px', fontSize: '12px' }}
                        value={settings.amnezia_jc ?? 4}
                        onChange={e => update('amnezia_jc', parseInt(e.target.value) || 4)}
                      />
                    }
                  />

                  <SettingRow
                    id="amnezia-jsize"
                    icon={<Layers size={16} />}
                    label={`${t('amnezia_jmin_label')} / ${t('amnezia_jmax_label')}`}
                    description={`${t('amnezia_jmin_desc')} & ${t('amnezia_jmax_desc')}`}
                    query={query}
                    control={
                      <div className="flex items-center gap-2">
                        <input
                          type="number"
                          className="form-control font-mono"
                          style={{ width: '80px', fontSize: '12px' }}
                          value={settings.amnezia_jmin ?? 40}
                          onChange={e => update('amnezia_jmin', parseInt(e.target.value) || 40)}
                          placeholder="Min"
                        />
                        <span className="text-muted-foreground text-xs">-</span>
                        <input
                          type="number"
                          className="form-control font-mono"
                          style={{ width: '80px', fontSize: '12px' }}
                          value={settings.amnezia_jmax ?? 70}
                          onChange={e => update('amnezia_jmax', parseInt(e.target.value) || 70)}
                          placeholder="Max"
                        />
                      </div>
                    }
                  />

                  <SettingRow
                    id="amnezia-s"
                    icon={<Radio size={16} />}
                    label={t('amnezia_s1_s2_label')}
                    description={t('amnezia_s1_s2_desc')}
                    query={query}
                    control={
                      <div className="flex items-center gap-2">
                        <input
                          type="number"
                          className="form-control font-mono"
                          style={{ width: '80px', fontSize: '12px' }}
                          value={settings.amnezia_s1 ?? 15}
                          onChange={e => update('amnezia_s1', parseInt(e.target.value) || 15)}
                          placeholder="S1"
                        />
                        <span className="text-muted-foreground text-xs">/</span>
                        <input
                          type="number"
                          className="form-control font-mono"
                          style={{ width: '80px', fontSize: '12px' }}
                          value={settings.amnezia_s2 ?? 20}
                          onChange={e => update('amnezia_s2', parseInt(e.target.value) || 20)}
                          placeholder="S2"
                        />
                      </div>
                    }
                  />

                  {/* AmneziaWG Magic Headers H1-H4 */}
                  <SettingRow
                    id="amnezia-h"
                    icon={<Sparkles size={16} />}
                    label={t('amnezia_h_label')}
                    description={t('amnezia_h_desc')}
                    query={query}
                    control={
                      <div className="flex gap-2">
                        <input
                          type="number"
                          className="form-control w-16 text-center text-xs"
                          value={settings.amnezia_h1 ?? 1}
                          onChange={e => update('amnezia_h1', parseInt(e.target.value) || 1)}
                          placeholder="H1"
                        />
                        <input
                          type="number"
                          className="form-control w-16 text-center text-xs"
                          value={settings.amnezia_h2 ?? 2}
                          onChange={e => update('amnezia_h2', parseInt(e.target.value) || 2)}
                          placeholder="H2"
                        />
                        <input
                          type="number"
                          className="form-control w-16 text-center text-xs"
                          value={settings.amnezia_h3 ?? 3}
                          onChange={e => update('amnezia_h3', parseInt(e.target.value) || 3)}
                          placeholder="H3"
                        />
                        <input
                          type="number"
                          className="form-control w-16 text-center text-xs"
                          value={settings.amnezia_h4 ?? 4}
                          onChange={e => update('amnezia_h4', parseInt(e.target.value) || 4)}
                          placeholder="H4"
                        />
                      </div>
                    }
                  />

                  {/* Stealth Traffic Camouflage */}
                  <SettingRow
                    id="stealth-mode-row"
                    icon={<Sparkles size={16} />}
                    label={t('stealth_traffic_label')}
                    description={t('stealth_traffic_desc')}
                    query={query}
                    control={
                      <label className="toggle">
                        <input
                          type="checkbox"
                          checked={!!settings.stealth_mode_enabled}
                          onChange={e => update('stealth_mode_enabled', e.target.checked)}
                        />
                        <span className="toggle-slider" />
                      </label>
                    }
                  />

                  {/* Privacy Shield — redacts tokens/keys/UUIDs from logs */}
                  <SettingRow
                    id="privacy-shield-row"
                    icon={<Shield size={16} />}
                    label={t('privacy_shield_label')}
                    description={t('privacy_shield_desc')}
                    query={query}
                    control={
                      <label className="toggle">
                        <input
                          type="checkbox"
                          checked={settings.privacy_shield_enabled !== false}
                          onChange={e => update('privacy_shield_enabled', e.target.checked)}
                        />
                        <span className="toggle-slider" />
                      </label>
                    }
                  />

                  {/* Native notifications */}
                  <SettingRow
                    id="notifications-row"
                    icon={<Bell size={16} />}
                    label={t('notifications_label')}
                    description={t('notifications_desc')}
                    query={query}
                    control={
                      <label className="toggle">
                        <input
                          type="checkbox"
                          checked={settings.notifications_enabled !== false}
                          onChange={e => update('notifications_enabled', e.target.checked)}
                        />
                        <span className="toggle-slider" />
                      </label>
                    }
                  />

                  {/* Log verbosity */}
                  <SettingRow
                    id="log-level-row"
                    icon={<Terminal size={16} />}
                    label={t('log_level_label')}
                    description={t('log_level_desc')}
                    query={query}
                    control={
                      <select
                        className="form-control text-xs"
                        value={settings.log_level || 'info'}
                        onChange={e => update('log_level', e.target.value as Settings['log_level'])}
                      >
                        <option value="debug">{t('log_level_debug')}</option>
                        <option value="info">{t('log_level_info')}</option>
                        <option value="warning">{t('log_level_warning')}</option>
                        <option value="error">{t('log_level_error')}</option>
                      </select>
                    }
                  />

                  {/* Retained log entries */}
                  <SettingRow
                    id="max-log-entries-row"
                    icon={<Terminal size={16} />}
                    label={t('max_log_entries_label')}
                    description={t('max_log_entries_desc')}
                    query={query}
                    control={
                      <input
                        type="number"
                        min={100}
                        max={100000}
                        step={100}
                        className="form-control text-xs font-mono w-32"
                        value={settings.max_log_entries ?? 1000}
                        onChange={e => update('max_log_entries', parseInt(e.target.value) || 1000)}
                      />
                    }
                  />
                </div>
              </Card>
            </div>

            {/* Smart Multi-Egress Category Routing Section */}
            <div className="section">
              <div className="section-title flex items-center justify-between">
                <div className="flex items-center gap-1.5">
                  <Sparkles size={14} className="text-warning" />
                  <span>{t('settings_multi_egress')}</span>
                </div>
              </div>
              <Card className="p-0">
                <div className="p-0">
                  <SettingRow
                    id="smart-multi-egress-toggle"
                    icon={<Sparkles size={16} />}
                    label={t('settings_smart_split_label')}
                    description={t('settings_smart_split_desc')}
                    query={query}
                    control={
                      <label className="toggle">
                        <input
                          type="checkbox"
                          checked={settings.smart_routing_enabled !== false}
                          onChange={e => update('smart_routing_enabled', e.target.checked)}
                        />
                        <span className="toggle-slider" />
                      </label>
                    }
                  />

                  {settings.smart_routing_enabled !== false && (
                    <>
                      <SettingRow
                        id="ai-routing-mode-row"
                        icon={<Cpu size={16} />}
                        label={t('settings_ai_services_label')}
                        description={t('settings_ai_services_desc')}
                        query={query}
                        control={
                          <select
                            className="form-control"
                            style={{ width: '220px', fontSize: '12px' }}
                            value={settings.ai_routing_mode || 'proxy'}
                            onChange={e => update('ai_routing_mode', e.target.value)}
                          >
                            <option value="proxy">{t('settings_route_proxy')}</option>
                            <option value="direct">{t('settings_route_direct')}</option>
                          </select>
                        }
                      />

                      <SettingRow
                        id="streaming-routing-mode-row"
                        icon={<Zap size={16} />}
                        label={t('settings_stream_services_label')}
                        description={t('settings_stream_services_desc')}
                        query={query}
                        control={
                          <select
                            className="form-control"
                            style={{ width: '220px', fontSize: '12px' }}
                            value={settings.streaming_routing_mode || 'proxy'}
                            onChange={e => update('streaming_routing_mode', e.target.value)}
                          >
                            <option value="proxy">{t('settings_route_proxy')}</option>
                            <option value="direct">{t('settings_route_direct')}</option>
                          </select>
                        }
                      />
                    </>
                  )}
                </div>
              </Card>
            </div>

            {/* Dynamic SNI Fallback Pool Section */}
            <div className="section">
              <div className="section-title flex items-center justify-between">
                <div className="flex items-center gap-1.5">
                  <Globe size={14} className="text-primary" />
                  <span>{t('settings_sni_pool')}</span>
                </div>
                <Button variant="secondary" size="sm" className="text-[11px] flex items-center gap-1.5"
                  onClick={handleAuditSNIPool}
                  disabled={isAuditingSNI}
                >
                  <RefreshCw size={12} className={isAuditingSNI ? 'animate-spin' : ''} />
                  {isAuditingSNI ? t('settings_sni_auditing') : t('settings_sni_live_test')}
                </Button>
              </div>
              <Card className="p-4 space-y-4">
                <p className="text-xs text-muted-foreground">
                  {t('settings_sni_desc')}
                </p>

                {/* Add Custom SNI Input */}
                <div className="flex items-center gap-2">
                  <input
                    type="text"
                    placeholder={t('settings_sni_placeholder')}
                    className="form-control text-xs flex-1 font-mono"
                    value={newCustomSNI}
                    onChange={e => setNewCustomSNI(e.target.value)}
                    onKeyDown={e => e.key === 'Enter' && handleAddCustomSNI()}
                  />
                  <Button size="sm" className="text-xs px-3 flex items-center gap-1"
                    type="button"
                    onClick={handleAddCustomSNI}
                  >
                    <Plus size={13} />
                    {t('settings_sni_add')}
                  </Button>
                </div>

                <div className="grid grid-cols-1 sm:grid-cols-2 md:grid-cols-3 gap-2 text-xs">
                  {(sniPool.length > 0
                    ? sniPool
                    : [
                      { domain: 'gateway.icloud.com', provider: 'Apple Cloud', latency_ms: 0, status: 'untested' },
                      { domain: 'images.apple.com', provider: 'Apple CDN', latency_ms: 0, status: 'untested' },
                      { domain: 'www.microsoft.com', provider: 'Microsoft', latency_ms: 0, status: 'untested' },
                      { domain: 'learn.microsoft.com', provider: 'Microsoft Docs', latency_ms: 0, status: 'untested' },
                      { domain: 'aws.amazon.com', provider: 'Amazon AWS', latency_ms: 0, status: 'untested' },
                      { domain: 'cdn.cloudflare.com', provider: 'Cloudflare CDN', latency_ms: 0, status: 'untested' },
                      { domain: 'dl.google.com', provider: 'Google CDN', latency_ms: 0, status: 'untested' },
                      { domain: 'www.samsung.com', provider: 'Samsung Global', latency_ms: 0, status: 'untested' },
                    ]
                  ).map((item, idx) => (
                    <div key={idx} className="p-2.5 bg-card/70 border border-border rounded-xl flex items-center justify-between group">
                      <div className="flex-1 min-w-0 pe-2">
                        <span className="font-mono text-foreground block text-[11px] truncate">{item.domain}</span>
                        <span className="text-muted-foreground text-[10px]">{item.provider}</span>
                      </div>
                      <div className="flex items-center gap-1.5 flex-shrink-0">
                        {item.status === 'working' ? (
                          <span className="font-mono text-[11px] text-success font-bold bg-success/60 px-1.5 py-0.5 rounded border border-success/20">
                            {item.latency_ms} ms
                          </span>
                        ) : item.status === 'blocked' ? (
                          <span className="text-[10px] text-destructive bg-destructive/60 px-1.5 py-0.5 rounded border border-destructive/20">
                            {t('settings_sni_blocked')}
                          </span>
                        ) : (
                          <span className="text-[10px] text-muted-foreground bg-card/60 px-1.5 py-0.5 rounded">
                            {t('settings_sni_ready')}
                          </span>
                        )}
                        {item.provider === 'User Custom' && (
                          <button
                            type="button"
                            onClick={() => handleRemoveCustomSNI(item.domain)}
                            className="text-muted-foreground hover:text-destructive p-1 transition-colors"
                            title={t('settings_sni_remove')}
                          >
                            <Trash2 size={12} />
                          </button>
                        )}
                      </div>
                    </div>
                  ))}
                </div>
              </Card>
            </div>
          </div>
        )}

        {/* TAB 3: Per-App Split Tunneling */}
        {activeTab === 'apps' && (
          <div className="space-y-4">
            <div className="section">
              <div className="section-title">
                <Split size={13} style={{ display: 'inline', marginInlineEnd: 6, color: 'hsl(var(--info))' }} />
                {t('sec_split_tunneling')}
              </div>
              <Card >
                <div style={{ padding: 0 }}>
                  <SettingRow
                    id="app-routing-mode"
                    icon={<Layers size={16} />}
                    label={t('app_routing_label')}
                    description={t('app_routing_desc')}
                    query={query}
                    control={
                      <select
                        id="app-routing-select"
                        className="form-control"
                        style={{ width: '250px', fontSize: '12px' }}
                        value={settings.app_routing_mode || 'all'}
                        onChange={e => update('app_routing_mode', e.target.value)}
                      >
                        <option value="all">{t('app_mode_all')}</option>
                        <option value="proxy_selected">{t('app_mode_proxy_selected')}</option>
                        <option value="bypass_selected">{t('app_mode_bypass_selected')}</option>
                      </select>
                    }
                  />

                  {settings.app_routing_mode !== 'all' && (
                    <div style={{ padding: '16px 20px', borderBottom: '1px solid var(--color-border)' }}>
                      <div className="text-xs font-semibold text-foreground mb-2">{t('popular_apps_title')}</div>
                      <div className="flex flex-wrap gap-2 mb-3">
                        {popularApps.map(app => {
                          const isSelected = (settings.selected_apps || []).includes(app.exe)
                          return (
                            <button
                              key={app.exe}
                              type="button"
                              onClick={() => toggleApp(app.exe)}
                              className={`text-xs px-3 py-1.5 rounded-lg border font-medium transition ${isSelected
                                ? 'bg-info/20 border-primary text-primary'
                                : 'bg-card/80 border-border text-muted-foreground hover:text-foreground'
                                }`}
                            >
                              {isSelected ? '✓ ' : '+ '} {app.name}
                            </button>
                          )
                        })}
                      </div>

                      <div className="flex gap-2">
                        <input
                          type="text"
                          placeholder={t('custom_app_placeholder')}
                          className="form-control font-mono"
                          style={{ fontSize: '12px', flex: 1 }}
                          value={newAppInput}
                          onChange={e => setNewAppInput(e.target.value)}
                          onKeyDown={e => e.key === 'Enter' && addCustomApp()}
                        />
                        <Button variant="secondary" size="sm" type="button" onClick={addCustomApp}>
                          <Plus size={14} /> {t('add_app_btn')}
                        </Button>
                      </div>

                      {(settings.selected_apps || []).length > 0 && (
                        <div className="flex flex-wrap gap-1.5 mt-3">
                          {settings.selected_apps?.map(exe => (
                            <span
                              key={exe}
                              className="inline-flex items-center gap-1.5 bg-card text-foreground px-2.5 py-1 rounded-md text-xs font-mono border border-border"
                            >
                              {exe}
                              <X
                                size={12}
                                className="cursor-pointer hover:text-destructive"
                                onClick={() => toggleApp(exe)}
                              />
                            </span>
                          ))}
                        </div>
                      )}
                    </div>
                  )}
                </div>
              </Card>
            </div>
          </div>
        )}

        {/* TAB 4: Appearance & UI Themes */}
        {activeTab === 'appearance' && (
          <div className="space-y-4">
            <div className="section">
              <div className="section-title">
                <Palette size={13} style={{ display: 'inline', marginInlineEnd: 6, color: 'var(--color-accent)' }} />
                {t('sec_appearance')}
              </div>
              <Card >
                <div style={{ padding: 0 }}>
                  {/* Language Selector */}
                  <SettingRow
                    id="language-select-row"
                    icon={<Globe size={16} />}
                    label={t('language_label')}
                    description={t('language_desc')}
                    query={query}
                    control={
                      <select
                        id="language-select"
                        className="form-control"
                        style={{ width: '180px', fontSize: '13px' }}
                        value={getLanguage()}
                        onChange={e => update('language', e.target.value)}
                      >
                        <option value="fa">{t('lang_fa')}</option>
                        <option value="en">{t('lang_en')}</option>
                      </select>
                    }
                  />

                  {/* Floating Mini-Widget Toggle */}
                  <SettingRow
                    id="show-mini-widget-row"
                    icon={<Sparkles size={16} />}
                    label={t('show_mini_widget_label')}
                    description={t('show_mini_widget_desc')}
                    query={query}
                    control={
                      <label className="toggle">
                        <input
                          type="checkbox"
                          checked={!!settings.show_mini_widget}
                          onChange={e => update('show_mini_widget', e.target.checked)}
                        />
                        <span className="toggle-slider" />
                      </label>
                    }
                  />

                  {/* Futuristic Audio Effects */}
                  <SettingRow
                    id="sound-effects-row"
                    icon={<Volume2 size={16} />}
                    label={t('sound_effects_label')}
                    description={t('sound_effects_desc')}
                    query={query}
                    control={
                      <label className="toggle">
                        <input
                          type="checkbox"
                          checked={settings.sound_effects_enabled !== false}
                          onChange={e => {
                            update('sound_effects_enabled', e.target.checked)
                            sound.setEnabled(e.target.checked)
                            if (e.target.checked) sound.playSuccess()
                          }}
                        />
                        <span className="toggle-slider" />
                      </label>
                    }
                  />

                  {/* 2026 Trend Themes Palette */}
                  <div style={{ padding: '18px 20px', borderBottom: '1px solid var(--color-border)' }}>
                    <div className="flex items-center gap-3 mb-3">
                      <div style={{ color: 'var(--color-accent-light)' }}>
                        <Palette size={16} />
                      </div>
                      <div>
                        <div style={{ fontWeight: 600, fontSize: '13px', color: 'var(--color-text-primary)' }}>
                          {t('theme_label')}
                        </div>
                        <div className="text-xs text-muted-foreground" style={{ marginTop: '2px' }}>
                          {t('theme_desc')}
                        </div>
                      </div>
                    </div>

                    <div className="grid grid-cols-2 md:grid-cols-4 gap-2.5 mt-3">
                      {THEMES.map(th => {
                        const isSelected = selectedTheme === th.id
                        return (
                          <button
                            key={th.id}
                            type="button"
                            onClick={() => handleThemeSelect(th.id)}
                            className={`flex items-center gap-2.5 p-2.5 rounded-xl border text-start transition ${isSelected
                              ? 'border-primary bg-primary/15 shadow-sm'
                              : 'border-border bg-card/60 hover:border-border'
                              }`}
                            style={{
                              borderColor: isSelected ? 'var(--color-accent)' : undefined,
                              background: isSelected ? 'var(--color-bg-card-hover)' : undefined,
                            }}
                          >
                            <div
                              className="w-5 h-5 rounded-full border shadow-inner flex-shrink-0"
                              style={{
                                background: th.bgPreview,
                                borderColor: th.borderPreview,
                                boxShadow: `0 0 8px ${th.accentColor}40`,
                              }}
                            />
                            <div className="min-w-0">
                              <div className="text-xs font-bold truncate" style={{ color: 'var(--color-text-primary)' }}>
                                {currentLang === 'fa' ? th.nameFa : th.nameEn}
                              </div>
                              {isSelected && (
                                <div className="text-[10px] text-primary font-semibold" style={{ color: 'var(--color-accent-light)' }}>
                                  ✓ Active
                                </div>
                              )}
                            </div>
                          </button>
                        )
                      })}
                    </div>
                  </div>
                </div>
              </Card>
            </div>
          </div>
        )}

        {/* TAB 5: System, Core Binaries & Maintenance */}
        {activeTab === 'system' && (
          <div className="space-y-4">
            <div className="section">
              <div className="section-title">
                <SettingsIcon size={13} style={{ display: 'inline', marginInlineEnd: 6 }} />
                {t('system_behavior_title')}
              </div>
              <Card >
                <div style={{ padding: 0 }}>
                  {/* Auto Connect */}
                  <SettingRow
                    id="auto-connect"
                    icon={<RefreshCw size={16} />}
                    label={t('auto_connect_label')}
                    description={t('auto_connect_desc')}
                    query={query}
                    control={
                      <label className="toggle">
                        <input
                          type="checkbox"
                          checked={settings.auto_connect}
                          onChange={e => update('auto_connect', e.target.checked)}
                        />
                        <span className="toggle-slider" />
                      </label>
                    }
                  />

                  {/* Start on Windows Boot */}
                  <SettingRow
                    id="start-on-boot"
                    icon={<Cpu size={16} />}
                    label={t('start_on_boot_label')}
                    description={t('start_on_boot_desc')}
                    query={query}
                    control={
                      <label className="toggle">
                        <input
                          type="checkbox"
                          checked={settings.start_on_boot}
                          onChange={e => update('start_on_boot', e.target.checked)}
                        />
                        <span className="toggle-slider" />
                      </label>
                    }
                  />

                  {/* Start minimized to the tray */}
                  <SettingRow
                    id="start-minimized"
                    icon={<Minimize2 size={16} />}
                    label={t('start_minimized_label')}
                    description={t('start_minimized_desc')}
                    query={query}
                    control={
                      <label className="toggle">
                        <input
                          type="checkbox"
                          checked={settings.start_minimized}
                          onChange={e => update('start_minimized', e.target.checked)}
                        />
                        <span className="toggle-slider" />
                      </label>
                    }
                  />

                  {/* Kill Switch */}
                  <SettingRow
                    id="kill-switch"
                    icon={<Shield size={16} />}
                    label={t('kill_switch_label')}
                    description={t('kill_switch_desc')}
                    query={query}
                    control={
                      <label className="toggle">
                        <input
                          type="checkbox"
                          checked={settings.kill_switch_enabled}
                          onChange={e => update('kill_switch_enabled', e.target.checked)}
                        />
                        <span className="toggle-slider" />
                      </label>
                    }
                  />

                  {/* Smart Failover */}
                  <SettingRow
                    id="auto-failover"
                    icon={<RefreshCw size={16} />}
                    label={t('auto_failover_label')}
                    description={t('auto_failover_desc')}
                    query={query}
                    control={
                      <label className="toggle">
                        <input
                          type="checkbox"
                          checked={settings.auto_failover !== false}
                          onChange={e => update('auto_failover', e.target.checked)}
                        />
                        <span className="toggle-slider" />
                      </label>
                    }
                  />

                  {/* Subscription Auto Sync Interval */}
                  <SettingRow
                    id="sub-auto-update"
                    icon={<Clock size={16} />}
                    label={t('sub_auto_sync_label')}
                    description={t('sub_auto_sync_desc')}
                    query={query}
                    control={
                      <select
                        className="form-control"
                        style={{ width: '180px', fontSize: '12px' }}
                        value={settings.subscription_auto_update_hours || 12}
                        onChange={e => update('subscription_auto_update_hours', parseInt(e.target.value))}
                      >
                        <option value="6">{t('sync_every_6h')}</option>
                        <option value="12">{t('sync_every_12h')}</option>
                        <option value="24">{t('sync_every_24h')}</option>
                      </select>
                    }
                  />

                  {/* Clear Stale Proxy */}
                  <SettingRow
                    id="clear-proxy-row"
                    icon={<Eraser size={16} />}
                    label={t('clear_stale_proxy_label')}
                    description={t('clear_stale_proxy_desc')}
                    query={query}
                    control={
                      <Button variant="secondary" size="sm"
                        onClick={handleClearStaleProxy}
                        disabled={clearingProxy}
                      >
                        {clearingProxy ? '...' : proxyCleared ? '✓ Cleared' : t('clear_proxy_btn')}
                      </Button>
                    }
                  />
                  <SettingRow
                    id="flush-dns-row"
                    icon={<Globe size={16} />}
                    label={t('settings_dns_flush_label')}
                    description={t('settings_dns_flush_desc')}
                    query={query}
                    control={
                      <Button variant="secondary" size="sm"
                        onClick={handleFlushDNS}
                        disabled={flushingDNS}
                      >
                        {flushingDNS ? '...' : dnsFlushed ? t('settings_dns_flushed') : t('settings_dns_flush_btn')}
                      </Button>
                    }
                  />
                </div>
              </Card>
            </div>

            {/* 1-Click Diagnostics & Network Recovery Section */}
            <div className="section">
              <div className="section-title flex items-center justify-between">
                <div className="flex items-center gap-1.5">
                  <Activity size={14} className="text-primary" />
                  <span>{t('settings_diag_title')}</span>
                </div>
                <div className="flex items-center gap-2">
                  <Button variant="secondary" size="sm" className="text-[11px] flex items-center gap-1.5"
                    onClick={handleRunDiagnostics}
                    disabled={isRunningDiagnostics}
                  >
                    <RefreshCw size={12} className={isRunningDiagnostics ? 'animate-spin' : ''} />
                    {isRunningDiagnostics ? t('settings_diag_scanning') : t('settings_diag_scan_btn')}
                  </Button>
                  <Button size="sm" className="text-[11px] flex items-center gap-1.5"
                    onClick={handleRepairDiagnostics}
                    disabled={isRepairing}
                  >
                    <Wrench size={12} className={isRepairing ? 'animate-spin' : ''} />
                    {isRepairing ? t('settings_diag_repairing') : t('settings_diag_repair_btn')}
                  </Button>
                </div>
              </div>
              <Card className="p-4 space-y-3">
                <p className="text-xs text-muted-foreground">
                  {t('settings_diag_desc')}
                </p>

                {repairResult && (
                  <div className={`p-3 rounded-xl border text-xs ${repairResult.success ? 'bg-success/40 border-success/30 text-success' : 'bg-warning/40 border-warning/30 text-warning'}`}>
                    <div className="font-semibold mb-1 flex items-center gap-1.5">
                      <CheckCircle2 size={14} />
                      <span>{repairResult.message}</span>
                    </div>
                    {repairResult.actions_taken.length > 0 && (
                      <ul className="list-disc list-inside space-y-0.5 text-[11px] text-foreground">
                        {repairResult.actions_taken.map((act, i) => (
                          <li key={i}>{act}</li>
                        ))}
                      </ul>
                    )}
                  </div>
                )}

                {diagnosticReport ? (
                  <div className="space-y-3">
                    <div className="grid grid-cols-1 sm:grid-cols-2 md:grid-cols-4 gap-2 text-xs">
                      <div className="p-2.5 bg-card/80 rounded-xl border border-border">
                        <span className="text-muted-foreground block text-[11px]">{t('settings_diag_overall')}</span>
                        <span className={`font-bold ${diagnosticReport.healthy ? 'text-success' : 'text-destructive'}`}>
                          {diagnosticReport.healthy ? t('settings_diag_healthy') : t('settings_diag_warning')}
                        </span>
                      </div>
                      <div className="p-2.5 bg-card/80 rounded-xl border border-border">
                        <span className="text-muted-foreground block text-[11px]">{t('settings_diag_wintun')}</span>
                        <span className="font-medium text-primary text-[11px]">{diagnosticReport.wintun_status}</span>
                      </div>
                      <div className="p-2.5 bg-card/80 rounded-xl border border-border">
                        <span className="text-muted-foreground block text-[11px]">{t('settings_diag_dns')}</span>
                        <span className="font-medium text-primary text-[11px]">{diagnosticReport.dns_status}</span>
                      </div>
                      <div className="p-2.5 bg-card/80 rounded-xl border border-border">
                        <span className="text-muted-foreground block text-[11px]">{t('settings_diag_sysproxy')}</span>
                        <span className="font-medium text-warning text-[11px]">{diagnosticReport.proxy_status}</span>
                      </div>
                    </div>

                    {diagnosticReport.issues_found.length > 0 && (
                      <div className="p-3 bg-destructive/40 border border-destructive/30 rounded-xl text-xs text-destructive">
                        <span className="font-semibold block mb-1">{t('settings_diag_issues')}</span>
                        <ul className="list-disc list-inside space-y-1 text-[11px]">
                          {diagnosticReport.issues_found.map((iss, i) => (
                            <li key={i}>{iss}</li>
                          ))}
                        </ul>
                      </div>
                    )}
                  </div>
                ) : (
                  <div className="text-center py-2 text-xs text-muted-foreground">
                    {t('settings_diag_hint')}
                  </div>
                )}
              </Card>
            </div>

            {/* Encrypted Backup & Restore Section */}
            <div className="section">
              <div className="section-title flex items-center gap-1.5">
                <Lock size={14} className="text-success" />
                <span>{t('settings_backup_title')}</span>
              </div>
              <Card className="p-4 space-y-4">
                <p className="text-xs text-muted-foreground">
                  {t('settings_backup_desc')}
                </p>

                {backupSuccess && (
                  <div className="p-3 bg-success/40 border border-success/30 rounded-xl text-xs text-success flex items-center gap-2">
                    <CheckCircle2 size={14} className="text-success" />
                    <span>{backupSuccess}</span>
                  </div>
                )}

                {backupError && (
                  <div className="p-3 bg-destructive/40 border border-destructive/30 rounded-xl text-xs text-destructive flex items-center gap-2">
                    <AlertCircle size={14} className="text-destructive" />
                    <span>{backupError}</span>
                  </div>
                )}

                <div className="grid grid-cols-1 md:grid-cols-2 gap-4">
                  {/* Export Card */}
                  <div className="p-3.5 bg-card/60 rounded-xl border border-border space-y-3">
                    <span className="font-semibold text-xs text-foreground block">{t('settings_backup_export')}</span>
                    <div>
                      <label className="text-[11px] text-muted-foreground block mb-1">{t('settings_backup_password_label')}</label>
                      <input
                        type="password"
                        className="form-control text-xs w-full"
                        placeholder={t('settings_backup_password_placeholder')}
                        value={backupPassword}
                        onChange={e => setBackupPassword(e.target.value)}
                      />
                    </div>
                    <Button size="sm" className="w-full flex items-center justify-center gap-1.5 text-xs"
                      onClick={handleExportBackup}
                      disabled={isExportingBackup}
                    >
                      <Download size={13} />
                      {isExportingBackup ? t('settings_backup_generating') : t('settings_backup_download_btn')}
                    </Button>
                  </div>

                  {/* Import Card */}
                  <div className="p-3.5 bg-card/60 rounded-xl border border-border space-y-3">
                    <span className="font-semibold text-xs text-foreground block">{t('settings_backup_restore')}</span>
                    <div>
                      <label className="text-[11px] text-muted-foreground block mb-1">{t('settings_backup_content_label')}</label>
                      <textarea
                        rows={2}
                        className="form-control text-xs w-full font-mono text-[11px]"
                        placeholder={t('settings_backup_content_placeholder')}
                        value={importBackupString}
                        onChange={e => setImportBackupString(e.target.value)}
                      />
                    </div>
                    <div>
                      <input
                        type="password"
                        className="form-control text-xs w-full"
                        placeholder={t('settings_backup_file_password_placeholder')}
                        value={importPassword}
                        onChange={e => setImportPassword(e.target.value)}
                      />
                    </div>
                    <Button variant="secondary" size="sm" className="w-full flex items-center justify-center gap-1.5 text-xs"
                      onClick={handleImportBackup}
                      disabled={isImportingBackup || !importBackupString.trim()}
                    >
                      <Upload size={13} />
                      {isImportingBackup ? t('settings_backup_restoring') : t('settings_backup_restore_btn')}
                    </Button>
                  </div>
                </div>
              </Card>
            </div>

            {/* Subscription Quota & Webhook Alerts Section */}
            <div className="section">
              <div className="section-title flex items-center gap-1.5">
                <Activity size={14} className="text-warning" />
                <span>{t('settings_quota_title')}</span>
              </div>
              <Card className="p-4 space-y-4">
                <p className="text-xs text-muted-foreground">
                  {t('settings_quota_desc')}
                </p>

                {quotaResult && (
                  <div className={`p-3 rounded-xl border text-xs ${quotaResult.alert_triggered ? 'bg-warning/40 border-warning/30 text-warning' : "bg-card/80 border-border text-foreground"}`}>
                    <div className="grid grid-cols-2 sm:grid-cols-4 gap-2 text-xs">
                      <div>
                        <span className="text-muted-foreground block text-[11px]">{t('settings_quota_total')}</span>
                        <span className="font-bold">{(quotaResult.total_bytes / (1024 * 1024 * 1024)).toFixed(1)} GB</span>
                      </div>
                      <div>
                        <span className="text-muted-foreground block text-[11px]">{t('settings_quota_used')}</span>
                        <span className="font-bold text-primary">{(quotaResult.used_bytes / (1024 * 1024 * 1024)).toFixed(1)} GB ({quotaResult.used_percent.toFixed(1)}%)</span>
                      </div>
                      <div>
                        <span className="text-muted-foreground block text-[11px]">{t('settings_quota_remaining')}</span>
                        <span className="font-bold text-success">{(quotaResult.remaining_bytes / (1024 * 1024 * 1024)).toFixed(1)} GB</span>
                      </div>
                      <div>
                        <span className="text-muted-foreground block text-[11px]">{t('settings_quota_expiry')}</span>
                        <span className="font-bold text-primary">{quotaResult.days_remaining} {t('settings_unit_days')}</span>
                      </div>
                    </div>
                    {quotaResult.alert_triggered && (
                      <div className="mt-2 text-destructive font-semibold text-[11px] flex items-center gap-1">
                        <AlertCircle size={13} />
                        <span>{quotaResult.alert_reason}</span>
                      </div>
                    )}
                  </div>
                )}

                <div className="grid grid-cols-1 md:grid-cols-2 gap-3">
                  <div className="space-y-1.5">
                    <label className="text-[11px] text-muted-foreground block">{t('settings_quota_url_label')}</label>
                    <div className="flex items-center gap-2">
                      <input
                        type="text"
                        className="form-control text-xs flex-1 font-mono"
                        placeholder="https://example.com/sub/..."
                        value={quotaCheckUrl}
                        onChange={e => setQuotaCheckUrl(e.target.value)}
                      />
                      <Button variant="secondary" size="sm" className="text-xs"
                        onClick={handleCheckQuota}
                        disabled={checkingQuota || !quotaCheckUrl.trim()}
                      >
                        {checkingQuota ? '...' : t('settings_quota_check_btn')}
                      </Button>
                    </div>
                  </div>

                  <div className="space-y-1.5">
                    <label className="text-[11px] text-muted-foreground block">{t('settings_quota_webhook_label')}</label>
                    <input
                      type="text"
                      className="form-control text-xs w-full font-mono"
                      placeholder="https://api.telegram.org/bot.../sendMessage"
                      value={settings.quota_webhook_url || ''}
                      onChange={e => update('quota_webhook_url', e.target.value)}
                    />
                  </div>

                  {/* Monthly allowance and alert threshold, used by the Quota
                    Manager and the background quota monitor. */}
                  <div className="grid grid-cols-2 gap-3">
                    <div className="space-y-1.5">
                      <label className="text-[11px] text-muted-foreground block">{t('settings_monthly_quota_label')}</label>
                      <input
                        type="number"
                        min={0}
                        max={100000}
                        className="form-control text-xs w-full font-mono"
                        value={settings.monthly_quota_gb ?? 50}
                        onChange={e => update('monthly_quota_gb', parseInt(e.target.value) || 0)}
                      />
                      <p className="text-[10px] text-muted-foreground leading-relaxed">{t('settings_monthly_quota_desc')}</p>
                    </div>
                    <div className="space-y-1.5">
                      <label className="text-[11px] text-muted-foreground block">{t('settings_quota_alert_label')}</label>
                      <select
                        className="form-control text-xs w-full"
                        value={settings.quota_alert_percentage ?? 80}
                        onChange={e => update('quota_alert_percentage', parseInt(e.target.value))}
                      >
                        <option value={70}>70%</option>
                        <option value={80}>80%</option>
                        <option value={90}>90%</option>
                        <option value={100}>100%</option>
                      </select>
                      <p className="text-[10px] text-muted-foreground leading-relaxed">{t('settings_quota_alert_desc')}</p>
                    </div>
                  </div>
                </div>
              </Card>
            </div>

            {/* Core Binaries & Routing Databases Management Section */}
            <div className="section">
              <div className="section-title flex items-center justify-between">
                <div>
                  <Cpu size={13} style={{ display: 'inline', marginInlineEnd: 6 }} />
                  {t('sec_cores')}
                </div>
                <Button variant="secondary" size="sm" className="text-[11px] flex items-center gap-1.5"
                  onClick={handleUpdateGeoData}
                  disabled={updatingGeo}
                >
                  <RefreshCw size={12} className={updatingGeo ? 'animate-spin' : ''} />
                  {updatingGeo ? t('updating') : geoUpdated ? '✓ GeoIP & GeoSite Updated' : t('update_geo_rules')}
                </Button>
              </div>
              <Card >
                <div className="flex items-center justify-between">
                  <p className="text-xs text-muted-foreground" style={{ margin: 0 }}>
                    {t('cores_desc')}
                  </p>
                  <Button size="sm"
                    onClick={handleAutoInstallMissingCores}
                    disabled={isInstallingCores}
                  >
                    <Download size={13} />
                    {isInstallingCores ? t('installing') : t('auto_install_all')}
                  </Button>
                </div>
                <div style={{ padding: 0 }}>
                  {/* Global / Active Download Progress Banner */}
                  {downloadProgress && (
                    <div className="p-3.5 bg-info/30 border-b border-primary/20 space-y-2 animate-in fade-in">
                      <div className="flex items-center justify-between text-xs text-primary">
                        <span className="flex items-center gap-2 font-semibold">
                          <RefreshCw className="w-3.5 h-3.5 text-primary animate-spin" />
                          <span>{t('settings_downloading')} {binaries[downloadProgress.key]?.name || downloadProgress.key}: {downloadProgress.status} ({downloadProgress.percent}%)</span>
                        </span>
                        {downloadProgress.speed > 0 && (
                          <span className="font-mono text-primary/80 text-[11px]">
                            {(downloadProgress.speed / (1024 * 1024)).toFixed(2)} MB/s
                          </span>
                        )}
                      </div>
                      <div className="w-full h-2 bg-card rounded-full overflow-hidden">
                        <div
                          className="bg-info h-full transition-all duration-200"
                          style={{ width: `${downloadProgress.percent}%` }}
                        />
                      </div>
                    </div>
                  )}
                  {downloadError && (
                    <div className="p-3 bg-destructive/50 border-b border-destructive/30 flex items-start gap-2 text-xs text-destructive">
                      <AlertCircle size={14} className="text-destructive mt-0.5 flex-shrink-0" />
                      <div>
                        <span className="font-bold">{t('settings_download_error')} {downloadError.key}: </span>
                        <span className="font-mono text-[11px] break-all">{downloadError.msg}</span>
                      </div>
                    </div>
                  )}

                  {Object.entries(binaries).map(([key, info]) => (
                    <div
                      key={key}
                      className="setting-row"
                      style={{
                        display: 'flex',
                        alignItems: 'center',
                        justifyContent: 'space-between',
                        padding: '14px 20px',
                        borderBottom: '1px solid var(--color-border)',
                      }}
                    >
                      <div>
                        <div className="flex items-center gap-2">
                          <span style={{ fontWeight: 600, fontSize: '13px', color: 'var(--color-text-primary)' }}>
                            {info.name}
                          </span>
                          {info.version && (
                            <span className="text-[10px] font-mono px-1.5 py-0.5 rounded bg-card text-primary border border-border">
                              {info.version}
                            </span>
                          )}
                        </div>
                        <div className="text-xs text-muted-foreground font-mono">{info.filename}</div>
                      </div>

                      <div className="flex items-center gap-2">
                        {(!info.download_url || info.download_url === '') ? (
                          <span className="text-[11px] px-2.5 py-1 rounded bg-card text-muted-foreground border border-border font-mono" title={t('settings_core_bundled_title')}>
                            {t('settings_core_bundled')}
                          </span>
                        ) : (
                          <Button variant="secondary" size="sm" className="text-[11px] px-2.5 py-1 flex items-center gap-1"
                            type="button"
                            onClick={() => handleUpdateCore(key)}
                            disabled={updatingCore === key || (downloadProgress?.key === key)}
                            title={info.download_url}
                          >
                            <RefreshCw size={11} className={updatingCore === key || (downloadProgress?.key === key) ? 'animate-spin' : ''} />
                            {updatingCore === key || (downloadProgress?.key === key) ? (downloadProgress?.percent ? `${downloadProgress.percent}%` : t('updating')) : t('update_core_btn')}
                          </Button>
                        )}

                        <span
                          className={`badge ${info.status === 'installed' ? 'badge-success' : 'badge-error'}`}
                        >
                          {info.status === 'installed' ? (
                            <>
                              <CheckCircle2 size={11} style={{ marginInlineEnd: 4 }} />
                              {t('installed')}
                            </>
                          ) : (
                            <>
                              <AlertCircle size={11} style={{ marginInlineEnd: 4 }} />
                              {t('missing')}
                            </>
                          )}
                        </span>
                      </div>
                    </div>
                  ))}
                </div>
              </Card>
            </div>

            {/* Routing Rules Maintenance */}
            <div className="section">
              <div className="section-title flex items-center gap-1.5">
                <Globe size={13} className="text-primary" />
                <span>{t('sec_routing_assets')}</span>
              </div>
              <Card className="p-0">
                <div className="p-0">
                  <SettingRow
                    id="update-geo-row"
                    icon={<Download size={16} />}
                    label={t('update_geo_label')}
                    description={t('update_geo_desc')}
                    query={query}
                    control={
                      <Button variant="secondary" size="sm" className="text-[11px] px-2.5 py-1 flex items-center gap-1"
                        type="button"
                        onClick={handleUpdateGeoData}
                        disabled={updatingGeo}
                      >
                        <RefreshCw size={11} className={updatingGeo ? 'animate-spin' : ''} />
                        {updatingGeo ? t('updating') : t('update_geo_btn')}
                      </Button>
                    }
                  />

                  {/* Auto failover between healthy profiles */}
                  <SettingRow
                    id="auto-failover-row"
                    icon={<Zap size={16} />}
                    label={t('auto_failover_label')}
                    description={t('auto_failover_desc')}
                    query={query}
                    control={
                      <label className="toggle">
                        <input
                          type="checkbox"
                          checked={!!settings.auto_failover_enabled}
                          onChange={e => handleToggleFailover(e.target.checked)}
                        />
                        <span className="toggle-slider" />
                      </label>
                    }
                  />
                </div>
              </Card>
            </div>

            {/* Privacy & Security Hardening */}
            <div className="section">
              <div className="section-title flex items-center gap-1.5">
                <Shield size={13} className="text-success" />
                <span>{t('sec_hardening')}</span>
              </div>
              <Card className="p-0">
                <div className="p-0">
                  {/* Custom DoH endpoint */}
                  <SettingRow
                    id="custom-doh-row"
                    icon={<Globe size={16} />}
                    label={t('custom_doh_label')}
                    description={t('custom_doh_desc')}
                    query={query}
                    control={
                      <input
                        type="text"
                        dir="ltr"
                        className="form-control text-xs w-64 font-mono"
                        placeholder="https://1.1.1.1/dns-query"
                        value={settings.custom_doh_url || ''}
                        onChange={e => update('custom_doh_url', e.target.value)}
                      />
                    }
                  />

                  {/* Block ads / trackers */}
                  <SettingRow
                    id="block-ads-row"
                    icon={<Shield size={16} />}
                    label={t('block_ads_label')}
                    description={t('block_ads_desc')}
                    query={query}
                    control={
                      <label className="toggle">
                        <input
                          type="checkbox"
                          checked={!!settings.block_ads}
                          onChange={e => update('block_ads', e.target.checked)}
                        />
                        <span className="toggle-slider" />
                      </label>
                    }
                  />

                  {/* LAN controller: restrict to local network */}
                  <SettingRow
                    id="remote-lan-only-row"
                    icon={<Shield size={16} />}
                    label={t('remote_lan_only_label')}
                    description={t('remote_lan_only_desc')}
                    query={query}
                    control={
                      <label className="toggle">
                        <input
                          type="checkbox"
                          // Defaults on in the backend; the merge guard means this
                          // can only be turned off here, deliberately.
                          checked={settings.remote_controller_lan_only !== false}
                          onChange={e => update('remote_controller_lan_only', e.target.checked)}
                        />
                        <span className="toggle-slider" />
                      </label>
                    }
                  />

                  {/* LAN controller: TLS */}
                  <SettingRow
                    id="remote-tls-row"
                    icon={<Lock size={16} />}
                    label={t('remote_tls_label_setting')}
                    description={t('remote_tls_desc_setting')}
                    query={query}
                    control={
                      <label className="toggle">
                        <input
                          type="checkbox"
                          checked={settings.remote_controller_tls !== false}
                          onChange={e => update('remote_controller_tls', e.target.checked)}
                        />
                        <span className="toggle-slider" />
                      </label>
                    }
                  />

                  {/* Unverified core downloads — dangerous, off by default */}
                  <SettingRow
                    id="unverified-core-row"
                    icon={<AlertTriangle size={16} />}
                    label={t('allow_unverified_core_label')}
                    description={t('allow_unverified_core_desc')}
                    query={query}
                    control={
                      <label className="toggle">
                        <input
                          type="checkbox"
                          checked={!!settings.allow_unverified_core_downloads}
                          onChange={e => update('allow_unverified_core_downloads', e.target.checked)}
                        />
                        <span className="toggle-slider" />
                      </label>
                    }
                  />

                  {/* WebView2 rendering fallbacks — read once at startup */}
                  <SettingRow
                    id="webview-disable-gpu-row"
                    icon={<Zap size={16} />}
                    label={t('webview_disable_gpu_label')}
                    description={t('webview_disable_gpu_desc')}
                    query={query}
                    control={
                      <label className="toggle">
                        <input
                          type="checkbox"
                          checked={!!settings.webview_disable_gpu}
                          onChange={e => update('webview_disable_gpu', e.target.checked)}
                        />
                        <span className="toggle-slider" />
                      </label>
                    }
                  />

                  <SettingRow
                    id="webview-disable-code-integrity-row"
                    icon={<Shield size={16} />}
                    label={t('webview_disable_ci_label')}
                    description={t('webview_disable_ci_desc')}
                    query={query}
                    control={
                      <label className="toggle">
                        <input
                          type="checkbox"
                          checked={!!settings.webview_disable_code_integrity}
                          onChange={e => update('webview_disable_code_integrity', e.target.checked)}
                        />
                        <span className="toggle-slider" />
                      </label>
                    }
                  />
                </div>
              </Card>
            </div>

            {/* Global Keyboard Shortcuts Guide */}
            <div className="section">
              <div className="section-title">
                <Keyboard size={13} style={{ display: 'inline', marginInlineEnd: 6, color: 'hsl(var(--info))' }} />
                {t('global_hotkeys_title')}
              </div>
              <Card className="p-4 space-y-3">
                <p className="text-xs text-muted-foreground">
                  {t('global_hotkeys_desc')}
                </p>
                <div className="grid grid-cols-1 sm:grid-cols-2 gap-2 text-xs">
                  <div className="bg-card/70 p-2.5 rounded-xl border border-border flex items-center justify-between">
                    <span className="text-foreground">{t('hotkey_toggle_connect')}</span>
                    <kbd className="font-mono font-bold bg-card text-primary px-2 py-0.5 rounded text-[11px] border border-border">
                      Ctrl + Shift + V
                    </kbd>
                  </div>

                  <div className="bg-card/70 p-2.5 rounded-xl border border-border flex items-center justify-between">
                    <span className="text-foreground">{t('hotkey_speed_test')}</span>
                    <kbd className="font-mono font-bold bg-card text-success px-2 py-0.5 rounded text-[11px] border border-border">
                      Ctrl + Shift + S
                    </kbd>
                  </div>

                  <div className="bg-card/70 p-2.5 rounded-xl border border-border flex items-center justify-between">
                    <span className="text-foreground">{t('hotkey_radar')}</span>
                    <kbd className="font-mono font-bold bg-card text-primary px-2 py-0.5 rounded text-[11px] border border-border">
                      Ctrl + Shift + R
                    </kbd>
                  </div>

                  <div className="bg-card/70 p-2.5 rounded-xl border border-border flex items-center justify-between">
                    <span className="text-foreground">{t('hotkey_lan_share')}</span>
                    <kbd className="font-mono font-bold bg-card text-primary px-2 py-0.5 rounded text-[11px] border border-border">
                      Ctrl + Shift + L
                    </kbd>
                  </div>

                  <div className="bg-card/70 p-2.5 rounded-xl border border-border flex items-center justify-between">
                    <span className="text-foreground">{t('hotkey_emergency')}</span>
                    <kbd className="font-mono font-bold bg-card text-destructive px-2 py-0.5 rounded text-[11px] border border-border">
                      Ctrl + Shift + E
                    </kbd>
                  </div>

                  <div className="bg-card/70 p-2.5 rounded-xl border border-border flex items-center justify-between">
                    <span className="text-foreground">{t('hotkey_quota')}</span>
                    <kbd className="font-mono font-bold bg-card text-warning px-2 py-0.5 rounded text-[11px] border border-border">
                      Ctrl + Shift + Q
                    </kbd>
                  </div>
                </div>
              </Card>
            </div>
          </div>
        )}

        {/* TAB 6: Automation & rules */}
        {activeTab === 'automation' && settings && (
          <AutomationPanel settings={settings} update={update} />
        )}

        {/* TAB 7: Diagnostics */}
        {activeTab === 'diagnostics' && settings && (
          <DiagnosticsPanel settings={settings} update={update} connectionState={connectionState} />
        )}
      </SettingMatchContext.Provider>
    </div>
  )
}

function SettingRow({
  id,
  icon,
  label,
  description,
  control,
  query,
}: {
  id: string
  icon: React.ReactNode
  label: string
  description: string
  control: React.ReactNode
  /** When non-empty, the row is hidden unless label/description match. */
  query?: string
}) {
  const report = React.useContext(SettingMatchContext)
  const q = (query ?? '').trim().toLowerCase()
  const matches = !q || `${label} ${description}`.toLowerCase().includes(q)

  React.useEffect(() => {
    if (report) report(id, matches)
    return () => {
      if (report) report(id, false)
    }
  }, [report, id, matches])

  if (!matches) return null
  return (
    <div
      id={id}
      className="setting-row"
      style={{
        display: 'flex',
        alignItems: 'center',
        justifyContent: 'space-between',
        padding: '18px 20px',
        borderBottom: '1px solid var(--color-border)',
        gap: '16px',
      }}
    >
      <div className="flex items-center gap-3">
        <div style={{ color: 'var(--color-accent-light)' }}>{icon}</div>
        <div>
          <div style={{ fontWeight: 600, fontSize: '13px', color: 'var(--color-text-primary)' }}>
            {label}
          </div>
          <div className="text-xs text-muted-foreground" style={{ marginTop: '2px' }}>
            {description}
          </div>
        </div>
      </div>
      <div>{control}</div>
    </div>
  )
}
