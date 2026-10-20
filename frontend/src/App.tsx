import { useState, useEffect, useCallback, memo, Suspense, lazy } from 'react'
import { AnimatePresence, motion } from 'framer-motion'
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
  Search,
  Info,
  Compass,
  Flame,
  type LucideIcon,
} from 'lucide-react'
import Dashboard from './pages/Dashboard'
const V2RayPage = lazy(() => import('./pages/V2RayPage'))
const WarpWireGuardPage = lazy(() => import('./pages/WarpWireGuardPage'))
const OpenVPNPage = lazy(() => import('./pages/OpenVPNPage'))
const DiscoveryHub = lazy(() => import('./pages/DiscoveryHub'))
const SettingsPage = lazy(() => import('./pages/Settings'))
const Logs = lazy(() => import('./pages/Logs'))
const AetherPage = lazy(() => import('./pages/AetherPage'))
const PsiphonPage = lazy(() => import('./pages/PsiphonPage'))

// Memoised page shells.
//
// The engine emits `status:changed` and `stats:update` once a second, and App
// keeps both in state — so every one of those ticks re-rendered whichever page
// was on screen. Pages that never read those values (Discovery, Settings, Logs)
// were doing full re-renders for nothing, and the project had no `React.memo`
// anywhere. Wrapping at the call site keeps each page a plain component and
// puts the decision in the one place that knows which props actually change.
//
// The pages that do take `status`/`stats` still re-render — that is correct, and
// the memo compare costs nothing — but the `onStatusChange`/`onSettingsSaved`
// props are stable setters, so their other props no longer force work.
const MemoV2RayPage = memo(V2RayPage)
const MemoWarpWireGuardPage = memo(WarpWireGuardPage)
const MemoOpenVPNPage = memo(OpenVPNPage)
const MemoDiscoveryHub = memo(DiscoveryHub)
const MemoSettingsPage = memo(SettingsPage)
const MemoLogs = memo(Logs)
const MemoAetherPage = memo(AetherPage)
const MemoPsiphonPage = memo(PsiphonPage)
const MemoDashboard = memo(Dashboard)

import { NetworkDoctorModal } from './components/NetworkDoctorModal'
import { VisualAppSelectorModal } from './components/VisualAppSelectorModal'
import FloatingMiniWidget from './components/FloatingMiniWidget'
import AboutModal from './components/AboutModal'
import GlobalContextMenu from './components/GlobalContextMenu'
import CommandPalette from './components/CommandPalette'
import AriaLive from './components/AriaLive'
import { Button } from './components/ui/button'
import { Separator } from './components/ui/separator'
import {
  Tooltip,
  TooltipContent,
  TooltipProvider,
  TooltipTrigger,
} from './components/ui/tooltip'
import { api } from './lib/api'
import { cn, reportFailure } from './lib/utils'
import { t, getLanguage, setLanguage, onLanguageChange } from './lib/i18n'
import { announce } from './lib/announcer'
import { initTheme, onThemeChange, getTheme, applyTheme, THEMES, type Theme } from './lib/theme'
import { handleGlobalStatusNotification, sendDesktopNotification } from './lib/notification'
import type { ConnectionStatus, TrafficStats, Settings } from './types'

/**
 * Suspense fallback for the lazily-loaded pages.
 *
 * Previously a cyan-bordered spinner with a mono caption. It is now a neutral
 * ring plus a skeleton stack, so a cold page load looks like the layout
 * arriving rather than like a different design.
 */
const PageLoader = () => (
  <div className="flex w-full flex-col gap-4" role="status" aria-busy="true">
    <div className="flex items-center gap-3">
      <span className="h-5 w-5 shrink-0 animate-spin rounded-full border-2 border-muted border-t-primary" />
      <span className="text-sm text-muted-foreground">Loading…</span>
    </div>
    <div className="grid gap-3 sm:grid-cols-2">
      <div className="h-28 animate-pulse rounded-xl bg-muted" />
      <div className="h-28 animate-pulse rounded-xl bg-muted" />
    </div>
  </div>
)

type NavTab =
  | 'dashboard'
  | 'v2ray'
  | 'warp'
  | 'openvpn'
  | 'aether'
  | 'psiphon'
  | 'discovery'
  | 'settings'
  | 'logs'

export default function App() {
  const [activeTab, setActiveTab] = useState<NavTab>('dashboard')
  const [status, setStatus] = useState<ConnectionStatus>({ state: 'disconnected' })
  const [lang, setLangState] = useState(getLanguage())
  const [theme, setThemeState] = useState(getTheme())
  const [settings, setSettings] = useState<Settings>({} as Settings)
  const [isDoctorOpen, setIsDoctorOpen] = useState(false)
  // Stable identity, so `MemoDashboard` is not invalidated by a new arrow
  // function on every one of the engine's once-a-second renders.
  const openDoctor = useCallback(() => setIsDoctorOpen(true), [])
  const [isAppSelectorOpen, setIsAppSelectorOpen] = useState(false)
  const [isAboutOpen, setIsAboutOpen] = useState(false)
  const [isCommandPaletteOpen, setIsCommandPaletteOpen] = useState(false)
  const [appVersion, setAppVersion] = useState<string>('1.4.0')
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
          setLanguage(s.language)
        }
        if (s.theme) {
          applyTheme(s.theme as Theme)
        }
      }
    }).catch(reportFailure('load settings'))

    const unsubLang = onLanguageChange(newLang => {
      setLangState(newLang)
    })

    const unsubTheme = onThemeChange(newTheme => {
      setThemeState(newTheme)
    })

    // Keep the native window title in sync with the connection state so the
    // taskbar/tray entry reflects reality even while the window is minimised.
    const applyWindowTitle = (st: ConnectionStatus | null | undefined) => {
      if (!st) return
      if (st.state === 'connected' && st.profile_name) {
        api.setTitle(`NexVPN Pro — ${st.profile_name}`)
      } else if (st.state === 'disconnected') {
        api.setTitle('NexVPN Pro')
      }
    }

    api.getConnectionStatus().then(st => {
      setStatus(st)
      applyWindowTitle(st)
    }).catch(reportFailure('initial status'))
    api.getTrafficStats().then(setStats).catch(reportFailure('initial stats'))

    // Listen to real-time events pushed directly from Go backend
    const unlistenStatus = api.onEvent('status:changed', (newStatus: ConnectionStatus) => {
      if (newStatus) {
        setStatus(newStatus)
        applyWindowTitle(newStatus)
        handleGlobalStatusNotification(newStatus)
      }
    })
    const unlistenStats = api.onEvent('stats:update', (newStats: TrafficStats) => {
      setStats(newStats)
    })

    // Background automation is otherwise invisible: the watchers in
    // app_watchdog.go silently re-validate the tunnel when the network changes
    // or the machine wakes from sleep. The engine already reacts on its own —
    // these two subscriptions exist so the user is *told* that something
    // happened, through the same two channels the connect/disconnect path
    // already uses: the ARIA live region, and a native notification for the
    // cases worth interrupting for. Nothing visual is added.
    const unlistenNetwork = api.onEvent('network:ssid', (ssid: string) => {
      const name = typeof ssid === 'string' ? ssid.trim() : ''
      const message = name ? `${t('adv_net_changed')} — ${name}` : t('adv_net_changed')
      announce(message)
      // Worth a native notification: the tunnel is being re-checked, which can
      // end in a disconnect the user would otherwise not see coming.
      sendDesktopNotification(t('adv_net_changed'), name, '📶')
    })
    const unlistenPower = api.onEvent('power:event', (kind: string) => {
      switch (kind) {
        case 'suspend':
          announce(t('adv_power_suspend'))
          break
        case 'resume':
          // The interesting one: the core process may have died while the
          // machine was asleep, so this is the user's explanation for whatever
          // the status indicator is about to say.
          announce(t('adv_power_resume'), 'assertive')
          sendDesktopNotification('NexVPN Pro', t('adv_power_resume'), '🌙')
          break
        case 'ac':
          // Mains/battery transitions are informational and frequent, so they
          // stay in the live region rather than interrupting with a toast.
          announce(t('adv_power_ac'))
          break
        case 'battery':
          announce(t('adv_power_battery'))
          break
      }
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
          }).catch(reportFailure('status poll'))
        }, 3000)

    // Global Command Palette Shortcut Listener (Ctrl+K or Cmd+K)
    const handleGlobalKeyDown = (e: KeyboardEvent) => {
      if ((e.ctrlKey || e.metaKey) && (e.key === 'k' || e.key === 'K')) {
        e.preventDefault()
        setIsCommandPaletteOpen(prev => !prev)
      }
    }
    window.addEventListener('keydown', handleGlobalKeyDown)

    return () => {
      unsubLang()
      unsubTheme()
      unlistenStatus()
      unlistenStats()
      unlistenNetwork()
      unlistenPower()
      if (interval) clearInterval(interval)
      window.removeEventListener('keydown', handleGlobalKeyDown)
    }
  }, [])

  const handleToggleLang = () => {
    const nextLang = lang === 'fa' ? 'en' : 'fa'
    setLanguage(nextLang)
    api.getSettings().then(s => {
      api.saveSettings({ ...s, language: nextLang })
    }).catch(reportFailure('save language'))
  }

  const handleCycleTheme = () => {
    const currentIndex = THEMES.findIndex(t => t.id === theme)
    const nextTheme = THEMES[(currentIndex + 1) % THEMES.length].id
    applyTheme(nextTheme)
    api.getSettings().then(s => {
      api.saveSettings({ ...s, theme: nextTheme })
    }).catch(reportFailure('save theme'))
  }

  const handleToggleConnect = async () => {
    if (status.state === 'connected' || status.state === 'connecting') {
      await api.disconnect()
    } else {
      await api.connectFastest('all')
    }
  }

  const handlePanicWipe = async () => {
    try {
      await api.panicWipe()
    } catch (e) {
      console.error(e)
    }
  }

  const handleToggleProxyMode = async () => {
    try {
      const current = settings.proxy_mode || 'tun'
      const next = current === 'tun' ? 'system_proxy' : 'tun'
      const updated = { ...settings, proxy_mode: next }
      setSettings(updated)
      await api.saveSettings(updated)
      await api.setProxyMode(next)
    } catch (e) {
      console.error(e)
    }
  }

  const handleOpenModal = (modalName: string) => {
    if (modalName === 'doctor') {
      setIsDoctorOpen(true)
    } else if (modalName === 'apps') {
      setIsAppSelectorOpen(true)
    } else if (modalName === 'about') {
      setIsAboutOpen(true)
    } else {
      setActiveTab('dashboard')
      setTimeout(() => {
        window.dispatchEvent(new CustomEvent('nexvpn:open-modal', { detail: modalName }))
      }, 50)
    }
  }

  const isConnected = status.state === 'connected'
  const isConnecting = status.state === 'connecting'

  /*
   * Nav is data-driven. Every row shares one icon size, one colour and one
   * active treatment; the per-page accent icons that used to sit here (an amber
   * flame on Psiphon, an indigo compass on Discovery) made the sidebar read as
   * a set of unrelated links and competed with the active-row marker.
   */
  const navItems: Array<{ id: NavTab; icon: LucideIcon; label: string }> = [
    { id: 'dashboard', icon: LayoutDashboard, label: t('nav_dashboard') },
    { id: 'v2ray', icon: Radio, label: t('nav_v2ray') },
    { id: 'warp', icon: Shield, label: t('nav_warp_wireguard') },
    { id: 'openvpn', icon: Lock, label: t('nav_openvpn') },
    { id: 'aether', icon: Cpu, label: 'Aether' },
    { id: 'psiphon', icon: Flame, label: t('nav_psiphon') },
    { id: 'discovery', icon: Compass, label: t('nav_discovery') },
    { id: 'settings', icon: SettingsIcon, label: t('nav_settings') },
    { id: 'logs', icon: Terminal, label: t('nav_logs') },
  ]

  return (
    // Use dir/lang attributes instead of key={lang} to avoid full remount on language change
    <TooltipProvider delayDuration={400}>
      <div className="app-container" dir={lang === 'fa' ? 'rtl' : 'ltr'} lang={lang}>
        <aside className="sidebar">
          <div className="sidebar-brand">
            <img
              src="/appicon.png"
              alt=""
              className="h-8 w-8 shrink-0 rounded-lg object-contain"
            />
            <div className="min-w-0 flex-1">
              <div className="sidebar-brand-title truncate">{t('app_title')}</div>
              <div className="sidebar-brand-subtitle truncate">{t('app_subtitle')}</div>
            </div>
          </div>

          {/* Command Palette Trigger — styled as a search field, which is what
              it behaves like, instead of as a call-to-action button. */}
          <div className="px-3 pt-3">
            <button
              id="btn-command-palette"
              onClick={() => setIsCommandPaletteOpen(true)}
              className="flex h-9 w-full items-center gap-2 rounded-md border border-input bg-background px-2.5 text-xs text-muted-foreground transition-colors hover:bg-accent hover:text-accent-foreground focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-ring focus-visible:ring-offset-2 focus-visible:ring-offset-background"
              title={t('command_palette_shortcut')}
            >
              <Search className="h-3.5 w-3.5 shrink-0" />
              <span className="min-w-0 flex-1 truncate text-start">
                {t('command_palette_shortcut')}
              </span>
              <kbd className="shrink-0 rounded border border-border bg-muted px-1.5 py-0.5 font-mono text-[10px] text-muted-foreground">
                Ctrl K
              </kbd>
            </button>
          </div>

          <nav className="sidebar-nav">
            {navItems.map(({ id, icon: Icon, label }) => (
              <button
                key={id}
                id={`nav-${id}`}
                className={cn('nav-item', activeTab === id && 'active')}
                aria-current={activeTab === id ? 'page' : undefined}
                onClick={() => setActiveTab(id)}
              >
                <Icon className="h-[18px] w-[18px] shrink-0" />
                <span className="truncate">{label}</span>
              </button>
            ))}
          </nav>

          {/* Quick Tools — neutral. These are utilities, not primary actions,
              and four differently-coloured buttons in the sidebar were the
              loudest thing on the screen. */}
          <div className="space-y-1 border-t border-border px-3 py-3">
            <button
              onClick={() => setIsDoctorOpen(true)}
              className="flex h-8 w-full items-center gap-2 rounded-md px-2.5 text-xs font-medium text-muted-foreground transition-colors hover:bg-accent hover:text-accent-foreground focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-ring"
              title={t('nav_doctor')}
            >
              <ShieldCheck className="h-4 w-4 shrink-0" />
              <span className="truncate">{t('nav_doctor')}</span>
            </button>

            <button
              onClick={() => setIsAppSelectorOpen(true)}
              className="flex h-8 w-full items-center gap-2 rounded-md px-2.5 text-xs font-medium text-muted-foreground transition-colors hover:bg-accent hover:text-accent-foreground focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-ring"
              title={t('nav_split')}
            >
              <Layers className="h-4 w-4 shrink-0" />
              <span className="truncate">{t('nav_split')}</span>
            </button>
          </div>

          <div className="sidebar-footer">
            <div className="flex items-center gap-1.5">
              <Button
                variant="outline"
                size="sm"
                className="min-w-0 flex-1"
                onClick={handleToggleLang}
                title={t('lang_btn_text')}
              >
                <Globe className="h-3.5 w-3.5 shrink-0" />
                <span className="truncate">{t('lang_btn_text')}</span>
              </Button>

              <Tooltip>
                <TooltipTrigger asChild>
                  <Button
                    variant="outline"
                    size="icon-sm"
                    onClick={handleCycleTheme}
                    aria-label={t('theme_label')}
                  >
                    <Palette className="h-3.5 w-3.5" />
                  </Button>
                </TooltipTrigger>
                <TooltipContent>{t('theme_label')}</TooltipContent>
              </Tooltip>

              <Tooltip>
                <TooltipTrigger asChild>
                  <Button
                    variant="outline"
                    size="icon-sm"
                    onClick={() => setIsAboutOpen(true)}
                    aria-label={t('nav_about')}
                  >
                    <Info className="h-3.5 w-3.5" />
                  </Button>
                </TooltipTrigger>
                <TooltipContent>{t('nav_about')}</TooltipContent>
              </Tooltip>
            </div>

            <Separator />

            <button
              onClick={() => setIsAboutOpen(true)}
              className="flex w-full items-center justify-between gap-2 rounded-md px-2.5 py-1.5 text-[11px] transition-colors hover:bg-accent focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-ring"
            >
              <div className="flex min-w-0 items-center gap-2">
                <span
                  aria-hidden="true"
                  className={cn(
                    'h-1.5 w-1.5 shrink-0 rounded-full',
                    isConnected
                      ? 'bg-success'
                      : isConnecting
                      ? 'animate-pulse bg-warning'
                      : 'bg-muted-foreground/50'
                  )}
                />
                <span className="truncate font-medium text-muted-foreground">
                  {t('engine_active')}
                </span>
              </div>
              <span className="shrink-0 font-mono text-[10px] text-muted-foreground">
                v{appVersion}
              </span>
            </button>
          </div>
        </aside>

        <main className="main-content">
          <Suspense fallback={<PageLoader />}>
            <AnimatePresence mode="wait">
              <motion.div
                key={activeTab}
                initial={{ opacity: 0, y: 8 }}
                animate={{ opacity: 1, y: 0 }}
                exit={{ opacity: 0, y: -8 }}
                transition={{ duration: 0.18 }}
                className="w-full"
              >
                {activeTab === 'dashboard' && (
                  <MemoDashboard
                    status={status}
                    stats={stats}
                    onOpenDoctor={openDoctor}
                  />
                )}
                {activeTab === 'v2ray' && <MemoV2RayPage status={status} onStatusChange={setStatus} />}
                {activeTab === 'warp' && <MemoWarpWireGuardPage status={status} onStatusChange={setStatus} onNavigateTab={setActiveTab} />}
                {activeTab === 'openvpn' && <MemoOpenVPNPage status={status} onStatusChange={setStatus} />}
                {activeTab === 'aether' && <MemoAetherPage status={status} onStatusChange={setStatus} />}
                {activeTab === 'psiphon' && <MemoPsiphonPage status={status} stats={stats} />}
                {activeTab === 'discovery' && <MemoDiscoveryHub onStatusChange={setStatus} />}
                {activeTab === 'settings' && (
                  <MemoSettingsPage onSettingsSaved={setSettings} connectionState={status.state} />
                )}
                {activeTab === 'logs' && <MemoLogs />}
              </motion.div>
            </AnimatePresence>
          </Suspense>
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
            onHideToTray={() => {
              // Keep the mini widget enabled, but hide the main window into the
              // tray so only the floating widget stays visible.
              api.hideWindow()
            }}
            onOpenMainWindow={() => {
              // Restore + focus the main window from the tray.
              api.showWindow()
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

        {/* Command Palette Modal */}
        <CommandPalette
          isOpen={isCommandPaletteOpen}
          onClose={() => setIsCommandPaletteOpen(false)}
          onNavigate={tab => setActiveTab(tab as NavTab)}
          onOpenModal={handleOpenModal}
          isConnected={isConnected}
          isConnecting={isConnecting}
          onToggleConnect={handleToggleConnect}
          onPanicWipe={handlePanicWipe}
          onCycleTheme={handleCycleTheme}
          onToggleLang={handleToggleLang}
          proxyMode={settings.proxy_mode || 'tun'}
          onToggleProxyMode={handleToggleProxyMode}
        />

        {/* Global Right-Click Context Menu */}
        <GlobalContextMenu />

        {/* App-wide ARIA live regions (screen-reader announcements) */}
        <AriaLive />
      </div>
    </TooltipProvider>
  )
}
