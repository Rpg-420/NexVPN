import React, { useState, useEffect } from 'react'
import {
  Gamepad2,
  Zap,
  CheckCircle2,
  X,
  Flame,
} from 'lucide-react'
import { api } from '../lib/api'
import { sound } from '../lib/sound'
import { t } from '../lib/i18n'
import type { Settings } from '../types'

interface Props {
  isOpen: boolean
  onClose: () => void
  settings: Settings
  onSettingsUpdated: (updated: Settings) => void
}

interface DNSOption {
  id: string
  name: string
  provider: string
  primary: string
  secondary: string
  dohUrl?: string
  description: string
  latencyBadge: string
}

const GAMING_DNS_LIST: DNSOption[] = [
  {
    id: 'electro',
    name: 'Electro DNS (الکترو)',
    provider: 'ElectroTM Iran',
    primary: '78.157.42.100',
    secondary: '78.157.42.101',
    dohUrl: 'https://dns.electrotm.org/dns-query',
    description: 'Specialized for PlayStation, Xbox, Steam, Epic Games & Discord bypass with zero latency.',
    latencyBadge: '⚡ Lowest Latency',
  },
  {
    id: 'shecan',
    name: 'Shecan (شکن)',
    provider: 'Shecan IR',
    primary: '178.22.122.100',
    secondary: '185.51.200.2',
    dohUrl: 'https://free.shecan.ir/dns-query',
    description: 'High-speed anti-sanction DNS for developer tools, Adobe, Docker, and gaming servers.',
    latencyBadge: '🌟 Anti-Sanction',
  },
  {
    id: 'begzar',
    name: 'Begzar (بگذر)',
    provider: 'Begzar Team',
    primary: '185.55.226.26',
    secondary: '185.55.225.25',
    description: 'Optimized routing for EA Play, Riot Games, Valorant, and Battle.net.',
    latencyBadge: '🎮 Riot / EA Focus',
  },
  {
    id: 'cloudflare_gaming',
    name: 'Cloudflare Zero Trust Gaming',
    provider: 'Cloudflare 1.1.1.1',
    primary: '1.1.1.1',
    secondary: '1.0.0.1',
    dohUrl: 'https://1.1.1.1/dns-query',
    description: 'Global Tier-1 Anycast DNS with sub-millisecond route resolution and DDoS protection.',
    latencyBadge: '🚀 Global Fast',
  },
]

export default function GamingModeModal({ isOpen, onClose, settings, onSettingsUpdated }: Props) {
  const [gameMode, setGameMode] = useState<boolean>(settings?.game_mode_enabled || false)
  const [selectedDNS, setSelectedDNS] = useState<string>('electro')
  const [isSaving, setIsSaving] = useState(false)
  const [successMsg, setSuccessMsg] = useState<string | null>(null)

  useEffect(() => {
    if (isOpen) {
      api.getSettings().then(s => {
        if (s) {
          setGameMode(s.game_mode_enabled || false)
          if (s.dns_server) {
            const found = GAMING_DNS_LIST.find(
              d => d.dohUrl === s.dns_server || d.primary === s.dns_server
            )
            if (found) setSelectedDNS(found.id)
          }
        }
      }).catch(err => console.error('Failed to load gaming settings:', err))
    }
  }, [isOpen])

  if (!isOpen) return null

  const handleSave = async () => {
    setIsSaving(true)
    setSuccessMsg(null)
    try {
      const current = await api.getSettings()
      const chosenDNS = GAMING_DNS_LIST.find(d => d.id === selectedDNS)
      const updated: Settings = {
        ...current,
        game_mode_enabled: gameMode,
        dns_server: chosenDNS?.dohUrl || chosenDNS?.primary || current.dns_server,
        dns_fallback: chosenDNS?.secondary || current.dns_fallback,
      }
      await api.saveSettings(updated)
      onSettingsUpdated(updated)
      setSuccessMsg(t('gaming_success'))
      sound.playSuccess()
      setTimeout(() => {
        setSuccessMsg(null)
        onClose()
      }, 1200)
    } catch (err) {
      console.error(err)
    } finally {
      setIsSaving(false)
    }
  }

  return (
    <div className="fixed inset-0 z-50 flex items-center justify-center p-4 bg-black/75 backdrop-blur-md animate-fade-in">
      <div className="relative w-full max-w-2xl bg-neutral-900 border border-neutral-800 rounded-3xl shadow-2xl overflow-hidden flex flex-col max-h-[90vh]">
        {/* Header */}
        <div className="flex items-center justify-between p-6 border-b border-neutral-800/80 bg-neutral-950/40">
          <div className="flex items-center gap-3">
            <div className="w-10 h-10 rounded-2xl bg-gradient-to-tr from-emerald-500/20 to-teal-500/20 border border-emerald-500/30 flex items-center justify-center text-emerald-400">
              <Gamepad2 className="w-5 h-5" />
            </div>
            <div>
              <h3 className="text-lg font-bold text-white flex items-center gap-2">
                {t('gaming_modal_title')}
                <span className="px-2 py-0.5 text-xs font-semibold rounded-full bg-emerald-500/20 text-emerald-300 border border-emerald-500/30 flex items-center gap-1">
                  <Flame className="w-3 h-3 text-emerald-400 animate-pulse" /> {t('gaming_badge')}
                </span>
              </h3>
              <p className="text-xs text-neutral-400">{t('gaming_modal_sub')}</p>
            </div>
          </div>
          <button
            onClick={onClose}
            className="p-2 rounded-xl bg-neutral-800/60 hover:bg-neutral-700 text-neutral-400 hover:text-white transition-all"
          >
            <X className="w-4 h-4" />
          </button>
        </div>

        {/* Body */}
        <div className="p-6 overflow-y-auto space-y-6 flex-1">
          {/* Game Mode Master Switch */}
          <div className="flex items-center justify-between p-4 rounded-2xl bg-neutral-950/50 border border-neutral-800">
            <div className="space-y-0.5">
              <div className="text-sm font-bold text-white flex items-center gap-2">
                {t('gaming_enable_title')}
              </div>
              <div className="text-xs text-neutral-400">
                {t('gaming_enable_sub')}
              </div>
            </div>
            <button
              onClick={() => setGameMode(!gameMode)}
              className={`relative inline-flex h-6 w-11 items-center rounded-full transition-colors ${
                gameMode ? 'bg-emerald-500' : 'bg-neutral-700'
              }`}
            >
              <span
                className={`inline-block h-4 w-4 transform rounded-full bg-white transition-transform ${
                  gameMode ? 'translate-x-6' : 'translate-x-1'
                }`}
              />
            </button>
          </div>

          {/* DNS Presets */}
          <div className="space-y-3">
            <div className="text-xs font-bold text-neutral-400 uppercase tracking-wider">
              {t('gaming_resolvers_title')}
            </div>
            <div className="grid grid-cols-1 md:grid-cols-2 gap-3">
              {GAMING_DNS_LIST.map(dns => (
                <div
                  key={dns.id}
                  onClick={() => setSelectedDNS(dns.id)}
                  className={`p-4 rounded-2xl border transition-all cursor-pointer flex flex-col justify-between ${
                    selectedDNS === dns.id
                      ? 'bg-emerald-500/10 border-emerald-500/50 shadow-lg shadow-emerald-500/10'
                      : 'bg-neutral-800/40 border-neutral-800 hover:bg-neutral-800/80 hover:border-neutral-700'
                  }`}
                >
                  <div className="space-y-1.5">
                    <div className="flex items-center justify-between">
                      <span className="text-[10px] font-bold px-2 py-0.5 rounded-full bg-neutral-800 text-neutral-300 border border-neutral-700">
                        {dns.latencyBadge}
                      </span>
                      {selectedDNS === dns.id && <CheckCircle2 className="w-4 h-4 text-emerald-400" />}
                    </div>
                    <div className="text-xs font-bold text-white pt-1">{dns.name}</div>
                    <p className="text-[11px] text-neutral-400 leading-relaxed">{dns.description}</p>
                  </div>
                  <div className="mt-3 pt-2 border-t border-neutral-800/60 text-[10px] font-mono text-emerald-300 flex justify-between">
                    <span>IP: {dns.primary}</span>
                    <span>Secondary: {dns.secondary}</span>
                  </div>
                </div>
              ))}
            </div>
          </div>

          {/* Features Highlights */}
          <div className="p-4 rounded-2xl bg-neutral-950/40 border border-neutral-800 space-y-2 text-xs text-neutral-300">
            <div className="font-bold text-neutral-200 flex items-center gap-1.5">
              <Zap className="w-4 h-4 text-emerald-400" /> Active Gaming Optimizations:
            </div>
            <ul className="list-disc list-inside space-y-1 text-neutral-400 pl-1 text-[11px]">
              <li><strong className="text-neutral-200">UDP Packet Prioritization:</strong> Zero queue buffering for Discord voice & online matchmaking.</li>
              <li><strong className="text-neutral-200">Anti-Sanction Unblocking:</strong> Full access to Steam, Epic Games, PlayStation Network & EA Play.</li>
              <li><strong className="text-neutral-200">MSS Clamping (1360):</strong> Prevents fragmentation packet drops on Mobile 4G/5G connections.</li>
            </ul>
          </div>

          {successMsg && (
            <div className="p-3 rounded-xl bg-emerald-500/10 border border-emerald-500/20 text-emerald-400 text-xs flex items-center gap-2">
              <CheckCircle2 className="w-4 h-4" />
              <span>{successMsg}</span>
            </div>
          )}
        </div>

        {/* Footer */}
        <div className="flex items-center justify-end gap-3 p-5 border-t border-neutral-800/80 bg-neutral-950/40">
          <button
            onClick={onClose}
            className="px-4 py-2 rounded-xl text-xs font-semibold text-neutral-400 hover:text-white transition"
          >
            {t('cancel')}
          </button>
          <button
            onClick={handleSave}
            disabled={isSaving}
            className="px-6 py-2.5 rounded-xl bg-gradient-to-r from-emerald-500 to-teal-500 text-neutral-950 text-xs font-bold shadow-lg shadow-emerald-500/20 hover:opacity-95 transition disabled:opacity-50"
          >
            {isSaving ? t('fragment_applying') : t('gaming_save_btn')}
          </button>
        </div>
      </div>
    </div>
  )
}
