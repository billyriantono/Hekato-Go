import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query'
import type { ColumnDef } from '@tanstack/react-table'
import { useState } from 'react'
import { LuCopy, LuKeyRound, LuLoader, LuPencil, LuPlus, LuRotateCcw, LuTrash2 } from 'react-icons/lu'
import { toast } from 'sonner'
import { ConfirmDialog, CopyButton, EmptyState, LoadingBlock, PageHeader, errorMessage, formatNumber, formatTime } from '@/components/common'
import { DataTable } from '@/components/data-table'
import { Badge } from '@/components/ui/badge'
import { Button } from '@/components/ui/button'
import { Card, CardContent } from '@/components/ui/card'
import { Dialog, DialogContent, DialogDescription, DialogFooter, DialogHeader, DialogTitle } from '@/components/ui/dialog'
import { Input } from '@/components/ui/input'
import { Label } from '@/components/ui/label'
import { Progress } from '@/components/ui/progress'
import { Switch } from '@/components/ui/switch'
import { del, get, post, put } from '@/lib/api'
import { useI18n } from '@/lib/i18n'

type ApiKey = {
  id: string
  name?: string
  keyMasked: string
  enabled: boolean
  migrated?: boolean
  createdAt: number
  lastUsedAt?: number
  tokenLimit?: number
  creditLimit?: number
  rpmLimit?: number
  concurrencyLimit?: number
  allowedModels?: string[]
  tokensUsed: number
  creditsUsed: number
  requestsCount: number
}

type Form = { name: string; key: string; enabled: boolean; tokenLimit: string; creditLimit: string; rpmLimit: string; concurrencyLimit: string; allowedModels: string }
const emptyForm: Form = { name: '', key: '', enabled: true, tokenLimit: '0', creditLimit: '0', rpmLimit: '0', concurrencyLimit: '0', allowedModels: '' }
const splitModels = (s: string) => s.split(/[,\n]/).map((m) => m.trim()).filter(Boolean)
const toForm = (k: ApiKey): Form => ({
  name: k.name ?? '',
  key: '',
  enabled: k.enabled,
  tokenLimit: String(k.tokenLimit ?? 0),
  creditLimit: String(k.creditLimit ?? 0),
  rpmLimit: String(k.rpmLimit ?? 0),
  concurrencyLimit: String(k.concurrencyLimit ?? 0),
  allowedModels: (k.allowedModels ?? []).join(', '),
})

const validNumber = (s: string, integer: boolean) => {
  const n = Number(s)
  return s.trim() !== '' && Number.isFinite(n) && n >= 0 && (!integer || Number.isInteger(n))
}

export function ApiKeysPage() {
  const { t } = useI18n()
  const qc = useQueryClient()
  const keysQ = useQuery({ queryKey: ['api-keys'], queryFn: () => get<{ apiKeys: ApiKey[] }>('/api-keys') })
  const settingsQ = useQuery({ queryKey: ['settings'], queryFn: () => get<{ requireApiKey: boolean }>('/settings') })
  const keys = keysQ.data?.apiKeys ?? []
  const enabledCount = keys.filter((k) => k.enabled).length
  const invalidateKeys = () => qc.invalidateQueries({ queryKey: ['api-keys'] })
  const onError = (e: unknown) => toast.error(errorMessage(e, t('common.unknownError')))

  const [editing, setEditing] = useState<ApiKey | 'new' | null>(null)
  const [shownKey, setShownKey] = useState<{ value: string; existing: boolean } | null>(null)
  const [copyingKeyId, setCopyingKeyId] = useState<string | null>(null)
  const [confirm, setConfirm] = useState<{ kind: 'delete' | 'reset'; key: ApiKey } | { kind: 'require' } | null>(null)

  const requireM = useMutation({
    mutationFn: (v: boolean) => post('/settings', { requireApiKey: v }),
    onSuccess: () => {
      toast.success(t('apiKeys.settingSaved'))
      qc.invalidateQueries({ queryKey: ['settings'] })
    },
    onError,
  })
  const toggleM = useMutation({
    mutationFn: (k: ApiKey) => put(`/api-keys/${k.id}`, { enabled: !k.enabled }),
    onSuccess: () => {
      toast.success(t('apiKeys.updated'))
      invalidateKeys()
    },
    onError,
  })

  const setRequire = (v: boolean) => (v && enabledCount === 0 ? setConfirm({ kind: 'require' }) : requireM.mutate(v))
  const copyExistingKey = async (key: ApiKey) => {
    setCopyingKeyId(key.id)
    try {
      const detail = await get<ApiKey & { key: string }>(`/api-keys/${key.id}`)
      setShownKey({ value: detail.key, existing: true })
      try {
        await navigator.clipboard.writeText(detail.key)
        toast.success(t('common.copied'))
      } catch {
        toast.error(t('common.failed'))
      }
    } catch (e) {
      onError(e)
    } finally {
      setCopyingKeyId(null)
    }
  }

  const baseUrl = `${location.origin}/v1`

  const columns: ColumnDef<ApiKey, unknown>[] = [
    {
      accessorKey: 'name',
      header: t('apiKeys.colName'),
      cell: ({ row }) => (
        <div className="flex items-center gap-2">
          <span className={row.original.name ? 'font-medium' : 'text-muted-foreground'}>{row.original.name || t('apiKeys.unnamed')}</span>
          {row.original.migrated && <Badge variant="outline">{t('apiKeys.migrated')}</Badge>}
        </div>
      ),
    },
    { accessorKey: 'keyMasked', header: t('apiKeys.colKey'), cell: ({ getValue }) => <code className="text-xs">{String(getValue())}</code> },
    {
      accessorKey: 'enabled',
      header: t('common.enabled'),
      cell: ({ row }) => (
        <Switch size="sm" checked={row.original.enabled} onCheckedChange={() => toggleM.mutate(row.original)} disabled={toggleM.isPending} />
      ),
    },
    {
      id: 'limits',
      header: t('apiKeys.colLimits'),
      enableSorting: false,
      cell: ({ row }) => {
        const k = row.original
        const lim = (v: number | undefined, unit: string) => (v ? `${formatNumber(v)} ${unit}` : `${unit}: ${t('apiKeys.unlimited')}`)
        return (
          <div className="text-xs text-muted-foreground">
            <div>
              {lim(k.tokenLimit, t('apiKeys.tokens'))} · {lim(k.creditLimit, t('apiKeys.credits'))}
            </div>
            <div>
              {lim(k.rpmLimit, t('apiKeys.rpm'))} · {lim(k.concurrencyLimit, t('apiKeys.concurrency'))}
            </div>
            <div className="font-mono" title={(k.allowedModels ?? []).join(', ')}>
              {k.allowedModels?.length ? t('apiKeys.modelsAllowed', k.allowedModels.length) + ': ' + k.allowedModels.slice(0, 3).join(', ') + (k.allowedModels.length > 3 ? '…' : '') : t('apiKeys.modelsAll')}
            </div>
          </div>
        )
      },
    },
    {
      accessorKey: 'requestsCount',
      header: t('apiKeys.colUsage'),
      cell: ({ row }) => <UsageCell k={row.original} />,
    },
    { accessorKey: 'lastUsedAt', header: t('apiKeys.colLastUsed'), cell: ({ getValue }) => <span className="text-xs">{(getValue() as number) ? formatTime(getValue() as number) : t('apiKeys.never')}</span> },
    { accessorKey: 'createdAt', header: t('apiKeys.colCreated'), cell: ({ getValue }) => <span className="text-xs">{formatTime(getValue() as number)}</span> },
    {
      id: 'actions',
      header: t('common.actions'),
      enableSorting: false,
      cell: ({ row }) => (
        <div className="flex gap-1">
          <Button
            variant="ghost"
            size="icon-sm"
            title={t('apiKeys.actionCopy')}
            aria-label={t('apiKeys.actionCopy')}
            onClick={() => copyExistingKey(row.original)}
            disabled={copyingKeyId === row.original.id}
          >
            {copyingKeyId === row.original.id ? <LuLoader className="animate-spin" /> : <LuCopy />}
          </Button>
          <Button variant="ghost" size="icon-sm" title={t('apiKeys.actionEdit')} onClick={() => setEditing(row.original)}>
            <LuPencil />
          </Button>
          <Button variant="ghost" size="icon-sm" title={t('apiKeys.actionReset')} onClick={() => setConfirm({ kind: 'reset', key: row.original })}>
            <LuRotateCcw />
          </Button>
          <Button variant="ghost" size="icon-sm" title={t('apiKeys.actionDelete')} className="text-destructive" onClick={() => setConfirm({ kind: 'delete', key: row.original })}>
            <LuTrash2 />
          </Button>
        </div>
      ),
    },
  ]

  return (
    <div>
      <PageHeader
        title={t('apiKeys.listTitle')}
        description={t('apiKeys.pageDescription')}
        actions={
          <Button onClick={() => setEditing('new')}>
            <LuPlus /> {t('apiKeys.add')}
          </Button>
        }
      />

      <Card size="sm" className="mb-4">
        <CardContent className="flex items-start justify-between gap-4">
          <div className="min-w-0 space-y-1">
            <div className="flex items-center gap-2 text-sm font-medium">
              <LuKeyRound className="size-4 text-muted-foreground" /> {t('apiKeys.requireTitle')}
              <Badge variant={enabledCount ? 'secondary' : 'destructive'}>{t('apiKeys.requireEnabledCount', enabledCount, keys.length)}</Badge>
            </div>
            <p className="text-xs text-muted-foreground">{t('apiKeys.requireHint')}</p>
            <p className="text-xs text-muted-foreground">{t('apiKeys.authHint', baseUrl)}</p>
          </div>
          <Switch checked={!!settingsQ.data?.requireApiKey} onCheckedChange={setRequire} disabled={settingsQ.isLoading || requireM.isPending} />
        </CardContent>
      </Card>

      {keysQ.isLoading ? (
        <LoadingBlock />
      ) : keysQ.isError ? (
        <EmptyState title={t('apiKeys.loadFailed')} description={errorMessage(keysQ.error)} />
      ) : (
        <DataTable
          data={keys}
          columns={columns}
          searchable
          searchPlaceholder={t('apiKeys.searchPlaceholder')}
          getRowId={(k) => k.id}
          emptyState={<EmptyState title={t('apiKeys.empty')} />}
        />
      )}

      {editing && (
        <KeyDialog
          initial={editing === 'new' ? null : editing}
          onClose={() => setEditing(null)}
          onCreated={(key) => {
            setEditing(null)
            setShownKey({ value: key, existing: false })
          }}
        />
      )}

      <Dialog open={shownKey !== null} onOpenChange={(o) => !o && setShownKey(null)}>
        <DialogContent className="sm:max-w-md">
          <DialogHeader>
            <DialogTitle>{t(shownKey?.existing ? 'apiKeys.showExistingTitle' : 'apiKeys.showTitle')}</DialogTitle>
            <DialogDescription className="text-amber-600 dark:text-amber-400">
              {t(shownKey?.existing ? 'apiKeys.showExistingWarning' : 'apiKeys.showWarning')}
            </DialogDescription>
          </DialogHeader>
          <div className="flex items-center gap-2 rounded-md border bg-muted/50 p-2">
            <code className="flex-1 text-xs break-all">{shownKey?.value}</code>
            <CopyButton value={shownKey?.value ?? ''} />
          </div>
          <DialogFooter>
            <Button onClick={() => setShownKey(null)}>{t('apiKeys.closeBtn')}</Button>
          </DialogFooter>
        </DialogContent>
      </Dialog>

      <ConfirmDialog
        open={confirm?.kind === 'require'}
        onOpenChange={(o) => !o && setConfirm(null)}
        title={t('apiKeys.requireWarnTitle')}
        description={t('apiKeys.requireWithoutEnabledKeyWarning')}
        onConfirm={() => requireM.mutateAsync(true).then(() => undefined)}
      />
      <ConfirmDialog
        open={confirm?.kind === 'delete'}
        onOpenChange={(o) => !o && setConfirm(null)}
        title={t('apiKeys.actionDelete')}
        description={confirm?.kind === 'delete' ? t('apiKeys.confirmDelete', confirm.key.name || confirm.key.keyMasked) : ''}
        destructive
        onConfirm={async () => {
          if (confirm?.kind !== 'delete') return
          try {
            await del(`/api-keys/${confirm.key.id}`)
            toast.success(t('apiKeys.deleteSuccess'))
            invalidateKeys()
          } catch (e) {
            onError(e)
          }
        }}
      />
      <ConfirmDialog
        open={confirm?.kind === 'reset'}
        onOpenChange={(o) => !o && setConfirm(null)}
        title={t('apiKeys.actionReset')}
        description={confirm?.kind === 'reset' ? t('apiKeys.confirmReset', confirm.key.name || confirm.key.keyMasked) : ''}
        onConfirm={async () => {
          if (confirm?.kind !== 'reset') return
          try {
            await post(`/api-keys/${confirm.key.id}/reset-usage`)
            toast.success(t('apiKeys.usageReset'))
            invalidateKeys()
          } catch (e) {
            onError(e)
          }
        }}
      />
    </div>
  )
}

function UsageCell({ k }: { k: ApiKey }) {
  const { t } = useI18n()
  const bar = (used: number, limit: number | undefined, label: string) => (
    <div className="flex items-center gap-2 text-xs tabular-nums">
      <span className="w-14 text-muted-foreground">{label}</span>
      <span className="w-24">
        {formatNumber(used)}
        {limit ? ` / ${formatNumber(limit)}` : ''}
      </span>
      {limit ? <Progress value={Math.min(100, (used / limit) * 100)} className="w-16" /> : null}
    </div>
  )
  return (
    <div className="space-y-0.5">
      {bar(k.tokensUsed, k.tokenLimit, t('apiKeys.tokens'))}
      {bar(k.creditsUsed, k.creditLimit, t('apiKeys.credits'))}
      <div className="text-xs text-muted-foreground">
        {t('apiKeys.requests')}: {formatNumber(k.requestsCount)}
      </div>
    </div>
  )
}

function KeyDialog({ initial, onClose, onCreated }: { initial: ApiKey | null; onClose: () => void; onCreated: (key: string) => void }) {
  const { t } = useI18n()
  const qc = useQueryClient()
  const [form, setForm] = useState<Form>(initial ? toForm(initial) : emptyForm)
  const set = (patch: Partial<Form>) => setForm((f) => ({ ...f, ...patch }))

  const numFields: { field: keyof Form; label: string; integer: boolean }[] = [
    { field: 'tokenLimit', label: t('apiKeys.limitTokens'), integer: true },
    { field: 'creditLimit', label: t('apiKeys.limitCredits'), integer: false },
    { field: 'rpmLimit', label: t('apiKeys.limitRpm'), integer: true },
    { field: 'concurrencyLimit', label: t('apiKeys.limitConcurrency'), integer: true },
  ]
  const errors = Object.fromEntries(numFields.map((f) => [f.field, validNumber(form[f.field] as string, f.integer) ? '' : t(f.integer ? 'apiKeys.invalidInteger' : 'apiKeys.invalidNumber')]))
  const valid = Object.values(errors).every((e) => !e)

  const saveM = useMutation({
    mutationFn: async () => {
      const body = {
        name: form.name.trim(),
        enabled: form.enabled,
        tokenLimit: Number(form.tokenLimit),
        creditLimit: Number(form.creditLimit),
        rpmLimit: Number(form.rpmLimit),
        concurrencyLimit: Number(form.concurrencyLimit),
        allowedModels: splitModels(form.allowedModels),
      }
      if (initial) return put<{ success: boolean }>(`/api-keys/${initial.id}`, body)
      return post<{ key: string }>('/api-keys', { ...body, key: form.key.trim() || undefined })
    },
    onSuccess: (res) => {
      qc.invalidateQueries({ queryKey: ['api-keys'] })
      if (initial) {
        toast.success(t('apiKeys.updated'))
        onClose()
      } else {
        toast.success(t('apiKeys.created'))
        onCreated((res as { key: string }).key)
      }
    },
    onError: (e) => toast.error(errorMessage(e, t('common.unknownError'))),
  })

  return (
    <Dialog open onOpenChange={(o) => !o && onClose()}>
      <DialogContent className="sm:max-w-md">
        <DialogHeader>
          <DialogTitle>{t(initial ? 'apiKeys.modalTitleEdit' : 'apiKeys.modalTitleCreate')}</DialogTitle>
        </DialogHeader>
        <form
          className="space-y-4"
          onSubmit={(e) => {
            e.preventDefault()
            if (valid) saveM.mutate()
          }}
        >
          <div className="space-y-2">
            <Label htmlFor="k-name">{t('apiKeys.formName')}</Label>
            <Input id="k-name" value={form.name} onChange={(e) => set({ name: e.target.value })} placeholder={t('apiKeys.formNamePlaceholder')} autoFocus />
          </div>
          <div className="space-y-2">
            <Label htmlFor="k-key">{t('apiKeys.formKey')}</Label>
            {initial ? (
              <Input id="k-key" value={initial.keyMasked} readOnly className="font-mono text-xs" />
            ) : (
              <Input id="k-key" value={form.key} onChange={(e) => set({ key: e.target.value })} placeholder={t('apiKeys.formKeyPlaceholder')} className="font-mono text-xs" />
            )}
          </div>
          <label className="flex items-center gap-2 text-sm">
            <Switch checked={form.enabled} onCheckedChange={(v) => set({ enabled: v })} /> {t('apiKeys.formEnabled')}
          </label>
          <div className="grid grid-cols-2 gap-3">
            {numFields.map(({ field, label, integer }) => (
              <div key={field} className="space-y-1">
                <Label htmlFor={`k-${field}`}>{label}</Label>
                <Input
                  id={`k-${field}`}
                  type="number"
                  min={0}
                  step={integer ? 1 : 'any'}
                  value={form[field] as string}
                  onChange={(e) => set({ [field]: e.target.value })}
                  aria-invalid={!!errors[field]}
                />
                <p className={errors[field] ? 'text-xs text-destructive' : 'text-xs text-muted-foreground'}>{errors[field] || t('apiKeys.limitHint')}</p>
              </div>
            ))}
          </div>
          <div className="space-y-1">
            <Label htmlFor="k-models">{t('apiKeys.allowedModels')}</Label>
            <Input id="k-models" value={form.allowedModels} onChange={(e) => set({ allowedModels: e.target.value })} placeholder={t('apiKeys.allowedModelsPlaceholder')} className="font-mono text-xs" />
            <p className="text-xs text-muted-foreground">{t('apiKeys.allowedModelsHint')}</p>
          </div>
          <DialogFooter>
            <Button type="button" variant="outline" onClick={onClose} disabled={saveM.isPending}>
              {t('apiKeys.cancelBtn')}
            </Button>
            <Button type="submit" disabled={!valid || saveM.isPending}>
              {saveM.isPending && <LuLoader className="animate-spin" />}
              {t('apiKeys.saveBtn')}
            </Button>
          </DialogFooter>
        </form>
      </DialogContent>
    </Dialog>
  )
}
