// Public community-event page: publishes one shared API key with its quota and
// live traffic, no login needed. The key is chosen server-side (COMMUNITY_KEY_ID)
// and all limits come from that key's normal API-key settings.
import { useEffect, useState } from 'react'
import { LuMoon, LuSun } from 'react-icons/lu'
import { Button } from '@/components/ui/button'
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from '@/components/ui/card'
import { Badge } from '@/components/ui/badge'
import { CopyButton, formatNumber } from '@/components/common'
import { useTheme } from '@/lib/theme'
import { LiveRoutingMap, type RequestLog } from '@/pages/overview/routing-map'
import { Bar } from '@/pages/usage'

type Community = {
  active: boolean
  name?: string
  key?: string
  tokensUsed?: number
  tokenLimit?: number
  requestsCount?: number
  rpmLimit?: number
  concurrencyLimit?: number
  allowedModels?: string[] | null
  logs?: (RequestLog & { tokens: number; clientIp?: string; userAgent?: string })[]
}

function Row({ label, value }: { label: string; value: string }) {
  return (
    <div className="flex items-center gap-2 rounded-md border bg-muted/40 py-1 pl-3 pr-1">
      <span className="w-16 shrink-0 text-xs text-muted-foreground">{label}</span>
      <code className="min-w-0 flex-1 truncate font-mono text-sm">{value}</code>
      <CopyButton value={value} />
    </div>
  )
}

export function CommunityPage() {
  const { theme, toggle } = useTheme()
  const [data, setData] = useState<Community | null>(null)
  const [error, setError] = useState('')

  useEffect(() => {
    let live = true
    const load = async () => {
      try {
        const res = await fetch('/v1/community', { headers: { Accept: 'application/json' } })
        if (!res.ok) throw new Error(res.statusText)
        const body = (await res.json()) as Community
        if (live) {
          setData(body)
          setError('')
        }
      } catch (err) {
        if (live) setError(err instanceof Error ? err.message : 'Failed to load')
      }
    }
    load()
    const id = setInterval(load, 5_000)
    return () => {
      live = false
      clearInterval(id)
    }
  }, [])

  const endpoint = `${window.location.origin}/v1`
  const logs = data?.logs ?? []
  const models = data?.allowedModels?.length ? data.allowedModels : null
  const sampleModel = models?.find((m) => !m.endsWith('*')) ?? 'auto'
  const ok = logs.filter((l) => l.status === 'success').length
  const used = data?.tokensUsed ?? 0
  const limit = data?.tokenLimit ?? 0

  return (
    <div className="flex min-h-svh flex-col bg-muted/40">
      <header className="flex h-14 items-center justify-between px-4">
        <div className="flex items-center gap-2">
          <div className="flex size-8 items-center justify-center rounded-lg bg-primary text-sm font-bold text-primary-foreground">H</div>
          <span className="text-sm font-semibold">Hekato</span>
        </div>
        <Button variant="ghost" size="icon" onClick={toggle} aria-label="Toggle theme">
          {theme === 'dark' ? <LuSun className="size-4" /> : <LuMoon className="size-4" />}
        </Button>
      </header>
      <main className="flex flex-1 items-start justify-center p-4 pt-8">
        <div className="w-full max-w-3xl space-y-4">
          <div className="space-y-1 px-1">
            <p className="text-xs font-semibold tracking-widest text-muted-foreground">BANSOS · COMMUNITY EVENT</p>
            <h1 className="text-2xl font-bold">{data?.active && data.name ? data.name : 'Bansos API Hekato'}</h1>
            <p className="text-sm text-muted-foreground">
              One shared key for the community. Grab it, plug it into your app, and see live usage below — no login needed.
            </p>
          </div>

          {error && <p className="text-sm text-destructive">{error}</p>}

          {data && !data.active && (
            <Card className="shadow-lg">
              <CardHeader>
                <CardTitle>No event running</CardTitle>
                <CardDescription>There's no active community key right now. Check back later!</CardDescription>
              </CardHeader>
            </Card>
          )}

          {data?.active && data.key && (
            <>
              <Card className="shadow-lg">
                <CardHeader>
                  <CardTitle>Connect your app</CardTitle>
                  <CardDescription>OpenAI- and Anthropic-compatible. Use this base URL with the key.</CardDescription>
                </CardHeader>
                <CardContent className="space-y-3">
                  <Row label="API key" value={data.key} />
                  <Row label="Base URL" value={endpoint} />
                  <div className="flex flex-wrap gap-2 text-xs text-muted-foreground">
                    {['/chat/completions', '/responses', '/messages', '/models'].map((p) => (
                      <code key={p} className="rounded bg-muted px-1.5 py-0.5">/v1{p}</code>
                    ))}
                  </div>
                  <pre className="overflow-x-auto rounded-md bg-muted p-3 text-xs">{`curl ${endpoint}/chat/completions \\
  -H "Authorization: Bearer ${data.key}" \\
  -H "Content-Type: application/json" \\
  -d '{"model":"${sampleModel}","messages":[{"role":"user","content":"Halo"}]}'`}</pre>
                  <div className="flex flex-wrap items-center gap-1.5">
                    <span className="text-xs text-muted-foreground">Models:</span>
                    {models ? models.map((m) => <Badge key={m} variant="secondary" className="font-mono">{m}</Badge>) : <Badge variant="secondary">all — see /v1/models</Badge>}
                  </div>
                </CardContent>
              </Card>

              <Card className="shadow-lg">
                <CardContent className="space-y-5 pt-6">
                  <Bar label="Tokens" used={used} limit={limit} pct={limit > 0 ? used / limit : 0} format={formatNumber} />
                  <div className="grid grid-cols-2 gap-3 text-sm sm:grid-cols-4">
                    {[
                      ['Remaining', limit > 0 ? formatNumber(Math.max(0, limit - used)) : 'Unlimited'],
                      ['Requests', formatNumber(data.requestsCount ?? 0)],
                      ['Success (recent)', logs.length ? `${Math.round((ok / logs.length) * 100)}%` : '—'],
                      ['Shared limit',`${data.rpmLimit || '∞'} rpm · ${data.concurrencyLimit || '∞'} parallel`],
                    ].map(([k, v]) => (
                      <div key={k} className="rounded-md border p-3">
                        <div className="text-xs text-muted-foreground">{k}</div>
                        <div className="mt-1 font-semibold tabular-nums">{v}</div>
                      </div>
                    ))}
                  </div>
                  <p className="text-xs text-muted-foreground">Rate limits apply to the shared key as a whole — be kind and leave room for others.</p>
                </CardContent>
              </Card>

              <LiveRoutingMap logs={logs} />

              <Card className="shadow-lg">
                <CardHeader>
                  <CardTitle>Recent requests</CardTitle>
                </CardHeader>
                <CardContent className="overflow-x-auto">
                  {logs.length === 0 ? (
                    <p className="text-sm text-muted-foreground">No requests yet — be the first!</p>
                  ) : (
                    <table className="w-full min-w-[36rem] text-left text-xs">
                      <thead className="text-muted-foreground">
                        <tr>
                          {['Time', 'Model', 'Status', 'Tokens', 'Client', 'IP'].map((h) => <th key={h} className="pb-2 font-medium">{h}</th>)}
                        </tr>
                      </thead>
                      <tbody>
                        {logs.slice(0, 50).map((l, i) => (
                          <tr key={i} className="border-t">
                            <td className="py-1.5 whitespace-nowrap">{new Date(l.time * 1000).toLocaleTimeString()}</td>
                            <td className="font-mono">{l.model}</td>
                            <td><Badge variant={l.status === 'success' ? 'secondary' : 'destructive'}>{l.status}</Badge></td>
                            <td className="tabular-nums">{formatNumber(l.tokens)}</td>
                            <td className="max-w-40 truncate" title={l.userAgent}>{l.userAgent || 'other'}</td>
                            <td className="font-mono">{l.clientIp || '—'}</td>
                          </tr>
                        ))}
                      </tbody>
                    </table>
                  )}
                </CardContent>
              </Card>
            </>
          )}
          <p className="text-center text-xs text-muted-foreground">IPs are partially masked. Don't send anything private through a shared key.</p>
        </div>
      </main>
    </div>
  )
}
