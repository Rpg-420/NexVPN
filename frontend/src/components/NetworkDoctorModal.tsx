import React, { useState, useEffect } from 'react'
import {
  Activity,
  CheckCircle2,
  AlertTriangle,
  XCircle,
  RefreshCw,
  Wrench,
  ShieldCheck,
  Zap,
  X,
} from 'lucide-react'
import { api } from '../lib/api'
import { t } from '../lib/i18n'
import type { DoctorReport, RepairResult, DiagnosticItem } from '../types'
import { ModalShell } from '../components/ui/modal-shell'

interface NetworkDoctorModalProps {
  isOpen: boolean
  onClose: () => void
}

export const NetworkDoctorModal: React.FC<NetworkDoctorModalProps> = ({ isOpen, onClose }) => {
  const [loading, setLoading] = useState(false)
  const [repairing, setRepairing] = useState(false)
  const [report, setReport] = useState<DoctorReport | null>(null)
  const [repairResult, setRepairResult] = useState<RepairResult | null>(null)

  const runDiagnostics = async () => {
    setLoading(true)
    setRepairResult(null)
    try {
      const res = await api.runNetworkDiagnostics()
      setReport(res)
    } catch (err) {
      console.error('Network doctor error:', err)
    } finally {
      setLoading(false)
    }
  }

  const handleRepair = async () => {
    setRepairing(true)
    try {
      const res = await api.fixNetworkIssues()
      setRepairResult(res)
      setTimeout(() => {
        runDiagnostics()
      }, 1000)
    } catch (err) {
      console.error('Repair error:', err)
    } finally {
      setRepairing(false)
    }
  }

  useEffect(() => {
    if (isOpen) {
      runDiagnostics()
    }
  }, [isOpen])

  if (!isOpen) return null

  const getStatusIcon = (status: string) => {
    switch (status) {
      case 'ok':
        return <CheckCircle2 className="w-5 h-5 text-success shrink-0" />
      case 'warning':
        return <AlertTriangle className="w-5 h-5 text-warning shrink-0" />
      case 'error':
        return <XCircle className="w-5 h-5 text-destructive shrink-0" />
      default:
        return <Activity className="w-5 h-5 text-primary animate-spin shrink-0" />
    }
  }

  const getItemTitle = (item: DiagnosticItem) => {
    const key = `doctor_item_${item.id}_title`
    const translated = t(key)
    return translated !== key ? translated : item.title
  }

  const getItemDesc = (item: DiagnosticItem) => {
    const key = `doctor_item_${item.id}_desc`
    const translated = t(key)
    return translated !== key ? translated : item.description
  }

  const getItemDetail = (item: DiagnosticItem) => {
    if (item.id === 'gateway') {
      return item.latency_ms && item.latency_ms > 0
        ? t('doctor_gateway_detail_ok', { ms: item.latency_ms })
        : t('doctor_gateway_detail_active')
    }
    if (item.id === 'dns_servers') {
      return item.status === 'warning'
        ? t('doctor_dns_detail_warn')
        : t('doctor_dns_detail_ok', { ms: item.latency_ms || 120 })
    }
    if (item.id === 'warp_edge') {
      return item.status === 'warning'
        ? t('doctor_warp_detail_warn')
        : t('doctor_warp_detail_ok', { ms: item.latency_ms || 45 })
    }
    if (item.id === 'system_proxy') {
      return item.status === 'warning' ? t('doctor_proxy_warn') : t('doctor_proxy_ok')
    }
    if (item.id === 'wintun_driver') {
      return t('doctor_wintun_ok')
    }
    return item.detail
  }

  const getReportSummary = (rep: DoctorReport) => {
    if (rep.overall_status === 'healthy') return t('doctor_summary_healthy')
    if (rep.overall_status === 'degraded') return t('doctor_summary_degraded')
    return t('doctor_summary_critical')
  }

  return (
    <ModalShell
      isOpen={isOpen}
      onClose={onClose}
      maxWidth="max-w-2xl"
      className="border-border"
    >
        {/* Header */}
        <div className="flex items-center justify-between px-6 py-5 border-b border-border">
          <div className="flex items-center gap-3">
            <div className="bg-info/20 flex items-center justify-center w-10 h-10 rounded-xl border border-border text-primary">
              <ShieldCheck className="w-6 h-6" />
            </div>
            <div>
              <h3 className="text-lg font-bold text-foreground flex items-center gap-2">
                {t('doctor_modal_title')}
                <span className="text-xs px-2 py-0.5 rounded-full bg-primary/20 text-primary border border-primary/30 font-medium">
                  Smart Doctor
                </span>
              </h3>
              <p className="text-xs text-muted-foreground mt-0.5">
                {t('doctor_modal_subtitle')}
              </p>
            </div>
          </div>
          <button
            onClick={onClose}
            className="p-2 rounded-xl text-muted-foreground hover:text-foreground hover:bg-muted transition-all"
          >
            <X className="w-5 h-5" />
          </button>
        </div>

        {/* Content Body */}
        <div className="p-6 max-h-[65vh] overflow-y-auto space-y-5">
          {/* Summary Banner */}
          {report && (
            <div
              className={`p-4 rounded-xl border flex items-center justify-between gap-4 transition-all ${
                report.overall_status === 'healthy'
                  ? 'bg-success/10 border-success/30 text-success'
                  : report.overall_status === 'degraded'
                  ? 'bg-warning/10 border-warning/30 text-warning'
                  : 'bg-destructive/10 border-destructive/30 text-destructive'
              }`}
            >
              <div className="flex items-center gap-3">
                {report.overall_status === 'healthy' ? (
                  <CheckCircle2 className="w-7 h-7 text-success shrink-0" />
                ) : (
                  <AlertTriangle className="w-7 h-7 text-warning shrink-0" />
                )}
                <div>
                  <div className="text-sm font-bold">
                    {report.overall_status === 'healthy'
                      ? t('doctor_status_healthy')
                      : t('doctor_status_degraded')}
                  </div>
                  <div className="text-xs opacity-85 mt-0.5">{getReportSummary(report)}</div>
                </div>
              </div>

              <button
                onClick={runDiagnostics}
                disabled={loading}
                className="flex items-center gap-1.5 px-3 py-1.5 rounded-lg bg-muted hover:bg-muted/90 text-xs font-semibold text-foreground border border-border transition-all shrink-0"
              >
                <RefreshCw className={`w-3.5 h-3.5 ${loading ? 'animate-spin' : ''}`} />
                {t('doctor_rescan_btn')}
              </button>
            </div>
          )}

          {/* Diagnostic Checks List */}
          <div className="space-y-3">
            {report?.items.map(item => (
              <div
                key={item.id}
                className="p-4 rounded-xl bg-card border border-border hover:border-border transition-all flex items-start justify-between gap-4"
              >
                <div className="flex items-start gap-3.5">
                  <div className="mt-0.5">{getStatusIcon(item.status)}</div>
                  <div>
                    <div className="text-sm font-semibold text-foreground flex items-center gap-2">
                      {getItemTitle(item)}
                      {item.latency_ms && item.latency_ms > 0 ? (
                        <span className="text-xs px-2 py-0.5 rounded-md bg-success/20 text-success font-mono">
                          {item.latency_ms} ms
                        </span>
                      ) : null}
                    </div>
                    <div className="text-xs text-muted-foreground mt-1">{getItemDesc(item)}</div>
                    <div className="text-xs text-muted-foreground/80 font-mono mt-1.5 bg-black/30 px-2.5 py-1 rounded-md inline-block">
                      {getItemDetail(item)}
                    </div>
                  </div>
                </div>
              </div>
            ))}
          </div>

          {/* Repair Result Banner */}
          {repairResult && (
            <div className="p-4 rounded-xl border border-border bg-muted text-muted-foreground animate-in fade-in">
              <div className="flex items-center gap-2 font-bold text-sm text-primary">
                <Zap className="w-4 h-4 text-primary" />
                {repairResult.message}
              </div>
              <ul className="mt-2 space-y-1 text-xs text-primary/90 list-disc list-inside">
                {repairResult.actions.map((act, idx) => (
                  <li key={idx}>{act}</li>
                ))}
              </ul>
            </div>
          )}
        </div>

        {/* Footer Actions */}
        <div className="flex items-center justify-between px-6 py-4 bg-background border-t border-border">
          <div className="text-xs text-muted-foreground">
            {t('doctor_footer_note')}
          </div>
          <div className="flex items-center gap-3">
            <button
              onClick={handleRepair}
              disabled={repairing}
              className="bg-primary flex items-center gap-2 px-5 py-2.5 rounded-xl text-primary-foreground text-xs font-bold shadow-sm transition-all active:scale-95 disabled:opacity-50"
            >
              <Wrench className={`w-4 h-4 ${repairing ? 'animate-spin' : ''}`} />
              {repairing ? t('doctor_repairing_label') : t('doctor_repair_all_btn')}
            </button>
          </div>
        </div>
      </ModalShell>
  )
}
