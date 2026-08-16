import React, { useState, useEffect } from 'react'
import {
  History,
  Clock,
  ArrowDown,
  ArrowUp,
  Trash2,
  X,
  Calendar,
  CheckCircle2,
  AlertCircle,
  Download,
} from 'lucide-react'
import { api } from '../lib/api'
import { t } from '../lib/i18n'
import { formatBytes, formatDuration } from '../lib/utils'
import type { ConnectionSession } from '../types'

interface Props {
  isOpen: boolean
  onClose: () => void
}

export default function ConnectionHistoryModal({ isOpen, onClose }: Props) {
  const [sessions, setSessions] = useState<ConnectionSession[]>([])
  const [isLoading, setIsLoading] = useState(false)

  const loadHistory = async () => {
    setIsLoading(true)
    try {
      const data = await api.getConnectionHistory()
      if (data && Array.isArray(data)) {
        setSessions(data)
      }
    } catch (err) {
      console.warn('History load error:', err)
    } finally {
      setIsLoading(false)
    }
  }

  const handleClear = async () => {
    if (window.confirm(t('confirm_delete_all'))) {
      try {
        await api.clearConnectionHistory()
        setSessions([])
      } catch (err) {
        console.error('Clear history error:', err)
      }
    }
  }

  const exportCSV = () => {
    if (sessions.length === 0) return
    const headers = 'ID,Profile,Protocol,Server,ConnectedAt,DisconnectedAt,DurationSec,BytesRx,BytesTx,Reason\n'
    const rows = sessions
      .map(
        s =>
          `"${s.id}","${s.profile_name}","${s.protocol}","${s.server}","${s.connected_at}","${s.disconnected_at}",${s.duration_sec},${s.bytes_received},${s.bytes_sent},"${s.reason}"`
      )
      .join('\n')
    const blob = new Blob([headers + rows], { type: 'text/csv' })
    const url = URL.createObjectURL(blob)
    const a = document.createElement('a')
    a.href = url
    a.download = `nexvpn_history_${new Date().toISOString().slice(0, 10)}.csv`
    a.click()
    URL.revokeObjectURL(url)
  }

  useEffect(() => {
    if (isOpen) {
      loadHistory()
    }
  }, [isOpen])

  if (!isOpen) return null

  const totalBytesRx = sessions.reduce((acc, s) => acc + (s.bytes_received || 0), 0)
  const totalBytesTx = sessions.reduce((acc, s) => acc + (s.bytes_sent || 0), 0)
  const totalDuration = sessions.reduce((acc, s) => acc + (s.duration_sec || 0), 0)

  return (
    <div className="fixed inset-0 z-50 flex items-center justify-center p-4 bg-black/70 backdrop-blur-md animate-fade-in">
      <div className="relative w-full max-w-3xl bg-neutral-900 border border-neutral-800 rounded-3xl shadow-2xl overflow-hidden flex flex-col max-h-[90vh]">
        {/* Header */}
        <div className="flex items-center justify-between p-6 border-b border-neutral-800/80 bg-neutral-950/40">
          <div className="flex items-center gap-3">
            <div className="w-10 h-10 rounded-2xl bg-indigo-500/10 border border-indigo-500/20 flex items-center justify-center text-indigo-400">
              <History className="w-5 h-5" />
            </div>
            <div>
              <h3 className="text-lg font-bold text-white flex items-center gap-2">
                {t('history_modal_title')}
                <span className="px-2 py-0.5 text-xs font-semibold rounded-full bg-indigo-500/20 text-indigo-300 border border-indigo-500/30">
                  {sessions.length}
                </span>
              </h3>
              <p className="text-xs text-neutral-400">{t('history_modal_sub')}</p>
            </div>
          </div>
          <div className="flex items-center gap-2">
            {sessions.length > 0 && (
              <>
                <button
                  onClick={exportCSV}
                  className="p-2 rounded-xl bg-neutral-800 hover:bg-neutral-700 text-neutral-300 transition-all text-xs flex items-center gap-1.5 px-3"
                  title="Export CSV"
                >
                  <Download className="w-3.5 h-3.5" />
                  <span>CSV</span>
                </button>
                <button
                  onClick={handleClear}
                  className="p-2 rounded-xl bg-red-500/10 hover:bg-red-500/20 border border-red-500/20 text-red-400 transition-all text-xs flex items-center gap-1.5 px-3"
                  title={t('history_clear_btn')}
                >
                  <Trash2 className="w-3.5 h-3.5" />
                  <span>{t('history_clear_btn')}</span>
                </button>
              </>
            )}
            <button
              onClick={onClose}
              className="p-2 rounded-xl bg-neutral-800/60 hover:bg-neutral-700 text-neutral-400 hover:text-white transition-all"
            >
              <X className="w-4 h-4" />
            </button>
          </div>
        </div>

        {/* Stats Overview */}
        <div className="grid grid-cols-3 gap-3 p-6 pb-2">
          <div className="p-3.5 rounded-2xl bg-neutral-950/50 border border-neutral-800/80">
            <div className="text-[11px] font-semibold text-neutral-400 flex items-center gap-1.5 uppercase">
              <Clock className="w-3.5 h-3.5 text-indigo-400" />
              {t('history_col_duration')}
            </div>
            <div className="text-base font-bold text-white mt-1">{formatDuration(totalDuration)}</div>
          </div>
          <div className="p-3.5 rounded-2xl bg-neutral-950/50 border border-neutral-800/80">
            <div className="text-[11px] font-semibold text-neutral-400 flex items-center gap-1.5 uppercase">
              <ArrowDown className="w-3.5 h-3.5 text-cyan-400" />
              {t('downloaded')}
            </div>
            <div className="text-base font-bold text-white mt-1">{formatBytes(totalBytesRx)}</div>
          </div>
          <div className="p-3.5 rounded-2xl bg-neutral-950/50 border border-neutral-800/80">
            <div className="text-[11px] font-semibold text-neutral-400 flex items-center gap-1.5 uppercase">
              <ArrowUp className="w-3.5 h-3.5 text-purple-400" />
              {t('uploaded')}
            </div>
            <div className="text-base font-bold text-white mt-1">{formatBytes(totalBytesTx)}</div>
          </div>
        </div>

        {/* Session List */}
        <div className="p-6 pt-2 overflow-y-auto space-y-3 flex-1">
          {isLoading ? (
            <div className="py-12 text-center text-sm text-neutral-400">{t('querying')}</div>
          ) : sessions.length === 0 ? (
            <div className="py-16 text-center space-y-2">
              <History className="w-10 h-10 text-neutral-600 mx-auto" />
              <p className="text-sm font-medium text-neutral-300">{t('history_empty')}</p>
            </div>
          ) : (
            <div className="divide-y divide-neutral-800/60 rounded-2xl bg-neutral-950/50 border border-neutral-800 overflow-hidden">
              {sessions.map((sess, idx) => (
                <div key={idx} className="p-4 flex items-center justify-between text-xs hover:bg-neutral-800/30 transition-all">
                  <div className="space-y-1">
                    <div className="flex items-center gap-2">
                      <span className="font-bold text-white text-sm">{sess.profile_name}</span>
                      <span className="px-2 py-0.5 rounded-md bg-neutral-800 text-neutral-300 font-mono text-[10px] uppercase">
                        {sess.protocol}
                      </span>
                      {sess.reason === 'manual' ? (
                        <span className="text-[10px] text-neutral-400 flex items-center gap-1">
                          <CheckCircle2 className="w-3 h-3 text-emerald-400" /> {t('disconnected')}
                        </span>
                      ) : (
                        <span className="text-[10px] text-amber-400 flex items-center gap-1">
                          <AlertCircle className="w-3 h-3" /> {sess.reason}
                        </span>
                      )}
                    </div>
                    <div className="flex items-center gap-3 text-neutral-500 text-[11px]">
                      <span className="flex items-center gap-1">
                        <Calendar className="w-3 h-3" />
                        {new Date(sess.connected_at).toLocaleString()}
                      </span>
                      <span>•</span>
                      <span>{t('duration')}: {formatDuration(sess.duration_sec)}</span>
                    </div>
                  </div>

                  <div className="text-right space-y-1">
                    <div className="flex items-center gap-3 text-xs font-mono">
                      <span className="text-cyan-400 flex items-center gap-1">
                        <ArrowDown className="w-3 h-3" /> {formatBytes(sess.bytes_received)}
                      </span>
                      <span className="text-purple-400 flex items-center gap-1">
                        <ArrowUp className="w-3 h-3" /> {formatBytes(sess.bytes_sent)}
                      </span>
                    </div>
                  </div>
                </div>
              ))}
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
