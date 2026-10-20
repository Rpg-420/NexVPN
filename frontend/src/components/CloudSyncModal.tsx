import { useState, useEffect } from 'react'
import { X, Cloud, Download, Upload, Copy, Check, Sparkles, RefreshCw, Key } from 'lucide-react'
import { api } from '../lib/api'
import { sound } from '../lib/sound'
import { t } from '../lib/i18n'
import { ModalShell } from '../components/ui/modal-shell'
import { Button } from '@/components/ui/button'

interface Props {
  isOpen: boolean
  onClose: () => void
}

export default function CloudSyncModal({ isOpen, onClose }: Props) {
  const [activeTab, setActiveTab] = useState<'feed' | 'backup'>('feed')
  const [feedRanges, setFeedRanges] = useState<Array<{ cidr: string; region: string; description: string }>>([])
  const [isLoadingFeed, setIsLoadingFeed] = useState(false)
  const [backupPassword, setBackupPassword] = useState('')
  const [backupPayload, setBackupPayload] = useState('')
  const [importPayload, setImportPayload] = useState('')
  const [isCopied, setIsCopied] = useState(false)
  const [importStatus, setImportStatus] = useState<string | null>(null)

  const loadFeed = async () => {
    setIsLoadingFeed(true)
    try {
      const res = await api.fetchLiveCleanIPFeed()
      if (res && Array.isArray(res)) setFeedRanges(res)
    } catch (err) {
      console.error('Fetch feed error:', err)
    } finally {
      setIsLoadingFeed(false)
    }
  }

  useEffect(() => {
    if (isOpen) {
      loadFeed()
    }
  }, [isOpen])

  if (!isOpen) return null

  const getFeedDescription = (cidr: string, fallback: string) => {
    const map: Record<string, string> = {
      '162.159.192.0/24': t('cloud_feed_162'),
      '188.114.96.0/24': t('cloud_feed_188'),
      '104.16.0.0/13': t('cloud_feed_104'),
      '172.64.0.0/13': t('cloud_feed_172'),
      '198.41.128.0/17': t('cloud_feed_198'),
      '141.101.64.0/18': t('cloud_feed_141'),
    }
    return map[cidr] || fallback
  }

  const handleGenerateBackup = async () => {
    try {
      const payload = await api.exportEncryptedBackup(backupPassword)
      setBackupPayload(payload)
      sound.playSuccess()
    } catch (err) {
      console.error('Generate backup error:', err)
    }
  }

  const handleCopyBackup = () => {
    if (!backupPayload) return
    navigator.clipboard.writeText(backupPayload)
    setIsCopied(true)
    sound.playSuccess()
    setTimeout(() => setIsCopied(false), 2000)
  }

  const handleRestoreBackup = async () => {
    if (!importPayload.trim()) return
    try {
      await api.importEncryptedBackup(importPayload, backupPassword)
      setImportStatus('success')
      sound.playSuccess()
      setTimeout(() => {
        setImportStatus(null)
        setImportPayload('')
      }, 2500)
    } catch (err: any) {
      setImportStatus('error')
    }
  }

  return (
    <ModalShell
      isOpen={isOpen}
      onClose={onClose}
      maxWidth="max-w-xl"
      bodyClassName="space-y-4 p-6"
    >
        {/* Header */}
        <div className="flex items-center justify-between border-b border-border pb-3">
          <div className="flex items-center gap-2.5">
            <div className="w-9 h-9 rounded-xl border border-border bg-muted flex items-center justify-center text-muted-foreground shadow-inner">
              <Cloud size={18} />
            </div>
            <div>
              <h3 className="font-bold text-foreground text-sm">{t('cloud_sync_title')}</h3>
              <p className="text-xs text-muted-foreground">{t('cloud_sync_subtitle')}</p>
            </div>
          </div>
          <button onClick={onClose} className="text-muted-foreground hover:text-foreground p-1 rounded-lg hover:bg-muted transition">
            <X className="w-5 h-5" />
          </button>
        </div>

        {/* Tab Selector */}
        <div className="flex items-center gap-2 border-b border-border pb-2 text-xs">
          <button
            onClick={() => setActiveTab('feed')}
            className={`px-3 py-1.5 rounded-lg font-bold flex items-center gap-1.5 transition ${
              activeTab === 'feed' ? 'bg-primary/20 text-primary border border-primary/30' : 'text-muted-foreground hover:text-primary'
            }`}
          >
            <Sparkles size={13} />
            <span>{t('cloud_tab_feed')}</span>
          </button>

          <button
            onClick={() => setActiveTab('backup')}
            className={`px-3 py-1.5 rounded-lg font-bold flex items-center gap-1.5 transition ${
              activeTab === 'backup' ? 'bg-primary/20 text-primary border border-primary/30' : 'text-muted-foreground hover:text-primary'
            }`}
          >
            <Key size={13} />
            <span>{t('cloud_tab_backup')}</span>
          </button>
        </div>

        {/* TAB 1: Clean IP Feed */}
        {activeTab === 'feed' && (
          <div className="space-y-2.5">
            <div className="flex items-center justify-between text-xs text-muted-foreground">
              <span>{t('cloud_feed_header')}</span>
              <button
                onClick={loadFeed}
                className="text-primary hover:text-primary flex items-center gap-1 text-[11px]"
                disabled={isLoadingFeed}
              >
                <RefreshCw size={11} className={isLoadingFeed ? 'animate-spin' : ''} />
                <span>{t('cloud_refresh_feed')}</span>
              </button>
            </div>

            <div className="space-y-1.5 max-h-56 overflow-y-auto pe-1">
              {feedRanges.map(r => (
                <div
                  key={r.cidr}
                  className="bg-background p-2.5 rounded-xl border border-border flex items-center justify-between text-xs"
                >
                  <div>
                    <div className="font-mono font-bold text-primary">{r.cidr}</div>
                    <div className="text-[11px] text-muted-foreground">{getFeedDescription(r.cidr, r.description)}</div>
                  </div>
                  <span className="text-[10px] bg-muted text-muted-foreground px-2 py-0.5 rounded font-mono">
                    {r.region}
                  </span>
                </div>
              ))}
            </div>
          </div>
        )}

        {/* TAB 2: Backup & Restore */}
        {activeTab === 'backup' && (
          <div className="space-y-3 text-xs">
            {/* Encryption password (shared by export & import) */}
            <div className="bg-background p-3 rounded-xl border border-border space-y-2">
              <label className="font-bold text-foreground block">{t('settings_backup_password_label')}</label>
              <input
                type="password"
                className="form-control text-xs w-full"
                placeholder={t('settings_backup_password_placeholder')}
                value={backupPassword}
                onChange={e => setBackupPassword(e.target.value)}
              />
            </div>

            {/* Export Section */}
            <div className="bg-background p-3 rounded-xl border border-border space-y-2">
              <span className="font-bold text-foreground block">{t('cloud_backup_title')}</span>
              <div className="flex items-center gap-2">
                <Button variant="secondary" size="sm" className="text-xs flex items-center gap-1"
                  onClick={handleGenerateBackup}
                >
                  <Download size={13} />
                  <span>{t('cloud_generate_backup')}</span>
                </Button>
                {backupPayload && (
                  <Button size="sm" className="text-xs flex items-center gap-1"
                    onClick={handleCopyBackup}
                  >
                    {isCopied ? <Check size={13} className="text-success" /> : <Copy size={13} />}
                    <span>{isCopied ? t('cloud_copied') : t('cloud_copy_backup')}</span>
                  </Button>
                )}
              </div>
            </div>

            {/* Import Section */}
            <div className="bg-background p-3 rounded-xl border border-border space-y-2">
              <span className="font-bold text-foreground block">{t('cloud_restore_title')}</span>
              <textarea
                placeholder={t('cloud_restore_placeholder')}
                className="form-control font-mono text-[11px] w-full h-16 resize-none"
                value={importPayload}
                onChange={e => setImportPayload(e.target.value)}
              />
              <Button variant="secondary" size="sm" className="text-xs flex items-center gap-1"
                onClick={handleRestoreBackup}
                disabled={!importPayload.trim()}
              >
                <Upload size={13} />
                <span>{t('cloud_restore_btn')}</span>
              </Button>
              {importStatus === 'success' && (
                <span className="text-success text-xs block">{t('cloud_restore_success')}</span>
              )}
              {importStatus === 'error' && (
                <span className="text-destructive text-xs block">{t('cloud_restore_error')}</span>
              )}
            </div>
          </div>
        )}

        {/* Footer */}
        <div className="flex items-center justify-end pt-2 border-t border-border">
          <Button variant="secondary" size="sm" className="text-xs" onClick={onClose}>
            {t('close')}
          </Button>
        </div>
      </ModalShell>
  )
}
