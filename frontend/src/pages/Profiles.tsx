import { useState, useEffect, useCallback, useRef } from 'react'
import {
  Plus,
  Trash2,
  Edit2,
  Play,
  Square,
  Upload,
  CheckCircle2,
  Search,
  Zap,
  Folder,
  Link,
  Globe,
  FileUp,
  Share2,
  Copy,
  Check,
  RefreshCw,
  QrCode,
  X,
} from 'lucide-react'
import { api } from '../lib/api'
import { t } from '../lib/i18n'
import type { Profile, ConnectionStatus } from '../types'
import {
  getProtocolName,
  getProtocolEmoji,
  getProtocolColor,
  formatRelative,
} from '../lib/utils'
import { generateQRCodeSVG } from '../lib/qrcode'
import EditProfileModal from '../components/EditProfileModal'

interface Props {
  status: ConnectionStatus
  onStatusChange: (s: ConnectionStatus) => void
}

type FilterProtocol = 'all' | 'wireguard' | 'xray' | 'singbox' | 'openvpn' | 'hysteria2'

export default function Profiles({ status, onStatusChange }: Props) {
  const [profiles, setProfiles] = useState<Profile[]>([])
  const [groups, setGroups] = useState<string[]>([])
  const [search, setSearch] = useState('')
  const [filterProtocol, setFilterProtocol] = useState<FilterProtocol>('all')
  const [filterGroup, setFilterGroup] = useState<string>('all')
  const [showAddModal, setShowAddModal] = useState(false)
  const [showImportModal, setShowImportModal] = useState(false)
  const [sharingProfile, setSharingProfile] = useState<Profile | null>(null)
  const [editingProfile, setEditingProfile] = useState<Profile | null>(null)
  const [deletingId, setDeletingId] = useState<string | null>(null)
  const [isPinging, setIsPinging] = useState(false)
  const [isUpdatingSubs, setIsUpdatingSubs] = useState(false)
  const [copiedId, setCopiedId] = useState<string | null>(null)

  const loadProfiles = useCallback(async () => {
    try {
      const [p, g] = await Promise.all([api.getProfiles(), api.getGroups()])
      if (p && Array.isArray(p)) setProfiles(p)
      if (g && Array.isArray(g)) setGroups(g)
    } catch (err) {
      console.error('Failed to load profiles:', err)
    }
  }, [])

  useEffect(() => {
    loadProfiles()
  }, [loadProfiles])

  const qrImageUrl = sharingProfile
    ? generateQRCodeSVG(sharingProfile.raw_uri || sharingProfile.config_content || sharingProfile.name)
    : ''

  const handlePingAll = async () => {
    setIsPinging(true)
    try {
      const updated = await api.pingAllProfiles()
      setProfiles(updated)
    } catch (err) {
      console.error('Ping all error:', err)
    } finally {
      setIsPinging(false)
    }
  }

  const handleUpdateAllSubscriptions = async () => {
    setIsUpdatingSubs(true)
    try {
      await api.updateAllSubscriptions()
      await loadProfiles()
    } catch (err) {
      console.error('Update subs error:', err)
    } finally {
      setIsUpdatingSubs(false)
    }
  }

  const handleDeleteAll = async () => {
    if (window.confirm(t('confirm_delete_all'))) {
      try {
        await api.deleteAllProfiles()
        await loadProfiles()
      } catch (err) {
        console.error('Delete all error:', err)
      }
    }
  }

  const handlePingSingle = async (id: string, e: React.MouseEvent) => {
    e.stopPropagation()
    try {
      const ms = await api.pingProfile(id)
      setProfiles(prev =>
        prev.map(p => (p.id === id ? { ...p, latency_ms: ms } : p))
      )
    } catch (err) {
      console.error('Ping error:', err)
    }
  }

  const handleCopyURI = (p: Profile, e: React.MouseEvent) => {
    e.stopPropagation()
    const text = p.raw_uri || p.config_content || p.name
    navigator.clipboard.writeText(text)
    setCopiedId(p.id)
    setTimeout(() => setCopiedId(null), 2000)
  }

  const filteredProfiles = profiles.filter(p => {
    const matchSearch =
      !search ||
      p.name.toLowerCase().includes(search.toLowerCase()) ||
      p.server?.toLowerCase().includes(search.toLowerCase()) ||
      p.description?.toLowerCase().includes(search.toLowerCase()) ||
      p.group?.toLowerCase().includes(search.toLowerCase())
    const matchProtocol = filterProtocol === 'all' || p.protocol === filterProtocol
    const matchGroup = filterGroup === 'all' || p.group === filterGroup
    return matchSearch && matchProtocol && matchGroup
  })

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
      setDeletingId(null)
    } catch (err) {
      console.error('Delete error:', err)
    }
  }

  return (
    <div className="page">
      {/* Page Header */}
      <div className="page-header">
        <div className="flex items-center justify-between">
          <div>
            <h1 className="page-title">{t('profiles_title')}</h1>
            <p className="page-subtitle">{t('profiles_subtitle')}</p>
          </div>
          <div className="flex gap-2">
            <button
              id="import-btn"
              className="btn btn-secondary"
              onClick={() => setShowImportModal(true)}
            >
              <Upload size={16} />
              {t('import_uri')}
            </button>
            <button
              id="add-profile-btn"
              className="btn btn-primary"
              onClick={() => setShowAddModal(true)}
            >
              <Plus size={16} />
              {t('add_profile')}
            </button>
          </div>
        </div>
      </div>

      {/* Filter and Action Bar */}
      <div className="flex items-center justify-between gap-3" style={{ marginBottom: '20px', flexWrap: 'wrap' }}>
        <div className="flex items-center gap-3" style={{ flex: 1, minWidth: '240px' }}>
          {/* Search */}
          <div style={{ position: 'relative', flex: 1, maxWidth: '280px' }}>
            <Search
              size={14}
              style={{
                position: 'absolute',
                left: '12px',
                top: '50%',
                transform: 'translateY(-50%)',
                color: 'var(--color-text-muted)',
              }}
            />
            <input
              id="profile-search"
              type="text"
              className="form-control"
              placeholder={t('search_placeholder')}
              style={{ paddingLeft: '34px', fontSize: '13px' }}
              value={search}
              onChange={e => setSearch(e.target.value)}
            />
          </div>

          {/* Protocol Filter */}
          <select
            id="protocol-filter"
            className="form-control"
            style={{ width: '140px', fontSize: '13px' }}
            value={filterProtocol}
            onChange={e => setFilterProtocol(e.target.value as FilterProtocol)}
          >
            <option value="all">{t('all_protocols')}</option>
            <option value="xray">⚡ Xray (VLESS/Reality)</option>
            <option value="singbox">🚀 Sing-box / Hy2</option>
            <option value="wireguard">🔒 WireGuard</option>
            <option value="openvpn">🛡️ OpenVPN</option>
          </select>

          {/* Group Filter */}
          {groups.length > 0 && (
            <select
              id="group-filter"
              className="form-control"
              style={{ width: '130px', fontSize: '13px' }}
              value={filterGroup}
              onChange={e => setFilterGroup(e.target.value)}
            >
              <option value="all">{t('all_groups')}</option>
              {groups.map(g => (
                <option key={g} value={g}>
                  {g}
                </option>
              ))}
            </select>
          )}
        </div>

        {/* Action Buttons */}
        <div className="flex gap-2">
          <button
            id="update-subs-btn"
            className="btn btn-secondary btn-sm"
            onClick={handleUpdateAllSubscriptions}
            disabled={isUpdatingSubs}
          >
            <RefreshCw size={13} style={isUpdatingSubs ? { animation: 'spin 1s linear infinite' } : {}} />
            {t('update_subs')}
          </button>

          <button
            id="ping-all-btn"
            className="btn btn-secondary btn-sm"
            onClick={handlePingAll}
            disabled={isPinging || profiles.length === 0}
          >
            <Zap size={13} color="var(--color-warning)" />
            {isPinging ? t('pinging') : t('ping_all')}
          </button>

          {profiles.length > 0 && (
            <button
              id="delete-all-btn"
              className="btn btn-danger btn-sm"
              onClick={handleDeleteAll}
            >
              <Trash2 size={13} />
              {t('delete_all')}
            </button>
          )}
        </div>
      </div>

      {/* Profiles Grid / List */}
      {filteredProfiles.length === 0 ? (
        <div className="card">
          <div
            className="card-body"
            style={{
              textAlign: 'center',
              padding: '60px 20px',
              color: 'var(--color-text-muted)',
            }}
          >
            <Globe size={40} style={{ opacity: 0.3, marginBottom: '16px' }} />
            <div style={{ fontSize: '15px', fontWeight: 600, color: 'var(--color-text-secondary)' }}>
              {t('no_profiles_found')}
            </div>
            <p style={{ fontSize: '13px', marginTop: '6px' }}>
              {t('create_first_profile')}
            </p>
          </div>
        </div>
      ) : (
        <div className="grid-2" style={{ gap: '16px' }}>
          {filteredProfiles.map(p => {
            const isConnectedToThis = status.state === 'connected' && status.profile_id === p.id
            const isConnectingThis = status.state === 'connecting' && status.profile_id === p.id

            return (
              <div
                key={p.id}
                className={`card profile-card ${isConnectedToThis ? 'active-border' : ''}`}
                style={{
                  border: isConnectedToThis ? '1px solid var(--color-success)' : undefined,
                  transition: 'all 0.2s ease',
                }}
              >
                <div className="card-body" style={{ padding: '18px 20px' }}>
                  {/* Top: Protocol Badge + Name + Status */}
                  <div className="flex items-center justify-between" style={{ marginBottom: '10px' }}>
                    <div className="flex items-center gap-2">
                      <span style={{ fontSize: '18px' }}>{getProtocolEmoji(p.protocol)}</span>
                      <div>
                        <div style={{ fontWeight: 600, fontSize: '14px', color: 'var(--color-text-primary)' }}>
                          {p.name}
                        </div>
                        {p.group && (
                          <span className="badge badge-muted" style={{ fontSize: '10px', padding: '1px 6px' }}>
                            {p.group}
                          </span>
                        )}
                      </div>
                    </div>

                    <div className="flex items-center gap-2">
                      {/* Latency badge */}
                      {p.latency_ms !== undefined && p.latency_ms !== 0 && (
                        <button
                          className={`badge ${
                            p.latency_ms > 0 && p.latency_ms < 150
                              ? 'badge-success'
                              : p.latency_ms >= 150
                              ? 'badge-warning'
                              : 'badge-error'
                          }`}
                          style={{ cursor: 'pointer', border: 'none' }}
                          onClick={e => handlePingSingle(p.id, e)}
                          title="Click to re-ping"
                        >
                          {p.latency_ms > 0 ? `${p.latency_ms} ms` : 'Timeout'}
                        </button>
                      )}

                      {/* Active Status */}
                      {isConnectedToThis && (
                        <span className="badge badge-success">
                          <CheckCircle2 size={11} style={{ marginRight: 4 }} />
                          {t('connected_badge')}
                        </span>
                      )}
                    </div>
                  </div>

                  {/* Server info */}
                  {p.server && (
                    <div className="text-xs text-muted" style={{ marginBottom: '12px' }}>
                      {p.server}:{p.port || 443}
                    </div>
                  )}

                  {/* Bottom Action Row */}
                  <div className="flex items-center justify-between" style={{ borderTop: '1px solid var(--color-border)', paddingTop: '12px' }}>
                    <div className="flex gap-1">
                      {/* QR Code Button */}
                      <button
                        className="btn btn-secondary btn-sm"
                        style={{ padding: '4px 8px' }}
                        onClick={() => setSharingProfile(p)}
                        title={t('show_qr')}
                      >
                        <QrCode size={13} />
                      </button>

                      {/* Copy Link Button */}
                      <button
                        className="btn btn-secondary btn-sm"
                        style={{ padding: '4px 8px' }}
                        onClick={e => handleCopyURI(p, e)}
                        title={t('copy_uri')}
                      >
                        {copiedId === p.id ? <Check size={13} color="var(--color-success)" /> : <Copy size={13} />}
                      </button>

                      {/* Delete Button */}
                      <button
                        className="btn btn-danger btn-sm"
                        style={{ padding: '4px 8px' }}
                        onClick={() => handleDelete(p.id)}
                        title={t('delete')}
                      >
                        <Trash2 size={13} />
                      </button>
                    </div>

                    {/* Connect / Disconnect button */}
                    {isConnectedToThis ? (
                      <button
                        className="btn btn-danger btn-sm"
                        onClick={handleDisconnect}
                      >
                        <Square size={12} />
                        {t('disconnect_node')}
                      </button>
                    ) : (
                      <button
                        className="btn btn-primary btn-sm"
                        onClick={() => handleConnect(p.id)}
                        disabled={isConnectingThis}
                      >
                        <Play size={12} />
                        {t('connect_node')}
                      </button>
                    )}
                  </div>
                </div>
              </div>
            )
          })}
        </div>
      )}

      {/* QR Code Modal */}
      {sharingProfile && (
        <div className="modal-backdrop" onClick={() => setSharingProfile(null)}>
          <div className="modal" onClick={e => e.stopPropagation()} style={{ maxWidth: '360px', textAlign: 'center' }}>
            <div className="modal-header">
              <div className="modal-title">{t('qr_code_modal_title')}</div>
              <button className="modal-close" onClick={() => setSharingProfile(null)}>
                <X size={18} />
              </button>
            </div>
            <div className="modal-body">
              <p className="text-xs text-muted" style={{ marginBottom: '12px' }}>
                {t('qr_code_modal_desc')}
              </p>
              <div className="qr-container">
                {qrImageUrl ? (
                  <img src={qrImageUrl} alt="QR Code" style={{ width: '220px', height: '220px' }} />
                ) : (
                  <div className="spinner" />
                )}
              </div>
              <div style={{ fontWeight: 600, fontSize: '13px', color: 'var(--color-text-primary)', marginBottom: '4px' }}>
                {sharingProfile.name}
              </div>
              <div className="text-xs text-muted truncate" style={{ maxWidth: '280px', margin: '0 auto' }}>
                {sharingProfile.raw_uri || sharingProfile.server}
              </div>
            </div>
            <div className="modal-footer" style={{ justifyContent: 'center' }}>
              <button className="btn btn-secondary" onClick={() => setSharingProfile(null)}>
                {t('close')}
              </button>
            </div>
          </div>
        </div>
      )}

      {/* Import Modal */}
      {showImportModal && (
        <ImportModal
          onClose={() => setShowImportModal(false)}
          onImported={() => {
            setShowImportModal(false)
            loadProfiles()
          }}
        />
      )}

      {/* Profile Edit Modal */}
      <EditProfileModal
        isOpen={!!editingProfile}
        profile={editingProfile}
        onClose={() => setEditingProfile(null)}
        onSaved={() => loadProfiles()}
        onDeleted={id => setProfiles(prev => prev.filter(p => p.id !== id))}
      />

      {/* Add Manual Modal */}
      {showAddModal && (
        <AddModal
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

function ImportModal({ onClose, onImported }: { onClose: () => void; onImported: () => void }) {
  const [tab, setTab] = useState<'uri' | 'sub'>('uri')
  const [uri, setUri] = useState('')
  const [subUrl, setSubUrl] = useState('')
  const [group, setGroup] = useState('')
  const [isImporting, setIsImporting] = useState(false)
  const [error, setError] = useState('')

  const handleImport = async () => {
    setIsImporting(true)
    setError('')
    try {
      if (tab === 'uri') {
        if (!uri.trim()) return
        await api.importURI(uri.trim(), group.trim())
      } else {
        if (!subUrl.trim()) return
        await api.importSubscription(subUrl.trim(), group.trim())
      }
      onImported()
    } catch (err: any) {
      setError(err?.message || 'Failed to import. Please verify URL / URI.')
    } finally {
      setIsImporting(false)
    }
  }

  return (
    <div className="modal-backdrop" onClick={onClose}>
      <div className="modal" onClick={e => e.stopPropagation()} style={{ maxWidth: '480px' }}>
        <div className="modal-header">
          <div className="modal-title">{t('import_title')}</div>
          <button className="modal-close" onClick={onClose}>
            <X size={18} />
          </button>
        </div>

        <div className="modal-body">
          {/* Tabs */}
          <div className="flex gap-2" style={{ marginBottom: '16px' }}>
            <button
              className={`btn btn-sm ${tab === 'uri' ? 'btn-primary' : 'btn-secondary'}`}
              onClick={() => setTab('uri')}
            >
              <Link size={13} />
              {t('import_tab_uri')}
            </button>
            <button
              className={`btn btn-sm ${tab === 'sub' ? 'btn-primary' : 'btn-secondary'}`}
              onClick={() => setTab('sub')}
            >
              <RefreshCw size={13} />
              {t('import_tab_sub')}
            </button>
          </div>

          {error && (
            <div
              className="badge badge-error"
              style={{ width: '100%', padding: '8px 12px', marginBottom: '12px', whiteSpace: 'normal' }}
            >
              {error}
            </div>
          )}

          {tab === 'uri' ? (
            <div className="form-group">
              <label className="form-label">{t('import_tab_uri')}</label>
              <textarea
                className="form-control"
                rows={4}
                placeholder={t('uri_placeholder')}
                value={uri}
                onChange={e => setUri(e.target.value)}
              />
            </div>
          ) : (
            <div className="form-group">
              <label className="form-label">{t('import_tab_sub')}</label>
              <input
                type="text"
                className="form-control"
                placeholder={t('sub_placeholder')}
                value={subUrl}
                onChange={e => setSubUrl(e.target.value)}
              />
            </div>
          )}

          <div className="form-group">
            <label className="form-label">{t('group_placeholder')}</label>
            <input
              type="text"
              className="form-control"
              placeholder="e.g. VIP / Germany"
              value={group}
              onChange={e => setGroup(e.target.value)}
            />
          </div>
        </div>

        <div className="modal-footer">
          <button className="btn btn-secondary" onClick={onClose}>
            {t('cancel')}
          </button>
          <button className="btn btn-primary" onClick={handleImport} disabled={isImporting}>
            {isImporting ? '...' : tab === 'uri' ? t('import_btn') : t('import_sub_btn')}
          </button>
        </div>
      </div>
    </div>
  )
}

function AddModal({ onClose, onAdded }: { onClose: () => void; onAdded: () => void }) {
  const [name, setName] = useState('')
  const [protocol, setProtocol] = useState<string>('xray')
  const [configContent, setConfigContent] = useState('')
  const [group, setGroup] = useState('')
  const [isSaving, setIsSaving] = useState(false)

  const handleSave = async () => {
    if (!name.trim()) return
    setIsSaving(true)
    try {
      await api.importProfile(name, protocol, configContent, group)
      onAdded()
    } catch (err) {
      console.error('Failed to add profile:', err)
    } finally {
      setIsSaving(false)
    }
  }

  return (
    <div className="modal-backdrop" onClick={onClose}>
      <div className="modal" onClick={e => e.stopPropagation()} style={{ maxWidth: '520px' }}>
        <div className="modal-header">
          <div className="modal-title">{t('add_profile')}</div>
          <button className="modal-close" onClick={onClose}>
            <X size={18} />
          </button>
        </div>

        <div className="modal-body">
          <div className="form-group">
            <label className="form-label">{t('profile_name')}</label>
            <input
              type="text"
              className="form-control"
              placeholder="My Fast Server"
              value={name}
              onChange={e => setName(e.target.value)}
            />
          </div>

          <div className="form-group">
            <label className="form-label">{t('protocol')}</label>
            <select
              className="form-control"
              value={protocol}
              onChange={e => setProtocol(e.target.value)}
            >
              <option value="xray">Xray (VLESS / VMess / Reality / Trojan)</option>
              <option value="singbox">Sing-box (Hysteria 2 / Shadowsocks / TUN)</option>
              <option value="wireguard">WireGuard (.conf)</option>
              <option value="openvpn">OpenVPN (.ovpn)</option>
            </select>
          </div>

          <div className="form-group">
            <label className="form-label">{t('group_placeholder')}</label>
            <input
              type="text"
              className="form-control"
              placeholder="VIP"
              value={group}
              onChange={e => setGroup(e.target.value)}
            />
          </div>

          <div className="form-group">
            <label className="form-label">{t('config_content')}</label>
            <textarea
              className="form-control"
              rows={6}
              placeholder="{ ... JSON or Raw config ... }"
              value={configContent}
              onChange={e => setConfigContent(e.target.value)}
              style={{ fontFamily: 'var(--font-mono)', fontSize: '11px' }}
            />
          </div>
        </div>

        <div className="modal-footer">
          <button className="btn btn-secondary" onClick={onClose}>
            {t('cancel')}
          </button>
          <button className="btn btn-primary" onClick={handleSave} disabled={isSaving || !name.trim()}>
            {t('save')}
          </button>
        </div>
      </div>
    </div>
  )
}
