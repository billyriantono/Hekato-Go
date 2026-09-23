// models.dev catalog browser: prices, limits and capabilities for every model
// the public feed tracks. Served from /admin/api/models-dev (cached server-side).
import { useQuery } from '@tanstack/react-query'
import type { ColumnDef } from '@tanstack/react-table'
import { useMemo, useState } from 'react'
import { LuRefreshCw } from 'react-icons/lu'
import { EmptyState, LoadingBlock, PageHeader, StatCard, errorMessage, formatNumber, formatTime } from '@/components/common'
import { DataTable } from '@/components/data-table'
import { Badge } from '@/components/ui/badge'
import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from '@/components/ui/select'
import { Switch } from '@/components/ui/switch'
import { get } from '@/lib/api'
import { useI18n } from '@/lib/i18n'

type DevModel = {
  provider: string
  providerName: string
  id: string
  name: string
  input: number
  output: number
  cacheRead: number
  cacheWrite: number
  free: boolean
  contextLimit: number
  outputLimit: number
  reasoning: boolean
  toolCall: boolean
  attachment: boolean
  modalities?: string[]
  releaseDate?: string
  knowledge?: string
}

/** USD per 1M tokens; sub-cent prices keep more digits so they don't read as free. */
const price = (v: number) => (v === 0 ? '$0' : v < 0.01 ? `$${v.toFixed(4)}` : `$${v.toFixed(2)}`)

export function ModelPricesPage() {
  const { t } = useI18n()
  const [provider, setProvider] = useState('all')
  const [freeOnly, setFreeOnly] = useState(false)
  const [search, setSearch] = useState('')

  const q = useQuery({
    queryKey: ['models-dev'],
    queryFn: () => get<{ models: DevModel[]; fetchedAt: number; source: string }>('/models-dev'),
    staleTime: 600_000,
  })
  // The server holds the cache, so a manual sync is a ?refresh=1 pull followed
  // by a plain refetch to pick up the new rows.
  const [refreshing, setRefreshing] = useState(false)
  const hardRefresh = async () => {
    setRefreshing(true)
    try {
      await get('/models-dev?refresh=1')
      await q.refetch()
    } finally {
      setRefreshing(false)
    }
  }

  const all = q.data?.models ?? []
  const providers = useMemo(() => {
    const seen = new Map<string, string>()
    for (const m of all) if (!seen.has(m.provider)) seen.set(m.provider, m.providerName || m.provider)
    return [...seen].sort((a, b) => a[1].localeCompare(b[1]))
  }, [all])

  const rows = useMemo(() => {
    const s = search.trim().toLowerCase()
    return all
      .filter(
        (m) =>
          (provider === 'all' || m.provider === provider) &&
          (!freeOnly || m.free) &&
          (!s || m.id.toLowerCase().includes(s) || m.name?.toLowerCase().includes(s) || m.providerName?.toLowerCase().includes(s)),
      )
      .sort((a, b) => a.provider.localeCompare(b.provider) || a.id.localeCompare(b.id))
  }, [all, provider, freeOnly, search])

  const columns: ColumnDef<DevModel, unknown>[] = [
    {
      accessorKey: 'providerName',
      header: t('models.provider'),
      cell: ({ row }) => <Badge variant="outline">{row.original.providerName || row.original.provider}</Badge>,
    },
    {
      accessorKey: 'id',
      header: t('models.model'),
      cell: ({ row }) => (
        <div className="min-w-0">
          <code className="text-xs">{row.original.id}</code>
          {row.original.name && row.original.name !== row.original.id && (
            <div className="truncate text-[11px] text-muted-foreground">{row.original.name}</div>
          )}
        </div>
      ),
    },
    {
      accessorKey: 'input',
      header: t('models.input'),
      cell: ({ row }) => <span className="text-xs tabular-nums">{price(row.original.input)}</span>,
    },
    {
      accessorKey: 'output',
      header: t('models.output'),
      cell: ({ row }) => <span className="text-xs tabular-nums">{price(row.original.output)}</span>,
    },
    {
      accessorKey: 'cacheRead',
      header: t('models.cacheRead'),
      cell: ({ row }) => <span className="text-xs tabular-nums text-muted-foreground">{price(row.original.cacheRead)}</span>,
    },
    {
      accessorKey: 'contextLimit',
      header: t('models.context'),
      cell: ({ row }) => <span className="text-xs tabular-nums">{row.original.contextLimit ? formatNumber(row.original.contextLimit) : '—'}</span>,
    },
    {
      accessorKey: 'outputLimit',
      header: t('models.maxOutput'),
      cell: ({ row }) => <span className="text-xs tabular-nums">{row.original.outputLimit ? formatNumber(row.original.outputLimit) : '—'}</span>,
    },
    {
      id: 'caps',
      header: t('models.capabilities'),
      cell: ({ row }) => {
        const m = row.original
        const caps = [
          m.free && t('models.free'),
          m.reasoning && t('models.reasoning'),
          m.toolCall && t('models.tools'),
          m.attachment && t('models.vision'),
        ].filter(Boolean) as string[]
        return (
          <div className="flex flex-wrap gap-1">
            {caps.map((c) => (
              <Badge key={c} variant="secondary" className="text-[10px]">
                {c}
              </Badge>
            ))}
          </div>
        )
      },
    },
    {
      accessorKey: 'releaseDate',
      header: t('models.released'),
      cell: ({ row }) => <span className="text-xs text-muted-foreground tabular-nums">{row.original.releaseDate || '—'}</span>,
    },
  ]

  const freeCount = rows.filter((m) => m.free).length

  return (
    <div>
      <PageHeader
        title={t('models.title')}
        description={t('models.pageDescription')}
        actions={
          <Button variant="outline" onClick={hardRefresh} disabled={refreshing || q.isFetching}>
            <LuRefreshCw className={refreshing || q.isFetching ? 'animate-spin' : ''} /> {t('models.refresh')}
          </Button>
        }
      />

      <div className="mb-4 grid grid-cols-2 gap-3 md:grid-cols-4">
        <StatCard label={t('models.providers')} value={formatNumber(providers.length)} />
        <StatCard label={t('models.totalModels')} value={formatNumber(all.length)} hint={`${t('models.shown')}: ${formatNumber(rows.length)}`} />
        <StatCard label={t('models.freeModels')} value={formatNumber(freeCount)} tone="success" />
        <StatCard label={t('models.updated')} value={<span className="text-sm">{formatTime(q.data?.fetchedAt)}</span>} hint="models.dev" />
      </div>

      {q.isLoading ? (
        <LoadingBlock />
      ) : q.isError ? (
        <EmptyState title={t('common.failed')} description={errorMessage(q.error)} />
      ) : (
        <DataTable
          data={rows}
          columns={columns}
          pageSize={50}
          toolbar={
            <>
              <Input value={search} onChange={(e) => setSearch(e.target.value)} placeholder={t('models.searchPlaceholder')} className="max-w-xs" />
              <Select value={provider} onValueChange={(v) => setProvider(v as string)}>
                <SelectTrigger className="w-52">
                  <SelectValue />
                </SelectTrigger>
                <SelectContent>
                  <SelectItem value="all">{t('models.allProviders')}</SelectItem>
                  {providers.map(([id, label]) => (
                    <SelectItem key={id} value={id}>
                      {label}
                    </SelectItem>
                  ))}
                </SelectContent>
              </Select>
              <label className="flex items-center gap-2 text-sm">
                <Switch checked={freeOnly} onCheckedChange={setFreeOnly} /> {t('models.freeOnly')}
              </label>
            </>
          }
          emptyState={<EmptyState title={t('models.noMatch')} />}
        />
      )}
    </div>
  )
}
