// Public self-service usage check: paste an API key, see its quota and usage.
// Served at /admin/usage without admin login; the key itself is the credential.
import { useState } from 'react'
import { LuLoader, LuSearch, LuMoon, LuSun, LuLanguages } from 'react-icons/lu'
import { Button } from '@/components/ui/button'
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from '@/components/ui/card'
import { Input } from '@/components/ui/input'
import { Badge } from '@/components/ui/badge'
import { formatNumber, formatTime } from '@/components/common'
import { useI18n } from '@/lib/i18n'
import { useTheme } from '@/lib/theme'

type Usage = {
  name: string
  keyMasked: string
  enabled: boolean
  createdAt: number
  lastUsedAt: number
  requestsCount: number
  tokensUsed: number
  tokenLimit: number
  tokenPercent: number
  creditsUsed: number
  creditLimit: number
  creditPercent: number
  rpmLimit: number
  concurrencyLimit: number
}

function Bar({ label, used, limit, pct, format }: { label: string; used: number; limit: number; pct: number; format: (n: number) => string }) {
  const { t } = useI18n()
  const p = Math.min(100, Math.round(pct * 100))
  const tone = p >= 90 ? 'bg-red-500' : p >= 70 ? 'bg-amber-500' : 'bg-emerald-500'
  return (
    <div className="space-y-1.5">
      <div className="flex items-baseline justify-between text-sm">
        <span className="font-medium">{label}</span>
        <span className="tabular-nums text-muted-foreground">
          {format(used)} / {limit > 0 ? format(limit) : t('apiKeys.unlimited')}
        </span>
      </div>
      <div className="h-2 w-full overflow-hidden rounded-full bg-muted">
        <div className={`h-full rounded-full ${limit > 0 ? tone : 'bg-primary/40'}`} style={{ width: limit > 0 ? `${p}%` : '100%' }} />
      </div>
      {limit > 0 && <div className="text-right text-xs text-muted-foreground">{p}%</div>}
    </div>
  )
}

export function UsagePage() {
  const { t, lang, setLang } = useI18n()
  const { theme, toggle } = useTheme()
  const [key, setKey] = useState('')
  const [busy, setBusy] = useState(false)
  const [error, setError] = useState('')
  const [data, setData] = useState<Usage | null>(null)

  const lookup = async (e: React.SyntheticEvent) => {
    e.preventDefault()
    if (!key.trim()) return
    setBusy(true)
    setError('')
    setData(null)
    try {
      const res = await fetch('/v1/usage', { headers: { Authorization: 'Bearer ' + key.trim(), Accept: 'application/json' } })
      const body = await res.json().catch(() => ({}))
      if (!res.ok) throw new Error(body.error || res.statusText)
      setData(body as Usage)
    } catch (err) {
      setError(err instanceof Error ? err.message : t('common.unknownError'))
    } finally {
      setBusy(false)
    }
  }

  return (
    <div className="flex min-h-svh flex-col bg-muted/40">
      <header className="flex h-14 items-center justify-between px-4">
        <div className="flex items-center gap-2">
          <div className="flex size-8 items-center justify-center rounded-lg bg-primary text-primary-foreground text-sm font-bold">H</div>
          <span className="text-sm font-semibold">Hekato</span>
        </div>
        <div className="flex items-center gap-1">
          <Button variant="ghost" size="icon" onClick={() => setLang(lang === 'en' ? 'zh' : 'en')} aria-label={t('header.language')}>
            <LuLanguages className="size-4" />
          </Button>
          <Button variant="ghost" size="icon" onClick={toggle} aria-label={t('header.theme')}>
            {theme === 'dark' ? <LuSun className="size-4" /> : <LuMoon className="size-4" />}
          </Button>
        </div>
      </header>
      <main className="flex flex-1 items-start justify-center p-4 pt-10">
        <div className="w-full max-w-lg space-y-4">
          <Card className="shadow-lg">
            <CardHeader>
              <CardTitle>{t('usage.title')}</CardTitle>
              <CardDescription>{t('usage.subtitle')}</CardDescription>
            </CardHeader>
            <CardContent>
              <form onSubmit={lookup} className="flex gap-2">
                <Input
                  type="password"
                  autoComplete="off"
                  value={key}
                  onChange={(e) => setKey(e.target.value)}
                  placeholder={t('usage.placeholder')}
                  autoFocus
                />
                <Button type="submit" disabled={busy || !key.trim()}>
                  {busy ? <LuLoader className="size-4 animate-spin" /> : <LuSearch className="size-4" />}
                  {t('usage.check')}
                </Button>
              </form>
              {error && <p className="mt-3 text-sm text-destructive">{error}</p>}
            </CardContent>
          </Card>

          {data && (
            <Card className="shadow-lg">
              <CardHeader>
                <div className="flex flex-wrap items-center justify-between gap-2">
                  <CardTitle className="font-mono text-base">{data.keyMasked}</CardTitle>
                  <Badge variant={data.enabled ? 'secondary' : 'destructive'}>{data.enabled ? t('common.enabled') : t('common.disabled')}</Badge>
                </div>
                {data.name && <CardDescription>{data.name}</CardDescription>}
              </CardHeader>
              <CardContent className="space-y-5">
                <Bar label={t('apiKeys.tokens')} used={data.tokensUsed} limit={data.tokenLimit} pct={data.tokenPercent} format={formatNumber} />
                <Bar label={t('apiKeys.credits')} used={data.creditsUsed} limit={data.creditLimit} pct={data.creditPercent} format={(n) => n.toFixed(2)} />
                <dl className="grid grid-cols-2 gap-x-4 gap-y-3 text-sm">
                  <dt className="text-muted-foreground">{t('apiKeys.requests')}</dt>
                  <dd className="text-right tabular-nums">{formatNumber(data.requestsCount)}</dd>
                  <dt className="text-muted-foreground">{t('apiKeys.limitRpm')}</dt>
                  <dd className="text-right tabular-nums">{data.rpmLimit || t('apiKeys.unlimited')}</dd>
                  <dt className="text-muted-foreground">{t('apiKeys.limitConcurrency')}</dt>
                  <dd className="text-right tabular-nums">{data.concurrencyLimit || t('apiKeys.unlimited')}</dd>
                  <dt className="text-muted-foreground">{t('usage.lastUsed')}</dt>
                  <dd className="text-right">{formatTime(data.lastUsedAt)}</dd>
                  <dt className="text-muted-foreground">{t('usage.created')}</dt>
                  <dd className="text-right">{formatTime(data.createdAt)}</dd>
                </dl>
              </CardContent>
            </Card>
          )}
          <p className="text-center text-xs text-muted-foreground">{t('usage.privacy')}</p>
        </div>
      </main>
    </div>
  )
}
