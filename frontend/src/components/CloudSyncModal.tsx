import React, { useState, useEffect } from 'react'
import { X, Cloud, Download, Upload, Copy, Check, Sparkles, RefreshCw, Key } from 'lucide-react'
import { api } from '../lib/api'
import { sound } from '../lib/sound'
import { t } from '../lib/i18n'

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
    <div className="fixed inset-0 z-50 flex items-center justify-center bg-black/80 backdrop-blur-md p-4 animate-fade-in">
      <div className="bg-slate-900 border border-slate-700/80 rounded-2xl w-full max-w-xl overflow-hidden shadow-2xl flex flex-col space-y-4 p-6">
        {/* Header */}
        <div className="flex items-center justify-between border-b border-slate-800 pb-3">
          <div className="flex items-center gap-2.5">
            <div className="w-9 h-9 rounded-xl bg-sky-500/10 border border-sky-500/30 flex items-center justify-center text-sky-400 shadow-inner">
              <Cloud size={18} />
            </div>
            <div>
              <h3 className="font-bold text-slate-100 text-sm">{t('cloud_sync_title')}</h3>
              <p className="text-xs text-slate-400">{t('cloud_sync_subtitle')}</p>
            </div>
          </div>
          <button onClick={onClose} className="text-slate-400 hover:text-slate-200 p-1 rounded-lg hover:bg-slate-800 transition">
            <X className="w-5 h-5" />
          </button>
        </div>

        {/* Tab Selector */}
        <div className="flex items-center gap-2 border-b border-slate-800 pb-2 text-xs">
          <button
            onClick={() => setActiveTab('feed')}
            className={`px-3 py-1.5 rounded-lg font-bold flex items-center gap-1.5 transition ${
              activeTab === 'feed' ? 'bg-sky-500/20 text-sky-300 border border-sky-500/30' : 'text-slate-400 hover:text-white'
            }`}
          >
            <Sparkles size={13} />
            <span>{t('cloud_tab_feed')}</span>
          </button>

          <button
            onClick={() => setActiveTab('backup')}
            className={`px-3 py-1.5 rounded-lg font-bold flex items-center gap-1.5 transition ${
              activeTab === 'backup' ? 'bg-sky-500/20 text-sky-300 border border-sky-500/30' : 'text-slate-400 hover:text-white'
            }`}
          >
            <Key size={13} />
            <span>{t('cloud_tab_backup')}</span>
          </button>
        </div>

        {/* TAB 1: Clean IP Feed */}
        {activeTab === 'feed' && (
          <div className="space-y-2.5">
            <div className="flex items-center justify-between text-xs text-slate-400">
              <span>{t('cloud_feed_header')}</span>
              <button
                onClick={loadFeed}
                className="text-sky-400 hover:text-sky-300 flex items-center gap-1 text-[11px]"
                disabled={isLoadingFeed}
              >
                <RefreshCw size={11} className={isLoadingFeed ? 'animate-spin' : ''} />
                <span>{t('cloud_refresh_feed')}</span>
              </button>
            </div>

            <div className="space-y-1.5 max-h-56 overflow-y-auto pr-1">
              {feedRanges.map(r => (
                <div
                  key={r.cidr}
                  className="bg-slate-950/70 p-2.5 rounded-xl border border-white/5 flex items-center justify-between text-xs"
                >
                  <div>
                    <div className="font-mono font-bold text-sky-300">{r.cidr}</div>
                    <div className="text-[11px] text-slate-400">{getFeedDescription(r.cidr, r.description)}</div>
                  </div>
                  <span className="text-[10px] bg-slate-800 text-slate-300 px-2 py-0.5 rounded font-mono">
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
            {/* Export Section */}
            <div className="bg-slate-950/60 p-3 rounded-xl border border-white/5 space-y-2">
              <span className="font-bold text-slate-200 block">{t('cloud_backup_title')}</span>
              <div className="flex items-center gap-2">
                <button
                  onClick={handleGenerateBackup}
                  className="btn btn-secondary btn-sm text-xs flex items-center gap-1"
                >
                  <Download size={13} />
                  <span>{t('cloud_generate_backup')}</span>
                </button>
                {backupPayload && (
                  <button
                    onClick={handleCopyBackup}
                    className="btn btn-primary btn-sm text-xs flex items-center gap-1"
                  >
                    {isCopied ? <Check size={13} className="text-emerald-400" /> : <Copy size={13} />}
                    <span>{isCopied ? t('cloud_copied') : t('cloud_copy_backup')}</span>
                  </button>
                )}
              </div>
            </div>

            {/* Import Section */}
            <div className="bg-slate-950/60 p-3 rounded-xl border border-white/5 space-y-2">
              <span className="font-bold text-slate-200 block">{t('cloud_restore_title')}</span>
              <textarea
                placeholder={t('cloud_restore_placeholder')}
                className="form-control font-mono text-[11px] w-full h-16 resize-none"
                value={importPayload}
                onChange={e => setImportPayload(e.target.value)}
              />
              <button
                onClick={handleRestoreBackup}
                disabled={!importPayload.trim()}
                className="btn btn-secondary btn-sm text-xs flex items-center gap-1"
              >
                <Upload size={13} />
                <span>{t('cloud_restore_btn')}</span>
              </button>
              {importStatus === 'success' && (
                <span className="text-emerald-400 text-xs block">{t('cloud_restore_success')}</span>
              )}
              {importStatus === 'error' && (
                <span className="text-rose-400 text-xs block">{t('cloud_restore_error')}</span>
              )}
            </div>
          </div>
        )}

        {/* Footer */}
        <div className="flex items-center justify-end pt-2 border-t border-slate-800">
          <button className="btn btn-secondary btn-sm text-xs" onClick={onClose}>
            {t('close')}
          </button>
        </div>
      </div>
    </div>
  )
}
