// Add Account dialog: method picker + one form component per auth flow (docs/admin-api.md section 4).
import { useEffect, useRef, useState, type ReactNode } from 'react'
import { useQueryClient } from '@tanstack/react-query'
import { LuArrowLeft, LuExternalLink, LuKey, LuLoader, LuMonitor, LuShieldCheck } from 'react-icons/lu'
import { toast } from 'sonner'
import { Alert, AlertDescription } from '@/components/ui/alert'
import { Button } from '@/components/ui/button'
import { Dialog, DialogContent, DialogDescription, DialogFooter, DialogHeader, DialogTitle } from '@/components/ui/dialog'
import { Input } from '@/components/ui/input'
import { Label } from '@/components/ui/label'
import { Textarea } from '@/components/ui/textarea'
import { Badge } from '@/components/ui/badge'
import { CopyButton, errorMessage } from '@/components/common'
import { api, post } from '@/lib/api'
import { useI18n } from '@/lib/i18n'
import { sleep } from './shared'
import { SimpleSelect } from './simple-select'

export type Method = 'builderid' | 'iam' | 'kirosso' | 'ssotoken' | 'local' | 'credentials' | 'cookie' | 'codebuddy' | 'grokDevice' | 'grokImport' | 'codexImport' | 'clinepassImport' | 'opencodeZenImport' | 'opencodeGoImport' | 'commandCodeImport' | 'openaiCompat' | 'anthropicCompat'

type Added = { id: string; email?: string }
type FormProps = { onDone: (accounts: Added[]) => void }

const METHODS: { id: Method; provider: string; title: string; desc: string }[] = [
  { id: 'builderid', provider: 'modal.kiroProvider', title: 'modal.builderIdTitle', desc: 'modal.builderIdDesc' },
  { id: 'iam', provider: 'modal.kiroProvider', title: 'modal.iamTitle', desc: 'modal.iamDesc' },
  { id: 'kirosso', provider: 'modal.kiroProvider', title: 'modal.enterpriseSsoTitle', desc: 'modal.enterpriseSsoDesc' },
  { id: 'ssotoken', provider: 'modal.kiroProvider', title: 'modal.ssoTitle', desc: 'modal.ssoDesc' },
  { id: 'local', provider: 'modal.kiroProvider', title: 'modal.localTitle', desc: 'modal.localDesc' },
  { id: 'credentials', provider: 'modal.kiroProvider', title: 'modal.credentialsTitle', desc: 'modal.credentialsDesc' },
  { id: 'cookie', provider: 'modal.kiroProvider', title: 'modal.cookieTitle', desc: 'modal.cookieDesc' },
  { id: 'codebuddy', provider: 'modal.codebuddyProvider', title: 'modal.codebuddyTitle', desc: 'modal.codebuddyDesc' },
  { id: 'grokDevice', provider: 'modal.grokProvider', title: 'grok.deviceLogin', desc: 'modal.grokProviderDesc' },
  { id: 'grokImport', provider: 'modal.grokProvider', title: 'grok.importTokens', desc: 'grok.importHint' },
  { id: 'codexImport', provider: 'modal.codexProvider', title: 'modal.codexImportTitle', desc: 'modal.codexImportDesc' },
  { id: 'clinepassImport', provider: 'modal.clinepassProvider', title: 'modal.clinepassImportTitle', desc: 'modal.clinepassImportDesc' },
  { id: 'opencodeZenImport', provider: 'modal.opencodeZenProvider', title: 'modal.opencodeZenImportTitle', desc: 'modal.opencodeZenImportDesc' },
  { id: 'opencodeGoImport', provider: 'modal.opencodeGoProvider', title: 'modal.opencodeGoImportTitle', desc: 'modal.opencodeGoImportDesc' },
  { id: 'commandCodeImport', provider: 'modal.commandCodeProvider', title: 'modal.commandCodeImportTitle', desc: 'modal.commandCodeImportDesc' },
  { id: 'openaiCompat', provider: 'modal.compatProvider', title: 'modal.openaiCompatTitle', desc: 'modal.openaiCompatDesc' },
  { id: 'anthropicCompat', provider: 'modal.compatProvider', title: 'modal.anthropicCompatTitle', desc: 'modal.anthropicCompatDesc' },
]

// Well-known gateways / vendors per wire protocol. OpenAI-style bases include
// the /v1 segment (the gateway appends /chat/completions); Anthropic-style
// bases are the vendor root (the gateway appends /v1/messages).
const COMPAT_PRESETS: Record<'openai_compat' | 'anthropic_compat', { name: string; url: string }[]> = {
  openai_compat: [
    { name: 'Vercel AI Gateway', url: 'https://ai-gateway.vercel.sh/v1' },
    { name: 'OpenRouter', url: 'https://openrouter.ai/api/v1' },
    { name: 'OpenAI', url: 'https://api.openai.com/v1' },
    { name: 'Groq', url: 'https://api.groq.com/openai/v1' },
    { name: 'DeepSeek', url: 'https://api.deepseek.com/v1' },
    { name: 'Z.ai', url: 'https://api.z.ai/api/paas/v4' },
  ],
  anthropic_compat: [
    { name: 'Anthropic', url: 'https://api.anthropic.com' },
    { name: 'Vercel AI Gateway', url: 'https://ai-gateway.vercel.sh' },
    { name: 'Z.ai', url: 'https://api.z.ai/api/anthropic' },
    { name: 'MiniMax', url: 'https://api.minimax.io/anthropic' },
    { name: 'Moonshot', url: 'https://api.moonshot.ai/anthropic' },
  ],
}

export function AddAccountDialog({ open, initialMethod, onClose }: { open: boolean; initialMethod: Method | null; onClose: () => void }) {
  const { t } = useI18n()
  const qc = useQueryClient()
  const [method, setMethod] = useState<Method | null>(initialMethod)
  // Reset to the requested method every time the dialog opens (derived-state pattern, no effect).
  const [wasOpen, setWasOpen] = useState(open)
  if (open !== wasOpen) {
    setWasOpen(open)
    if (open) setMethod(initialMethod)
  }

  const onDone = (accounts: Added[]) => {
    qc.invalidateQueries({ queryKey: ['accounts'] })
    // Warm up usage/subscription info for the new accounts; failures are non-fatal.
    for (const a of accounts) post(`/accounts/${a.id}/refresh`).catch(() => {}).finally(() => qc.invalidateQueries({ queryKey: ['accounts'] }))
    onClose()
  }
  const meta = METHODS.find((m) => m.id === method)

  return (
    <Dialog open={open} onOpenChange={(o) => !o && onClose()}>
      <DialogContent className="sm:max-w-xl">
        <DialogHeader>
          <div className="flex items-center gap-2">
            {method && (
              <Button variant="ghost" size="icon-sm" onClick={() => setMethod(null)} aria-label={t('common.back')}>
                <LuArrowLeft />
              </Button>
            )}
            <DialogTitle>{meta ? t(meta.title) : t('modal.addAccount')}</DialogTitle>
          </div>
          <DialogDescription>{meta ? t(meta.desc) : t('modal.chooseProviderHint')}</DialogDescription>
        </DialogHeader>
        {!method ? (
          <MethodPicker onPick={setMethod} />
        ) : method === 'builderid' ? (
          <DeviceCodeForm key="b" startPath="/auth/builderid/start" pollPath="/auth/builderid/poll" withRegion onDone={onDone} />
        ) : method === 'grokDevice' ? (
          <DeviceCodeForm key="g" startPath="/auth/grok/start" pollPath="/auth/grok/poll" onDone={onDone} />
        ) : method === 'iam' ? (
          <IamSsoForm onDone={onDone} />
        ) : method === 'kirosso' ? (
          <KiroSsoForm onDone={onDone} />
        ) : method === 'ssotoken' ? (
          <SsoTokenForm onDone={onDone} />
        ) : method === 'local' ? (
          <LocalCacheForm onDone={onDone} />
        ) : method === 'credentials' ? (
          <CredentialsForm onDone={onDone} />
        ) : method === 'cookie' ? (
          <CookieForm onDone={onDone} />
        ) : method === 'codebuddy' ? (
          <CodeBuddyForm onDone={onDone} />
        ) : method === 'codexImport' ? (
          <CodexImportForm onDone={onDone} />
        ) : method === 'clinepassImport' ? (
          <ClinepassImportForm onDone={onDone} />
        ) : method === 'opencodeZenImport' ? (
          <OpenCodeTokenImportForm key="zen" provider="opencode_zen" route="/auth/opencodezen/import" successKey="opencodezen.importSuccess" tokenLabel="opencodezen.tokensLabel" hintKey="opencodezen.importHint" optional onDone={onDone} />
        ) : method === 'opencodeGoImport' ? (
          <OpenCodeTokenImportForm key="go" provider="opencode_go" route="/auth/opencodego/import" successKey="opencodego.importSuccess" tokenLabel="opencodego.tokensLabel" hintKey="opencodego.importHint" onDone={onDone} />
        ) : method === 'commandCodeImport' ? (
          <OpenCodeTokenImportForm key="cc" provider="commandcode" route="/auth/commandcode/import" successKey="commandcode.importSuccess" tokenLabel="commandcode.tokensLabel" hintKey="commandcode.importHint" onDone={onDone} />
        ) : method === 'openaiCompat' ? (
          <CompatForm key="oc" protocol="openai_compat" onDone={onDone} />
        ) : method === 'anthropicCompat' ? (
          <CompatForm key="ac" protocol="anthropic_compat" onDone={onDone} />
        ) : (
          <GrokImportForm onDone={onDone} />
        )}
      </DialogContent>
    </Dialog>
  )
}

function MethodPicker({ onPick }: { onPick: (m: Method) => void }) {
  const { t } = useI18n()
  const providers = [...new Set(METHODS.map((m) => m.provider))]
  const icon: Record<string, ReactNode> = {
    'modal.kiroProvider': <LuShieldCheck className="size-4" />,
    'modal.codebuddyProvider': <LuKey className="size-4" />,
    'modal.grokProvider': <LuMonitor className="size-4" />,
    'modal.codexProvider': <LuKey className="size-4" />,
    'modal.clinepassProvider': <LuKey className="size-4" />,
    'modal.compatProvider': <LuKey className="size-4" />,
    'modal.commandCodeProvider': <LuKey className="size-4" />,
  }
  return (
    <div className="max-h-[60vh] space-y-4 overflow-y-auto pr-1">
      {providers.map((p) => (
        <div key={p}>
          <div className="mb-2 flex items-center gap-2 text-xs font-semibold uppercase tracking-wider text-muted-foreground">
            {icon[p]} {t(p)}
          </div>
          <div className="grid gap-2 sm:grid-cols-2">
            {METHODS.filter((m) => m.provider === p).map((m) => (
              <button
                key={m.id}
                type="button"
                onClick={() => onPick(m.id)}
                className="rounded-lg border p-3 text-left transition-colors hover:border-primary/50 hover:bg-muted/60 focus-visible:ring-3 focus-visible:ring-ring/50 focus-visible:outline-none"
              >
                <div className="text-sm font-medium">{t(m.title)}</div>
                <div className="mt-0.5 line-clamp-2 text-xs text-muted-foreground">{t(m.desc)}</div>
              </button>
            ))}
          </div>
        </div>
      ))}
    </div>
  )
}

// ---- helpers ---------------------------------------------------------------

/** Tracks mount state so polling loops stop after close/back. */
function useAlive() {
  const alive = useRef(true)
  useEffect(() => {
    alive.current = true
    return () => {
      alive.current = false
    }
  }, [])
  return alive
}

function useSubmit() {
  const [busy, setBusy] = useState(false)
  const submit = async (fn: () => Promise<void>) => {
    setBusy(true)
    try {
      await fn()
    } catch (e) {
      toast.error(errorMessage(e))
    } finally {
      setBusy(false)
    }
  }
  return { busy, submit }
}

type PollResult = { completed: boolean; status?: string; interval?: number; account?: Added & { authMethod?: string } }

async function pollUntilDone(path: string, sessionId: string, interval: number, alive: { current: boolean }): Promise<PollResult> {
  for (;;) {
    await sleep(Math.max(1, interval) * 1000)
    if (!alive.current) throw new Error('cancelled')
    const r = await post<PollResult>(path, { sessionId })
    if (r.completed) return r
    if (r.interval) interval = r.interval
  }
}

function Field({ label, hint, children }: { label: string; hint?: string; children: ReactNode }) {
  return (
    <div className="space-y-1">
      <Label>{label}</Label>
      {children}
      {hint && <p className="text-[11px] text-muted-foreground">{hint}</p>}
    </div>
  )
}

function FileInput({ onText }: { onText: (s: string) => void }) {
  return (
    <Input
      type="file"
      accept=".json,application/json,text/plain"
      className="text-xs"
      onChange={(e) => {
        const f = e.target.files?.[0]
        if (f) f.text().then(onText)
      }}
    />
  )
}

function OpenLink({ url }: { url: string }) {
  const { t } = useI18n()
  return (
    <div className="flex items-center gap-1">
      <Input readOnly value={url} className="flex-1 font-mono text-xs" />
      <CopyButton value={url} />
      <Button variant="outline" size="sm" onClick={() => window.open(url, '_blank', 'noopener')}>
        <LuExternalLink /> {t('builderid.open')}
      </Button>
    </div>
  )
}

function Waiting({ text }: { text: string }) {
  return (
    <div className="flex items-center gap-2 text-sm text-muted-foreground">
      <LuLoader className="size-4 animate-spin" /> {text}
    </div>
  )
}

function SubmitRow({ busy, label, onClick, disabled }: { busy: boolean; label: string; onClick: () => void; disabled?: boolean }) {
  return (
    <DialogFooter>
      <Button onClick={onClick} disabled={busy || disabled}>
        {busy && <LuLoader className="animate-spin" />} {label}
      </Button>
    </DialogFooter>
  )
}

const REGION = 'us-east-1'

// ---- 4.2 Builder ID / 4.7 Grok device code -----------------------------------

function DeviceCodeForm({ startPath, pollPath, withRegion, onDone }: FormProps & { startPath: string; pollPath: string; withRegion?: boolean }) {
  const { t } = useI18n()
  const alive = useAlive()
  const { busy, submit } = useSubmit()
  const [region, setRegion] = useState(REGION)
  const [session, setSession] = useState<{ sessionId: string; userCode: string; verificationUri: string; interval: number } | null>(null)

  const start = () =>
    submit(async () => {
      const s = await post<NonNullable<typeof session>>(startPath, withRegion ? { region } : undefined)
      setSession(s)
      window.open(s.verificationUri, '_blank', 'noopener')
      try {
        const r = await pollUntilDone(pollPath, s.sessionId, s.interval || 5, alive)
        toast.success(t('builderid.success'))
        if (r.account) onDone([r.account])
      } catch (e) {
        if (alive.current) throw e
      }
    })

  if (!session)
    return (
      <>
        {withRegion && (
          <Field label={t('detail.region')}>
            <Input value={region} onChange={(e) => setRegion(e.target.value)} />
          </Field>
        )}
        <SubmitRow busy={busy} label={t('builderid.startLogin')} onClick={start} />
      </>
    )
  return (
    <div className="space-y-4">
      <div className="rounded-lg border bg-muted/40 p-4 text-center">
        <div className="text-[11px] uppercase tracking-wider text-muted-foreground">{t('builderid.verifyCode')}</div>
        <div className="mt-1 flex items-center justify-center gap-2 font-mono text-2xl font-semibold tracking-widest">
          {session.userCode}
          <CopyButton value={session.userCode} />
        </div>
      </div>
      <Field label={t('builderid.verifyUrl')}>
        <OpenLink url={session.verificationUri} />
      </Field>
      <Waiting text={t('builderid.waiting')} />
    </div>
  )
}

// ---- 4.3 IAM Identity Center --------------------------------------------------

function IamSsoForm({ onDone }: FormProps) {
  const { t } = useI18n()
  const { busy, submit } = useSubmit()
  const [startUrl, setStartUrl] = useState('')
  const [region, setRegion] = useState(REGION)
  const [session, setSession] = useState<{ sessionId: string; authorizeUrl: string } | null>(null)
  const [callbackUrl, setCallbackUrl] = useState('')

  const start = () =>
    submit(async () => {
      const s = await post<NonNullable<typeof session>>('/auth/iam-sso/start', { startUrl: startUrl.trim(), region })
      setSession(s)
      window.open(s.authorizeUrl, '_blank', 'noopener')
    })
  const complete = () =>
    submit(async () => {
      const r = await post<{ account: Added }>('/auth/iam-sso/complete', { sessionId: session!.sessionId, callbackUrl: callbackUrl.trim() })
      toast.success(t('builderid.success'))
      onDone([r.account])
    })

  if (!session)
    return (
      <>
        <Field label={t('iam.startUrl')}>
          <Input value={startUrl} onChange={(e) => setStartUrl(e.target.value)} placeholder="https://d-xxxxxxxxxx.awsapps.com/start" />
        </Field>
        <Field label={t('detail.region')}>
          <Input value={region} onChange={(e) => setRegion(e.target.value)} />
        </Field>
        <SubmitRow busy={busy} label={t('builderid.startLogin')} onClick={start} disabled={!startUrl.trim()} />
      </>
    )
  return (
    <>
      <Field label={t('iam.loginUrl')} hint={t('iam.completeLogin')}>
        <OpenLink url={session.authorizeUrl} />
      </Field>
      <Field label={t('iam.callbackUrl')}>
        <Textarea value={callbackUrl} onChange={(e) => setCallbackUrl(e.target.value)} rows={3} className="font-mono text-xs" placeholder="http://127.0.0.1/oauth/callback?code=…&state=…" />
      </Field>
      <SubmitRow busy={busy} label={t('iam.complete')} onClick={complete} disabled={!callbackUrl.trim()} />
    </>
  )
}

// ---- 4.4 Kiro hosted SSO ------------------------------------------------------

function KiroSsoForm({ onDone }: FormProps) {
  const { t } = useI18n()
  const alive = useAlive()
  const { busy, submit } = useSubmit()
  const [session, setSession] = useState<{ sessionId: string; signInUrl: string; interval: number } | null>(null)
  const [relayUrl, setRelayUrl] = useState('')
  const [relayBusy, setRelayBusy] = useState(false)
  const [relayNote, setRelayNote] = useState('')
  const done = useRef(false)

  // Free the loopback port if the operator closes/leaves before completion.
  useEffect(() => {
    return () => {
      if (session && !done.current) post('/auth/kiro-sso/cancel', { sessionId: session.sessionId }).catch(() => {})
    }
  }, [session])

  const start = () =>
    submit(async () => {
      const s = await post<NonNullable<typeof session>>('/auth/kiro-sso/start', {})
      setSession(s)
      window.open(s.signInUrl, '_blank', 'noopener')
      try {
        const r = await pollUntilDone('/auth/kiro-sso/poll', s.sessionId, s.interval || 2, alive)
        done.current = true
        toast.success(t('builderid.success'))
        if (r.account) onDone([r.account])
      } catch (e) {
        if (alive.current) throw e
      }
    })
  const relay = async () => {
    if (!session || !relayUrl.trim()) return
    setRelayBusy(true)
    try {
      const r = await post<{ done: boolean; authorizeUrl?: string }>('/auth/kiro-sso/relay', { sessionId: session.sessionId, url: relayUrl.trim() })
      setRelayUrl('')
      if (r.authorizeUrl) {
        window.open(r.authorizeUrl, '_blank', 'noopener')
        setRelayNote(t('kirosso.relayContinue'))
      } else setRelayNote(r.done ? t('accounts.relayDone') : t('kirosso.relayContinue'))
    } catch (e) {
      toast.error(errorMessage(e))
    } finally {
      setRelayBusy(false)
    }
  }

  if (!session)
    return (
      <>
        <Alert>
          <AlertDescription>{t('kirosso.hostNote')}</AlertDescription>
        </Alert>
        <SubmitRow busy={busy} label={t('builderid.startLogin')} onClick={start} />
      </>
    )
  return (
    <div className="space-y-4">
      <p className="text-xs text-muted-foreground">{t('kirosso.openInstruction')}</p>
      <OpenLink url={session.signInUrl} />
      <Waiting text={t('builderid.waiting')} />
      <div className="space-y-2 rounded-lg border p-3">
        <div className="text-sm font-medium">{t('kirosso.relayLabel')}</div>
        <p className="text-[11px] text-muted-foreground">{t('kirosso.relayNote')}</p>
        <div className="flex gap-2">
          <Input value={relayUrl} onChange={(e) => setRelayUrl(e.target.value)} placeholder="http://127.0.0.1:3128/…" className="font-mono text-xs" />
          <Button variant="outline" size="sm" onClick={relay} disabled={relayBusy || !relayUrl.trim()}>
            {relayBusy && <LuLoader className="animate-spin" />} {t('kirosso.relaySubmit')}
          </Button>
        </div>
        {relayNote && <p className="text-xs text-emerald-600 dark:text-emerald-400">{relayNote}</p>}
      </div>
    </div>
  )
}

// ---- 4.5 SSO token --------------------------------------------------------------

function SsoTokenForm({ onDone }: FormProps) {
  const { t } = useI18n()
  const { busy, submit } = useSubmit()
  const [tokens, setTokens] = useState('')
  const [region, setRegion] = useState(REGION)
  const go = () =>
    submit(async () => {
      const r = await post<{ accounts: Added[]; errors?: string[] }>('/auth/sso-token', { bearerToken: tokens.trim(), region })
      toast.success(t('sso.importSuccess', r.accounts.length) + (r.errors?.length ? t('sso.importPartial', r.errors.length) : ''))
      onDone(r.accounts)
    })
  return (
    <>
      <Alert>
        <AlertDescription>
          {t('sso.step1')} <a href="https://view.awsapps.com/start" target="_blank" rel="noreferrer">view.awsapps.com/start</a> → {t('sso.step2')} → {t('sso.step3')}{' '}
          <code>{t('sso.tokenLabel')}</code>
        </AlertDescription>
      </Alert>
      <Field label={t('sso.tokenLabel')} hint={t('sso.tokenHint')}>
        <Textarea value={tokens} onChange={(e) => setTokens(e.target.value)} rows={5} placeholder={t('sso.tokenPlaceholder')} className="font-mono text-xs" />
      </Field>
      <Field label={t('detail.region')}>
        <Input value={region} onChange={(e) => setRegion(e.target.value)} />
      </Field>
      <SubmitRow busy={busy} label={t('common.add')} onClick={go} disabled={!tokens.trim()} />
    </>
  )
}

// ---- 4.1 credential import variants --------------------------------------------

type Cred = Record<string, unknown>

/** Accepts object, array, KAM export {accounts:[{credentials:{…}}]} or `a----b----refreshToken----clientId----clientSecret` lines. */
function parseCredentials(text: string): { items: Cred[]; skipped: number } {
  const trimmed = text.trim()
  let parsed: unknown
  try {
    parsed = JSON.parse(trimmed)
  } catch {
    let skipped = 0
    const items: Cred[] = []
    for (const line of trimmed.split(/\r?\n/).map((l) => l.trim()).filter(Boolean)) {
      const parts = line.includes('----') ? line.split('----') : line.split(/\s+/)
      if (parts.length < 3) {
        skipped++
        continue
      }
      // Preserve the legacy 5-field layout; tolerate 3-field (refreshToken----clientId----clientSecret).
      const [email, , refreshToken, clientId, clientSecret] = parts.length >= 5 ? parts : ['', '', ...parts]
      items.push({ email, refreshToken, clientId: clientId ?? '', clientSecret: clientSecret ?? '' })
    }
    return { items, skipped }
  }
  const raw = Array.isArray(parsed) ? parsed : parsed && typeof parsed === 'object' && Array.isArray((parsed as Cred).accounts) ? ((parsed as Cred).accounts as unknown[]) : [parsed]
  const items = raw
    .filter((x): x is Cred => !!x && typeof x === 'object')
    .map((x) => {
      const creds = x.credentials && typeof x.credentials === 'object' ? (x.credentials as Cred) : {}
      const item: Cred = { ...x, ...creds }
      delete item.credentials
      if (!item.authMethod && item.tokenEndpoint) item.authMethod = 'external_idp'
      if (!item.provider && x.idp) item.provider = x.idp
      return item
    })
  return { items, skipped: 0 }
}

async function importCredentials(items: Cred[]): Promise<{ added: Added[]; errors: string[] }> {
  const added: Added[] = []
  const errors: string[] = []
  for (const item of items) {
    try {
      const r = await post<{ account: Added }>('/auth/credentials', item)
      added.push(r.account)
    } catch (e) {
      errors.push(errorMessage(e))
    }
  }
  return { added, errors }
}

function useCredentialImport(onDone: (a: Added[]) => void) {
  const { t } = useI18n()
  const { busy, submit } = useSubmit()
  const go = (items: Cred[], skipped = 0) =>
    submit(async () => {
      if (!items.length) throw new Error(skipped ? t('credentials.lineParseAllSkipped', skipped) : t('credentials.jsonError'))
      const { added, errors } = await importCredentials(items)
      if (!added.length) throw new Error(errors.join('; '))
      toast.success(t('sso.importSuccess', added.length) + (errors.length ? t('sso.importPartial', errors.length) : '') + (skipped ? t('credentials.lineParseSkipped', skipped) : ''))
      if (errors.length) toast.warning(errors.join('\n'))
      onDone(added)
    })
  return { busy, go }
}

function CredentialsForm({ onDone }: FormProps) {
  const { t } = useI18n()
  const { busy, go } = useCredentialImport(onDone)
  const [text, setText] = useState('')
  return (
    <>
      <Field label={t('credentials.label')} hint={t('credentials.batchHint')}>
        <Textarea value={text} onChange={(e) => setText(e.target.value)} rows={8} className="font-mono text-xs" placeholder='{"refreshToken": "…", "clientId": "…", "clientSecret": "…"}' />
      </Field>
      <FileInput onText={setText} />
      <SubmitRow
        busy={busy}
        label={t('accounts.import')}
        disabled={!text.trim()}
        onClick={() => {
          const { items, skipped } = parseCredentials(text)
          go(items, skipped)
        }}
      />
    </>
  )
}

function LocalCacheForm({ onDone }: FormProps) {
  const { t } = useI18n()
  const { busy, go } = useCredentialImport(onDone)
  const [channel, setChannel] = useState('BuilderId')
  const [tokenJson, setTokenJson] = useState('')
  const [clientJson, setClientJson] = useState('')
  const idc = channel === 'BuilderId' || channel === 'Enterprise'

  const parse = (): Cred[] => {
    let tok: Cred
    try {
      tok = JSON.parse(tokenJson)
    } catch {
      throw new Error(t('local.tokenInvalid'))
    }
    if (!tok.refreshToken) throw new Error(t('local.refreshTokenMissing'))
    let client: Cred = {}
    if (idc) {
      if (!clientJson.trim()) throw new Error(t('local.clientMissing'))
      try {
        client = JSON.parse(clientJson)
      } catch {
        throw new Error(t('local.clientInvalid'))
      }
      if (!client.clientId || !client.clientSecret) throw new Error(t('local.clientSecretMissing'))
    }
    return [
      {
        refreshToken: tok.refreshToken,
        accessToken: tok.accessToken ?? '',
        clientId: client.clientId ?? '',
        clientSecret: client.clientSecret ?? '',
        region: tok.region ?? client.region ?? REGION,
        authMethod: idc ? 'idc' : 'social',
        provider: channel,
      },
    ]
  }
  return (
    <>
      <Field label={t('local.loginChannel')}>
        <SimpleSelect
          value={channel}
          onChange={setChannel}
          className="w-full"
          options={[
            { value: 'BuilderId', label: t('local.providerBuilderId') },
            { value: 'Enterprise', label: t('local.providerEnterprise') },
            { value: 'Google', label: t('local.providerGoogle') },
            { value: 'Github', label: t('local.providerGithub') },
          ]}
        />
      </Field>
      <p className="text-[11px] text-muted-foreground">
        {t('local.fileLocation')}: <code>~/.aws/sso/cache/</code> ({t('local.macosLinux')}) · <code>%USERPROFILE%\.aws\sso\cache\</code> ({t('local.windows')})
      </p>
      <Field label={`${t('local.tokenFile')} ${t('local.tokenRequired')}`} hint={t('local.pasteOrUpload')}>
        <Textarea value={tokenJson} onChange={(e) => setTokenJson(e.target.value)} rows={4} className="font-mono text-xs" />
        <FileInput onText={setTokenJson} />
      </Field>
      {idc && (
        <Field label={`${t('local.clientFile')} ${t('local.clientRequired')}`}>
          <Textarea value={clientJson} onChange={(e) => setClientJson(e.target.value)} rows={3} className="font-mono text-xs" />
          <FileInput onText={setClientJson} />
        </Field>
      )}
      <SubmitRow
        busy={busy}
        label={t('accounts.import')}
        disabled={!tokenJson.trim()}
        onClick={() => {
          try {
            go(parse())
          } catch (e) {
            toast.error(errorMessage(e))
          }
        }}
      />
    </>
  )
}

function CookieForm({ onDone }: FormProps) {
  const { t } = useI18n()
  const { busy, go } = useCredentialImport(onDone)
  const [refreshToken, setRefreshToken] = useState('')
  const [provider, setProvider] = useState('Google')
  return (
    <>
      <Alert>
        <AlertDescription>
          {t('cookie.step1')}{' '}
          <a href={t('cookie.link')} target="_blank" rel="noreferrer">
            app.kiro.dev
          </a>{' '}
          → {t('cookie.step2')} → {t('cookie.step3')}
        </AlertDescription>
      </Alert>
      <Field label={t('cookie.provider')}>
        <SimpleSelect
          value={provider}
          onChange={setProvider}
          className="w-full"
          options={[
            { value: 'Google', label: t('cookie.google') },
            { value: 'Github', label: t('cookie.github') },
          ]}
        />
      </Field>
      <Field label={t('cookie.refreshToken')}>
        <Textarea value={refreshToken} onChange={(e) => setRefreshToken(e.target.value)} rows={4} className="font-mono text-xs" placeholder={t('cookie.refreshTokenPlaceholder')} />
      </Field>
      <SubmitRow
        busy={busy}
        label={t('common.add')}
        disabled={!refreshToken.trim()}
        onClick={() => go([{ refreshToken: refreshToken.trim(), accessToken: '', clientId: '', clientSecret: '', authMethod: 'social', provider }])}
      />
    </>
  )
}

// ---- 4.6 CodeBuddy -----------------------------------------------------------------

function CodeBuddyForm({ onDone }: FormProps) {
  const { t } = useI18n()
  const { busy, submit } = useSubmit()
  const [label, setLabel] = useState('')
  const [variant, setVariant] = useState('global')
  const [text, setText] = useState('')

  // Accepted, mixed freely: CodeBuddy API keys (sk-…), JWT session tokens
  // (eyJ…), and token JSON ({"access_token","refresh_token","uid"}) as one
  // object, an array, or one object per line. Files are read into the box.
  const isJwt = (v: string) => v.startsWith('eyJ') && v.split('.').length === 3
  const jwtExpiry = (v: string): number | null => {
    try {
      const payload = JSON.parse(atob(v.split('.')[1].replace(/-/g, '+').replace(/_/g, '/')))
      return typeof payload.exp === 'number' ? payload.exp : null
    } catch {
      return null
    }
  }
  type Cred = { access: string; refresh?: string }
  const fromObj = (o: unknown): Cred | null => {
    if (!o || typeof o !== 'object') return null
    const m = o as Record<string, unknown>
    const pick = (...ks: string[]) => ks.map((k) => m[k]).find((v): v is string => typeof v === 'string' && v.trim() !== '')?.trim()
    const access = pick('access_token', 'accessToken', 'token', 'api_key', 'apiKey')
    return access ? { access, refresh: pick('refresh_token', 'refreshToken') } : null
  }
  const creds: Cred[] = (() => {
    const trimmed = text.trim()
    if (!trimmed) return []
    if (trimmed.startsWith('{') || trimmed.startsWith('[')) {
      try {
        const parsed = JSON.parse(trimmed)
        const list = (Array.isArray(parsed) ? parsed : [parsed]).map(fromObj).filter((c): c is Cred => !!c)
        if (list.length) return list
      } catch {
        /* fall through to line parsing */
      }
    }
    const out: Cred[] = []
    for (const raw of trimmed.split(/\r?\n/)) {
      const line = raw.trim().replace(/,$/, '')
      if (!line) continue
      if (line.startsWith('{')) {
        try {
          const c = fromObj(JSON.parse(line))
          if (c) out.push(c)
        } catch {
          /* skip unparsable line */
        }
        continue
      }
      out.push({ access: line })
    }
    return out
  })()
  const lines = creds.map((c) => c.access)
  const keys = creds.filter((c) => !isJwt(c.access))
  const jwts = creds.filter((c) => isJwt(c.access))
  const refreshable = creds.filter((c) => c.refresh && isJwt(c.refresh))
  const expiredJwts = jwts.filter((c) => {
    const exp = jwtExpiry(c.access)
    return exp !== null && exp * 1000 < Date.now()
  })

  const go = () =>
    submit(async () => {
      const r = await post<{ account: Added; accounts?: Added[]; errors?: string[] }>('/auth/codebuddy', {
        apiKey: text,
        label: label.trim(),
        variant,
      })
      const added = r.accounts && r.accounts.length ? r.accounts : [r.account]
      toast.success(t('codebuddy.importCount', added.length))
      if (r.errors?.length) toast.warning(r.errors.join('; '))
      onDone(added)
    })

  return (
    <>
      <Field label={t('codebuddy.label')} hint={t('codebuddy.labelHint')}>
        <Input value={label} onChange={(e) => setLabel(e.target.value)} placeholder={t('codebuddy.labelPlaceholder')} />
      </Field>
      <Field label={t('codebuddy.variant')}>
        <SimpleSelect
          value={variant}
          onChange={setVariant}
          className="w-full"
          options={[
            { value: 'global', label: t('codebuddy.global') },
            { value: 'cn', label: t('codebuddy.cn') },
          ]}
        />
      </Field>
      <Field label={t('codebuddy.credentials')} hint={t('codebuddy.credentialsHint')}>
        <Textarea
          value={text}
          onChange={(e) => setText(e.target.value)}
          rows={5}
          className="font-mono text-xs"
          autoComplete="off"
          spellCheck={false}
          placeholder={'sk-…\neyJhbGciOi…\n{"access_token":"eyJ…","refresh_token":"eyJ…","uid":"…"}'}
        />
        <div className="mt-2">
          <FileInput onText={(s) => setText((prev) => (prev.trim() ? prev.trimEnd() + '\n' + s : s))} />
        </div>
        {lines.length > 0 && (
          <div className="mt-2 flex flex-wrap gap-2 text-xs">
            {keys.length > 0 && <Badge variant="secondary">{t('codebuddy.detectedKeys', keys.length)}</Badge>}
            {jwts.length > 0 && <Badge variant="secondary">{t('codebuddy.detectedTokens', jwts.length)}</Badge>}
            {refreshable.length > 0 && <Badge variant="secondary">{t('codebuddy.detectedRefresh', refreshable.length)}</Badge>}
            {expiredJwts.length > 0 && <Badge variant="destructive">{t('codebuddy.expiredTokens', expiredJwts.length)}</Badge>}
          </div>
        )}
      </Field>
      <SubmitRow busy={busy} label={lines.length > 1 ? t('codebuddy.addMany', lines.length) : t('common.add')} onClick={go} disabled={lines.length === 0} />
    </>
  )
}

// ---- 4.7 Grok import ----------------------------------------------------------------

function GrokImportForm({ onDone }: FormProps) {
  const { t } = useI18n()
  const { busy, submit } = useSubmit()
  const [text, setText] = useState('')
  const go = () =>
    submit(async () => {
      // Raw body: server accepts object, array or NDJSON, so do not re-encode.
      const r = await api<{ imported: number; accounts: Added[]; errors?: string[] }>('/auth/grok/import', { method: 'POST', body: text.trim() })
      toast.success(`${t('grok.importSuccess')} (${r.imported})` + (r.errors?.length ? t('sso.importPartial', r.errors.length) : ''))
      onDone(r.accounts)
    })
  return (
    <>
      <Field label={t('grok.tokensLabel')} hint={t('grok.importHint')}>
        <Textarea value={text} onChange={(e) => setText(e.target.value)} rows={8} className="font-mono text-xs" placeholder='{"email":"…","tokens":{"access_token":"…","refresh_token":"…"}}' />
      </Field>
      <FileInput onText={setText} />
      <SubmitRow busy={busy} label={t('accounts.import')} onClick={go} disabled={!text.trim()} />
    </>
  )
}

// ---- 4.8 Codex (OpenAI / ChatGPT) token import -------------------------------

function CodexImportForm({ onDone }: FormProps) {
  const { t } = useI18n()
  const { busy, submit } = useSubmit()
  const [text, setText] = useState('')
  const go = () =>
    submit(async () => {
      // Raw body: server accepts object, array or NDJSON, so do not re-encode.
      const r = await api<{ imported: number; accounts: Added[]; errors?: string[] }>('/auth/codex/import', { method: 'POST', body: text.trim() })
      toast.success(`${t('codex.importSuccess')} (${r.imported})` + (r.errors?.length ? t('sso.importPartial', r.errors.length) : ''))
      onDone(r.accounts)
    })
  return (
    <>
      <Field label={t('codex.tokensLabel')} hint={t('codex.importHint')}>
        <Textarea value={text} onChange={(e) => setText(e.target.value)} rows={8} className="font-mono text-xs" placeholder='{"access_token":"…","refresh_token":"…","email":"…"}' />
      </Field>
      <FileInput onText={setText} />
      <SubmitRow busy={busy} label={t('accounts.import')} onClick={go} disabled={!text.trim()} />
    </>
  )
}

// ---- 4.9 ClinePass (api.cline.bot) token import -------------------------------

function ClinepassImportForm({ onDone }: FormProps) {
  const { t } = useI18n()
  const { busy, submit } = useSubmit()
  const [text, setText] = useState('')
  const go = () =>
    submit(async () => {
      // Server accepts a single JSON object, a JSON array, NDJSON, OR bare-token
      // lines (one clp_… / sk_… / workos:… token per line). Mixed bodies fail.
      const r = await api<{ imported: number; accounts: Added[]; errors?: string[] }>('/auth/clinepass/import', { method: 'POST', body: text.trim() })
      toast.success(`${t('clinepass.importSuccess')} (${r.imported})` + (r.errors?.length ? t('sso.importPartial', r.errors.length) : ''))
      onDone(r.accounts)
    })
  return (
    <>
      <Field label={t('clinepass.tokensLabel')} hint={t('clinepass.importHint')}>
        <Textarea value={text} onChange={(e) => setText(e.target.value)} rows={8} className="font-mono text-xs" placeholder={'{"access_token":"workos:…","refresh_token":"…","email":"…"}\n# or one token per line: sk_… / clp_…'} />
      </Field>
      <FileInput onText={setText} />
      <SubmitRow busy={busy} label={t('accounts.import')} onClick={go} disabled={!text.trim()} />
    </>
  )
}

// ---- OpenCode Zen / Go token import ----

function OpenCodeTokenImportForm({ provider, route, successKey, tokenLabel, hintKey, optional, onDone }: FormProps & {
  provider: string; route: string; successKey: string; tokenLabel: string; hintKey: string; optional?: boolean
}) {
  const { t } = useI18n()
  const { busy, submit } = useSubmit()
  const [text, setText] = useState('')
  const go = () =>
    submit(async () => {
      const r = await api<{ imported: number; accounts: Added[]; errors?: string[] }>(route, { method: 'POST', body: text.trim() || 'public' })
      toast.success(`${t(successKey)} (${r.imported})` + (r.errors?.length ? ` — ${r.errors.length} error(s)` : ''))
      onDone(r.accounts)
    })
  return (
    <>
      <Field label={t(tokenLabel)} hint={t(hintKey)}>
        <Textarea value={text} onChange={(e) => setText(e.target.value)} rows={4} className="font-mono text-xs" placeholder={optional ? 'Leave blank for free public tier, or paste API key(s) — one per line' : 'Paste API key(s) — one per line'} />
      </Field>
      <FileInput onText={setText} />
      <SubmitRow busy={busy} label={t('accounts.import')} onClick={go} disabled={!optional && !text.trim()} />
    </>
  )
}

// ---- OpenAI / Anthropic-compatible vendors (Vercel AI Gateway, OpenRouter, …) ----

function CompatForm({ protocol, onDone }: FormProps & { protocol: 'openai_compat' | 'anthropic_compat' }) {
  const { t } = useI18n()
  const { busy, submit } = useSubmit()
  const presets = COMPAT_PRESETS[protocol]
  const [name, setName] = useState('')
  const [baseUrl, setBaseUrl] = useState(presets[0].url)
  const [keys, setKeys] = useState('')
  const [models, setModels] = useState('')
  const keyList = keys.split(/\n/).map((k) => k.trim()).filter(Boolean)
  const modelList = models.split(/[,\n]/).map((m) => m.trim()).filter(Boolean)
  const preset = presets.find((p) => p.url === baseUrl)
  const go = () =>
    submit(async () => {
      const added: Added[] = []
      const errors: string[] = []
      for (const [i, key] of keyList.entries()) {
        const label = (name.trim() || preset?.name || new URL(baseUrl).host) + (keyList.length > 1 ? ` #${i + 1}` : '')
        try {
          const r = await post<{ id: string }>('/accounts', {
            providerKind: protocol,
            compatProtocol: protocol,
            authMethod: protocol,
            provider: preset?.name || protocol,
            email: label,
            nickname: label,
            baseUrl: baseUrl.trim(),
            compatApiKey: key,
            extraModels: modelList,
            enabled: true,
          })
          added.push({ id: r.id, email: label })
        } catch (e) {
          errors.push(`${label}: ${errorMessage(e)}`)
        }
      }
      if (added.length) toast.success(t('compat.added', added.length))
      for (const err of errors) toast.error(err)
      if (added.length) onDone(added)
    })
  return (
    <>
      <Field label={t('compat.preset')}>
        <SimpleSelect
          value={preset ? preset.url : '__custom__'}
          onChange={(v) => v !== '__custom__' && setBaseUrl(v)}
          options={[...presets.map((p) => ({ value: p.url, label: p.name })), { value: '__custom__', label: t('compat.custom') }]}
        />
      </Field>
      <Field label={t('compat.baseUrl')} hint={t(protocol === 'openai_compat' ? 'compat.baseUrlHintOpenai' : 'compat.baseUrlHintAnthropic')}>
        <Input value={baseUrl} onChange={(e) => setBaseUrl(e.target.value)} className="font-mono text-xs" placeholder="https://" />
      </Field>
      <Field label={t('compat.name')} hint={t('compat.nameHint')}>
        <Input value={name} onChange={(e) => setName(e.target.value)} placeholder={preset?.name || ''} />
      </Field>
      <Field label={t('compat.keys')} hint={t('compat.keysHint')}>
        <Textarea value={keys} onChange={(e) => setKeys(e.target.value)} rows={4} className="font-mono text-xs" placeholder={'sk-…\nsk-…'} />
      </Field>
      <Field label={t('compat.models')} hint={t('compat.modelsHint')}>
        <Input value={models} onChange={(e) => setModels(e.target.value)} className="font-mono text-xs" placeholder="anthropic/claude-sonnet-4.5, openai/gpt-5" />
      </Field>
      <SubmitRow busy={busy} label={keyList.length > 1 ? t('compat.addMany', keyList.length) : t('modal.addAccount')} onClick={go} disabled={!baseUrl.trim() || keyList.length === 0} />
    </>
  )
}
