import { useState, useEffect, useMemo, useCallback } from 'react'
import {
  X,
  Globe,
  RefreshCw,
  Activity,
  Download,
  Search,
  CheckSquare,
  Square,
  AlertTriangle,
  Server,
  Link2,
  Key,
  Check,
} from 'lucide-react'
import { api } from '../lib/api'
import { t } from '../lib/i18n'
import type { OpenVPNSourceInfo, OpenVPNServer, OpenVPNServerTestProgress } from '../types'
import CountryFlag from './CountryFlag'
import { ModalShell } from '../components/ui/modal-shell'
import { Button } from '@/components/ui/button'

interface Props {
  isOpen: boolean
  onClose: () => void
  /** Called after a successful import so the page can reload its profile list. */
  onImported: (count: number) => void
}

function formatSpeed(bps: number): string {
  if (!bps || bps <= 0) return '—'
  const mbps = bps / 1e6
  if (mbps >= 1) return `${mbps.toFixed(1)} Mbps`
  return `${(bps / 1e3).toFixed(0)} Kbps`
}

export default function OpenVPNCatalogModal({ isOpen, onClose, onImported }: Props) {
  const [sources, setSources] = useState<OpenVPNSourceInfo[]>([])
  const [sourceId, setSourceId] = useState('vpngate')
  const [mirrorUrl, setMirrorUrl] = useState('')
  const [servers, setServers] = useState<OpenVPNServer[]>([])
  const [selectedIds, setSelectedIds] = useState<string[]>([])
  const [search, setSearch] = useState('')
  const [username, setUsername] = useState('')
  const [password, setPassword] = useState('')

  const [isFetching, setIsFetching] = useState(false)
  // Seconds the current fetch has been running. A 1.3 MB list can legitimately
  // take a minute, and a spinner with no movement is indistinguishable from a
  // hang, so the wait is made visible.
  const [fetchElapsed, setFetchElapsed] = useState(0)
  const [isTesting, setIsTesting] = useState(false)
  const [isImporting, setIsImporting] = useState(false)
  const [hasTested, setHasTested] = useState(false)
  const [progress, setProgress] = useState<OpenVPNServerTestProgress | null>(null)
  const [error, setError] = useState<string | null>(null)
  const [notice, setNotice] = useState<string | null>(null)

  const currentSource = useMemo(
    () => sources.find(s => s.id === sourceId) || null,
    [sources, sourceId]
  )

  // The catalogue list is static (it is a compile-time registry in Go), so it is
  // loaded once per open rather than on every render.
  useEffect(() => {
    if (!isOpen) return
    let cancelled = false
    api
      .listOpenVPNSources()
      .then(list => {
        if (cancelled) return
        if (Array.isArray(list) && list.length > 0) {
          setSources(list)
          // Only adopt the first source as the default; a user who already
          // picked one should not be reset by a re-open.
          setSourceId(prev => (list.some(s => s.id === prev) ? prev : list[0].id))
        }
      })
      .catch(err => {
        console.error('Failed to list OpenVPN sources:', err)
      })
    return () => {
      cancelled = true
    }
  }, [isOpen])

  // Reachability progress is streamed, not returned: a 98-relay sweep takes
  // tens of seconds and the final `testOpenVPNServers` promise only settles at
  // the end.
  useEffect(() => {
    if (!isOpen) return
    const unsubProgress = api.onEvent('openvpn_sources:test_progress', (p: OpenVPNServerTestProgress) => {
      setProgress(p)
    })
    return () => {
      unsubProgress()
    }
  }, [isOpen])

  // Reset transient state whenever the dialog closes, so re-opening it never
  // shows a stale error banner or a half-finished progress bar.
  useEffect(() => {
    if (isOpen) return
    setError(null)
    setNotice(null)
    setProgress(null)
    setIsTesting(false)
    setIsImporting(false)
  }, [isOpen])

  useEffect(() => {
    if (!isFetching) {
      setFetchElapsed(0)
      return
    }
    const started = Date.now()
    setFetchElapsed(0)
    const id = window.setInterval(() => {
      setFetchElapsed(Math.floor((Date.now() - started) / 1000))
    }, 1000)
    return () => window.clearInterval(id)
  }, [isFetching])

  const handleFetch = useCallback(async () => {
    setIsFetching(true)
    setError(null)
    setNotice(null)
    try {
      const list = await api.fetchOpenVPNServers(sourceId, mirrorUrl.trim())
      const rows = Array.isArray(list) ? list : []
      setServers(rows)
      setSelectedIds([])
      setHasTested(false)
      setProgress(null)
      if (rows.length === 0) {
        setError(t('ovpn_catalog_empty'))
      }
    } catch (err) {
      // callGo rethrows the Go error, so the message is the one the backend
      // wrote — which for this call names the source and the transport failure.
      console.error('Fetch OpenVPN catalogue error:', err)
      setError(String(err))
      setServers([])
      setSelectedIds([])
    } finally {
      setIsFetching(false)
    }
  }, [sourceId, mirrorUrl])

  const handleTest = useCallback(async () => {
    if (servers.length === 0) return
    setIsTesting(true)
    setError(null)
    setNotice(null)
    setProgress({ tested_count: 0, total_count: servers.length, current: '' })
    try {
      const results = await api.testOpenVPNServers(servers)
      if (Array.isArray(results)) {
        setServers(results)
        const alive = results.filter(x => x.is_alive).length
        setNotice(t('ovpn_catalog_alive', { n: alive }))
      }
      setHasTested(true)
    } catch (err) {
      console.error('Test OpenVPN catalogue error:', err)
      setError(String(err))
    } finally {
      setIsTesting(false)
      setProgress(null)
    }
  }, [servers])

  const filtered = useMemo(() => {
    const q = search.trim().toLowerCase()
    if (!q) return servers
    return servers.filter(s => {
      return (
        s.country.toLowerCase().includes(q) ||
        s.country_code.toLowerCase().includes(q) ||
        s.ip.toLowerCase().includes(q) ||
        s.host_name.toLowerCase().includes(q) ||
        s.operator.toLowerCase().includes(q)
      )
    })
  }, [servers, search])

  const toggleSelect = (id: string) => {
    setSelectedIds(prev => (prev.includes(id) ? prev.filter(x => x !== id) : [...prev, id]))
  }

  const allFilteredSelected =
    filtered.length > 0 && filtered.every(s => selectedIds.includes(s.id))

  const handleSelectAll = () => {
    if (allFilteredSelected) {
      setSelectedIds([])
    } else {
      setSelectedIds(filtered.map(s => s.id))
    }
  }

  const handleImport = async () => {
    if (selectedIds.length === 0) return
    const toImport = servers.filter(s => selectedIds.includes(s.id))
    if (toImport.length === 0) return

    setIsImporting(true)
    setError(null)
    setNotice(null)
    try {
      const count = await api.importOpenVPNServers(
        toImport,
        currentSource?.name || 'VPN Gate',
        username.trim(),
        password.trim()
      )
      setNotice(t('ovpn_catalog_imported', { n: count }))
      setSelectedIds([])
      onImported(count)
    } catch (err) {
      // The stale-list error is written to be read by the user, so surface it
      // verbatim rather than collapsing it to a generic failure.
      console.error('Import OpenVPN catalogue error:', err)
      setError(String(err))
    } finally {
      setIsImporting(false)
    }
  }

  const hasServers = servers.length > 0

  return (
    <ModalShell
      isOpen={isOpen}
      onClose={onClose}
      maxWidth="max-w-3xl"
      className="max-h-[90vh] overflow-y-auto"
      bodyClassName="px-5 py-4 space-y-4"
    >
      {/* Header */}
      <div className="flex items-start justify-between gap-3 border-b border-border pb-3">
        <div className="flex items-center gap-3">
          <div className="w-10 h-10 rounded-xl border border-border bg-muted flex items-center justify-center text-primary shrink-0">
            <Globe className="w-5 h-5" />
          </div>
          <div>
            <h3 className="font-bold text-foreground text-sm">{t('ovpn_catalog_title')}</h3>
            <p className="text-[11px] text-muted-foreground leading-relaxed">
              {t('ovpn_catalog_subtitle')}
            </p>
          </div>
        </div>
        <button
          onClick={onClose}
          className="text-muted-foreground hover:text-foreground p-1.5 rounded-lg hover:bg-muted transition shrink-0"
        >
          <X className="w-5 h-5" />
        </button>
      </div>

      {/* Source + mirror */}
      <div className="grid grid-cols-1 md:grid-cols-2 gap-3">
        <div>
          <label className="text-xs font-semibold text-muted-foreground block mb-1.5 flex items-center gap-1.5">
            <Server className="w-3.5 h-3.5 text-primary" />
            <span>{t('ovpn_catalog_source')}</span>
          </label>
          <select
            className="form-control w-full text-xs font-semibold bg-surface border-border hover:border-primary transition cursor-pointer"
            value={sourceId}
            onChange={e => {
              setSourceId(e.target.value)
              // A mirror is per-source: carrying it across a source change would
              // silently point the new source's fetch at the old one's relay.
              setMirrorUrl('')
              setServers([])
              setSelectedIds([])
              setHasTested(false)
              setError(null)
              setNotice(null)
            }}
            disabled={isFetching || isTesting}
          >
            {sources.length === 0 && <option value="vpngate">VPN Gate</option>}
            {sources.map(s => (
              <option key={s.id} value={s.id}>
                {s.name}
              </option>
            ))}
          </select>
          {currentSource && (
            <p className="text-[10px] text-muted-foreground mt-1 leading-relaxed">
              <span className="font-semibold">{t('ovpn_catalog_operator')}:</span>{' '}
              {currentSource.operator}
            </p>
          )}
        </div>

        <div>
          <label className="text-xs font-semibold text-muted-foreground block mb-1.5 flex items-center gap-1.5">
            <Link2 className="w-3.5 h-3.5 text-warning" />
            <span>{t('ovpn_catalog_mirror')}</span>
          </label>
          <input
            type="text"
            className="form-control w-full font-mono text-xs"
            value={mirrorUrl}
            onChange={e => setMirrorUrl(e.target.value)}
            placeholder="https://…"
            disabled={isFetching || isTesting}
          />
          <p className="text-[10px] text-muted-foreground mt-1 leading-relaxed">
            {t('ovpn_catalog_mirror_hint')}
          </p>
        </div>
      </div>

      {/* Action bar */}
      <div className="flex flex-wrap items-center gap-2">
        <Button size="sm" onClick={handleFetch} disabled={isFetching || isTesting}>
          <RefreshCw className={`w-3.5 h-3.5 ${isFetching ? 'animate-spin' : ''}`} />
          <span>{hasServers ? t('ovpn_catalog_refetch') : t('ovpn_catalog_fetch')}</span>
        </Button>

        <Button
          size="sm"
          variant="secondary"
          onClick={handleTest}
          disabled={isTesting || isFetching || !hasServers}
        >
          <Activity className={`w-3.5 h-3.5 ${isTesting ? 'animate-pulse' : ''}`} />
          <span>{isTesting ? t('ovpn_catalog_testing') : t('ovpn_catalog_test')}</span>
        </Button>

        <Button
          size="sm"
          variant="ghost"
          onClick={handleSelectAll}
          disabled={!hasServers || filtered.length === 0}
        >
          {allFilteredSelected ? (
            <CheckSquare className="w-3.5 h-3.5 text-primary" />
          ) : (
            <Square className="w-3.5 h-3.5" />
          )}
          <span>{allFilteredSelected ? t('ovpn_catalog_clear') : t('ovpn_catalog_select_all')}</span>
        </Button>

        {selectedIds.length > 0 && (
          <span className="text-[11px] font-semibold text-primary ms-auto">
            {t('ovpn_catalog_selected', { n: selectedIds.length })}
          </span>
        )}
      </div>

      {/* Credentials — only for a catalogue that signs in with a personal account */}
      {currentSource?.needs_credentials && (
        <div className="bg-muted border border-border p-3.5 rounded-xl space-y-2.5">
          <p className="text-[11px] text-warning font-medium leading-relaxed">
            {t('ovpn_catalog_needs_creds')}
          </p>
          <div className="grid grid-cols-1 md:grid-cols-2 gap-3">
            <div>
              <label className="text-xs font-semibold text-muted-foreground block mb-1.5 flex items-center gap-1.5">
                <Key className="w-3.5 h-3.5 text-warning" />
                <span>{t('ovpn_username')}</span>
              </label>
              <input
                type="text"
                className="form-control w-full text-xs font-mono"
                value={username}
                onChange={e => setUsername(e.target.value)}
              />
            </div>
            <div>
              <label className="text-xs font-semibold text-muted-foreground block mb-1.5 flex items-center gap-1.5">
                <Key className="w-3.5 h-3.5 text-warning" />
                <span>{t('ovpn_password')}</span>
              </label>
              <input
                type="password"
                className="form-control w-full text-xs font-mono"
                value={password}
                onChange={e => setPassword(e.target.value)}
              />
            </div>
          </div>
        </div>
      )}

      {/* Public-pool note */}
      {currentSource && !currentSource.needs_credentials && (
        <p className="text-[10px] text-muted-foreground leading-relaxed">
          {t('ovpn_catalog_no_creds')}
        </p>
      )}

      {/* Feedback */}
      {error && (
        <div className="p-3 rounded-xl bg-destructive/10 border border-destructive/30 flex items-start gap-2">
          <AlertTriangle className="w-3.5 h-3.5 text-destructive shrink-0 mt-0.5" />
          <p className="text-[11px] text-destructive leading-relaxed font-medium break-words">
            {error}
          </p>
        </div>
      )}
      {notice && !error && (
        <div className="p-3 rounded-xl bg-success/10 border border-success/30 flex items-start gap-2">
          <Check className="w-3.5 h-3.5 text-success shrink-0 mt-0.5" />
          <p className="text-[11px] text-success leading-relaxed font-medium">{notice}</p>
        </div>
      )}

      {/* Reachability progress */}
      {progress && (
        <div className="p-3 bg-primary/10 border border-primary/30 rounded-xl space-y-1.5">
          <div className="flex items-center justify-between text-xs text-primary">
            <span className="flex items-center gap-2">
              <Activity className="w-3.5 h-3.5 text-primary animate-spin" />
              <span>
                {t('ovpn_catalog_testing')} ({progress.tested_count} / {progress.total_count})
              </span>
            </span>
            <span className="font-mono text-muted-foreground text-[11px] truncate max-w-xs">
              {progress.current}
            </span>
          </div>
          <div className="w-full h-1.5 bg-card rounded-full overflow-hidden">
            <div
              className="bg-primary h-full transition-all duration-200"
              style={{
                width: `${Math.min(
                  100,
                  Math.round((progress.tested_count / (progress.total_count || 1)) * 100)
                )}%`,
              }}
            />
          </div>
        </div>
      )}

      {/* Loading.
          The list is ~1.3 MB and legitimately takes tens of seconds on a slow
          route, so a bare spinner reads as a hang. The elapsed counter says it
          is still moving, and the note appears only once the wait is long
          enough to be worrying. */}
      {isFetching && (
        <div className="py-10 text-center space-y-2">
          <RefreshCw className="w-6 h-6 mx-auto text-primary animate-spin" />
          <p className="text-xs text-muted-foreground">
            {t('ovpn_catalog_loading')}
            {fetchElapsed > 0 && (
              <span className="ms-1.5 font-mono text-[11px]">
                {fetchElapsed}
                {t('ovpn_catalog_seconds')}
              </span>
            )}
          </p>
          {fetchElapsed >= 10 && (
            <p className="text-[10px] text-muted-foreground leading-relaxed max-w-md mx-auto">
              {t('ovpn_catalog_loading_slow')}
            </p>
          )}
        </div>
      )}

      {/* Table */}
      {!isFetching && hasServers && (
        <>
          <div className="relative">
            <Search className="w-3.5 h-3.5 text-muted-foreground absolute start-3 top-2.5" />
            <input
              type="text"
              value={search}
              onChange={e => setSearch(e.target.value)}
              placeholder={t('ovpn_catalog_search')}
              className="w-full ps-8 pe-3 py-1.5 bg-card border border-border rounded-xl text-xs text-foreground placeholder-muted-foreground focus:outline-none focus:border-primary/50"
            />
          </div>

          {filtered.length === 0 ? (
            <div className="py-10 text-center">
              <p className="text-xs text-muted-foreground">{t('ovpn_catalog_no_results')}</p>
            </div>
          ) : (
            <div className="max-h-[42vh] overflow-y-auto border border-border rounded-xl">
              <table className="w-full text-xs">
                <thead className="sticky top-0 bg-muted z-10">
                  <tr className="text-[10px] uppercase tracking-wide text-muted-foreground">
                    <th className="w-8 p-2"></th>
                    <th className="text-start p-2 font-semibold">{t('ovpn_catalog_col_country')}</th>
                    <th className="text-start p-2 font-semibold">{t('ovpn_catalog_col_endpoint')}</th>
                    <th className="text-end p-2 font-semibold">{t('ovpn_catalog_col_latency')}</th>
                    <th className="text-end p-2 font-semibold">{t('ovpn_catalog_col_sessions')}</th>
                    <th className="text-end p-2 font-semibold">{t('ovpn_catalog_col_speed')}</th>
                  </tr>
                </thead>
                <tbody>
                  {filtered.map(s => {
                    const checked = selectedIds.includes(s.id)
                    return (
                      <tr
                        key={s.id}
                        onClick={() => toggleSelect(s.id)}
                        className={`border-t border-border cursor-pointer transition-colors ${
                          checked ? 'bg-primary/10' : 'hover:bg-muted/60'
                        }`}
                      >
                        <td className="p-2">
                          <input
                            type="checkbox"
                            checked={checked}
                            onChange={() => toggleSelect(s.id)}
                            onClick={e => e.stopPropagation()}
                            className="accent-primary cursor-pointer"
                          />
                        </td>
                        <td className="p-2">
                          <span className="flex items-center gap-2">
                            {/* CountryFlag renders a flag-icons sprite, not an
                                emoji: no emoji font is embedded in the app, so
                                the old getCountryFlag() emoji came out blank. */}
                            <CountryFlag
                              codeOrName={s.country_code || s.country}
                              size="sm"
                              circular={false}
                            />
                            <span className="truncate">{s.country || s.country_code || '—'}</span>
                          </span>
                        </td>
                        <td className="p-2 font-mono text-[11px] text-muted-foreground">
                          <span className="block truncate">{s.ip || s.host_name}</span>
                          <span className="text-[10px]">:{s.port}</span>
                        </td>
                        <td className="p-2 text-end font-mono">
                          {!hasTested ? (
                            <span className="text-muted-foreground">{t('ovpn_catalog_not_tested')}</span>
                          ) : s.is_alive && s.latency_ms > 0 ? (
                            <span className="text-success font-semibold">{s.latency_ms} ms</span>
                          ) : (
                            <span className="text-destructive">{t('ovpn_catalog_unreachable')}</span>
                          )}
                        </td>
                        <td className="p-2 text-end font-mono text-muted-foreground">
                          {s.sessions > 0 ? s.sessions.toLocaleString() : '—'}
                        </td>
                        <td className="p-2 text-end font-mono text-muted-foreground">
                          {formatSpeed(s.speed_bps)}
                        </td>
                      </tr>
                    )
                  })}
                </tbody>
              </table>
            </div>
          )}
        </>
      )}

      {/* Empty state */}
      {!isFetching && !hasServers && !error && (
        <div className="py-10 text-center space-y-2">
          <Globe className="w-10 h-10 mx-auto text-muted-foreground opacity-40" />
          <p className="text-xs text-muted-foreground">{t('ovpn_catalog_empty')}</p>
        </div>
      )}

      {/* Footer */}
      <div className="flex items-center justify-end gap-2 pt-3 border-t border-border">
        <Button variant="secondary" size="sm" onClick={onClose}>
          {t('cancel')}
        </Button>
        <Button
          size="sm"
          onClick={handleImport}
          disabled={isImporting || selectedIds.length === 0}
        >
          <Download className="w-3.5 h-3.5" />
          <span>
            {t('ovpn_catalog_import')}
            {selectedIds.length > 0 ? ` (${selectedIds.length})` : ''}
          </span>
        </Button>
      </div>
    </ModalShell>
  )
}
