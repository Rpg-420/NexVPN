import React, { useState, useEffect, useCallback, useMemo } from 'react'
import {
  Plus,
  Trash2,
  Play,
  Square,
  Globe,
  Copy,
  Check,
  RefreshCw,
  QrCode,
  X,
  Radio,
  Search,
  Link,
  Shield,
  Activity,
  Edit3,
  FolderEdit,
  FolderMinus,
  FolderSync,
  Layers,
  Sparkles,
  Send,
  Download,
  ArrowUpDown,
  Zap,
  Scan,
  ArrowRightLeft,
} from 'lucide-react'
import { api } from '../lib/api'
import { t } from '../lib/i18n'
import type { Profile, ConnectionStatus } from '../types'
import CountryFlag from '../components/CountryFlag'
import {
  getProtocolEmoji,
  getProtocolName,
  getCountryFlag,
} from '../lib/utils'
import { generateQRCodeSVG } from '../lib/qrcode'
import EditProfileModal from '../components/EditProfileModal'
import ScreenQRScannerModal from '../components/ScreenQRScannerModal'
import ConfigConverterModal from '../components/ConfigConverterModal'

interface Props {
  status: ConnectionStatus
  onStatusChange?: (s: ConnectionStatus) => void
}

type FilterProtocol = 'all' | 'vless' | 'vmess' | 'trojan' | 'shadowsocks' | 'hysteria2' | 'wireguard' | 'warp' | 'tuic'
type SortMode = 'default' | 'ping_asc' | 'ping_desc' | 'country' | 'name'

export default function V2RayPage({ status, onStatusChange }: Props) {
  const [profiles, setProfiles] = useState<Profile[]>([])
  const [search, setSearch] = useState('')
  const [filterProtocol, setFilterProtocolState] = useState<FilterProtocol>(() => {
    return (localStorage.getItem('nexvpn_v2ray_filter_proto') as FilterProtocol) || 'all'
  })
  const setFilterProtocol = (proto: FilterProtocol) => {
    setFilterProtocolState(proto)
    localStorage.setItem('nexvpn_v2ray_filter_proto', proto)
  }

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
  const [isTestingRealDelay, setIsTestingRealDelay] = useState(false)
  const [isUpdatingSubs, setIsUpdatingSubs] = useState(false)
  const [subUserInfo, setSubUserInfo] = useState<any | null>(null)
  const [loadingSubInfo, setLoadingSubInfo] = useState(false)
  const [copiedId, setCopiedId] = useState<string | null>(null)
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
  }, [loadProfiles])

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
      setLoadingSubInfo(true)
      api.getSubscriptionInfo(activeSubUrl)
        .then(info => setSubUserInfo(info))
        .catch(() => setSubUserInfo(null))
        .finally(() => setLoadingSubInfo(false))
    } else {
      setSubUserInfo(null)
    }
  }, [activeSubUrl])

  const qrImageUrl = sharingProfile
    ? generateQRCodeSVG(sharingProfile.raw_uri || sharingProfile.config_content || sharingProfile.name)
    : ''

  const handlePingAll = async () => {
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
      const count = await api.updateAllSubscriptions()
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

  const handleOpenTelegram = async () => {
    try {
      await api.openTelegramProxy()
      showToast('Telegram proxy connected')
    } catch (err) {
      console.error('Open Telegram error:', err)
      window.open('tg://socks?server=127.0.0.1&port=10808', '_blank')
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

  const handleDelete = async (id: string, e: React.MouseEvent) => {
    e.stopPropagation()
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

  // Extract unique groups & subscription names
  const availableGroups = Array.from(
    new Set(
      profiles
        .map(p => p.subscription_name || p.group)
        .filter(Boolean)
    )
  ) as string[]

  let filtered = profiles.filter(p => {
    const matchSearch =
      !search ||
      p.name.toLowerCase().includes(search.toLowerCase()) ||
      p.server?.toLowerCase().includes(search.toLowerCase()) ||
      p.group?.toLowerCase().includes(search.toLowerCase())

    const matchProtocol = filterProtocol === 'all' || p.protocol.toLowerCase().includes(filterProtocol)
    const grp = p.subscription_name || p.group
    const matchGroup = filterGroup === 'all' || grp === filterGroup

    return matchSearch && matchProtocol && matchGroup
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

  return (
    <div className="page space-y-4">
      {/* Toast Notification */}
      {toastMessage && (
        <div className="fixed bottom-6 right-6 z-50 bg-slate-900 border border-cyan-500/50 text-cyan-300 text-xs px-4 py-2.5 rounded-xl shadow-2xl shadow-cyan-500/20 animate-fade-in flex items-center gap-2">
          <Sparkles size={14} className="text-cyan-400 shrink-0" />
          <span>{toastMessage}</span>
        </div>
      )}

      {/* Header & Main Actions */}
      <div className="page-header sticky top-0 z-30 flex items-center justify-between flex-wrap gap-3 bg-slate-900/90 backdrop-blur-xl py-3 -mx-2 px-4 rounded-2xl border border-white/5 shadow-lg shadow-black/20">
        <div>
          <h1 className="page-title">{t('v2ray_title')}</h1>
          <p className="page-subtitle">{t('v2ray_subtitle')}</p>
        </div>

        <div className="flex items-center gap-2 flex-wrap">
          <button
            className="btn btn-secondary btn-sm flex items-center gap-1.5"
            onClick={() => setShowScreenQRModal(true)}
            title="Scan QR Code from Screen or Image"
          >
            <Scan size={14} className="text-cyan-400" />
            <span>{t('screen_qr_title') || 'Scan QR'}</span>
          </button>

          <button
            className="btn btn-secondary btn-sm flex items-center gap-1.5"
            onClick={() => setShowConverterModal(true)}
            title="Universal Config Converter"
          >
            <ArrowRightLeft size={14} className="text-amber-400" />
            <span>{t('config_converter_title') || 'Converter'}</span>
          </button>

          <button
            className="btn btn-secondary btn-sm"
            onClick={() => setShowImportModal(true)}
          >
            <Link size={14} className="text-cyan-400" />
            <span>{t('import_uri_sub')}</span>
          </button>

          <button
            id="add-node-btn"
            className="btn btn-primary btn-sm"
            onClick={() => setShowAddModal(true)}
          >
            <Plus size={14} />
            <span>{t('add_v2ray_node')}</span>
          </button>
        </div>
      </div>

      {/* Dynamic Group Selector & Automation Bar */}
      <div className="bg-slate-900/70 border border-white/5 rounded-2xl p-3 space-y-2.5">
        <div className="flex items-center justify-between flex-wrap gap-2">
          <div className="flex items-center gap-1.5 overflow-x-auto pb-1 text-xs no-scrollbar flex-1">
            <button
              onClick={() => setFilterGroup('all')}
              className={`px-3 py-1.5 rounded-xl font-bold whitespace-nowrap transition-all ${
                filterGroup === 'all'
                  ? 'bg-purple-600 text-white shadow-md shadow-purple-600/20'
                  : 'bg-slate-800/80 text-slate-400 hover:text-white border border-white/5'
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
                      ? 'bg-purple-500 text-white shadow-md shadow-purple-500/20'
                      : 'bg-slate-800/80 text-slate-400 hover:text-white border border-white/5'
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
              <button
                type="button"
                className="btn btn-secondary btn-sm text-xs py-1 px-2.5 flex items-center gap-1"
                onClick={handleUpdateActiveGroup}
                disabled={isUpdatingSubs}
                title={t('update_subs')}
              >
                <FolderSync size={13} className={`text-cyan-400 ${isUpdatingSubs ? 'animate-spin' : ''}`} />
                <span>{t('update_subs')}</span>
              </button>

              <button
                type="button"
                className="btn btn-secondary btn-sm text-xs py-1 px-2.5 flex items-center gap-1 text-rose-400 hover:bg-rose-500/10"
                onClick={handleDeleteActiveGroup}
                title="Delete Group"
              >
                <FolderMinus size={13} />
                <span>{t('delete_all')}</span>
              </button>
            </div>
          )}
        </div>

        {/* Subscription Live Quota & Expiry Banner */}
        {subUserInfo && (
          <div className="p-3.5 bg-gradient-to-r from-purple-950/40 via-slate-900/60 to-indigo-950/40 border border-purple-500/30 rounded-2xl space-y-2 animate-in fade-in duration-300">
            <div className="flex items-center justify-between text-xs flex-wrap gap-2">
              <div className="flex items-center gap-2">
                <span className="font-bold text-purple-300 flex items-center gap-1.5">
                  <Activity size={13} className="text-purple-400" />
                  <span>حجم مصرفی اشتراک:</span>
                </span>
                <span className="font-mono font-bold text-white">
                  {(subUserInfo.used_bytes / (1024 * 1024 * 1024)).toFixed(1)} GB / {(subUserInfo.total_bytes / (1024 * 1024 * 1024)).toFixed(0)} GB
                </span>
                <span className="text-[11px] text-purple-300 bg-purple-500/20 px-2 py-0.5 rounded-full font-bold">
                  {subUserInfo.percent_used}%
                </span>
              </div>

              {subUserInfo.expire_date > 0 && (
                <div className="text-slate-400 font-mono text-[11px] flex items-center gap-1">
                  <span>انقضا:</span>
                  <span className="text-amber-300 font-bold">
                    {new Date(subUserInfo.expire_date * 1000).toLocaleDateString('fa-IR')}
                  </span>
                </div>
              )}
            </div>

            {/* Quota Progress bar */}
            <div className="w-full h-2 bg-slate-950 rounded-full overflow-hidden border border-white/5">
              <div
                className={`h-full rounded-full transition-all duration-500 ${
                  subUserInfo.percent_used > 85
                    ? 'bg-rose-500 shadow-md shadow-rose-500/50'
                    : subUserInfo.percent_used > 60
                    ? 'bg-amber-500'
                    : 'bg-gradient-to-r from-purple-500 to-indigo-400'
                }`}
                style={{ width: `${Math.min(100, subUserInfo.percent_used)}%` }}
              />
            </div>
          </div>
        )}

        {/* Search, Smart Sorting, and Automation Toolbar */}
        <div className="flex items-center justify-between flex-wrap gap-2 pt-2 border-t border-white/5">
          <div className="flex items-center gap-2 flex-wrap flex-1">
            <div className="search-bar" style={{ maxWidth: '240px' }}>
              <Search size={14} className="text-slate-400" />
              <input
                type="text"
                placeholder={t('search_nodes')}
                value={search}
                onChange={e => setSearch(e.target.value)}
                className="text-xs"
              />
            </div>

            {/* Smart Sort Dropdown */}
            <div className="flex items-center gap-1.5 text-xs text-slate-300 bg-slate-800/80 px-2.5 py-1.5 rounded-xl border border-white/5">
              <ArrowUpDown size={12} className="text-cyan-400" />
              <select
                value={sortMode}
                onChange={e => setSortMode(e.target.value as SortMode)}
                className="bg-transparent text-xs font-semibold text-slate-200 outline-none cursor-pointer"
              >
                <option value="default" className="bg-slate-900">{t('sort_default')}</option>
                <option value="ping_asc" className="bg-slate-900">⚡ {t('sort_ping_asc')}</option>
                <option value="ping_desc" className="bg-slate-900">{t('sort_ping_desc')}</option>
                <option value="country" className="bg-slate-900">🌍 {t('sort_country')}</option>
                <option value="name" className="bg-slate-900">🔤 {t('sort_name')}</option>
              </select>
            </div>
          </div>

          <div className="flex items-center gap-1.5 flex-wrap">
            {/* Real Delay HTTP URL Test */}
            <button
              className="btn btn-secondary btn-sm text-xs text-cyan-300 border-cyan-500/30 hover:bg-cyan-500/10"
              onClick={handleRealDelayAll}
              disabled={isTestingRealDelay}
              title="Real HTTP/204 URL Delay Test"
            >
              <Activity size={13} className={isTestingRealDelay ? 'animate-spin text-cyan-400' : 'text-cyan-400'} />
              <span>{isTestingRealDelay ? 'Testing...' : '⚡ URL Test'}</span>
            </button>

            {/* Ping All */}
            <button
              className="btn btn-secondary btn-sm text-xs"
              onClick={handlePingAll}
              disabled={isPinging}
              title={t('ping_all')}
            >
              <Radio size={13} className="text-emerald-400" />
              <span>{isPinging ? t('pinging') : t('ping_all')}</span>
            </button>

            {/* Remove Timeout / Dead Nodes */}
            <button
              className="btn btn-secondary btn-sm text-xs"
              onClick={handleRemoveDead}
              title={t('remove_dead')}
            >
              <Zap size={13} className="text-amber-400" />
              <span>{t('remove_dead')}</span>
            </button>

            {/* Deduplicate Nodes */}
            <button
              className="btn btn-secondary btn-sm text-xs"
              onClick={handleDeduplicate}
              title={t('remove_duplicates')}
            >
              <Layers size={13} className="text-purple-400" />
              <span>{t('remove_duplicates')}</span>
            </button>

            {/* Export All to Clipboard */}
            <button
              className="btn btn-secondary btn-sm text-xs"
              onClick={handleExportAll}
              title={t('copy_for_mobile')}
            >
              <Download size={13} className="text-blue-400" />
              <span>{t('copy_for_mobile')}</span>
            </button>

            {/* Update Subscriptions */}
            <button
              className="btn btn-secondary btn-sm text-xs p-2"
              onClick={handleUpdateAllSubscriptions}
              disabled={isUpdatingSubs}
              title={t('update_subs')}
            >
              <RefreshCw size={13} className={`text-cyan-400 ${isUpdatingSubs ? 'animate-spin' : ''}`} />
            </button>

            {profiles.length > 0 && (
              <button
                className="btn btn-secondary btn-sm text-rose-400 hover:bg-rose-500/10 p-2"
                onClick={handleDeleteAll}
                title={t('delete_all')}
              >
                <Trash2 size={13} />
              </button>
            )}
          </div>
        </div>
      </div>

      {/* Node Cards Grid */}
      {filtered.length === 0 ? (
        <div className="card text-center p-12 space-y-3">
          <Globe size={48} className="mx-auto text-slate-600 mb-2 opacity-40" />
          <h3 className="font-bold text-sm text-slate-200">
            {t('no_v2ray_nodes')}
          </h3>
          <button className="btn btn-primary btn-sm mx-auto" onClick={() => setShowImportModal(true)}>
            <Plus size={14} />
            <span>{t('import_uri_sub')}</span>
          </button>
        </div>
      ) : (
        <div className="grid grid-cols-1 md:grid-cols-2 2xl:grid-cols-3 gap-3.5">
          {filtered.map(p => {
            const isConnected = status.state === 'connected' && status.profile_id === p.id
            const isConnecting = status.state === 'connecting' && status.profile_id === p.id
            const flag = getCountryFlag(p.country_code || p.name)

            return (
              <div
                key={p.id}
                className={`card p-3.5 sm:p-4 transition-all rounded-2xl flex flex-col justify-between overflow-hidden ${
                  isConnected
                    ? 'border-cyan-500/80 shadow-lg shadow-cyan-500/20 bg-cyan-950/20 ring-1 ring-cyan-500/40'
                    : 'hover:border-slate-700 bg-slate-900/70 border-white/5'
                }`}
              >
                {/* Top Row: Flag, Name, Protocol Badge */}
                <div>
                  <div className="flex items-center justify-between gap-2 mb-1.5 min-w-0">
                    <div className="flex items-center gap-2 min-w-0 flex-1">
                      <CountryFlag codeOrName={p.country_code || p.name} size="md" />
                      <h4 className="font-bold text-xs text-slate-100 truncate min-w-0" title={p.name}>
                        {p.name}
                      </h4>
                    </div>

                    <span className="text-[9px] font-mono uppercase px-1.5 py-0.5 rounded bg-cyan-500/10 text-cyan-400 border border-cyan-500/20 shrink-0 font-bold">
                      {p.protocol}
                    </span>
                  </div>

                  {/* Middle Row: Server Address & Group Info */}
                  <div className="flex items-center justify-between text-[11px] text-slate-400 font-mono mb-2.5 gap-2">
                    <span className="truncate flex-1 text-slate-400">{p.server ? `${p.server}:${p.port}` : 'Direct Tunnel'}</span>
                    {(p.subscription_name || p.group) && (
                      <span className="text-[10px] px-1.5 py-0.5 rounded bg-slate-800/90 text-slate-300 truncate max-w-[120px] shrink-0">
                        📁 {p.subscription_name || p.group}
                      </span>
                    )}
                  </div>
                </div>

                {/* Bottom Row: Ping Badge, Action Icons, Connect Button */}
                <div className="flex items-center justify-between pt-2.5 border-t border-white/5 gap-1">
                  <button
                    type="button"
                    className="px-2 py-1 rounded-lg bg-slate-800/80 hover:bg-slate-800 text-[11px] text-slate-300 transition flex items-center gap-1 font-mono border border-slate-700/50 shrink-0"
                    onClick={e => handlePingSingle(p.id, e)}
                    title={t('ping_test')}
                  >
                    <Activity
                      size={11}
                      className={p.latency_ms && p.latency_ms > 0 ? 'text-emerald-400' : 'text-slate-500'}
                    />
                    <span
                      className={
                        p.latency_ms && p.latency_ms > 0
                          ? p.latency_ms < 150
                            ? 'text-emerald-400 font-bold'
                            : p.latency_ms < 300
                            ? 'text-amber-400 font-bold'
                            : 'text-rose-400 font-bold'
                          : 'text-slate-400'
                      }
                    >
                      {p.latency_ms && p.latency_ms > 0 ? `${p.latency_ms}ms` : 'Ping'}
                    </span>
                  </button>

                  <div className="flex items-center gap-0.5 shrink-0">
                    <button
                      type="button"
                      className="p-1.5 rounded-md text-slate-400 hover:text-cyan-300 hover:bg-slate-800 transition"
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
                      className="p-1.5 rounded-md text-slate-400 hover:text-cyan-300 hover:bg-slate-800 transition"
                      title={t('copy_config_link')}
                      onClick={e => handleCopyURI(p, e)}
                    >
                      {copiedId === p.id ? <Check size={13} className="text-emerald-400" /> : <Copy size={13} />}
                    </button>

                    <button
                      type="button"
                      className="p-1.5 rounded-md text-slate-400 hover:text-amber-300 hover:bg-slate-800 transition"
                      title={t('edit')}
                      onClick={e => {
                        e.stopPropagation()
                        setEditingProfile(p)
                      }}
                    >
                      <Edit3 size={13} />
                    </button>

                    <button
                      type="button"
                      className="p-1.5 rounded-md text-slate-400 hover:text-rose-400 hover:bg-rose-500/10 transition"
                      title={t('delete')}
                      onClick={e => handleDelete(p.id, e)}
                    >
                      <Trash2 size={13} />
                    </button>
                  </div>

                  <button
                    type="button"
                    className={`px-3 py-1 rounded-lg font-bold text-xs flex items-center gap-1 transition shrink-0 min-w-[65px] justify-center ${
                      isConnected
                        ? 'bg-rose-500 hover:bg-rose-600 text-white shadow-sm shadow-rose-500/20'
                        : isConnecting
                        ? 'bg-amber-500 text-slate-950 animate-pulse'
                        : 'bg-cyan-500 hover:bg-cyan-400 text-slate-950 shadow-sm shadow-cyan-500/20'
                    }`}
                    onClick={() => (isConnected ? handleDisconnect() : handleConnect(p.id))}
                  >
                    {isConnected ? (
                      <>
                        <Square size={10} className="fill-current" />
                        <span>{t('disconnect')}</span>
                      </>
                    ) : isConnecting ? (
                      <>
                        <RefreshCw size={10} className="animate-spin" />
                        <span>...</span>
                      </>
                    ) : (
                      <>
                        <Play size={10} className="fill-current" />
                        <span>{t('connect')}</span>
                      </>
                    )}
                  </button>
                </div>
              </div>
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
        <div className="fixed inset-0 z-50 flex items-center justify-center bg-black/80 backdrop-blur-md p-4 animate-fade-in">
          <div className="bg-slate-900 border border-slate-700/80 rounded-2xl w-full max-w-sm overflow-hidden shadow-2xl p-6 text-center space-y-4">
            <h3 className="font-bold text-slate-200 text-sm truncate">{sharingProfile.name}</h3>
            {qrImageUrl && (
              <div
                className="bg-white p-3 rounded-xl mx-auto inline-block shadow-lg"
                dangerouslySetInnerHTML={{ __html: qrImageUrl }}
              />
            )}
            <button className="btn btn-secondary w-full text-xs" onClick={() => setSharingProfile(null)}>
              {t('close') || 'Close'}
            </button>
          </div>
        </div>
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
    <div className="fixed inset-0 z-50 flex items-center justify-center bg-black/80 backdrop-blur-md p-4 animate-fade-in">
      <div className="bg-slate-900 border border-slate-700/80 rounded-2xl w-full max-w-md overflow-hidden shadow-2xl p-6 space-y-4">
        <div className="flex items-center justify-between border-b border-slate-800 pb-3">
          <h3 className="font-bold text-slate-200 text-sm">{t('rename_group_title')}</h3>
          <button onClick={onClose} className="text-slate-400 hover:text-slate-200">
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
            <label className="block text-xs font-semibold text-slate-300 mb-1.5">{t('new_group_name_label')}</label>
            <input
              type="text"
              className="form-control text-xs"
              value={name}
              onChange={e => setName(e.target.value)}
              required
              autoFocus
            />
          </div>

          <div className="flex items-center justify-end gap-2 pt-2 border-t border-slate-800">
            <button type="button" className="btn btn-secondary btn-sm" onClick={onClose}>
              {t('cancel')}
            </button>
            <button type="submit" className="btn btn-primary btn-sm">
              {t('save_name_btn')}
            </button>
          </div>
        </form>
      </div>
    </div>
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
      alert('Failed to parse URI')
    } finally {
      setIsSubmitting(false)
    }
  }

  return (
    <div className="fixed inset-0 z-50 flex items-center justify-center bg-black/80 backdrop-blur-md p-4 animate-fade-in">
      <div className="bg-slate-900 border border-slate-700/80 rounded-2xl w-full max-w-lg overflow-hidden shadow-2xl p-6 space-y-4">
        <div className="flex items-center justify-between border-b border-slate-800 pb-3">
          <h3 className="font-bold text-slate-200 text-sm">{t('add_v2ray_node')}</h3>
          <button onClick={onClose} className="text-slate-400 hover:text-slate-200">
            <X className="w-5 h-5" />
          </button>
        </div>

        <form onSubmit={handleSubmit} className="space-y-4">
          <div>
            <label className="block text-xs font-semibold text-slate-300 mb-1.5">
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

          <div className="flex items-center justify-end gap-2 pt-2 border-t border-slate-800">
            <button type="button" className="btn btn-secondary btn-sm" onClick={onClose}>
              {t('close') || 'Cancel'}
            </button>
            <button type="submit" className="btn btn-primary btn-sm" disabled={isSubmitting}>
              {isSubmitting ? '...' : t('add_v2ray_node')}
            </button>
          </div>
        </form>
      </div>
    </div>
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

  if (!isOpen) return null

  const handleSubmit = async (e: React.FormEvent) => {
    e.preventDefault()
    if (!url.trim()) return
    setIsSubmitting(true)
    try {
      await api.importSubscription(url.trim(), group.trim())
      onImported()
    } catch (err) {
      console.error('Import Subscription error:', err)
      alert('Failed to import subscription link')
    } finally {
      setIsSubmitting(false)
    }
  }

  return (
    <div className="fixed inset-0 z-50 flex items-center justify-center bg-black/80 backdrop-blur-md p-4 animate-fade-in">
      <div className="bg-slate-900 border border-slate-700/80 rounded-2xl w-full max-w-lg overflow-hidden shadow-2xl p-6 space-y-4">
        <div className="flex items-center justify-between border-b border-slate-800 pb-3">
          <h3 className="font-bold text-slate-200 text-sm">{t('import_uri_sub')}</h3>
          <button onClick={onClose} className="text-slate-400 hover:text-slate-200">
            <X className="w-5 h-5" />
          </button>
        </div>

        <form onSubmit={handleSubmit} className="space-y-4">
          <div>
            <label className="block text-xs font-semibold text-slate-300 mb-1.5">
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

          <div>
            <label className="block text-xs font-semibold text-slate-300 mb-1.5">
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

          <div className="flex items-center justify-end gap-2 pt-2 border-t border-slate-800">
            <button type="button" className="btn btn-secondary btn-sm" onClick={onClose}>
              {t('close') || 'Cancel'}
            </button>
            <button type="submit" className="btn btn-primary btn-sm" disabled={isSubmitting}>
              {isSubmitting ? '...' : t('update_subs')}
            </button>
          </div>
        </form>
      </div>
    </div>
  )
}
