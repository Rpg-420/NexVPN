import React, { useState } from 'react'
import {
  Download,
  Upload,
  Archive,
  CheckCircle2,
  AlertCircle,
  Copy,
  Check,
  X,
  RefreshCw,
} from 'lucide-react'
import { api } from '../lib/api'
import { t } from '../lib/i18n'

interface Props {
  isOpen: boolean
  onClose: () => void
  onSuccess?: () => void
}

export default function FullBackupModal({ isOpen, onClose, onSuccess }: Props) {
  const [activeTab, setActiveTab] = useState<'export' | 'import'>('export')
  const [exportedJSON, setExportedJSON] = useState<string>('')
  const [importJSON, setImportJSON] = useState<string>('')
  const [overwriteMode, setOverwriteMode] = useState<boolean>(false)
  const [isLoading, setIsLoading] = useState<boolean>(false)
  const [copied, setCopied] = useState<boolean>(false)
  const [message, setMessage] = useState<{ text: string; type: 'success' | 'error' } | null>(null)

  const handleExport = async () => {
    setIsLoading(true)
    setMessage(null)
    try {
      const jsonStr = await api.exportFullBackup()
      setExportedJSON(jsonStr)
      setMessage({ text: t('backup_export_success'), type: 'success' })
    } catch (err: any) {
      setMessage({ text: err?.message || 'Failed to generate backup', type: 'error' })
    } finally {
      setIsLoading(false)
    }
  }

  const downloadBackupFile = () => {
    if (!exportedJSON) return
    const blob = new Blob([exportedJSON], { type: 'application/json' })
    const url = URL.createObjectURL(blob)
    const a = document.createElement('a')
    a.href = url
    a.download = `nexvpn_backup_${new Date().toISOString().slice(0, 10)}.json`
    a.click()
    URL.revokeObjectURL(url)
  }

  const handleCopy = () => {
    if (!exportedJSON) return
    navigator.clipboard.writeText(exportedJSON)
    setCopied(true)
    setTimeout(() => setCopied(false), 2000)
  }

  const handleImport = async () => {
    if (!importJSON.trim()) {
      setMessage({ text: 'Please paste the backup JSON data to import', type: 'error' })
      return
    }
    setIsLoading(true)
    setMessage(null)
    try {
      await api.importFullBackup(importJSON.trim(), overwriteMode)
      setMessage({ text: t('backup_import_success'), type: 'success' })
      if (onSuccess) onSuccess()
    } catch (err: any) {
      setMessage({ text: err?.message || 'Failed to import backup: invalid JSON format', type: 'error' })
    } finally {
      setIsLoading(false)
    }
  }

  const handleFileUpload = (e: React.ChangeEvent<HTMLInputElement>) => {
    const file = e.target.files?.[0]
    if (!file) return
    const reader = new FileReader()
    reader.onload = evt => {
      const content = evt.target?.result as string
      if (content) {
        setImportJSON(content)
      }
    }
    reader.readAsText(file)
  }

  if (!isOpen) return null

  return (
    <div className="fixed inset-0 z-50 flex items-center justify-center p-4 bg-black/70 backdrop-blur-md animate-fade-in">
      <div className="relative w-full max-w-2xl bg-neutral-900 border border-neutral-800 rounded-3xl shadow-2xl overflow-hidden flex flex-col max-h-[90vh]">
        {/* Header */}
        <div className="flex items-center justify-between p-6 border-b border-neutral-800/80 bg-neutral-950/40">
          <div className="flex items-center gap-3">
            <div className="w-10 h-10 rounded-2xl bg-blue-500/10 border border-blue-500/20 flex items-center justify-center text-blue-400">
              <Archive className="w-5 h-5" />
            </div>
            <div>
              <h3 className="text-lg font-bold text-white flex items-center gap-2">{t('backup_modal_title')}</h3>
              <p className="text-xs text-neutral-400">{t('backup_modal_sub')}</p>
            </div>
          </div>
          <button
            onClick={onClose}
            className="p-2 rounded-xl bg-neutral-800/60 hover:bg-neutral-700 text-neutral-400 hover:text-white transition-all"
          >
            <X className="w-4 h-4" />
          </button>
        </div>

        {/* Tab Switcher */}
        <div className="flex border-b border-neutral-800 bg-neutral-950/20 px-6 pt-2">
          <button
            onClick={() => {
              setActiveTab('export')
              if (!exportedJSON) handleExport()
            }}
            className={`pb-3 px-4 text-xs font-bold transition-all border-b-2 flex items-center gap-2 ${
              activeTab === 'export'
                ? 'border-blue-500 text-blue-400'
                : 'border-transparent text-neutral-400 hover:text-white'
            }`}
          >
            <Download className="w-4 h-4" />
            {t('backup_export_title')}
          </button>
          <button
            onClick={() => setActiveTab('import')}
            className={`pb-3 px-4 text-xs font-bold transition-all border-b-2 flex items-center gap-2 ${
              activeTab === 'import'
                ? 'border-blue-500 text-blue-400'
                : 'border-transparent text-neutral-400 hover:text-white'
            }`}
          >
            <Upload className="w-4 h-4" />
            {t('backup_import_title')}
          </button>
        </div>

        {/* Body */}
        <div className="p-6 overflow-y-auto space-y-4 flex-1">
          {message && (
            <div
              className={`p-4 rounded-2xl text-xs flex items-center gap-2.5 border ${
                message.type === 'success'
                  ? 'bg-emerald-500/10 border-emerald-500/20 text-emerald-300'
                  : 'bg-red-500/10 border-red-500/20 text-red-300'
              }`}
            >
              {message.type === 'success' ? (
                <CheckCircle2 className="w-4 h-4 text-emerald-400 flex-shrink-0" />
              ) : (
                <AlertCircle className="w-4 h-4 text-red-400 flex-shrink-0" />
              )}
              <span>{message.text}</span>
            </div>
          )}

          {activeTab === 'export' ? (
            <div className="space-y-4">
              <div className="p-4 rounded-2xl bg-neutral-950/50 border border-neutral-800 space-y-3">
                <p className="text-xs text-neutral-300 leading-relaxed">
                  {t('backup_export_sub')}
                </p>
                <div className="flex gap-2">
                  <button
                    onClick={downloadBackupFile}
                    disabled={!exportedJSON || isLoading}
                    className="px-4 py-2 rounded-xl bg-blue-600 hover:bg-blue-500 text-white text-xs font-semibold flex items-center gap-2 transition-all disabled:opacity-50"
                  >
                    <Download className="w-4 h-4" />
                    {t('backup_export_btn')}
                  </button>
                  <button
                    onClick={handleCopy}
                    disabled={!exportedJSON || isLoading}
                    className="px-4 py-2 rounded-xl bg-neutral-800 hover:bg-neutral-700 text-neutral-300 text-xs font-semibold flex items-center gap-2 transition-all disabled:opacity-50"
                  >
                    {copied ? <Check className="w-4 h-4 text-emerald-400" /> : <Copy className="w-4 h-4" />}
                    {copied ? t('remote_copied_btn') : t('remote_copy_btn')}
                  </button>
                </div>
              </div>

              {exportedJSON && (
                <div className="space-y-1.5">
                  <textarea
                    readOnly
                    value={exportedJSON}
                    className="w-full h-44 p-3.5 rounded-2xl bg-neutral-950/80 border border-neutral-800 text-neutral-300 font-mono text-xs focus:outline-none resize-none ltr"
                  />
                </div>
              )}
            </div>
          ) : (
            <div className="space-y-4">
              <div className="p-4 rounded-2xl bg-neutral-950/50 border border-neutral-800 space-y-3">
                <label className="block text-xs font-semibold text-neutral-300">{t('backup_import_btn')}</label>
                <input
                  type="file"
                  accept=".json,.nexvpn"
                  onChange={handleFileUpload}
                  className="block w-full text-xs text-neutral-400 file:mr-3 file:py-2 file:px-4 file:rounded-xl file:border-0 file:text-xs file:font-semibold file:bg-neutral-800 file:text-neutral-200 hover:file:bg-neutral-700"
                />
              </div>

              <div className="space-y-1.5">
                <textarea
                  value={importJSON}
                  onChange={e => setImportJSON(e.target.value)}
                  placeholder="Paste backup JSON string here..."
                  className="w-full h-36 p-3.5 rounded-2xl bg-neutral-950/80 border border-neutral-800 text-neutral-300 font-mono text-xs focus:border-blue-500 focus:outline-none resize-none ltr"
                />
              </div>

              <button
                onClick={handleImport}
                disabled={isLoading || !importJSON.trim()}
                className="w-full py-2.5 rounded-xl bg-blue-600 hover:bg-blue-500 text-white font-semibold text-xs transition-all flex items-center justify-center gap-2 disabled:opacity-50"
              >
                {isLoading ? <RefreshCw className="w-4 h-4 animate-spin" /> : <Upload className="w-4 h-4" />}
                {t('backup_import_btn')}
              </button>
            </div>
          )}
        </div>

        {/* Footer */}
        <div className="p-4 border-t border-neutral-800/80 bg-neutral-950/60 flex items-center justify-end">
          <button
            onClick={onClose}
            className="px-5 py-2 rounded-xl bg-neutral-800 hover:bg-neutral-700 text-white font-medium text-xs transition-all"
          >
            {t('close')}
          </button>
        </div>
      </div>
    </div>
  )
}
