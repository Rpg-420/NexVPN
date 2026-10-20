import { useState, useEffect } from 'react'
import {
  Wifi,
  Radio,
  Download,
  Share2,
  CheckCircle2,
  RefreshCw,
  X,
} from 'lucide-react'
import { api } from '../lib/api'
import { sound } from '../lib/sound'
import { t } from '../lib/i18n'
import { ModalShell } from '../components/ui/modal-shell'

interface Props {
  isOpen: boolean
  onClose: () => void
  onProfilesImported?: () => void
}

export default function NearbySyncModal({ isOpen, onClose, onProfilesImported }: Props) {
  const [activeTab, setActiveTab] = useState<'send' | 'receive'>('send')
  const [isBroadcasting, setIsBroadcasting] = useState(false)
  const [isListening, setIsListening] = useState(false)
  const [pin, setPin] = useState('')
  const [statusMsg, setStatusMsg] = useState<string | null>(null)
  const [receivedPayload, setReceivedPayload] = useState<any | null>(null)
  const [isImporting, setIsImporting] = useState(false)

  // A PIN is mandatory: without one the payload would be encrypted with a publicly
  // known key and any device on the LAN could read the shared proxy credentials.
  // Pre-fill a fresh random PIN whenever the modal is opened.
  useEffect(() => {
    if (!isOpen) return
    let cancelled = false
    api
      .generateSyncPIN()
      .then(generated => {
        if (!cancelled && generated) setPin(generated)
      })
      .catch(err => {
        console.warn('[NearbySync] failed to generate a pairing PIN:', err)
      })
    return () => {
      cancelled = true
    }
  }, [isOpen])

  if (!isOpen) return null

  const PIN_MIN_LENGTH = 4
  const pinIsValid = pin.trim().length >= PIN_MIN_LENGTH

  const handleBroadcast = async () => {
    if (!pinIsValid) {
      setStatusMsg(`Please enter a pairing PIN of at least ${PIN_MIN_LENGTH} characters before broadcasting.`)
      return
    }
    setIsBroadcasting(true)
    setStatusMsg(null)
    try {
      const count = await api.broadcastProfilesToLAN('all', 'NexVPN Device', pin)
      sound.playSuccess()
      setStatusMsg(`Broadcasted ${count} profiles to local network successfully!`)
    } catch (err: any) {
      setStatusMsg('Broadcast failed: ' + (err?.message || 'Error'))
    } finally {
      setIsBroadcasting(false)
    }
  }

  const handleStartListening = async () => {
    if (!pinIsValid) {
      setStatusMsg(`Please enter the same pairing PIN (at least ${PIN_MIN_LENGTH} characters) the sender used.`)
      return
    }
    setIsListening(true)
    setStatusMsg(null)
    setReceivedPayload(null)
    try {
      const payload = await api.listenForNearbyProfiles(10, pin)
      if (payload && payload.profiles && payload.profiles.length > 0) {
        setReceivedPayload(payload)
        sound.playSuccess()
        setStatusMsg(`Discovered ${payload.profiles.length} profiles from ${payload.device_name || 'Nearby Device'}!`)
      } else {
        setStatusMsg('No nearby broadcast detected. Make sure sender is on the same Wi-Fi with matching PIN.')
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
      const count = await api.importNearbyProfiles(receivedPayload.profiles, `Nearby (${receivedPayload.device_name})`)
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
    <ModalShell
      isOpen={isOpen}
      onClose={onClose}
      maxWidth="max-w-lg"
      className="max-h-[90vh]"
    >
        {/* Header */}
        <div className="flex items-center justify-between p-5 border-b border-border bg-background">
          <div className="flex items-center gap-3">
            <div className="w-10 h-10 rounded-2xl bg-info/20 border border-border flex items-center justify-center text-primary">
              <Radio className="w-5 h-5" />
            </div>
            <div>
              <h3 className="text-base font-bold text-foreground flex items-center gap-2">
                <span>{t('nearby_share_title') || 'Nearby Wi-Fi Share'}</span>
                <span className="px-2 py-0.5 text-[10px] font-bold rounded-full bg-primary/20 text-primary border border-primary/30">
                  Local P2P
                </span>
              </h3>
              <p className="text-xs text-muted-foreground">
                {t('nearby_share_subtitle') || 'Transfer VPN nodes between computers on the same Wi-Fi network'}
              </p>
            </div>
          </div>
          <button
            onClick={onClose}
            className="p-1.5 rounded-xl bg-muted hover:bg-muted/90 text-muted-foreground hover:text-foreground transition-all"
          >
            <X className="w-4 h-4" />
          </button>
        </div>

        {/* Tab Switcher */}
        <div className="p-4 bg-background border-b border-border flex items-center gap-2">
          <button
            type="button"
            onClick={() => setActiveTab('send')}
            className={`flex-1 py-2 rounded-xl text-xs font-bold transition flex items-center justify-center gap-2 ${
              activeTab === 'send'
                ? 'bg-primary text-primary-foreground shadow-sm'
                : 'text-muted-foreground hover:text-foreground hover:bg-muted'
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
                ? 'bg-primary text-primary-foreground shadow-sm'
                : 'text-muted-foreground hover:text-foreground hover:bg-muted'
            }`}
          >
            <Download className="w-4 h-4" />
            <span>{t('receive_profiles_tab') || 'Receive Profiles'}</span>
          </button>
        </div>

        {/* Body */}
        <div className="p-5 space-y-4 overflow-y-auto flex-1">
          {/* Required PIN Pairing Input */}
          <div className="p-3 bg-background border border-border rounded-2xl space-y-1.5 text-start rtl:text-end">
            <label className="text-[11px] font-semibold text-muted-foreground flex items-center justify-between">
              <span>{t('pairing_pin_label') || 'Pairing PIN / Passcode (Required)'}</span>
              <span className="text-[10px] text-muted-foreground">AES-256-GCM</span>
            </label>
            <input
              type="text"
              value={pin}
              onChange={e => setPin(e.target.value)}
              placeholder="e.g. 482913"
              className={`w-full px-3 py-2 bg-card border rounded-xl text-xs font-mono text-foreground outline-none ${
                pinIsValid ? 'border-border focus:border-primary' : 'border-warning/60 focus:border-warning'
              }`}
            />
            <p className="text-[10px] leading-relaxed text-muted-foreground">
              Enter the same PIN on both devices. A PIN is required so the shared profiles are never
              transmitted with a publicly known key.
            </p>
          </div>

          {activeTab === 'send' ? (
            <div className="space-y-4 text-center">
              <div className="w-16 h-16 rounded-3xl border border-border bg-muted flex items-center justify-center text-muted-foreground mx-auto">
                <Wifi className="w-8 h-8 animate-pulse" />
              </div>
              <div className="space-y-1">
                <h4 className="text-sm font-bold text-foreground">
                  {t('broadcast_nodes_title') || 'Broadcast Profiles over Local Wi-Fi'}
                </h4>
                <p className="text-xs text-muted-foreground leading-relaxed max-w-sm mx-auto">
                  {t('broadcast_nodes_desc') ||
                    'Send your active VPN configurations directly to other computers running NexVPN on the same Wi-Fi router.'}
                </p>
              </div>

              <button
                type="button"
                onClick={handleBroadcast}
                disabled={isBroadcasting}
                className="bg-primary w-full py-3 px-4 disabled:opacity-50 text-primary-foreground rounded-2xl text-xs font-bold shadow-sm flex items-center justify-center gap-2 active:scale-95 transition-all"
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
              <div className="w-16 h-16 rounded-3xl bg-primary/10 border border-primary/20 flex items-center justify-center text-primary mx-auto">
                <Radio className={`w-8 h-8 ${isListening ? 'animate-spin' : ''}`} />
              </div>
              <div className="space-y-1">
                <h4 className="text-sm font-bold text-foreground">
                  {t('listen_nodes_title') || 'Discover Incoming Node Shares'}
                </h4>
                <p className="text-xs text-muted-foreground leading-relaxed max-w-sm mx-auto">
                  {t('listen_nodes_desc') ||
                    'Listen for incoming broadcasts from other devices on your local network.'}
                </p>
              </div>

              <button
                type="button"
                onClick={handleStartListening}
                disabled={isListening}
                className="bg-primary w-full py-3 px-4 disabled:opacity-50 text-primary-foreground rounded-2xl text-xs font-bold shadow-sm flex items-center justify-center gap-2 active:scale-95 transition-all"
              >
                {isListening ? (
                  <RefreshCw className="w-4 h-4 animate-spin" />
                ) : (
                  <Radio className="w-4 h-4" />
                )}
                <span>{isListening ? 'Searching on Wi-Fi...' : t('start_discovery_btn') || 'Search for Nearby Shares'}</span>
              </button>

              {receivedPayload && (
                <div className="p-4 bg-background border border-border rounded-2xl space-y-3 text-start rtl:text-end animate-in fade-in duration-200">
                  <div className="flex items-center justify-between text-xs">
                    <span className="font-bold text-success flex items-center gap-1.5">
                      <CheckCircle2 className="w-4 h-4" />
                      <span>{receivedPayload.profiles?.length} Nodes Discovered</span>
                    </span>
                    <span className="text-muted-foreground font-mono">{receivedPayload.device_name}</span>
                  </div>

                  <button
                    type="button"
                    onClick={handleImportReceived}
                    disabled={isImporting}
                    className="w-full py-2 px-3 bg-primary hover:bg-primary/90 text-primary-foreground rounded-xl text-xs font-bold flex items-center justify-center gap-2 transition"
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
            <div className="p-3 bg-background border border-border rounded-xl text-xs text-muted-foreground text-center">
              {statusMsg}
            </div>
          )}
        </div>

        {/* Footer */}
        <div className="flex items-center justify-end p-4 border-t border-border bg-background">
          <button
            type="button"
            onClick={onClose}
            className="px-4 py-2 bg-muted hover:bg-muted/90 text-muted-foreground rounded-xl text-xs font-semibold"
          >
            {t('close')}
          </button>
        </div>
      </ModalShell>
  )
}
