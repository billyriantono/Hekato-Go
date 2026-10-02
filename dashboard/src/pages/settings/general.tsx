import { useState } from 'react'
import { Link } from '@tanstack/react-router'
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query'
import { toast } from 'sonner'
import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from '@/components/ui/select'
import { LoadingBlock, errorMessage } from '@/components/common'
import { get, post, setPassword } from '@/lib/api'
import { useI18n } from '@/lib/i18n'
import { Field, SaveButton, Section, SwitchRow, useDraft } from './shared'

type Settings = {
  requireApiKey: boolean
  allowOverUsage: boolean
  logLevel?: string
  accountRefreshMinutes?: number
  modelsDevSyncHours?: number
  warmupProbe?: boolean
  warmupRecover?: boolean
  testModel?: string
}
const LOG_LEVELS = ['debug', 'info', 'warn', 'error']

export function GeneralSection() {
  const { t } = useI18n()
  const qc = useQueryClient()
  const q = useQuery({ queryKey: ['settings'], queryFn: () => get<Settings>('/settings') })
  const { draft, patch, dirty } = useDraft(q.data)

  const save = useMutation({
    mutationFn: () =>
      post('/settings', {
        allowOverUsage: draft!.allowOverUsage,
        logLevel: draft!.logLevel ?? 'info',
        accountRefreshMinutes: draft!.accountRefreshMinutes ?? 0,
        modelsDevSyncHours: draft!.modelsDevSyncHours ?? 0,
        warmupProbe: !!draft!.warmupProbe,
        warmupRecover: !!draft!.warmupRecover,
        testModel: draft!.testModel ?? '',
      }),
    onSuccess: () => {
      toast.success(t('settings.generalSaved'))
      qc.invalidateQueries({ queryKey: ['settings'] })
    },
    onError: (e) => toast.error(errorMessage(e, t('common.saveFailed'))),
  })

  const [pw, setPw] = useState('')
  const [confirm, setConfirm] = useState('')
  const pwError = pw && pw.length < 8 ? t('settings.passwordTooShort') : confirm && pw !== confirm ? t('settings.passwordMismatch') : ''
  const changePw = useMutation({
    mutationFn: () => post('/settings', { password: pw }),
    onSuccess: () => {
      setPassword(pw, true)
      setPw('')
      setConfirm('')
      toast.success(t('settings.passwordChanged'))
    },
    onError: (e) => toast.error(errorMessage(e, t('common.saveFailed'))),
  })

  return (
    <>
      <Section
        id="general"
        title={t('settings.general')}
        description={t('settings.generalHint')}
        footer={<SaveButton onClick={() => save.mutate()} disabled={!dirty} busy={save.isPending} label={t('settings.saveGeneral')} />}
      >
        {!draft ? (
          <LoadingBlock />
        ) : (
          <>
            <Field label={t('settings.logLevel')} hint={t('settings.logLevelHint')}>
              <Select value={draft.logLevel ?? 'info'} onValueChange={(v) => patch({ logLevel: String(v) })}>
                <SelectTrigger className="w-44">
                  <SelectValue />
                </SelectTrigger>
                <SelectContent>
                  {LOG_LEVELS.map((l) => (
                    <SelectItem key={l} value={l}>
                      {l}
                    </SelectItem>
                  ))}
                </SelectContent>
              </Select>
            </Field>
            <Field label={t('settings.accountRefresh')} hint={t('settings.accountRefreshHint')} htmlFor="refresh-min">
              <Input
                id="refresh-min"
                type="number"
                min={0}
                max={1440}
                step={1}
                className="w-44"
                value={draft.accountRefreshMinutes ?? 0}
                onChange={(e) => patch({ accountRefreshMinutes: Math.max(0, Math.min(1440, parseInt(e.target.value, 10) || 0)) })}
              />
            </Field>
            <Field label={t('settings.modelsDevSync')} hint={t('settings.modelsDevSyncHint')} htmlFor="modelsdev-hours">
              <Input
                id="modelsdev-hours"
                type="number"
                min={-1}
                max={168}
                step={1}
                className="w-44"
                value={draft.modelsDevSyncHours ?? 0}
                onChange={(e) => patch({ modelsDevSyncHours: Math.max(-1, Math.min(168, parseInt(e.target.value, 10) || 0)) })}
              />
            </Field>
            <Field label={t('settings.testModel')} hint={t('settings.testModelHint')} htmlFor="test-model">
              <Input
                id="test-model"
                className="w-full font-mono sm:w-96"
                value={draft.testModel ?? ''}
                onChange={(e) => patch({ testModel: e.target.value })}
                placeholder={t('settings.testModelPlaceholder')}
              />
            </Field>
            <div className="space-y-2">
              <div className="text-sm font-medium">{t('settings.warmup.title')}</div>
              <p className="text-xs text-muted-foreground">{t('settings.warmup.hint')}</p>
              <SwitchRow
                label={t('settings.warmup.probe')}
                hint={t('settings.warmup.probeHint')}
                checked={!!draft.warmupProbe}
                onChange={(v) => patch({ warmupProbe: v })}
              />
              <SwitchRow
                label={t('settings.warmup.recover')}
                hint={t('settings.warmup.recoverHint')}
                checked={!!draft.warmupRecover}
                onChange={(v) => patch({ warmupRecover: v })}
              />
            </div>
            <SwitchRow
              label={t('settings.allowOverUsage')}
              hint={t('settings.allowOverUsageHint')}
              checked={draft.allowOverUsage}
              onChange={(v) => patch({ allowOverUsage: v })}
            />
            <div className="flex flex-wrap items-center justify-between gap-2 rounded-lg border border-dashed p-3 text-xs text-muted-foreground">
              <span>
                {t('settings.enableApiKey')}: {draft.requireApiKey ? t('common.enabled') : t('common.disabled')} — {t('settings.requireApiKeyHint')}
              </span>
              <Link to="/api-keys" className="font-medium text-foreground underline-offset-4 hover:underline">
                {t('nav.apiKeys')}
              </Link>
            </div>
          </>
        )}
      </Section>

      <Section
        id="password"
        title={t('settings.adminPassword')}
        footer={
          <Button size="sm" onClick={() => changePw.mutate()} disabled={!pw || !confirm || !!pwError || changePw.isPending}>
            {t('settings.changePassword')}
          </Button>
        }
      >
        <div className="grid gap-4 sm:grid-cols-2">
          <Field label={t('settings.newPassword')} htmlFor="new-pw">
            <Input id="new-pw" type="password" autoComplete="new-password" value={pw} onChange={(e) => setPw(e.target.value)} placeholder={t('settings.newPasswordPlaceholder')} />
          </Field>
          <Field label={t('settings.confirmPassword')} htmlFor="confirm-pw">
            <Input id="confirm-pw" type="password" autoComplete="new-password" value={confirm} onChange={(e) => setConfirm(e.target.value)} />
          </Field>
        </div>
        {pwError && <p className="text-xs text-destructive">{pwError}</p>}
      </Section>
    </>
  )
}
