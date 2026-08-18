import React, { useState } from 'react'
import {
  Wifi,
  Radio,
  Download,
  Share2,
  CheckCircle2,
  AlertCircle,
  RefreshCw,
  X,
  Sparkles,
  Layers,
  ArrowRight,
} from 'lucide-react'
import { api } from '../lib/api'
import { sound } from '../lib/sound'
import { t } from '../lib/i18n'
import type { Profile } from '../types'

interface Props {
  isOpen: boolean
  onClose: () => void
  onProfilesImported?: () => void
}

export default function NearbySyncModal({ isOpen, onClose, onProfilesImported }: Props) {
  const [activeTab, setActiveTab] = useState<'send' | 'receive'>('send')
  const [isBroadcasting, setIsBroadcasting] = useState(false)
  const [isListening, setIsListening] = useState(false)
  const [statusMsg, setStatusMsg] = useState<string | null>(null)
  const [receivedPayload, setReceivedPayload] = useState<any | null>(null)
  const [isImporting, setIsImporting] = useState(false)

  if (!isOpen) return null

  const handleBroadcast = async () => {
    setIsBroadcasting(true)
    setStatusMsg(null)
    try {
      const count = await (api as any).broadcastProfilesToLAN('all', 'NexVPN Device')
      sound.playSuccess()
      setStatusMsg(`Broadcasted ${count} profiles to local network successfully!`)
    } catch (err: any) {
      setStatusMsg('Broadcast failed: ' + (err?.message || 'Error'))
    } finally {
      setIsBroadcasting(false)
    }
  }

  const handleStartListening = async () => {
    setIsListening(true)
    setStatusMsg(null)
    setReceivedPayload(null)
    try {
      const payload = await (api as any).listenForNearbyProfiles(10)
      if (payload && payload.profiles && payload.profiles.length > 0) {
        setReceivedPayload(payload)
        sound.playSuccess()
        setStatusMsg(`Discovered ${payload.profiles.length} profiles from ${payload.device_name || 'Nearby Device'}!`)
      } else {
        setStatusMsg('No nearby broadcast detected. Make sure sender is on the same Wi-Fi.')
      }
    } catch (err: any) {
      setStatusMsg('Discovery timeout: ' + (err?.message || 'No devices found'))
    } finally {
      setIsListening(false)
    }
  }

  const handleImportReceived = async () => {
    if (!receivedPayload || !receivedPayload.profiles) return
    setIsImporting(true)
    try {
      const count = await (api as any).importNearbyProfiles(receivedPayload.profiles, `Nearby (${receivedPayload.device_name})`)
      sound.playSuccess()
      if (onProfilesImported) onProfilesImported()
      setStatusMsg(`Successfully imported ${count} profiles!`)
      setTimeout(() => {
        onClose()
      }, 1500)
    } catch (err: any) {
      setStatusMsg('Import failed: ' + (err?.message || 'Error'))
    } finally {
      setIsImporting(false)
    }
  }

  return (
    <div className="fixed inset-0 z-50 flex items-center justify-center p-4 bg-black/80 backdrop-blur-md animate-fade-in">
      <div className="relative w-full max-w-lg bg-slate-900 border border-slate-700/80 rounded-3xl shadow-2xl overflow-hidden flex flex-col max-h-[90vh]">
        {/* Header */}
        <div className="flex items-center justify-between p-5 border-b border-slate-800 bg-slate-950/40">
          <div className="flex items-center gap-3">
            <div className="w-10 h-10 rounded-2xl bg-cyan-500/20 border border-cyan-500/30 flex items-center justify-center text-cyan-400">
              <Radio className="w-5 h-5" />
            </div>
            <div>
              <h3 className="text-base font-bold text-white flex items-center gap-2">
                <span>{t('nearby_share_title') || 'Nearby Wi-Fi Share'}</span>
                <span className="px-2 py-0.5 text-[10px] font-bold rounded-full bg-cyan-500/20 text-cyan-300 border border-cyan-500/30">
                  Local P2P
                </span>
              </h3>
              <p className="text-xs text-slate-400">
                {t('nearby_share_subtitle') || 'Transfer VPN nodes between computers on the same Wi-Fi network'}
              </p>
            </div>
          </div>
          <button
            onClick={onClose}
            className="p-1.5 rounded-xl bg-slate-800/60 hover:bg-slate-700 text-slate-400 hover:text-white transition-all"
          >
            <X className="w-4 h-4" />
          </button>
        </div>

        {/* Tab Switcher */}
        <div className="p-4 bg-slate-950/40 border-b border-slate-800 flex items-center gap-2">
          <button
            type="button"
            onClick={() => setActiveTab('send')}
            className={`flex-1 py-2 rounded-xl text-xs font-bold transition flex items-center justify-center gap-2 ${
              activeTab === 'send'
                ? 'bg-cyan-500 text-slate-950 shadow-md shadow-cyan-500/20'
                : 'text-slate-400 hover:text-slate-200 hover:bg-slate-800/40'
            }`}
          >
            <Share2 className="w-4 h-4" />
            <span>{t('send_profiles_tab') || 'Send to Nearby'}</span>
          </button>

          <button
            type="button"
            onClick={() => setActiveTab('receive')}
            className={`flex-1 py-2 rounded-xl text-xs font-bold transition flex items-center justify-center gap-2 ${
              activeTab === 'receive'
                ? 'bg-purple-500 text-white shadow-md shadow-purple-500/20'
                : 'text-slate-400 hover:text-slate-200 hover:bg-slate-800/40'
            }`}
          >
            <Download className="w-4 h-4" />
            <span>{t('receive_profiles_tab') || 'Receive Profiles'}</span>
          </button>
        </div>

        {/* Body */}
        <div className="p-5 space-y-4 overflow-y-auto flex-1">
          {activeTab === 'send' ? (
            <div className="space-y-4 text-center">
              <div className="w-16 h-16 rounded-3xl bg-cyan-500/10 border border-cyan-500/20 flex items-center justify-center text-cyan-400 mx-auto">
                <Wifi className="w-8 h-8 animate-pulse" />
              </div>
              <div className="space-y-1">
                <h4 className="text-sm font-bold text-slate-200">
                  {t('broadcast_nodes_title') || 'Broadcast Profiles over Local Wi-Fi'}
                </h4>
                <p className="text-xs text-slate-400 leading-relaxed max-w-sm mx-auto">
                  {t('broadcast_nodes_desc') ||
                    'Send your active VPN configurations directly to other computers running NexVPN on the same Wi-Fi router.'}
                </p>
              </div>

              <button
                type="button"
                onClick={handleBroadcast}
                disabled={isBroadcasting}
                className="w-full py-3 px-4 bg-gradient-to-r from-cyan-600 to-blue-600 hover:from-cyan-500 hover:to-blue-500 disabled:opacity-50 text-white rounded-2xl text-xs font-bold shadow-lg shadow-cyan-600/20 flex items-center justify-center gap-2 active:scale-95 transition-all"
              >
                {isBroadcasting ? (
                  <RefreshCw className="w-4 h-4 animate-spin" />
                ) : (
                  <Share2 className="w-4 h-4" />
                )}
                <span>{isBroadcasting ? 'Broadcasting...' : t('broadcast_now_btn') || 'Broadcast Profiles Now'}</span>
              </button>
            </div>
          ) : (
            <div className="space-y-4 text-center">
              <div className="w-16 h-16 rounded-3xl bg-purple-500/10 border border-purple-500/20 flex items-center justify-center text-purple-400 mx-auto">
                <Radio className={`w-8 h-8 ${isListening ? 'animate-spin' : ''}`} />
              </div>
              <div className="space-y-1">
                <h4 className="text-sm font-bold text-slate-200">
                  {t('listen_nodes_title') || 'Discover Incoming Node Shares'}
                </h4>
                <p className="text-xs text-slate-400 leading-relaxed max-w-sm mx-auto">
                  {t('listen_nodes_desc') ||
                    'Listen for incoming broadcasts from other devices on your local network.'}
                </p>
              </div>

              <button
                type="button"
                onClick={handleStartListening}
                disabled={isListening}
                className="w-full py-3 px-4 bg-gradient-to-r from-purple-600 to-indigo-600 hover:from-purple-500 hover:to-indigo-500 disabled:opacity-50 text-white rounded-2xl text-xs font-bold shadow-lg shadow-purple-600/20 flex items-center justify-center gap-2 active:scale-95 transition-all"
              >
                {isListening ? (
                  <RefreshCw className="w-4 h-4 animate-spin" />
                ) : (
                  <Radio className="w-4 h-4" />
                )}
                <span>{isListening ? 'Searching on Wi-Fi...' : t('start_discovery_btn') || 'Search for Nearby Shares'}</span>
              </button>

              {receivedPayload && (
                <div className="p-4 bg-slate-950/80 border border-slate-800 rounded-2xl space-y-3 text-left rtl:text-right animate-in fade-in duration-200">
                  <div className="flex items-center justify-between text-xs">
                    <span className="font-bold text-emerald-400 flex items-center gap-1.5">
                      <CheckCircle2 className="w-4 h-4" />
                      <span>{receivedPayload.profiles?.length} Nodes Discovered</span>
                    </span>
                    <span className="text-slate-400 font-mono">{receivedPayload.device_name}</span>
                  </div>

                  <button
                    type="button"
                    onClick={handleImportReceived}
                    disabled={isImporting}
                    className="w-full py-2 px-3 bg-emerald-600 hover:bg-emerald-500 text-white rounded-xl text-xs font-bold flex items-center justify-center gap-2 transition"
                  >
                    <Download className="w-3.5 h-3.5" />
                    <span>{isImporting ? 'Importing...' : 'Import to My Profiles'}</span>
                  </button>
                </div>
              )}
            </div>
          )}

          {/* Status Message */}
          {statusMsg && (
            <div className="p-3 bg-slate-950/60 border border-slate-800 rounded-xl text-xs text-slate-300 text-center">
              {statusMsg}
            </div>
          )}
        </div>

        {/* Footer */}
        <div className="flex items-center justify-end p-4 border-t border-slate-800 bg-slate-950/40">
          <button
            type="button"
            onClick={onClose}
            className="px-4 py-2 bg-slate-800 hover:bg-slate-700 text-slate-300 rounded-xl text-xs font-semibold"
          >
            {t('close')}
          </button>
        </div>
      </div>
    </div>
  )
}
