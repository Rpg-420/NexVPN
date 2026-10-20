import { useState } from 'react'
import { Clock, Network, Plus, Trash2, Wifi } from 'lucide-react'

import { Button } from '@/components/ui/button'
import { Card } from '@/components/ui/card'
import { Switch } from '@/components/ui/switch'
import { t } from '../lib/i18n'
import type {
  NetworkRule,
  NetworkRuleAction,
  RoutingRule,
  RoutingRuleAction,
  RoutingRuleKind,
  ScheduleRule,
  Settings,
} from '../types'

/**
 * AutomationPanel — the three rule editors that decide when the app connects
 * and how it routes, without the user having to touch a config file.
 *
 * Every editor here writes through the same `update` callback the rest of the
 * Settings page uses, so the debounced auto-save, the "unsaved" indicator and
 * the pending-write flush on unmount all apply without this component knowing
 * about them.
 *
 * Rules are validated before they are added, not on save. A rule that the
 * backend would silently drop (see NormalizeRoutingRules in Go) is a rule the
 * user believes is active — refusing it in the form is the only way the two
 * halves stay honest with each other.
 */
interface Props {
  settings: Settings
  update: <K extends keyof Settings>(key: K, value: Settings[K]) => void
}

/** A new rule id. Only has to be unique within one settings document. */
function newId(prefix: string): string {
  return `${prefix}_${Date.now().toString(36)}_${Math.random().toString(36).slice(2, 8)}`
}

const DAY_KEYS = ['adv_day_0', 'adv_day_1', 'adv_day_2', 'adv_day_3', 'adv_day_4', 'adv_day_5', 'adv_day_6']

/** Client-side validation mirroring NormalizeRoutingRule in Go. */
function validateRoutingValue(kind: RoutingRuleKind, value: string): boolean {
  const v = value.trim()
  if (!v) return false
  if (kind === 'domain') {
    return /^[a-z0-9*]([a-z0-9*.-]*[a-z0-9*])?$/i.test(v.replace(/^\*\./, '')) && !v.includes('/')
  }
  if (kind === 'ip') {
    return /^(\d{1,3}\.){3}\d{1,3}$/.test(v) || /^[0-9a-f:]+$/i.test(v) && v.includes(':')
  }
  // CIDR
  return /^(\d{1,3}\.){3}\d{1,3}\/\d{1,2}$/.test(v) || (v.includes(':') && v.includes('/'))
}

export default function AutomationPanel({ settings, update }: Props) {
  const networkRules = settings.network_rules ?? []
  const routingRules = settings.routing_rules ?? []
  const scheduleRules = settings.schedule_rules ?? []

  // --- Network rules -------------------------------------------------------
  const [ssidPattern, setSsidPattern] = useState('')
  const [ssidAction, setSsidAction] = useState<NetworkRuleAction>('connect')
  const [detectedSSID, setDetectedSSID] = useState('')
  const [ssidError, setSsidError] = useState('')

  const addNetworkRule = () => {
    const pattern = ssidPattern.trim()
    if (!pattern) {
      setSsidError(t('adv_net_pattern_hint'))
      return
    }
    setSsidError('')
    const rule: NetworkRule = {
      id: newId('net'),
      pattern,
      action: ssidAction,
      enabled: true,
    }
    update('network_rules', [...networkRules, rule])
    setSsidPattern('')
  }

  const detectSSID = async () => {
    const ssid = await apiCurrentSSID()
    setDetectedSSID(ssid)
    if (ssid) setSsidPattern(ssid)
  }

  // --- Routing rules -------------------------------------------------------
  const [routeKind, setRouteKind] = useState<RoutingRuleKind>('domain')
  const [routeValue, setRouteValue] = useState('')
  const [routeAction, setRouteAction] = useState<RoutingRuleAction>('direct')
  const [routeError, setRouteError] = useState('')

  const addRoutingRule = () => {
    const value = routeValue.trim()
    if (!validateRoutingValue(routeKind, value)) {
      setRouteError(t('adv_routing_invalid'))
      return
    }
    // Refuse an exact duplicate rather than adding a rule that can never
    // change the outcome: two identical rules look like redundancy to the user
    // and like noise to the config generator.
    const duplicate = routingRules.some(
      r => r.kind === routeKind && r.value.toLowerCase() === value.toLowerCase() && r.action === routeAction
    )
    if (duplicate) {
      setRouteError(t('adv_routing_invalid'))
      return
    }
    setRouteError('')
    const rule: RoutingRule = {
      id: newId('route'),
      kind: routeKind,
      value,
      action: routeAction,
      enabled: true,
    }
    update('routing_rules', [...routingRules, rule])
    setRouteValue('')
  }

  // --- Schedule rules ------------------------------------------------------
  const [scheduleAction, setScheduleAction] = useState<'connect' | 'disconnect'>('connect')
  const [scheduleTime, setScheduleTime] = useState('09:00')
  const [scheduleDays, setScheduleDays] = useState<number[]>([])

  const toggleDay = (day: number) => {
    setScheduleDays(prev => (prev.includes(day) ? prev.filter(d => d !== day) : [...prev, day].sort()))
  }

  const addScheduleRule = () => {
    if (!/^\d{1,2}:\d{2}$/.test(scheduleTime)) return
    const rule: ScheduleRule = {
      id: newId('sched'),
      action: scheduleAction,
      time: scheduleTime,
      days: scheduleDays,
      enabled: true,
    }
    update('schedule_rules', [...scheduleRules, rule])
  }

  // --- Shared list helpers -------------------------------------------------
  const patchNetworkRule = (id: string, patch: Partial<NetworkRule>) =>
    update(
      'network_rules',
      networkRules.map(r => (r.id === id ? { ...r, ...patch } : r))
    )

  const patchRoutingRule = (id: string, patch: Partial<RoutingRule>) =>
    update(
      'routing_rules',
      routingRules.map(r => (r.id === id ? { ...r, ...patch } : r))
    )

  const patchScheduleRule = (id: string, patch: Partial<ScheduleRule>) =>
    update(
      'schedule_rules',
      scheduleRules.map(r => (r.id === id ? { ...r, ...patch } : r))
    )

  const selectClass =
    'bg-card border border-border rounded-xl px-3 py-1.5 text-xs text-foreground focus:outline-none focus:border-primary/50'
  const inputClass =
    'bg-card border border-border rounded-xl px-3 py-1.5 text-xs text-foreground placeholder-muted-foreground focus:outline-none focus:border-primary/50'

  return (
    <div className="space-y-4">
      {/* ---------------- Wi-Fi network rules ---------------- */}
      <div className="section">
        <div className="section-title flex items-center gap-1.5">
          <Wifi size={14} className="text-primary" />
          <span>{t('adv_net_rules')}</span>
        </div>
        <Card className="space-y-3 p-4">
          <p className="text-xs text-muted-foreground">{t('adv_net_rules_desc')}</p>

          {networkRules.length === 0 ? (
            <p className="py-2 text-xs text-muted-foreground">{t('adv_net_empty')}</p>
          ) : (
            <div className="space-y-2">
              {networkRules.map(rule => (
                <div
                  key={rule.id}
                  className="flex flex-wrap items-center gap-2 rounded-xl border border-border bg-muted/40 p-2.5"
                >
                  <Switch
                    checked={rule.enabled}
                    onCheckedChange={checked => patchNetworkRule(rule.id, { enabled: checked })}
                    aria-label={rule.pattern}
                  />
                  <span className="min-w-0 flex-1 truncate font-mono text-xs text-foreground">
                    {rule.pattern}
                  </span>
                  <select
                    className={selectClass}
                    value={rule.action}
                    onChange={e =>
                      patchNetworkRule(rule.id, { action: e.target.value as NetworkRuleAction })
                    }
                  >
                    <option value="connect">{t('adv_net_action_connect')}</option>
                    <option value="direct">{t('adv_net_action_direct')}</option>
                    <option value="skip">{t('adv_net_action_skip')}</option>
                  </select>
                  <Button
                    variant="ghost"
                    size="icon-sm"
                    aria-label={t('delete')}
                    className="text-muted-foreground hover:bg-destructive/10 hover:text-destructive"
                    onClick={() =>
                      update(
                        'network_rules',
                        networkRules.filter(r => r.id !== rule.id)
                      )
                    }
                  >
                    <Trash2 className="h-3.5 w-3.5" />
                  </Button>
                </div>
              ))}
            </div>
          )}

          <div className="flex flex-wrap items-center gap-2 border-t border-border pt-3">
            <input
              className={`${inputClass} min-w-[160px] flex-1`}
              placeholder={t('adv_net_pattern')}
              value={ssidPattern}
              onChange={e => setSsidPattern(e.target.value)}
              onKeyDown={e => e.key === 'Enter' && addNetworkRule()}
            />
            <select
              className={selectClass}
              value={ssidAction}
              onChange={e => setSsidAction(e.target.value as NetworkRuleAction)}
            >
              <option value="connect">{t('adv_net_action_connect')}</option>
              <option value="direct">{t('adv_net_action_direct')}</option>
              <option value="skip">{t('adv_net_action_skip')}</option>
            </select>
            <Button size="sm" onClick={addNetworkRule}>
              <Plus className="h-3.5 w-3.5" />
              <span>{t('adv_net_add')}</span>
            </Button>
            <Button variant="secondary" size="sm" onClick={detectSSID}>
              <Wifi className="h-3.5 w-3.5" />
              <span>{t('adv_net_current')}</span>
            </Button>
          </div>
          {ssidError && <p className="text-[11px] text-destructive">{ssidError}</p>}
          {detectedSSID === '' && (
            <p className="text-[11px] text-muted-foreground">{t('adv_net_no_ssid')}</p>
          )}
          <p className="text-[11px] text-muted-foreground">{t('adv_net_pattern_hint')}</p>
        </Card>
      </div>

      {/* ---------------- Routing rules ---------------- */}
      <div className="section">
        <div className="section-title flex items-center gap-1.5">
          <Network size={14} className="text-primary" />
          <span>{t('adv_routing_rules')}</span>
        </div>
        <Card className="space-y-3 p-4">
          <p className="text-xs text-muted-foreground">{t('adv_routing_rules_desc')}</p>

          {routingRules.length === 0 ? (
            <p className="py-2 text-xs text-muted-foreground">{t('adv_routing_empty')}</p>
          ) : (
            <div className="space-y-2">
              {routingRules.map(rule => (
                <div
                  key={rule.id}
                  className="flex flex-wrap items-center gap-2 rounded-xl border border-border bg-muted/40 p-2.5"
                >
                  <Switch
                    checked={rule.enabled}
                    onCheckedChange={checked => patchRoutingRule(rule.id, { enabled: checked })}
                    aria-label={rule.value}
                  />
                  <span className="shrink-0 rounded-md border border-border bg-card px-1.5 py-0.5 font-mono text-[10px] uppercase text-muted-foreground">
                    {rule.kind}
                  </span>
                  <span className="min-w-0 flex-1 truncate font-mono text-xs text-foreground" dir="ltr">
                    {rule.value}
                  </span>
                  <select
                    className={selectClass}
                    value={rule.action}
                    onChange={e =>
                      patchRoutingRule(rule.id, { action: e.target.value as RoutingRuleAction })
                    }
                  >
                    <option value="proxy">{t('adv_routing_action_proxy')}</option>
                    <option value="direct">{t('adv_routing_action_direct')}</option>
                    <option value="block">{t('adv_routing_action_block')}</option>
                  </select>
                  <Button
                    variant="ghost"
                    size="icon-sm"
                    aria-label={t('delete')}
                    className="text-muted-foreground hover:bg-destructive/10 hover:text-destructive"
                    onClick={() =>
                      update(
                        'routing_rules',
                        routingRules.filter(r => r.id !== rule.id)
                      )
                    }
                  >
                    <Trash2 className="h-3.5 w-3.5" />
                  </Button>
                </div>
              ))}
            </div>
          )}

          <div className="flex flex-wrap items-center gap-2 border-t border-border pt-3">
            <select
              className={selectClass}
              value={routeKind}
              onChange={e => setRouteKind(e.target.value as RoutingRuleKind)}
            >
              <option value="domain">{t('adv_routing_kind_domain')}</option>
              <option value="ip">{t('adv_routing_kind_ip')}</option>
              <option value="cidr">{t('adv_routing_kind_cidr')}</option>
            </select>
            <input
              className={`${inputClass} min-w-[160px] flex-1`}
              dir="ltr"
              placeholder={t('adv_routing_value')}
              value={routeValue}
              onChange={e => setRouteValue(e.target.value)}
              onKeyDown={e => e.key === 'Enter' && addRoutingRule()}
            />
            <select
              className={selectClass}
              value={routeAction}
              onChange={e => setRouteAction(e.target.value as RoutingRuleAction)}
            >
              <option value="proxy">{t('adv_routing_action_proxy')}</option>
              <option value="direct">{t('adv_routing_action_direct')}</option>
              <option value="block">{t('adv_routing_action_block')}</option>
            </select>
            <Button size="sm" onClick={addRoutingRule}>
              <Plus className="h-3.5 w-3.5" />
              <span>{t('adv_routing_add')}</span>
            </Button>
          </div>
          {routeError && <p className="text-[11px] text-destructive">{routeError}</p>}
        </Card>
      </div>

      {/* ---------------- Schedule ---------------- */}
      <div className="section">
        <div className="section-title flex items-center gap-1.5">
          <Clock size={14} className="text-primary" />
          <span>{t('adv_schedule')}</span>
        </div>
        <Card className="space-y-3 p-4">
          <p className="text-xs text-muted-foreground">{t('adv_schedule_desc')}</p>

          {scheduleRules.length === 0 ? (
            <p className="py-2 text-xs text-muted-foreground">{t('adv_schedule_empty')}</p>
          ) : (
            <div className="space-y-2">
              {scheduleRules.map(rule => (
                <div
                  key={rule.id}
                  className="flex flex-wrap items-center gap-2 rounded-xl border border-border bg-muted/40 p-2.5"
                >
                  <Switch
                    checked={rule.enabled}
                    onCheckedChange={checked => patchScheduleRule(rule.id, { enabled: checked })}
                    aria-label={rule.time}
                  />
                  <span className="shrink-0 font-mono text-xs text-foreground">{rule.time}</span>
                  <span className="shrink-0 rounded-md border border-border bg-card px-1.5 py-0.5 text-[10px] uppercase text-muted-foreground">
                    {rule.action === 'connect' ? t('adv_schedule_connect') : t('adv_schedule_disconnect')}
                  </span>
                  <span className="min-w-0 flex-1 truncate text-xs text-muted-foreground">
                    {rule.days.length === 0
                      ? t('adv_schedule_every_day')
                      : rule.days.map(d => t(DAY_KEYS[d])).join('، ')}
                  </span>
                  <Button
                    variant="ghost"
                    size="icon-sm"
                    aria-label={t('delete')}
                    className="text-muted-foreground hover:bg-destructive/10 hover:text-destructive"
                    onClick={() =>
                      update(
                        'schedule_rules',
                        scheduleRules.filter(r => r.id !== rule.id)
                      )
                    }
                  >
                    <Trash2 className="h-3.5 w-3.5" />
                  </Button>
                </div>
              ))}
            </div>
          )}

          <div className="flex flex-wrap items-center gap-2 border-t border-border pt-3">
            <select
              className={selectClass}
              value={scheduleAction}
              onChange={e => setScheduleAction(e.target.value as 'connect' | 'disconnect')}
            >
              <option value="connect">{t('adv_schedule_connect')}</option>
              <option value="disconnect">{t('adv_schedule_disconnect')}</option>
            </select>
            <input
              type="time"
              className={inputClass}
              value={scheduleTime}
              onChange={e => setScheduleTime(e.target.value)}
            />
            <div className="flex flex-wrap items-center gap-1">
              {DAY_KEYS.map((key, day) => (
                <button
                  key={key}
                  type="button"
                  onClick={() => toggleDay(day)}
                  className={`rounded-lg border px-2 py-1 text-[10px] font-bold transition-colors ${
                    scheduleDays.includes(day)
                      ? 'border-primary bg-primary text-primary-foreground'
                      : 'border-border bg-card text-muted-foreground hover:text-foreground'
                  }`}
                >
                  {t(key)}
                </button>
              ))}
            </div>
            <Button size="sm" onClick={addScheduleRule}>
              <Plus className="h-3.5 w-3.5" />
              <span>{t('adv_schedule_add')}</span>
            </Button>
          </div>
        </Card>
      </div>
    </div>
  )
}

/**
 * Read the joined SSID.
 *
 * Kept as a local wrapper rather than importing the api module at the top of a
 * purely presentational component: the panel is only rendered from Settings,
 * and a failed call must leave the field editable by hand rather than throwing
 * out of an event handler.
 */
async function apiCurrentSSID(): Promise<string> {
  try {
    const { api } = await import('../lib/api')
    return (await api.currentNetworkSSID()) || ''
  } catch {
    return ''
  }
}
