import React, { useState, useEffect, useCallback } from 'react'
import {
  Plus,
  Upload,
  Search,
  Lock,
  X,
  Shield,
  RefreshCw,
  AlertTriangle,
  AlertCircle,
  Activity,
  Globe,
  ShieldCheck,
} from 'lucide-react'
import { api } from '../lib/api'
import { t } from '../lib/i18n'
import { translateBackendError } from '../lib/backendErrors'
import type { Profile, ConnectionStatus } from '../types'
import EditProfileModal from '../components/EditProfileModal'
import OpenVPNCatalogModal from '../components/OpenVPNCatalogModal'
import TunnelBearModal from '../components/TunnelBearModal'
import CountryFlag from '../components/CountryFlag'
import { PingProgressBar } from '../components/PingProgressBar'
import { usePingProgress } from '../hooks/usePingProgress'
import { pinnedFirst, toggleProfilePin } from '../lib/profilePin'
import { ModalShell } from '../components/ui/modal-shell'
import { ProfileCard } from '../components/ui/profile-card'
import { Badge } from '@/components/ui/badge'
import { Button } from '@/components/ui/button'
import { Card } from '@/components/ui/card'

interface Props {
  status: ConnectionStatus
  onStatusChange?: (s: ConnectionStatus) => void
}

type SortMode = 'default' | 'ping_asc' | 'ping_desc' | 'name'

export default function OpenVPNPage({ status, onStatusChange }: Props) {
  const [profiles, setProfiles] = useState<Profile[]>([])
  const [search, setSearch] = useState('')
  const [sortMode, setSortModeState] = useState<SortMode>(() => {
    return (localStorage.getItem('nexvpn_openvpn_sort') as SortMode) || 'default'
  })
  const setSortMode = (mode: SortMode) => {
    setSortModeState(mode)
    localStorage.setItem('nexvpn_openvpn_sort', mode)
  }
  const [showAddModal, setShowAddModal] = useState(false)
  const [showCatalogModal, setShowCatalogModal] = useState(false)
  const [showTunnelBearModal, setShowTunnelBearModal] = useState(false)
  const [editingProfile, setEditingProfile] = useState<Profile | null>(null)
  const [isPingingAll, setIsPingingAll] = useState(false)
  // Sweeping a whole OpenVPN list takes tens of seconds at 3 s per probe, and
  // the promise only settles at the end. Without a visible counter the button
  // looked broken rather than busy.
  const pingProgress = usePingProgress(isPingingAll)

  const loadProfiles = useCallback(async () => {
    try {
      const list = await api.getProfilesByCategory('openvpn')
      if (list && Array.isArray(list)) setProfiles(list)
    } catch (err) {
      console.error('Failed to load OpenVPN profiles:', err)
    }
  }, [])

  useEffect(() => {
    loadProfiles()
  }, [loadProfiles])

  // The sweep is streamed from Go so the progress bar advances while it runs.
  const handlePingAll = async () => {
    if (isPingingAll) return
    setIsPingingAll(true)
    try {
      const updated = await api.pingAllProfiles('openvpn')
      if (updated) setProfiles(updated)
    } catch (err) {
      console.error('Ping all OpenVPN profiles error:', err)
    } finally {
      setIsPingingAll(false)
    }
  }

  const handleCancelPingAll = () => {
    api.cancelPingAll().catch(() => {})
  }

  // Per-row probe. OpenVPN relays are measured with a plain TCP connect, so a
  // reachable relay answers in milliseconds and a dead one reports -1.
  const handlePingSingle = async (id: string, e: React.MouseEvent) => {
    e.stopPropagation()
    try {
      const ms = await api.pingProfile(id)
      setProfiles(prev => prev.map(p => (p.id === id ? { ...p, latency_ms: ms } : p)))
    } catch (err) {
      console.error('Ping single OpenVPN profile error:', err)
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

  const handleDelete = async (id: string) => {
    try {
      await api.deleteProfile(id)
      setProfiles(prev => prev.filter(p => p.id !== id))
    } catch (err) {
      console.error('Delete error:', err)
    }
  }

  const filtered = profiles.filter(p =>
    !search || p.name.toLowerCase().includes(search.toLowerCase()) || p.group?.includes(search)
  )

  /**
   * Pin or unpin a profile.
   *
   * The list is updated optimistically and then reconciled with what the
   * backend reports: pinning writes the config file, and waiting for that round
   * trip before the star changes makes the control feel broken on a slow disk.
   */
  const handleTogglePin = (profile: Profile) => toggleProfilePin(profile, setProfiles)

  if (sortMode === 'ping_asc') {
    filtered.sort((a, b) => {
      const latA = a.latency_ms && a.latency_ms > 0 ? a.latency_ms : 99999
      const latB = b.latency_ms && b.latency_ms > 0 ? b.latency_ms : 99999
      return latA - latB
    })
  } else if (sortMode === 'ping_desc') {
    filtered.sort((a, b) => (b.latency_ms || 0) - (a.latency_ms || 0))
  } else if (sortMode === 'name') {
    filtered.sort((a, b) => a.name.localeCompare(b.name))
  }

  // Pinned profiles lead, whichever sort is active. Applied last on purpose:
  // Array.prototype.sort is stable, so sorting only by the pin preserves the
  // order the chosen sort mode just established *within* each group.
  filtered.sort(pinnedFirst)

  return (
    <div className="page space-y-5">
      {/* Header */}
      <div className="page-header sticky top-0 z-30 flex items-center justify-between flex-wrap gap-3 surface-toolbar py-3 -mx-2 px-4 rounded-2xl shadow-md">
        <div>
          <h1 className="page-title">{t('openvpn_title')}</h1>
          <p className="page-subtitle">{t('openvpn_subtitle')}</p>
        </div>
        <div className="flex items-center gap-2">
          <Button variant="secondary" size="sm"
            onClick={handlePingAll}
            disabled={isPingingAll || profiles.length === 0}
            title={t('ping_all_hint')}
          >
            <RefreshCw size={14} className={isPingingAll ? 'animate-spin' : ''} />
            <span>{isPingingAll ? t('pinging_all') : t('ping_all')}</span>
          </Button>
          <Button variant="secondary" size="sm"
            id="browse-ovpn-catalog-btn"
            onClick={() => setShowCatalogModal(true)}
            title={t('ovpn_catalog_title')}
          >
            <Globe size={14} />
            <span>{t('ovpn_catalog_btn')}</span>
          </Button>
          <Button variant="secondary" size="sm"
            id="tunnelbear-btn"
            onClick={() => setShowTunnelBearModal(true)}
            title={t('tb_title')}
          >
            <ShieldCheck size={14} />
            <span>{t('tb_btn')}</span>
          </Button>
          <Button size="sm"
            id="add-ovpn-btn"
            onClick={() => setShowAddModal(true)}
          >
            <Plus size={14} />
            {t('add_openvpn_btn')}
          </Button>
        </div>
      </div>

      {/* Reachability sweep progress */}
      <PingProgressBar progress={pingProgress} onCancel={handleCancelPingAll} />

      {/* Connection Failure Error Diagnostic Banner */}
      {status.state === 'error' && status.error_msg && (
        <div className="p-3.5 rounded-2xl error-banner-modern text-start rtl:text-end space-y-1.5 animate-in fade-in duration-200 shadow-sm">
          <div className="flex items-center gap-2 text-destructive font-bold text-xs">
            <AlertTriangle size={15} className="text-destructive shrink-0 animate-pulse" />
            <span>{t('connection_failed')}</span>
          </div>
          <p
            className="text-xs text-destructive leading-relaxed font-sans font-medium"
            title={status.error_msg}
          >
            {translateBackendError(status.error_msg)}
          </p>
          <div className="text-[11px] text-destructive/80 dark:text-destructive/70 pt-0.5 font-medium">
            💡 {t('openvpn_error_hint')}
          </div>
        </div>
      )}

      {/* Pro Engine Optimization Banner */}
      <Card className="p-4 rounded-2xl border border-primary/25 flex items-center justify-between flex-wrap gap-3 text-xs shadow-sm">
        <div className="flex items-center gap-3">
          <div className="w-10 h-10 rounded-2xl bg-primary/10 border border-primary/30 flex items-center justify-center text-primary shrink-0">
            <Shield size={20} />
          </div>
          <div>
            <div className="font-medium text-foreground flex items-center gap-2">
              <span>OpenVPN Turbo Engine (Wintun Active)</span>
              <Badge variant="success">100% App Compatible</Badge>
            </div>
            <p className="text-[11px] text-muted-foreground mt-0.5">
              MSS Clamping (1360), Wintun Low-Latency Driver, Anti-DNS Leak & Auto Iran Route Bypass Active
            </p>
          </div>
        </div>
        <div className="flex items-center gap-2 text-[10px] font-mono font-bold text-primary">
          <span className="px-2.5 py-1 rounded-xl segmented-pill">MTU: 1400</span>
          <span className="px-2.5 py-1 rounded-xl segmented-pill">Driver: Wintun</span>
          <span className="px-2.5 py-1 rounded-xl segmented-pill">Block-DNS: ON</span>
        </div>
      </Card>

      {/* Search & Sort Bar */}
      <Card className="p-3 flex items-center justify-between flex-wrap gap-3">
        <div className="flex items-center gap-2 flex-1" style={{ maxWidth: '460px' }}>
          <div className="search-bar flex-1">
            <Search size={15} />
            <input
              type="text"
              placeholder={t('search_nodes')}
              value={search}
              onChange={e => setSearch(e.target.value)}
            />
          </div>

          <select
            className="form-control text-xs font-semibold bg-surface border-border hover:border-primary transition cursor-pointer shrink-0"
            style={{ width: '130px' }}
            value={sortMode}
            onChange={e => setSortMode(e.target.value as SortMode)}
          >
            <option value="default">{t('sort_default')}</option>
            <option value="ping_asc">⚡ {t('sort_ping_asc')}</option>
            <option value="ping_desc">{t('sort_ping_desc')}</option>
            <option value="name">🔤 {t('sort_name')}</option>
          </select>
        </div>
      </Card>

      {/* Profiles Grid */}
      {filtered.length === 0 ? (
        <Card className="text-center p-12 space-y-3">
          <Lock size={48} className="mx-auto text-muted-foreground mb-2 opacity-40" />
          <h3 className="font-bold text-sm text-primary">
            {t('no_openvpn_nodes')}
          </h3>
          <div className="flex items-center justify-center gap-2">
            <Button size="sm" onClick={() => setShowAddModal(true)}>
              <Plus size={14} />
              {t('add_openvpn_btn')}
            </Button>
            <Button variant="secondary" size="sm" onClick={() => setShowCatalogModal(true)}>
              <Globe size={14} />
              {t('ovpn_catalog_btn')}
            </Button>
            <Button variant="secondary" size="sm" onClick={() => setShowTunnelBearModal(true)}>
              <ShieldCheck size={14} />
              {t('tb_btn')}
            </Button>
          </div>
        </Card>
      ) : (
        <div className="grid grid-cols-1 md:grid-cols-2 2xl:grid-cols-3 gap-3.5">
          {filtered.map(p => {
            const isConnected = status.state === 'connected' && status.profile_id === p.id
            const isConnecting = status.state === 'connecting' && status.profile_id === p.id

            return (
              <ProfileCard
                key={p.id}
                name={p.name}
                icon={
                  <CountryFlag
                    codeOrName={p.country_code || p.country || p.group || p.name}
                    size="md"
                  />
                }
                protocolLabel="OpenVPN"
                metaSlot={
                  <>
                    <span className="truncate flex-1">
                      {p.server ? `${p.server}:${p.port || 1194}` : 'OpenVPN TAP'}
                    </span>
                    {p.username && (
                      <span className="text-[10px] px-1.5 py-0.5 rounded segmented-pill truncate shrink-0 max-w-[110px]">
                        {p.username}
                      </span>
                    )}
                    <span className="text-[10px] px-1.5 py-0.5 rounded segmented-pill truncate shrink-0">
                      TAP
                    </span>
                  </>
                }
                footnoteSlot={
                  // The sweep and the single-row probe both write latency_ms back
                  // to the profile, but the card never showed it — the value was
                  // measured and then thrown away. This is the readout.
                  <button
                    type="button"
                    className="px-2 py-1 rounded-lg bg-muted hover:bg-muted/90 text-[11px] text-foreground transition flex items-center gap-1 font-mono border border-border"
                    onClick={e => handlePingSingle(p.id, e)}
                    title={t('ping_test')}
                  >
                    <Activity
                      size={11}
                      className={
                        p.latency_ms && p.latency_ms > 0
                          ? 'text-success'
                          : p.latency_ms && p.latency_ms < 0
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
                    ) : p.latency_ms && p.latency_ms < 0 ? (
                      <span className="text-destructive font-bold flex items-center gap-0.5">
                        <AlertCircle size={10} />
                        <span>-1 ms</span>
                      </span>
                    ) : (
                      <span className="text-muted-foreground font-mono">
                        {t('ping_test')}
                      </span>
                    )}
                  </button>
                }
                state={isConnected ? 'connected' : isConnecting ? 'connecting' : 'idle'}
                connectClassName="bg-primary hover:bg-primary/90 text-primary-foreground shadow-sm"
                onConnect={() => handleConnect(p.id)}
                onDisconnect={handleDisconnect}
                onEdit={() => setEditingProfile(p)}
                onDelete={() => handleDelete(p.id)}
                pinned={!!p.favorite}
                onTogglePin={() => handleTogglePin(p)}
                pinLabels={{ pin: t('adv_pin'), unpin: t('adv_unpin') }}
                labels={{ connect: t('connect'), disconnect: t('disconnect'), edit: t('edit'), delete: t('delete') }}
              />
            )
          })}
        </div>
      )}

      {/* Profile Edit Modal */}
      <EditProfileModal
        isOpen={!!editingProfile}
        profile={editingProfile}
        onClose={() => setEditingProfile(null)}
        onSaved={() => loadProfiles()}
        onDeleted={id => setProfiles(prev => prev.filter(p => p.id !== id))}
      />

      {/* Add OpenVPN Modal */}
      {showAddModal && (
        <AddOpenVPNModal
          isOpen={showAddModal}
          onClose={() => setShowAddModal(false)}
          onAdded={() => {
            setShowAddModal(false)
            loadProfiles()
          }}
        />
      )}

      {/* Public OpenVPN Server Catalogue Modal */}
      <OpenVPNCatalogModal
        isOpen={showCatalogModal}
        onClose={() => setShowCatalogModal(false)}
        onImported={() => loadProfiles()}
      />

      {/* TunnelBear: the operator's countries, signed in with the user's own account */}
      <TunnelBearModal
        isOpen={showTunnelBearModal}
        onClose={() => setShowTunnelBearModal(false)}
        onImported={() => loadProfiles()}
      />
    </div>
  )
}

function AddOpenVPNModal({
  isOpen,
  onClose,
  onAdded,
}: {
  isOpen: boolean
  onClose: () => void
  onAdded: () => void
}) {
  const [name, setName] = useState('')
  const [ovpnContent, setOvpnContent] = useState('')
  const [username, setUsername] = useState('')
  const [password, setPassword] = useState('')
  const [isSubmitting, setIsSubmitting] = useState(false)

  if (!isOpen) return null

  const handleFileUpload = (e: React.ChangeEvent<HTMLInputElement>) => {
    const file = e.target.files?.[0]
    if (!file) return
    if (!name) {
      setName(file.name.replace(/\.ovpn$/i, ''))
    }
    const reader = new FileReader()
    reader.onload = evt => {
      if (typeof evt.target?.result === 'string') {
        setOvpnContent(evt.target.result)
      }
    }
    reader.readAsText(file)
  }

  const handleSubmit = async (e: React.FormEvent) => {
    e.preventDefault()
    if (!ovpnContent.trim()) return
    setIsSubmitting(true)
    try {
      await api.importOpenVPNProfile(
        name || 'OpenVPN Profile',
        ovpnContent.trim(),
        username.trim(),
        password.trim()
      )
      onAdded()
    } catch (err) {
      console.error('Import OpenVPN error:', err)
      alert(t('alert_import_ovpn_failed'))
    } finally {
      setIsSubmitting(false)
    }
  }

  return (
    <ModalShell
      isOpen={isOpen}
      onClose={onClose}
      maxWidth="max-w-lg"
      className="max-h-[90vh] overflow-y-auto"
      bodyClassName="p-6 space-y-4"
    >
        <div className="flex items-center justify-between border-b border-border pb-3">
          <h3 className="font-bold text-foreground text-sm">{t('add_openvpn_btn')}</h3>
          <button onClick={onClose} className="text-muted-foreground hover:text-foreground">
            <X className="w-5 h-5" />
          </button>
        </div>

        <form onSubmit={handleSubmit} className="space-y-4">
          <div>
            <label className="text-xs text-muted-foreground font-semibold block mb-1.5">Profile Name:</label>
            <input
              type="text"
              placeholder="e.g. My OpenVPN Server"
              className="form-control text-xs"
              value={name}
              onChange={e => setName(e.target.value)}
            />
          </div>

          <div className="border border-dashed border-border hover:border-primary/50 p-4 rounded-xl text-center bg-muted transition cursor-pointer">
            <label className="cursor-pointer flex flex-col items-center gap-1.5">
              <Upload size={20} className="text-primary" />
              <span className="text-xs text-muted-foreground font-medium">Click to upload .ovpn file</span>
              <span className="text-[10px] text-muted-foreground">Supports OpenVPN client configuration files</span>
              <input
                type="file"
                accept=".ovpn,.conf,.txt"
                className="hidden"
                onChange={handleFileUpload}
              />
            </label>
          </div>

          <div>
            <label className="text-xs text-muted-foreground font-semibold block mb-1.5">{t('ovpn_content_label')}:</label>
            <textarea
              rows={5}
              placeholder={t('ovpn_placeholder')}
              className="form-control font-mono text-xs"
              value={ovpnContent}
              onChange={e => setOvpnContent(e.target.value)}
              required
            />
          </div>

          <div className="grid grid-cols-2 gap-3">
            <div>
              <label className="text-xs text-muted-foreground font-semibold block mb-1.5">{t('ovpn_username')}:</label>
              <input
                type="text"
                placeholder="Username (optional)"
                className="form-control text-xs font-mono"
                value={username}
                onChange={e => setUsername(e.target.value)}
              />
            </div>
            <div>
              <label className="text-xs text-muted-foreground font-semibold block mb-1.5">{t('ovpn_password')}:</label>
              <input
                type="password"
                placeholder="Password (optional)"
                className="form-control text-xs font-mono"
                value={password}
                onChange={e => setPassword(e.target.value)}
              />
            </div>
          </div>

          <div className="flex justify-end gap-2 pt-2 border-t border-border">
            <Button variant="secondary" size="sm" type="button" onClick={onClose}>
              {t('cancel') || 'Cancel'}
            </Button>
            <Button size="sm" type="submit" disabled={isSubmitting || !ovpnContent.trim()}>
              {isSubmitting ? '...' : t('add_openvpn_btn')}
            </Button>
          </div>
        </form>
      </ModalShell>
  )
}
