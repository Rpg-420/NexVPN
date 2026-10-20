import { useState, useEffect, useMemo, useCallback, useRef } from 'react'
import {
  X,
  Globe,
  Plus,
  Trash2,
  Key,
  Check,
  AlertTriangle,
  Activity,
  Download,
  Search,
  ShieldCheck,
  UserCheck,
  Pencil,
  Square,
} from 'lucide-react'
import { api } from '../lib/api'
import { t, getLanguage } from '../lib/i18n'
import type { TunnelBearCountry, TunnelBearAccount } from '../types'
import CountryFlag from './CountryFlag'
import { ModalShell } from '../components/ui/modal-shell'
import { Button } from '@/components/ui/button'

interface Props {
  isOpen: boolean
  onClose: () => void
  /** Called after a country is added, so the page can reload its profile list. */
  onImported: () => void
}

type Tab = 'servers' | 'accounts'

/** latencies maps country id to a measured figure, or -1 for "did not answer". */
type LatencyMap = Record<string, number>

export default function TunnelBearModal({ isOpen, onClose, onImported }: Props) {
  const [tab, setTab] = useState<Tab>('servers')

  const [countries, setCountries] = useState<TunnelBearCountry[]>([])
  const [accounts, setAccounts] = useState<TunnelBearAccount[]>([])
  const [backend, setBackend] = useState('')
  const [search, setSearch] = useState('')

  const [latencies, setLatencies] = useState<LatencyMap>({})
  const [testingId, setTestingId] = useState<string | null>(null)
  const [importingId, setImportingId] = useState<string | null>(null)

  /**
   * Bulk reachability sweep. `running` drives the progress line and the stop
   * button, `done`/`total` count the visible list. A ref, not state, carries the
   * cancel flag so a stop takes effect between two awaited probes instead of
   * after a re-render.
   */
  const [bulk, setBulk] = useState<{ running: boolean; done: number; total: number }>({
    running: false,
    done: 0,
    total: 0,
  })
  const bulkAbortRef = useRef(false)

  const [error, setError] = useState<string | null>(null)
  const [notice, setNotice] = useState<string | null>(null)

  // Account form. `editingId` empty means "adding".
  const [editingId, setEditingId] = useState('')
  const [formUsername, setFormUsername] = useState('')
  const [formPassword, setFormPassword] = useState('')
  const [savingAccount, setSavingAccount] = useState(false)

  const isFa = getLanguage() === 'fa'

  const refreshAccounts = useCallback(async () => {
    try {
      const list = await api.listTunnelBearAccounts()
      setAccounts(Array.isArray(list) ? list : [])
    } catch (err) {
      console.error('Failed to load TunnelBear accounts:', err)
    }
  }, [])

  useEffect(() => {
    if (!isOpen) return
    let cancelled = false

    api
      .listTunnelBearCountries()
      .then(list => {
        if (!cancelled && Array.isArray(list)) setCountries(list)
      })
      .catch(err => console.error('Failed to load TunnelBear countries:', err))

    api
      .tunnelBearSecretBackend()
      .then(name => {
        if (!cancelled) setBackend(name)
      })
      .catch(() => undefined)

    void refreshAccounts()

    return () => {
      cancelled = true
    }
  }, [isOpen, refreshAccounts])

  // Closing must not leave a stale error or a half-filled edit form behind.
  useEffect(() => {
    if (isOpen) return
    // A sweep still in flight must not keep probing (or writing into) a closed
    // modal, so the cancel flag is raised before the state is reset.
    bulkAbortRef.current = true
    setBulk({ running: false, done: 0, total: 0 })
    setError(null)
    setNotice(null)
    setTestingId(null)
    setImportingId(null)
    setEditingId('')
    setFormUsername('')
    setFormPassword('')
  }, [isOpen])

  const activeAccount = useMemo(() => accounts.find(a => a.active) || null, [accounts])

  const filtered = useMemo(() => {
    const q = search.trim().toLowerCase()
    if (!q) return countries
    return countries.filter(c => {
      return (
        c.name.toLowerCase().includes(q) ||
        c.name_fa.includes(q) ||
        c.code.toLowerCase().includes(q) ||
        c.host.toLowerCase().includes(q)
      )
    })
  }, [countries, search])

  const resetForm = () => {
    setEditingId('')
    setFormUsername('')
    setFormPassword('')
  }

  const handleSaveAccount = async () => {
    if (!formUsername.trim()) {
      setError(t('tb_username_required'))
      return
    }
    // Adding requires a password; editing may leave it blank to keep the stored
    // one, which the backend understands and the UI explains.
    if (!editingId && !formPassword) {
      setError(t('tb_password_required'))
      return
    }

    setSavingAccount(true)
    setError(null)
    setNotice(null)
    try {
      await api.saveTunnelBearAccount(editingId, formUsername.trim(), formPassword)
      resetForm()
      await refreshAccounts()
    } catch (err) {
      console.error('Save TunnelBear account error:', err)
      setError(String(err))
    } finally {
      setSavingAccount(false)
    }
  }

  const handleDeleteAccount = async (acc: TunnelBearAccount) => {
    if (!confirm(t('tb_confirm_delete_account'))) return
    setError(null)
    setNotice(null)
    try {
      await api.deleteTunnelBearAccount(acc.id)
      if (editingId === acc.id) resetForm()
      await refreshAccounts()
    } catch (err) {
      console.error('Delete TunnelBear account error:', err)
      setError(String(err))
    }
  }

  const handleSetActive = async (acc: TunnelBearAccount) => {
    setError(null)
    try {
      await api.setActiveTunnelBearAccount(acc.id)
      await refreshAccounts()
    } catch (err) {
      console.error('Set active TunnelBear account error:', err)
      setError(String(err))
    }
  }

  const handleTest = async (country: TunnelBearCountry) => {
    setTestingId(country.id)
    setError(null)
    try {
      const ms = await api.testTunnelBearCountry(country.id)
      setLatencies(prev => ({ ...prev, [country.id]: typeof ms === 'number' ? ms : -1 }))
    } catch (err) {
      console.error('Test TunnelBear country error:', err)
      setLatencies(prev => ({ ...prev, [country.id]: -1 }))
    } finally {
      setTestingId(null)
    }
  }

  /**
   * Test every country currently on screen, a few at a time.
   *
   * Sequentially this is the worst case for the user: the list has dozens of
   * rows, each probe has to wait out its own timeout when a host is dead, and
   * the only way to get a full picture was to click every row in turn. A small
   * worker pool keeps the sweep bounded — four sockets at once is enough to
   * finish quickly without looking like a port scan to the far end.
   *
   * The sweep covers `filtered`, so the search box doubles as a scope selector:
   * type "de" and test only the German gateways.
   */
  const handleTestAll = useCallback(async () => {
    if (bulk.running) return
    const targets = filtered
    if (targets.length === 0) return

    bulkAbortRef.current = false
    setError(null)
    setBulk({ running: true, done: 0, total: targets.length })

    const CONCURRENCY = 4
    let cursor = 0
    let completed = 0

    const worker = async () => {
      for (;;) {
        if (bulkAbortRef.current) return
        const index = cursor
        cursor += 1
        if (index >= targets.length) return

        const country = targets[index]
        try {
          const ms = await api.testTunnelBearCountry(country.id)
          setLatencies(prev => ({ ...prev, [country.id]: typeof ms === 'number' ? ms : -1 }))
        } catch (err) {
          console.error('Test TunnelBear country error:', err)
          setLatencies(prev => ({ ...prev, [country.id]: -1 }))
        }
        completed += 1
        setBulk(prev => (prev.running ? { ...prev, done: completed } : prev))
      }
    }

    const workers = Math.min(CONCURRENCY, targets.length)
    await Promise.all(Array.from({ length: workers }, () => worker()))

    // A stopped sweep still reports how far it got, so the progress line never
    // lies about having finished the whole list.
    setBulk({ running: false, done: completed, total: targets.length })
  }, [bulk.running, filtered])

  const handleStopTestAll = () => {
    bulkAbortRef.current = true
  }

  const handleImport = async (country: TunnelBearCountry) => {
    setImportingId(country.id)
    setError(null)
    setNotice(null)
    try {
      await api.importTunnelBearProfile(country.id)
      setNotice(t('tb_imported'))
      onImported()
    } catch (err) {
      // The backend refuses to add a profile with no account saved, and says so
      // in a sentence worth reading — show it rather than a generic failure.
      console.error('Import TunnelBear profile error:', err)
      setError(String(err))
    } finally {
      setImportingId(null)
    }
  }

  const authBadge = (acc: TunnelBearAccount) => {
    if (acc.auth === 'ok') {
      return (
        <span className="inline-flex items-center gap-1 text-[10px] font-semibold text-success">
          <ShieldCheck className="w-3 h-3" />
          {t('tb_auth_ok')}
        </span>
      )
    }
    if (acc.auth === 'failed') {
      return (
        <span
          className="inline-flex items-center gap-1 text-[10px] font-semibold text-destructive"
          title={acc.last_error || ''}
        >
          <AlertTriangle className="w-3 h-3" />
          {t('tb_auth_failed')}
        </span>
      )
    }
    return (
      <span className="inline-flex items-center gap-1 text-[10px] font-medium text-muted-foreground">
        <Activity className="w-3 h-3" />
        {t('tb_auth_unknown')}
      </span>
    )
  }

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
            <h3 className="font-bold text-foreground text-sm">{t('tb_title')}</h3>
            <p className="text-[11px] text-muted-foreground leading-relaxed">{t('tb_subtitle')}</p>
          </div>
        </div>
        <button
          onClick={onClose}
          className="text-muted-foreground hover:text-foreground p-1.5 rounded-lg hover:bg-muted transition shrink-0"
        >
          <X className="w-5 h-5" />
        </button>
      </div>

      {/* Tabs */}
      <div className="flex items-center gap-1 p-0.5 bg-muted rounded-xl border border-border text-xs w-fit">
        <button
          onClick={() => setTab('servers')}
          className={`px-3.5 py-1.5 rounded-lg font-semibold transition-all ${
            tab === 'servers' ? 'bg-card text-foreground shadow-sm' : 'text-muted-foreground hover:text-foreground'
          }`}
        >
          {t('tb_tab_servers')}
        </button>
        <button
          onClick={() => setTab('accounts')}
          className={`px-3.5 py-1.5 rounded-lg font-semibold transition-all flex items-center gap-1.5 ${
            tab === 'accounts' ? 'bg-card text-foreground shadow-sm' : 'text-muted-foreground hover:text-foreground'
          }`}
        >
          {t('tb_tab_accounts')}
          {accounts.length > 0 && (
            <span className="text-[10px] px-1.5 py-0.5 rounded-full bg-primary/15 text-primary font-mono">
              {accounts.length}
            </span>
          )}
        </button>
      </div>

      {/* Feedback */}
      {error && (
        <div className="p-3 rounded-xl bg-destructive/10 border border-destructive/30 flex items-start gap-2">
          <AlertTriangle className="w-3.5 h-3.5 text-destructive shrink-0 mt-0.5" />
          <p className="text-[11px] text-destructive leading-relaxed font-medium break-words">{error}</p>
        </div>
      )}
      {notice && !error && (
        <div className="p-3 rounded-xl bg-success/10 border border-success/30 flex items-start gap-2">
          <Check className="w-3.5 h-3.5 text-success shrink-0 mt-0.5" />
          <p className="text-[11px] text-success leading-relaxed font-medium">{notice}</p>
        </div>
      )}

      {/* ── Accounts ─────────────────────────────────────────────── */}
      {tab === 'accounts' && (
        <div className="space-y-3">
          <div>
            <h4 className="text-xs font-bold text-foreground">{t('tb_accounts_title')}</h4>
            <p className="text-[10px] text-muted-foreground leading-relaxed mt-0.5">
              {t('tb_accounts_hint')}
            </p>
            {backend && (
              <p className="text-[10px] text-muted-foreground mt-1">
                <span className="font-semibold">{t('tb_secret_backend')}:</span> {backend}
              </p>
            )}
          </div>

          {accounts.length === 0 ? (
            <div className="py-6 text-center">
              <p className="text-xs text-muted-foreground">{t('tb_no_accounts')}</p>
            </div>
          ) : (
            <div className="border border-border rounded-xl divide-y divide-border">
              {accounts.map(acc => (
                <div key={acc.id} className="flex items-center gap-3 p-3">
                  <input
                    type="radio"
                    name="tb-active-account"
                    checked={acc.active}
                    onChange={() => handleSetActive(acc)}
                    className="accent-primary cursor-pointer shrink-0"
                    title={t('tb_set_active')}
                  />
                  <div className="min-w-0 flex-1">
                    <div className="flex items-center gap-2 flex-wrap">
                      <span className="text-xs font-semibold text-foreground truncate">{acc.username}</span>
                      {acc.active && (
                        <span className="text-[10px] px-1.5 py-0.5 rounded-full bg-primary/15 text-primary font-semibold">
                          {t('tb_active')}
                        </span>
                      )}
                    </div>
                    <div className="mt-0.5">{authBadge(acc)}</div>
                  </div>
                  <button
                    onClick={() => {
                      setEditingId(acc.id)
                      setFormUsername(acc.username)
                      // Never pre-filled: the password is not readable from the
                      // backend, and showing a placeholder in a password field
                      // implies otherwise.
                      setFormPassword('')
                      setError(null)
                      setNotice(null)
                    }}
                    className="p-1.5 rounded-lg text-muted-foreground hover:text-foreground hover:bg-muted transition"
                    title={t('tb_edit_account')}
                  >
                    <Pencil className="w-3.5 h-3.5" />
                  </button>
                  <button
                    onClick={() => handleDeleteAccount(acc)}
                    className="p-1.5 rounded-lg text-muted-foreground hover:text-destructive hover:bg-destructive/10 transition"
                    title={t('tb_delete')}
                  >
                    <Trash2 className="w-3.5 h-3.5" />
                  </button>
                </div>
              ))}
            </div>
          )}

          {/* Add / edit form */}
          <div className="border border-border rounded-xl p-3.5 space-y-3 bg-muted/40">
            <h4 className="text-xs font-bold text-foreground flex items-center gap-1.5">
              <UserCheck className="w-3.5 h-3.5 text-primary" />
              {editingId ? t('tb_edit_account') : t('tb_add_account')}
            </h4>
            <div className="grid grid-cols-1 md:grid-cols-2 gap-3">
              <div>
                <label className="text-xs font-semibold text-muted-foreground block mb-1.5">
                  {t('tb_username_label')}
                </label>
                <input
                  type="text"
                  className="form-control w-full text-xs"
                  value={formUsername}
                  onChange={e => setFormUsername(e.target.value)}
                  autoComplete="off"
                />
              </div>
              <div>
                <label className="text-xs font-semibold text-muted-foreground block mb-1.5 flex items-center gap-1.5">
                  <Key className="w-3.5 h-3.5 text-warning" />
                  <span>{t('tb_password_label')}</span>
                </label>
                <input
                  type="password"
                  className="form-control w-full text-xs"
                  value={formPassword}
                  onChange={e => setFormPassword(e.target.value)}
                  autoComplete="new-password"
                />
                {editingId && (
                  <p className="text-[10px] text-muted-foreground mt-1">{t('tb_password_keep')}</p>
                )}
              </div>
            </div>
            <div className="flex items-center justify-end gap-2">
              {editingId && (
                <Button variant="secondary" size="sm" onClick={resetForm}>
                  {t('tb_cancel')}
                </Button>
              )}
              <Button size="sm" onClick={handleSaveAccount} disabled={savingAccount}>
                {editingId ? <Check className="w-3.5 h-3.5" /> : <Plus className="w-3.5 h-3.5" />}
                <span>{savingAccount ? t('tb_saving') : t('tb_save')}</span>
              </Button>
            </div>
          </div>
        </div>
      )}

      {/* ── Servers ──────────────────────────────────────────────── */}
      {tab === 'servers' && (
        <div className="space-y-3">
          {/* The gate is "is any account saved", not "is one marked active":
              the backend falls back to the first account when none is, so a
              missing selection is not a reason to refuse to import. */}
          {accounts.length === 0 ? (
            <div className="p-3 rounded-xl bg-warning/10 border border-warning/30 flex items-start gap-2">
              <AlertTriangle className="w-3.5 h-3.5 text-warning shrink-0 mt-0.5" />
              <button
                onClick={() => setTab('accounts')}
                className="text-[11px] text-warning leading-relaxed font-medium text-start hover:underline"
              >
                {t('tb_needs_account')}
              </button>
            </div>
          ) : (
            activeAccount && (
              <p className="text-[10px] text-muted-foreground flex items-center gap-1.5">
                <UserCheck className="w-3 h-3 text-primary shrink-0" />
                <span>
                  {t('tb_signing_in_as')} <span className="font-semibold text-foreground">{activeAccount.username}</span>
                </span>
              </p>
            )
          )}

          <div>
            <h4 className="text-xs font-bold text-foreground">
              {t('tb_servers_title')}
              <span className="ms-1.5 font-mono text-[10px] text-muted-foreground">({countries.length})</span>
            </h4>
            <p className="text-[10px] text-muted-foreground leading-relaxed mt-0.5">
              {t('tb_servers_hint')}
            </p>
          </div>

          <div className="flex items-center gap-2">
            <div className="relative flex-1 min-w-0">
              <Search className="w-3.5 h-3.5 text-muted-foreground absolute start-3 top-2.5" />
              <input
                type="text"
                value={search}
                onChange={e => setSearch(e.target.value)}
                placeholder={t('tb_search')}
                className="w-full ps-8 pe-3 py-1.5 bg-card border border-border rounded-xl text-xs text-foreground placeholder-muted-foreground focus:outline-none focus:border-primary/50"
              />
            </div>
            {/* The sweep runs over the filtered list, so the search box doubles
                as a scope selector rather than forcing a test of all rows. */}
            <Button
              variant="outline"
              size="sm"
              onClick={bulk.running ? handleStopTestAll : handleTestAll}
              disabled={!bulk.running && filtered.length === 0}
              title={bulk.running ? t('tb_test_stop') : t('tb_test_all')}
              className="shrink-0"
            >
              {bulk.running ? <Square className="w-3.5 h-3.5" /> : <Activity className="w-3.5 h-3.5" />}
              <span>{bulk.running ? t('tb_test_stop') : t('tb_test_all')}</span>
            </Button>
          </div>

          {bulk.running && (
            <p className="text-[10px] text-muted-foreground flex items-center gap-1.5">
              <Activity className="w-3 h-3 text-primary animate-pulse shrink-0" />
              <span className="font-mono">
                {t('tb_test_progress')} {bulk.done}/{bulk.total}
              </span>
            </p>
          )}

          <div className="max-h-[46vh] overflow-y-auto border border-border rounded-xl">
            <table className="w-full text-xs">
              <thead className="sticky top-0 bg-muted z-10">
                <tr className="text-[10px] uppercase tracking-wide text-muted-foreground">
                  <th className="text-start p-2 font-semibold">{t('tb_col_country')}</th>
                  <th className="text-start p-2 font-semibold">{t('tb_col_host')}</th>
                  <th className="text-end p-2 font-semibold">{t('tb_col_latency')}</th>
                  <th className="text-end p-2 font-semibold"></th>
                </tr>
              </thead>
              <tbody>
                {filtered.map(c => {
                  const ms = latencies[c.id]
                  const tested = ms !== undefined
                  const busy = testingId === c.id
                  const adding = importingId === c.id
                  return (
                    <tr key={c.id} className="border-t border-border hover:bg-muted/50 transition-colors">
                      <td className="p-2">
                        <span className="flex items-center gap-1.5">
                          {/* The sprite-based component, not an emoji flag: no emoji
                              font is embedded, so `getCountryFlag`'s 🇩🇪 rendered as
                              an empty box and the column looked flag-less. */}
                          <CountryFlag codeOrName={c.code} size="xs" circular />
                          <span className="truncate">{isFa ? c.name_fa : c.name}</span>
                        </span>
                      </td>
                      <td className="p-2 font-mono text-[11px] text-muted-foreground">
                        <span className="block truncate">{c.host}</span>
                      </td>
                      <td className="p-2 text-end font-mono">
                        {!tested ? (
                          <span className="text-muted-foreground">{t('tb_not_tested')}</span>
                        ) : ms < 0 ? (
                          <span className="text-destructive">{t('tb_unreachable')}</span>
                        ) : (
                          <span className="text-success font-semibold">{ms} ms</span>
                        )}
                      </td>
                      <td className="p-2">
                        <div className="flex items-center justify-end gap-1.5">
                          <Button
                            variant="ghost"
                            size="sm"
                            onClick={() => handleTest(c)}
                            disabled={busy}
                            title={t('tb_test')}
                          >
                            {busy ? (
                              <Activity className="w-3.5 h-3.5 animate-pulse" />
                            ) : (
                              <Activity className="w-3.5 h-3.5" />
                            )}
                            {busy && <span>{t('tb_testing')}</span>}
                          </Button>
                          <Button
                            size="sm"
                            onClick={() => handleImport(c)}
                            disabled={adding}
                            title={t('tb_import')}
                          >
                            <Download className="w-3.5 h-3.5" />
                            <span>{t('tb_import')}</span>
                          </Button>
                        </div>
                      </td>
                    </tr>
                  )
                })}
              </tbody>
            </table>
            {filtered.length === 0 && (
              <div className="py-8 text-center">
                <p className="text-xs text-muted-foreground">{t('tb_search')}</p>
              </div>
            )}
          </div>
        </div>
      )}

      {/* Footer */}
      <div className="flex items-center justify-end gap-2 pt-3 border-t border-border">
        <Button variant="secondary" size="sm" onClick={onClose}>
          {t('cancel')}
        </Button>
      </div>
    </ModalShell>
  )
}
