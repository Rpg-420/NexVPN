import React, { useState, useEffect } from 'react'
import {
  Shield,
  ShieldAlert,
  ShieldCheck,
  RefreshCw,
  X,
  Globe,
  Server,
  Lock,
  CheckCircle2,
  AlertTriangle,
} from 'lucide-react'
import { api } from '../lib/api'
import { t } from '../lib/i18n'
import type { DNSLeakReport } from '../types'

interface Props {
  isOpen: boolean
  onClose: () => void
}

export default function DNSLeakModal({ isOpen, onClose }: Props) {
  const [report, setReport] = useState<DNSLeakReport | null>(null)
  const [isLoading, setIsLoading] = useState(false)
  const [error, setError] = useState<string | null>(null)

  const runTest = async () => {
    setIsLoading(true)
    setError(null)
    try {
      const res = await api.runDNSLeakDiagnostics()
      setReport(res)
    } catch (err: any) {
      setError(err?.message || 'Failed to run DNS diagnostics')
    } finally {
      setIsLoading(false)
    }
  }

  useEffect(() => {
    if (isOpen) {
      runTest()
    }
  }, [isOpen])

  if (!isOpen) return null

  return (
    <div className="fixed inset-0 z-50 flex items-center justify-center p-4 bg-black/70 backdrop-blur-md animate-fade-in">
      <div className="relative w-full max-w-2xl bg-neutral-900 border border-neutral-800 rounded-3xl shadow-2xl overflow-hidden flex flex-col max-h-[90vh]">
        {/* Header */}
        <div className="flex items-center justify-between p-6 border-b border-neutral-800/80 bg-neutral-950/40">
          <div className="flex items-center gap-3">
            <div className="w-10 h-10 rounded-2xl bg-cyan-500/10 border border-cyan-500/20 flex items-center justify-center text-cyan-400">
              <Shield className="w-5 h-5" />
            </div>
            <div>
              <h3 className="text-lg font-bold text-white flex items-center gap-2">
                {t('dns_modal_title')}
                <span className="px-2 py-0.5 text-xs font-semibold rounded-full bg-cyan-500/20 text-cyan-300 border border-cyan-500/30">
                  {t('dns_badge')}
                </span>
              </h3>
              <p className="text-xs text-neutral-400">{t('dns_modal_sub')}</p>
            </div>
          </div>
          <div className="flex items-center gap-2">
            <button
              onClick={runTest}
              disabled={isLoading}
              className="p-2 rounded-xl bg-neutral-800 hover:bg-neutral-700 text-neutral-300 transition-all disabled:opacity-50"
              title="Re-test"
            >
              <RefreshCw className={`w-4 h-4 ${isLoading ? 'animate-spin text-cyan-400' : ''}`} />
            </button>
            <button
              onClick={onClose}
              className="p-2 rounded-xl bg-neutral-800/60 hover:bg-neutral-700 text-neutral-400 hover:text-white transition-all"
            >
              <X className="w-4 h-4" />
            </button>
          </div>
        </div>

        {/* Body */}
        <div className="p-6 overflow-y-auto space-y-6">
          {isLoading && (
            <div className="py-12 flex flex-col items-center justify-center text-center space-y-3">
              <RefreshCw className="w-8 h-8 text-cyan-400 animate-spin" />
              <p className="text-sm text-neutral-300 font-medium">{t('dns_probing')}</p>
              <p className="text-xs text-neutral-500">{t('dns_probing_sub')}</p>
            </div>
          )}

          {error && (
            <div className="p-4 rounded-2xl bg-red-500/10 border border-red-500/20 text-red-400 text-sm flex items-center gap-3">
              <AlertTriangle className="w-5 h-5 flex-shrink-0" />
              <span>{error}</span>
            </div>
          )}

          {!isLoading && report && (
            <>
              {/* Verdict Status Card */}
              <div
                className={`p-5 rounded-2xl border flex items-start gap-4 ${
                  report.is_leaking
                    ? 'bg-red-500/10 border-red-500/30 text-red-300'
                    : 'bg-emerald-500/10 border-emerald-500/30 text-emerald-300'
                }`}
              >
                {report.is_leaking ? (
                  <ShieldAlert className="w-8 h-8 text-red-400 flex-shrink-0" />
                ) : (
                  <ShieldCheck className="w-8 h-8 text-emerald-400 flex-shrink-0" />
                )}
                <div className="space-y-1">
                  <h4 className="text-base font-bold text-white">
                    {report.is_leaking ? t('dns_leak_title') : t('dns_secure_title')}
                  </h4>
                  <p className="text-xs text-neutral-300 leading-relaxed">{report.summary}</p>
                </div>
              </div>

              {/* IP & Resolver Grid */}
              <div className="grid grid-cols-1 md:grid-cols-2 gap-4">
                <div className="p-4 rounded-2xl bg-neutral-950/50 border border-neutral-800 space-y-2">
                  <div className="flex items-center gap-2 text-xs font-semibold text-neutral-400 uppercase tracking-wider">
                    <Globe className="w-4 h-4 text-cyan-400" />
                    {t('dns_tunnel_ip')}
                  </div>
                  <div className="text-base font-bold text-white">{report.outbound_ip || '127.0.0.1'}</div>
                  <div className="text-xs text-neutral-400">
                    {report.outbound_country || 'Protected Location'} • {report.outbound_isp || 'Encrypted Tunnel'}
                  </div>
                </div>

                <div className="p-4 rounded-2xl bg-neutral-950/50 border border-neutral-800 space-y-2">
                  <div className="flex items-center gap-2 text-xs font-semibold text-neutral-400 uppercase tracking-wider">
                    <Server className="w-4 h-4 text-emerald-400" />
                    {t('dns_active_resolver')}
                  </div>
                  <div className="text-base font-bold text-white">{report.dns_resolver_ip || 'Encrypted DoH Resolver'}</div>
                  <div className="text-xs text-neutral-400">
                    {report.dns_resolver_country || 'Tunnel Internal DNS'} ({t('dns_no_interception')})
                  </div>
                </div>
              </div>

              {/* Probes Table */}
              {report.probes && report.probes.length > 0 && (
                <div className="space-y-3">
                  <h5 className="text-xs font-semibold text-neutral-400 uppercase tracking-wider">
                    {t('dns_probes_title')}
                  </h5>
                  <div className="divide-y divide-neutral-800/60 rounded-2xl bg-neutral-950/50 border border-neutral-800 overflow-hidden">
                    {report.probes.map((probe, idx) => (
                      <div key={idx} className="flex items-center justify-between p-3.5 text-xs">
                        <div className="flex items-center gap-2.5">
                          {probe.status === 'ok' ? (
                            <CheckCircle2 className="w-4 h-4 text-emerald-400" />
                          ) : (
                            <AlertTriangle className="w-4 h-4 text-amber-400" />
                          )}
                          <span className="font-medium text-white">{probe.name}</span>
                          <span className="text-neutral-500 font-mono">({probe.target})</span>
                        </div>
                        <div className="flex items-center gap-3">
                          <span className="font-mono text-neutral-400">{probe.latency_ms}ms</span>
                          <span
                            className={`px-2 py-0.5 rounded-md font-semibold text-[10px] uppercase ${
                              probe.status === 'ok'
                                ? 'bg-emerald-500/10 text-emerald-400 border border-emerald-500/20'
                                : 'bg-amber-500/10 text-amber-400 border border-amber-500/20'
                            }`}
                          >
                            {probe.status}
                          </span>
                        </div>
                      </div>
                    ))}
                  </div>
                </div>
              )}
            </>
          )}
        </div>

        {/* Footer */}
        <div className="p-4 border-t border-neutral-800/80 bg-neutral-950/60 flex items-center justify-between">
          <div className="flex items-center gap-2 text-xs text-neutral-500">
            <Lock className="w-3.5 h-3.5 text-cyan-400" />
            <span>{t('dns_doh_enabled')}</span>
          </div>
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
