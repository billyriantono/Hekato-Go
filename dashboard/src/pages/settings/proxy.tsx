import { useEffect, useState } from 'react'
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query'
import { toast } from 'sonner'
import { Input } from '@/components/ui/input'
import { Textarea } from '@/components/ui/textarea'
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from '@/components/ui/select'
import { LoadingBlock, errorMessage } from '@/components/common'
import { get, post } from '@/lib/api'
import { useI18n } from '@/lib/i18n'
import { Field, SaveButton, Section } from './shared'

type Proxy = { proxyURL: string; useRelay: boolean; proxyPool?: string[] }
type Mode = 'direct' | 'socks5' | 'http' | 'relay'
type Form = { mode: Mode; host: string; port: string; user: string; pass: string; pool: string }

const SCHEME_RE = /^(https?|socks5h?):\/\//

function toForm(p: Proxy): Form {
  const f: Form = { mode: 'direct', host: '', port: '', user: '', pass: '', pool: (p.proxyPool ?? []).join('\n') }
  if (p.useRelay) return { ...f, mode: 'relay' }
  if (!p.proxyURL) return f
  try {
    const u = new URL(p.proxyURL)
    return {
      ...f,
      mode: u.protocol.startsWith('socks') ? 'socks5' : 'http',
      host: u.hostname,
      port: u.port,
      user: decodeURIComponent(u.username),
      pass: decodeURIComponent(u.password),
    }
  } catch {
    return f
  }
}

function toProxyURL(f: Form): string {
  if (f.mode === 'direct' || f.mode === 'relay') return ''
  const auth = f.user ? encodeURIComponent(f.user) + (f.pass ? ':' + encodeURIComponent(f.pass) : '') + '@' : ''
  return `${f.mode}://${auth}${f.host}:${f.port}`
}

export function ProxySection() {
  const { t } = useI18n()
  const qc = useQueryClient()
  const q = useQuery({ queryKey: ['proxy'], queryFn: () => get<Proxy>('/proxy') })
  const [form, setForm] = useState<Form>()
  const [initial, setInitial] = useState('')
  useEffect(() => {
    if (!q.data) return
    const f = toForm(q.data)
    setForm(f)
    setInitial(JSON.stringify(f))
  }, [q.data])
  const patch = (p: Partial<Form>) => setForm((f) => (f ? { ...f, ...p } : f))
  const dirty = !!form && JSON.stringify(form) !== initial

  const pool = form?.pool.split('\n').map((s) => s.trim()).filter(Boolean) ?? []
  const badPool = pool.find((u) => !SCHEME_RE.test(u))
  const needHost = !!form && (form.mode === 'socks5' || form.mode === 'http') && (!form.host || !form.port)
  const error = badPool ? t('settings.proxyInvalidScheme') + ': ' + badPool : needHost ? t('settings.proxyHostRequired') : ''

  const save = useMutation({
    mutationFn: () => post('/proxy', { proxyURL: toProxyURL(form!), useRelay: form!.mode === 'relay', proxyPool: pool }),
    onSuccess: () => {
      toast.success(t('settings.proxySaved'))
      qc.invalidateQueries({ queryKey: ['proxy'] })
    },
    onError: (e) => toast.error(errorMessage(e, t('common.saveFailed'))),
  })

  const modes: Record<Mode, string> = {
    direct: t('settings.proxyNone'),
    socks5: t('settings.proxySocks5'),
    http: t('settings.proxyHttp'),
    relay: t('settings.proxyRelay'),
  }

  return (
    <Section
      id="egress"
      title={t('settings.proxySettings')}
      description={t('settings.egressHint')}
      footer={<SaveButton onClick={() => save.mutate()} disabled={!dirty || !!error} busy={save.isPending} label={t('settings.saveProxy')} />}
    >
      {!form ? (
        <LoadingBlock />
      ) : (
        <>
          <Field label={t('settings.proxyType')}>
            <Select items={modes} value={form.mode} onValueChange={(v) => patch({ mode: v as Mode })}>
              <SelectTrigger className="w-full sm:w-72">
                <SelectValue />
              </SelectTrigger>
              <SelectContent>
                {(Object.keys(modes) as Mode[]).map((m) => (
                  <SelectItem key={m} value={m}>
                    {modes[m]}
                  </SelectItem>
                ))}
              </SelectContent>
            </Select>
          </Field>
          {form.mode === 'relay' && <p className="text-xs text-muted-foreground">{t('settings.proxyRelayHint')}</p>}
          {(form.mode === 'socks5' || form.mode === 'http') && (
            <div className="grid gap-4 sm:grid-cols-2">
              <Field label={t('settings.proxyHost')} htmlFor="px-host">
                <div className="flex gap-2">
                  <Input id="px-host" value={form.host} onChange={(e) => patch({ host: e.target.value })} placeholder="127.0.0.1" />
                  <Input className="w-24" inputMode="numeric" value={form.port} onChange={(e) => patch({ port: e.target.value.replace(/\D/g, '') })} placeholder="1080" />
                </div>
              </Field>
              <Field label={t('settings.proxyAuth')} htmlFor="px-user">
                <div className="flex gap-2">
                  <Input id="px-user" value={form.user} onChange={(e) => patch({ user: e.target.value })} placeholder={t('settings.proxyUsername')} autoComplete="off" />
                  <Input type="password" value={form.pass} onChange={(e) => patch({ pass: e.target.value })} placeholder={t('settings.proxyPassword')} autoComplete="new-password" />
                </div>
              </Field>
            </div>
          )}
          <Field label={t('settings.proxyPool')} hint={t('settings.proxyPoolHint')} htmlFor="px-pool">
            <Textarea
              id="px-pool"
              className="font-mono text-xs"
              rows={4}
              value={form.pool}
              onChange={(e) => patch({ pool: e.target.value })}
              placeholder={'socks5://user:pass@10.0.0.1:1080\nhttp://10.0.0.2:8080'}
            />
          </Field>
          {error && <p className="text-xs text-destructive">{error}</p>}
        </>
      )}
    </Section>
  )
}
