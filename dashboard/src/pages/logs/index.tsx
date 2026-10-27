import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query'
import type { ColumnDef } from '@tanstack/react-table'
import { useMemo, useState } from 'react'
import { LuRefreshCw, LuTrash2 } from 'react-icons/lu'
import { toast } from 'sonner'
import { ConfirmDialog, EmptyState, LoadingBlock, PageHeader, StatCard, StatusDot, errorMessage, formatNumber, formatTime } from '@/components/common'
import { DataTable } from '@/components/data-table'
import { Badge } from '@/components/ui/badge'
import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from '@/components/ui/select'
import { Switch } from '@/components/ui/switch'
import { del, get } from '@/lib/api'
import { useI18n } from '@/lib/i18n'

type RequestLog = {
  time: number
  endpoint: string
  model: string
  accountId: string
  status: 'success' | 'error'
  error: string
  errorType: string
  tokens: number
  outputTokens?: number
  cacheReadTokens?: number
  cacheWriteTokens?: number
  credits: number
  duration: number
  ttftMs?: number
  tps?: number
  userAgent?: string
  clientIp?: string
}

const ENDPOINTS = ['claude', 'openai', 'responses']

export function LogsPage() {
  const { t } = useI18n()
  const qc = useQueryClient()
  const [auto, setAuto] = useState(false)
  const [endpoint, setEndpoint] = useState('all')
  const [status, setStatus] = useState('all')
  const [search, setSearch] = useState('')
  const [confirmClear, setConfirmClear] = useState(false)

  const logsQ = useQuery({
    queryKey: ['logs'],
    queryFn: () => get<{ logs: RequestLog[] }>('/logs'),
    refetchInterval: auto ? 5000 : false,
  })
  const clearM = useMutation({
    mutationFn: () => del('/logs'),
    onSuccess: () => {
      toast.success(t('logs.cleared'))
      qc.invalidateQueries({ queryKey: ['logs'] })
    },
    onError: (e) => toast.error(errorMessage(e, t('common.unknownError'))),
  })

  const rows = useMemo(() => {
    const q = search.trim().toLowerCase()
    return [...(logsQ.data?.logs ?? [])]
      .sort((a, b) => b.time - a.time) // newest first regardless of API order
      .filter(
        (l) =>
          (endpoint === 'all' || l.endpoint === endpoint) &&
          (status === 'all' || l.status === status) &&
          (!q || [l.model, l.accountId, l.error, l.errorType, l.userAgent, l.clientIp].some((s) => s?.toLowerCase().includes(q))),
      )
  }, [logsQ.data, endpoint, status, search])

  const total = rows.length
  const ok = rows.filter((l) => l.status === 'success').length
  const sum = (f: (l: RequestLog) => number) => rows.reduce((a, l) => a + (f(l) || 0), 0)

  const columns: ColumnDef<RequestLog, unknown>[] = [
    { accessorKey: 'time', header: t('logs.time'), cell: ({ getValue }) => <span className="text-xs whitespace-nowrap tabular-nums">{formatTime(getValue() as number)}</span> },
    { accessorKey: 'endpoint', header: t('logs.endpoint'), cell: ({ getValue }) => <Badge variant="outline">{String(getValue())}</Badge> },
    { accessorKey: 'model', header: t('logs.model'), cell: ({ getValue }) => <span className="text-xs">{String(getValue() ?? '—')}</span> },
    {
      accessorKey: 'accountId',
      header: t('logs.account'),
      cell: ({ getValue }) => {
        const id = String(getValue() ?? '')
        return (
          <code className="text-xs" title={id}>
            {id.length > 10 ? id.slice(0, 8) + '…' : id || '—'}
          </code>
        )
      },
    },
    { accessorKey: 'tokens', header: t('logs.tokens'), cell: ({ getValue }) => <span className="text-xs tabular-nums">{formatNumber(getValue() as number)}</span> },
    {
      accessorKey: 'cacheReadTokens',
      header: t('logs.cacheRead'),
      cell: ({ row }) => {
        const read = row.original.cacheReadTokens ?? 0
        const write = row.original.cacheWriteTokens ?? 0
        if (!read && !write) return <span className="text-xs text-muted-foreground">—</span>
        return (
          <span className="text-xs tabular-nums" title={`${read} read / ${write} written`}>
            {formatNumber(read)}
          </span>
        )
      },
    },
    { accessorKey: 'credits', header: t('logs.credits'), cell: ({ getValue }) => <span className="text-xs tabular-nums">{((getValue() as number) || 0).toFixed(2)}</span> },
    { accessorKey: 'duration', header: t('logs.duration'), cell: ({ getValue }) => <span className="text-xs tabular-nums">{formatNumber(getValue() as number)} ms</span> },
    {
      accessorKey: 'ttftMs',
      header: () => <span title="Time to first token">TTFT</span>,
      cell: ({ row }) => {
        const v = row.original.ttftMs
        return <span className="text-xs tabular-nums">{v && v > 0 ? `${formatNumber(v)} ms` : '—'}</span>
      },
    },
    {
      accessorKey: 'tps',
      header: () => <span title="Output tokens per second">TPS</span>,
      cell: ({ row }) => {
        const v = row.original.tps
        return <span className="text-xs tabular-nums">{v && v > 0 ? v.toFixed(1) : '—'}</span>
      },
    },
    {
      accessorKey: 'userAgent',
      header: t('logs.userAgent'),
      cell: ({ getValue }) => {
        const v = String(getValue() ?? '')
        return (
          <span className="block max-w-[14rem] truncate text-xs text-muted-foreground" title={v || '—'}>
            {v || '—'}
          </span>
        )
      },
    },
    {
      accessorKey: 'clientIp',
      header: t('logs.clientIp'),
      cell: ({ getValue }) => <span className="font-mono text-xs tabular-nums">{String(getValue() ?? '') || '—'}</span>,
    },
    {
      accessorKey: 'status',
      header: t('logs.status'),
      cell: ({ row }) => {
        const l = row.original
        const okRow = l.status === 'success'
        return (
          <div className="min-w-0">
            <StatusDot tone={okRow ? 'success' : 'danger'} label={t(okRow ? 'logs.statusSuccess' : 'logs.statusError')} />
            {!okRow && (l.error || l.errorType) && (
              <div className="max-w-xs truncate text-xs text-destructive" title={[l.errorType, l.error].filter(Boolean).join(': ')}>
                {l.errorType && <span className="font-medium">{l.errorType}: </span>}
                {l.error}
              </div>
            )}
          </div>
        )
      },
    },
  ]

  return (
    <div>
      <PageHeader
        title={t('logs.title')}
        description={t('logs.pageDescription')}
        actions={
          <>
            <label className="flex items-center gap-2 text-sm">
              <Switch checked={auto} onCheckedChange={setAuto} /> {t('logs.autoRefresh')}
            </label>
            <Button variant="outline" onClick={() => logsQ.refetch()} disabled={logsQ.isFetching}>
              <LuRefreshCw className={logsQ.isFetching ? 'animate-spin' : ''} /> {t('logs.refresh')}
            </Button>
            <Button variant="destructive" onClick={() => setConfirmClear(true)} disabled={!logsQ.data?.logs?.length}>
              <LuTrash2 /> {t('logs.clear')}
            </Button>
          </>
        }
      />

      <div className="mb-4 grid grid-cols-2 gap-3 md:grid-cols-4">
        <StatCard label={t('logs.total')} value={formatNumber(total)} hint={`${t('logs.errors')}: ${formatNumber(total - ok)}`} />
        <StatCard label={t('logs.successRate')} value={total ? `${((ok / total) * 100).toFixed(1)}%` : '—'} tone={total && ok < total ? 'warning' : 'success'} />
        <StatCard label={t('logs.avgDuration')} value={total ? `${formatNumber(Math.round(sum((l) => l.duration) / total))} ms` : '—'} />
        <StatCard label={t('logs.totalTokens')} value={formatNumber(sum((l) => l.tokens))} hint={`${t('logs.credits')}: ${sum((l) => l.credits).toFixed(2)}`} />
      </div>

      {logsQ.isLoading ? (
        <LoadingBlock />
      ) : logsQ.isError ? (
        <EmptyState title={t('common.failed')} description={errorMessage(logsQ.error)} />
      ) : (
        <DataTable
          data={rows}
          columns={columns}
          toolbar={
            <>
              <Input value={search} onChange={(e) => setSearch(e.target.value)} placeholder={t('logs.searchPlaceholder')} className="max-w-xs" />
              <Select value={endpoint} onValueChange={(v) => setEndpoint(v as string)}>
                <SelectTrigger className="w-40">
                  <SelectValue />
                </SelectTrigger>
                <SelectContent>
                  <SelectItem value="all">{t('logs.allEndpoints')}</SelectItem>
                  {ENDPOINTS.map((e) => (
                    <SelectItem key={e} value={e}>
                      {e}
                    </SelectItem>
                  ))}
                </SelectContent>
              </Select>
              <Select value={status} onValueChange={(v) => setStatus(v as string)}>
                <SelectTrigger className="w-36">
                  <SelectValue />
                </SelectTrigger>
                <SelectContent>
                  <SelectItem value="all">{t('logs.filterAll')}</SelectItem>
                  <SelectItem value="success">{t('logs.filterSuccess')}</SelectItem>
                  <SelectItem value="error">{t('logs.filterError')}</SelectItem>
                </SelectContent>
              </Select>
            </>
          }
          emptyState={<EmptyState title={logsQ.data?.logs?.length ? t('logs.noMatch') : t('logs.empty')} />}
        />
      )}

      <ConfirmDialog
        open={confirmClear}
        onOpenChange={setConfirmClear}
        title={t('logs.clear')}
        description={t('logs.clearConfirm')}
        destructive
        onConfirm={() => clearM.mutateAsync().then(() => undefined)}
      />
    </div>
  )
}
