// The auto-router's recent decisions and learned per-(account, model) stats.
// Lived under the auto-route settings form, where its length pushed the Save
// button out of reach; blacklisting from here saves the config directly.
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query'
import { LuBan, LuRefreshCw } from 'react-icons/lu'
import { toast } from 'sonner'
import { errorMessage, formatTime } from '@/components/common'
import { Badge } from '@/components/ui/badge'
import { Button } from '@/components/ui/button'
import { Card, CardContent } from '@/components/ui/card'
import { Table, TableBody, TableCell, TableHead, TableHeader, TableRow } from '@/components/ui/table'
import { get, post } from '@/lib/api'
import { useI18n } from '@/lib/i18n'

type Tier = 'fast' | 'balanced' | 'strong'
type Decision = {
  time: number
  endpoint: string
  tier: Tier
  model: string
  accountId: string
  score: number
  explored: boolean
  pinned: boolean
  thinking?: boolean
  email?: string
  provider?: string
  reason: string
}
type Candidate = {
  accountId: string
  provider: string
  email: string
  model: string
  successes: number
  failures: number
  ewmaLatencyMs: number
  reliability: number
}
type Decisions = { decisions: Decision[]; candidates: Candidate[] }
type RouteConfig = { enabled: boolean; blacklist?: string[] } & Record<string, unknown>

const TIER_VARIANT: Record<Tier, 'secondary' | 'outline' | 'default'> = { fast: 'secondary', balanced: 'outline', strong: 'default' }
const short = (id: string) => (id.length > 10 ? id.slice(0, 8) + '…' : id)

export function RouterActivity() {
  const { t } = useI18n()
  const qc = useQueryClient()
  const cfg = useQuery({ queryKey: ['auto-route'], queryFn: () => get<RouteConfig>('/auto-route') })
  const dq = useQuery({
    queryKey: ['auto-route-decisions'],
    queryFn: () => get<Decisions>('/auto-route/decisions'),
    refetchInterval: cfg.data?.enabled ? 10_000 : false,
  })
  const decisions = dq.data?.decisions?.slice(0, 20) ?? []
  const candidates = dq.data?.candidates ?? []
  const blacklist = cfg.data?.blacklist ?? []

  const ban = useMutation({
    // Re-read the saved config so a stale cache cannot undo someone else's edit.
    mutationFn: async (entry: string) => {
      const cur = await get<RouteConfig>('/auto-route')
      await post('/auto-route', { ...cur, blacklist: [...(cur.blacklist ?? []), entry] })
    },
    onSuccess: (_, entry) => {
      toast.success(t('modelStatus.blacklisted', entry))
      qc.invalidateQueries({ queryKey: ['auto-route'] })
      qc.invalidateQueries({ queryKey: ['model-health'] })
    },
    onError: (e) => toast.error(errorMessage(e, t('common.saveFailed'))),
  })

  return (
    <Card>
      <CardContent className="space-y-3">
        <div className="flex items-center justify-between gap-2">
          <div className="text-sm font-medium">{t('settings.autoRoute.decisions')}</div>
          <Button size="sm" variant="outline" onClick={() => dq.refetch()} disabled={dq.isFetching}>
            <LuRefreshCw className={dq.isFetching ? 'size-3.5 animate-spin' : 'size-3.5'} />
            {t('common.refresh')}
          </Button>
        </div>
        {decisions.length === 0 ? (
          <p className="py-4 text-center text-xs text-muted-foreground">{t('settings.autoRoute.noDecisions')}</p>
        ) : (
          <div className="overflow-x-auto">
            <Table className="text-xs">
              <TableHeader>
                <TableRow>
                  <TableHead>{t('settings.autoRoute.col.time')}</TableHead>
                  <TableHead>{t('settings.autoRoute.col.endpoint')}</TableHead>
                  <TableHead>{t('settings.autoRoute.col.tier')}</TableHead>
                  <TableHead>{t('settings.autoRoute.col.model')}</TableHead>
                  <TableHead>{t('settings.autoRoute.col.account')}</TableHead>
                  <TableHead className="text-right">{t('settings.autoRoute.col.score')}</TableHead>
                  <TableHead>{t('settings.autoRoute.col.flags')}</TableHead>
                  <TableHead>{t('settings.autoRoute.col.reason')}</TableHead>
                </TableRow>
              </TableHeader>
              <TableBody>
                {decisions.map((d, i) => (
                  <TableRow key={i}>
                    <TableCell className="whitespace-nowrap">{formatTime(d.time)}</TableCell>
                    <TableCell className="font-mono">{d.endpoint}</TableCell>
                    <TableCell>
                      <Badge variant={TIER_VARIANT[d.tier] ?? 'outline'}>{d.tier}</Badge>
                    </TableCell>
                    <TableCell className="font-mono">{d.model}</TableCell>
                    <TableCell className="max-w-56 truncate" title={`${d.email || ''} (${d.accountId})`}>
                      {d.provider && <Badge variant="outline" className="mr-1">{d.provider}</Badge>}
                      {d.email || short(d.accountId)}
                    </TableCell>
                    <TableCell className="text-right tabular-nums">{d.score.toFixed(3)}</TableCell>
                    <TableCell className="space-x-1">
                      {d.explored && <Badge variant="secondary">{t('settings.autoRoute.explored')}</Badge>}
                      {d.pinned && <Badge variant="outline">{t('settings.autoRoute.pinned')}</Badge>}
                      {d.thinking && <Badge variant="default">{t('settings.autoRoute.thinking')}</Badge>}
                    </TableCell>
                    <TableCell className="max-w-48 truncate text-muted-foreground" title={d.reason}>
                      {d.reason}
                    </TableCell>
                  </TableRow>
                ))}
              </TableBody>
            </Table>
          </div>
        )}

        <div className="text-sm font-medium">{t('settings.autoRoute.candidates')}</div>
        {candidates.length === 0 ? (
          <p className="py-4 text-center text-xs text-muted-foreground">{t('settings.autoRoute.noCandidates')}</p>
        ) : (
          <div className="overflow-x-auto">
            <Table className="text-xs">
              <TableHeader>
                <TableRow>
                  <TableHead>{t('settings.autoRoute.col.provider')}</TableHead>
                  <TableHead>{t('settings.autoRoute.col.model')}</TableHead>
                  <TableHead>{t('settings.autoRoute.col.account')}</TableHead>
                  <TableHead className="text-right">{t('settings.autoRoute.col.reliability')}</TableHead>
                  <TableHead className="text-right">{t('settings.autoRoute.col.latency')}</TableHead>
                  <TableHead className="text-right">{t('settings.autoRoute.col.outcomes')}</TableHead>
                  <TableHead />
                </TableRow>
              </TableHeader>
              <TableBody>
                {candidates.map((c) => {
                  const entry = `${c.provider || '*'}:${c.model}`
                  const listed = blacklist.some((x) => x.toLowerCase() === entry.toLowerCase())
                  return (
                    <TableRow key={c.accountId + c.model}>
                      <TableCell>
                        <Badge variant="outline">{c.provider || '?'}</Badge>
                      </TableCell>
                      <TableCell className="font-mono">{c.model}</TableCell>
                      <TableCell className="max-w-56 truncate" title={`${c.email} (${c.accountId})`}>
                        {c.email || short(c.accountId)}
                      </TableCell>
                      <TableCell className="text-right tabular-nums">{Math.round(c.reliability * 100)}%</TableCell>
                      <TableCell className="text-right tabular-nums">{Math.round(c.ewmaLatencyMs)} ms</TableCell>
                      <TableCell className="text-right tabular-nums">
                        {Math.round(c.successes)} / {Math.round(c.failures)}
                      </TableCell>
                      <TableCell className="text-right">
                        <Button size="sm" variant="ghost" className="h-6 px-2 text-[11px]" disabled={listed || !cfg.data || ban.isPending} title={t('settings.autoRoute.blacklistThis')} onClick={() => ban.mutate(entry)}>
                          <LuBan className="size-3" /> {listed ? t('settings.autoRoute.blacklisted') : t('settings.autoRoute.blacklistThis')}
                        </Button>
                      </TableCell>
                    </TableRow>
                  )
                })}
              </TableBody>
            </Table>
          </div>
        )}
      </CardContent>
    </Card>
  )
}
