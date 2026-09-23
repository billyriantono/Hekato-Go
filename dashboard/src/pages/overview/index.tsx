import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query'
import { Link } from '@tanstack/react-router'
import type { ColumnDef } from '@tanstack/react-table'
import { useMemo, useState } from 'react'
import {
  LuActivity,
  LuArrowRight,
  LuCircleCheck,
  LuCircleX,
  LuClock,
  LuCoins,
  LuCpu,
  LuGauge,
  LuLoader,
  LuRefreshCw,
  LuRotateCcw,
  LuTimer,
  LuUsers,
} from 'react-icons/lu'
import { toast } from 'sonner'
import {
  ConfirmDialog,
  CopyButton,
  EmptyState,
  LoadingBlock,
  PageHeader,
  StatCard,
  StatusDot,
  errorMessage,
  formatNumber,
  formatTime,
} from '@/components/common'
import { DataTable } from '@/components/data-table'
import { Button } from '@/components/ui/button'
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from '@/components/ui/card'
import { Tabs, TabsList, TabsTrigger } from '@/components/ui/tabs'
import { Tooltip, TooltipContent, TooltipTrigger } from '@/components/ui/tooltip'
import { get, post } from '@/lib/api'
import { useI18n } from '@/lib/i18n'
import { AutoRouteCard } from './auto-route'
import { LiveRoutingMap } from './routing-map'
import { WarmupCard } from './warmup'
import { MetricsCharts, RANGES, StackedBar, short, type Metrics, type Range } from './charts'

type Status = {
  version?: string
  storage?: string
  accounts?: number
  available?: number
  totalRequests?: number
  successRequests?: number
  failedRequests?: number
  totalTokens?: number
  totalCredits?: number
  uptime?: number
}
type Account = {
  id: string
  email: string
  nickname: string
  provider: string
  enabled: boolean
  banStatus: string
  expiresAt: number
  usageCurrent: number
  usageLimit: number
}
type RequestLog = {
  time: number
  endpoint: string
  model: string
  accountId: string
  status: 'success' | 'error'
  error: string
  tokens: number
  credits: number
  duration: number
}

const RECENT = 15
const REFETCH = 15000

function formatUptime(s = 0) {
  const d = Math.floor(s / 86400)
  const h = Math.floor((s % 86400) / 3600)
  const m = Math.floor((s % 3600) / 60)
  return d ? `${d}d ${h}h` : h ? `${h}h ${m}m` : `${m}m`
}

const isBanned = (a: Account) => !!a.banStatus && a.banStatus !== 'ACTIVE'
const isOverQuota = (a: Account) => a.usageLimit > 0 && a.usageCurrent >= a.usageLimit

export function OverviewPage() {
  const { t } = useI18n()
  const qc = useQueryClient()
  const [resetOpen, setResetOpen] = useState(false)
  const [range, setRange] = useState<Range>('1h')

  const status = useQuery({ queryKey: ['status'], queryFn: () => get<Status>('/status'), refetchInterval: REFETCH })
  const accounts = useQuery({ queryKey: ['accounts'], queryFn: () => get<Account[]>('/accounts'), refetchInterval: REFETCH })
  const metrics = useQuery({ queryKey: ['metrics', range], queryFn: () => get<Metrics>(`/metrics?range=${range}`), refetchInterval: REFETCH })
  const logs = useQuery({
    queryKey: ['logs'],
    queryFn: () => get<{ logs: RequestLog[] }>('/logs'),
    refetchInterval: REFETCH,
    select: (d) => (d.logs ?? []).slice().sort((a, b) => b.time - a.time).slice(0, RECENT),
  })

  const accountNames = useMemo(() => {
    const map: Record<string, string> = {}
    for (const a of accounts.data ?? []) {
      map[a.id] = a.nickname || a.email || a.id
    }
    return map
  }, [accounts.data])

  const resetStats = useMutation({
    mutationFn: () => post('/stats/reset'),
    onSuccess: () => {
      toast.success(t('overview.resetStatsDone'))
      qc.invalidateQueries({ queryKey: ['status'] })
      qc.invalidateQueries({ queryKey: ['stats'] })
      qc.invalidateQueries({ queryKey: ['metrics'] })
    },
    onError: (e) => toast.error(errorMessage(e)),
  })
  const refreshModels = useMutation({
    mutationFn: () => post<{ refreshed?: number }>('/accounts/models/refresh'),
    onSuccess: (d) => toast.success(t('overview.refreshModelsDone', d?.refreshed ?? 0)),
    onError: (e) => toast.error(errorMessage(e)),
  })

  const s = status.data
  const total = s?.totalRequests ?? 0
  const ok = s?.successRequests ?? 0
  const failed = s?.failedRequests ?? 0
  const rate = total ? (ok / total) * 100 : 100
  const m = metrics.data?.totals
  const mReq = m?.requests ?? 0
  const mErrRate = mReq ? ((m?.errors ?? 0) / mReq) * 100 : 0

  const pool = useMemo(() => {
    const list = accounts.data ?? []
    const now = Date.now() / 1000
    const byProvider = new Map<string, { ok: number; bad: number }>()
    for (const a of list) {
      const p = byProvider.get(a.provider || '—') ?? { ok: 0, bad: 0 }
      if (a.enabled && !isBanned(a) && !isOverQuota(a)) p.ok++
      else p.bad++
      byProvider.set(a.provider || '—', p)
    }
    return {
      enabled: list.filter((a) => a.enabled).length,
      disabled: list.filter((a) => !a.enabled).length,
      banned: list.filter(isBanned).length,
      overQuota: list.filter(isOverQuota).length,
      expiring: list.filter((a) => a.expiresAt > 0 && a.expiresAt - now < 3600).length,
      byProvider: [...byProvider.entries()].sort((a, b) => b[1].ok + b[1].bad - (a[1].ok + a[1].bad)),
      attention: list.filter((a) => !a.enabled || isBanned(a) || isOverQuota(a)),
    }
  }, [accounts.data])

  const logColumns = useMemo<ColumnDef<RequestLog, unknown>[]>(
    () => [
      { accessorKey: 'time', header: t('logs.time'), cell: ({ getValue }) => <span className="whitespace-nowrap text-xs">{formatTime(getValue<number>())}</span> },
      { accessorKey: 'endpoint', header: t('logs.endpoint'), cell: ({ getValue }) => <span className="font-mono text-xs">{getValue<string>()}</span> },
      { accessorKey: 'model', header: t('logs.model'), cell: ({ getValue }) => <span className="font-mono text-xs">{getValue<string>()}</span> },
      { accessorKey: 'accountId', header: t('logs.account'), cell: ({ getValue }) => { const id = getValue<string>() ?? ''; return <span className="font-mono text-xs" title={id}>{accountNames[id] || short(id)}</span> } },
      { accessorKey: 'tokens', header: t('logs.tokens'), cell: ({ getValue }) => <span className="tabular-nums">{formatNumber(getValue<number>())}</span> },
      { accessorKey: 'credits', header: t('overview.credits'), cell: ({ getValue }) => <span className="tabular-nums">{(getValue<number>() ?? 0).toFixed(2)}</span> },
      { accessorKey: 'duration', header: t('logs.duration'), cell: ({ getValue }) => <span className="tabular-nums">{getValue<number>()}ms</span> },
      {
        accessorKey: 'status',
        header: t('logs.status'),
        cell: ({ row }) => {
          const r = row.original
          if (r.status === 'success') return <StatusDot tone="success" label={t('logs.statusSuccess')} />
          return (
            <Tooltip>
              <TooltipTrigger render={<span className="cursor-help" />}>
                <StatusDot tone="danger" label={t('logs.statusError')} />
              </TooltipTrigger>
              <TooltipContent className="max-w-xs break-words">{r.error || r.status}</TooltipContent>
            </Tooltip>
          )
        },
      },
    ],
    [t, accountNames],
  )

  const origin = window.location.origin
  const curls = [
    {
      path: '/v1/messages',
      cmd: `curl ${origin}/v1/messages \\\n  -H "Content-Type: application/json" \\\n  -H "anthropic-version: 2023-06-01" \\\n  -d '{"model":"claude-sonnet-4.5","max_tokens":1024,"messages":[{"role":"user","content":"Hello!"}]}'`,
    },
    {
      path: '/v1/chat/completions',
      cmd: `curl ${origin}/v1/chat/completions \\\n  -H "Content-Type: application/json" \\\n  -H "Authorization: Bearer any" \\\n  -d '{"model":"claude-sonnet-4.5","messages":[{"role":"user","content":"Hello!"}]}'`,
    },
  ]

  if (status.isError) {
    return (
      <>
        <PageHeader title={t('overview.title')} description={t('overview.subtitle')} />
        <EmptyState
          title={t('overview.loadFailed', errorMessage(status.error))}
          action={<Button variant="outline" onClick={() => status.refetch()}>{t('common.refresh')}</Button>}
        />
      </>
    )
  }

  return (
    <div className="space-y-6">
      <PageHeader
        title={t('overview.title')}
        description={t('overview.subtitle')}
        actions={
          <>
            <Button variant="outline" size="sm" onClick={() => refreshModels.mutate()} disabled={refreshModels.isPending}>
              {refreshModels.isPending ? <LuLoader className="size-4 animate-spin" /> : <LuRefreshCw className="size-4" />}
              {t('overview.refreshModels')}
            </Button>
            <Button variant="outline" size="sm" onClick={() => setResetOpen(true)}>
              <LuRotateCcw className="size-4" />
              {t('overview.resetStats')}
            </Button>
          </>
        }
      />

      {/* Range selector */}
      <div className="flex items-center justify-between gap-3">
        <span className="text-xs font-medium text-muted-foreground">{t('overview.range')}</span>
        <Tabs value={range} onValueChange={(v) => setRange(v as Range)}>
          <TabsList>
            {RANGES.map((r) => (
              <TabsTrigger key={r} value={r} className="px-3">{r}</TabsTrigger>
            ))}
          </TabsList>
        </Tabs>
      </div>

      {/* KPI row: lifetime counters */}
      <div className="grid grid-cols-2 gap-3 md:grid-cols-4 xl:grid-cols-7">
        <StatCard label={t('stats.requests')} value={formatNumber(total)} icon={<LuActivity className="size-4" />} />
        <StatCard
          label={t('overview.successRate')}
          value={`${rate.toFixed(1)}%`}
          tone={rate >= 95 ? 'success' : rate >= 80 ? 'warning' : 'danger'}
          hint={<StackedBar segments={[['bg-emerald-500', ok], ['bg-red-500', failed]]} className="mt-2 h-1.5" />}
          icon={<LuCircleCheck className="size-4" />}
        />
        <StatCard label={t('stats.failed')} value={formatNumber(failed)} tone={failed ? 'danger' : 'default'} icon={<LuCircleX className="size-4" />} />
        <StatCard label={t('stats.tokens')} value={formatNumber(s?.totalTokens)} icon={<LuCpu className="size-4" />} />
        <StatCard label={t('stats.credits')} value={(s?.totalCredits ?? 0).toFixed(2)} icon={<LuCoins className="size-4" />} />
        <StatCard label={t('overview.uptime')} value={formatUptime(s?.uptime)} icon={<LuClock className="size-4" />} />
        <StatCard
          label={t('stats.accounts')}
          value={t('overview.ofTotal', s?.available ?? 0, s?.accounts ?? 0)}
          tone={(s?.accounts ?? 0) > 0 && (s?.available ?? 0) === 0 ? 'danger' : 'default'}
          hint={t('overview.availableOf', s?.available ?? 0, s?.accounts ?? 0)}
          icon={<LuUsers className="size-4" />}
        />
      </div>

      <LiveRoutingMap />

      {/* KPI row: selected range */}
      <div className="grid grid-cols-2 gap-3 md:grid-cols-4">
        <StatCard label={t('overview.requestsInRange', range)} value={formatNumber(mReq)} icon={<LuActivity className="size-4" />} />
        <StatCard
          label={t('overview.errorRateInRange', range)}
          value={`${mErrRate.toFixed(1)}%`}
          tone={!mReq ? 'default' : mErrRate <= 5 ? 'success' : mErrRate <= 20 ? 'warning' : 'danger'}
          hint={t('overview.errCount', formatNumber(m?.errors))}
          icon={<LuGauge className="size-4" />}
        />
        <StatCard label={t('overview.latencyInRange')} value={`${formatNumber(m?.p50Ms)} / ${formatNumber(m?.p95Ms)} ms`} hint="p50 / p95" icon={<LuTimer className="size-4" />} />
        <StatCard label={t('overview.tokensInRange', range)} value={formatNumber(m?.tokens)} hint={t('overview.creditsHint', (m?.credits ?? 0).toFixed(2))} icon={<LuCpu className="size-4" />} />
      </div>

      <MetricsCharts data={metrics.data} range={range} loading={metrics.isPending} accountNames={accountNames} />

      <WarmupCard />

      <AutoRouteCard />

      <div className="grid gap-6 lg:grid-cols-3">
        {/* Pool health */}
        <Card className="lg:col-span-1">
          <CardHeader>
            <CardTitle>{t('overview.poolHealth')}</CardTitle>
            <CardDescription>{t('overview.poolHealthHint')}</CardDescription>
          </CardHeader>
          <CardContent className="space-y-5">
            {accounts.isPending ? (
              <LoadingBlock />
            ) : !accounts.data?.length ? (
              <EmptyState title={t('overview.noAccounts')} action={<Link to="/accounts"><Button variant="outline" size="sm">{t('overview.viewAccounts')}</Button></Link>} />
            ) : (
              <>
                <div className="grid grid-cols-2 gap-x-4 gap-y-2 text-sm">
                  {(
                    [
                      ['overview.enabled', pool.enabled, 'success'],
                      ['overview.disabled', pool.disabled, 'muted'],
                      ['overview.banned', pool.banned, 'danger'],
                      ['overview.overQuota', pool.overQuota, 'warning'],
                      ['overview.expiringSoon', pool.expiring, 'warning'],
                    ] as const
                  ).map(([k, n, tone]) => (
                    <div key={k} className="flex items-center justify-between gap-2">
                      <StatusDot tone={n ? tone : 'muted'} label={t(k)} />
                      <span className="font-medium tabular-nums">{n}</span>
                    </div>
                  ))}
                </div>

                <div className="space-y-2">
                  <div className="text-xs font-medium text-muted-foreground">{t('overview.byProvider')}</div>
                  {pool.byProvider.map(([p, c]) => (
                    <div key={p} className="space-y-1">
                      <div className="flex justify-between text-xs">
                        <span>{p}</span>
                        <span className="tabular-nums text-muted-foreground">{c.ok + c.bad}</span>
                      </div>
                      <StackedBar segments={[['bg-emerald-500', c.ok], ['bg-red-400', c.bad]]} />
                    </div>
                  ))}
                </div>

                <div className="space-y-2">
                  <div className="text-xs font-medium text-muted-foreground">{t('overview.needsAttention')}</div>
                  {pool.attention.length === 0 ? (
                    <div className="text-xs text-muted-foreground">{t('overview.allHealthy')}</div>
                  ) : (
                    <ul className="space-y-1 text-xs">
                      {pool.attention.slice(0, 8).map((a) => (
                        <li key={a.id} className="flex items-center justify-between gap-2">
                          <span className="truncate">{a.nickname || a.email || short(a.id)}</span>
                          <StatusDot
                            tone={isBanned(a) ? 'danger' : isOverQuota(a) ? 'warning' : 'muted'}
                            label={t(isBanned(a) ? 'overview.banned' : isOverQuota(a) ? 'overview.overQuota' : 'overview.disabled')}
                          />
                        </li>
                      ))}
                    </ul>
                  )}
                  <Link to="/accounts" className="inline-flex items-center gap-1 text-xs text-primary hover:underline">
                    {t('overview.viewAccounts')} <LuArrowRight className="size-3" />
                  </Link>
                </div>
              </>
            )}
          </CardContent>
        </Card>

        {/* Recent activity */}
        <Card className="lg:col-span-2">
          <CardHeader className="flex flex-row items-start justify-between">
            <div>
              <CardTitle>{t('overview.recentActivity')}</CardTitle>
              <CardDescription>{t('overview.recentActivityHint', RECENT)}</CardDescription>
            </div>
            <Link to="/logs" className="inline-flex items-center gap-1 text-xs text-primary hover:underline">
              {t('overview.viewLogs')} <LuArrowRight className="size-3" />
            </Link>
          </CardHeader>
          <CardContent>
            {logs.isPending ? (
              <LoadingBlock />
            ) : (
              <DataTable data={logs.data ?? []} columns={logColumns} emptyState={<EmptyState title={t('logs.empty')} />} />
            )}
          </CardContent>
        </Card>
      </div>
      {/* Endpoints */}
      <Card>
        <CardHeader>
          <CardTitle>{t('overview.endpoints')}</CardTitle>
          <CardDescription>
            {t('overview.endpointsHint')} <code className="rounded bg-muted px-1 py-0.5 text-xs">{origin}</code>
          </CardDescription>
        </CardHeader>
        <CardContent className="grid gap-3 md:grid-cols-2">
          {curls.map((c) => (
            <div key={c.path} className="rounded-lg border bg-muted/40">
              <div className="flex items-center justify-between border-b px-3 py-1.5">
                <span className="font-mono text-xs font-medium">POST {c.path}</span>
                <CopyButton value={c.cmd} />
              </div>
              <pre className="overflow-x-auto p-3 font-mono text-[11px] leading-relaxed whitespace-pre">{c.cmd}</pre>
            </div>
          ))}
        </CardContent>
      </Card>

      {s?.storage && (
        <div className="text-xs text-muted-foreground">
          {t('overview.storage')}: {s.storage}
        </div>
      )}

      <ConfirmDialog
        open={resetOpen}
        onOpenChange={setResetOpen}
        title={t('overview.resetStats')}
        description={t('overview.resetStatsConfirm')}
        destructive
        onConfirm={() => resetStats.mutateAsync().then(() => undefined)}
      />
    </div>
  )
}
