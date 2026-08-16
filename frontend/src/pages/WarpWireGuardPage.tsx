import React, { useState, useEffect, useCallback } from 'react'
import {
  Zap,
  Plus,
  Trash2,
  Play,
  Square,
  Copy,
  Check,
  Shield,
  Upload,
  Globe,
  Radio,
  X,
  CheckCircle2,
  Search,
  QrCode,
  Key,
  Layers,
  Activity,
  Edit3,
  RefreshCw,
} from 'lucide-react'
import { api } from '../lib/api'
import { t } from '../lib/i18n'
import type { Profile, ConnectionStatus } from '../types'
import { generateQRCodeSVG } from '../lib/qrcode'
import CleanIPScannerModal from '../components/CleanIPScannerModal'
import EditProfileModal from '../components/EditProfileModal'

interface Props {
  status: ConnectionStatus
  onStatusChange?: (s: ConnectionStatus) => void
}

type SortMode = 'default' | 'ping_asc' | 'ping_desc' | 'name'

export default function WarpWireGuardPage({ status, onStatusChange }: Props) {
  const [profiles, setProfiles] = useState<Profile[]>([])
  const [search, setSearch] = useState('')
  const [sortMode, setSortModeState] = useState<SortMode>(() => {
    return (localStorage.getItem('nexvpn_warp_sort') as SortMode) || 'default'
  })
  const setSortMode = (mode: SortMode) => {
    setSortModeState(mode)
    localStorage.setItem('nexvpn_warp_sort', mode)
  }
  const [isGeneratingWarp, setIsGeneratingWarp] = useState(false)
  const [isGeneratingWoW, setIsGeneratingWoW] = useState(false)
  const [selectedCleanIP, setSelectedCleanIP] = useState('162.159.192.1')
  const [customPort, setCustomPort] = useState(2408)
  const [showImportModal, setShowImportModal] = useState(false)
  const [showScannerModal, setShowScannerModal] = useState(false)
  const [showLicenseModal, setShowLicenseModal] = useState(false)
  const [editingProfile, setEditingProfile] = useState<Profile | null>(null)
  const [licenseKeyInput, setLicenseKeyInput] = useState('')
  const [isApplyingLicense, setIsApplyingLicense] = useState(false)
  const [sharingProfile, setSharingProfile] = useState<Profile | null>(null)
  const [copiedId, setCopiedId] = useState<string | null>(null)
  const [isPinging, setIsPinging] = useState(false)
  const [toastMessage, setToastMessage] = useState<string | null>(null)

  const showToast = (msg: string) => {
    setToastMessage(msg)
    setTimeout(() => setToastMessage(null), 3000)
  }

  const loadProfiles = useCallback(async () => {
    try {
      const list = await api.getProfilesByCategory('wireguard_warp')
      if (list && Array.isArray(list)) setProfiles(list)
    } catch (err) {
      console.error('Failed to load WireGuard/WARP profiles:', err)
    }
  }, [])

  useEffect(() => {
    loadProfiles()
  }, [loadProfiles])

  const handleGenerateWarp = async () => {
    setIsGeneratingWarp(true)
    try {
      await api.generateWarpProfile(selectedCleanIP, customPort)
      await loadProfiles()
      showToast(t('saved_successfully'))
    } catch (err) {
      console.error('Generate WARP error:', err)
      alert(t('error'))
    } finally {
      setIsGeneratingWarp(false)
    }
  }

  const handleGenerateWarpOnWarp = async () => {
    setIsGeneratingWoW(true)
    try {
      await api.generateWarpOnWarpProfile(selectedCleanIP, customPort, '162.159.193.10', 2408)
      await loadProfiles()
      showToast(t('saved_successfully'))
    } catch (err) {
      console.error('Generate WoW error:', err)
      alert(t('error'))
    } finally {
      setIsGeneratingWoW(false)
    }
  }

  const handleDeduplicate = async () => {
    try {
      const count = await api.deduplicateProfiles('wireguard_warp')
      await loadProfiles()
      showToast(count > 0 ? `${count} duplicate profiles removed` : 'No duplicate profiles found')
    } catch (err) {
      console.error('Deduplicate error:', err)
    }
  }

  const handleRemoveDead = async () => {
    if (window.confirm(t('confirm_delete'))) {
      try {
        const count = await api.removeDeadProfiles('wireguard_warp')
        await loadProfiles()
        showToast(count > 0 ? `${count} dead profiles removed` : 'No dead profiles in list')
      } catch (err) {
        console.error('Remove dead error:', err)
      }
    }
  }

  const handleExportAll = async () => {
    try {
      const data = await api.exportAllProfiles('wireguard_warp')
      if (!data) {
        showToast(t('no_profiles_found'))
        return
      }
      await navigator.clipboard.writeText(data)
      showToast(t('link_copied'))
    } catch (err) {
      console.error('Export all error:', err)
    }
  }

  const handleApplyLicense = async () => {
    if (!licenseKeyInput.trim()) return
    setIsApplyingLicense(true)
    try {
      await api.applyWarpLicenseKey(licenseKeyInput.trim())
      setShowLicenseModal(false)
      setLicenseKeyInput('')
      await loadProfiles()
      alert(t('saved_successfully'))
    } catch (err) {
      console.error('Apply license error:', err)
      alert(t('error'))
    } finally {
      setIsApplyingLicense(false)
    }
  }

  const handlePingAll = async () => {
    setIsPinging(true)
    try {
      const updated = await api.pingAllProfiles('wireguard_warp')
      setProfiles(updated)
    } catch (err) {
      console.error('Ping all error:', err)
    } finally {
      setIsPinging(false)
    }
  }

  const handlePingSingle = async (id: string, e: React.MouseEvent) => {
    e.stopPropagation()
    try {
      const ms = await api.pingProfile(id)
      setProfiles(prev => prev.map(p => (p.id === id ? { ...p, latency_ms: ms } : p)))
    } catch (err) {
      console.error('Ping single error:', err)
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

  const handleCopyURI = (p: Profile, e: React.MouseEvent) => {
    e.stopPropagation()
    const text = p.raw_uri || p.config_content || p.name
    navigator.clipboard.writeText(text)
    setCopiedId(p.id)
    setTimeout(() => setCopiedId(null), 2000)
  }

  const filtered = profiles.filter(p =>
    !search || p.name.toLowerCase().includes(search.toLowerCase()) || p.server?.includes(search)
  )

  // Apply sorting
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

  const qrImageUrl = sharingProfile
    ? generateQRCodeSVG(sharingProfile.raw_uri || sharingProfile.config_content || sharingProfile.name)
    : ''

  return (
    <div className="page space-y-5">
      {/* Header */}
      <div className="page-header flex items-center justify-between flex-wrap gap-3">
        <div>
          <h1 className="page-title">{t('warp_title')}</h1>
          <p className="page-subtitle">{t('warp_subtitle')}</p>
        </div>
        <div className="flex items-center gap-2">
          <button
            className="btn btn-secondary btn-sm"
            onClick={() => setShowScannerModal(true)}
          >
            <Zap size={14} className="text-amber-400" />
            {t('scanner_btn')}
          </button>
          <button
            className="btn btn-secondary btn-sm"
            onClick={() => setShowLicenseModal(true)}
          >
            <Key size={14} className="text-cyan-400" />
            {t('warp_license_btn')}
          </button>
          <button
            id="import-wg-btn"
            className="btn btn-secondary btn-sm"
            onClick={() => setShowImportModal(true)}
          >
            <Upload size={14} />
            {t('add_wireguard_conf')}
          </button>
        </div>
      </div>

      {/* Cloudflare WARP 1-Click Generator Hero Card */}
      <div className="card p-5 bg-gradient-to-br from-amber-950/20 via-slate-900/80 to-slate-900 border-amber-500/30 shadow-xl">
        <div className="flex items-start justify-between gap-4 flex-wrap">
          <div>
            <div className="flex items-center gap-2 mb-2">
              <span className="badge badge-warning text-[10px]">Cloudflare Official</span>
              <span className="badge badge-success text-[10px]">Unlimited Bandwidth</span>
              <span className="badge badge-info text-[10px]">WireGuard Protocol</span>
            </div>
            <h2 className="text-base font-bold text-slate-100">
              {t('warp_generator_title')}
            </h2>
            <p className="text-xs text-slate-400 max-w-xl mt-1">
              {t('warp_generator_desc')}
            </p>
          </div>

          <div className="flex items-center gap-2">
            <button
              id="generate-warp-btn"
              className="btn btn-primary bg-gradient-to-r from-amber-500 to-orange-600 hover:from-amber-400 hover:to-orange-500 text-white font-bold py-2.5 px-4 rounded-xl shadow-lg shadow-amber-500/25 text-xs active:scale-[0.99]"
              onClick={handleGenerateWarp}
              disabled={isGeneratingWarp}
            >
              <Zap size={14} className="fill-current" />
              {isGeneratingWarp ? t('generating_warp') : t('generate_warp_btn')}
            </button>

            <button
              id="generate-wow-btn"
              className="btn btn-secondary text-xs font-bold py-2.5 px-4 rounded-xl border border-purple-500/40 text-purple-300 hover:bg-purple-500/10 shadow-lg shadow-purple-500/10 active:scale-[0.99]"
              onClick={handleGenerateWarpOnWarp}
              disabled={isGeneratingWoW}
              title="WARP-on-WARP"
            >
              <Shield size={14} className="text-purple-400" />
              {isGeneratingWoW ? t('generating_wow') : t('generate_wow_btn')}
            </button>
          </div>
        </div>

        <div className="flex items-center justify-between gap-4 mt-4 pt-4 border-t border-slate-800/80 flex-wrap">
          <div className="flex items-center gap-3 flex-wrap">
            <div className="flex items-center gap-2">
              <label className="text-xs text-slate-400 font-semibold">{t('clean_ip_label')}:</label>
              <select
                className="form-control text-xs font-mono w-44"
                value={selectedCleanIP}
                onChange={e => setSelectedCleanIP(e.target.value)}
              >
                <option value="162.159.192.1">162.159.192.1 (Standard)</option>
                <option value="162.159.193.1">162.159.193.1 (IR Clean)</option>
                <option value="162.159.195.1">162.159.195.1 (Fast RTT)</option>
                <option value="188.114.96.1">188.114.96.1 (EU)</option>
                <option value="188.114.97.1">188.114.97.1 (EU Clean)</option>
              </select>
            </div>

            <div className="flex items-center gap-2">
              <label className="text-xs text-slate-400 font-semibold">{t('port_label')}:</label>
              <input
                type="number"
                className="form-control text-xs font-mono w-24"
                value={customPort}
                onChange={e => setCustomPort(parseInt(e.target.value) || 2408)}
              />
            </div>
          </div>

          <button
            onClick={() => setShowScannerModal(true)}
            className="btn btn-secondary btn-sm text-xs text-amber-400 border-amber-500/30 hover:bg-amber-500/10"
          >
            <Search size={13} />
            {t('scanner_btn')}...
          </button>
        </div>
      </div>

      {/* Profile List Controls & Automation Toolbar */}
      <div className="card p-3 flex items-center justify-between flex-wrap gap-2">
        <div className="flex items-center gap-2 flex-1" style={{ maxWidth: '420px' }}>
          <div className="search-bar flex-1">
            <Search size={14} />
            <input
              type="text"
              placeholder={t('search_profiles')}
              value={search}
              onChange={e => setSearch(e.target.value)}
              className="text-xs"
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

        <div className="flex items-center gap-1.5 flex-wrap">
          <button
            className="btn btn-secondary btn-sm text-xs"
            onClick={handlePingAll}
            disabled={isPinging}
          >
            <Radio size={13} className="text-emerald-400" />
            <span>{isPinging ? t('pinging_all') : t('ping_all')}</span>
          </button>

          <button
            className="btn btn-secondary btn-sm text-xs"
            onClick={handleRemoveDead}
            title={t('remove_dead')}
          >
            <Zap size={13} className="text-amber-400" />
            <span>{t('remove_dead')}</span>
          </button>

          <button
            className="btn btn-secondary btn-sm text-xs"
            onClick={handleDeduplicate}
            title={t('remove_duplicates')}
          >
            <Layers size={13} className="text-purple-400" />
            <span>{t('remove_duplicates')}</span>
          </button>

          <button
            className="btn btn-secondary btn-sm text-xs"
            onClick={handleExportAll}
            title={t('copy_all')}
          >
            <Copy size={13} className="text-blue-400" />
            <span>{t('copy_all')}</span>
          </button>
        </div>
      </div>

      {/* Profiles Grid */}
      {filtered.length === 0 ? (
        <div className="card text-center p-12 space-y-3">
          <Globe size={48} className="mx-auto text-slate-600 mb-2 opacity-40" />
          <h3 className="font-bold text-sm text-slate-200">
            {t('no_profiles_found')}
          </h3>
          <p className="text-xs text-slate-400 max-w-sm mx-auto">
            {t('warp_empty_hint')}
          </p>
          <button className="btn btn-primary btn-sm mx-auto" onClick={handleGenerateWarp}>
            <Zap size={14} />
            {t('generate_warp_btn')}
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
                    ? 'border-amber-500/80 shadow-lg shadow-amber-500/20 bg-amber-950/20 ring-1 ring-amber-500/40'
                    : 'hover:border-slate-700 bg-slate-900/60 border-white/5'
                }`}
              >
                <div>
                  <div className="flex items-center justify-between gap-2 mb-1.5 min-w-0">
                    <div className="flex items-center gap-2 flex-1 min-w-0">
                      <span className="text-lg shrink-0 leading-none">⚡</span>
                      <h4 className="font-bold text-xs truncate min-w-0 text-slate-100" title={p.name}>
                        {p.name}
                      </h4>
                    </div>

                    <span className="text-[9px] font-mono uppercase px-1.5 py-0.5 rounded bg-amber-500/10 text-amber-400 border border-amber-500/30 shrink-0 font-bold">
                      WARP
                    </span>
                  </div>

                  <div className="flex items-center justify-between text-[11px] text-slate-400 font-mono mb-2.5 gap-2">
                    <span className="truncate flex-1 text-slate-400">{p.server}:{p.port}</span>
                    <span className="text-[10px] px-1.5 py-0.5 rounded bg-slate-800/90 text-slate-300 truncate shrink-0">
                      WireGuard
                    </span>
                  </div>
                </div>

                <div className="flex items-center justify-between pt-2.5 border-t border-white/5 gap-1">
                  <button
                    type="button"
                    className="px-2 py-1 rounded-lg bg-slate-800/80 hover:bg-slate-800 text-[11px] text-slate-300 transition flex items-center gap-1 font-mono border border-slate-700/50 shrink-0"
                    onClick={e => handlePingSingle(p.id, e)}
                    title={t('ping_test')}
                  >
                    <Activity size={11} className={p.latency_ms && p.latency_ms > 0 ? 'text-emerald-400' : 'text-slate-500'} />
                    <span
                      className={
                        p.latency_ms && p.latency_ms > 0
                          ? p.latency_ms < 150
                            ? 'text-emerald-400 font-bold'
                            : p.latency_ms < 300
                            ? 'text-amber-400 font-bold'
                            : 'text-rose-400 font-bold'
                          : 'text-slate-400'
                      }
                    >
                      {p.latency_ms && p.latency_ms > 0 ? `${p.latency_ms}ms` : 'Ping'}
                    </span>
                  </button>

                  <div className="flex items-center gap-0.5 shrink-0">
                    <button
                      type="button"
                      className="p-1.5 rounded-md text-slate-400 hover:text-slate-200 hover:bg-slate-800 transition"
                      title={t('qr_code_modal_title')}
                      onClick={e => {
                        e.stopPropagation()
                        setSharingProfile(p)
                      }}
                    >
                      <QrCode size={13} />
                    </button>

                    <button
                      type="button"
                      className="p-1.5 rounded-md text-slate-400 hover:text-slate-200 hover:bg-slate-800 transition"
                      title={t('copy_config_link')}
                      onClick={e => handleCopyURI(p, e)}
                    >
                      {copiedId === p.id ? <Check size={13} className="text-emerald-400" /> : <Copy size={13} />}
                    </button>

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
                        : 'bg-amber-500 hover:bg-amber-400 text-slate-950 shadow-sm shadow-amber-500/20'
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

      {/* Clean IP Scanner Modal */}
      <CleanIPScannerModal
        isOpen={showScannerModal}
        onClose={() => setShowScannerModal(false)}
        onProfilesCreated={() => loadProfiles()}
        onSelectIP={(ip, port) => {
          setSelectedCleanIP(ip)
          setCustomPort(port)
          api.generateWarpProfile(ip, port).then(() => loadProfiles())
        }}
      />

      {/* Profile Edit Modal */}
      <EditProfileModal
        isOpen={!!editingProfile}
        profile={editingProfile}
        onClose={() => setEditingProfile(null)}
        onSaved={() => loadProfiles()}
        onDeleted={id => setProfiles(prev => prev.filter(p => p.id !== id))}
      />

      {/* WARP+ License Modal */}
      {showLicenseModal && (
        <div className="fixed inset-0 z-50 flex items-center justify-center bg-black/80 backdrop-blur-md p-4 animate-fade-in">
          <div className="bg-slate-900 border border-slate-700/80 rounded-2xl w-full max-w-md overflow-hidden shadow-2xl p-6 space-y-4">
            <div className="flex items-center justify-between border-b border-slate-800 pb-3">
              <div className="flex items-center gap-2 text-cyan-400 font-bold text-sm">
                <Key className="w-5 h-5" />
                {t('warp_license_title')}
              </div>
              <button onClick={() => setShowLicenseModal(false)} className="text-slate-400 hover:text-slate-200">
                <X className="w-5 h-5" />
              </button>
            </div>

            <p className="text-xs text-slate-400">
              {t('warp_license_desc')}
            </p>

            <input
              type="text"
              placeholder="xxxxxxxx-xxxxxxxx-xxxxxxxx"
              className="form-control w-full font-mono text-xs"
              value={licenseKeyInput}
              onChange={e => setLicenseKeyInput(e.target.value)}
            />

            <div className="flex justify-end gap-2 pt-2 border-t border-slate-800">
              <button className="btn btn-secondary btn-sm" onClick={() => setShowLicenseModal(false)}>
                {t('warp_license_cancel')}
              </button>
              <button
                className="btn btn-primary btn-sm"
                onClick={handleApplyLicense}
                disabled={isApplyingLicense || !licenseKeyInput.trim()}
              >
                {isApplyingLicense ? '...' : t('warp_license_apply')}
              </button>
            </div>
          </div>
        </div>
      )}

      {/* WireGuard Import Modal */}
      {showImportModal && (
        <ImportWireGuardModal
          isOpen={showImportModal}
          onClose={() => setShowImportModal(false)}
          onImported={() => {
            setShowImportModal(false)
            loadProfiles()
          }}
        />
      )}

      {/* QR Code Sharing Modal */}
      {sharingProfile && (
        <div className="fixed inset-0 z-50 flex items-center justify-center bg-black/80 backdrop-blur-md p-4 animate-fade-in">
          <div className="bg-slate-900 border border-slate-700/80 rounded-2xl w-full max-w-sm overflow-hidden shadow-2xl p-6 text-center space-y-4">
            <h3 className="font-bold text-slate-200 text-sm truncate">{sharingProfile.name}</h3>
            {qrImageUrl && (
              <div
                className="bg-white p-3 rounded-xl mx-auto inline-block shadow-lg"
                dangerouslySetInnerHTML={{ __html: qrImageUrl }}
              />
            )}
            <button className="btn btn-secondary w-full text-xs" onClick={() => setSharingProfile(null)}>
              {t('close')}
            </button>
          </div>
        </div>
      )}
    </div>
  )
}

function ImportWireGuardModal({
  isOpen,
  onClose,
  onImported,
}: {
  isOpen: boolean
  onClose: () => void
  onImported: () => void
}) {
  const [name, setName] = useState('')
  const [conf, setConf] = useState('')
  const [isSubmitting, setIsSubmitting] = useState(false)
  const fileInputRef = React.useRef<HTMLInputElement>(null)

  if (!isOpen) return null

  const handleFileUpload = (e: React.ChangeEvent<HTMLInputElement>) => {
    const file = e.target.files?.[0]
    if (!file) return
    const filename = file.name.replace(/\.(conf|json|txt)$/i, '')
    if (!name) setName(filename)

    const reader = new FileReader()
    reader.onload = event => {
      const text = event.target?.result as string
      if (text) setConf(text)
    }
    reader.readAsText(file)
  }

  const handleApplyAmneziaHeaders = () => {
    if (!conf.includes('Jc =')) {
      const amneziaParams = `\n# AmneziaWG Anti-DPI Obfuscation\nJc = 4\nJmin = 40\nJmax = 70\nS1 = 15\nS2 = 20\nH1 = 1\nH2 = 2\nH3 = 3\nH4 = 4\n`
      if (conf.includes('[Interface]')) {
        setConf(conf.replace('[Interface]', `[Interface]${amneziaParams}`))
      } else {
        setConf(conf + amneziaParams)
      }
    }
  }

  const handleSubmit = async (e: React.FormEvent) => {
    e.preventDefault()
    if (!conf.trim()) return
    setIsSubmitting(true)
    try {
      await api.importWireGuardProfile(name || 'WireGuard Profile', conf.trim())
      onImported()
    } catch (err) {
      console.error('Import WG error:', err)
      alert('Failed to import WireGuard config')
    } finally {
      setIsSubmitting(false)
    }
  }

  return (
    <div className="fixed inset-0 z-50 flex items-center justify-center bg-black/80 backdrop-blur-md p-4 animate-fade-in">
      <div className="bg-slate-900 border border-slate-700/80 rounded-2xl w-full max-w-lg overflow-hidden shadow-2xl p-6 space-y-4">
        <div className="flex items-center justify-between border-b border-slate-800 pb-3">
          <div className="flex items-center gap-2">
            <Upload className="w-4 h-4 text-cyan-400" />
            <h3 className="font-bold text-slate-200 text-sm">{t('add_wg_modal_title')}</h3>
          </div>
          <button onClick={onClose} className="text-slate-400 hover:text-slate-200">
            <X className="w-5 h-5" />
          </button>
        </div>

        {/* File Upload Trigger */}
        <div
          onClick={() => fileInputRef.current?.click()}
          className="border-2 border-dashed border-slate-700 hover:border-cyan-500/70 bg-slate-800/40 hover:bg-slate-800/80 rounded-xl p-4 text-center cursor-pointer transition flex flex-col items-center justify-center gap-1.5"
        >
          <Upload className="w-6 h-6 text-slate-400" />
          <div className="text-xs font-semibold text-slate-200">
            {t('add_wg_drag_drop')}
          </div>
          <div className="text-[11px] text-slate-400">
            {t('add_wg_browse_hint')}
          </div>
          <input
            type="file"
            ref={fileInputRef}
            onChange={handleFileUpload}
            accept=".conf,.json,.txt"
            className="hidden"
          />
        </div>

        <form onSubmit={handleSubmit} className="space-y-3">
          <div>
            <label className="text-xs text-slate-400 font-medium block mb-1">{t('add_wg_profile_name')}</label>
            <input
              type="text"
              placeholder={t('add_wg_name_placeholder')}
              className="form-control w-full text-xs"
              value={name}
              onChange={e => setName(e.target.value)}
            />
          </div>

          <div>
            <div className="flex items-center justify-between mb-1">
              <label className="text-xs text-slate-400 font-medium">{t('add_wg_conf_content')}</label>
              <button
                type="button"
                onClick={handleApplyAmneziaHeaders}
                className="text-[11px] text-cyan-400 hover:text-cyan-300 font-semibold hover:underline"
              >
                {t('add_wg_add_amnezia_headers')}
              </button>
            </div>
            <textarea
              rows={7}
              placeholder="[Interface]&#10;PrivateKey = ...&#10;Address = 10.0.0.2/32&#10;&#10;[Peer]&#10;PublicKey = ...&#10;Endpoint = 1.2.3.4:51820"
              className="form-control w-full font-mono text-xs"
              value={conf}
              onChange={e => setConf(e.target.value)}
              required
            />
          </div>

          <div className="flex justify-end gap-2 pt-2 border-t border-slate-800">
            <button type="button" className="btn btn-secondary btn-sm" onClick={onClose}>
              {t('cancel')}
            </button>
            <button type="submit" className="btn btn-primary btn-sm" disabled={isSubmitting || !conf.trim()}>
              {isSubmitting ? t('submitting') : t('add_wg_submit')}
            </button>
          </div>
        </form>
      </div>
    </div>
  )
}
