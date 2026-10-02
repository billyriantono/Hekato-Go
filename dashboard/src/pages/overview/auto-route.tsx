// Auto-routing card: last decisions + candidate reliability table. Hidden behind config.enabled.
import { useQuery } from '@tanstack/react-query'
import { Link } from '@tanstack/react-router'
import { LuArrowRight } from 'react-icons/lu'
import { EmptyState, LoadingBlock, formatTime } from '@/components/common'
import { Badge } from '@/components/ui/badge'
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from '@/components/ui/card'
import { Table, TableBody, TableCell, TableHead, TableHeader, TableRow } from '@/components/ui/table'
import { Tooltip, TooltipContent, TooltipTrigger } from '@/components/ui/tooltip'
import { get } from '@/lib/api'
import { useI18n } from '@/lib/i18n'
import { short } from './bits'

type Decision = {
  time: number
  endpoint: string
  tier: string
  model: string
  accountId: string
  score: number
  explored: boolean
  pinned: boolean
  thinking?: boolean
  email?: string
  provider?: string
  signals: { inputTokens: number; tools: number; turns: number; images: number; thinking: boolean }
  reason: string
  wantedTier?: string
  candidates?: number
}
type TierHealth = { tier: string; candidates: number; wanted: number; served: number; starved: number; deadPatterns?: string[] }
type Candidate = { accountId: string; email?: string; provider?: string; model: string; successes: number; failures: number; ewmaLatencyMs: number; reliability: number; lastUpdated: number; quarantinedUntil?: number; quarantineReason?: string }
const who = (email?: string, id = '') => email || short(id)

const REFETCH = 15000

export function AutoRouteCard() {
  const { t } = useI18n()
  const config = useQuery({ queryKey: ['auto-route'], queryFn: () => get<{ enabled: boolean }>('/auto-route'), refetchInterval: REFETCH })
  const enabled = !!config.data?.enabled
  const data = useQuery({
    queryKey: ['auto-route', 'decisions'],
    queryFn: () => get<{ decisions: Decision[]; candidates: Candidate[]; tierHealth?: TierHealth[] }>('/auto-route/decisions'),
    refetchInterval: REFETCH,
    enabled,
  })

  if (!enabled) {
    return (
      <p className="text-xs text-muted-foreground">
        {t('overview.autoRouteOff')}{' '}
        <Link to="/settings" className="inline-flex items-center gap-1 text-primary hover:underline">
          {t('overview.autoRouteSettings')} <LuArrowRight className="size-3" />
        </Link>
      </p>
    )
  }

  const tierHealth = data.data?.tierHealth ?? []
  // A tier is starved when requests classified into it had to be served
  // elsewhere — misconfiguration, not bad luck, and invisible in the decision
  // list because each row shows only the tier that answered.
  const starved = tierHealth.filter((h) => h.starved > 0 || (h.wanted > 0 && h.candidates <= 1))
  const dead = tierHealth.filter((h) => (h.deadPatterns ?? []).length > 0)
  const decisions = (data.data?.decisions ?? []).slice(0, 10)
  const candidates = (data.data?.candidates ?? []).slice().sort((a, b) => b.reliability - a.reliability)
  // ponytail: reliability may arrive as 0..1 or 0..100; normalise by magnitude
  const pct = (r: number) => `${(r <= 1 ? r * 100 : r).toFixed(0)}%`

  return (
    <Card>
      <CardHeader>
        <CardTitle>{t('overview.autoRoute')}</CardTitle>
        <CardDescription>{t('overview.autoRouteHint')}</CardDescription>
      </CardHeader>
      {(starved.length > 0 || dead.length > 0) && (
        <div className="mx-6 mb-2 space-y-1 rounded-md border border-amber-500/40 bg-amber-500/5 px-3 py-2 text-xs">
          {starved.map((h) => (
            <div key={h.tier} className="text-amber-700 dark:text-amber-400">
              {t('overview.tierStarved', h.tier, h.starved, h.wanted, h.candidates)}
            </div>
          ))}
          {dead.map((h) => (
            <div key={`dead-${h.tier}`} className="text-muted-foreground">
              <span className="font-mono">{h.tier}</span> · {t('overview.deadPatterns', (h.deadPatterns ?? []).join(', '))}
            </div>
          ))}
        </div>
      )}
      <CardContent className="grid gap-6 xl:grid-cols-2">
        {data.isPending ? (
          <LoadingBlock />
        ) : (
          <>
            <div className="space-y-2">
              <div className="flex flex-wrap items-center gap-2">
                <div className="text-xs font-medium text-muted-foreground">{t('overview.tiers')}</div>
                {tierHealth.map((h) => (
                  <Tooltip key={h.tier}>
                    <TooltipTrigger render={<Badge variant={h.candidates === 0 ? 'destructive' : 'outline'} className="cursor-help font-mono text-[10px]" />}>
                      {h.tier} {h.candidates}
                    </TooltipTrigger>
                    <TooltipContent>{t('overview.tierHealthHint', h.candidates, h.wanted, h.served, h.starved)}</TooltipContent>
                  </Tooltip>
                ))}
              </div>
              <div className="text-xs font-medium text-muted-foreground">{t('overview.decisions')}</div>
              {decisions.length === 0 ? (
                <EmptyState title={t('overview.noDecisions')} />
              ) : (
                <ul className="divide-y text-xs">
                  {decisions.map((d, i) => (
                    <li key={`${d.time}-${i}`} className="flex items-center gap-2 py-1.5">
                      <span className="w-[4.5rem] shrink-0 whitespace-nowrap text-muted-foreground">{new Date(d.time * 1000).toLocaleTimeString([], { hour: '2-digit', minute: '2-digit', second: '2-digit' })}</span>
                      <Badge variant="secondary">{d.tier}</Badge>
                      <Tooltip>
                        <TooltipTrigger render={<span className="min-w-0 flex-1 cursor-help truncate font-mono" />}>
                          {d.model} <span className="text-muted-foreground">· {d.provider ? `${d.provider} / ` : ''}{who(d.email, d.accountId)}</span>
                        </TooltipTrigger>
                        <TooltipContent className="max-w-xs break-words">
                          <div>{d.reason || '—'}</div>
                          <div className="mt-1 opacity-70">
                            {d.endpoint} · in={d.signals?.inputTokens ?? 0} tools={d.signals?.tools ?? 0} turns={d.signals?.turns ?? 0} img={d.signals?.images ?? 0} {d.signals?.thinking ? '· thinking' : ''}
                          </div>
                        </TooltipContent>
                      </Tooltip>
                      <span className="shrink-0 tabular-nums text-muted-foreground">{d.score.toFixed(2)}</span>
                      {d.wantedTier && <Badge variant="outline" className="text-amber-600 dark:text-amber-400">{t('overview.wantedTier', d.wantedTier)}</Badge>}
                      {d.pinned && <Badge variant="outline">{t('overview.pinned')}</Badge>}
                      {d.explored && <Badge variant="outline">{t('overview.explored')}</Badge>}
                      {d.thinking && <Badge variant="outline">{t('overview.thinking')}</Badge>}
                    </li>
                  ))}
                </ul>
              )}
            </div>

            <div className="space-y-2">
              <div className="text-xs font-medium text-muted-foreground">{t('overview.candidates')}</div>
              {candidates.length === 0 ? (
                <EmptyState title={t('overview.noCandidates')} />
              ) : (
                <Table>
                  <TableHeader>
                    <TableRow>
                      <TableHead>{t('overview.provider')}</TableHead>
                      <TableHead>{t('logs.model')}</TableHead>
                      <TableHead>{t('logs.account')}</TableHead>
                      <TableHead className="text-right">{t('overview.reliability')}</TableHead>
                      <TableHead className="text-right">{t('overview.ewma')}</TableHead>
                      <TableHead className="text-right">{t('overview.okFail')}</TableHead>
                    </TableRow>
                  </TableHeader>
                  <TableBody>
                    {candidates.map((c) => (
                      <TableRow key={`${c.accountId}/${c.model}`}>
                        <TableCell className="text-xs">{c.provider ? <Badge variant="outline">{c.provider}</Badge> : '—'}</TableCell>
                        <TableCell className="font-mono text-xs">
                          {c.model}
                          {c.quarantinedUntil && (
                            <Tooltip>
                              <TooltipTrigger render={<Badge variant="destructive" className="ml-1.5 cursor-help text-[10px]" />}>
                                {t('overview.quarantined')}
                              </TooltipTrigger>
                              <TooltipContent className="max-w-xs break-words">
                                {t('overview.quarantinedUntil', formatTime(c.quarantinedUntil))}
                                {c.quarantineReason ? <div className="mt-1 opacity-70">{c.quarantineReason}</div> : null}
                              </TooltipContent>
                            </Tooltip>
                          )}
                        </TableCell>
                        <TableCell className="max-w-48 truncate text-xs" title={`${c.email || ''} ${c.accountId} · ${formatTime(c.lastUpdated)}`}>{who(c.email, c.accountId)}</TableCell>
                        <TableCell className="text-right text-xs tabular-nums">{pct(c.reliability)}</TableCell>
                        <TableCell className="text-right text-xs tabular-nums">{Math.round(c.ewmaLatencyMs)}ms</TableCell>
                        <TableCell className="text-right text-xs tabular-nums">{c.successes}/{c.failures}</TableCell>
                      </TableRow>
                    ))}
                  </TableBody>
                </Table>
              )}
            </div>
          </>
        )}
      </CardContent>
    </Card>
  )
}
