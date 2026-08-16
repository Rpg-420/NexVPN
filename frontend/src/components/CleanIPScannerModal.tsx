import React, { useState, useEffect } from 'react'
import {
  X,
  Play,
  Square,
  Check,
  Copy,
  Zap,
  RefreshCw,
  Globe,
  Shield,
  Layers,
  CheckSquare,
  Square as SquareOutline,
  Sparkles,
  ArrowRight,
  Filter,
} from 'lucide-react'
import { api } from '../lib/api'
import { t } from '../lib/i18n'
import type { CleanIPResult, ScanProgress } from '../types'

interface Props {
  isOpen: boolean
  onClose: () => void
  onSelectIP: (ip: string, port: number) => void
  onProfilesCreated?: () => void
}

export default function CleanIPScannerModal({ isOpen, onClose, onSelectIP, onProfilesCreated }: Props) {
  const [isScanning, setIsScanning] = useState(false)
  const [selectedIPs, setSelectedIPs] = useState<CleanIPResult[]>([])
  const [copiedText, setCopiedText] = useState<string | null>(null)
  const [filterMode, setFilterMode] = useState<'all' | 'fast' | 'noloss'>('all')
  const [isBatchCreating, setIsBatchCreating] = useState(false)
  const [progress, setProgress] = useState<ScanProgress>({
    total: 0,
    scanned: 0,
    found: 0,
    current_ip: '',
    results: [],
  })

  useEffect(() => {
    if (!isOpen) return

    const unsubProg = api.onEvent('scanner:progress', (p: ScanProgress) => {
      setProgress(prev => ({
        ...p,
        results: Array.isArray(p?.results) ? p.results : prev.results || [],
      }))
      setIsScanning(true)
    })

    const unsubDone = api.onEvent('scanner:complete', (results: CleanIPResult[]) => {
      setIsScanning(false)
      const list = Array.isArray(results) ? results : []
      setProgress(prev => ({
        ...prev,
        results: list,
      }))
      // Auto select top 5
      if (list.length > 0) {
        setSelectedIPs(list.slice(0, Math.min(5, list.length)))
      }
    })

    return () => {
      if (typeof unsubProg === 'function') unsubProg()
      if (typeof unsubDone === 'function') unsubDone()
    }
  }, [isOpen])

  if (!isOpen) return null

  const handleStartScan = async () => {
    setIsScanning(true)
    setSelectedIPs([])
    setProgress({ total: 100, scanned: 0, found: 0, current_ip: 'Checking Anycast feeds...', results: [] })
    await api.startCleanIPScan()
  }

  const handleStopScan = async () => {
    setIsScanning(false)
    await api.stopCleanIPScan()
  }

  const toggleSelectIP = (item: CleanIPResult) => {
    const exists = selectedIPs.some(x => x.ip === item.ip && x.port === item.port)
    if (exists) {
      setSelectedIPs(selectedIPs.filter(x => !(x.ip === item.ip && x.port === item.port)))
    } else {
      setSelectedIPs([...selectedIPs, item])
    }
  }

  const handleSelectAll = () => {
    const list = Array.isArray(progress.results) ? progress.results : []
    if (selectedIPs.length === list.length) {
      setSelectedIPs([])
    } else {
      setSelectedIPs([...list])
    }
  }

  const handleCopySingle = (ip: string, port: number, e: React.MouseEvent) => {
    e.stopPropagation()
    const text = `${ip}:${port}`
    navigator.clipboard.writeText(text)
    setCopiedText(text)
    setTimeout(() => setCopiedText(null), 1500)
  }

  const handleCopyAll = () => {
    const list = selectedIPs.length > 0 ? selectedIPs : progress.results || []
    const lines = list.map(r => `${r.ip}:${r.port} (${r.latency_ms}ms - ${r.location || 'Cloudflare'})`).join('\n')
    navigator.clipboard.writeText(lines)
    setCopiedText('all')
    setTimeout(() => setCopiedText(null), 2000)
  }

  const handleBatchCreate = async (useAmnezia: boolean) => {
    const targets = selectedIPs.length > 0 ? selectedIPs : (progress.results || []).slice(0, 5)
    if (targets.length === 0) return

    setIsBatchCreating(true)
    try {
      await api.batchGenerateWarpProfiles(
        targets.map(t => ({ ip: t.ip, port: t.port })),
        useAmnezia
      )
      if (onProfilesCreated) onProfilesCreated()
      onClose()
    } catch (err) {
      console.error('Batch create error:', err)
      alert(t('error'))
    } finally {
      setIsBatchCreating(false)
    }
  }

  const handleSingleCreate = async (item: CleanIPResult, useAmnezia: boolean, e: React.MouseEvent) => {
    e.stopPropagation()
    setIsBatchCreating(true)
    try {
      await api.batchGenerateWarpProfiles([{ ip: item.ip, port: item.port }], useAmnezia)
      if (onProfilesCreated) onProfilesCreated()
      onSelectIP(item.ip, item.port)
      onClose()
    } catch (err) {
      console.error('Single create error:', err)
    } finally {
      setIsBatchCreating(false)
    }
  }

  const rawResults = Array.isArray(progress.results) ? progress.results : []
  const filteredResults = rawResults.filter(r => {
    if (filterMode === 'fast') return r.latency_ms < 100
    if (filterMode === 'noloss') return r.loss_rate === 0
    return true
  })

  const percent = progress.total > 0 ? Math.min(100, Math.round((progress.scanned / progress.total) * 100)) : 0

  return (
    <div className="fixed inset-0 z-50 flex items-center justify-center bg-black/80 backdrop-blur-md p-4 animate-fade-in">
      <div className="bg-slate-900 border border-slate-700/80 rounded-2xl w-full max-w-3xl overflow-hidden shadow-2xl flex flex-col max-h-[90vh]">
        {/* Header */}
        <div className="flex items-center justify-between px-6 py-4 border-b border-slate-800 bg-slate-900/80">
          <div className="flex items-center gap-3">
            <div className="w-10 h-10 rounded-xl bg-gradient-to-tr from-orange-500/20 to-amber-500/20 border border-orange-500/30 flex items-center justify-center text-orange-400">
              <Zap className="w-5 h-5" />
            </div>
            <div>
              <h3 className="text-base font-bold text-slate-100 flex items-center gap-2">
                {t('scanner_title')}
                <span className="text-xs bg-orange-500/20 text-orange-300 px-2.5 py-0.5 rounded-full font-medium">
                  Smart Zero-Config
                </span>
              </h3>
              <p className="text-xs text-slate-400">
                {t('scanner_subtitle')}
              </p>
            </div>
          </div>
          <button
            onClick={onClose}
            className="text-slate-400 hover:text-slate-200 p-1.5 rounded-lg hover:bg-slate-800 transition"
          >
            <X className="w-5 h-5" />
          </button>
        </div>

        {/* Content */}
        <div className="p-6 overflow-y-auto flex-1 space-y-5">
          {/* Main Action Bar */}
          <div className="flex items-center justify-between bg-gradient-to-r from-slate-800/80 to-slate-855/80 border border-slate-700/60 p-4 rounded-xl">
            <div className="space-y-1">
              <div className="text-sm font-bold text-slate-100 flex items-center gap-2">
                {isScanning ? (
                  <span className="flex items-center gap-2 text-cyan-400">
                    <RefreshCw className="w-4 h-4 animate-spin" />
                    {t('scanner_scanning')}
                  </span>
                ) : rawResults.length > 0 ? (
                  <span className="text-emerald-400 flex items-center gap-1.5">
                    <Check className="w-4 h-4" />
                    {rawResults.length} {t('scanner_found_count')}
                  </span>
                ) : (
                  <span className="text-slate-200">{t('scanner_ready')}</span>
                )}
              </div>
              <div className="text-xs text-slate-400 font-mono">
                {progress.current_ip ? `IP: ${progress.current_ip}` : 'Cloudflare Anycast Global Subnets'}
              </div>
            </div>

            <div className="flex items-center gap-2">
              {isScanning ? (
                <button
                  onClick={handleStopScan}
                  className="flex items-center gap-2 bg-rose-600 hover:bg-rose-500 text-white px-5 py-2.5 rounded-xl text-xs font-bold transition shadow-lg shadow-rose-600/20"
                >
                  <Square className="w-4 h-4 fill-current" />
                  {t('scanner_stop')}
                </button>
              ) : (
                <button
                  onClick={handleStartScan}
                  className="flex items-center gap-2 bg-gradient-to-r from-orange-500 to-amber-500 hover:from-orange-400 hover:to-amber-400 text-slate-950 font-bold px-6 py-2.5 rounded-xl text-xs transition shadow-lg shadow-orange-500/25 active:scale-95"
                >
                  <Play className="w-4 h-4 fill-current" />
                  {t('scanner_start')}
                </button>
              )}
            </div>
          </div>

          {/* Progress Bar */}
          {isScanning && (
            <div className="space-y-1.5">
              <div className="flex justify-between text-xs text-slate-400">
                <span>{t('scanner_progress')}: {percent}%</span>
                <span className="font-mono">{progress.scanned} / {progress.total} {t('scanner_targets')}</span>
              </div>
              <div className="h-2.5 bg-slate-800 rounded-full overflow-hidden">
                <div
                  className="h-full bg-gradient-to-r from-orange-500 via-amber-400 to-emerald-400 transition-all duration-300"
                  style={{ width: `${percent}%` }}
                />
              </div>
            </div>
          )}

          {/* Filter and Selection Tools */}
          {rawResults.length > 0 && (
            <div className="flex flex-wrap items-center justify-between gap-3 bg-slate-800/40 border border-slate-700/50 p-3 rounded-xl">
              {/* Filter Pills */}
              <div className="flex items-center gap-2">
                <button
                  onClick={() => setFilterMode('all')}
                  className={`text-xs px-3 py-1.5 rounded-lg font-medium transition ${
                    filterMode === 'all'
                      ? 'bg-orange-500/20 border border-orange-500/50 text-orange-300 font-bold'
                      : 'text-slate-400 hover:text-slate-200'
                  }`}
                >
                  {t('scanner_filter_all')} ({rawResults.length})
                </button>
                <button
                  onClick={() => setFilterMode('fast')}
                  className={`text-xs px-3 py-1.5 rounded-lg font-medium transition ${
                    filterMode === 'fast'
                      ? 'bg-emerald-500/20 border border-emerald-500/50 text-emerald-300 font-bold'
                      : 'text-slate-400 hover:text-slate-200'
                  }`}
                >
                  {t('scanner_filter_fast')}
                </button>
                <button
                  onClick={() => setFilterMode('noloss')}
                  className={`text-xs px-3 py-1.5 rounded-lg font-medium transition ${
                    filterMode === 'noloss'
                      ? 'bg-cyan-500/20 border border-cyan-500/50 text-cyan-300 font-bold'
                      : 'text-slate-400 hover:text-slate-200'
                  }`}
                >
                  {t('scanner_filter_noloss')}
                </button>
              </div>

              {/* Selection Summary & Batch Actions */}
              <div className="flex items-center gap-2">
                <button
                  onClick={handleSelectAll}
                  className="flex items-center gap-1.5 text-xs text-slate-300 hover:text-white px-2.5 py-1.5 rounded-lg hover:bg-slate-700/50 transition font-medium"
                >
                  {selectedIPs.length === rawResults.length ? (
                    <CheckSquare className="w-4 h-4 text-orange-400" />
                  ) : (
                    <SquareOutline className="w-4 h-4 text-slate-400" />
                  )}
                  <span>{selectedIPs.length === rawResults.length ? t('scanner_deselect_all') : t('scanner_select_all')}</span>
                </button>

                <button
                  onClick={handleCopyAll}
                  className="flex items-center gap-1 text-xs text-slate-300 hover:text-white px-2.5 py-1.5 rounded-lg hover:bg-slate-700/50 transition"
                  title={t('scanner_copy_list')}
                >
                  {copiedText === 'all' ? <Check className="w-4 h-4 text-emerald-400" /> : <Copy className="w-4 h-4" />}
                  <span>{t('scanner_copy_list')}</span>
                </button>
              </div>
            </div>
          )}

          {/* Batch Floating Actions if Items Selected */}
          {selectedIPs.length > 0 && (
            <div className="bg-gradient-to-r from-orange-500/10 via-amber-500/10 to-emerald-500/10 border border-orange-500/30 p-3.5 rounded-xl flex items-center justify-between gap-3 animate-fade-in">
              <div className="text-xs font-semibold text-slate-200 flex items-center gap-2">
                <Sparkles className="w-4 h-4 text-orange-400" />
                <span>{selectedIPs.length} {t('scanner_selected_count')}</span>
              </div>

              <div className="flex items-center gap-2">
                <button
                  onClick={() => handleBatchCreate(false)}
                  disabled={isBatchCreating}
                  className="flex items-center gap-1.5 bg-gradient-to-r from-orange-500 to-amber-500 hover:from-orange-400 hover:to-amber-400 text-slate-950 font-bold px-4 py-2 rounded-lg text-xs transition shadow-md shadow-orange-500/20"
                >
                  <Zap className="w-3.5 h-3.5 fill-current" />
                  {isBatchCreating ? t('submitting') : t('scanner_batch_warp')}
                </button>

                <button
                  onClick={() => handleBatchCreate(true)}
                  disabled={isBatchCreating}
                  className="flex items-center gap-1.5 bg-emerald-600 hover:bg-emerald-500 text-white font-bold px-4 py-2 rounded-lg text-xs transition shadow-md shadow-emerald-600/20"
                >
                  <Shield className="w-3.5 h-3.5" />
                  {isBatchCreating ? t('submitting') : t('scanner_batch_amnezia')}
                </button>
              </div>
            </div>
          )}

          {/* Results List */}
          <div className="space-y-2">
            {filteredResults.length === 0 ? (
              <div className="text-center py-12 border border-dashed border-slate-800 rounded-xl text-slate-500 text-xs">
                {isScanning ? t('scanner_searching_hint') : t('scanner_no_results')}
              </div>
            ) : (
              <div className="space-y-2 max-h-[340px] overflow-y-auto pr-1">
                {filteredResults.map((res, idx) => {
                  const isSelected = selectedIPs.some(x => x.ip === res.ip && x.port === res.port)
                  return (
                    <div
                      key={`${res.ip}:${res.port}`}
                      onClick={() => toggleSelectIP(res)}
                      className={`group flex items-center justify-between p-3.5 border rounded-xl transition cursor-pointer ${
                        isSelected
                          ? 'bg-orange-500/10 border-orange-500/60 shadow-sm'
                          : 'bg-slate-800/40 hover:bg-slate-800 border-slate-700/40 hover:border-slate-600'
                      }`}
                    >
                      {/* Left info */}
                      <div className="flex items-center gap-3">
                        <input
                          type="checkbox"
                          checked={isSelected}
                          onChange={() => toggleSelectIP(res)}
                          onClick={e => e.stopPropagation()}
                          className="w-4 h-4 rounded border-slate-700 text-orange-500 focus:ring-0 cursor-pointer"
                        />
                        <span className="w-6 h-6 rounded-full bg-slate-700/60 text-slate-300 text-xs font-bold flex items-center justify-center font-mono">
                          {idx + 1}
                        </span>
                        <div>
                          <div className="font-mono font-bold text-slate-100 group-hover:text-orange-400 transition text-sm flex items-center gap-2">
                            <span>{res.ip}:{res.port}</span>
                            <span className="text-[10px] bg-slate-700/60 text-slate-300 px-2 py-0.5 rounded-full font-sans font-medium">
                              {res.location || 'Cloudflare Anycast'}
                            </span>
                          </div>
                          <div className="text-[11px] text-slate-400 font-mono flex items-center gap-2 mt-0.5">
                            <span>{t('scanner_item_port')} {res.port}</span>
                            <span>•</span>
                            <span className={res.loss_rate === 0 ? 'text-emerald-400' : 'text-amber-400'}>
                              {t('scanner_item_loss')} {res.loss_rate || 0}%
                            </span>
                          </div>
                        </div>
                      </div>

                      {/* Right actions */}
                      <div className="flex items-center gap-2.5">
                        <span
                          className={`text-xs font-bold px-3 py-1.5 rounded-lg font-mono ${
                            res.latency_ms < 80
                              ? 'bg-emerald-500/20 text-emerald-300 border border-emerald-500/30'
                              : res.latency_ms < 140
                              ? 'bg-amber-500/20 text-amber-300 border border-amber-500/30'
                              : 'bg-rose-500/20 text-rose-300 border border-rose-500/30'
                          }`}
                        >
                          {res.latency_ms} ms
                        </span>

                        <button
                          onClick={e => handleCopySingle(res.ip, res.port, e)}
                          className="p-2 text-slate-400 hover:text-slate-200 hover:bg-slate-700 rounded-lg transition"
                          title="Copy IP:Port"
                        >
                          {copiedText === `${res.ip}:${res.port}` ? (
                            <Check className="w-4 h-4 text-emerald-400" />
                          ) : (
                            <Copy className="w-4 h-4" />
                          )}
                        </button>

                        <button
                          onClick={e => handleSingleCreate(res, false, e)}
                          className="text-xs bg-orange-500/20 hover:bg-orange-500 text-orange-300 hover:text-slate-950 font-bold px-3 py-1.5 rounded-lg transition flex items-center gap-1"
                          title="WARP"
                        >
                          <Zap className="w-3.5 h-3.5" />
                          {t('scanner_item_create_warp')}
                        </button>

                        <button
                          onClick={e => handleSingleCreate(res, true, e)}
                          className="text-xs bg-emerald-500/20 hover:bg-emerald-600 text-emerald-300 hover:text-white font-bold px-3 py-1.5 rounded-lg transition flex items-center gap-1"
                          title="AmneziaWG"
                        >
                          <Shield className="w-3.5 h-3.5" />
                          {t('scanner_item_create_amnezia')}
                        </button>
                      </div>
                    </div>
                  )
                })}
              </div>
            )}
          </div>
        </div>
      </div>
    </div>
  )
}
