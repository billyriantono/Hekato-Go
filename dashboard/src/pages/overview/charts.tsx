// Metrics charts for the Overview page. All series colours come from PALETTE; text/grid use theme tokens.
import { useMemo } from 'react'
import {
  Area,
  AreaChart,
  Bar,
  BarChart,
  CartesianGrid,
  Legend,
  ResponsiveContainer,
  Tooltip,
  XAxis,
  YAxis,
} from 'recharts'
import { EmptyState, LoadingBlock, formatMs, formatNumber } from '@/components/common'
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from '@/components/ui/card'
import { useI18n } from '@/lib/i18n'
import { StackedBar, short, type Bucket, type Metrics, type Range } from './bits'

// Series colours are theme tokens, so charts retint with the theme for free.
const slots = ['var(--chart-1)', 'var(--chart-2)', 'var(--chart-3)']
const error = 'var(--destructive)'

const tick = { fill: 'var(--muted-foreground)', fontSize: 10.5, fontFamily: 'var(--font-mono)' }
const tooltipProps = {
  contentStyle: {
    background: 'color-mix(in oklch, var(--popover) 92%, transparent)',
    backdropFilter: 'blur(8px)',
    border: '1px solid var(--border)',
    borderRadius: 10,
    boxShadow: '0 16px 40px -12px hsl(var(--shadow-color) / 0.45)',
    color: 'var(--popover-foreground)',
    fontSize: 12,
    fontFamily: 'var(--font-mono)',
    padding: '8px 12px',
  },
  labelStyle: { color: 'var(--muted-foreground)', fontSize: 10.5, letterSpacing: '0.08em', marginBottom: 4 },
  itemStyle: { color: 'var(--popover-foreground)', padding: 0 },
  animationDuration: 200,
}
const legendText = (v: string) => <span style={{ color: 'var(--muted-foreground)', fontSize: 11, fontFamily: 'var(--font-mono)' }}>{v}</span>
const grid = <CartesianGrid vertical={false} strokeDasharray="2 4" stroke="var(--border)" />
const ms = (v: unknown) => formatMs(Number(v))
/** Vertical fade under a series, for area fills and bar faces. */
const Fade = ({ id, color, from = 0.35, to = 0 }: { id: string; color: string; from?: number; to?: number }) => (
  <defs>
    <linearGradient id={id} x1="0" y1="0" x2="0" y2="1">
      <stop offset="0%" stopColor={color} stopOpacity={from} />
      <stop offset="100%" stopColor={color} stopOpacity={to} />
    </linearGradient>
  </defs>
)

function ChartCard({ title, description, empty, loading, children }: { title: string; description: string; empty: boolean; loading: boolean; children: React.ReactNode }) {
  const { t } = useI18n()
  return (
    <Card>
      <CardHeader>
        <CardTitle>{title}</CardTitle>
        <CardDescription>{description}</CardDescription>
      </CardHeader>
      <CardContent>{loading ? <div className="skeleton h-[220px] rounded-lg" /> : empty ? <EmptyState title={t('overview.noTraffic')} /> : <div className="h-[220px] animate-[fade_0.6s_var(--ease-out-expo)_both]">{children}</div>}</CardContent>
    </Card>
  )
}

/** Ranked horizontal bar list: requests (slot colour) with errors (status red) as a trailing segment. */
function TopList({ rows, colour, error, label }: { rows: Bucket[]; colour: string; error: string; label: (k: string) => string }) {
  const { t } = useI18n()
  const max = Math.max(1, ...rows.map((r) => r.requests))
  return (
    <div className="space-y-2.5">
      {rows.map((r) => (
        <div key={r.key} className="space-y-1">
          <div className="flex justify-between gap-2 text-xs">
            <span className="truncate font-mono" title={r.key}>{label(r.key)}</span>
            <span className="shrink-0 font-mono text-[11px] tabular-nums text-muted-foreground">
              {formatNumber(r.requests)}
              {r.errors > 0 && <span className="ml-1.5" style={{ color: error }}>{t('overview.errCount', formatNumber(r.errors))}</span>}
            </span>
          </div>
          <div style={{ width: `${(r.requests / max) * 100}%` }}>
            <StackedBar segments={[[colour, r.requests - r.errors], [error, r.errors]]} className="h-1.5 bg-transparent" />
          </div>
        </div>
      ))}
    </div>
  )
}

export default function MetricsCharts({ data, range, loading, accountNames }: { data?: Metrics; range: Range; loading: boolean; accountNames?: Record<string, string> }) {
  const { t } = useI18n()
  const empty = !loading && (data?.totals.requests ?? 0) === 0

  const fmtTime = useMemo(() => {
    const long = range === '24h' || range === '7d'
    const opts: Intl.DateTimeFormatOptions = long ? { month: 'short', day: 'numeric', hour: '2-digit' } : { hour: '2-digit', minute: '2-digit' }
    return (t: number) => (long ? new Date(t * 1000).toLocaleString([], opts) : new Date(t * 1000).toLocaleTimeString([], opts))
  }, [range])
  const series = useMemo(() => (data?.series ?? []).map((p) => ({ ...p, ok: p.requests - p.errors, label: fmtTime(p.t) })), [data, fmtTime])
  const top = (rows?: Bucket[]) => (rows ?? []).slice().sort((a, b) => b.requests - a.requests).slice(0, 6)
  const endpoints = top(data?.byEndpoint)
  const num = (v: unknown) => formatNumber(Number(v))

  return (
    <>
      <div className="grid gap-6 lg:grid-cols-3">
        <ChartCard title={t('overview.throughput')} description={t('overview.throughputHint')} empty={empty} loading={loading}>
          <ResponsiveContainer>
            <BarChart data={series} margin={{ top: 4, right: 4, left: -16, bottom: 0 }} barCategoryGap="18%">
              <Fade id="bar-ok" color={slots[0]} from={1} to={0.55} />
              {grid}
              <XAxis dataKey="label" tick={tick} tickLine={false} axisLine={false} minTickGap={24} />
              <YAxis tick={tick} tickLine={false} axisLine={false} tickFormatter={num} allowDecimals={false} />
              <Tooltip {...tooltipProps} cursor={{ fill: 'var(--foreground)', fillOpacity: 0.04 }} formatter={num} />
              <Legend formatter={legendText} iconType="square" iconSize={8} />
              <Bar dataKey="ok" name={t('overview.seriesOk')} stackId="r" fill="url(#bar-ok)" isAnimationActive={false} />
              <Bar dataKey="errors" name={t('overview.seriesErrors')} stackId="r" fill={error} radius={[2, 2, 0, 0]} isAnimationActive={false} />
            </BarChart>
          </ResponsiveContainer>
        </ChartCard>

        <ChartCard title={t('overview.latency')} description={t('overview.latencyHint')} empty={empty} loading={loading}>
          <ResponsiveContainer>
            <AreaChart data={series} margin={{ top: 4, right: 4, left: -16, bottom: 0 }}>
              <Fade id="lat-p95" color={slots[1]} from={0.18} />
              <Fade id="lat-p50" color={slots[0]} from={0.3} />
              {grid}
              <XAxis dataKey="label" tick={tick} tickLine={false} axisLine={false} minTickGap={24} />
              <YAxis tick={tick} tickLine={false} axisLine={false} tickFormatter={ms} />
              <Tooltip {...tooltipProps} cursor={{ stroke: 'var(--signal)', strokeOpacity: 0.4, strokeDasharray: '3 3' }} formatter={(v) => ms(v)} />
              <Legend formatter={legendText} iconType="plainline" iconSize={12} />
              <Area type="monotone" dataKey="p95Ms" name="p95" stroke={slots[1]} strokeWidth={1.5} fill="url(#lat-p95)" dot={false} activeDot={{ r: 3, strokeWidth: 0 }} isAnimationActive={false} />
              <Area type="monotone" dataKey="p50Ms" name="p50" stroke={slots[0]} strokeWidth={2} fill="url(#lat-p50)" dot={false} activeDot={{ r: 3, strokeWidth: 0 }} isAnimationActive={false} />
            </AreaChart>
          </ResponsiveContainer>
        </ChartCard>

        <ChartCard title={t('overview.tokensOverTime')} description={t('overview.tokensOverTimeHint')} empty={empty} loading={loading}>
          <ResponsiveContainer>
            <AreaChart data={series} margin={{ top: 4, right: 4, left: -16, bottom: 0 }}>
              <Fade id="tok" color={slots[2]} from={0.32} />
              {grid}
              <XAxis dataKey="label" tick={tick} tickLine={false} axisLine={false} minTickGap={24} />
              <YAxis tick={tick} tickLine={false} axisLine={false} tickFormatter={num} />
              <Tooltip {...tooltipProps} cursor={{ stroke: 'var(--signal)', strokeOpacity: 0.4, strokeDasharray: '3 3' }} formatter={num} />
              <Area type="monotone" dataKey="tokens" name={t('stats.tokens')} stroke={slots[2]} strokeWidth={2} fill="url(#tok)" dot={false} activeDot={{ r: 3, strokeWidth: 0 }} isAnimationActive={false} />
            </AreaChart>
          </ResponsiveContainer>
        </ChartCard>
      </div>

      <div className="grid gap-6 lg:grid-cols-3">
        <Card>
          <CardHeader>
            <CardTitle>{t('overview.topModels')}</CardTitle>
            <CardDescription>{t('overview.topHint')}</CardDescription>
          </CardHeader>
          <CardContent>{loading ? <LoadingBlock /> : empty ? <EmptyState title={t('overview.noTraffic')} /> : <TopList rows={top(data?.byModel)} colour={slots[0]} error={error} label={(k) => k} />}</CardContent>
        </Card>
        <Card>
          <CardHeader>
            <CardTitle>{t('overview.topAccounts')}</CardTitle>
            <CardDescription>{t('overview.topHint')}</CardDescription>
          </CardHeader>
          <CardContent>{loading ? <LoadingBlock /> : empty ? <EmptyState title={t('overview.noTraffic')} /> : <TopList rows={top(data?.byAccount)} colour={slots[1]} error={error} label={(k) => accountNames?.[k] || short(k)} />}</CardContent>
        </Card>
        <Card>
          <CardHeader>
            <CardTitle>{t('overview.endpointSplit')}</CardTitle>
            <CardDescription>{t('overview.endpointSplitHint')}</CardDescription>
          </CardHeader>
          <CardContent className="space-y-3">
            {loading ? (
              <LoadingBlock />
            ) : empty ? (
              <EmptyState title={t('overview.noTraffic')} />
            ) : (
              <>
                <StackedBar segments={endpoints.map((e, i) => [slots[i % slots.length], e.requests])} className="h-3" />
                <ul className="space-y-1.5 text-xs">
                  {endpoints.map((e, i) => (
                    <li key={e.key} className="flex items-center justify-between gap-2">
                      <span className="inline-flex items-center gap-1.5 font-mono">
                        <span className="size-2 rounded-sm" style={{ background: slots[i % slots.length] }} />
                        {e.key}
                      </span>
                      <span className="tabular-nums text-muted-foreground">
                        {formatNumber(e.requests)} · {((e.requests / Math.max(1, data?.totals.requests ?? 0)) * 100).toFixed(0)}%
                        {e.errors > 0 && <span className="ml-1.5" style={{ color: error }}>{t('overview.errCount', formatNumber(e.errors))}</span>}
                      </span>
                    </li>
                  ))}
                </ul>
              </>
            )}
          </CardContent>
        </Card>
      </div>
    </>
  )
}
