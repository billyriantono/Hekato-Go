import { useState, type ReactNode } from 'react'
import { useQuery, useQueryClient } from '@tanstack/react-query'
import { LuCloudDownload, LuFlame, LuFlaskConical, LuLoader, LuPlus, LuRefreshCw, LuTrash2, LuWand, LuX } from 'react-icons/lu'
import { toast } from 'sonner'
import { Alert, AlertDescription, AlertTitle } from '@/components/ui/alert'
import { Badge } from '@/components/ui/badge'
import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'
import { Label } from '@/components/ui/label'
import { Progress } from '@/components/ui/progress'
import { Separator } from '@/components/ui/separator'
import { Sheet, SheetContent, SheetDescription, SheetHeader, SheetTitle } from '@/components/ui/sheet'
import { Switch } from '@/components/ui/switch'
import { ConfirmDialog, CopyButton, StatusDot, errorMessage, formatNumber, formatTime } from '@/components/common'
import { del, get, post, put } from '@/lib/api'
import { useI18n } from '@/lib/i18n'
import { SimpleSelect } from './simple-select'
import {
  accountStatus,
  countdown,
  isHttpUrl,
  isMachineId,
  isProxyUrl,
  pct,
  statusKey,
  statusTone,
  subscriptionLabel,
  warmupCounts,
  type Account,
  type WarmupResult,
  isKiroAccount,
} from './shared'

export function AccountDetailSheet({ account, onClose }: { account: Account | null; onClose: () => void }) {
  return (
    <Sheet open={!!account} onOpenChange={(o) => !o && onClose()}>
      <SheetContent className="w-full overflow-y-auto sm:max-w-xl">
        {account && <Body key={account.id} a={account} onClose={onClose} />}
      </SheetContent>
    </Sheet>
  )
}

function Section({ title, action, children }: { title: string; action?: ReactNode; children: ReactNode }) {
  return (
    <section className="px-4">
      <div className="mb-2 flex items-center justify-between">
        <h3 className="text-xs font-semibold uppercase tracking-wider text-muted-foreground">{title}</h3>
        {action}
      </div>
      <div className="space-y-2">{children}</div>
      <Separator className="mt-4" />
    </section>
  )
}

function Row({ label, children }: { label: string; children: ReactNode }) {
  return (
    <div className="flex items-start justify-between gap-4 text-sm">
      <span className="shrink-0 text-muted-foreground">{label}</span>
      <span className="min-w-0 text-right break-all tabular-nums">{children}</span>
    </div>
  )
}

type TestResult = { ok: boolean; text: string; ms: number }

function Body({ a, onClose }: { a: Account; onClose: () => void }) {
  const { t } = useI18n()
  const qc = useQueryClient()
  const invalidate = () => qc.invalidateQueries({ queryKey: ['accounts'] })
  const [busy, setBusy] = useState<string | null>(null)
  const run = async (key: string, fn: () => Promise<void>) => {
    setBusy(key)
    try {
      await fn()
    } catch (e) {
      toast.error(errorMessage(e))
    } finally {
      setBusy(null)
    }
  }

  // editable fields
  const [nickname, setNickname] = useState(a.nickname ?? '')
  const [weight, setWeight] = useState(String(a.weight ?? 0))
  const [probeModel, setProbeModel] = useState(a.probeModel ?? '')
  const [newModel, setNewModel] = useState('')
  const extraModels = a.extraModels ?? []
  const saveExtraModels = (list: string[]) =>
    run('extra', async () => {
      await put(`/accounts/${a.id}`, { extraModels: list })
      await post(`/accounts/${a.id}/models/refresh`).catch(() => undefined)
      toast.success(t('detail.saved'))
      invalidate()
      qc.invalidateQueries({ queryKey: ['account-models', a.id] })
    })
  const addModel = () => {
    const id = newModel.trim()
    if (!id) return
    if (extraModels.some((m) => m.toLowerCase() === id.toLowerCase())) return
    setNewModel('')
    void saveExtraModels([...extraModels, id])
  }
  const [machineId, setMachineId] = useState(a.machineId ?? '')
  const [egress, setEgress] = useState<'inherit' | 'proxy' | 'relay'>(a.relayURL ? 'relay' : a.proxyURL ? 'proxy' : 'inherit')
  const [proxyURL, setProxyURL] = useState(a.proxyURL ?? '')
  const [relayURL, setRelayURL] = useState(a.relayURL ?? '')
  const [relaySecret, setRelaySecret] = useState('')
  const [testModel, setTestModel] = useState<string | undefined>()
  const [testResult, setTestResult] = useState<TestResult | null>(null)
  const [confirmDelete, setConfirmDelete] = useState(false)

  const models = useQuery({
    queryKey: ['account-models', a.id],
    queryFn: () => get<{ models: string[] }>(`/accounts/${a.id}/models/cached`),
  })
  const modelList = models.data?.models ?? []
  const settings = useQuery({ queryKey: ['settings'], queryFn: () => get<{ testModel?: string }>('/settings') })
  const configuredTestModel = settings.data?.testModel?.trim() ?? ''
  const accountDefaultModel =
    modelList.find((model) => model.toLowerCase() === (a.probeModel ?? '').toLowerCase()) ??
    modelList.find((model) => model.toLowerCase() === configuredTestModel.toLowerCase())
  const selectedTestModel = testModel ?? accountDefaultModel ?? modelList[0] ?? ''

  const saveIdentity = () =>
    run('identity', async () => {
      if (machineId && !isMachineId(machineId)) throw new Error(t('detail.machineIdError'))
      await put(`/accounts/${a.id}`, { nickname, weight: Number(weight) || 0, machineId, probeModel })
      toast.success(t('detail.saved'))
      invalidate()
    })
  const generateMachineId = () =>
    run('gen', async () => {
      const r = await get<{ machineId: string }>('/generate-machine-id')
      setMachineId(r.machineId)
    })
  const setEnabled = (enabled: boolean) =>
    run('enabled', async () => {
      await put(`/accounts/${a.id}`, { enabled })
      invalidate()
    })
  const pullOverage = () =>
    run('overage', async () => {
      await get(`/accounts/${a.id}/overage`)
      invalidate()
    })
  const setOverage = (enabled: boolean) =>
    run('overage', async () => {
      await post(`/accounts/${a.id}/overage`, { enabled })
      invalidate()
    })
  const saveEgress = () =>
    run('egress', async () => {
      if (egress === 'inherit') await put(`/accounts/${a.id}`, { proxyURL: '', relayURL: '' })
      else if (egress === 'proxy') {
        if (!isProxyUrl(proxyURL)) throw new Error(t('detail.proxyFormatError'))
        await put(`/accounts/${a.id}`, { proxyURL })
      } else {
        if (!isHttpUrl(relayURL)) throw new Error(t('detail.relayFormatError'))
        await put(`/accounts/${a.id}`, { relayURL, relaySecret })
      }
      toast.success(t('detail.proxySaved'))
      setRelaySecret('')
      invalidate()
    })
  const refreshModelCache = () =>
    run('models', async () => {
      const r = await post<{ count: number }>(`/accounts/${a.id}/models/refresh`)
      toast.success(t('accounts.modelsRefreshed', r.count))
      qc.invalidateQueries({ queryKey: ['account-models', a.id] })
    })
  const refreshAccount = () =>
    run('refresh', async () => {
      await post(`/accounts/${a.id}/refresh`)
      toast.success(t('accounts.refreshed'))
      invalidate()
    })
  const warmup = () =>
    run('warmup', async () => {
      const r = await post<{ results: WarmupResult[] }>('/warmup', { ids: [a.id] })
      toast.success(t('accounts.warmup.result', ...warmupCounts(r.results)))
      invalidate()
    })
  const runTest = () =>
    run('test', async () => {
      const started = Date.now()
      try {
        const r = await post<{ reply: string; model: string }>(
          `/accounts/${a.id}/test`,
          selectedTestModel ? { model: selectedTestModel } : undefined,
        )
        setTestResult({ ok: true, text: `${r.model}: ${r.reply}`, ms: Date.now() - started })
      } catch (e) {
        setTestResult({ ok: false, text: errorMessage(e), ms: Date.now() - started })
      }
    })
  const copyJson = () =>
    run('copy', async () => {
      const full = await get(`/accounts/${a.id}/full`)
      await navigator.clipboard.writeText(JSON.stringify(full, null, 2))
      toast.success(t('accounts.copyJSONSuccess'))
    })
  const remove = async () => {
    await del(`/accounts/${a.id}`)
    toast.success(t('accounts.deleteSuccess'))
    invalidate()
    onClose()
  }

  const s = accountStatus(a)
  const spin = (k: string) => (busy === k ? <LuLoader className="animate-spin" /> : null)
  const overageCapable = a.overageCapability === 'OVERAGE_CAPABLE'
  const usd = (v: number | undefined) => `$${(v ?? 0).toFixed(2)}`

  return (
    <div className="flex flex-col gap-4 pb-6">
      <SheetHeader className="pr-10">
        <SheetTitle className="truncate">{a.nickname || a.email}</SheetTitle>
        <SheetDescription className="truncate">{a.email}</SheetDescription>
        <div className="mt-1 flex flex-wrap items-center gap-2">
          <StatusDot tone={statusTone[s]} label={t(statusKey[s])} />
          <Badge variant="secondary">{a.provider}</Badge>
          <Badge variant="outline">{subscriptionLabel(a, t)}</Badge>
          {a.overageStatus && <Badge variant="outline">{t(a.overageStatus === 'ENABLED' ? 'accounts.overageOn' : 'accounts.overageOff')}</Badge>}
          <div className="ml-auto flex items-center gap-2 text-xs">
            <span className="text-muted-foreground">{t('accounts.enabled')}</span>
            <Switch checked={a.enabled} onCheckedChange={(v) => setEnabled(v)} disabled={busy === 'enabled'} />
          </div>
        </div>
      </SheetHeader>

      <Section title={t('detail.basicInfo')}>
        <Row label={t('detail.userId')}>{a.userId || '—'}</Row>
        <Row label={t('detail.authMethod')}>{a.authMethod || '—'}</Row>
        <Row label={t('detail.region')}>{a.region || '—'}</Row>
        <Row label={t('detail.tokenExpiry')}>
          {a.expiresAt ? `${formatTime(a.expiresAt)} ${countdown(a.expiresAt) ? `(${countdown(a.expiresAt)})` : `(${t('accounts.expired')})`}` : t('accounts.noToken')}
        </Row>
        {a.banStatus && a.banStatus !== 'ACTIVE' && (
          <Alert variant="destructive">
            <AlertTitle>{a.banStatus}</AlertTitle>
            <AlertDescription>
              {a.banReason} {a.banTime ? `· ${formatTime(a.banTime)}` : ''}
            </AlertDescription>
          </Alert>
        )}
        <div className="grid grid-cols-1 gap-3 pt-1 sm:grid-cols-2">
          <div className="space-y-1">
            <Label htmlFor="nick">{t('accounts.nickname')}</Label>
            <Input id="nick" value={nickname} onChange={(e) => setNickname(e.target.value)} />
          </div>
          <div className="space-y-1">
            <Label htmlFor="weight">{t('detail.weight')}</Label>
            <Input id="weight" type="number" min={0} value={weight} onChange={(e) => setWeight(e.target.value)} />
            <p className="text-[11px] text-muted-foreground">{t('detail.weightHint')}</p>
          </div>
        </div>
        <div className="space-y-1">
          <Label htmlFor="probe-model">{t('detail.probeModel')}</Label>
          <select
            id="probe-model"
            value={probeModel}
            onChange={(e) => setProbeModel(e.target.value)}
            className="h-9 w-full rounded-md border border-input bg-background px-2 text-sm"
          >
            <option value="">{t('detail.probeModelDefault')}</option>
            {modelList.map((m) => (
              <option key={m} value={m}>
                {m}
              </option>
            ))}
          </select>
          <p className="text-[11px] text-muted-foreground">{t('detail.probeModelHint')}</p>
        </div>
        <div className="space-y-1">
          <Label htmlFor="mid">{t('detail.machineId')}</Label>
          <div className="flex gap-2">
            <Input id="mid" value={machineId} onChange={(e) => setMachineId(e.target.value)} className="font-mono text-xs" />
            <Button variant="outline" size="sm" onClick={generateMachineId} disabled={!!busy}>
              {spin('gen') ?? <LuWand />} {t('detail.generate')}
            </Button>
          </div>
        </div>
        <div className="flex justify-end">
          <Button size="sm" onClick={saveIdentity} disabled={!!busy}>
            {spin('identity')} {t('detail.save')}
          </Button>
        </div>
      </Section>

      <Section
        title={t('detail.usage')}
        action={
          <Button variant="ghost" size="sm" onClick={refreshAccount} disabled={!!busy}>
            {spin('refresh') ?? <LuRefreshCw />} {t('accounts.refresh')}
          </Button>
        }
      >
        <Row label={t('detail.subscriptionType')}>
          {subscriptionLabel(a, t)}
          {a.subscriptionTitle && a.subscriptionTitle !== subscriptionLabel(a, t) ? ` · ${a.subscriptionTitle}` : ''}
          {a.daysRemaining > 0 ? ` · ${a.daysRemaining}${t('time.days')}` : ''}
        </Row>
        <div className="space-y-1 pt-1">
          <div className="flex justify-between text-xs">
            <span>{t('detail.mainQuota')}</span>
            <span className="tabular-nums text-muted-foreground">
              {formatNumber(a.usageCurrent)} / {formatNumber(a.usageLimit)} ({pct(a.usagePercent)}%)
            </span>
          </div>
          <Progress value={pct(a.usagePercent)} className="gap-0" />
        </div>
        {a.trialUsageLimit > 0 && (
          <div className="space-y-1">
            <div className="flex justify-between text-xs">
              <span>{t('detail.trialQuota')}</span>
              <span className="tabular-nums text-muted-foreground">
                {formatNumber(a.trialUsageCurrent)} / {formatNumber(a.trialUsageLimit)} ({pct(a.trialUsagePercent)}%)
              </span>
            </div>
            <Progress value={pct(a.trialUsagePercent)} className="gap-0" />
            <Row label={t('detail.trialStatus')}>{a.trialStatus || '—'}</Row>
            <Row label={t('detail.trialExpiry')}>{formatTime(a.trialExpiresAt)}</Row>
          </div>
        )}
        <Row label={t('detail.resetDate')}>{a.nextResetDate || '—'}</Row>
        <Row label={t('accounts.lastRefresh')}>{formatTime(a.lastRefresh)}</Row>
      </Section>

      <Section
        title={t('accounts.warmup.label')}
        action={
          <Button variant="ghost" size="sm" onClick={warmup} disabled={!!busy}>
            {spin('warmup') ?? <LuFlame />} {t('accounts.warmup.runNow')}
          </Button>
        }
      >
        <Row label={t('accounts.warmup.last')}>{formatTime(a.lastWarmup)}</Row>
        <Row label={t('accounts.warmup.status')}>
          {a.warmupStatus ? (
            <StatusDot tone={a.warmupStatus === 'ok' ? 'success' : 'danger'} label={t(a.warmupStatus === 'ok' ? 'accounts.warmup.ok' : 'accounts.warmup.failed')} />
          ) : (
            t('accounts.warmup.never')
          )}
        </Row>
        {a.warmupError && <Row label={t('accounts.warmup.error')}>{a.warmupError}</Row>}
      </Section>

      {isKiroAccount(a) && (
      <Section
        title={t('detail.overage')}
        action={
          <Button variant="ghost" size="sm" onClick={pullOverage} disabled={!!busy}>
            {spin('overage') ?? <LuCloudDownload />} {t('detail.overageRefresh')}
          </Button>
        }
      >
        <p className="text-[11px] text-muted-foreground">{t('detail.overageHint')}</p>
        {!overageCapable && a.overageCapability && <p className="text-xs text-amber-600 dark:text-amber-400">{t('detail.overageNotCapable')}</p>}
        <div className="flex items-center justify-between text-sm">
          <span className="text-muted-foreground">{t('detail.overageStatus')}</span>
          <div className="flex items-center gap-2">
            <span className="text-xs">
              {a.overageStatus === 'ENABLED' ? t('accounts.overageOn') : a.overageStatus === 'DISABLED' ? t('accounts.overageOff') : t('detail.overageUnknown')}
            </span>
            <Switch checked={a.overageStatus === 'ENABLED'} onCheckedChange={(v) => setOverage(v)} disabled={!!busy || !overageCapable} />
          </div>
        </div>
        <Row label={t('detail.overageCap')}>{usd(a.overageCap)}</Row>
        <Row label={t('detail.overageRate')}>{usd(a.overageRate)}</Row>
        <Row label={t('detail.overageCurrent')}>{usd(a.currentOverages)}</Row>
        <Row label={t('detail.overageCheckedAt')}>{formatTime(a.overageCheckedAt)}</Row>
      </Section>
      )}

      <Section title={t('detail.proxyURL')}>
        <p className="text-[11px] text-muted-foreground">{t('detail.proxyHint')}</p>
        <SimpleSelect
          value={egress}
          onChange={(v) => setEgress(v as typeof egress)}
          className="w-full"
          options={[
            { value: 'inherit', label: t('detail.egressInherit') },
            { value: 'proxy', label: t('detail.egressProxy') },
            { value: 'relay', label: t('detail.egressRelay') },
          ]}
        />
        {egress === 'proxy' && (
          <Input value={proxyURL} onChange={(e) => setProxyURL(e.target.value)} placeholder="socks5://host:port" className="font-mono text-xs" />
        )}
        {egress === 'relay' && (
          <>
            <Input value={relayURL} onChange={(e) => setRelayURL(e.target.value)} placeholder="https://relay.example.com" className="font-mono text-xs" />
            <Input
              type="password"
              value={relaySecret}
              onChange={(e) => setRelaySecret(e.target.value)}
              placeholder={a.hasRelaySecret ? t('accounts.relaySecretStored') : t('accounts.relaySecret')}
            />
          </>
        )}
        <div className="flex justify-end">
          <Button size="sm" onClick={saveEgress} disabled={!!busy}>
            {spin('egress')} {t('detail.save')}
          </Button>
        </div>
      </Section>

      <Section title={t('detail.statistics')}>
        <div className="grid grid-cols-2 gap-2 sm:grid-cols-4">
          {[
            [t('detail.requestCount'), formatNumber(a.requestCount)],
            [t('detail.errorCount'), formatNumber(a.errorCount)],
            [t('detail.totalTokens'), formatNumber(a.totalTokens)],
            [t('detail.totalCredits'), (a.totalCredits ?? 0).toFixed(2)],
          ].map(([l, v]) => (
            <div key={l} className="rounded-md bg-muted/50 p-2">
              <div className="text-[11px] text-muted-foreground">{l}</div>
              <div className="text-base font-semibold tabular-nums">{v}</div>
            </div>
          ))}
        </div>
        <Row label={t('accounts.lastUsed')}>{formatTime(a.lastUsed)}</Row>
      </Section>

      <Section
        title={t('detail.models')}
        action={
          <Button variant="ghost" size="sm" onClick={refreshModelCache} disabled={!!busy}>
            {spin('models') ?? <LuRefreshCw />} {t('detail.refreshModelCache')}
          </Button>
        }
      >
        {models.isLoading ? (
          <p className="text-xs text-muted-foreground">{t('detail.loading')}</p>
        ) : modelList.length === 0 ? (
          <p className="text-xs text-muted-foreground">{t('detail.noModels')}</p>
        ) : (
          <div className="flex flex-wrap gap-1">
            {modelList.map((m) => {
              const manual = extraModels.some((x) => x.toLowerCase() === m.toLowerCase())
              return (
                <Badge key={m} variant={manual ? 'secondary' : 'outline'} className="font-mono">
                  {m}
                  {manual && (
                    <button
                      type="button"
                      className="ml-1 opacity-60 hover:opacity-100"
                      title={t('detail.removeModel')}
                      onClick={() => void saveExtraModels(extraModels.filter((x) => x.toLowerCase() !== m.toLowerCase()))}
                    >
                      <LuX className="size-3" />
                    </button>
                  )}
                </Badge>
              )
            })}
          </div>
        )}
        <div className="flex gap-2 pt-1">
          <Input
            value={newModel}
            onChange={(e) => setNewModel(e.target.value)}
            onKeyDown={(e) => e.key === 'Enter' && addModel()}
            placeholder={t('detail.addModelPlaceholder')}
            className="flex-1 font-mono text-xs"
          />
          <Button size="sm" variant="outline" onClick={addModel} disabled={!!busy || !newModel.trim()}>
            {spin('extra') ?? <LuPlus />} {t('detail.addModel')}
          </Button>
        </div>
        <p className="text-[11px] text-muted-foreground">{t('detail.addModelHint')}</p>
      </Section>

      <Section title={t('accounts.testModalTitle')}>
        <div className="flex gap-2">
          {modelList.length ? (
            <SimpleSelect
              value={selectedTestModel}
              onChange={setTestModel}
              className="flex-1"
              options={modelList.map((m) => ({ value: m, label: m }))}
            />
          ) : (
            <Input value={testModel} onChange={(e) => setTestModel(e.target.value)} placeholder="claude-sonnet-4" className="flex-1 font-mono text-xs" />
          )}
          <Button size="sm" onClick={runTest} disabled={!!busy}>
            {spin('test') ?? <LuFlaskConical />} {t('accounts.test')}
          </Button>
        </div>
        <div className="flex items-center justify-between gap-2 text-[11px] text-muted-foreground">
          <span>
            {a.probeModel ? t('detail.probeModelCurrent', a.probeModel) : t('detail.probeModelUnset')}
          </span>
          <Button
            size="sm"
            variant="outline"
            disabled={!!busy || !selectedTestModel || selectedTestModel === (a.probeModel ?? '')}
            onClick={() =>
              run('probe', async () => {
                await put(`/accounts/${a.id}`, { probeModel: selectedTestModel })
                setProbeModel(selectedTestModel)
                toast.success(t('detail.probeModelSaved', selectedTestModel))
                invalidate()
              })
            }
          >
            {spin('probe')} {t('detail.setAsDefault')}
          </Button>
        </div>
        {testResult && (
          <Alert variant={testResult.ok ? 'default' : 'destructive'}>
            <AlertTitle>
              {t(testResult.ok ? 'accounts.testSuccess' : 'accounts.testFailed')} · {(testResult.ms / 1000).toFixed(1)}s
            </AlertTitle>
            <AlertDescription className="break-all">{testResult.text}</AlertDescription>
          </Alert>
        )}
      </Section>

      <div className="flex items-center justify-between gap-2 px-4">
        <div className="flex items-center gap-1">
          <CopyButton value={a.id} size="sm" label={t('accounts.copyId')} />
          <Button variant="outline" size="sm" onClick={copyJson} disabled={!!busy}>
            {spin('copy')} {t('accounts.copyJSON')}
          </Button>
        </div>
        <Button variant="destructive" size="sm" onClick={() => setConfirmDelete(true)}>
          <LuTrash2 /> {t('accounts.delete')}
        </Button>
      </div>
      <ConfirmDialog
        open={confirmDelete}
        onOpenChange={setConfirmDelete}
        title={t('accounts.confirmDelete')}
        description={a.email}
        destructive
        confirmLabel={t('common.delete')}
        onConfirm={remove}
      />
    </div>
  )
}
