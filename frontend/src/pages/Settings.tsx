import React, { useState, useEffect } from 'react'
import {
  Shield,
  Wifi,
  RefreshCw,
  Globe,
  Sparkles,
  Radio,
  Eraser,
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
  Sliders,
  Gamepad2,
  Volume2,
  Keyboard,
  Network,
  Settings as SettingsIcon,
} from 'lucide-react'
import { api } from '../lib/api'
import { t, setLanguage, getLanguage } from '../lib/i18n'
import { THEMES, getTheme, applyTheme, Theme } from '../lib/theme'
import { sound } from '../lib/sound'
import type { Settings, BinaryInfo } from '../types'

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
  subscription_auto_update_hours: 12,
  multi_hop_warp: false,
  show_mini_widget: false,
  v2ray_core: 'auto',
}

const popularApps = [
  { name: 'Telegram', exe: 'telegram.exe' },
  { name: 'Discord', exe: 'discord.exe' },
  { name: 'Chrome', exe: 'chrome.exe' },
  { name: 'Spotify', exe: 'spotify.exe' },
  { name: 'Steam', exe: 'steam.exe' },
  { name: 'IDM', exe: 'idman.exe' },
]

type SettingsTab = 'network' | 'antidpi' | 'apps' | 'appearance' | 'system'

interface SettingsPageProps {
  onSettingsSaved?: (s: Settings) => void
}

export default function SettingsPage({ onSettingsSaved }: SettingsPageProps = {}) {
  const [settings, setSettings] = useState<Settings>(defaultSettings)
  const [activeTab, setActiveTab] = useState<SettingsTab>('network')
  const [selectedTheme, setSelectedTheme] = useState<Theme>(getTheme())
  const [isSaving, setIsSaving] = useState(false)
  const [saved, setSaved] = useState(false)
  const [clearingProxy, setClearingProxy] = useState(false)
  const [proxyCleared, setProxyCleared] = useState(false)
  const [binaries, setBinaries] = useState<Record<string, BinaryInfo>>({})
  const [isInstallingCores, setIsInstallingCores] = useState(false)
  const [updatingCore, setUpdatingCore] = useState<string | null>(null)
  const [updatingGeo, setUpdatingGeo] = useState(false)
  const [geoUpdated, setGeoUpdated] = useState(false)
  const [newAppInput, setNewAppInput] = useState('')

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
  }, [])

  const handleUpdateCore = async (key: string) => {
    setUpdatingCore(key)
    try {
      await api.updateCoreEngine(key)
      const updated = await api.checkCoreBinaries()
      setBinaries(updated)
    } catch (err) {
      console.error(`Update ${key} error:`, err)
    } finally {
      setUpdatingCore(null)
    }
  }

  const handleUpdateGeoData = async () => {
    setUpdatingGeo(true)
    try {
      await api.updateGeoData()
      setGeoUpdated(true)
      setTimeout(() => setGeoUpdated(false), 3000)
    } catch (err) {
      console.error('Update geo data error:', err)
    } finally {
      setUpdatingGeo(false)
    }
  }

  const update = <K extends keyof Settings>(key: K, value: Settings[K]) => {
    if (!settings) return
    const updated = { ...settings, [key]: value }
    setSettings(updated)

    if (key === 'language') {
      setLanguage(value as any)
    }

    // Auto-persist to storage so settings never revert when navigating away
    api.saveSettings(updated).then(() => {
      if (onSettingsSaved) onSettingsSaved(updated)
    }).catch(err => console.error('Auto save settings error:', err))
  }

  const handleThemeSelect = (themeId: Theme) => {
    setSelectedTheme(themeId)
    applyTheme(themeId)
    update('theme', themeId)
  }

  const handleSave = async () => {
    if (!settings) return
    setIsSaving(true)
    try {
      await api.saveSettings(settings)
      if (onSettingsSaved) {
        onSettingsSaved(settings)
      }
      setSaved(true)
      setTimeout(() => setSaved(false), 2000)
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
      setTimeout(() => setProxyCleared(false), 2500)
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
        <button
          id="save-settings-btn"
          className={`btn ${saved ? 'btn-secondary' : 'btn-primary'}`}
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
        </button>
      </div>

      {/* Modern Categorized Tab Navigation */}
      <div className="flex items-center gap-2 overflow-x-auto pb-1 text-xs no-scrollbar">
        <button
          type="button"
          onClick={() => setActiveTab('network')}
          className={`flex items-center gap-2 px-3.5 py-2 rounded-xl font-bold whitespace-nowrap transition-all ${
            activeTab === 'network'
              ? 'bg-accent text-white shadow-md shadow-accent/20'
              : 'bg-slate-900/80 text-slate-400 hover:text-white border border-white/5'
          }`}
        >
          <Network size={14} />
          <span>{t('sec_routing_cores')}</span>
        </button>

        <button
          type="button"
          onClick={() => setActiveTab('antidpi')}
          className={`flex items-center gap-2 px-3.5 py-2 rounded-xl font-bold whitespace-nowrap transition-all ${
            activeTab === 'antidpi'
              ? 'bg-amber-500 text-slate-950 shadow-md shadow-amber-500/20'
              : 'bg-slate-900/80 text-slate-400 hover:text-white border border-white/5'
          }`}
        >
          <Zap size={14} />
          <span>{t('sec_anti_dpi')}</span>
        </button>

        <button
          type="button"
          onClick={() => setActiveTab('apps')}
          className={`flex items-center gap-2 px-3.5 py-2 rounded-xl font-bold whitespace-nowrap transition-all ${
            activeTab === 'apps'
              ? 'bg-sky-500 text-slate-950 shadow-md shadow-sky-500/20'
              : 'bg-slate-900/80 text-slate-400 hover:text-white border border-white/5'
          }`}
        >
          <Split size={14} />
          <span>{t('sec_split_tunneling')}</span>
        </button>

        <button
          type="button"
          onClick={() => setActiveTab('appearance')}
          className={`flex items-center gap-2 px-3.5 py-2 rounded-xl font-bold whitespace-nowrap transition-all ${
            activeTab === 'appearance'
              ? 'bg-purple-500 text-white shadow-md shadow-purple-500/20'
              : 'bg-slate-900/80 text-slate-400 hover:text-white border border-white/5'
          }`}
        >
          <Palette size={14} />
          <span>{t('sec_appearance')}</span>
        </button>

        <button
          type="button"
          onClick={() => setActiveTab('system')}
          className={`flex items-center gap-2 px-3.5 py-2 rounded-xl font-bold whitespace-nowrap transition-all ${
            activeTab === 'system'
              ? 'bg-emerald-500 text-slate-950 shadow-md shadow-emerald-500/20'
              : 'bg-slate-900/80 text-slate-400 hover:text-white border border-white/5'
          }`}
        >
          <Cpu size={14} />
          <span>{t('sec_system')}</span>
        </button>
      </div>

      {/* TAB 1: Network & Cores */}
      {activeTab === 'network' && (
        <div className="space-y-4">
          <div className="section">
            <div className="section-title">
              <Wifi size={13} style={{ display: 'inline', marginInlineEnd: 6 }} />
              {t('sec_routing_cores')}
            </div>
            <div className="card">
              <div className="card-body" style={{ padding: 0 }}>
                {/* Proxy Mode */}
                <SettingRow
                  id="proxy-mode-row"
                  icon={<Layers size={16} />}
                  label={t('vpn_proxy_mode_label')}
                  description={t('vpn_proxy_mode_desc')}
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

                {/* V2Ray Core Selector */}
                <SettingRow
                  id="v2ray-core-select-row"
                  icon={<Cpu size={16} />}
                  label={t('v2ray_core_label')}
                  description={t('v2ray_core_desc')}
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

                {/* Primary DNS Engine */}
                <SettingRow
                  id="dns-server-row"
                  icon={<Globe size={16} />}
                  label={t('dns_engine_label')}
                  description={t('dns_engine_desc')}
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
                    </div>
                  }
                />

                {/* Fallback DNS */}
                <SettingRow
                  id="dns-fallback-row"
                  icon={<Radio size={16} />}
                  label={t('dns_fallback_label')}
                  description={t('dns_fallback_desc')}
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

                {/* TUN MTU Size */}
                <SettingRow
                  id="tun-mtu-row"
                  icon={<Layers size={16} />}
                  label={t('tun_mtu_label')}
                  description={t('tun_mtu_desc')}
                  control={
                    <select
                      className="form-control font-mono"
                      style={{ width: '250px', fontSize: '12px' }}
                      value={settings.warp_port === 1280 ? '1280' : settings.warp_port === 1500 ? '1500' : '1420'}
                      onChange={e => update('warp_port', parseInt(e.target.value))}
                    >
                      <option value="1280">1280 Bytes (WARP / Mobile LTE Optimal)</option>
                      <option value="1420">1420 Bytes (WireGuard / V2Ray Standard)</option>
                      <option value="1500">1500 Bytes (Standard Ethernet LAN / Wi-Fi)</option>
                    </select>
                  }
                />

                {/* Real Delay Test URL */}
                <SettingRow
                  id="real-delay-url-row"
                  icon={<Radio size={16} />}
                  label={t('real_delay_url_label')}
                  description={t('real_delay_url_desc')}
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
            </div>
          </div>

          {/* Visual Custom Domain Rules Manager */}
          <div className="section">
            <div className="section-title">
              <Shield size={13} style={{ display: 'inline', marginInlineEnd: 6, color: '#06b6d4' }} />
              {t('custom_domain_rules_title')}
            </div>
            <div className="card p-4 space-y-4">
              {/* Always Proxy Domains */}
              <div className="space-y-2">
                <div className="flex items-center justify-between text-xs font-bold text-slate-200">
                  <span className="flex items-center gap-1.5 text-cyan-400">
                    <span>🌐</span> {t('always_proxy_domains_label')}
                  </span>
                  <span className="text-[11px] font-normal text-slate-400">{t('always_proxy_domains_desc')}</span>
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
                  <button type="button" className="btn btn-secondary btn-sm text-xs px-3" onClick={() => addDomainRule('proxy')}>
                    {t('add')}
                  </button>
                </div>
                <div className="flex items-center gap-1.5 flex-wrap">
                  {(settings.custom_proxy_domains || []).map(d => (
                    <span key={d} className="inline-flex items-center gap-1 text-xs px-2.5 py-1 rounded-lg bg-cyan-500/10 text-cyan-300 border border-cyan-500/20 font-mono">
                      {d}
                      <button type="button" onClick={() => removeDomainRule('proxy', d)} className="text-slate-400 hover:text-rose-400 ml-1">
                        <X size={11} />
                      </button>
                    </span>
                  ))}
                  {(!settings.custom_proxy_domains || settings.custom_proxy_domains.length === 0) && (
                    <span className="text-[11px] text-slate-500 italic">{t('no_custom_domains')}</span>
                  )}
                </div>
              </div>

              {/* Always Direct Domains */}
              <div className="space-y-2 pt-3 border-t border-white/5">
                <div className="flex items-center justify-between text-xs font-bold text-slate-200">
                  <span className="flex items-center gap-1.5 text-emerald-400">
                    <span>⚡</span> {t('always_direct_domains_label')}
                  </span>
                  <span className="text-[11px] font-normal text-slate-400">{t('always_direct_domains_desc')}</span>
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
                  <button type="button" className="btn btn-secondary btn-sm text-xs px-3" onClick={() => addDomainRule('direct')}>
                    {t('add')}
                  </button>
                </div>
                <div className="flex items-center gap-1.5 flex-wrap">
                  {(settings.custom_direct_domains || []).map(d => (
                    <span key={d} className="inline-flex items-center gap-1 text-xs px-2.5 py-1 rounded-lg bg-emerald-500/10 text-emerald-300 border border-emerald-500/20 font-mono">
                      {d}
                      <button type="button" onClick={() => removeDomainRule('direct', d)} className="text-slate-400 hover:text-rose-400 ml-1">
                        <X size={11} />
                      </button>
                    </span>
                  ))}
                  {(!settings.custom_direct_domains || settings.custom_direct_domains.length === 0) && (
                    <span className="text-[11px] text-slate-500 italic">{t('no_custom_domains')}</span>
                  )}
                </div>
              </div>

              {/* Blocked Domains */}
              <div className="space-y-2 pt-3 border-t border-white/5">
                <div className="flex items-center justify-between text-xs font-bold text-slate-200">
                  <span className="flex items-center gap-1.5 text-rose-400">
                    <span>🛑</span> {t('blocked_domains_label')}
                  </span>
                  <span className="text-[11px] font-normal text-slate-400">{t('blocked_domains_desc')}</span>
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
                  <button type="button" className="btn btn-secondary btn-sm text-xs px-3" onClick={() => addDomainRule('block')}>
                    {t('add')}
                  </button>
                </div>
                <div className="flex items-center gap-1.5 flex-wrap">
                  {(settings.custom_block_domains || []).map(d => (
                    <span key={d} className="inline-flex items-center gap-1 text-xs px-2.5 py-1 rounded-lg bg-rose-500/10 text-rose-300 border border-rose-500/20 font-mono">
                      {d}
                      <button type="button" onClick={() => removeDomainRule('block', d)} className="text-slate-400 hover:text-rose-400 ml-1">
                        <X size={11} />
                      </button>
                    </span>
                  ))}
                  {(!settings.custom_block_domains || settings.custom_block_domains.length === 0) && (
                    <span className="text-[11px] text-slate-500 italic">{t('no_custom_domains')}</span>
                  )}
                </div>
              </div>
            </div>
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
            <div className="card">
              <div className="card-body" style={{ padding: 0 }}>
                {/* TLS Fragment */}
                <SettingRow
                  id="fragment-toggle"
                  icon={<Zap size={16} />}
                  label={t('anti_dpi_fragment_label')}
                  description={t('anti_dpi_fragment_desc')}
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
              </div>
            </div>
          </div>

          {/* AmneziaWG Settings */}
          <div className="section">
            <div className="section-title">
              <Shield size={13} style={{ display: 'inline', marginInlineEnd: 6, color: '#10b981' }} />
              {t('amnezia_section_title')}
            </div>
            <div className="card">
              <div className="card-body" style={{ padding: 0 }}>
                <SettingRow
                  id="amnezia-jc"
                  icon={<Sparkles size={16} />}
                  label={t('amnezia_jc_label')}
                  description={t('amnezia_jc_desc')}
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
                      <span className="text-slate-400 text-xs">-</span>
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
                      <span className="text-slate-400 text-xs">/</span>
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

                {/* Stealth Traffic Camouflage */}
                <SettingRow
                  id="stealth-mode-row"
                  icon={<Sparkles size={16} />}
                  label={t('stealth_traffic_label')}
                  description={t('stealth_traffic_desc')}
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
              </div>
            </div>
          </div>
        </div>
      )}

      {/* TAB 3: Per-App Split Tunneling */}
      {activeTab === 'apps' && (
        <div className="space-y-4">
          <div className="section">
            <div className="section-title">
              <Split size={13} style={{ display: 'inline', marginInlineEnd: 6, color: '#38bdf8' }} />
              {t('sec_split_tunneling')}
            </div>
            <div className="card">
              <div className="card-body" style={{ padding: 0 }}>
                <SettingRow
                  id="app-routing-mode"
                  icon={<Layers size={16} />}
                  label={t('app_routing_label')}
                  description={t('app_routing_desc')}
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
                    <div className="text-xs font-semibold text-slate-300 mb-2">{t('popular_apps_title')}</div>
                    <div className="flex flex-wrap gap-2 mb-3">
                      {popularApps.map(app => {
                        const isSelected = (settings.selected_apps || []).includes(app.exe)
                        return (
                          <button
                            key={app.exe}
                            type="button"
                            onClick={() => toggleApp(app.exe)}
                            className={`text-xs px-3 py-1.5 rounded-lg border font-medium transition ${
                              isSelected
                                ? 'bg-cyan-500/20 border-cyan-500 text-cyan-400'
                                : 'bg-slate-800/80 border-slate-700 text-slate-400 hover:text-slate-200'
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
                      <button type="button" className="btn btn-secondary btn-sm" onClick={addCustomApp}>
                        <Plus size={14} /> {t('add_app_btn')}
                      </button>
                    </div>

                    {(settings.selected_apps || []).length > 0 && (
                      <div className="flex flex-wrap gap-1.5 mt-3">
                        {settings.selected_apps?.map(exe => (
                          <span
                            key={exe}
                            className="inline-flex items-center gap-1.5 bg-slate-800 text-slate-300 px-2.5 py-1 rounded-md text-xs font-mono border border-slate-700"
                          >
                            {exe}
                            <X
                              size={12}
                              className="cursor-pointer hover:text-rose-400"
                              onClick={() => toggleApp(exe)}
                            />
                          </span>
                        ))}
                      </div>
                    )}
                  </div>
                )}
              </div>
            </div>
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
            <div className="card">
              <div className="card-body" style={{ padding: 0 }}>
                {/* Language Selector */}
                <SettingRow
                  id="language-select-row"
                  icon={<Globe size={16} />}
                  label={t('language_label')}
                  description={t('language_desc')}
                  control={
                    <select
                      id="language-select"
                      className="form-control"
                      style={{ width: '180px', fontSize: '13px' }}
                      value={getLanguage()}
                      onChange={e => update('language', e.target.value as any)}
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
                      <div className="text-xs text-muted" style={{ marginTop: '2px' }}>
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
                          className={`flex items-center gap-2.5 p-2.5 rounded-xl border text-left transition ${
                            isSelected
                              ? 'border-accent bg-accent/15 shadow-md shadow-accent/20'
                              : 'border-slate-800 bg-slate-900/60 hover:border-slate-700'
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
                              <div className="text-[10px] text-accent font-semibold" style={{ color: 'var(--color-accent-light)' }}>
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
            </div>
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
            <div className="card">
              <div className="card-body" style={{ padding: 0 }}>
                {/* Auto Connect */}
                <SettingRow
                  id="auto-connect"
                  icon={<RefreshCw size={16} />}
                  label={t('auto_connect_label')}
                  description={t('auto_connect_desc')}
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

                {/* Kill Switch */}
                <SettingRow
                  id="kill-switch"
                  icon={<Shield size={16} />}
                  label={t('kill_switch_label')}
                  description={t('kill_switch_desc')}
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
                  control={
                    <button
                      className="btn btn-secondary btn-sm"
                      onClick={handleClearStaleProxy}
                      disabled={clearingProxy}
                    >
                      {clearingProxy ? '...' : proxyCleared ? '✓ Cleared' : t('clear_proxy_btn')}
                    </button>
                  }
                />
              </div>
            </div>
          </div>

          {/* Core Binaries & Routing Databases Management Section */}
          <div className="section">
            <div className="section-title flex items-center justify-between">
              <div>
                <Cpu size={13} style={{ display: 'inline', marginInlineEnd: 6 }} />
                {t('sec_cores')}
              </div>
              <button
                className="btn btn-secondary btn-sm text-[11px] flex items-center gap-1.5"
                onClick={handleUpdateGeoData}
                disabled={updatingGeo}
              >
                <RefreshCw size={12} className={updatingGeo ? 'animate-spin' : ''} />
                {updatingGeo ? t('updating') : geoUpdated ? '✓ GeoIP & GeoSite Updated' : t('update_geo_rules')}
              </button>
            </div>
            <div className="card">
              <div className="card-header flex items-center justify-between">
                <p className="text-xs text-muted" style={{ margin: 0 }}>
                  {t('cores_desc')}
                </p>
                <button
                  className="btn btn-primary btn-sm"
                  onClick={handleAutoInstallMissingCores}
                  disabled={isInstallingCores}
                >
                  <Download size={13} />
                  {isInstallingCores ? t('installing') : t('auto_install_all')}
                </button>
              </div>
              <div className="card-body" style={{ padding: 0 }}>
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
                          <span className="text-[10px] font-mono px-1.5 py-0.5 rounded bg-slate-800 text-cyan-400 border border-slate-700">
                            {info.version}
                          </span>
                        )}
                      </div>
                      <div className="text-xs text-muted font-mono">{info.filename}</div>
                    </div>

                    <div className="flex items-center gap-2">
                      <button
                        type="button"
                        className="btn btn-secondary btn-sm text-[11px] px-2.5 py-1 flex items-center gap-1"
                        onClick={() => handleUpdateCore(key)}
                        disabled={updatingCore === key}
                      >
                        <RefreshCw size={11} className={updatingCore === key ? 'animate-spin' : ''} />
                        {updatingCore === key ? t('updating') : t('update_core_btn')}
                      </button>

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
            </div>
          </div>

          {/* Global Keyboard Shortcuts Guide */}
          <div className="section">
            <div className="section-title">
              <Keyboard size={13} style={{ display: 'inline', marginInlineEnd: 6, color: '#38bdf8' }} />
              {t('global_hotkeys_title')}
            </div>
            <div className="card p-4 space-y-3">
              <p className="text-xs text-slate-400">
                {t('global_hotkeys_desc')}
              </p>
              <div className="grid grid-cols-1 sm:grid-cols-2 gap-2 text-xs">
                <div className="bg-slate-950/70 p-2.5 rounded-xl border border-white/5 flex items-center justify-between">
                  <span className="text-slate-300">{t('hotkey_toggle_connect')}</span>
                  <kbd className="font-mono font-bold bg-slate-800 text-cyan-300 px-2 py-0.5 rounded text-[11px] border border-slate-700">
                    Ctrl + Shift + V
                  </kbd>
                </div>

                <div className="bg-slate-950/70 p-2.5 rounded-xl border border-white/5 flex items-center justify-between">
                  <span className="text-slate-300">{t('hotkey_speed_test')}</span>
                  <kbd className="font-mono font-bold bg-slate-800 text-emerald-300 px-2 py-0.5 rounded text-[11px] border border-slate-700">
                    Ctrl + Shift + S
                  </kbd>
                </div>

                <div className="bg-slate-950/70 p-2.5 rounded-xl border border-white/5 flex items-center justify-between">
                  <span className="text-slate-300">{t('hotkey_radar')}</span>
                  <kbd className="font-mono font-bold bg-slate-800 text-cyan-300 px-2 py-0.5 rounded text-[11px] border border-slate-700">
                    Ctrl + Shift + R
                  </kbd>
                </div>

                <div className="bg-slate-950/70 p-2.5 rounded-xl border border-white/5 flex items-center justify-between">
                  <span className="text-slate-300">{t('hotkey_lan_share')}</span>
                  <kbd className="font-mono font-bold bg-slate-800 text-purple-300 px-2 py-0.5 rounded text-[11px] border border-slate-700">
                    Ctrl + Shift + L
                  </kbd>
                </div>

                <div className="bg-slate-950/70 p-2.5 rounded-xl border border-white/5 flex items-center justify-between">
                  <span className="text-slate-300">{t('hotkey_emergency')}</span>
                  <kbd className="font-mono font-bold bg-slate-800 text-rose-300 px-2 py-0.5 rounded text-[11px] border border-slate-700">
                    Ctrl + Shift + E
                  </kbd>
                </div>

                <div className="bg-slate-950/70 p-2.5 rounded-xl border border-white/5 flex items-center justify-between">
                  <span className="text-slate-300">{t('hotkey_quota')}</span>
                  <kbd className="font-mono font-bold bg-slate-800 text-amber-300 px-2 py-0.5 rounded text-[11px] border border-slate-700">
                    Ctrl + Shift + Q
                  </kbd>
                </div>
              </div>
            </div>
          </div>
        </div>
      )}
    </div>
  )
}

function SettingRow({
  id,
  icon,
  label,
  description,
  control,
}: {
  id: string
  icon: React.ReactNode
  label: string
  description: string
  control: React.ReactNode
}) {
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
          <div className="text-xs text-muted" style={{ marginTop: '2px' }}>
            {description}
          </div>
        </div>
      </div>
      <div>{control}</div>
    </div>
  )
}
