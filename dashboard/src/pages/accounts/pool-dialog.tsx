import { useState } from 'react'
import { useQueries, useQuery, useQueryClient } from '@tanstack/react-query'
import { LuLoader } from 'react-icons/lu'
import { toast } from 'sonner'
import { Badge } from '@/components/ui/badge'
import { Button } from '@/components/ui/button'
import { Checkbox } from '@/components/ui/checkbox'
import { Dialog, DialogContent, DialogDescription, DialogHeader, DialogTitle } from '@/components/ui/dialog'
import { Input } from '@/components/ui/input'
import { ScrollArea } from '@/components/ui/scroll-area'
import { errorMessage } from '@/components/common'
import { get, post } from '@/lib/api'
import { useI18n } from '@/lib/i18n'
import type { Account } from './shared'

/** Pool-wide model switches, the pool's custom model prefix and test model. */
export function PoolDialog({ pool, members, onClose }: { pool: string | null; members: Account[]; onClose: () => void }) {
  const { t } = useI18n()
  const qc = useQueryClient()
  const [busy, setBusy] = useState<string | null>(null)
  const [prefix, setPrefix] = useState<string | null>(null)
  const [testModel, setTestModel] = useState<string | null>(null)
  const [filter, setFilter] = useState('')

  const settings = useQuery({ queryKey: ['settings'], queryFn: () => get<{ poolPrefixes?: Record<string, string>; poolTestModels?: Record<string, string> }>('/settings') })
  const prefixes = settings.data?.poolPrefixes ?? {}
  const savedPrefix = pool ? (prefixes[pool] ?? '') : ''
  const testModels = settings.data?.poolTestModels ?? {}
  const savedTestModel = pool ? (testModels[pool] ?? '') : ''
  const catalogs = useQueries({
    queries: members.map((a) => ({
      queryKey: ['account-models', a.id],
      queryFn: () => get<{ models: string[] }>(`/accounts/${a.id}/models/cached`),
      enabled: !!pool,
    })),
  })
  const loading = catalogs.some((c) => c.isLoading)

  // Per model: how many members advertise it, and on how many it is switched on.
  const stats = new Map<string, { has: number; on: number }>()
  members.forEach((a, i) => {
    const allow = a.enabledModels ? new Set(a.enabledModels.map((m) => m.toLowerCase())) : null
    for (const m of catalogs[i]?.data?.models ?? []) {
      const k = m.toLowerCase()
      const s = stats.get(k) ?? { has: 0, on: 0 }
      s.has++
      if (allow === null || allow.has(k)) s.on++
      stats.set(k, s)
    }
  })
  const q = filter.trim().toLowerCase()
  // Enable all / Disable all act on what the filter shows (e.g. ":batch").
  const models = [...stats.keys()].filter((m) => m.includes(q)).sort()

  const apply = (key: string, body: { enable?: string[]; disable?: string[] }) => {
    setBusy(key)
    return post<{ count: number }>('/accounts/batch', { ids: members.map((a) => a.id), action: 'models', ...body })
      .then((r) => toast.success(t('pool.modelsSaved', r.count)))
      .catch((e) => toast.error(errorMessage(e)))
      .finally(() => {
        setBusy(null)
        qc.invalidateQueries({ queryKey: ['accounts'] })
      })
  }
  const saveSetting = (key: 'prefix' | 'testModel', body: object, reset: () => void) => {
    setBusy(key)
    post('/settings', body)
      .then(() => {
        toast.success(t('detail.saved'))
        reset()
        qc.invalidateQueries({ queryKey: ['settings'] })
      })
      .catch((e) => toast.error(errorMessage(e)))
      .finally(() => setBusy(null))
  }
  const savePrefix = () =>
    pool && saveSetting('prefix', { poolPrefixes: { ...prefixes, [pool]: (prefix ?? savedPrefix).trim() } }, () => setPrefix(null))
  const saveTestModel = () =>
    pool && saveSetting('testModel', { poolTestModels: { ...testModels, [pool]: (testModel ?? savedTestModel).trim() } }, () => setTestModel(null))

  return (
    <Dialog open={!!pool} onOpenChange={(o) => !o && onClose()}>
      {/* minmax(0,1fr): the grid's auto column would otherwise grow to the
          longest model ID and push content past the dialog edge. */}
      <DialogContent className="grid-cols-[minmax(0,1fr)] sm:max-w-lg">
        <DialogHeader>
          <DialogTitle>{t('pool.title', pool ?? '')}</DialogTitle>
          <DialogDescription>{t('pool.subtitle', members.length)}</DialogDescription>
        </DialogHeader>

        <div className="space-y-1.5">
          <div className="text-sm font-medium">{t('pool.prefix')}</div>
          <div className="flex gap-2">
            <Input value={prefix ?? savedPrefix} onChange={(e) => setPrefix(e.target.value)} placeholder={pool ?? ''} className="max-w-48" />
            <Button size="sm" variant="outline" onClick={savePrefix} disabled={!!busy || prefix === null || prefix.trim() === savedPrefix}>
              {busy === 'prefix' && <LuLoader className="animate-spin" />} {t('detail.save')}
            </Button>
          </div>
          <div className="text-xs text-muted-foreground">{t('pool.prefixHint', `${(prefix ?? savedPrefix).trim() || pool}/<model>`)}</div>
        </div>

        <div className="space-y-1.5">
          <div className="text-sm font-medium">{t('pool.testModel')}</div>
          <div className="flex gap-2">
            <Input
              value={testModel ?? savedTestModel}
              onChange={(e) => setTestModel(e.target.value)}
              list="pool-test-models"
              placeholder="glm-5.3"
              className="max-w-64 font-mono text-xs"
            />
            <datalist id="pool-test-models">
              {[...stats.keys()].sort().map((m) => (
                <option key={m} value={m} />
              ))}
            </datalist>
            <Button size="sm" variant="outline" onClick={saveTestModel} disabled={!!busy || testModel === null || testModel.trim() === savedTestModel}>
              {busy === 'testModel' && <LuLoader className="animate-spin" />} {t('detail.save')}
            </Button>
          </div>
          <div className="text-xs text-muted-foreground">{t('pool.testModelHint')}</div>
        </div>

        <div className="space-y-1.5">
          <div className="flex flex-wrap items-center gap-2">
            <div className="flex-1 text-sm font-medium">{t('pool.models')}</div>
            <Input value={filter} onChange={(e) => setFilter(e.target.value)} placeholder={t('pool.filter')} className="h-8 w-40" />
            <Button size="sm" variant="outline" disabled={!!busy || !models.length} onClick={() => apply('all-on', { enable: models })}>
              {t('pool.enableAll')}
            </Button>
            <Button size="sm" variant="outline" disabled={!!busy || !models.length} onClick={() => apply('all-off', { disable: models })}>
              {t('pool.disableAll')}
            </Button>
          </div>
          {loading ? (
            <div className="py-6 text-center text-muted-foreground">
              <LuLoader className="mx-auto animate-spin" />
            </div>
          ) : !models.length ? (
            <div className="py-6 text-center text-sm text-muted-foreground">{t(stats.size ? 'common.noData' : 'pool.noModels')}</div>
          ) : (
            <ScrollArea className="h-72 rounded-md border">
              <div className="divide-y">
                {models.map((m) => {
                  const s = stats.get(m)!
                  const allOn = s.on === s.has
                  return (
                    <label key={m} className="flex cursor-pointer items-center gap-2 px-3 py-1.5 text-sm">
                      <Checkbox
                        checked={allOn}
                        indeterminate={s.on > 0 && !allOn}
                        disabled={!!busy}
                        onCheckedChange={() => apply(m, allOn ? { disable: [m] } : { enable: [m] })}
                      />
                      <span className="min-w-0 flex-1 truncate font-mono text-xs" title={m}>
                        {m}
                      </span>
                      {busy === m && <LuLoader className="size-3 animate-spin" />}
                      <Badge variant="outline" className="shrink-0 tabular-nums">
                        {s.on}/{s.has}
                      </Badge>
                    </label>
                  )
                })}
              </div>
            </ScrollArea>
          )}
        </div>
      </DialogContent>
    </Dialog>
  )
}
