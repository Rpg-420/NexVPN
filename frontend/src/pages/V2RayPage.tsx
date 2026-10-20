import React, { useState, useEffect, useCallback, useMemo } from 'react'
import {
  Plus,
  Trash2,
  Globe,
  Copy,
  Check,
  RefreshCw,
  QrCode,
  Wand2,
  X,
  Radio,
  Search,
  Link,
  Activity,
  FolderMinus,
  FolderSync,
  Layers,
  Sparkles,
  Download,
  ArrowUpDown,
  Zap,
  Scan,
  ArrowRightLeft,
  ClipboardList,
  AlertCircle,
} from 'lucide-react'
import { api } from '../lib/api'
import { t } from '../lib/i18n'
import type { Profile, ConnectionStatus, SubscriptionUserInfo } from '../types'
import CountryFlag, { cleanServerName } from '../components/CountryFlag'
import { generateQRCodeSVG } from '../lib/qrcode'
import EditProfileModal from '../components/EditProfileModal'
import ScreenQRScannerModal from '../components/ScreenQRScannerModal'
import ConfigConverterModal from '../components/ConfigConverterModal'
import { ModalShell } from '../components/ui/modal-shell'
import { ProfileCard } from '../components/ui/profile-card'
import { PingProgressBar } from '../components/PingProgressBar'
import { usePingProgress } from '../hooks/usePingProgress'
import { pinnedFirst, toggleProfilePin } from '../lib/profilePin'
import { Button } from '@/components/ui/button'
import { Card } from '@/components/ui/card'

interface Props {
  status: ConnectionStatus
  onStatusChange?: (s: ConnectionStatus) => void
}

type SortMode = 'default' | 'ping_asc' | 'ping_desc' | 'country' | 'name'

// Helper to determine exact protocol label
function getDisplayProtocol(p: Profile): { label: string; color: string } {
  const raw = (p.raw_uri || '').toLowerCase()
  if (raw.startsWith('vless://')) {
    if (raw.includes('security=reality')) return { label: 'VLESS Reality', color: 'bg-success/10 text-success border-success/20' }
    return { label: 'VLESS', color: 'bg-info/10 text-info border-info/20' }
  }
  if (raw.startsWith('vmess://')) return { label: 'VMess', color: 'bg-primary/10 text-primary border-primary/20' }
  if (raw.startsWith('trojan://')) return { label: 'Trojan', color: 'bg-warning/10 text-warning border-warning/20' }
  if (raw.startsWith('ss://')) return { label: 'Shadowsocks', color: 'bg-info/10 text-info border-info/20' }
  if (raw.startsWith('hysteria2://') || raw.startsWith('hy2://')) return { label: 'Hysteria 2', color: 'bg-destructive/10 text-destructive border-destructive/20' }
  if (raw.startsWith('wireguard://') || raw.startsWith('warp://')) return { label: 'WireGuard', color: 'bg-primary/10 text-primary border-primary/20' }
  if (raw.startsWith('tuic://')) return { label: 'TUIC', color: 'bg-success/10 text-success border-success/20' }
  if (p.protocol && p.protocol !== 'singbox' && p.protocol !== 'xray') {
    return { label: p.protocol.toUpperCase(), color: 'bg-info/10 text-info border-info/20' }
  }
  return { label: 'VLESS', color: 'bg-info/10 text-info border-info/20' }
}

export default function V2RayPage({ status, onStatusChange }: Props) {
  const [profiles, setProfiles] = useState<Profile[]>([])
  const [search, setSearch] = useState('')

  const [filterGroup, setFilterGroupState] = useState<string>(() => {
    return localStorage.getItem('nexvpn_v2ray_filter_group') || 'all'
  })
  const setFilterGroup = (grp: string) => {
    setFilterGroupState(grp)
    localStorage.setItem('nexvpn_v2ray_filter_group', grp)
  }

  const [sortMode, setSortModeState] = useState<SortMode>(() => {
    return (localStorage.getItem('nexvpn_v2ray_sort') as SortMode) || 'default'
  })
  const setSortMode = (mode: SortMode) => {
    setSortModeState(mode)
    localStorage.setItem('nexvpn_v2ray_sort', mode)
  }
  const [showAddModal, setShowAddModal] = useState(false)
  const [showImportModal, setShowImportModal] = useState(false)
  const [showScreenQRModal, setShowScreenQRModal] = useState(false)
  const [showConverterModal, setShowConverterModal] = useState(false)
  const [editingProfile, setEditingProfile] = useState<Profile | null>(null)
  const [sharingProfile, setSharingProfile] = useState<Profile | null>(null)
  const [renamingGroup, setRenamingGroup] = useState<string | null>(null)
  const [isPinging, setIsPinging] = useState(false)
  const pingProgress = usePingProgress(isPinging)
  const [isTestingRealDelay, setIsTestingRealDelay] = useState(false)
  const [isUpdatingSubs, setIsUpdatingSubs] = useState(false)
  const [subUserInfo, setSubUserInfo] = useState<SubscriptionUserInfo | null>(null)
  const [copiedId, setCopiedId] = useState<string | null>(null)
  const [optimizingSniId, setOptimizingSniId] = useState<string | null>(null)
  const [backendGroups, setBackendGroups] = useState<string[]>([])
  const [toastMessage, setToastMessage] = useState<string | null>(null)

  const showToast = (msg: string) => {
    setToastMessage(msg)
    setTimeout(() => setToastMessage(null), 3000)
  }

  const loadProfiles = useCallback(async () => {
    try {
      const p = await api.getProfilesByCategory('v2ray')
      if (p && Array.isArray(p)) setProfiles(p)
    } catch (err) {
      console.error('Failed to load V2Ray profiles:', err)
    }
  }, [])

  useEffect(() => {
    loadProfiles()
    // Refresh the backend group list too; failures are non-fatal because the
    // client-side derivation still provides a usable filter.
    api
      .getGroupsByCategory('v2ray')
      .then(g => setBackendGroups(Array.isArray(g) ? g : []))
      .catch(err => console.error('Failed to load groups:', err))
  }, [loadProfiles])

  const handleImportFromClipboard = async () => {
    try {
      const text = await navigator.clipboard.readText()
      if (!text || !text.trim()) {
        showToast(t('clipboard_empty'))
        return
      }
      const count = await api.importSubscription(text.trim())
      await loadProfiles()
      showToast(t('imported_nodes_count', { count: count || 1 }))
    } catch (err) {
      console.error('Clipboard import error:', err)
      showToast(t('import_failed'))
    }
  }

  const activeGroupProfiles = useMemo(() => {
    if (filterGroup === 'all') return []
    return profiles.filter(p => (p.subscription_name || p.group) === filterGroup)
  }, [profiles, filterGroup])

  const activeSubUrl = useMemo(() => {
    const found = activeGroupProfiles.find(p => p.subscription_url)
    return found ? found.subscription_url : null
  }, [activeGroupProfiles])

  useEffect(() => {
    if (activeSubUrl) {
      api.getSubscriptionInfo(activeSubUrl)
        .then(info => setSubUserInfo(info))
        .catch(() => setSubUserInfo(null))
    } else {
      setSubUserInfo(null)
    }
  }, [activeSubUrl])

  const qrImageUrl = sharingProfile
    ? generateQRCodeSVG(sharingProfile.raw_uri || sharingProfile.config_content || sharingProfile.name)
    : ''

  const handlePingAll = async () => {
    if (isPinging) return
    setIsPinging(true)
    try {
      const updated = await api.pingAllProfiles('v2ray')
      setProfiles(updated)
      showToast(t('saved_successfully'))
    } catch (err) {
      console.error('Ping all error:', err)
    } finally {
      setIsPinging(false)
    }
  }

  const handleCancelPingAll = () => {
    api.cancelPingAll().catch(() => {})
  }

  const handleRealDelayAll = async () => {
    setIsTestingRealDelay(true)
    try {
      const results = await api.testAllProfilesRealDelay('v2ray')
      if (Array.isArray(results)) {
        await loadProfiles()
        showToast(t('saved_successfully'))
      }
    } catch (err) {
      console.error('Real delay error:', err)
    } finally {
      setIsTestingRealDelay(false)
    }
  }

  const handleUpdateAllSubscriptions = async () => {
    setIsUpdatingSubs(true)
    try {
      await api.updateAllSubscriptions()
      await loadProfiles()
      showToast(t('saved_successfully'))
    } catch (err) {
      console.error('Update subs error:', err)
    } finally {
      setIsUpdatingSubs(false)
    }
  }

  const handleUpdateActiveGroup = async () => {
    if (filterGroup === 'all') return
    setIsUpdatingSubs(true)
    try {
      await api.updateSubscriptionGroup(filterGroup)
      await loadProfiles()
      showToast(t('saved_successfully'))
    } catch (err) {
      console.error('Update group error:', err)
      alert(t('error'))
    } finally {
      setIsUpdatingSubs(false)
    }
  }

  const handleDeleteActiveGroup = async () => {
    if (filterGroup === 'all') return
    if (window.confirm(t('confirm_delete'))) {
      try {
        await api.deleteGroup(filterGroup)
        setFilterGroup('all')
        await loadProfiles()
        showToast(t('saved_successfully'))
      } catch (err) {
        console.error('Delete group error:', err)
      }
    }
  }

  const handleDeduplicate = async () => {
    try {
      const count = await api.deduplicateProfiles('v2ray')
      await loadProfiles()
      showToast(count > 0 ? `${count} duplicate profiles removed` : 'No duplicate profiles found')
    } catch (err) {
      console.error('Deduplicate error:', err)
    }
  }

  const handleRemoveDead = async () => {
    if (window.confirm(t('confirm_delete'))) {
      try {
        const count = await api.removeDeadProfiles('v2ray')
        await loadProfiles()
        showToast(count > 0 ? `${count} dead profiles removed` : 'No dead profiles in list')
      } catch (err) {
        console.error('Remove dead error:', err)
      }
    }
  }

  const handleExportAll = async () => {
    try {
      const data = await api.exportAllProfiles('v2ray')
      if (!data) {
        showToast(t('no_profiles_found'))
        return
      }
      await navigator.clipboard.writeText(data)
      showToast(t('link_copied'))
    } catch (err) {
      console.error('Export all error:', err)
    }
  }

  const handleDeleteAll = async () => {
    if (window.confirm(t('confirm_delete_all') || 'Are you sure you want to delete all V2Ray nodes?')) {
      try {
        await api.deleteProfilesByCategory('v2ray')
        await loadProfiles()
        showToast(t('saved_successfully'))
      } catch (err) {
        console.error('Delete all error:', err)
      }
    }
  }

  const handlePingSingle = async (id: string, e: React.MouseEvent) => {
    e.stopPropagation()
    try {
      const ms = await api.pingProfile(id)
      setProfiles(prev => prev.map(p => (p.id === id ? { ...p, latency_ms: ms } : p)))
    } catch (err) {
      console.error('Ping single error:', err)
    }
  }

  // Swaps the profile's SNI for the fastest healthy entry from the live pool.
  const handleOptimizeSNI = async (id: string, e: React.MouseEvent) => {
    e.stopPropagation()
    setOptimizingSniId(id)
    try {
      const updated = await api.optimizeProfileSNI(id)
      if (updated) {
        setProfiles(prev => prev.map(p => (p.id === id ? updated : p)))
        showToast(t('sni_optimized_toast'))
      } else {
        showToast(t('sni_no_better_candidate'))
      }
    } catch (err) {
      console.error('Optimize SNI error:', err)
      showToast(t('error'))
    } finally {
      setOptimizingSniId(null)
    }
  }

  const handleConnect = async (profileId: string) => {
    try {
      const targetP = profiles.find(p => p.id === profileId)
      if (onStatusChange) {
        onStatusChange({
          state: 'connecting',
          profile_id: profileId,
          profile_name: targetP?.name || '',
          protocol: targetP?.protocol,
          server: targetP?.server || '',
        })
      }
      await api.connect(profileId)
      const newSt = await api.getConnectionStatus()
      if (onStatusChange) onStatusChange(newSt)
    } catch (err) {
      console.error('Connect error:', err)
      if (onStatusChange) {
        onStatusChange({ state: 'error', error_msg: String(err) })
      }
    }
  }

  const handleDisconnect = async () => {
    try {
      if (onStatusChange) {
        onStatusChange({ state: 'disconnecting' })
      }
      await api.disconnect()
      const newSt = await api.getConnectionStatus()
      if (onStatusChange) onStatusChange(newSt)
    } catch (err) {
      console.error('Disconnect error:', err)
    }
  }

  const handleDelete = async (id: string, e?: React.MouseEvent) => {
    e?.stopPropagation()
    if (window.confirm(t('confirm_delete'))) {
      try {
        await api.deleteProfile(id)
        setProfiles(prev => prev.filter(p => p.id !== id))
      } catch (err) {
        console.error('Delete error:', err)
      }
    }
  }

  const handleCopyURI = (p: Profile, e: React.MouseEvent) => {
    e.stopPropagation()
    const text = p.raw_uri || p.config_content || p.name
    navigator.clipboard.writeText(text)
    setCopiedId(p.id)
    setTimeout(() => setCopiedId(null), 2000)
  }

  // Extract unique groups & subscription names.
  // Intersect the client-side view (which also surfaces subscription names the
  // backend does not track as groups) with the backend's authoritative group
  // list, so a group that only exists server-side is still selectable.
  const availableGroups = useMemo(() => {
    const local = new Set(
      profiles.map(p => p.subscription_name || p.group).filter(Boolean) as string[]
    )
    for (const g of backendGroups) local.add(g)
    return Array.from(local).sort((a, b) => a.localeCompare(b))
  }, [profiles, backendGroups])

  let filtered = profiles.filter(p => {
    const matchSearch =
      !search ||
      p.name.toLowerCase().includes(search.toLowerCase()) ||
      p.server?.toLowerCase().includes(search.toLowerCase()) ||
      p.group?.toLowerCase().includes(search.toLowerCase())

    const grp = p.subscription_name || p.group
    const matchGroup = filterGroup === 'all' || grp === filterGroup

    return matchSearch && matchGroup
  })

  // Apply sorting
  if (sortMode === 'ping_asc') {
    filtered.sort((a, b) => {
      const pingA = a.latency_ms && a.latency_ms > 0 ? a.latency_ms : 99999
      const pingB = b.latency_ms && b.latency_ms > 0 ? b.latency_ms : 99999
      return pingA - pingB
    })
  } else if (sortMode === 'ping_desc') {
    filtered.sort((a, b) => (b.latency_ms || 0) - (a.latency_ms || 0))
  } else if (sortMode === 'name') {
    filtered.sort((a, b) => a.name.localeCompare(b.name))
  } else if (sortMode === 'country') {
    filtered.sort((a, b) => (a.country_code || a.name).localeCompare(b.country_code || b.name))
  }

  // Pinned nodes lead, whichever sort is active. Applied last so the chosen
  // sort order survives within each group — see lib/profilePin.ts.
  filtered.sort(pinnedFirst)

  return (
    <div className="page space-y-4">
      {/* Toast Notification */}
      {toastMessage && (
        <div className="fixed bottom-6 right-6 z-50 bg-card border border-primary/50 text-primary text-xs px-4 py-2.5 rounded-xl shadow-2xl animate-fade-in flex items-center gap-2">
          <Sparkles size={14} className="text-primary shrink-0" />
          <span>{toastMessage}</span>
        </div>
      )}

      {/* Header & Main Actions */}
      <div className="page-header sticky top-0 z-30 flex items-center justify-between flex-wrap gap-3 bg-card/85 dark:bg-card backdrop-blur-xl py-3 -mx-2 px-4 rounded-2xl border border-border/80 dark:border-white/5 shadow-sm dark:shadow-lg dark:shadow-black/20">
        <div>
          <h1 className="page-title">{t('v2ray_title')}</h1>
          <p className="page-subtitle">{t('v2ray_subtitle')}</p>
        </div>

        <div className="flex items-center gap-2 flex-wrap">
          <Button variant="secondary" size="sm" className="flex items-center gap-1.5"
            onClick={handleImportFromClipboard}
            title={t('import_from_clipboard')}
          >
            <ClipboardList size={14} className="text-success" />
            <span>{t('import_from_clipboard')}</span>
          </Button>

          <Button variant="secondary" size="sm" className="flex items-center gap-1.5"
            onClick={() => setShowScreenQRModal(true)}
            title="Scan QR Code from Screen or Image"
          >
            <Scan size={14} className="text-primary" />
            <span>{t('screen_qr_title') || 'Scan QR'}</span>
          </Button>

          <Button variant="secondary" size="sm" className="flex items-center gap-1.5"
            onClick={() => setShowConverterModal(true)}
            title="Universal Config Converter"
          >
            <ArrowRightLeft size={14} className="text-warning" />
            <span>{t('config_converter_title') || 'Converter'}</span>
          </Button>

          <Button variant="secondary" size="sm"
            onClick={() => setShowImportModal(true)}
          >
            <Link size={14} className="text-primary" />
            <span>{t('import_uri_sub')}</span>
          </Button>

          <Button size="sm"
            id="add-node-btn"
            onClick={() => setShowAddModal(true)}
          >
            <Plus size={14} />
            <span>{t('add_v2ray_node')}</span>
          </Button>
        </div>
      </div>

      {/* Dynamic Group Selector & Automation Bar */}
      <div className="bg-card border border-border/80 dark:border-white/5 rounded-2xl p-3 space-y-2.5 shadow-sm">
        <div className="flex items-center justify-between flex-wrap gap-2">
          <div className="flex items-center gap-1.5 overflow-x-auto pb-1 text-xs no-scrollbar flex-1">
            <button
              onClick={() => setFilterGroup('all')}
              className={`px-3 py-1.5 rounded-xl font-bold whitespace-nowrap transition-all ${
                filterGroup === 'all'
                  ? 'bg-primary text-primary-foreground shadow-sm'
                  : 'bg-muted hover:bg-muted/90 text-foreground dark:bg-muted dark:text-muted-foreground dark:hover:text-foreground border border-border dark:border-white/5'
              }`}
            >
              {t('all_nodes')} ({profiles.length})
            </button>

            {availableGroups.map(g => {
              const count = profiles.filter(p => (p.subscription_name || p.group) === g).length
              const isSelected = filterGroup === g
              return (
                <button
                  key={g}
                  onClick={() => setFilterGroup(g)}
                  className={`px-3 py-1.5 rounded-xl font-bold whitespace-nowrap transition-all flex items-center gap-1.5 ${
                    isSelected
                      ? 'bg-primary text-primary-foreground shadow-sm'
                      : 'bg-muted hover:bg-muted/90 text-foreground dark:bg-muted dark:text-muted-foreground dark:hover:text-foreground border border-border dark:border-white/5'
                  }`}
                >
                  <span>📁</span>
                  <span className="truncate max-w-[140px]">{g}</span>
                  <span className="opacity-70 font-mono text-[11px]">({count})</span>
                </button>
              )
            })}
          </div>

          {/* Group Actions */}
          {filterGroup !== 'all' && (
            <div className="flex items-center gap-1.5 shrink-0">
              <Button variant="secondary" size="sm" className="text-xs py-1 px-2.5 flex items-center gap-1"
                type="button"
                onClick={handleUpdateActiveGroup}
                disabled={isUpdatingSubs}
                title={t('update_subs')}
              >
                <FolderSync size={13} className={`text-primary ${isUpdatingSubs ? 'animate-spin' : ''}`} />
                <span>{t('update_subs')}</span>
              </Button>

              <Button variant="secondary" size="sm" className="text-xs py-1 px-2.5 flex items-center gap-1 text-destructive hover:bg-destructive/10"
                type="button"
                onClick={handleDeleteActiveGroup}
                title="Delete Group"
              >
                <FolderMinus size={13} />
                <span>{t('delete_all')}</span>
              </Button>
            </div>
          )}
        </div>

        {/* Subscription Live Quota & Expiry Banner */}
        {subUserInfo && (
          <div className="bg-primary/10 p-3.5 border border-primary/20 dark:border-primary/30 rounded-2xl space-y-2 animate-in fade-in duration-300">
            <div className="flex items-center justify-between text-xs flex-wrap gap-2">
              <div className="flex items-center gap-2">
                <span className="font-bold text-primary flex items-center gap-1.5">
                  <Activity size={13} className="text-primary" />
                  <span>{t('sub_data_usage')}</span>
                </span>
                <span className="font-mono font-bold text-foreground">
                  {(subUserInfo.used_bytes / (1024 * 1024 * 1024)).toFixed(1)} GB / {(subUserInfo.total_bytes / (1024 * 1024 * 1024)).toFixed(0)} GB
                </span>
                <span className="text-[11px] text-primary bg-primary/20 px-2 py-0.5 rounded-full font-bold">
                  {subUserInfo.percent_used}%
                </span>
              </div>

              {subUserInfo.expire_date > 0 && (
                <div className="text-muted-foreground font-mono text-[11px] flex items-center gap-1">
                  <span>{t('sub_expires')}</span>
                  <span className="text-warning font-bold">
                    {new Date(subUserInfo.expire_date * 1000).toLocaleDateString()}
                  </span>
                </div>
              )}
            </div>

            {/* Quota Progress bar */}
            <div className="w-full h-2 bg-muted dark:bg-background rounded-full overflow-hidden border border-border dark:border-white/5">
              <div
                className={`h-full rounded-full transition-all duration-500 ${
                  subUserInfo.percent_used > 85
                    ? 'bg-destructive shadow-sm'
                    : subUserInfo.percent_used > 60
                    ? 'bg-warning'
                    : 'bg-primary'
                }`}
                style={{ width: `${Math.min(100, subUserInfo.percent_used)}%` }}
              />
            </div>
          </div>
        )}

        {/* Search, Smart Sorting, and Automation Toolbar */}
        <div className="flex items-center justify-between flex-wrap gap-2 pt-2 border-t border-border/80 dark:border-white/5">
          <div className="flex items-center gap-2 flex-wrap flex-1">
            <div className="search-bar" style={{ maxWidth: '240px' }}>
              <Search size={14} className="text-muted-foreground" />
              <input
                type="text"
                placeholder={t('search_nodes')}
                value={search}
                onChange={e => setSearch(e.target.value)}
                className="text-xs"
              />
            </div>

            {/* Smart Sort Dropdown */}
            <div className="flex items-center gap-1.5 text-xs text-foreground dark:text-muted-foreground bg-muted px-2.5 py-1.5 rounded-xl border border-border dark:border-white/5">
              <ArrowUpDown size={12} className="text-primary" />
              <select
                value={sortMode}
                onChange={e => setSortMode(e.target.value as SortMode)}
                className="bg-transparent text-xs font-semibold text-foreground outline-none cursor-pointer"
              >
                <option value="default" className="bg-card text-foreground">{t('sort_default')}</option>
                <option value="ping_asc" className="bg-card text-foreground">⚡ {t('sort_ping_asc')}</option>
                <option value="ping_desc" className="bg-card text-foreground">{t('sort_ping_desc')}</option>
                <option value="country" className="bg-card text-foreground">🌍 {t('sort_country')}</option>
                <option value="name" className="bg-card text-foreground">🔤 {t('sort_name')}</option>
              </select>
            </div>
          </div>

          <div className="flex items-center gap-1.5 flex-wrap">
            {/* Real Delay HTTP URL Test */}
            <Button variant="secondary" size="sm" className="text-xs text-primary border-primary/30 hover:bg-primary/10"
              onClick={handleRealDelayAll}
              disabled={isTestingRealDelay}
              title={t('url_test_all')}
            >
              <Activity size={13} className={isTestingRealDelay ? 'animate-spin text-primary' : 'text-primary'} />
              <span>{isTestingRealDelay ? t('testing_url') : t('url_test_all')}</span>
            </Button>

            {/* TCP Ping All */}
            <Button variant="secondary" size="sm" className="text-xs"
              onClick={handlePingAll}
              disabled={isPinging}
              title={t('ping_all')}
            >
              <Radio size={13} className="text-success" />
              <span>{isPinging ? t('pinging') : t('ping_all')}</span>
            </Button>

            {/* Remove Timeout / Dead Nodes */}
            <Button variant="secondary" size="sm" className="text-xs"
              onClick={handleRemoveDead}
              title={t('remove_dead')}
            >
              <Zap size={13} className="text-warning" />
              <span>{t('remove_dead')}</span>
            </Button>

            {/* Deduplicate Nodes */}
            <Button variant="secondary" size="sm" className="text-xs"
              onClick={handleDeduplicate}
              title={t('remove_duplicates')}
            >
              <Layers size={13} className="text-primary" />
              <span>{t('remove_duplicates')}</span>
            </Button>

            {/* Export All to Clipboard */}
            <Button variant="secondary" size="sm" className="text-xs"
              onClick={handleExportAll}
              title={t('copy_for_mobile')}
            >
              <Download size={13} className="text-primary" />
              <span>{t('copy_for_mobile')}</span>
            </Button>

            {/* Update Subscriptions */}
            <Button variant="secondary" size="sm" className="text-xs p-2"
              onClick={handleUpdateAllSubscriptions}
              disabled={isUpdatingSubs}
              title={t('update_subs')}
            >
              <RefreshCw size={13} className={`text-primary ${isUpdatingSubs ? 'animate-spin' : ''}`} />
            </Button>

            {profiles.length > 0 && (
              <Button variant="secondary" size="sm" className="text-destructive hover:bg-destructive/10 p-2"
                onClick={handleDeleteAll}
                title={t('delete_all')}
              >
                <Trash2 size={13} />
              </Button>
            )}
          </div>
        </div>
      </div>

      <PingProgressBar progress={pingProgress} onCancel={handleCancelPingAll} />

      {/* Node Cards Grid */}
      {filtered.length === 0 ? (
        <Card className="text-center p-12 space-y-3">
          <Globe size={48} className="mx-auto text-muted-foreground mb-2 opacity-40" />
          <h3 className="font-bold text-sm text-foreground">
            {t('no_v2ray_nodes')}
          </h3>
          <Button size="sm" className="mx-auto" onClick={() => setShowImportModal(true)}>
            <Plus size={14} />
            <span>{t('import_uri_sub')}</span>
          </Button>
        </Card>
      ) : (
        <div className="grid grid-cols-1 md:grid-cols-2 2xl:grid-cols-3 gap-3.5">
          {filtered.map(p => {
            const isConnected = status.state === 'connected' && status.profile_id === p.id
            const isConnecting = status.state === 'connecting' && status.profile_id === p.id
            const protoInfo = getDisplayProtocol(p)

            return (
              <ProfileCard
                key={p.id}
                name={cleanServerName(p.name)}
                icon={<CountryFlag codeOrName={p.country_code || p.name} size="md" />}
                protocolLabel={protoInfo.label}
                protocolClassName={protoInfo.color}
                className={
                  isConnected
                    ? 'border-primary shadow-sm bg-info/80 dark:bg-info/20 ring-1 ring-border'
                    : 'bg-card border-border/80 dark:border-white/5 hover:border-primary/50'
                }
                metaSlot={
                  <>
                    <span className="truncate flex-1">
                      {p.server ? `${p.server}:${p.port}` : 'Direct Tunnel'}
                    </span>
                    {(p.subscription_name || p.group) && (
                      <span className="text-[10px] px-1.5 py-0.5 rounded bg-muted text-muted-foreground truncate max-w-[120px] shrink-0 border border-border dark:border-transparent">
                        📁 {p.subscription_name || p.group}
                      </span>
                    )}
                  </>
                }
                footnoteSlot={
                  <button
                    type="button"
                    className="px-2 py-1 rounded-lg bg-muted hover:bg-muted/90 text-[11px] text-foreground dark:text-muted-foreground transition flex items-center gap-1 font-mono border border-border"
                    onClick={e => handlePingSingle(p.id, e)}
                    title={t('ping_test')}
                  >
                    <Activity
                      size={11}
                      className={
                        p.latency_ms && p.latency_ms > 0
                          ? 'text-success'
                          : p.latency_ms === -1 || (p.latency_ms && p.latency_ms < 0)
                          ? 'text-destructive'
                          : 'text-muted-foreground'
                      }
                    />
                    {p.latency_ms && p.latency_ms > 0 ? (
                      <span
                        className={
                          p.latency_ms < 150
                            ? 'text-success font-bold'
                            : p.latency_ms < 300
                            ? 'text-warning font-bold'
                            : 'text-destructive font-bold'
                        }
                      >
                        {p.latency_ms}ms
                      </span>
                    ) : p.latency_ms === -1 || (p.latency_ms && p.latency_ms < 0) ? (
                      <span className="text-destructive font-bold flex items-center gap-0.5">
                        <AlertCircle size={10} />
                        <span>-1 ms</span>
                      </span>
                    ) : (
                      <span className="text-muted-foreground font-mono">Ping</span>
                    )}
                  </button>
                }
                actionsSlot={
                  <>
                    <button
                      type="button"
                      className="p-1.5 rounded-md text-muted-foreground hover:text-primary hover:bg-muted dark:hover:text-primary dark:hover:bg-muted transition"
                      title={t('qr_code_modal_title')}
                      onClick={e => {
                        e.stopPropagation()
                        setSharingProfile(p)
                      }}
                    >
                      <QrCode size={13} />
                    </button>

                    <button
                      type="button"
                      className="p-1.5 rounded-md text-muted-foreground hover:text-primary hover:bg-muted dark:hover:text-primary dark:hover:bg-muted transition"
                      title={t('copy_config_link')}
                      onClick={e => handleCopyURI(p, e)}
                    >
                      {copiedId === p.id ? (
                        <Check size={13} className="text-success" />
                      ) : (
                        <Copy size={13} />
                      )}
                    </button>

                    <button
                      type="button"
                      className="p-1.5 rounded-md text-muted-foreground hover:text-primary hover:bg-muted dark:hover:text-primary dark:hover:bg-muted transition"
                      title={t('sni_optimize_action')}
                      onClick={e => handleOptimizeSNI(p.id, e)}
                      disabled={optimizingSniId === p.id}
                    >
                      <Wand2 size={13} className={optimizingSniId === p.id ? 'animate-pulse' : ''} />
                    </button>
                  </>
                }
                state={isConnected ? 'connected' : isConnecting ? 'connecting' : 'idle'}
                connectClassName="bg-primary hover:bg-primary/90 text-primary-foreground shadow-sm"
                onConnect={() => handleConnect(p.id)}
                onDisconnect={handleDisconnect}
                onEdit={() => setEditingProfile(p)}
                onDelete={() => handleDelete(p.id)}
                pinned={!!p.favorite}
                onTogglePin={() => toggleProfilePin(p, setProfiles)}
                pinLabels={{ pin: t('adv_pin'), unpin: t('adv_unpin') }}
                labels={{
                  connect: t('connect'),
                  disconnect: t('disconnect'),
                  edit: t('edit'),
                  delete: t('delete'),
                }}
              />
            )
          })}
        </div>
      )}

      {/* Rename Group Modal */}
      {renamingGroup && (
        <RenameGroupModal
          groupName={renamingGroup}
          onClose={() => setRenamingGroup(null)}
          onRenamed={async newName => {
            await api.renameGroup(renamingGroup, newName)
            setFilterGroup(newName)
            setRenamingGroup(null)
            loadProfiles()
            showToast(t('saved_successfully'))
          }}
        />
      )}

      {/* Profile Edit Modal */}
      <EditProfileModal
        isOpen={!!editingProfile}
        profile={editingProfile}
        onClose={() => setEditingProfile(null)}
        onSaved={() => loadProfiles()}
        onDeleted={id => setProfiles(prev => prev.filter(p => p.id !== id))}
      />

      {/* Add Single URI Modal */}
      {showAddModal && (
        <AddURIModal
          isOpen={showAddModal}
          onClose={() => setShowAddModal(false)}
          onAdded={() => {
            setShowAddModal(false)
            loadProfiles()
            showToast(t('saved_successfully'))
          }}
        />
      )}

      {/* Import Subscription Modal */}
      {showImportModal && (
        <ImportSubModal
          isOpen={showImportModal}
          onClose={() => setShowImportModal(false)}
          onImported={() => {
            setShowImportModal(false)
            loadProfiles()
            showToast(t('saved_successfully'))
          }}
        />
      )}

      {/* QR Code Modal */}
      {sharingProfile && (
        <ModalShell
          isOpen={true}
          onClose={() => setSharingProfile(null)}
          maxWidth="max-w-sm"
          bodyClassName="p-6 text-center space-y-4"
        >
            <h3 className="font-bold text-foreground text-sm truncate">{sharingProfile.name}</h3>
            {qrImageUrl && (
              <img
                src={qrImageUrl}
                alt="QR Code"
                className="bg-card p-3 rounded-xl mx-auto inline-block shadow-lg w-[220px] h-[220px]"
              />
            )}
            <Button variant="secondary" className="w-full text-xs" onClick={() => setSharingProfile(null)}>
              {t('close') || 'Close'}
            </Button>
        </ModalShell>
      )}

      {/* Screen QR Code Scanner Modal */}
      <ScreenQRScannerModal
        isOpen={showScreenQRModal}
        onClose={() => setShowScreenQRModal(false)}
        onProfileImported={loadProfiles}
      />

      {/* Universal Config Converter Modal */}
      <ConfigConverterModal
        isOpen={showConverterModal}
        onClose={() => setShowConverterModal(false)}
      />
    </div>
  )
}

function RenameGroupModal({
  groupName,
  onClose,
  onRenamed,
}: {
  groupName: string
  onClose: () => void
  onRenamed: (newName: string) => void
}) {
  const [name, setName] = useState(groupName)

  return (
    <ModalShell
      isOpen={true}
      onClose={onClose}
      maxWidth="max-w-md"
      bodyClassName="p-6 space-y-4"
    >
        <div className="flex items-center justify-between border-b border-border pb-3">
          <h3 className="font-bold text-foreground text-sm">{t('rename_group_title')}</h3>
          <button onClick={onClose} className="text-muted-foreground hover:text-foreground">
            <X className="w-5 h-5" />
          </button>
        </div>

        <form
          onSubmit={e => {
            e.preventDefault()
            if (name.trim()) onRenamed(name.trim())
          }}
          className="space-y-4"
        >
          <div>
            <label className="block text-xs font-semibold text-muted-foreground mb-1.5">{t('new_group_name_label')}</label>
            <input
              type="text"
              className="form-control text-xs"
              value={name}
              onChange={e => setName(e.target.value)}
              required
              autoFocus
            />
          </div>

          <div className="flex items-center justify-end gap-2 pt-2 border-t border-border">
            <Button variant="secondary" size="sm" type="button" onClick={onClose}>
              {t('cancel')}
            </Button>
            <Button size="sm" type="submit">
              {t('save_name_btn')}
            </Button>
          </div>
        </form>
    </ModalShell>
  )
}

function AddURIModal({
  isOpen,
  onClose,
  onAdded,
}: {
  isOpen: boolean
  onClose: () => void
  onAdded: () => void
}) {
  const [uri, setUri] = useState('')
  const [isSubmitting, setIsSubmitting] = useState(false)


  if (!isOpen) return null

  const handleSubmit = async (e: React.FormEvent) => {
    e.preventDefault()
    if (!uri.trim()) return
    setIsSubmitting(true)
    try {
      await api.importURI(uri.trim())
      onAdded()
    } catch (err) {
      console.error('Import URI error:', err)
      alert(t('alert_parse_uri_failed'))
    } finally {
      setIsSubmitting(false)
    }
  }

  return (
    <ModalShell
      isOpen={isOpen}
      onClose={onClose}
      maxWidth="max-w-lg"
      bodyClassName="p-6 space-y-4"
    >
        <div className="flex items-center justify-between border-b border-border pb-3">
          <h3 className="font-bold text-foreground text-sm">{t('add_v2ray_node')}</h3>
          <button onClick={onClose} className="text-muted-foreground hover:text-foreground">
            <X className="w-5 h-5" />
          </button>
        </div>

        <form onSubmit={handleSubmit} className="space-y-4">
          <div>
            <label className="block text-xs font-semibold text-muted-foreground mb-1.5">
              VLESS / VMess / Trojan / Shadowsocks / Hysteria 2 URI
            </label>
            <textarea
              className="form-control font-mono text-xs h-32"
              placeholder="vless://... or vmess://... or hysteria2://..."
              value={uri}
              onChange={e => setUri(e.target.value)}
              required
            />
          </div>

          <div className="flex items-center justify-end gap-2 pt-2 border-t border-border">
            <Button variant="secondary" size="sm" type="button" onClick={onClose}>
              {t('close') || 'Cancel'}
            </Button>
            <Button size="sm" type="submit" disabled={isSubmitting}>
              {isSubmitting ? '...' : t('add_v2ray_node')}
            </Button>
          </div>
        </form>
    </ModalShell>
  )
}

function ImportSubModal({
  isOpen,
  onClose,
  onImported,
}: {
  isOpen: boolean
  onClose: () => void
  onImported: () => void
}) {
  const [url, setUrl] = useState('')
  const [group, setGroup] = useState('')
  const [isSubmitting, setIsSubmitting] = useState(false)
  // Raw / base64 config pasting (as opposed to a subscription URL)
  const [mode, setMode] = useState<'url' | 'raw'>('url')
  const [rawName, setRawName] = useState('')
  const [rawContent, setRawContent] = useState('')


  if (!isOpen) return null

  const handleSubmit = async (e: React.FormEvent) => {
    e.preventDefault()
    setIsSubmitting(true)
    try {
      if (mode === 'url') {
        if (!url.trim()) return
        await api.importSubscription(url.trim(), group.trim())
      } else {
        if (!rawContent.trim()) return
        // Detect the protocol from the URI scheme so the backend parser is
        // picked correctly; fall back to auto for base64 bundles/JSON.
        await api.importProfile(
          rawName.trim() || t('import_raw_default_name'),
          detectProtocolFromContent(rawContent),
          rawContent.trim(),
          group.trim(),
          'v2ray'
        )
      }
      onImported()
    } catch (err) {
      console.error('Import error:', err)
      alert(t('alert_import_sub_failed'))
    } finally {
      setIsSubmitting(false)
    }
  }

  return (
    <ModalShell
      isOpen={isOpen}
      onClose={onClose}
      maxWidth="max-w-lg"
      bodyClassName="p-6 space-y-4"
    >
        <div className="flex items-center justify-between border-b border-border pb-3">
          <h3 className="font-bold text-foreground text-sm">{t('import_uri_sub')}</h3>
          <button onClick={onClose} className="text-muted-foreground hover:text-foreground">
            <X className="w-5 h-5" />
          </button>
        </div>

        {/* Source mode switch */}
        <div className="flex gap-2 rounded-xl bg-background p-1 border border-border">
          <button
            type="button"
            onClick={() => setMode('url')}
            className={`flex-1 rounded-lg px-3 py-1.5 text-xs font-semibold transition ${
              mode === 'url' ? 'bg-primary text-primary-foreground' : 'text-muted-foreground hover:text-foreground'
            }`}
          >
            {t('import_mode_url')}
          </button>
          <button
            type="button"
            onClick={() => setMode('raw')}
            className={`flex-1 rounded-lg px-3 py-1.5 text-xs font-semibold transition ${
              mode === 'raw' ? 'bg-primary text-primary-foreground' : 'text-muted-foreground hover:text-foreground'
            }`}
          >
            {t('import_mode_raw')}
          </button>
        </div>

        <form onSubmit={handleSubmit} className="space-y-4">
          {mode === 'url' ? (
            <div>
              <label className="block text-xs font-semibold text-muted-foreground mb-1.5">
                Subscription HTTP/HTTPS URL
              </label>
              <input
                type="url"
                className="form-control font-mono text-xs"
                placeholder="https://example.com/sub/v2ray"
                value={url}
                onChange={e => setUrl(e.target.value)}
                required
              />
            </div>
          ) : (
            <>
              <div>
                <label className="block text-xs font-semibold text-muted-foreground mb-1.5">
                  {t('import_raw_name_label')}
                </label>
                <input
                  type="text"
                  className="form-control text-xs"
                  placeholder="My Server"
                  value={rawName}
                  onChange={e => setRawName(e.target.value)}
                />
              </div>
              <div>
                <label className="block text-xs font-semibold text-muted-foreground mb-1.5">
                  {t('import_raw_content_label')}
                </label>
                <textarea
                  className="form-control font-mono text-[11px] h-28 resize-none"
                  placeholder={'vless://...\nvmess://...\nss://...'}
                  value={rawContent}
                  onChange={e => setRawContent(e.target.value)}
                  required
                />
                <p className="mt-1 text-[10px] text-muted-foreground">{t('import_raw_content_hint')}</p>
              </div>
            </>
          )}

          <div>
            <label className="block text-xs font-semibold text-muted-foreground mb-1.5">
              Group / Subscription Name (Optional)
            </label>
            <input
              type="text"
              className="form-control text-xs"
              placeholder="VIP Servers"
              value={group}
              onChange={e => setGroup(e.target.value)}
            />
          </div>

          <div className="flex items-center justify-end gap-2 pt-2 border-t border-border">
            <Button variant="secondary" size="sm" type="button" onClick={onClose}>
              {t('close') || 'Cancel'}
            </Button>
            <Button size="sm" type="submit" disabled={isSubmitting}>
              {isSubmitting ? '...' : t('update_subs')}
            </Button>
          </div>
        </form>
    </ModalShell>
  )
}

/** Best-effort protocol sniffing for a pasted config blob. */
function detectProtocolFromContent(content: string): string {
  const c = content.trim().toLowerCase()
  if (c.startsWith('vless://')) return 'vless'
  if (c.startsWith('vmess://')) return 'vmess'
  if (c.startsWith('trojan://')) return 'trojan'
  if (c.startsWith('ss://')) return 'shadowsocks'
  if (c.startsWith('hysteria2://') || c.startsWith('hy2://')) return 'hysteria2'
  if (c.startsWith('tuic://')) return 'tuic'
  if (c.startsWith('{')) return 'singbox'
  return 'auto'
}
