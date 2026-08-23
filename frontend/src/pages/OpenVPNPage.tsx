import React, { useState, useEffect, useCallback } from 'react'
import {
  Plus,
  Trash2,
  Play,
  Square,
  Upload,
  CheckCircle2,
  Search,
  Lock,
  X,
  FileText,
  User,
  Key,
  Shield,
  Edit3,
  RefreshCw,
  AlertTriangle,
} from 'lucide-react'
import { api } from '../lib/api'
import { t } from '../lib/i18n'
import type { Profile, ConnectionStatus } from '../types'
import EditProfileModal from '../components/EditProfileModal'

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
  const [editingProfile, setEditingProfile] = useState<Profile | null>(null)

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

  return (
    <div className="page space-y-5">
      {/* Header */}
      <div className="page-header sticky top-0 z-30 flex items-center justify-between flex-wrap gap-3 bg-slate-900/90 backdrop-blur-xl py-3 -mx-2 px-4 rounded-2xl border border-white/5 shadow-lg shadow-black/20">
        <div>
          <h1 className="page-title">{t('openvpn_title')}</h1>
          <p className="page-subtitle">{t('openvpn_subtitle')}</p>
        </div>
        <div className="flex items-center gap-2">
          <button
            className="btn btn-secondary btn-sm"
            onClick={async () => {
              const updated = await api.pingAllProfiles('openvpn')
              if (updated) setProfiles(updated)
            }}
            title="Ping All OpenVPN Profiles"
          >
            <RefreshCw size={14} />
            <span>{t('ping_all')}</span>
          </button>
          <button
            id="add-ovpn-btn"
            className="btn btn-primary btn-sm"
            onClick={() => setShowAddModal(true)}
          >
            <Plus size={14} />
            {t('add_openvpn_btn')}
          </button>
        </div>
      </div>

      {/* Connection Failure Error Diagnostic Banner */}
      {status.state === 'error' && status.error_msg && (
        <div className="p-3.5 rounded-2xl bg-rose-950/60 border border-rose-500/40 text-left rtl:text-right space-y-1.5 animate-in fade-in duration-200">
          <div className="flex items-center gap-2 text-rose-300 font-bold text-xs">
            <AlertTriangle size={15} className="text-rose-400 shrink-0 animate-pulse" />
            <span>{t('connection_failed')}</span>
          </div>
          <p className="text-xs text-rose-200/90 leading-relaxed font-sans font-medium">
            {status.error_msg}
          </p>
          <div className="text-[11px] text-rose-300/70 pt-0.5">
            💡 {t('openvpn_error_hint')}
          </div>
        </div>
      )}

      {/* Pro Engine Optimization Banner */}
      <div className="p-4 rounded-2xl bg-gradient-to-r from-indigo-950/40 via-purple-950/20 to-slate-900 border border-indigo-500/20 flex items-center justify-between flex-wrap gap-3 text-xs">
        <div className="flex items-center gap-3">
          <div className="w-10 h-10 rounded-2xl bg-indigo-500/10 border border-indigo-500/30 flex items-center justify-center text-indigo-400 shrink-0">
            <Shield size={20} />
          </div>
          <div>
            <div className="font-bold text-slate-100 flex items-center gap-2">
              <span>OpenVPN Turbo Engine (Wintun Active)</span>
              <span className="px-2 py-0.5 text-[10px] font-bold rounded-full bg-emerald-500/20 text-emerald-300 border border-emerald-500/30">
                100% App Compatible
              </span>
            </div>
            <p className="text-[11px] text-slate-400 mt-0.5">
              MSS Clamping (1360), Wintun Low-Latency Driver, Anti-DNS Leak & Auto Iran Route Bypass Active
            </p>
          </div>
        </div>
        <div className="flex items-center gap-2 text-[10px] font-mono text-indigo-300">
          <span className="px-2.5 py-1 rounded-xl bg-slate-900/80 border border-slate-800">MTU: 1400</span>
          <span className="px-2.5 py-1 rounded-xl bg-slate-900/80 border border-slate-800">Driver: Wintun</span>
          <span className="px-2.5 py-1 rounded-xl bg-slate-900/80 border border-slate-800">Block-DNS: ON</span>
        </div>
      </div>

      {/* Search & Sort Bar */}
      <div className="card p-3 flex items-center justify-between flex-wrap gap-3">
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
            className="form-control text-xs font-semibold bg-slate-900 border-slate-700/60 hover:border-cyan-500/50 transition cursor-pointer shrink-0"
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
      </div>

      {/* Profiles Grid */}
      {filtered.length === 0 ? (
        <div className="card text-center p-12 space-y-3">
          <Lock size={48} className="mx-auto text-slate-600 mb-2 opacity-40" />
          <h3 className="font-bold text-sm text-slate-200">
            {t('no_openvpn_nodes')}
          </h3>
          <button className="btn btn-primary btn-sm mx-auto" onClick={() => setShowAddModal(true)}>
            <Plus size={14} />
            {t('add_openvpn_btn')}
          </button>
        </div>
      ) : (
        <div className="grid grid-cols-1 md:grid-cols-2 2xl:grid-cols-3 gap-3.5">
          {filtered.map(p => {
            const isConnected = status.state === 'connected' && status.profile_id === p.id
            const isConnecting = status.state === 'connecting' && status.profile_id === p.id

            return (
              <div
                key={p.id}
                className={`card p-3.5 sm:p-4 transition-all rounded-2xl flex flex-col justify-between overflow-hidden ${
                  isConnected
                    ? 'border-indigo-500/80 shadow-lg shadow-indigo-500/20 bg-indigo-950/20 ring-1 ring-indigo-500/40'
                    : 'hover:border-slate-700 bg-slate-900/60 border-white/5'
                }`}
              >
                <div>
                  <div className="flex items-center justify-between gap-2 mb-1.5 min-w-0">
                    <div className="flex items-center gap-2 flex-1 min-w-0">
                      <span className="text-lg shrink-0 leading-none">🔒</span>
                      <h4 className="font-bold text-xs truncate min-w-0 text-slate-100" title={p.name}>
                        {p.name}
                      </h4>
                    </div>

                    <span className="text-[9px] font-mono uppercase px-1.5 py-0.5 rounded bg-indigo-500/10 text-indigo-400 border border-indigo-500/30 shrink-0 font-bold">
                      OpenVPN
                    </span>
                  </div>

                  <div className="flex items-center justify-between text-[11px] text-slate-400 font-mono mb-2.5 gap-2">
                    <span className="truncate flex-1 text-slate-400">{p.server ? `${p.server}:${p.port || 1194}` : 'OpenVPN TAP'}</span>
                    <span className="text-[10px] px-1.5 py-0.5 rounded bg-slate-800/90 text-slate-300 truncate shrink-0">
                      TAP
                    </span>
                  </div>
                </div>

                <div className="flex items-center justify-between pt-2.5 border-t border-white/5 gap-1 mt-1">
                  <div className="text-[11px] text-slate-400 font-mono truncate max-w-[130px]">
                    {p.username ? `User: ${p.username}` : 'Direct TAP'}
                  </div>

                  <div className="flex items-center gap-0.5 shrink-0">
                    <button
                      type="button"
                      className="p-1.5 rounded-md text-slate-400 hover:text-cyan-400 hover:bg-slate-800 transition"
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
                      className="p-1.5 rounded-md text-rose-400 hover:bg-rose-500/20 transition"
                      title={t('delete')}
                      onClick={e => {
                        e.stopPropagation()
                        handleDelete(p.id)
                      }}
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
                        : 'bg-indigo-500 hover:bg-indigo-400 text-white shadow-sm shadow-indigo-500/20'
                    }`}
                    onClick={e => {
                      e.stopPropagation()
                      isConnected ? handleDisconnect() : handleConnect(p.id)
                    }}
                  >
                    {isConnecting ? (
                      <>
                        <RefreshCw size={10} className="animate-spin" />
                        <span>...</span>
                      </>
                    ) : isConnected ? (
                      <>
                        <Square size={10} className="fill-current" />
                        <span>{t('disconnect')}</span>
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
      alert('Failed to import OpenVPN configuration')
    } finally {
      setIsSubmitting(false)
    }
  }

  return (
    <div className="fixed inset-0 z-50 flex items-center justify-center bg-black/80 backdrop-blur-md p-4 animate-fade-in">
      <div className="bg-slate-900 border border-slate-700/80 rounded-2xl w-full max-w-lg overflow-hidden shadow-2xl p-6 space-y-4 max-h-[90vh] overflow-y-auto">
        <div className="flex items-center justify-between border-b border-slate-800 pb-3">
          <h3 className="font-bold text-slate-200 text-sm">{t('add_openvpn_btn')}</h3>
          <button onClick={onClose} className="text-slate-400 hover:text-slate-200">
            <X className="w-5 h-5" />
          </button>
        </div>

        <form onSubmit={handleSubmit} className="space-y-4">
          <div>
            <label className="text-xs text-slate-300 font-semibold block mb-1.5">Profile Name:</label>
            <input
              type="text"
              placeholder="e.g. My OpenVPN Server"
              className="form-control text-xs"
              value={name}
              onChange={e => setName(e.target.value)}
            />
          </div>

          <div className="border border-dashed border-slate-700 hover:border-cyan-500/50 p-4 rounded-xl text-center bg-slate-800/40 transition cursor-pointer">
            <label className="cursor-pointer flex flex-col items-center gap-1.5">
              <Upload size={20} className="text-cyan-400" />
              <span className="text-xs text-slate-300 font-medium">Click to upload .ovpn file</span>
              <span className="text-[10px] text-slate-500">Supports OpenVPN client configuration files</span>
              <input
                type="file"
                accept=".ovpn,.conf,.txt"
                className="hidden"
                onChange={handleFileUpload}
              />
            </label>
          </div>

          <div>
            <label className="text-xs text-slate-300 font-semibold block mb-1.5">{t('ovpn_content_label')}:</label>
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
              <label className="text-xs text-slate-300 font-semibold block mb-1.5">{t('ovpn_username')}:</label>
              <input
                type="text"
                placeholder="Username (optional)"
                className="form-control text-xs font-mono"
                value={username}
                onChange={e => setUsername(e.target.value)}
              />
            </div>
            <div>
              <label className="text-xs text-slate-300 font-semibold block mb-1.5">{t('ovpn_password')}:</label>
              <input
                type="password"
                placeholder="Password (optional)"
                className="form-control text-xs font-mono"
                value={password}
                onChange={e => setPassword(e.target.value)}
              />
            </div>
          </div>

          <div className="flex justify-end gap-2 pt-2 border-t border-slate-800">
            <button type="button" className="btn btn-secondary btn-sm" onClick={onClose}>
              {t('cancel') || 'Cancel'}
            </button>
            <button type="submit" className="btn btn-primary btn-sm" disabled={isSubmitting || !ovpnContent.trim()}>
              {isSubmitting ? '...' : t('add_openvpn_btn')}
            </button>
          </div>
        </form>
      </div>
    </div>
  )
}
