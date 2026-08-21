import React, { useState, useEffect, useCallback } from 'react'
import {
  Plus,
  Trash2,
  Play,
  Square,
  CheckCircle2,
  Search,
  Lock,
  X,
  Cpu,
  RefreshCw,
  Zap,
  Globe,
  Settings as SettingsIcon,
  Download,
  AlertTriangle,
} from 'lucide-react'
import { api } from '../lib/api'
import { t } from '../lib/i18n'
import type { Profile, ConnectionStatus } from '../types'

interface Props {
  status: ConnectionStatus
  onStatusChange?: (s: ConnectionStatus) => void
}

interface AetherConfig {
  protocol: 'masque' | 'wireguard' | 'gool'
  scan_mode: 'turbo' | 'balanced' | 'thorough' | 'stealth' | 'ironclad'
  socks_port: number
  http_port: number
  upstream: string
  quick_reconnect: boolean
}

const FA_TRANSLATIONS: Record<string, string> = {
  aether_title: 'مدیریت هسته Aether',
  aether_subtitle: 'دور زدن فیلترینگ با پروتکل MASQUE و gool (Cloudflare nested tunnel)',
  add_profile: 'افزودن سرور Aether',
  profile_name: 'نام سرور',
  protocol: 'پروتکل ارتباطی',
  scan_mode: 'حالت اسکنر هوشمند (Scan Mode)',
  socks_port: 'پورت SOCKS5 محلی',
  http_port: 'پورت HTTP محلی',
  upstream: 'پروکسی بالادستی (اختیاری - Upstream)',
  quick_reconnect: 'اتصال مجدد سریع (Quick Reconnect)',
  core_status: 'وضعیت هسته Aether',
  core_installed: 'هسته نصب شده و آماده استفاده است.',
  core_missing: 'فایل باینری Aether یافت نشد. لطفا برای استفاده هسته را دانلود کنید.',
  download_core: 'دانلود و نصب هسته Aether',
  saving: 'در حال ذخیره‌سازی...',
  connecting: 'در حال اتصال...',
  connected: 'متصل',
  disconnected: 'قطع شده',
  no_profiles: 'هیچ سرور Aether ثبت نشده است. برای شروع یکی ایجاد کنید.',
}

const EN_TRANSLATIONS: Record<string, string> = {
  aether_title: 'Aether Engine Manager',
  aether_subtitle: 'Censorship circumvention using MASQUE and gool (Cloudflare nested tunnel)',
  add_profile: 'Add Aether Profile',
  profile_name: 'Profile Name',
  protocol: 'Transport Protocol',
  scan_mode: 'Scanner Mode',
  socks_port: 'Local SOCKS5 Port',
  http_port: 'Local HTTP Port',
  upstream: 'Upstream Proxy (Optional)',
  quick_reconnect: 'Quick Reconnect',
  core_status: 'Aether Core Status',
  core_installed: 'Aether core is installed and ready.',
  core_missing: 'Aether binary is missing. Please download to enable routing.',
  download_core: 'Download & Install Aether Core',
  saving: 'Saving...',
  connecting: 'Connecting...',
  connected: 'Connected',
  disconnected: 'Disconnected',
  no_profiles: 'No Aether profiles configured. Create one to get started.',
}

export default function AetherPage({ status, onStatusChange }: Props) {
  const [profiles, setProfiles] = useState<Profile[]>([])
  const [search, setSearch] = useState('')
  const [showAddModal, setShowAddModal] = useState(false)
  const [coreInstalled, setCoreInstalled] = useState(false)
  const [updatingCore, setUpdatingCore] = useState(false)

  // Form states
  const [name, setName] = useState('')
  const [aetherProtocol, setAetherProtocol] = useState<'masque' | 'wireguard' | 'gool'>('masque')
  const [scanMode, setScanMode] = useState<'turbo' | 'balanced' | 'thorough' | 'stealth' | 'ironclad'>('balanced')
  const [socksPort, setSocksPort] = useState(10808)
  const [httpPort, setHttpPort] = useState(10809)
  const [upstream, setUpstream] = useState('')
  const [quickReconnect, setQuickReconnect] = useState(false)

  const isFarsi = document.documentElement.lang === 'fa' || t('app_title') === 'نکس وی پی ان'
  const trans = (key: string) => {
    return isFarsi ? (FA_TRANSLATIONS[key] || key) : (EN_TRANSLATIONS[key] || key)
  }

  const loadProfiles = useCallback(async () => {
    try {
      const list = await api.getProfilesByCategory('aether')
      if (list && Array.isArray(list)) setProfiles(list)
    } catch (err) {
      console.error('Failed to load Aether profiles:', err)
    }
  }, [])

  const checkCoreStatus = useCallback(async () => {
    try {
      const b = await api.checkCoreBinaries()
      if (b && b.aether && b.aether.status === 'installed') {
        setCoreInstalled(true)
      } else {
        setCoreInstalled(false)
      }
    } catch (err) {
      console.error('Failed to check Aether core status:', err)
    }
  }, [])

  useEffect(() => {
    loadProfiles()
    checkCoreStatus()
  }, [loadProfiles, checkCoreStatus])

  const handleInstallCore = useCallback(async () => {
    setUpdatingCore(true)
    try {
      await api.updateCoreEngine('aether')
      await checkCoreStatus()
    } catch (err) {
      console.error('Failed to download Aether core:', err)
    } finally {
      setUpdatingCore(false)
    }
  }, [checkCoreStatus])

  const handleConnect = async (profileId: string) => {
    try {
      const targetP = profiles.find(p => p.id === profileId)
      if (onStatusChange) {
        onStatusChange({
          state: 'connecting',
          profile_id: profileId,
          profile_name: targetP?.name || '',
          protocol: 'aether',
          server: '127.0.0.1',
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

  const handleDelete = async (id: string) => {
    try {
      await api.deleteProfile(id)
      setProfiles(prev => prev.filter(p => p.id !== id))
    } catch (err) {
      console.error('Delete error:', err)
    }
  }

  const handleAddProfile = async (e: React.FormEvent) => {
    e.preventDefault()
    if (!name.trim()) return

    const conf: AetherConfig = {
      protocol: aetherProtocol,
      scan_mode: scanMode,
      socks_port: socksPort,
      http_port: httpPort,
      upstream: upstream.trim(),
      quick_reconnect: quickReconnect,
    }

    const newProfile: Omit<Profile, 'id' | 'created_at' | 'updated_at' | 'usage_count'> = {
      name: name.trim(),
      protocol: 'aether',
      category: 'aether',
      config_content: JSON.stringify(conf),
      server: '127.0.0.1',
      port: socksPort,
    }

    try {
      await api.addProfile(newProfile as Profile)
      await loadProfiles()
      setShowAddModal(false)
      // Reset form
      setName('')
      setAetherProtocol('masque')
      setScanMode('balanced')
      setSocksPort(10808)
      setHttpPort(10809)
      setUpstream('')
      setQuickReconnect(false)
    } catch (err) {
      console.error('Add profile error:', err)
    }
  }

  const filtered = profiles.filter(p =>
    p.name.toLowerCase().includes(search.toLowerCase())
  )

  return (
    <div className="flex-1 overflow-y-auto p-6 space-y-6">
      {/* Header */}
      <div className="flex flex-col md:flex-row justify-between items-start md:items-center gap-4 border-b border-white/5 pb-5">
        <div className="min-w-0 flex-1">
          <h1 className="text-2xl font-bold tracking-tight text-white flex items-center gap-2">
            <Cpu className="text-cyan-400 animate-pulse" size={24} />
            {trans('aether_title')}
          </h1>
          <p className="text-sm text-slate-400 mt-1">{trans('aether_subtitle')}</p>
        </div>
        <button
          onClick={() => setShowAddModal(true)}
          className="flex items-center gap-1.5 px-4 py-2 bg-gradient-to-r from-cyan-500 to-blue-500 hover:from-cyan-600 hover:to-blue-600 text-white rounded-lg shadow-md text-sm font-medium transition-all"
        >
          <Plus size={16} />
          {trans('add_profile')}
        </button>
      </div>

      {/* Core Status Banner */}
      <div className={`p-4 rounded-xl border flex flex-col sm:flex-row justify-between items-start sm:items-center gap-4 ${
        coreInstalled 
          ? 'bg-emerald-500/5 border-emerald-500/20 text-emerald-400' 
          : 'bg-amber-500/5 border-amber-500/20 text-amber-400'
      }`}>
        <div className="flex items-center gap-3">
          {coreInstalled ? (
            <CheckCircle2 size={24} className="text-emerald-400 shrink-0" />
          ) : (
            <AlertTriangle size={24} className="text-amber-400 shrink-0" />
          )}
          <div>
            <h3 className="font-semibold text-white text-sm">{trans('core_status')}</h3>
            <p className="text-xs text-slate-400 mt-0.5">
              {coreInstalled ? trans('core_installed') : trans('core_missing')}
            </p>
          </div>
        </div>
        {!coreInstalled && (
          <button
            onClick={handleInstallCore}
            disabled={updatingCore}
            className="flex items-center gap-1.5 px-4 py-2 bg-amber-500 hover:bg-amber-600 disabled:bg-slate-700 text-slate-950 disabled:text-slate-400 rounded-lg text-xs font-semibold transition-all shrink-0"
          >
            {updatingCore ? (
              <RefreshCw className="animate-spin" size={14} />
            ) : (
              <Download size={14} />
            )}
            {trans('download_core')}
          </button>
        )}
      </div>

      {/* Search Filter */}
      <div className="relative">
        <span className="absolute inset-y-0 start-0 flex items-center ps-3 pointer-events-none text-slate-500">
          <Search size={16} />
        </span>
        <input
          type="text"
          placeholder={t('search_server') || 'جستجو در سرورها...'}
          value={search}
          onChange={e => setSearch(e.target.value)}
          className="w-full pl-10 pr-4 py-2 bg-slate-900/50 border border-white/5 rounded-lg text-sm text-slate-200 placeholder-slate-500 focus:outline-none focus:border-cyan-500/50"
        />
      </div>

      {/* Profiles Grid */}
      {filtered.length === 0 ? (
        <div className="flex flex-col items-center justify-center p-12 bg-slate-950/20 border border-white/5 rounded-xl text-center">
          <Cpu className="text-slate-600 mb-3 animate-pulse" size={48} />
          <p className="text-sm text-slate-400">{trans('no_profiles')}</p>
        </div>
      ) : (
        <div className="grid grid-cols-1 md:grid-cols-2 gap-4">
          {filtered.map(p => {
            let conf: Partial<AetherConfig> = {}
            try {
              conf = JSON.parse(p.config_content)
            } catch {}

            const isConnectingThis = status.state === 'connecting' && status.profile_id === p.id
            const isConnectedThis = status.state === 'connected' && status.profile_id === p.id

            return (
              <div 
                key={p.id}
                className={`p-4 rounded-xl border transition-all ${
                  isConnectedThis 
                    ? 'bg-cyan-500/5 border-cyan-500/30' 
                    : 'bg-slate-900/40 border-white/5 hover:border-white/10'
                }`}
              >
                <div className="flex justify-between items-start gap-4">
                  <div className="min-w-0">
                    <h3 className="font-semibold text-white text-sm truncate">{p.name}</h3>
                    <div className="flex flex-wrap gap-2 mt-2">
                      <span className="px-2 py-0.5 bg-slate-800 text-slate-400 rounded-md text-[10px] uppercase font-semibold border border-white/5">
                        {conf.protocol || 'masque'}
                      </span>
                      <span className="px-2 py-0.5 bg-slate-800 text-slate-400 rounded-md text-[10px] uppercase font-semibold border border-white/5">
                        {conf.scan_mode || 'balanced'}
                      </span>
                      <span className="px-2 py-0.5 bg-slate-800 text-slate-400 rounded-md text-[10px] uppercase font-semibold border border-white/5">
                        SOCKS: {conf.socks_port || 10808}
                      </span>
                    </div>
                  </div>

                  <div className="flex items-center gap-2 shrink-0">
                    {isConnectedThis || isConnectingThis ? (
                      <button
                        onClick={handleDisconnect}
                        className="p-2 bg-red-500/10 hover:bg-red-500/20 text-red-400 rounded-lg transition-all"
                        title={t('disconnect') || 'قطع اتصال'}
                      >
                        <Square size={16} fill="currentColor" />
                      </button>
                    ) : (
                      <button
                        onClick={() => handleConnect(p.id)}
                        disabled={status.state === 'connecting'}
                        className="p-2 bg-cyan-500/10 hover:bg-cyan-500/20 disabled:bg-slate-800 text-cyan-400 disabled:text-slate-600 rounded-lg transition-all"
                        title={t('connect') || 'اتصال'}
                      >
                        <Play size={16} fill="currentColor" />
                      </button>
                    )}
                    <button
                      onClick={() => handleDelete(p.id)}
                      disabled={isConnectedThis || isConnectingThis}
                      className="p-2 bg-red-500/5 hover:bg-red-500/15 disabled:bg-transparent text-red-500/60 disabled:text-slate-700 rounded-lg transition-all border border-transparent hover:border-red-500/10"
                      title={t('delete') || 'حذف'}
                    >
                      <Trash2 size={16} />
                    </button>
                  </div>
                </div>
              </div>
            )
          })}
        </div>
      )}

      {/* Add Profile Modal */}
      {showAddModal && (
        <div className="fixed inset-0 bg-slate-950/80 backdrop-blur-sm z-50 flex items-center justify-center p-4">
          <div className="w-full max-w-lg bg-slate-900 border border-white/5 rounded-xl shadow-2xl overflow-hidden flex flex-col">
            <div className="flex justify-between items-center p-4 border-b border-white/5">
              <h3 className="font-semibold text-white text-base">{trans('add_profile')}</h3>
              <button 
                onClick={() => setShowAddModal(false)}
                className="text-slate-500 hover:text-white transition-all"
              >
                <X size={20} />
              </button>
            </div>

            <form onSubmit={handleAddProfile} className="p-5 space-y-4 overflow-y-auto max-h-[75vh]">
              {/* Profile Name */}
              <div className="space-y-1.5">
                <label className="text-xs font-semibold text-slate-400">{trans('profile_name')}</label>
                <input
                  type="text"
                  required
                  value={name}
                  onChange={e => setName(e.target.value)}
                  className="w-full px-3 py-2 bg-slate-950 border border-white/5 rounded-lg text-sm text-slate-200 focus:outline-none focus:border-cyan-500/50"
                  placeholder="E.g., Cloudflare MASQUE Bypass"
                />
              </div>

              {/* Protocol */}
              <div className="space-y-1.5">
                <label className="text-xs font-semibold text-slate-400">{trans('protocol')}</label>
                <select
                  value={aetherProtocol}
                  onChange={e => setAetherProtocol(e.target.value as any)}
                  className="w-full px-3 py-2 bg-slate-950 border border-white/5 rounded-lg text-sm text-slate-200 focus:outline-none focus:border-cyan-500/50"
                >
                  <option value="masque">MASQUE (HTTP/3 or HTTP/2 over UDP/TCP)</option>
                  <option value="wireguard">WireGuard (Cloudflare WARP)</option>
                  <option value="gool">Gool (WARP-in-WARP / Double tunnel)</option>
                </select>
              </div>

              {/* Scan Mode */}
              <div className="space-y-1.5">
                <label className="text-xs font-semibold text-slate-400">{trans('scan_mode')}</label>
                <select
                  value={scanMode}
                  onChange={e => setScanMode(e.target.value as any)}
                  className="w-full px-3 py-2 bg-slate-950 border border-white/5 rounded-lg text-sm text-slate-200 focus:outline-none focus:border-cyan-500/50"
                >
                  <option value="turbo">Turbo (Fastest scanner, heavy load)</option>
                  <option value="balanced">Balanced (Recommended scanner)</option>
                  <option value="thorough">Thorough (High precision scanner)</option>
                  <option value="stealth">Stealth (Low profile scanner)</option>
                  <option value="ironclad">Ironclad (Deep DPI circumvention scanner)</option>
                </select>
              </div>

              {/* Ports */}
              <div className="grid grid-cols-2 gap-4">
                <div className="space-y-1.5">
                  <label className="text-xs font-semibold text-slate-400">{trans('socks_port')}</label>
                  <input
                    type="number"
                    required
                    value={socksPort}
                    onChange={e => setSocksPort(Number(e.target.value))}
                    className="w-full px-3 py-2 bg-slate-950 border border-white/5 rounded-lg text-sm text-slate-200 focus:outline-none focus:border-cyan-500/50"
                  />
                </div>
                <div className="space-y-1.5">
                  <label className="text-xs font-semibold text-slate-400">{trans('http_port')}</label>
                  <input
                    type="number"
                    required
                    value={httpPort}
                    onChange={e => setHttpPort(Number(e.target.value))}
                    className="w-full px-3 py-2 bg-slate-950 border border-white/5 rounded-lg text-sm text-slate-200 focus:outline-none focus:border-cyan-500/50"
                  />
                </div>
              </div>

              {/* Upstream */}
              <div className="space-y-1.5">
                <label className="text-xs font-semibold text-slate-400">{trans('upstream')}</label>
                <input
                  type="text"
                  value={upstream}
                  onChange={e => setUpstream(e.target.value)}
                  className="w-full px-3 py-2 bg-slate-950 border border-white/5 rounded-lg text-sm text-slate-200 focus:outline-none focus:border-cyan-500/50"
                  placeholder="socks5://127.0.0.1:10808 or http://host:port"
                />
              </div>

              {/* Quick Reconnect */}
              <div className="flex items-center gap-2 pt-2">
                <input
                  type="checkbox"
                  id="quickReconnect"
                  checked={quickReconnect}
                  onChange={e => setQuickReconnect(e.target.checked)}
                  className="w-4 h-4 bg-slate-950 border border-white/5 rounded text-cyan-500 focus:ring-cyan-500/50 focus:ring-offset-0 focus:outline-none"
                />
                <label htmlFor="quickReconnect" className="text-xs font-semibold text-slate-400 cursor-pointer select-none">
                  {trans('quick_reconnect')}
                </label>
              </div>

              {/* Submit Buttons */}
              <div className="flex justify-end gap-3 pt-4 border-t border-white/5">
                <button
                  type="button"
                  onClick={() => setShowAddModal(false)}
                  className="px-4 py-2 bg-slate-800 hover:bg-slate-700 text-white rounded-lg text-xs font-semibold transition-all"
                >
                  {t('cancel') || 'لغو'}
                </button>
                <button
                  type="submit"
                  className="px-4 py-2 bg-cyan-500 hover:bg-cyan-600 text-slate-950 rounded-lg text-xs font-semibold transition-all"
                >
                  {t('save') || 'ذخیره'}
                </button>
              </div>
            </form>
          </div>
        </div>
      )}
    </div>
  )
}
