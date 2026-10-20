import React, { useState, useEffect, useRef, useMemo } from 'react'
import {
  Search,
  Zap,
  Power,
  Layers,
  Palette,
  Globe,
  AlertTriangle,
  LayoutDashboard,
  Radio,
  Shield,
  Lock,
  Cpu,
  Flame,
  Compass,
  Settings as SettingsIcon,
  Terminal,
  ShieldCheck,
  Activity,
  Gauge,
  Scan,
  Share2,
  Sparkles,
  ArrowRightLeft,
  X,
  CornerDownLeft,
  type LucideIcon,
} from 'lucide-react'
import { t } from '../lib/i18n'
import { ModalShell } from '../components/ui/modal-shell'

export interface CommandItem {
  id: string
  title: string
  subtitle?: string
  group: 'actions' | 'navigation' | 'tools'
  icon: LucideIcon
  shortcut?: string
  keywords?: string[]
  onSelect: () => void
}

interface Props {
  isOpen: boolean
  onClose: () => void
  onNavigate: (tab: string) => void
  onOpenModal: (modalName: string) => void
  isConnected: boolean
  isConnecting: boolean
  onToggleConnect: () => void
  onPanicWipe: () => void
  onCycleTheme: () => void
  onToggleLang: () => void
  proxyMode: string
  onToggleProxyMode: () => void
}

export default function CommandPalette({
  isOpen,
  onClose,
  onNavigate,
  onOpenModal,
  isConnected,
  isConnecting,
  onToggleConnect,
  onPanicWipe,
  onCycleTheme,
  onToggleLang,
  proxyMode,
  onToggleProxyMode,
}: Props) {
  const [query, setQuery] = useState('')
  const [selectedIndex, setSelectedIndex] = useState(0)
  const inputRef = useRef<HTMLInputElement>(null)
  const listRef = useRef<HTMLDivElement>(null)

  // Focus input when opened
  useEffect(() => {
    if (isOpen) {
      setQuery('')
      setSelectedIndex(0)
      setTimeout(() => {
        inputRef.current?.focus()
      }, 50)
    }
  }, [isOpen])

  // Build command items list
  const allCommands = useMemo<CommandItem[]>(() => {
    return [
      // ─── Quick Actions ───
      {
        id: 'action-connect',
        title: isConnected ? t('disconnect') : t('connect'),
        subtitle: isConnected ? t('click_to_disconnect') : t('cmd_connect_fastest'),
        group: 'actions',
        icon: Power,
        shortcut: 'Ctrl+Shift+V',
        keywords: ['connect', 'disconnect', 'power', 'on', 'off', 'وصل', 'قطع'],
        onSelect: () => {
          onToggleConnect()
          onClose()
        },
      },
      {
        id: 'action-proxy-mode',
        title: t('cmd_toggle_proxy_mode'),
        subtitle: proxyMode === 'tun' ? t('mode_switch_proxy') : t('mode_switch_tun'),
        group: 'actions',
        icon: Layers,
        keywords: ['mode', 'tun', 'system proxy', 'پروکسی', 'تانل'],
        onSelect: () => {
          onToggleProxyMode()
          onClose()
        },
      },
      {
        id: 'action-panic-wipe',
        title: t('cmd_panic_wipe'),
        subtitle: t('panic_wipe_confirm'),
        group: 'actions',
        icon: AlertTriangle,
        keywords: ['panic', 'wipe', 'emergency', 'kill', 'پاکسازی', 'اضطراری'],
        onSelect: () => {
          onPanicWipe()
          onClose()
        },
      },
      {
        id: 'action-theme',
        title: t('cmd_cycle_theme'),
        subtitle: t('theme_label'),
        group: 'actions',
        icon: Palette,
        keywords: ['theme', 'dark', 'light', 'cyber', 'رنگ', 'تم'],
        onSelect: () => {
          onCycleTheme()
          onClose()
        },
      },
      {
        id: 'action-lang',
        title: t('cmd_toggle_lang'),
        subtitle: t('lang_btn_text'),
        group: 'actions',
        icon: Globe,
        keywords: ['language', 'english', 'persian', 'فارسی', 'انگلیسی', 'زبان'],
        onSelect: () => {
          onToggleLang()
          onClose()
        },
      },

      // ─── Navigation ───
      {
        id: 'nav-dashboard',
        title: t('nav_dashboard'),
        subtitle: t('dashboard_subtitle'),
        group: 'navigation',
        icon: LayoutDashboard,
        keywords: ['home', 'dashboard', 'داشبورد', 'خانه'],
        onSelect: () => {
          onNavigate('dashboard')
          onClose()
        },
      },
      {
        id: 'nav-v2ray',
        title: t('nav_v2ray'),
        subtitle: 'VLESS, VMess, Trojan, Shadowsocks, Hysteria 2',
        group: 'navigation',
        icon: Radio,
        keywords: ['v2ray', 'xray', 'vless', 'vmess', 'trojan', 'hy2', 'پروکسی'],
        onSelect: () => {
          onNavigate('v2ray')
          onClose()
        },
      },
      {
        id: 'nav-warp',
        title: t('nav_warp_wireguard'),
        subtitle: 'Cloudflare WARP, Clean IP & AmneziaWG',
        group: 'navigation',
        icon: Shield,
        keywords: ['warp', 'wireguard', 'amnezia', 'cloudflare', 'وارپ', 'وایرگارد'],
        onSelect: () => {
          onNavigate('warp')
          onClose()
        },
      },
      {
        id: 'nav-openvpn',
        title: t('nav_openvpn'),
        subtitle: 'TCP/UDP Custom Ovpn Configs',
        group: 'navigation',
        icon: Lock,
        keywords: ['openvpn', 'ovpn', 'اوپن وی پی ان'],
        onSelect: () => {
          onNavigate('openvpn')
          onClose()
        },
      },
      {
        id: 'nav-aether',
        title: 'Aether Engine',
        subtitle: 'Multi-Protocol Resilience Proxy',
        group: 'navigation',
        icon: Cpu,
        keywords: ['aether', 'engine', 'اثر'],
        onSelect: () => {
          onNavigate('aether')
          onClose()
        },
      },
      {
        id: 'nav-psiphon',
        title: t('nav_psiphon'),
        subtitle: 'Circumvention Mesh Tunnel',
        group: 'navigation',
        icon: Flame,
        keywords: ['psiphon', 'emergency', 'سایفون'],
        onSelect: () => {
          onNavigate('psiphon')
          onClose()
        },
      },
      {
        id: 'nav-discovery',
        title: t('nav_discovery'),
        subtitle: 'Telegram Proxies, Free Configs, Scanner',
        group: 'navigation',
        icon: Compass,
        keywords: ['discovery', 'free', 'telegram', 'کانفیگ رایگان', 'تلگرام'],
        onSelect: () => {
          onNavigate('discovery')
          onClose()
        },
      },
      {
        id: 'nav-settings',
        title: t('nav_settings'),
        subtitle: 'Core, Routing, DNS, Fragment & Startup',
        group: 'navigation',
        icon: SettingsIcon,
        keywords: ['settings', 'config', 'تنظیمات', 'پیکربندی'],
        onSelect: () => {
          onNavigate('settings')
          onClose()
        },
      },
      {
        id: 'nav-logs',
        title: t('nav_logs'),
        subtitle: 'Live Console & Connection Diagnostics',
        group: 'navigation',
        icon: Terminal,
        keywords: ['logs', 'console', 'debug', 'لاگ'],
        onSelect: () => {
          onNavigate('logs')
          onClose()
        },
      },

      // ─── Tools & Diagnostics ───
      {
        id: 'tool-doctor',
        title: t('nav_doctor'),
        subtitle: t('doctor'),
        group: 'tools',
        icon: ShieldCheck,
        keywords: ['doctor', 'diagnostics', 'health', 'پزشک', 'عیب‌یابی'],
        onSelect: () => {
          onOpenModal('doctor')
          onClose()
        },
      },
      {
        id: 'tool-speedtest',
        title: t('speed_test_btn'),
        subtitle: 'Live Latency, Download & Upload Test',
        group: 'tools',
        icon: Gauge,
        keywords: ['speed', 'test', 'bandwidth', 'تست سرعت'],
        onSelect: () => {
          onOpenModal('speedtest')
          onClose()
        },
      },
      {
        id: 'tool-dns-leak',
        title: t('dns_leak'),
        subtitle: 'Verify Encrypted DNS Shielding',
        group: 'tools',
        icon: Shield,
        keywords: ['dns', 'leak', 'نشت', 'امنیت'],
        onSelect: () => {
          onOpenModal('dnsleak')
          onClose()
        },
      },
      {
        id: 'tool-fragment',
        title: t('anti_dpi'),
        subtitle: 'TLS Fragment, SNI Spoofing & Anti-DPI Studio',
        group: 'tools',
        icon: Sparkles,
        keywords: ['fragment', 'dpi', 'anti-dpi', 'فیلترینگ', 'فرگمنت'],
        onSelect: () => {
          onOpenModal('fragment')
          onClose()
        },
      },
      {
        id: 'tool-service-unlock',
        title: t('service_unblock_btn') || 'AI & Service Unlock Matrix',
        subtitle: 'Check ChatGPT, Claude, Spotify, Netflix accessibility',
        group: 'tools',
        icon: Activity,
        keywords: ['ai', 'chatgpt', 'claude', 'spotify', 'unlock', 'هوش مصنوعی'],
        onSelect: () => {
          onOpenModal('serviceunlock')
          onClose()
        },
      },
      {
        id: 'tool-screen-qr',
        title: t('screen_qr_title') || 'Scan QR Code from Screen',
        subtitle: 'Import VPN Config from any Window or Browser',
        group: 'tools',
        icon: Scan,
        keywords: ['qr', 'scan', 'اسکن', 'کیوآر'],
        onSelect: () => {
          onOpenModal('screenqr')
          onClose()
        },
      },
      {
        id: 'tool-lan-share',
        title: t('wifi_share'),
        subtitle: 'Share Protected VPN Proxy over Local Wi-Fi',
        group: 'tools',
        icon: Share2,
        keywords: ['lan', 'wifi', 'share', 'اشتراک'],
        onSelect: () => {
          onOpenModal('lanshare')
          onClose()
        },
      },
      {
        id: 'tool-converter',
        title: t('config_converter_title') || 'Config Format Converter',
        subtitle: 'Convert between V2Ray, Clash, Sing-box formats',
        group: 'tools',
        icon: ArrowRightLeft,
        keywords: ['converter', 'clash', 'singbox', 'مبدل'],
        onSelect: () => {
          onOpenModal('converter')
          onClose()
        },
      },
      {
        id: 'tool-repair',
        title: t('emergency_repair'),
        subtitle: 'Flush DNS, Reset Winsock, Clear Stale Proxies',
        group: 'tools',
        icon: Zap,
        keywords: ['repair', 'fix', 'reset', 'تعمیر'],
        onSelect: () => {
          onOpenModal('repair')
          onClose()
        },
      },
    ]
  }, [
    isConnected,
    isConnecting,
    proxyMode,
    onToggleConnect,
    onToggleProxyMode,
    onPanicWipe,
    onCycleTheme,
    onToggleLang,
    onNavigate,
    onOpenModal,
    onClose,
  ])

  // Filter commands by user query
  const filteredCommands = useMemo(() => {
    if (!query.trim()) return allCommands
    const q = query.toLowerCase().trim()
    return allCommands.filter(cmd => {
      const matchTitle = cmd.title.toLowerCase().includes(q)
      const matchSubtitle = cmd.subtitle?.toLowerCase().includes(q)
      const matchKeywords = cmd.keywords?.some(k => k.toLowerCase().includes(q))
      return matchTitle || matchSubtitle || matchKeywords
    })
  }, [allCommands, query])

  // Reset selected index if out of bounds
  useEffect(() => {
    if (selectedIndex >= filteredCommands.length) {
      setSelectedIndex(0)
    }
  }, [filteredCommands.length, selectedIndex])

  // Handle keyboard navigation
  const handleKeyDown = (e: React.KeyboardEvent) => {
    if (e.key === 'ArrowDown') {
      e.preventDefault()
      setSelectedIndex(prev => (prev + 1) % Math.max(1, filteredCommands.length))
    } else if (e.key === 'ArrowUp') {
      e.preventDefault()
      setSelectedIndex(prev => (prev - 1 + filteredCommands.length) % Math.max(1, filteredCommands.length))
    } else if (e.key === 'Enter') {
      e.preventDefault()
      if (filteredCommands[selectedIndex]) {
        filteredCommands[selectedIndex].onSelect()
      }
    } else if (e.key === 'Escape') {
      e.preventDefault()
      onClose()
    }
  }

  // Scroll active item into view
  useEffect(() => {
    const listEl = listRef.current
    if (!listEl) return
    const activeEl = listEl.querySelector(`[data-index="${selectedIndex}"]`) as HTMLElement
    if (activeEl) {
      activeEl.scrollIntoView({ block: 'nearest' })
    }
  }, [selectedIndex])

  if (!isOpen) return null

  return (
    <ModalShell
      isOpen={isOpen}
      onClose={onClose}
      maxWidth="max-w-2xl"
      className="border-border max-h-[75vh] animate-in zoom-in-95 duration-150 backdrop-blur-xl ring-1 ring-white/10"
    >
        {/* Search Header */}
        <div className="flex items-center gap-3 px-4 py-3.5 border-b border-border bg-background">
          <Search size={18} className="text-primary shrink-0" />
          <input
            ref={inputRef}
            type="text"
            value={query}
            onChange={e => {
              setQuery(e.target.value)
              setSelectedIndex(0)
            }}
            placeholder={t('command_palette_placeholder')}
            onKeyDown={handleKeyDown}
            className="w-full bg-transparent border-none outline-none text-foreground placeholder:text-muted-foreground text-sm font-medium"
          />
          {query && (
            <button
              onClick={() => {
                setQuery('')
                inputRef.current?.focus()
              }}
              className="p-1 rounded-lg text-muted-foreground hover:text-foreground hover:bg-muted transition"
            >
              <X size={15} />
            </button>
          )}
          <kbd className="hidden sm:inline-flex items-center gap-0.5 px-2 py-0.5 text-[10px] font-mono font-semibold text-muted-foreground bg-muted border border-border rounded-md">
            ESC
          </kbd>
        </div>

        {/* Commands List */}
        <div ref={listRef} className="flex-1 overflow-y-auto p-2 space-y-1">
          {filteredCommands.length === 0 ? (
            <div className="py-12 text-center text-muted-foreground text-sm">
              <Search size={28} className="mx-auto text-muted-foreground mb-2" />
              <span>{t('command_palette_no_results')}</span>
            </div>
          ) : (
            filteredCommands.map((cmd, index) => {
              const Icon = cmd.icon
              const isSelected = index === selectedIndex
              return (
                <div
                  key={cmd.id}
                  data-index={index}
                  onClick={() => cmd.onSelect()}
                  onMouseEnter={() => setSelectedIndex(index)}
                  className={`flex items-center justify-between gap-3 px-3.5 py-2.5 rounded-xl cursor-pointer transition text-xs select-none ${
                    isSelected
                      ? 'bg-primary/10 border border-primary/40 text-foreground shadow-sm'
                      : 'text-muted-foreground hover:bg-muted border border-transparent'
                  }`}
                >
                  <div className="flex items-center gap-3 min-w-0">
                    <div
                      className={`p-2 rounded-lg shrink-0 transition ${
                        isSelected
                          ? 'bg-primary/20 text-primary ring-1 ring-primary/30'
                          : 'bg-muted text-muted-foreground'
                      }`}
                    >
                      <Icon size={16} />
                    </div>
                    <div className="min-w-0">
                      <div className="font-semibold text-foreground truncate flex items-center gap-2">
                        <span>{cmd.title}</span>
                        {cmd.group === 'actions' && (
                          <span className="text-[9px] font-mono px-1.5 py-0.5 rounded bg-warning/10 text-warning border border-warning/20">
                            Action
                          </span>
                        )}
                      </div>
                      {cmd.subtitle && (
                        <div className="text-[11px] text-muted-foreground truncate mt-0.5">
                          {cmd.subtitle}
                        </div>
                      )}
                    </div>
                  </div>

                  <div className="flex items-center gap-2 shrink-0">
                    {cmd.shortcut && (
                      <kbd className="hidden sm:inline-flex items-center px-1.5 py-0.5 text-[10px] font-mono text-muted-foreground bg-muted border border-border rounded">
                        {cmd.shortcut}
                      </kbd>
                    )}
                    {isSelected && (
                      <CornerDownLeft size={14} className="text-primary animate-pulse" />
                    )}
                  </div>
                </div>
              )
            })
          )}
        </div>

        {/* Footer Hint */}
        <div className="px-4 py-2 border-t border-border bg-background flex items-center justify-between text-[11px] text-muted-foreground">
          <div className="flex items-center gap-3">
            <span>{t('command_palette_hint')}</span>
          </div>
          <div className="flex items-center gap-1.5 font-mono text-[10px]">
            <span className="w-2 h-2 rounded-full bg-info" />
            <span>NexVPN Command Matrix</span>
          </div>
        </div>
      </ModalShell>
  )
}