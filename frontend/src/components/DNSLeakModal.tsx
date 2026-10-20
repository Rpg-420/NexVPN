import { useState, useEffect } from 'react'
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
import { ModalShell } from '../components/ui/modal-shell'

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
    <ModalShell
      isOpen={isOpen}
      onClose={onClose}
      maxWidth="max-w-2xl"
      className="max-h-[90vh]"
    >
        {/* Header */}
        <div className="flex items-center justify-between p-6 border-b border-border bg-background">
          <div className="flex items-center gap-3">
            <div className="w-10 h-10 rounded-2xl border border-border bg-muted flex items-center justify-center text-muted-foreground">
              <Shield className="w-5 h-5" />
            </div>
            <div>
              <h3 className="text-lg font-bold text-foreground flex items-center gap-2">
                {t('dns_modal_title')}
                <span className="px-2 py-0.5 text-xs font-semibold rounded-full bg-primary/20 text-primary border border-primary/30">
                  {t('dns_badge')}
                </span>
              </h3>
              <p className="text-xs text-muted-foreground">{t('dns_modal_sub')}</p>
            </div>
          </div>
          <div className="flex items-center gap-2">
            <button
              onClick={runTest}
              disabled={isLoading}
              className="p-2 rounded-xl bg-muted hover:bg-muted/90 text-muted-foreground transition-all disabled:opacity-50"
              title="Re-test"
            >
              <RefreshCw className={`w-4 h-4 ${isLoading ? 'animate-spin text-primary' : ''}`} />
            </button>
            <button
              onClick={onClose}
              className="p-2 rounded-xl bg-muted hover:bg-muted/90 text-muted-foreground hover:text-foreground transition-all"
            >
              <X className="w-4 h-4" />
            </button>
          </div>
        </div>

        {/* Body */}
        <div className="p-6 overflow-y-auto space-y-6">
          {isLoading && (
            <div className="py-12 flex flex-col items-center justify-center text-center space-y-3">
              <RefreshCw className="w-8 h-8 text-primary animate-spin" />
              <p className="text-sm text-muted-foreground font-medium">{t('dns_probing')}</p>
              <p className="text-xs text-muted-foreground">{t('dns_probing_sub')}</p>
            </div>
          )}

          {error && (
            <div className="p-4 rounded-2xl bg-destructive/10 border border-destructive/20 text-destructive text-sm flex items-center gap-3">
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
                    ? 'bg-destructive/10 border-destructive/30 text-destructive'
                    : 'bg-success/10 border-success/30 text-success'
                }`}
              >
                {report.is_leaking ? (
                  <ShieldAlert className="w-8 h-8 text-destructive flex-shrink-0" />
                ) : (
                  <ShieldCheck className="w-8 h-8 text-success flex-shrink-0" />
                )}
                <div className="space-y-1">
                  <h4 className="text-base font-bold text-foreground">
                    {report.is_leaking ? t('dns_leak_title') : t('dns_secure_title')}
                  </h4>
                  <p className="text-xs text-muted-foreground leading-relaxed">{report.summary}</p>
                </div>
              </div>

              {/* IP & Resolver Grid */}
              <div className="grid grid-cols-1 md:grid-cols-2 gap-4">
                <div className="p-4 rounded-2xl bg-background border border-border space-y-2">
                  <div className="flex items-center gap-2 text-xs font-semibold text-muted-foreground uppercase tracking-wider">
                    <Globe className="w-4 h-4 text-primary" />
                    {t('dns_tunnel_ip')}
                  </div>
                  <div className="text-base font-bold text-foreground">{report.outbound_ip || '127.0.0.1'}</div>
                  <div className="text-xs text-muted-foreground">
                    {report.outbound_country || 'Protected Location'} • {report.outbound_isp || 'Encrypted Tunnel'}
                  </div>
                </div>

                <div className="p-4 rounded-2xl bg-background border border-border space-y-2">
                  <div className="flex items-center gap-2 text-xs font-semibold text-muted-foreground uppercase tracking-wider">
                    <Server className="w-4 h-4 text-success" />
                    {t('dns_active_resolver')}
                  </div>
                  <div className="text-base font-bold text-foreground">{report.dns_resolver_ip || 'Encrypted DoH Resolver'}</div>
                  <div className="text-xs text-muted-foreground">
                    {report.dns_resolver_country || 'Tunnel Internal DNS'} ({t('dns_no_interception')})
                  </div>
                </div>
              </div>

              {/* Probes Table */}
              {report.probes && report.probes.length > 0 && (
                <div className="space-y-3">
                  <h5 className="text-xs font-semibold text-muted-foreground uppercase tracking-wider">
                    {t('dns_probes_title')}
                  </h5>
                  <div className="divide-y divide-border/60 rounded-2xl bg-background border border-border overflow-hidden">
                    {report.probes.map((probe, idx) => (
                      <div key={idx} className="flex items-center justify-between p-3.5 text-xs">
                        <div className="flex items-center gap-2.5">
                          {probe.status === 'ok' ? (
                            <CheckCircle2 className="w-4 h-4 text-success" />
                          ) : (
                            <AlertTriangle className="w-4 h-4 text-warning" />
                          )}
                          <span className="font-medium text-foreground">{probe.name}</span>
                          <span className="text-muted-foreground font-mono">({probe.target})</span>
                        </div>
                        <div className="flex items-center gap-3">
                          <span className="font-mono text-muted-foreground">{probe.latency_ms}ms</span>
                          <span
                            className={`px-2 py-0.5 rounded-md font-semibold text-[10px] uppercase ${
                              probe.status === 'ok'
                                ? 'bg-success/10 text-success border border-success/20'
                                : 'bg-warning/10 text-warning border border-warning/20'
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
        <div className="p-4 border-t border-border bg-background flex items-center justify-between">
          <div className="flex items-center gap-2 text-xs text-muted-foreground">
            <Lock className="w-3.5 h-3.5 text-primary" />
            <span>{t('dns_doh_enabled')}</span>
          </div>
          <button
            onClick={onClose}
            className="px-5 py-2 rounded-xl bg-muted hover:bg-muted/90 text-foreground font-medium text-xs transition-all"
          >
            {t('close')}
          </button>
        </div>
      </ModalShell>
  )
}
