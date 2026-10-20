import { useCallback, useEffect, useState, type CSSProperties } from 'react'
import {
  Activity,
  ArrowDown,
  ArrowUp,
  Clock,
  Gauge,
  RefreshCw,
  Server,
  Timer,
} from 'lucide-react'
import {
  Bar,
  BarChart,
  CartesianGrid,
  ResponsiveContainer,
  Tooltip,
  XAxis,
  YAxis,
} from 'recharts'

import { Button } from '@/components/ui/button'
import { Card } from '@/components/ui/card'
import { api } from '../lib/api'
import { getLanguage, t } from '../lib/i18n'
import { formatBytes, formatDuration } from '../lib/utils'
import type { ConnectionStatistics } from '../types'

/**
 * StatisticsPanel — the "how have I actually been using this" surface.
 *
 * Everything is derived server-side from the recorded connection history
 * (`GetConnectionStatistics`), so this component holds no aggregation logic of
 * its own beyond presentation. That matters: the same numbers must not drift
 * between the summary tiles and the chart.
 *
 * The panel is read-only. It is mounted inside the connection-history modal,
 * where the raw session list already lives, so the chart and the rows behind it
 * can never disagree about what is being shown.
 */

/** One row of the chart. Recharts needs the fields flattened onto the datum. */
interface ChartRow {
  date: string
  label: string
  minutes: number
  rx: number
  tx: number
}

/**
 * The `date` field is a plain `YYYY-MM-DD` from Go. `new Date('2026-10-01')` is
 * parsed as UTC midnight, which renders as the *previous* day anywhere west of
 * Greenwich — so the time is appended to force local interpretation.
 */
function dayLabel(date: string, lang: string): string {
  const parsed = new Date(`${date}T00:00:00`)
  if (Number.isNaN(parsed.getTime())) return date
  return parsed.toLocaleDateString(lang === 'fa' ? 'fa-IR' : 'en-US', {
    month: 'short',
    day: 'numeric',
  })
}

/** Themed tooltip styling. recharts takes inline styles, so the CSS variables
 * resolve against whichever theme is active without this component knowing
 * which one that is. */
const tooltipContentStyle: CSSProperties = {
  backgroundColor: 'hsl(var(--card))',
  border: '1px solid hsl(var(--border))',
  borderRadius: '0.75rem',
  fontSize: 11,
  color: 'hsl(var(--card-foreground))',
  padding: '6px 10px',
}
const tooltipLabelStyle: CSSProperties = {
  color: 'hsl(var(--muted-foreground))',
  fontSize: 10,
  marginBottom: 2,
}

interface Props {
  /** Reload when the history is cleared, so the panel cannot show stale totals. */
  refreshKey?: number
}

export default function StatisticsPanel({ refreshKey = 0 }: Props) {
  const [stats, setStats] = useState<ConnectionStatistics | null>(null)
  const [loading, setLoading] = useState(true)
  const lang = getLanguage()

  const load = useCallback(async () => {
    setLoading(true)
    try {
      setStats(await api.getConnectionStatistics())
    } catch (err) {
      console.warn('Statistics load error:', err)
    } finally {
      setLoading(false)
    }
  }, [])

  useEffect(() => {
    load()
  }, [load, refreshKey])

  if (loading && !stats) {
    return (
      <Card className="p-4 text-center text-xs text-muted-foreground">
        {t('querying')}
      </Card>
    )
  }

  if (!stats || stats.total_sessions === 0) {
    return (
      <Card className="space-y-1 p-6 text-center">
        <Activity className="mx-auto mb-1 h-6 w-6 text-muted-foreground opacity-50" />
        <p className="text-xs text-muted-foreground">{t('adv_stats_empty')}</p>
      </Card>
    )
  }

  const rows: ChartRow[] = (stats.daily || [])
    .map(day => ({
      date: day.date,
      label: dayLabel(day.date, lang),
      minutes: Math.round((day.duration_sec || 0) / 60),
      rx: day.bytes_rx || 0,
      tx: day.bytes_tx || 0,
    }))
    .sort((a, b) => a.date.localeCompare(b.date))

  const totalData = (stats.total_bytes_rx || 0) + (stats.total_bytes_tx || 0)
  const maxServerBytes = Math.max(1, ...(stats.top_servers || []).map(s => s.bytes || 0))
  // A single day of history draws a single bar, which looks like a rendering
  // bug rather than a chart. Two points is the honest minimum.
  const chartable = rows.length >= 2

  const tiles = [
    {
      icon: <Activity className="h-3.5 w-3.5 text-primary" />,
      label: t('adv_stats_sessions'),
      value: String(stats.total_sessions),
    },
    {
      icon: <Clock className="h-3.5 w-3.5 text-primary" />,
      label: t('adv_stats_duration'),
      value: formatDuration(stats.total_duration_sec || 0),
    },
    {
      icon: <ArrowDown className="h-3.5 w-3.5 text-primary" />,
      label: t('adv_stats_data'),
      value: formatBytes(totalData),
    },
    {
      icon: <Gauge className="h-3.5 w-3.5 text-primary" />,
      label: t('adv_stats_uptime'),
      value: `${Math.round(stats.uptime_percent || 0)}%`,
    },
    {
      icon: <Timer className="h-3.5 w-3.5 text-primary" />,
      label: t('adv_stats_avg'),
      value: formatDuration(stats.average_duration_sec || 0),
    },
  ]

  return (
    <div className="space-y-3">
      <div className="flex items-center justify-between gap-2">
        <span className="text-[11px] font-semibold uppercase tracking-wide text-muted-foreground">
          {t('adv_stats_window', { days: stats.window_days || 30 })}
        </span>
        <Button variant="ghost" size="sm" onClick={load} disabled={loading}>
          <RefreshCw className={`h-3.5 w-3.5 ${loading ? 'animate-spin' : ''}`} />
        </Button>
      </div>

      {/* Summary tiles */}
      <div className="grid grid-cols-2 gap-2 sm:grid-cols-3 lg:grid-cols-5">
        {tiles.map(tile => (
          <div
            key={tile.label}
            className="rounded-2xl border border-border bg-background p-3"
          >
            <div className="flex items-center gap-1.5 text-[10px] font-semibold uppercase text-muted-foreground">
              {tile.icon}
              <span className="min-w-0 truncate">{tile.label}</span>
            </div>
            <div className="mt-1 font-mono text-sm font-bold text-foreground" dir="ltr">
              {tile.value}
            </div>
          </div>
        ))}
      </div>

      {/* Daily usage chart */}
      <Card className="space-y-2 p-4">
        <div className="flex items-center gap-1.5 text-xs font-bold text-foreground">
          <Activity size={13} className="text-primary" />
          <span>{t('adv_stats_daily')}</span>
        </div>

        {!chartable ? (
          <p className="py-6 text-center text-[11px] text-muted-foreground">
            {t('adv_stats_no_chart')}
          </p>
        ) : (
          <>
            <div className="h-40 w-full" dir="ltr">
              <ResponsiveContainer width="100%" height="100%">
                <BarChart data={rows} margin={{ top: 4, right: 4, left: -18, bottom: 0 }}>
                  <CartesianGrid
                    strokeDasharray="3 3"
                    stroke="hsl(var(--border))"
                    vertical={false}
                  />
                  <XAxis
                    dataKey="label"
                    tick={{ fontSize: 10, fill: 'hsl(var(--muted-foreground))' }}
                    tickLine={false}
                    axisLine={{ stroke: 'hsl(var(--border))' }}
                    interval="preserveStartEnd"
                    minTickGap={12}
                  />
                  <YAxis
                    tick={{ fontSize: 10, fill: 'hsl(var(--muted-foreground))' }}
                    tickLine={false}
                    axisLine={false}
                    width={44}
                    allowDecimals={false}
                  />
                  <Tooltip
                    cursor={{ fill: 'hsl(var(--muted) / 0.4)' }}
                    contentStyle={tooltipContentStyle}
                    labelStyle={tooltipLabelStyle}
                    formatter={value => `${Number(value ?? 0)} ${t('adv_stats_minutes')}`}
                  />
                  <Bar
                    dataKey="minutes"
                    name={t('adv_stats_duration')}
                    fill="hsl(var(--primary))"
                    radius={[4, 4, 0, 0]}
                    maxBarSize={28}
                  />
                </BarChart>
              </ResponsiveContainer>
            </div>

            {/* The same series, as data — the chart is unreadable for anyone
                using a screen reader, and the numbers are what matter. */}
            <div className="flex flex-wrap items-center gap-3 text-[10px] text-muted-foreground">
              <span className="flex items-center gap-1">
                <span
                  className="inline-block h-2 w-2 rounded-sm"
                  style={{ backgroundColor: 'hsl(var(--primary))' }}
                />
                {t('adv_stats_duration')}
              </span>
            </div>
          </>
        )}
      </Card>

      {/* Per-day data table */}
      {chartable && (
        <Card className="overflow-hidden p-0">
          <table className="w-full text-[11px]">
            <thead>
              <tr className="border-b border-border bg-muted/40 text-[10px] uppercase text-muted-foreground">
                <th className="px-3 py-2 text-start font-semibold">{t('adv_stats_daily')}</th>
                <th className="px-3 py-2 text-start font-semibold">{t('adv_stats_sessions')}</th>
                <th className="px-3 py-2 text-start font-semibold">
                  <span className="flex items-center gap-1">
                    <ArrowDown size={10} />
                    {t('adv_stats_download')}
                  </span>
                </th>
                <th className="px-3 py-2 text-start font-semibold">
                  <span className="flex items-center gap-1">
                    <ArrowUp size={10} />
                    {t('adv_stats_upload')}
                  </span>
                </th>
              </tr>
            </thead>
            <tbody className="divide-y divide-border/60">
              {[...rows].reverse().map(row => (
                <tr key={row.date} className="hover:bg-muted/40">
                  <td className="px-3 py-1.5 font-mono text-foreground" dir="ltr">
                    {row.label}
                  </td>
                  <td className="px-3 py-1.5 font-mono text-muted-foreground" dir="ltr">
                    {row.minutes}m
                  </td>
                  <td className="px-3 py-1.5 font-mono text-foreground" dir="ltr">
                    {formatBytes(row.rx)}
                  </td>
                  <td className="px-3 py-1.5 font-mono text-foreground" dir="ltr">
                    {formatBytes(row.tx)}
                  </td>
                </tr>
              ))}
            </tbody>
          </table>
        </Card>
      )}

      {/* Most-used servers */}
      {(stats.top_servers || []).length > 0 && (
        <Card className="space-y-3 p-4">
          <div className="flex items-center gap-1.5 text-xs font-bold text-foreground">
            <Server size={13} className="text-primary" />
            <span>{t('adv_stats_top')}</span>
          </div>
          <div className="space-y-2.5">
            {stats.top_servers.map(server => (
              <div key={`${server.profile_name}-${server.protocol}`} className="space-y-1">
                <div className="flex items-center justify-between gap-2 text-[11px]">
                  <span className="flex min-w-0 items-center gap-1.5">
                    <span className="truncate font-medium text-foreground">
                      {server.profile_name}
                    </span>
                    {server.protocol && (
                      <span className="shrink-0 rounded-md bg-muted px-1.5 py-0.5 font-mono text-[9px] uppercase text-muted-foreground">
                        {server.protocol}
                      </span>
                    )}
                  </span>
                  <span className="shrink-0 font-mono text-muted-foreground" dir="ltr">
                    {server.sessions} × · {formatBytes(server.bytes || 0)}
                  </span>
                </div>
                <div className="h-1.5 w-full overflow-hidden rounded-full bg-muted">
                  <div
                    className="h-full rounded-full bg-primary/70"
                    style={{
                      width: `${Math.max(2, ((server.bytes || 0) / maxServerBytes) * 100)}%`,
                    }}
                  />
                </div>
              </div>
            ))}
          </div>
        </Card>
      )}
    </div>
  )
}
