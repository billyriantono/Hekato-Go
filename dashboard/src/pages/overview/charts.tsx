// Metrics charts for the Overview page. All series colours come from PALETTE; text/grid use theme tokens.
import { useMemo } from 'react'
import {
  Area,
  AreaChart,
  Bar,
  BarChart,
  CartesianGrid,
  Legend,
  Line,
  LineChart,
  ResponsiveContainer,
  Tooltip,
  XAxis,
  YAxis,
} from 'recharts'
import { EmptyState, LoadingBlock, formatNumber } from '@/components/common'
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from '@/components/ui/card'
import { useI18n } from '@/lib/i18n'
import { useTheme } from '@/lib/theme'
import { cn } from '@/lib/utils'

export type Range = '1h' | '6h' | '24h' | '7d'
export const RANGES: Range[] = ['1h', '6h', '24h', '7d']

export type Point = { t: number; requests: number; errors: number; tokens: number; credits: number; avgLatencyMs: number; p50Ms: number; p95Ms: number }
export type Bucket = { key: string; requests: number; errors: number; tokens: number; credits: number; avgLatencyMs: number }
export type Metrics = { rangeMinutes: number; stepMinutes: number; series: Point[]; totals: Point; byModel: Bucket[]; byAccount: Bucket[]; byEndpoint: Bucket[] }

const PALETTE = {
  light: { slots: ['#2a78d6', '#eb6834', '#1baf7a'], error: '#e34948' },
  dark: { slots: ['#3987e5', '#d95926', '#199e70'], error: '#e66767' },
}
export const short = (s: string) => (s.length > 14 ? s.slice(0, 6) + '…' + s.slice(-4) : s)

const tick = { fill: 'var(--muted-foreground)', fontSize: 11 }
const tooltipProps = {
  contentStyle: { background: 'var(--popover)', border: '1px solid var(--border)', borderRadius: 'var(--radius)', color: 'var(--popover-foreground)', fontSize: 12 },
  labelStyle: { color: 'var(--muted-foreground)' },
  itemStyle: { color: 'var(--popover-foreground)' },
}
const legendText = (v: string) => <span style={{ color: 'var(--muted-foreground)', fontSize: 11 }}>{v}</span>
const grid = <CartesianGrid vertical={false} strokeDasharray="3 3" stroke="var(--border)" />

/** Horizontal stacked bar; segments are [colour (css class or hex), value]. */
export function StackedBar({ segments, className }: { segments: [string, number][]; className?: string }) {
  const total = segments.reduce((n, [, v]) => n + v, 0) || 1
  return (
    <div className={cn('flex h-2 w-full overflow-hidden rounded-full bg-muted', className)}>
      {segments.map(([c, v], i) =>
        v > 0 ? <div key={i} className={c.startsWith('#') ? undefined : c} style={{ width: `${(v / total) * 100}%`, background: c.startsWith('#') ? c : undefined }} /> : null,
      )}
    </div>
  )
}

function ChartCard({ title, description, empty, loading, children }: { title: string; description: string; empty: boolean; loading: boolean; children: React.ReactNode }) {
  const { t } = useI18n()
  return (
    <Card>
      <CardHeader>
        <CardTitle>{title}</CardTitle>
        <CardDescription>{description}</CardDescription>
      </CardHeader>
      <CardContent>{loading ? <LoadingBlock /> : empty ? <EmptyState title={t('overview.noTraffic')} /> : <div className="h-[220px]">{children}</div>}</CardContent>
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
            <span className="shrink-0 tabular-nums text-muted-foreground">
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

export function MetricsCharts({ data, range, loading }: { data?: Metrics; range: Range; loading: boolean }) {
  const { t } = useI18n()
  const { theme } = useTheme()
  const { slots, error } = PALETTE[theme]
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
            <BarChart data={series} margin={{ top: 4, right: 4, left: -16, bottom: 0 }}>
              {grid}
              <XAxis dataKey="label" tick={tick} tickLine={false} axisLine={false} minTickGap={24} />
              <YAxis tick={tick} tickLine={false} axisLine={false} tickFormatter={num} allowDecimals={false} />
              <Tooltip {...tooltipProps} cursor={{ fill: 'var(--muted)' }} formatter={num} />
              <Legend formatter={legendText} iconType="square" iconSize={8} />
              <Bar dataKey="ok" name={t('overview.seriesOk')} stackId="r" fill={slots[0]} radius={[2, 2, 0, 0]} isAnimationActive={false} />
              <Bar dataKey="errors" name={t('overview.seriesErrors')} stackId="r" fill={error} radius={[2, 2, 0, 0]} isAnimationActive={false} />
            </BarChart>
          </ResponsiveContainer>
        </ChartCard>

        <ChartCard title={t('overview.latency')} description={t('overview.latencyHint')} empty={empty} loading={loading}>
          <ResponsiveContainer>
            <LineChart data={series} margin={{ top: 4, right: 4, left: -16, bottom: 0 }}>
              {grid}
              <XAxis dataKey="label" tick={tick} tickLine={false} axisLine={false} minTickGap={24} />
              <YAxis tick={tick} tickLine={false} axisLine={false} tickFormatter={(v) => `${num(v)}ms`} />
              <Tooltip {...tooltipProps} cursor={{ stroke: 'var(--border)' }} formatter={(v) => `${num(v)} ms`} />
              <Legend formatter={legendText} iconType="plainline" iconSize={12} />
              <Line type="monotone" dataKey="p50Ms" name="p50" stroke={slots[0]} strokeWidth={2} dot={false} activeDot={{ r: 3 }} isAnimationActive={false} />
              <Line type="monotone" dataKey="p95Ms" name="p95" stroke={slots[1]} strokeWidth={2} dot={false} activeDot={{ r: 3 }} isAnimationActive={false} />
            </LineChart>
          </ResponsiveContainer>
        </ChartCard>

        <ChartCard title={t('overview.tokensOverTime')} description={t('overview.tokensOverTimeHint')} empty={empty} loading={loading}>
          <ResponsiveContainer>
            <AreaChart data={series} margin={{ top: 4, right: 4, left: -16, bottom: 0 }}>
              {grid}
              <XAxis dataKey="label" tick={tick} tickLine={false} axisLine={false} minTickGap={24} />
              <YAxis tick={tick} tickLine={false} axisLine={false} tickFormatter={num} />
              <Tooltip {...tooltipProps} cursor={{ stroke: 'var(--border)' }} formatter={num} />
              <Area type="monotone" dataKey="tokens" name={t('stats.tokens')} stroke={slots[2]} strokeWidth={2} fill={slots[2]} fillOpacity={0.15} dot={false} activeDot={{ r: 3 }} isAnimationActive={false} />
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
          <CardContent>{loading ? <LoadingBlock /> : empty ? <EmptyState title={t('overview.noTraffic')} /> : <TopList rows={top(data?.byAccount)} colour={slots[1]} error={error} label={short} />}</CardContent>
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
