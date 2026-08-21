import React, { useState, useEffect } from 'react'
import {
  LayoutDashboard,
  Radio,
  Shield,
  Lock,
  Settings as SettingsIcon,
  Terminal,
  Globe,
  Palette,
  ShieldCheck,
  Layers,
  Cpu,
} from 'lucide-react'
import Dashboard from './pages/Dashboard'
import V2RayPage from './pages/V2RayPage'
import WarpWireGuardPage from './pages/WarpWireGuardPage'
import OpenVPNPage from './pages/OpenVPNPage'
import DiscoveryHub from './pages/DiscoveryHub'
import SettingsPage from './pages/Settings'
import Logs from './pages/Logs'
import AetherPage from './pages/AetherPage'
import { NetworkDoctorModal } from './components/NetworkDoctorModal'
import { VisualAppSelectorModal } from './components/VisualAppSelectorModal'
import FloatingMiniWidget from './components/FloatingMiniWidget'
import AboutModal from './components/AboutModal'
import GlobalContextMenu from './components/GlobalContextMenu'
import { api } from './lib/api'
import { t, getLanguage, setLanguage, onLanguageChange } from './lib/i18n'
import { initTheme, onThemeChange, getTheme, applyTheme, THEMES } from './lib/theme'
import { handleGlobalStatusNotification } from './lib/notification'
import type { ConnectionStatus, TrafficStats, Settings } from './types'
import { Info, Compass } from 'lucide-react'

type NavTab = 'dashboard' | 'v2ray' | 'warp' | 'openvpn' | 'aether' | 'discovery' | 'settings' | 'logs'

export default function App() {
  const [activeTab, setActiveTab] = useState<NavTab>('dashboard')
  const [status, setStatus] = useState<ConnectionStatus>({ state: 'disconnected' })
  const [lang, setLangState] = useState(getLanguage())
  const [theme, setThemeState] = useState(getTheme())
  const [settings, setSettings] = useState<Settings>({} as Settings)
  const [isDoctorOpen, setIsDoctorOpen] = useState(false)
  const [isAppSelectorOpen, setIsAppSelectorOpen] = useState(false)
  const [isAboutOpen, setIsAboutOpen] = useState(false)
  const [appVersion, setAppVersion] = useState<string>('1.3.3')
  const [stats, setStats] = useState<TrafficStats>({
    bytes_received: 0,
    bytes_sent: 0,
    speed_download: 0,
    speed_upload: 0,
    packets_received: 0,
    packets_sent: 0,
  })

  useEffect(() => {
    initTheme()
    api.setTrayLanguage(getLanguage()).catch(() => {})

    api.getVersion().then(v => {
      if (v) setAppVersion(v)
    }).catch(() => {})

    api.getSettings().then(s => {
      if (s) {
        setSettings(s)
        if (s.language && (s.language === 'fa' || s.language === 'en')) {
          setLanguage(s.language as any)
        }
        if (s.theme) {
          applyTheme(s.theme as any)
        }
      }
    })

    const unsubLang = onLanguageChange(newLang => {
      setLangState(newLang)
    })

    const unsubTheme = onThemeChange(newTheme => {
      setThemeState(newTheme)
    })

    api.getConnectionStatus().then(setStatus)
    api.getTrafficStats().then(setStats)

    // Listen to real-time events pushed directly from Go backend
    const unlistenStatus = api.onEvent('status:changed', (newStatus: ConnectionStatus) => {
      if (newStatus) {
        setStatus(newStatus)
        handleGlobalStatusNotification(newStatus)
      }
    })
    const unlistenStats = api.onEvent('stats:update', (newStats: TrafficStats) => {
      setStats(newStats)
    })

    // Low-overhead fallback poll — only fires if Wails events are unavailable (dev/mock mode)
    const hasWailsRuntime =
      typeof window !== 'undefined' &&
      (window as any).runtime?.EventsOn
    const interval = hasWailsRuntime
      ? null
      : setInterval(() => {
          api.getConnectionStatus().then(st => {
            if (st) {
              setStatus(st)
              handleGlobalStatusNotification(st)
            }
          })
        }, 3000)

    return () => {
      unsubLang()
      unsubTheme()
      unlistenStatus()
      unlistenStats()
      if (interval) clearInterval(interval)
    }
  }, [])

  const handleToggleLang = () => {
    const nextLang = lang === 'fa' ? 'en' : 'fa'
    setLanguage(nextLang)
    api.getSettings().then(s => {
      api.saveSettings({ ...s, language: nextLang })
    })
  }

  const handleCycleTheme = () => {
    const currentIndex = THEMES.findIndex(t => t.id === theme)
    const nextTheme = THEMES[(currentIndex + 1) % THEMES.length].id
    applyTheme(nextTheme)
    api.getSettings().then(s => {
      api.saveSettings({ ...s, theme: nextTheme })
    })
  }

  const isConnected = status.state === 'connected'
  const isConnecting = status.state === 'connecting'

  return (
    // Use dir/lang attributes instead of key={lang} to avoid full remount on language change
    <div className="app-container" dir={lang === 'fa' ? 'rtl' : 'ltr'} lang={lang}>
      <aside className="sidebar">
        <div className="sidebar-brand">
          <div className="sidebar-brand-icon overflow-hidden bg-transparent border-0">
            <img src="/appicon.png" alt="NexVPN" className="w-8 h-8 rounded-xl object-contain shadow-md ring-1 ring-cyan-500/30" />
          </div>
          <div className="min-w-0 flex-1">
            <div className="sidebar-brand-title">{t('app_title')}</div>
            <div className="sidebar-brand-subtitle">{t('app_subtitle')}</div>
          </div>
        </div>

        <nav className="sidebar-nav">
          <button
            id="nav-dashboard"
            className={`nav-item${activeTab === 'dashboard' ? ' active' : ''}`}
            onClick={() => setActiveTab('dashboard')}
            title={t('nav_dashboard')}
          >
            <LayoutDashboard size={18} />
            <span>{t('nav_dashboard')}</span>
          </button>

          <button
            id="nav-v2ray"
            className={`nav-item${activeTab === 'v2ray' ? ' active' : ''}`}
            onClick={() => setActiveTab('v2ray')}
            title={t('nav_v2ray')}
          >
            <Radio size={18} />
            <span>{t('nav_v2ray')}</span>
          </button>

          <button
            id="nav-warp"
            className={`nav-item${activeTab === 'warp' ? ' active' : ''}`}
            onClick={() => setActiveTab('warp')}
            title={t('nav_warp_wireguard')}
          >
            <Shield size={18} />
            <span>{t('nav_warp_wireguard')}</span>
          </button>

          <button
            id="nav-openvpn"
            className={`nav-item${activeTab === 'openvpn' ? ' active' : ''}`}
            onClick={() => setActiveTab('openvpn')}
            title={t('nav_openvpn')}
          >
            <Lock size={18} />
            <span>{t('nav_openvpn')}</span>
          </button>

          <button
            id="nav-aether"
            className={`nav-item${activeTab === 'aether' ? ' active' : ''}`}
            onClick={() => setActiveTab('aether')}
            title="Aether Proxy"
          >
            <Cpu size={18} />
            <span>Aether</span>
          </button>

          <button
            id="nav-discovery"
            className={`nav-item${activeTab === 'discovery' ? ' active' : ''}`}
            onClick={() => setActiveTab('discovery')}
            title={t('nav_discovery')}
          >
            <Compass size={18} className={activeTab === 'discovery' ? 'text-indigo-400 animate-spin-slow' : 'text-indigo-400/80'} />
            <span className="font-semibold">{t('nav_discovery')}</span>
          </button>

          <button
            id="nav-settings"
            className={`nav-item${activeTab === 'settings' ? ' active' : ''}`}
            onClick={() => setActiveTab('settings')}
            title={t('nav_settings')}
          >
            <SettingsIcon size={18} />
            <span>{t('nav_settings')}</span>
          </button>

          <button
            id="nav-logs"
            className={`nav-item${activeTab === 'logs' ? ' active' : ''}`}
            onClick={() => setActiveTab('logs')}
            title={t('nav_logs')}
          >
            <Terminal size={18} />
            <span>{t('nav_logs')}</span>
          </button>
        </nav>

        {/* Quick Tools Section */}
        <div className="px-3 py-2 border-t border-white/5 space-y-1">
          <button
            onClick={() => setIsDoctorOpen(true)}
            className="w-full flex items-center gap-2 px-3 py-2 rounded-xl text-xs font-semibold text-cyan-300 bg-cyan-500/10 hover:bg-cyan-500/20 border border-cyan-500/20 transition-all"
            title={t('nav_doctor')}
          >
            <ShieldCheck size={16} className="text-cyan-400 shrink-0" />
            <span className="truncate">{t('nav_doctor')}</span>
          </button>

          <button
            onClick={() => setIsAppSelectorOpen(true)}
            className="w-full flex items-center gap-2 px-3 py-2 rounded-xl text-xs font-semibold text-purple-300 bg-purple-500/10 hover:bg-purple-500/20 border border-purple-500/20 transition-all"
            title={t('nav_split')}
          >
            <Layers size={16} className="text-purple-400 shrink-0" />
            <span className="truncate">{t('nav_split')}</span>
          </button>
        </div>

        {/* Sidebar Footer Controls & Status */}
        <div className="sidebar-footer">
          <div className="flex items-center justify-between gap-1.5">
            <button
              onClick={handleToggleLang}
              className="btn btn-secondary btn-sm flex-1 flex items-center justify-center gap-1.5 text-xs font-bold py-1.5"
              title="Toggle Language"
            >
              <Globe size={13} />
              <span>{lang === 'fa' ? 'English' : 'فارسی'}</span>
            </button>

            <button
              onClick={handleCycleTheme}
              className="btn btn-secondary btn-sm p-1.5"
              title={t('theme_label')}
            >
              <Palette size={14} className="text-accent" />
            </button>

            <button
              onClick={() => setIsAboutOpen(true)}
              className="btn btn-secondary btn-sm p-1.5 text-slate-300 hover:text-cyan-300"
              title={t('nav_about')}
            >
              <Info size={14} />
            </button>
          </div>

          <div
            onClick={() => setIsAboutOpen(true)}
            className="flex items-center justify-between px-2.5 py-1.5 rounded-xl bg-slate-900/80 hover:bg-slate-800/80 border border-slate-800 transition cursor-pointer text-[11px]"
          >
            <div className="flex items-center gap-2">
              <span
                className={`w-2 h-2 rounded-full ${
                  isConnected
                    ? 'bg-emerald-400 shadow-[0_0_8px_#10b981]'
                    : isConnecting
                    ? 'bg-amber-400 shadow-[0_0_8px_#f59e0b] animate-ping'
                    : 'bg-slate-500'
                }`}
              />
              <span className="font-semibold text-slate-300 truncate">{t('engine_active')}</span>
            </div>
            <span className="text-[10px] text-slate-400 font-mono">v{appVersion}</span>
          </div>
        </div>
      </aside>

      {/* Main Content Area */}
      <main className="main-content">
        {activeTab === 'dashboard' && (
          <Dashboard
            status={status}
            stats={stats}
            onOpenDoctor={() => setIsDoctorOpen(true)}
            onOpenAppSelector={() => setIsAppSelectorOpen(true)}
          />
        )}
        {activeTab === 'v2ray' && <V2RayPage status={status} onStatusChange={setStatus} />}
        {activeTab === 'warp' && <WarpWireGuardPage status={status} onStatusChange={setStatus} onNavigateTab={setActiveTab} />}
        {activeTab === 'openvpn' && <OpenVPNPage status={status} onStatusChange={setStatus} />}
        {activeTab === 'aether' && <AetherPage status={status} onStatusChange={setStatus} />}
        {activeTab === 'discovery' && <DiscoveryHub status={status} onStatusChange={setStatus} />}
        {activeTab === 'settings' && <SettingsPage onSettingsSaved={setSettings} />}
        {activeTab === 'logs' && <Logs />}
      </main>

      {/* Floating Mini Widget */}
      {settings && settings.show_mini_widget === true && (
        <FloatingMiniWidget
          status={status}
          stats={stats}
          onCloseWidget={async () => {
            const updated = { ...settings, show_mini_widget: false }
            setSettings(updated)
            await api.saveSettings(updated)
          }}
          onToggleConnect={async () => {
            if (isConnected || isConnecting) {
              await api.disconnect()
            } else {
              await api.connectFastest('all')
            }
          }}
        />
      )}

      {/* Network Doctor Diagnostics Modal */}
      <NetworkDoctorModal isOpen={isDoctorOpen} onClose={() => setIsDoctorOpen(false)} />

      {/* Visual App Split-Tunneling Selector Modal */}
      <VisualAppSelectorModal
        isOpen={isAppSelectorOpen}
        onClose={() => setIsAppSelectorOpen(false)}
        settings={settings}
        onSettingsUpdated={setSettings}
      />

      {/* About Modal */}
      <AboutModal isOpen={isAboutOpen} onClose={() => setIsAboutOpen(false)} />

      {/* Global Right-Click Context Menu */}
      <GlobalContextMenu />
    </div>
  )
}
