import React, { useState, useEffect, useCallback } from 'react'
import {
  Zap,
  Copy,
  Check,
  Shield,
  Upload,
  Globe,
  Radio,
  X,
  Search,
  QrCode,
  Key,
  Layers,
  Activity,
  Scan,
  ArrowRightLeft,
} from 'lucide-react'
import { api } from '../lib/api'
import { t } from '../lib/i18n'
import type { Profile, ConnectionStatus } from '../types'
import { generateQRCodeSVG } from '../lib/qrcode'
import CleanIPScannerModal from '../components/CleanIPScannerModal'
import EditProfileModal from '../components/EditProfileModal'
import ScreenQRScannerModal from '../components/ScreenQRScannerModal'
import ConfigConverterModal from '../components/ConfigConverterModal'
import { ModalShell } from '../components/ui/modal-shell'
import { ProfileCard } from '../components/ui/profile-card'
import { PingProgressBar } from '../components/PingProgressBar'
import { usePingProgress } from '../hooks/usePingProgress'
import { pinnedFirst, toggleProfilePin } from '../lib/profilePin'
import { Badge } from '@/components/ui/badge'
import { Button } from '@/components/ui/button'
import { Card } from '@/components/ui/card'

interface Props {
  status: ConnectionStatus
  onStatusChange?: (s: ConnectionStatus) => void
  onNavigateTab?: (tab: any) => void
}

type SortMode = 'default' | 'ping_asc' | 'ping_desc' | 'name'

export default function WarpWireGuardPage({ status, onStatusChange, onNavigateTab }: Props) {
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
  const [selectedCleanIP, setSelectedCleanIP] = useState('8.6.112.31')
  const [customPort, setCustomPort] = useState(1180)
  const [showImportModal, setShowImportModal] = useState(false)
  const [showScannerModal, setShowScannerModal] = useState(false)
  const [showLicenseModal, setShowLicenseModal] = useState(false)
  const [showScreenQRModal, setShowScreenQRModal] = useState(false)
  const [showConverterModal, setShowConverterModal] = useState(false)
  const [editingProfile, setEditingProfile] = useState<Profile | null>(null)
  const [licenseKeyInput, setLicenseKeyInput] = useState('')
  const [isApplyingLicense, setIsApplyingLicense] = useState(false)
  const [warpLicenseKey, setWarpLicenseKey] = useState('')
  const [warpAccountType, setWarpAccountType] = useState<'free' | 'plus' | string>('free')
  const [sharingProfile, setSharingProfile] = useState<Profile | null>(null)
  const [copiedId, setCopiedId] = useState<string | null>(null)
  const [isPinging, setIsPinging] = useState(false)
  const pingProgress = usePingProgress(isPinging)
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

  // Seed the Clean IP / port inputs from the persisted settings so a value
  // discovered by an optimal-IP scan is actually reused instead of silently
  // falling back to the hardcoded default on every page visit.
  useEffect(() => {
    let cancelled = false
    ;(async () => {
      try {
        const s = await api.getSettings()
        if (cancelled || !s) return
        if (s.warp_clean_ip) setSelectedCleanIP(s.warp_clean_ip)
        if (s.warp_port && s.warp_port > 0) setCustomPort(s.warp_port)
        setWarpLicenseKey(s.warp_license_key || '')
        setWarpAccountType(s.warp_account_type || 'free')
      } catch (err) {
        console.error('Failed to load WARP settings:', err)
      }
    })()
    return () => {
      cancelled = true
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
    } catch (err: any) {
      console.error('Generate WARP error:', err)
      showToast(err?.message || String(err) || t('error'))
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
    } catch (err: any) {
      console.error('Generate WoW error:', err)
      showToast(err?.message || String(err) || t('error'))
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
    const key = licenseKeyInput.trim()
    setIsApplyingLicense(true)
    try {
      await api.applyWarpLicenseKey(key)
      setShowLicenseModal(false)
      setLicenseKeyInput('')
      // Reflect the newly persisted state instead of assuming it.
      setWarpLicenseKey(key)
      setWarpAccountType('plus')
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
    if (isPinging) return
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

  const handleCancelPingAll = () => {
    api.cancelPingAll().catch(() => {})
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

  // Pinned profiles lead, whichever sort is active — see lib/profilePin.ts.
  filtered.sort(pinnedFirst)

  const qrImageUrl = sharingProfile
    ? generateQRCodeSVG(sharingProfile.raw_uri || sharingProfile.config_content || sharingProfile.name)
    : ''

  return (
    <div className="page space-y-5">
      {toastMessage && (
        <div
          role="status"
          aria-live="polite"
          className="fixed bottom-6 left-1/2 -translate-x-1/2 z-[60] px-4 py-2 rounded-xl bg-card border border-primary/40 text-primary text-xs font-semibold shadow-2xl animate-fade-in"
        >
          {toastMessage}
        </div>
      )}
      {/* Header */}
      <div className="page-header sticky top-0 z-30 flex items-center justify-between flex-wrap gap-3 bg-card/85 dark:bg-card backdrop-blur-xl py-3 -mx-2 px-4 rounded-2xl border border-border/80 dark:border-white/5 shadow-sm dark:shadow-lg dark:shadow-black/20">
        <div>
          <h1 className="page-title">{t('warp_title')}</h1>
          <p className="page-subtitle">{t('warp_subtitle')}</p>
        </div>
        <div className="flex items-center gap-2 flex-wrap">
          <Button variant="secondary" size="sm" className="flex items-center gap-1.5"
            onClick={() => setShowScreenQRModal(true)}
            title="Scan QR Code from Screen or Image"
          >
            <Scan size={14} className="text-primary" />
            <span>{t('screen_qr_title') || 'Scan QR'}</span>
          </Button>

          <Button variant="secondary" size="sm" className="flex items-center gap-1.5"
            onClick={() => setShowConverterModal(true)}
            title="Universal Config Converter"
          >
            <ArrowRightLeft size={14} className="text-warning" />
            <span>{t('config_converter_title') || 'Converter'}</span>
          </Button>

          <Button variant="secondary" size="sm"
            onClick={() => (onNavigateTab ? onNavigateTab('discovery') : setShowScannerModal(true))}
            title="Discovery Hub Cloudflare Scanner"
          >
            <Zap size={14} className="text-warning" />
            {t('scanner_btn')}
          </Button>
          <Button variant="secondary" size="sm"
            onClick={() => setShowLicenseModal(true)}
          >
            <Key size={14} className="text-primary" />
            {t('warp_license_btn')}
          </Button>
          <Button variant="secondary" size="sm"
            id="import-wg-btn"
            onClick={() => setShowImportModal(true)}
          >
            <Upload size={14} />
            {t('add_wireguard_conf')}
          </Button>
        </div>
      </div>

      {/* Cloudflare WARP 1-Click Generator Hero Card */}
      <Card className="p-5">
        <div className="flex flex-wrap items-start justify-between gap-4">
          <div>
            <div className="mb-2 flex flex-wrap items-center gap-2">
              <Badge variant="secondary">Cloudflare Official</Badge>
              <Badge variant="secondary">AmneziaWG Anti-DPI</Badge>
              <Badge variant="secondary">Unlimited Bandwidth</Badge>
              <Badge variant="secondary">WireGuard Protocol</Badge>
            </div>
            <h2 className="text-base font-semibold text-foreground">
              {t('warp_generator_title')}
            </h2>
            <p className="mt-1 max-w-xl text-xs text-muted-foreground">
              {t('warp_generator_desc')}
            </p>
          </div>

          <div className="flex items-center gap-2">
            <Button
              id="generate-warp-btn"
              size="sm"
              onClick={handleGenerateWarp}
              disabled={isGeneratingWarp}
            >
              <Zap className="h-3.5 w-3.5 fill-current" />
              {isGeneratingWarp ? t('generating_warp') : t('generate_warp_btn')}
            </Button>

            <Button
              id="generate-wow-btn"
              variant="outline"
              size="sm"
              onClick={handleGenerateWarpOnWarp}
              disabled={isGeneratingWoW}
              title="WARP-on-WARP"
            >
              <Shield className="h-3.5 w-3.5" />
              {isGeneratingWoW ? t('generating_wow') : t('generate_wow_btn')}
            </Button>
          </div>
        </div>

        <div className="flex items-center justify-between gap-4 mt-4 pt-4 border-t border-border flex-wrap">
          <div className="flex items-center gap-3 flex-wrap">
            <div className="flex items-center gap-2">
              <label className="text-xs text-muted-foreground font-semibold">{t('clean_ip_label')}:</label>
              <select
                className="form-control text-xs font-mono w-52 bg-card text-foreground border-border"
                value={selectedCleanIP}
                onChange={e => {
                  setSelectedCleanIP(e.target.value)
                  if (e.target.value === '8.6.112.31') setCustomPort(1180)
                  else if (e.target.value === '8.6.112.135') setCustomPort(864)
                  else if (e.target.value === '188.114.96.22') setCustomPort(878)
                  else if (e.target.value === '188.114.97.105') setCustomPort(1070)
                  else if (e.target.value === '162.159.193.10') setCustomPort(854)
                }}
              >
                <option value="8.6.112.31" className="bg-card text-foreground">8.6.112.31:1180 (⚡ Anycast Ultra Fast)</option>
                <option value="8.6.112.135" className="bg-card text-foreground">8.6.112.135:864 (⚡ Anycast Clean)</option>
                <option value="188.114.96.22" className="bg-card text-foreground">188.114.96.22:878 (EU Clean)</option>
                <option value="188.114.97.105" className="bg-card text-foreground">188.114.97.105:1070 (EU Fast)</option>
                <option value="162.159.193.10" className="bg-card text-foreground">162.159.193.10:854 (IR Clean)</option>
                <option value="162.159.192.1" className="bg-card text-foreground">162.159.192.1:2408 (Standard)</option>
              </select>
            </div>

            <div className="flex items-center gap-2">
              <label className="text-xs text-muted-foreground font-semibold">{t('port_label')}:</label>
              <input
                type="number"
                className="form-control text-xs font-mono w-20 bg-card text-foreground border-border"
                value={customPort}
                onChange={e => setCustomPort(parseInt(e.target.value) || 1180)}
              />
            </div>
          </div>

          <Button variant="secondary" size="sm" className="text-xs text-primary border-primary/30 hover:bg-primary/10 flex items-center gap-1.5"
            onClick={() => (onNavigateTab ? onNavigateTab('discovery') : setShowScannerModal(true))}
            title="Discovery Hub Cloudflare Scanner"
          >
            <Search size={13} />
            <span>🚀 {t('tab_cloudflare_ips')}</span>
          </Button>
        </div>
      </Card>

      {/* Profile List Controls & Automation Toolbar */}
      <Card className="p-3 flex items-center justify-between flex-wrap gap-2">
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
            className="form-control text-xs font-semibold bg-card text-foreground border-border hover:border-primary/50 transition cursor-pointer shrink-0"
            style={{ width: '130px' }}
            value={sortMode}
            onChange={e => setSortMode(e.target.value as SortMode)}
          >
            <option value="default" className="bg-card text-foreground">{t('sort_default')}</option>
            <option value="ping_asc" className="bg-card text-foreground">⚡ {t('sort_ping_asc')}</option>
            <option value="ping_desc" className="bg-card text-foreground">{t('sort_ping_desc')}</option>
            <option value="name" className="bg-card text-foreground">🔤 {t('sort_name')}</option>
          </select>
        </div>

        <div className="flex items-center gap-1.5 flex-wrap">
          <Button variant="secondary" size="sm" className="text-xs"
            onClick={handlePingAll}
            disabled={isPinging}
          >
            <Radio size={13} className="text-success" />
            <span>{isPinging ? t('pinging_all') : t('ping_all')}</span>
          </Button>

          <Button variant="secondary" size="sm" className="text-xs"
            onClick={handleRemoveDead}
            title={t('remove_dead')}
          >
            <Zap size={13} className="text-warning" />
            <span>{t('remove_dead')}</span>
          </Button>

          <Button variant="secondary" size="sm" className="text-xs"
            onClick={handleDeduplicate}
            title={t('remove_duplicates')}
          >
            <Layers size={13} className="text-primary" />
            <span>{t('remove_duplicates')}</span>
          </Button>

          <Button variant="secondary" size="sm" className="text-xs"
            onClick={handleExportAll}
            title={t('copy_all')}
          >
            <Copy size={13} className="text-primary" />
            <span>{t('copy_all')}</span>
          </Button>
        </div>
      </Card>

      <PingProgressBar progress={pingProgress} onCancel={handleCancelPingAll} />

      {/* Profiles Grid */}
      {filtered.length === 0 ? (
        <Card className="text-center p-12 space-y-3">
          <Globe size={48} className="mx-auto text-muted-foreground mb-2 opacity-40" />
          <h3 className="font-bold text-sm text-foreground">
            {t('no_profiles_found')}
          </h3>
          <p className="text-xs text-muted-foreground max-w-sm mx-auto">
            {t('warp_empty_hint')}
          </p>
          <Button size="sm" className="mx-auto" onClick={handleGenerateWarp}>
            <Zap size={14} />
            {t('generate_warp_btn')}
          </Button>
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
                icon="⚡"
                protocolLabel="WARP"
                metaSlot={
                  <>
                    <span className="truncate flex-1">{p.server}:{p.port}</span>
                    <span className="text-[10px] px-1.5 py-0.5 rounded bg-muted text-muted-foreground truncate shrink-0">
                      WireGuard
                    </span>
                  </>
                }
                actionsSlot={
                  <>
                    <button
                      type="button"
                      className="px-2 py-1 rounded-lg bg-muted hover:bg-muted/90 text-[11px] text-muted-foreground transition flex items-center gap-1 font-mono shrink-0"
                      onClick={e => handlePingSingle(p.id, e)}
                      title={t('ping_test')}
                    >
                      <Activity size={11} className={p.latency_ms && p.latency_ms > 0 ? 'text-success' : 'text-muted-foreground'} />
                      <span
                        className={
                          p.latency_ms && p.latency_ms > 0
                            ? p.latency_ms < 150
                              ? 'text-success font-bold'
                              : p.latency_ms < 300
                              ? 'text-warning font-bold'
                              : 'text-destructive font-bold'
                            : 'text-muted-foreground'
                        }
                      >
                        {p.latency_ms && p.latency_ms > 0 ? `${p.latency_ms}ms` : 'Ping'}
                      </span>
                    </button>

                    <button
                      type="button"
                      className="p-1.5 rounded-md text-muted-foreground hover:text-foreground hover:bg-muted transition"
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
                      className="p-1.5 rounded-md text-muted-foreground hover:text-foreground hover:bg-muted transition"
                      title={t('copy_config_link')}
                      onClick={e => handleCopyURI(p, e)}
                    >
                      {copiedId === p.id ? <Check size={13} className="text-success" /> : <Copy size={13} />}
                    </button>
                  </>
                }
                state={isConnected ? 'connected' : isConnecting ? 'connecting' : 'idle'}
                connectClassName="bg-primary hover:bg-primary/90 text-primary-foreground shadow-sm"
                onConnect={() => handleConnect(p.id)}
                onDisconnect={handleDisconnect}
                onEdit={() => setEditingProfile(p)}
                onDelete={() => handleDelete(p.id)}
                pinned={!!p.favorite}
                onTogglePin={() => toggleProfilePin(p, setProfiles)}
                pinLabels={{ pin: t('adv_pin'), unpin: t('adv_unpin') }}
                labels={{ connect: t('connect'), disconnect: t('disconnect'), edit: t('edit'), delete: t('delete') }}
              />
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
        <ModalShell
          isOpen={showLicenseModal}
          onClose={() => setShowLicenseModal(false)}
          maxWidth="max-w-md"
          bodyClassName="p-6 space-y-4"
        >
            <div className="flex items-center justify-between border-b border-border pb-3">
              <div className="flex items-center gap-2 text-primary font-bold text-sm">
                <Key className="w-5 h-5" />
                {t('warp_license_title')}
              </div>
              <button onClick={() => setShowLicenseModal(false)} className="text-muted-foreground hover:text-foreground">
                <X className="w-5 h-5" />
              </button>
            </div>

            <p className="text-xs text-muted-foreground">
              {t('warp_license_desc')}
            </p>

            {/* Live account status read back from settings */}
            <div className="flex items-center justify-between rounded-lg border border-border bg-background px-3 py-2">
              <span className="text-[11px] text-muted-foreground">{t('warp_account_status')}</span>
              <span
                className={`text-[11px] font-bold ${
                  warpAccountType === 'plus' ? 'text-success' : 'text-muted-foreground'
                }`}
              >
                {warpAccountType === 'plus' ? t('warp_account_plus') : t('warp_account_free')}
              </span>
            </div>
            {warpLicenseKey && (
              <div className="flex items-center justify-between rounded-lg border border-border bg-background px-3 py-2">
                <span className="text-[11px] text-muted-foreground">{t('warp_license_current')}</span>
                <span className="font-mono text-[11px] text-muted-foreground">
                  {warpLicenseKey.slice(0, 8)}…{warpLicenseKey.slice(-4)}
                </span>
              </div>
            )}

            <input
              type="text"
              placeholder="xxxxxxxx-xxxxxxxx-xxxxxxxx"
              className="form-control w-full font-mono text-xs"
              value={licenseKeyInput}
              onChange={e => setLicenseKeyInput(e.target.value)}
            />

            <div className="flex justify-end gap-2 pt-2 border-t border-border">
              <Button variant="secondary" size="sm" onClick={() => setShowLicenseModal(false)}>
                {t('warp_license_cancel')}
              </Button>
              <Button size="sm"
                onClick={handleApplyLicense}
                disabled={isApplyingLicense || !licenseKeyInput.trim()}
              >
                {isApplyingLicense ? '...' : t('warp_license_apply')}
              </Button>
            </div>
        </ModalShell>
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
        <ModalShell
          isOpen={true}
          onClose={() => setSharingProfile(null)}
          maxWidth="max-w-sm"
          bodyClassName="p-6 text-center space-y-4"
        >
            <h3 className="font-bold text-foreground text-sm truncate">{sharingProfile.name}</h3>
            {qrImageUrl && (
              <img
                src={qrImageUrl}
                alt="QR Code"
                className="bg-card p-3 rounded-xl mx-auto inline-block shadow-lg w-[220px] h-[220px]"
              />
            )}
            <Button variant="secondary" className="w-full text-xs" onClick={() => setSharingProfile(null)}>
              {t('close')}
            </Button>
        </ModalShell>
      )}

      {/* Screen QR Code Scanner Modal */}
      <ScreenQRScannerModal
        isOpen={showScreenQRModal}
        onClose={() => setShowScreenQRModal(false)}
        onProfileImported={loadProfiles}
      />

      {/* Universal Config Converter Modal */}
      <ConfigConverterModal
        isOpen={showConverterModal}
        onClose={() => setShowConverterModal(false)}
      />
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
      alert(t('alert_import_wg_failed'))
    } finally {
      setIsSubmitting(false)
    }
  }

  return (
    <ModalShell
      isOpen={isOpen}
      onClose={onClose}
      maxWidth="max-w-lg"
      bodyClassName="p-6 space-y-4"
    >
        <div className="flex items-center justify-between border-b border-border pb-3">
          <div className="flex items-center gap-2">
            <Upload className="w-4 h-4 text-primary" />
            <h3 className="font-bold text-foreground text-sm">{t('add_wg_modal_title')}</h3>
          </div>
          <button onClick={onClose} className="text-muted-foreground hover:text-foreground">
            <X className="w-5 h-5" />
          </button>
        </div>

        {/* File Upload Trigger */}
        <div
          onClick={() => fileInputRef.current?.click()}
          className="border-2 border-dashed border-border hover:border-primary/70 bg-muted hover:bg-muted/90 rounded-xl p-4 text-center cursor-pointer transition flex flex-col items-center justify-center gap-1.5"
        >
          <Upload className="w-6 h-6 text-muted-foreground" />
          <div className="text-xs font-semibold text-foreground">
            {t('add_wg_drag_drop')}
          </div>
          <div className="text-[11px] text-muted-foreground">
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
            <label className="text-xs text-muted-foreground font-medium block mb-1">{t('add_wg_profile_name')}</label>
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
              <label className="text-xs text-muted-foreground font-medium">{t('add_wg_conf_content')}</label>
              <button
                type="button"
                onClick={handleApplyAmneziaHeaders}
                className="text-[11px] text-primary hover:text-primary font-semibold hover:underline"
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

          <div className="flex justify-end gap-2 pt-2 border-t border-border">
            <Button variant="secondary" size="sm" type="button" onClick={onClose}>
              {t('cancel')}
            </Button>
            <Button size="sm" type="submit" disabled={isSubmitting || !conf.trim()}>
              {isSubmitting ? t('submitting') : t('add_wg_submit')}
            </Button>
          </div>
        </form>
    </ModalShell>
  )
}
