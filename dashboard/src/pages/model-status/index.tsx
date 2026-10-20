// Live health of every model the auto-route tiers cover: last-24h success,
// volume, median TTFT and output speed per pool/model, plus a 60-day strip.
// Served from /admin/api/model-health; built from real traffic only.
import { useQuery } from '@tanstack/react-query'
import { EmptyState, LoadingBlock, PageHeader, StatCard, errorMessage, formatNumber, formatTime } from '@/components/common'
import { Badge } from '@/components/ui/badge'
import { Card, CardContent } from '@/components/ui/card'
import { Tooltip, TooltipContent, TooltipTrigger } from '@/components/ui/tooltip'
import { get } from '@/lib/api'
import { useI18n } from '@/lib/i18n'
import { RouterActivity } from './decisions'

type Status = 'operational' | 'degraded' | 'outage' | 'none'
type Route = {
  route: string
  pool: string
  model: string
  tiers: string[]
  accounts: number
  status: Status
  requests: number
  errors: number
  ttftP50: number
  tps: number
  context: number
  maxOutput: number
  slow?: boolean
  quarantined?: boolean
  days: Status[]
}
type Day = { day: number; status: Status; requests: number; errors: number }
type Health = { updated: number; status: Status; routes: Route[]; days: Day[] }

const DOT: Record<Status, string> = {
  operational: 'bg-success',
  degraded: 'bg-warning',
  outage: 'bg-destructive',
  none: 'bg-muted-foreground/25',
}
const INK: Record<Status, string> = {
  operational: 'text-success-ink',
  degraded: 'text-warning-ink',
  outage: 'text-destructive',
  none: 'text-muted-foreground',
}
const TONE: Record<Status, 'success' | 'warning' | 'danger' | 'default'> = {
  operational: 'success',
  degraded: 'warning',
  outage: 'danger',
  none: 'default',
}

const pct = (r: Route) => (r.requests ? `${(((r.requests - r.errors) / r.requests) * 100).toFixed(1)}%` : '—')
const ms = (v: number) => (!v ? '—' : v >= 10_000 ? `${(v / 1000).toFixed(1)} s` : `${formatNumber(v)} ms`)
const day = (unix: number) => new Date(unix * 1000).toLocaleDateString(undefined, { month: 'short', day: 'numeric' })

export function ModelStatusPage() {
  const { t } = useI18n()
  const q = useQuery({ queryKey: ['model-health'], queryFn: () => get<Health>('/model-health'), refetchInterval: 30_000 })
  const d = q.data
  const routes = d?.routes ?? []
  const label = (s: Status) => t(`modelStatus.status.${s}`)

  return (
    <div>
      <PageHeader title={t('modelStatus.title')} description={t('modelStatus.description')} />

      {q.isLoading ? (
        <LoadingBlock />
      ) : q.isError || !d ? (
        <EmptyState title={t('common.failed')} description={errorMessage(q.error)} />
      ) : (
        <div className="space-y-6">
          <div className="grid grid-cols-2 gap-3 md:grid-cols-4">
            <StatCard label={t('modelStatus.poolStatus')} value={label(d.status)} tone={TONE[d.status]} />
            <StatCard label={t('modelStatus.routes')} value={formatNumber(routes.length)} hint={t('modelStatus.routesHint')} />
            <StatCard label={t('modelStatus.requests24h')} value={formatNumber(routes.reduce((n, r) => n + r.requests, 0))} />
            <StatCard label={t('modelStatus.updated')} value={<span className="text-sm">{formatTime(d.updated)}</span>} hint={t('modelStatus.refresh')} />
          </div>

          <Card>
            <CardContent className="space-y-2">
              <div className="flex items-center justify-between text-[11px] uppercase tracking-wider text-muted-foreground">
                <span>{t('modelStatus.history')}</span>
                <span>{t('modelStatus.dailySamples')}</span>
              </div>
              <Strip days={d.days.map((x) => ({ status: x.status, tip: `${day(x.day)} · ${label(x.status)}${x.requests ? ` · ${formatNumber(x.requests - x.errors)}/${formatNumber(x.requests)}` : ''}` }))} tall />
              <div className="flex justify-between text-[11px] text-muted-foreground">
                <span>{d.days.length ? day(d.days[0].day) : ''}</span>
                <span>{t('modelStatus.today')}</span>
              </div>
              <Legend label={label} />
            </CardContent>
          </Card>

          {routes.length === 0 ? (
            <EmptyState title={t('modelStatus.empty')} description={t('modelStatus.emptyHint')} />
          ) : (
            <div className="grid gap-3 sm:grid-cols-2 xl:grid-cols-4">
              {routes.map((r) => (
                <Card key={r.route} size="sm">
                  <CardContent className="space-y-3">
                    <div className="flex items-start justify-between gap-2">
                      <div className="min-w-0">
                        <div className="text-[11px] text-muted-foreground">{r.pool}</div>
                        <div className="font-mono text-xs font-medium [overflow-wrap:anywhere]">{r.model}</div>
                        <div className="mt-1 flex flex-wrap gap-1">
                          {r.tiers.map((tier) => (
                            <Badge key={tier} variant="outline" className="text-[10px]">
                              {t(`settings.autoRoute.tier.${tier}`)}
                            </Badge>
                          ))}
                          {r.slow && (
                            <Badge variant="secondary" className="text-[10px]">
                              {t('overview.slow')}
                            </Badge>
                          )}
                          {r.quarantined && (
                            <Badge variant="destructive" className="text-[10px]">
                              {t('overview.quarantined')}
                            </Badge>
                          )}
                        </div>
                      </div>
                      <span className={`flex shrink-0 items-center gap-1.5 text-[10px] uppercase tracking-wider ${INK[r.status]}`}>
                        <span className={`size-1.5 rounded-full ${DOT[r.status]}`} />
                        {label(r.status)}
                      </span>
                    </div>
                    <dl className="grid grid-cols-2 gap-x-3 gap-y-2">
                      <Metric label={t('modelStatus.success')} value={<span className={INK[r.status]}>{pct(r)}</span>} />
                      <Metric label={t('modelStatus.requests')} value={formatNumber(r.requests)} />
                      <Metric label={t('modelStatus.ttft')} value={ms(r.ttftP50)} />
                      <Metric label={t('modelStatus.speed')} value={r.tps ? `${r.tps} TPS` : '—'} />
                    </dl>
                    <Strip days={r.days.slice(-14).map((s) => ({ status: s, tip: label(s) }))} />
                    <div className="flex justify-between border-t pt-2 text-[11px] text-muted-foreground">
                      <span>{r.context ? `${formatNumber(r.context)} ${t('modelStatus.context')}` : '—'}</span>
                      <span>{r.maxOutput ? `${formatNumber(r.maxOutput)} ${t('modelStatus.maxOutput')}` : ''}</span>
                    </div>
                  </CardContent>
                </Card>
              ))}
            </div>
          )}
          <p className="text-xs text-muted-foreground">{t('modelStatus.legendHint')}</p>
          <RouterActivity />
        </div>
      )}
    </div>
  )
}

function Metric({ label, value }: { label: string; value: React.ReactNode }) {
  return (
    <div>
      <dt className="text-[10px] uppercase tracking-wider text-muted-foreground">{label}</dt>
      <dd className="font-mono text-sm tabular-nums">{value}</dd>
    </div>
  )
}

function Strip({ days, tall }: { days: { status: Status; tip: string }[]; tall?: boolean }) {
  return (
    <div className="flex gap-0.5">
      {days.map((x, i) => (
        <Tooltip key={i}>
          <TooltipTrigger render={<span className={`${tall ? 'h-5' : 'h-2'} min-w-0 flex-1 rounded-[2px] ${DOT[x.status]}`} />} />
          <TooltipContent>{x.tip}</TooltipContent>
        </Tooltip>
      ))}
    </div>
  )
}

function Legend({ label }: { label: (s: Status) => string }) {
  return (
    <div className="flex flex-wrap gap-x-4 gap-y-1 pt-1 text-[11px] text-muted-foreground">
      {(['operational', 'degraded', 'outage', 'none'] as Status[]).map((s) => (
        <span key={s} className="flex items-center gap-1.5">
          <span className={`size-2 rounded-[2px] ${DOT[s]}`} />
          {label(s)}
        </span>
      ))}
    </div>
  )
}
