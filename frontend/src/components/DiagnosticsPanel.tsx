import { useCallback, useEffect, useState } from 'react'
import {
  AlertTriangle,
  CheckCircle2,
  Download,
  ExternalLink,
  Gauge,
  History,
  RefreshCw,
  ShieldAlert,
  ShieldCheck,
  Undo2,
} from 'lucide-react'

import { Button } from '@/components/ui/button'
import { Card } from '@/components/ui/card'
import { Switch } from '@/components/ui/switch'
import { api } from '../lib/api'
import { t } from '../lib/i18n'
import type {
  ConnectionState,
  DiaryEntry,
  IPv6LeakResult,
  Settings,
  SubscriptionChange,
  UpdateInfo,
} from '../types'

/**
 * DiagnosticsPanel — the "something is wrong, help me find out what" surface.
 *
 * Everything here reads state the app already records (the launch diary, the
 * engine log, the update metadata, the subscription deltas) and presents it.
 * None of it changes the connection, with two deliberate exceptions: the
 * configuration rollback and the export, both of which are explicit button
 * presses with their own confirmation or file dialog.
 */
interface Props {
  settings: Settings
  update: <K extends keyof Settings>(key: K, value: Settings[K]) => void
  /** Live connection state, used to explain why a leak test is inconclusive. */
  connectionState?: ConnectionState
}

/** One step of the last connect attempt, derived from the launch diary. */
interface ConnectPhase {
  step: string
  tookMs: number
}

/** Convert a Go duration string ("4.123s", "120ms") into milliseconds. */
function parseGoDuration(value: string | undefined): number {
  if (!value || value === '-') return 0
  const match = /^(-?[\d.]+)(ns|µs|us|ms|s|m|h)$/.exec(value.trim())
  if (!match) return 0
  const amount = Number.parseFloat(match[1])
  if (!Number.isFinite(amount)) return 0
  switch (match[2]) {
    case 'ns':
      return amount / 1e6
    case 'µs':
    case 'us':
      return amount / 1e3
    case 'ms':
      return amount
    case 's':
      return amount * 1000
    case 'm':
      return amount * 60000
    default:
      return amount * 3600000
  }
}

/**
 * Extract the phases of the most recent connect attempt.
 *
 * The diary is a flat append-only log, so "the last attempt" means everything
 * after the final `connect/begin`. Reading it that way rather than filtering by
 * timestamp keeps it correct across a midnight boundary and across a rotated
 * file, both of which a time window would get wrong.
 */
function lastConnectPhases(entries: DiaryEntry[]): { phases: ConnectPhase[]; totalMs: number } {
  let start = -1
  for (let i = entries.length - 1; i >= 0; i--) {
    if (entries[i].category === 'connect' && entries[i].event === 'begin') {
      start = i
      break
    }
  }
  if (start < 0) return { phases: [], totalMs: 0 }

  const phases: ConnectPhase[] = []
  let totalMs = 0
  for (let i = start; i < entries.length; i++) {
    const entry = entries[i]
    if (entry.category !== 'connect') continue
    if (entry.event === 'phase') {
      const tookMs = parseGoDuration(entry.fields.took)
      phases.push({ step: entry.fields.step || '?', tookMs })
      totalMs += tookMs
    }
    if (entry.event === 'connected' || entry.event === 'abort-race') break
  }
  return { phases, totalMs }
}

/** Human-readable duration for the timeline. */
function formatMs(ms: number): string {
  if (ms <= 0) return '—'
  if (ms < 1000) return `${Math.round(ms)} ms`
  return `${(ms / 1000).toFixed(ms < 10000 ? 2 : 1)} s`
}

/**
 * Probe for a WebRTC leak.
 *
 * A page that can create a peer connection can learn the machine's local
 * addresses from the ICE candidates it gathers, unless the browser substitutes
 * mDNS hostnames. The check therefore looks for a *raw* host candidate: an
 * `.local` name is the browser protecting the address, and a public candidate
 * is the STUN-reflexive address, which is the VPN's own address when the tunnel
 * is up and is not a leak.
 */
async function probeWebRTC(): Promise<{ supported: boolean; exposed: string[] }> {
  const RTCPeer = (window as unknown as { RTCPeerConnection?: typeof RTCPeerConnection })
    .RTCPeerConnection
  if (!RTCPeer) return { supported: false, exposed: [] }

  const pc = new RTCPeer()
  const exposed: string[] = []
  try {
    pc.createDataChannel('nexvpn-leak-probe')
    const offer = await pc.createOffer()
    await pc.setLocalDescription(offer)

    await new Promise<void>(resolve => {
      // ICE gathering is normally done in well under a second; the timeout only
      // exists so a stalled gathering cannot leave the button spinning.
      const timer = window.setTimeout(resolve, 3000)
      pc.onicecandidate = event => {
        if (!event.candidate) {
          window.clearTimeout(timer)
          resolve()
        }
      }
    })

    const sdp = pc.localDescription?.sdp ?? ''
    for (const line of sdp.split('\n')) {
      const match = /a=candidate:\S+ \d+ \S+ \d+ (\S+) \d+ typ host/.exec(line.trim())
      if (!match) continue
      const address = match[1]
      // An mDNS name means the browser deliberately hid the address.
      if (address.endsWith('.local')) continue
      if (!exposed.includes(address)) exposed.push(address)
    }
  } catch {
    return { supported: false, exposed: [] }
  } finally {
    pc.close()
  }

  return { supported: true, exposed }
}

export default function DiagnosticsPanel({ settings, update, connectionState }: Props) {
  const [phases, setPhases] = useState<ConnectPhase[]>([])
  const [totalMs, setTotalMs] = useState(0)
  const [timelineLoaded, setTimelineLoaded] = useState(false)

  const [ipv6, setIpv6] = useState<IPv6LeakResult | null>(null)
  const [ipv6Busy, setIpv6Busy] = useState(false)
  const [webrtc, setWebrtc] = useState<{ supported: boolean; exposed: string[] } | null>(null)
  const [webrtcBusy, setWebrtcBusy] = useState(false)

  const [updateInfo, setUpdateInfo] = useState<UpdateInfo | null>(null)
  const [updateBusy, setUpdateBusy] = useState(false)

  const [changes, setChanges] = useState<SubscriptionChange[]>([])
  const [canRollback, setCanRollback] = useState(false)
  const [busy, setBusy] = useState('')
  const [notice, setNotice] = useState('')

  const loadTimeline = useCallback(async () => {
    try {
      const entries = await api.getDiaryEntries(1200)
      const { phases: found, totalMs: total } = lastConnectPhases(entries)
      setPhases(found)
      setTotalMs(total)
    } catch (err) {
      console.error('Diary read error:', err)
    } finally {
      setTimelineLoaded(true)
    }
  }, [])

  useEffect(() => {
    loadTimeline()
    api
      .getSubscriptionChanges()
      .then(setChanges)
      .catch(() => setChanges([]))
    api
      .hasPreviousConfigSnapshot()
      .then(setCanRollback)
      .catch(() => setCanRollback(false))
  }, [loadTimeline])

  const runIPv6 = async () => {
    setIpv6Busy(true)
    try {
      setIpv6(await api.testIPv6Leak())
    } catch (err) {
      console.error('IPv6 probe error:', err)
    } finally {
      setIpv6Busy(false)
    }
  }

  const runWebRTC = async () => {
    setWebrtcBusy(true)
    try {
      setWebrtc(await probeWebRTC())
    } finally {
      setWebrtcBusy(false)
    }
  }

  const runUpdateCheck = async () => {
    setUpdateBusy(true)
    try {
      setUpdateInfo(await api.checkAppUpdate())
    } catch (err) {
      console.error('Update check error:', err)
    } finally {
      setUpdateBusy(false)
    }
  }

  const handleExport = async () => {
    setBusy('export')
    try {
      const path = await api.exportDiagnosticsBundle()
      if (path) setNotice(`${t('adv_export_ok')}: ${path}`)
    } catch (err) {
      console.error('Diagnostics export error:', err)
      setNotice(err instanceof Error ? err.message : String(err))
    } finally {
      setBusy('')
    }
  }

  const handleRollback = async () => {
    if (!window.confirm(t('adv_rollback_confirm'))) return
    setBusy('rollback')
    try {
      await api.restorePreviousConfig()
      setNotice(t('adv_rollback_ok'))
      setCanRollback(false)
    } catch (err) {
      console.error('Rollback error:', err)
      setNotice(err instanceof Error ? err.message : String(err))
    } finally {
      setBusy('')
    }
  }

  const dismissChanges = async () => {
    await api.clearSubscriptionChanges()
    setChanges([])
  }

  const connected = connectionState === 'connected'
  const maxPhase = Math.max(1, ...phases.map(p => p.tookMs))

  return (
    <div className="space-y-4">
      {notice && (
        <Card className="flex items-start gap-2 border-primary/40 bg-primary/5 p-3">
          <CheckCircle2 className="mt-0.5 h-4 w-4 shrink-0 text-primary" />
          <span className="min-w-0 flex-1 break-words text-xs text-foreground">{notice}</span>
          <button
            type="button"
            className="shrink-0 text-xs text-muted-foreground hover:text-foreground"
            onClick={() => setNotice('')}
          >
            ×
          </button>
        </Card>
      )}

      {/* ---------------- Leak tests ---------------- */}
      <div className="section">
        <div className="section-title flex items-center gap-1.5">
          <ShieldAlert size={14} className="text-primary" />
          <span>{t('adv_leak')}</span>
        </div>
        <Card className="space-y-3 p-4">
          <p className="text-xs text-muted-foreground">{t('adv_leak_desc')}</p>
          {!connected && (
            <p className="flex items-start gap-1.5 text-[11px] text-warning">
              <AlertTriangle className="mt-0.5 h-3.5 w-3.5 shrink-0" />
              <span>{t('adv_leak_connect_first')}</span>
            </p>
          )}

          {/* IPv6 */}
          <div className="flex flex-wrap items-center gap-2 rounded-xl border border-border bg-muted/40 p-3">
            <span className="min-w-0 flex-1 text-xs font-medium text-foreground">
              {t('adv_leak_ipv6')}
            </span>
            {ipv6 && (
              <span
                className={`flex shrink-0 items-center gap-1 text-[11px] font-bold ${
                  ipv6.leaking ? 'text-destructive' : 'text-success'
                }`}
              >
                {ipv6.leaking ? <ShieldAlert size={12} /> : <ShieldCheck size={12} />}
                {ipv6.leaking ? t('adv_leak_ipv6_bad') : t('adv_leak_ipv6_ok')}
              </span>
            )}
            <Button variant="secondary" size="sm" onClick={runIPv6} disabled={ipv6Busy}>
              <RefreshCw className={`h-3.5 w-3.5 ${ipv6Busy ? 'animate-spin' : ''}`} />
              <span>{t('adv_leak_ipv6_run')}</span>
            </Button>
          </div>
          {ipv6 && <p className="text-[11px] text-muted-foreground">{ipv6.detail}</p>}

          {/* WebRTC */}
          <div className="flex flex-wrap items-center gap-2 rounded-xl border border-border bg-muted/40 p-3">
            <span className="min-w-0 flex-1 text-xs font-medium text-foreground">
              {t('adv_leak_webrtc')}
            </span>
            {webrtc && webrtc.supported && (
              <span
                className={`flex shrink-0 items-center gap-1 text-[11px] font-bold ${
                  webrtc.exposed.length > 0 ? 'text-destructive' : 'text-success'
                }`}
              >
                {webrtc.exposed.length > 0 ? <ShieldAlert size={12} /> : <ShieldCheck size={12} />}
                {webrtc.exposed.length > 0 ? t('adv_leak_webrtc_bad') : t('adv_leak_webrtc_ok')}
              </span>
            )}
            <Button variant="secondary" size="sm" onClick={runWebRTC} disabled={webrtcBusy}>
              <RefreshCw className={`h-3.5 w-3.5 ${webrtcBusy ? 'animate-spin' : ''}`} />
              <span>{t('adv_leak_webrtc_run')}</span>
            </Button>
          </div>
          {webrtc && !webrtc.supported && (
            <p className="text-[11px] text-muted-foreground">{t('adv_leak_webrtc_unsupported')}</p>
          )}
          {webrtc && webrtc.supported && webrtc.exposed.length > 0 && (
            <p className="font-mono text-[11px] text-destructive" dir="ltr">
              {webrtc.exposed.join(', ')}
            </p>
          )}
        </Card>
      </div>

      {/* ---------------- Connect timeline ---------------- */}
      <div className="section">
        <div className="section-title flex items-center justify-between gap-2">
          <div className="flex items-center gap-1.5">
            <Gauge size={14} className="text-primary" />
            <span>{t('adv_timeline')}</span>
          </div>
          <Button variant="ghost" size="sm" onClick={loadTimeline}>
            <RefreshCw className="h-3.5 w-3.5" />
          </Button>
        </div>
        <Card className="space-y-3 p-4">
          <p className="text-xs text-muted-foreground">{t('adv_timeline_desc')}</p>

          {timelineLoaded && phases.length === 0 ? (
            <p className="py-2 text-xs text-muted-foreground">{t('adv_timeline_empty')}</p>
          ) : (
            <>
              {phases.length > 0 && (
                <div className="flex items-center justify-between text-xs">
                  <span className="font-bold text-foreground">
                    {formatMs(totalMs)}
                  </span>
                  <span className="text-muted-foreground">{t('adv_timeline_took')}</span>
                </div>
              )}
              <div className="space-y-1.5">
                {phases.map((phase, index) => (
                  <div key={`${phase.step}-${index}`} className="space-y-1">
                    <div className="flex items-center justify-between gap-2 text-[11px]">
                      <span className="min-w-0 truncate font-mono text-muted-foreground" dir="ltr">
                        {phase.step}
                      </span>
                      <span className="shrink-0 font-mono text-foreground">{formatMs(phase.tookMs)}</span>
                    </div>
                    <div className="h-1.5 w-full overflow-hidden rounded-full bg-muted">
                      <div
                        className="h-full rounded-full bg-primary/70"
                        style={{ width: `${Math.max(2, (phase.tookMs / maxPhase) * 100)}%` }}
                      />
                    </div>
                  </div>
                ))}
              </div>
            </>
          )}
        </Card>
      </div>

      {/* ---------------- Subscription changes ---------------- */}
      <div className="section">
        <div className="section-title flex items-center justify-between gap-2">
          <div className="flex items-center gap-1.5">
            <History size={14} className="text-primary" />
            <span>{t('adv_sub_changes')}</span>
          </div>
          {changes.length > 0 && (
            <Button variant="ghost" size="sm" onClick={dismissChanges}>
              {t('adv_sub_clear')}
            </Button>
          )}
        </div>
        <Card className="space-y-3 p-4">
          <p className="text-xs text-muted-foreground">{t('adv_sub_changes_desc')}</p>
          {changes.length === 0 ? (
            <p className="py-2 text-xs text-muted-foreground">{t('adv_sub_empty')}</p>
          ) : (
            <div className="space-y-2">
              {changes.map(change => (
                <div
                  key={`${change.url}-${change.at}`}
                  className="space-y-1.5 rounded-xl border border-border bg-muted/40 p-3"
                >
                  <div className="flex flex-wrap items-center gap-2 text-[11px]">
                    <span className="min-w-0 flex-1 truncate font-medium text-foreground">
                      {change.group || change.url}
                    </span>
                    {change.added > 0 && (
                      <span className="shrink-0 rounded-md bg-success/15 px-1.5 py-0.5 font-bold text-success">
                        +{change.added} {t('adv_sub_added')}
                      </span>
                    )}
                    {change.removed > 0 && (
                      <span className="shrink-0 rounded-md bg-destructive/15 px-1.5 py-0.5 font-bold text-destructive">
                        −{change.removed} {t('adv_sub_removed')}
                      </span>
                    )}
                    {change.renamed > 0 && (
                      <span className="shrink-0 rounded-md bg-muted px-1.5 py-0.5 font-bold text-muted-foreground">
                        {change.renamed} {t('adv_sub_renamed')}
                      </span>
                    )}
                  </div>
                  {change.removed_names.length > 0 && (
                    <p className="break-words text-[10px] text-destructive/90" dir="ltr">
                      {change.removed_names.join(', ')}
                    </p>
                  )}
                </div>
              ))}
            </div>
          )}
        </Card>
      </div>

      {/* ---------------- Updates ---------------- */}
      <div className="section">
        <div className="section-title flex items-center gap-1.5">
          <Download size={14} className="text-primary" />
          <span>{t('adv_update')}</span>
        </div>
        <Card className="space-y-3 p-4">
          <div className="flex flex-wrap items-center gap-2 rounded-xl border border-border bg-muted/40 p-3">
            <span className="min-w-0 flex-1 text-xs font-medium text-foreground">
              {t('adv_update_enabled')}
            </span>
            <Switch
              checked={settings.app_update_check_enabled ?? true}
              onCheckedChange={checked => update('app_update_check_enabled', checked)}
              aria-label={t('adv_update_enabled')}
            />
          </div>
          <p className="text-[11px] text-muted-foreground">{t('adv_update_enabled_desc')}</p>

          <div className="flex flex-wrap items-center gap-2">
            <Button variant="secondary" size="sm" onClick={runUpdateCheck} disabled={updateBusy}>
              <RefreshCw className={`h-3.5 w-3.5 ${updateBusy ? 'animate-spin' : ''}`} />
              <span>{updateBusy ? t('adv_update_checking') : t('adv_update_check')}</span>
            </Button>
            {updateInfo?.release_url && updateInfo.update_available && (
              <Button
                size="sm"
                onClick={() => window.open(updateInfo.release_url, '_blank', 'noopener,noreferrer')}
              >
                <ExternalLink className="h-3.5 w-3.5" />
                <span>{t('adv_update_open')}</span>
              </Button>
            )}
          </div>

          {updateInfo && (
            <p
              className={`flex items-center gap-1.5 text-xs ${
                updateInfo.update_available ? 'font-bold text-primary' : 'text-muted-foreground'
              }`}
            >
              {updateInfo.update_available ? (
                <Download size={12} />
              ) : (
                <CheckCircle2 size={12} className="text-success" />
              )}
              {updateInfo.update_available
                ? t('adv_update_available').replace('{version}', updateInfo.latest_version)
                : updateInfo.detail || t('adv_update_current')}
            </p>
          )}
        </Card>
      </div>

      {/* ---------------- Maintenance ---------------- */}
      <div className="section">
        <div className="section-title flex items-center gap-1.5">
          <Undo2 size={14} className="text-primary" />
          <span>{t('adv_diag')}</span>
        </div>
        <Card className="space-y-3 p-4">
          <p className="text-xs text-muted-foreground">{t('adv_diag_desc')}</p>

          <div className="space-y-2">
            <div className="rounded-xl border border-border bg-muted/40 p-3">
              <p className="mb-2 text-xs font-medium text-foreground">{t('adv_export_bundle')}</p>
              <p className="mb-2 text-[11px] text-muted-foreground">{t('adv_export_bundle_desc')}</p>
              <Button
                variant="secondary"
                size="sm"
                onClick={handleExport}
                disabled={busy === 'export'}
              >
                <Download className={`h-3.5 w-3.5 ${busy === 'export' ? 'animate-spin' : ''}`} />
                <span>{t('adv_export_bundle')}</span>
              </Button>
            </div>

            <div className="rounded-xl border border-border bg-muted/40 p-3">
              <p className="mb-2 text-xs font-medium text-foreground">{t('adv_rollback')}</p>
              <p className="mb-2 text-[11px] text-muted-foreground">
                {canRollback ? t('adv_rollback_desc') : t('adv_rollback_none')}
              </p>
              <Button
                variant="secondary"
                size="sm"
                onClick={handleRollback}
                disabled={!canRollback || busy === 'rollback'}
              >
                <Undo2 className="h-3.5 w-3.5" />
                <span>{t('adv_rollback')}</span>
              </Button>
            </div>
          </div>
        </Card>
      </div>
    </div>
  )
}
