import { useMemo, useState, type ReactNode } from 'react'
import { useQuery, useQueryClient } from '@tanstack/react-query'
import type { ColumnDef, RowSelectionState } from '@tanstack/react-table'
import {
  LuBan,
  LuCircleCheck,
  LuCircleOff,
  LuDownload,
  LuEllipsis,
  LuFlaskConical,
  LuGauge,
  LuLoader,
  LuPlus,
  LuRefreshCw,
  LuTrash2,
  LuUpload,
  LuUsers,
} from 'react-icons/lu'
import { toast } from 'sonner'
import { Badge } from '@/components/ui/badge'
import { Button } from '@/components/ui/button'
import { Checkbox } from '@/components/ui/checkbox'
import {
  DropdownMenu,
  DropdownMenuContent,
  DropdownMenuItem,
  DropdownMenuSeparator,
  DropdownMenuTrigger,
} from '@/components/ui/dropdown-menu'
import { Input } from '@/components/ui/input'
import { Progress } from '@/components/ui/progress'
import { ConfirmDialog, EmptyState, LoadingBlock, PageHeader, StatCard, StatusDot, errorMessage, formatNumber, formatTime } from '@/components/common'
import { DataTable } from '@/components/data-table'
import { del, get, post, put } from '@/lib/api'
import { useI18n } from '@/lib/i18n'
import { AddAccountDialog, type Method } from './add-account-dialog'
import { AccountDetailSheet } from './detail-sheet'
import { ExportDialog } from './export-dialog'
import { accountStatus, countdown, pct, statusKey, statusTone, subscriptionLabel, type Account } from './shared'
import { SimpleSelect } from './simple-select'

type Confirm = { title: string; description?: ReactNode; onConfirm: () => Promise<void> }

export function AccountsPage() {
  const { t } = useI18n()
  const qc = useQueryClient()
  const { data, isLoading } = useQuery({ queryKey: ['accounts'], queryFn: () => get<Account[]>('/accounts') })
  const accounts = useMemo(() => data ?? [], [data])

  const [search, setSearch] = useState('')
  const [provider, setProvider] = useState('all')
  const [status, setStatus] = useState('all')
  const [selection, setSelection] = useState<RowSelectionState>({})
  const [detailId, setDetailId] = useState<string | null>(null)
  const [addMethod, setAddMethod] = useState<Method | 'pick' | null>(null)
  const [exportOpen, setExportOpen] = useState(false)
  const [confirm, setConfirm] = useState<Confirm | null>(null)
  const [busy, setBusy] = useState<string | null>(null)

  const invalidate = () => qc.invalidateQueries({ queryKey: ['accounts'] })
  /** Runs an action with a busy marker, error toast and list refresh. */
  const run = async (key: string, fn: () => Promise<void>) => {
    setBusy(key)
    try {
      await fn()
    } catch (e) {
      toast.error(errorMessage(e))
    } finally {
      setBusy(null)
      invalidate()
    }
  }

  const counts = useMemo(() => {
    const c = { total: accounts.length, active: 0, disabled: 0, banned: 0, overQuota: 0 }
    for (const a of accounts) {
      const s = accountStatus(a)
      if (s in c) c[s as keyof typeof c]++
    }
    return c
  }, [accounts])

  const providers = useMemo(() => [...new Set(accounts.map((a) => a.provider).filter(Boolean))].sort(), [accounts])

  const rows = useMemo(() => {
    const q = search.trim().toLowerCase()
    return accounts.filter((a) => {
      if (provider !== 'all' && a.provider !== provider) return false
      const s = accountStatus(a)
      if (status === 'enabled' && !a.enabled) return false
      if (status === 'disabled' && a.enabled) return false
      if (status !== 'all' && status !== 'enabled' && status !== 'disabled' && s !== status) return false
      return !q || (a.email ?? '').toLowerCase().includes(q) || (a.nickname ?? '').toLowerCase().includes(q)
    })
  }, [accounts, search, provider, status])

  const selectedIds = useMemo(() => accounts.filter((a) => selection[a.id]).map((a) => a.id), [accounts, selection])

  // ---- per-account actions -------------------------------------------------
  const testAccount = (a: Account) =>
    run(`test:${a.id}`, async () => {
      const r = await post<{ reply: string; model: string }>(`/accounts/${a.id}/test`)
      toast.success(`${t('accounts.testSuccess')} · ${r.model}`, { description: r.reply })
    })
  const refreshAccount = (a: Account) =>
    run(`refresh:${a.id}`, async () => {
      await post(`/accounts/${a.id}/refresh`)
      toast.success(t('accounts.refreshed'))
    })
  const refreshModels = (a: Account) =>
    run(`models:${a.id}`, async () => {
      const r = await post<{ count: number }>(`/accounts/${a.id}/models/refresh`)
      toast.success(t('accounts.modelsRefreshed', r.count))
    })
  const toggleEnabled = (a: Account) =>
    run(`toggle:${a.id}`, async () => {
      await put(`/accounts/${a.id}`, { enabled: !a.enabled })
      toast.success(t(a.enabled ? 'accounts.disabled' : 'accounts.enabled'))
    })
  const copyJson = (a: Account) =>
    run(`copy:${a.id}`, async () => {
      const full = await get(`/accounts/${a.id}/full`)
      await navigator.clipboard.writeText(JSON.stringify(full, null, 2))
      toast.success(t('accounts.copyJSONSuccess'))
    })
  const deleteAccount = (a: Account) =>
    setConfirm({
      title: t('accounts.confirmDelete'),
      description: a.email,
      onConfirm: () =>
        run(`delete:${a.id}`, async () => {
          await del(`/accounts/${a.id}`)
          toast.success(t('accounts.deleteSuccess'))
        }),
    })

  // ---- bulk / header actions ----------------------------------------------
  const batch = (action: 'enable' | 'disable' | 'refresh', ids: string[]) =>
    run(`batch:${action}`, async () => {
      const r = await post<{ count?: number; refreshed?: number; failed?: number }>('/accounts/batch', { ids, action })
      if (action === 'refresh') toast.success(t('batch.refreshResult', r.refreshed ?? 0, r.failed ?? 0))
      else toast.success(t(action === 'enable' ? 'batch.enableResult' : 'batch.disableResult', r.count ?? ids.length))
      setSelection({})
    })
  const loop = async (ids: string[], fn: (id: string) => Promise<unknown>) => {
    const results = await Promise.allSettled(ids.map(fn))
    const ok = results.filter((r) => r.status === 'fulfilled').length
    return [ok, ids.length - ok] as const
  }
  const bulkRefreshModels = () =>
    run('batch:models', async () => {
      const [ok, fail] = await loop(selectedIds, (id) => post(`/accounts/${id}/models/refresh`))
      toast.success(t('batch.refreshModelsResult', ok, fail))
    })
  const bulkDelete = () =>
    setConfirm({
      title: t('batch.delete'),
      description: t('batch.confirmDelete', selectedIds.length),
      onConfirm: () =>
        run('batch:delete', async () => {
          const [ok, fail] = await loop(selectedIds, (id) => del(`/accounts/${id}`))
          toast.success(t('batch.deleteResult', ok, fail))
          setSelection({})
        }),
    })
  const refreshAllModels = () =>
    run('models:all', async () => {
      const r = await post<{ refreshed: number }>('/accounts/models/refresh')
      toast.success(t('models.refreshAllDone', r.refreshed))
    })

  // ---- table ---------------------------------------------------------------
  const columns: ColumnDef<Account, unknown>[] = [
    {
      id: 'select',
      enableSorting: false,
      header: ({ table }) => (
        <Checkbox
          checked={table.getIsAllRowsSelected()}
          indeterminate={table.getIsSomeRowsSelected()}
          onCheckedChange={(v) => table.toggleAllRowsSelected(!!v)}
          aria-label={t('batch.selectAll')}
        />
      ),
      cell: ({ row }) => (
        <Checkbox
          checked={row.getIsSelected()}
          onCheckedChange={(v) => row.toggleSelected(!!v)}
          onClick={(e) => e.stopPropagation()}
          aria-label={t('accounts.selectAccount', row.original.email)}
        />
      ),
    },
    {
      id: 'account',
      accessorFn: (a) => a.nickname || a.email,
      header: t('accounts.account'),
      cell: ({ row: { original: a } }) => (
        <div className="min-w-0 max-w-64">
          <div className="truncate font-medium">{a.nickname || a.email}</div>
          {a.nickname && <div className="truncate text-xs text-muted-foreground">{a.email}</div>}
          <div className="mt-1 flex flex-wrap items-center gap-1">
            <Badge variant="secondary">{a.provider || '—'}</Badge>
            <span className="text-[11px] text-muted-foreground">
              {a.authMethod}
              {a.region && ` · ${a.region}`}
            </span>
          </div>
        </div>
      ),
    },
    {
      id: 'status',
      accessorFn: (a) => accountStatus(a),
      header: t('filter.status'),
      cell: ({ row: { original: a } }) => {
        const s = accountStatus(a)
        const left = countdown(a.expiresAt)
        return (
          <div className="space-y-0.5">
            <StatusDot tone={statusTone[s]} label={t(statusKey[s])} />
            {s === 'banned' && a.banStatus && <div className="text-[11px] text-muted-foreground">{a.banStatus}</div>}
            {a.expiresAt > 0 && (
              <div className="text-[11px] text-muted-foreground">
                {t('accounts.expiry')}: {left || t('accounts.expired')}
              </div>
            )}
          </div>
        )
      },
    },
    {
      id: 'usage',
      accessorFn: (a) => a.usagePercent,
      header: t('detail.usage'),
      cell: ({ row: { original: a } }) => (
        <div className="w-40 space-y-1.5">
          <div className="flex items-center justify-between gap-2 text-xs">
            <Badge variant="outline">{subscriptionLabel(a, t)}</Badge>
            <span className="tabular-nums text-muted-foreground">
              {formatNumber(a.usageCurrent)}/{formatNumber(a.usageLimit)}
            </span>
          </div>
          <Progress value={pct(a.usagePercent)} className="gap-0" />
          {a.trialUsageLimit > 0 && (
            <>
              <div className="flex items-center justify-between text-[11px] text-muted-foreground">
                <span>{t('accounts.trial')}</span>
                <span className="tabular-nums">
                  {formatNumber(a.trialUsageCurrent)}/{formatNumber(a.trialUsageLimit)}
                </span>
              </div>
              <Progress value={pct(a.trialUsagePercent)} className="gap-0 opacity-70" />
            </>
          )}
        </div>
      ),
    },
    {
      id: 'weight',
      accessorFn: (a) => a.weight,
      header: t('accounts.weight'),
      cell: ({ row: { original: a } }) =>
        a.weight >= 2 ? <Badge>{a.weight}</Badge> : <span className="tabular-nums text-muted-foreground">{a.weight}</span>,
    },
    {
      id: 'stats',
      accessorFn: (a) => a.requestCount,
      header: t('detail.statistics'),
      cell: ({ row: { original: a } }) => (
        <div className="text-xs tabular-nums leading-5">
          <div>
            {formatNumber(a.requestCount)} <span className="text-muted-foreground">{t('accounts.requests')}</span>
          </div>
          <div>
            {formatNumber(a.totalTokens)} <span className="text-muted-foreground">{t('accounts.tokens')}</span>
          </div>
          <div>
            {(a.totalCredits ?? 0).toFixed(2)} <span className="text-muted-foreground">{t('accounts.credits')}</span>
          </div>
        </div>
      ),
    },
    {
      id: 'lastUsed',
      accessorFn: (a) => a.lastUsed,
      header: t('accounts.lastUsed'),
      cell: ({ row: { original: a } }) => <span className="text-xs text-muted-foreground">{formatTime(a.lastUsed)}</span>,
    },
    {
      id: 'actions',
      enableSorting: false,
      header: '',
      cell: ({ row: { original: a } }) => {
        const rowBusy = busy?.endsWith(`:${a.id}`)
        return (
          <div onClick={(e) => e.stopPropagation()}>
            <DropdownMenu>
              <DropdownMenuTrigger
                render={
                  <Button variant="ghost" size="icon-sm" aria-label={t('common.actions')}>
                    {rowBusy ? <LuLoader className="animate-spin" /> : <LuEllipsis />}
                  </Button>
                }
              />
              <DropdownMenuContent align="end">
                <DropdownMenuItem onClick={() => testAccount(a)}>
                  <LuFlaskConical /> {t('accounts.test')}
                </DropdownMenuItem>
                <DropdownMenuItem onClick={() => refreshAccount(a)}>
                  <LuRefreshCw /> {t('accounts.refresh')}
                </DropdownMenuItem>
                <DropdownMenuItem onClick={() => refreshModels(a)}>
                  <LuGauge /> {t('batch.refreshModels')}
                </DropdownMenuItem>
                <DropdownMenuItem onClick={() => copyJson(a)}>{t('accounts.copyJSON')}</DropdownMenuItem>
                <DropdownMenuSeparator />
                <DropdownMenuItem onClick={() => toggleEnabled(a)}>
                  {a.enabled ? <LuCircleOff /> : <LuCircleCheck />} {t(a.enabled ? 'accounts.disable' : 'accounts.enable')}
                </DropdownMenuItem>
                <DropdownMenuItem variant="destructive" onClick={() => deleteAccount(a)}>
                  <LuTrash2 /> {t('accounts.delete')}
                </DropdownMenuItem>
              </DropdownMenuContent>
            </DropdownMenu>
          </div>
        )
      },
    },
  ]

  const statusOptions = [
    { value: 'all', label: t('filter.all') },
    { value: 'enabled', label: t('filter.enabled') },
    { value: 'disabled', label: t('filter.disabled') },
    { value: 'banned', label: t('filter.banned') },
    { value: 'overQuota', label: t('accounts.overQuota') },
    { value: 'expired', label: t('accounts.expired') },
    { value: 'noToken', label: t('accounts.noToken') },
  ]

  return (
    <div>
      <PageHeader
        title={t('accounts.title')}
        description={t('accounts.subtitle')}
        actions={
          <>
            <Button variant="outline" size="sm" onClick={() => batch('refresh', accounts.map((a) => a.id))} disabled={!!busy || !accounts.length}>
              {busy === 'batch:refresh' ? <LuLoader className="animate-spin" /> : <LuRefreshCw />} {t('accounts.refreshAll')}
            </Button>
            <Button variant="outline" size="sm" onClick={refreshAllModels} disabled={!!busy || !accounts.length}>
              {busy === 'models:all' ? <LuLoader className="animate-spin" /> : <LuGauge />} {t('models.refreshAll')}
            </Button>
            <Button variant="outline" size="sm" onClick={() => setExportOpen(true)} disabled={!accounts.length}>
              <LuDownload /> {t('accounts.export')}
            </Button>
            <Button variant="outline" size="sm" onClick={() => setAddMethod('credentials')}>
              <LuUpload /> {t('accounts.import')}
            </Button>
            <Button size="sm" onClick={() => setAddMethod('pick')}>
              <LuPlus /> {t('accounts.add')}
            </Button>
          </>
        }
      />

      <div className="mb-6 grid grid-cols-2 gap-3 md:grid-cols-5">
        <StatCard label={t('accounts.total')} value={counts.total} icon={<LuUsers className="size-4" />} />
        <StatCard label={t('accounts.available')} value={counts.active} tone="success" icon={<LuCircleCheck className="size-4" />} />
        <StatCard label={t('accounts.disabled')} value={counts.disabled} icon={<LuCircleOff className="size-4" />} />
        <StatCard label={t('accounts.banned')} value={counts.banned} tone={counts.banned ? 'danger' : 'default'} icon={<LuBan className="size-4" />} />
        <StatCard label={t('accounts.overQuota')} value={counts.overQuota} tone={counts.overQuota ? 'warning' : 'default'} icon={<LuGauge className="size-4" />} />
      </div>

      {selectedIds.length > 0 && (
        <div className="mb-3 flex flex-wrap items-center gap-2 rounded-lg border bg-muted/40 px-3 py-2 text-sm">
          <span className="font-medium">{t('batch.selected', selectedIds.length)}</span>
          <div className="flex-1" />
          <Button size="sm" variant="outline" disabled={!!busy} onClick={() => batch('enable', selectedIds)}>
            <LuCircleCheck /> {t('batch.enable')}
          </Button>
          <Button size="sm" variant="outline" disabled={!!busy} onClick={() => batch('disable', selectedIds)}>
            <LuCircleOff /> {t('batch.disable')}
          </Button>
          <Button size="sm" variant="outline" disabled={!!busy} onClick={() => batch('refresh', selectedIds)}>
            <LuRefreshCw /> {t('batch.refresh')}
          </Button>
          <Button size="sm" variant="outline" disabled={!!busy} onClick={bulkRefreshModels}>
            <LuGauge /> {t('batch.refreshModels')}
          </Button>
          <Button size="sm" variant="destructive" disabled={!!busy} onClick={bulkDelete}>
            <LuTrash2 /> {t('batch.delete')}
          </Button>
          <Button size="sm" variant="ghost" onClick={() => setSelection({})}>
            {t('common.cancel')}
          </Button>
        </div>
      )}

      {isLoading ? (
        <LoadingBlock />
      ) : (
        <DataTable
          data={rows}
          columns={columns}
          getRowId={(a) => a.id}
          rowSelection={selection}
          onRowSelectionChange={setSelection}
          onRowClick={(a) => setDetailId(a.id)}
          toolbar={
            <>
              <Input value={search} onChange={(e) => setSearch(e.target.value)} placeholder={t('filter.search')} className="max-w-xs" />
              <SimpleSelect
                value={provider}
                onChange={setProvider}
                options={[{ value: 'all', label: t('accounts.allProviders') }, ...providers.map((p) => ({ value: p, label: p }))]}
              />
              <SimpleSelect value={status} onChange={setStatus} options={statusOptions} />
              <span className="ml-auto text-xs text-muted-foreground">{t('accounts.showing', rows.length, accounts.length)}</span>
            </>
          }
          emptyState={
            <EmptyState
              title={t('accounts.empty')}
              description={accounts.length ? undefined : t('accounts.emptyHint')}
              action={
                !accounts.length && (
                  <Button size="sm" onClick={() => setAddMethod('pick')}>
                    <LuPlus /> {t('accounts.add')}
                  </Button>
                )
              }
            />
          }
        />
      )}

      <AccountDetailSheet account={accounts.find((a) => a.id === detailId) ?? null} onClose={() => setDetailId(null)} />
      <AddAccountDialog open={addMethod !== null} initialMethod={addMethod === 'pick' ? null : addMethod} onClose={() => setAddMethod(null)} />
      <ExportDialog open={exportOpen} onOpenChange={setExportOpen} accounts={accounts} preselected={selectedIds} />
      <ConfirmDialog
        open={!!confirm}
        onOpenChange={(o) => !o && setConfirm(null)}
        title={confirm?.title ?? ''}
        description={confirm?.description}
        destructive
        confirmLabel={t('common.delete')}
        onConfirm={() => confirm?.onConfirm() ?? Promise.resolve()}
      />
    </div>
  )
}
