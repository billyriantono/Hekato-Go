import { useEffect, useState } from 'react'
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query'
import { LuCheck, LuLoader, LuX } from 'react-icons/lu'
import { toast } from 'sonner'
import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'
import { CopyButton, LoadingBlock, errorMessage } from '@/components/common'
import { get, post } from '@/lib/api'
import { useI18n } from '@/lib/i18n'
import { Field, SaveButton, Section } from './shared'

type Relay = { relayUrl: string; hasSecret: boolean }
type TestResult = { ok: boolean; status?: number; detail?: string; error?: string }
type Source = { platform: string; filename: string; language: string; code: string; secret: string }
type Platform = 'cloudflare' | 'vercel' | 'deno'
const PLATFORMS: Platform[] = ['cloudflare', 'vercel', 'deno']

export function RelaySection() {
  const { t } = useI18n()
  const qc = useQueryClient()
  const q = useQuery({ queryKey: ['relay'], queryFn: () => get<Relay>('/relay') })
  const [url, setUrl] = useState('')
  const [secret, setSecret] = useState('')
  useEffect(() => {
    if (q.data) setUrl(q.data.relayUrl)
  }, [q.data])
  const dirty = !!q.data && (url !== q.data.relayUrl || secret !== '')
  const badUrl = !!url && !/^https?:\/\//.test(url)

  const save = useMutation({
    mutationFn: () => post('/relay', { relayUrl: url.trim(), relaySecret: secret }),
    onSuccess: () => {
      toast.success(t('relay.saved'))
      setSecret('')
      qc.invalidateQueries({ queryKey: ['relay'] })
    },
    onError: (e) => toast.error(errorMessage(e, t('common.saveFailed'))),
  })

  const [result, setResult] = useState<TestResult>()
  const test = useMutation({
    mutationFn: () => post<TestResult>('/relay/test', { relayUrl: url.trim(), relaySecret: secret }),
    onSuccess: (r) => {
      setResult(r)
      if (r.ok) toast.success(t('relay.testOk'))
      else toast.error(t('relay.testFail'))
    },
    onError: (e) => {
      setResult({ ok: false, error: errorMessage(e) })
      toast.error(t('relay.testFail'))
    },
  })

  const [platform, setPlatform] = useState<Platform>()
  const src = useQuery({
    queryKey: ['relay-source', platform],
    queryFn: () => get<Source>('/relay/source?platform=' + platform),
    enabled: !!platform,
  })

  return (
    <Section
      id="relay"
      title={t('relay.settings')}
      description={t('relay.desc')}
      footer={
        <>
          <Button size="sm" variant="outline" onClick={() => test.mutate()} disabled={!url && !q.data?.relayUrl}>
            {test.isPending && <LuLoader className="size-3.5 animate-spin" />}
            {t('relay.test')}
          </Button>
          <SaveButton onClick={() => save.mutate()} disabled={!dirty || badUrl} busy={save.isPending} label={t('relay.save')} />
        </>
      }
    >
      {!q.data ? (
        <LoadingBlock />
      ) : (
        <>
          <div className="grid gap-4 sm:grid-cols-2">
            <Field label={t('relay.url')} htmlFor="relay-url" hint={badUrl ? t('settings.relayUrlInvalid') : undefined}>
              <Input id="relay-url" value={url} onChange={(e) => setUrl(e.target.value)} placeholder="https://relay.example.workers.dev" aria-invalid={badUrl} />
            </Field>
            <Field label={t('relay.secret')} htmlFor="relay-secret" hint={q.data.hasSecret ? t('relay.secretStored') : undefined}>
              <Input id="relay-secret" type="password" autoComplete="new-password" value={secret} onChange={(e) => setSecret(e.target.value)} placeholder={t('relay.secretPlaceholder')} />
            </Field>
          </div>
          {result && (
            <div className={'flex items-start gap-2 rounded-lg border p-3 text-xs ' + (result.ok ? 'border-emerald-500/40 text-emerald-700 dark:text-emerald-400' : 'border-destructive/40 text-destructive')}>
              {result.ok ? <LuCheck className="mt-0.5 size-3.5 shrink-0" /> : <LuX className="mt-0.5 size-3.5 shrink-0" />}
              <span>
                {result.ok ? t('relay.testOk') : t('relay.testFail')}
                {result.status ? ` (HTTP ${result.status})` : ''}
                {result.detail || result.error ? ` — ${result.detail || result.error}` : ''}
              </span>
            </div>
          )}

          <div className="space-y-3 rounded-lg border p-3">
            <div>
              <div className="text-sm font-medium">{t('relay.deploy')}</div>
              <div className="mt-0.5 text-xs text-muted-foreground">{t('settings.relayDeployHint')}</div>
            </div>
            <div className="flex flex-wrap gap-2">
              {PLATFORMS.map((p) => (
                <Button key={p} size="sm" variant={platform === p ? 'default' : 'outline'} onClick={() => setPlatform(p)}>
                  {t('relay.' + p)}
                </Button>
              ))}
            </div>
            {platform && src.isPending && <LoadingBlock />}
            {platform && src.error && <p className="text-xs text-destructive">{errorMessage(src.error)}</p>}
            {platform && src.data && (
              <div className="space-y-2">
                <p className="text-xs text-muted-foreground">{t('relay.steps.' + platform)}</p>
                <p className="text-xs text-muted-foreground">{t('relay.secretBaked')}</p>
                <div className="flex items-center justify-between gap-2">
                  <code className="text-xs">{src.data.filename}</code>
                  <CopyButton value={src.data.code} size="sm" label={t('common.copy')} />
                </div>
                <pre className="max-h-80 overflow-auto rounded-md bg-muted p-3 text-xs leading-relaxed">
                  <code>{src.data.code}</code>
                </pre>
              </div>
            )}
          </div>
        </>
      )}
    </Section>
  )
}
