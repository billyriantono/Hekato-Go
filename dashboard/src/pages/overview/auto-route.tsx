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
import { short } from './charts'

type Decision = {
  time: number
  endpoint: string
  tier: string
  model: string
  accountId: string
  score: number
  explored: boolean
  pinned: boolean
  signals: { inputTokens: number; tools: number; turns: number; images: number; thinking: boolean }
  reason: string
}
type Candidate = { accountId: string; model: string; successes: number; failures: number; ewmaLatencyMs: number; reliability: number; lastUpdated: number }

const REFETCH = 15000

export function AutoRouteCard() {
  const { t } = useI18n()
  const config = useQuery({ queryKey: ['auto-route'], queryFn: () => get<{ enabled: boolean }>('/auto-route'), refetchInterval: REFETCH })
  const enabled = !!config.data?.enabled
  const data = useQuery({
    queryKey: ['auto-route', 'decisions'],
    queryFn: () => get<{ decisions: Decision[]; candidates: Candidate[] }>('/auto-route/decisions'),
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
      <CardContent className="grid gap-6 xl:grid-cols-2">
        {data.isPending ? (
          <LoadingBlock />
        ) : (
          <>
            <div className="space-y-2">
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
                          {d.model} <span className="text-muted-foreground">· {short(d.accountId)}</span>
                        </TooltipTrigger>
                        <TooltipContent className="max-w-xs break-words">
                          <div>{d.reason || '—'}</div>
                          <div className="mt-1 opacity-70">
                            {d.endpoint} · in={d.signals?.inputTokens ?? 0} tools={d.signals?.tools ?? 0} turns={d.signals?.turns ?? 0} img={d.signals?.images ?? 0} {d.signals?.thinking ? '· thinking' : ''}
                          </div>
                        </TooltipContent>
                      </Tooltip>
                      <span className="shrink-0 tabular-nums text-muted-foreground">{d.score.toFixed(2)}</span>
                      {d.pinned && <Badge variant="outline">{t('overview.pinned')}</Badge>}
                      {d.explored && <Badge variant="outline">{t('overview.explored')}</Badge>}
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
                        <TableCell className="font-mono text-xs">{c.model}</TableCell>
                        <TableCell className="font-mono text-xs" title={`${c.accountId} · ${formatTime(c.lastUpdated)}`}>{short(c.accountId)}</TableCell>
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
