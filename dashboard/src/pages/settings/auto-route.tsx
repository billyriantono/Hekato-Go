import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query'
import { LuBan, LuRefreshCw, LuX } from 'react-icons/lu'
import { toast } from 'sonner'
import { Badge } from '@/components/ui/badge'
import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'
import { Table, TableBody, TableCell, TableHead, TableHeader, TableRow } from '@/components/ui/table'
import { LoadingBlock, errorMessage, formatTime } from '@/components/common'
import { get, post } from '@/lib/api'
import { useI18n } from '@/lib/i18n'
import { Field, SaveButton, Section, SwitchRow, useDraft } from './shared'

type Tier = 'fast' | 'balanced' | 'strong'
type Config = {
  enabled: boolean
  qualityWeight: number
  costWeight: number
  speedWeight: number
  explore: number
  fast: string[]
  balanced: string[]
  strong: string[]
  blacklist: string[]
}
type Decision = {
  time: number
  endpoint: string
  tier: Tier
  model: string
  accountId: string
  score: number
  explored: boolean
  pinned: boolean
  signals: { inputTokens: number; tools: number; turns: number; images: number; thinking: boolean }
  reason: string
}
type Candidate = {
  accountId: string
  provider: string
  model: string
  successes: number
  failures: number
  ewmaLatencyMs: number
  reliability: number
  lastUpdated: number
}
type Decisions = { decisions: Decision[]; candidates: Candidate[] }

const TIERS: Tier[] = ['fast', 'balanced', 'strong']
const TIER_VARIANT: Record<Tier, 'secondary' | 'outline' | 'default'> = { fast: 'secondary', balanced: 'outline', strong: 'default' }
const short = (id: string) => (id.length > 10 ? id.slice(0, 8) + '…' : id)
const splitPatterns = (s: string) => s.split(',').map((p) => p.trim()).filter(Boolean)

function Slider({ id, label, hint, value, max, onChange }: { id: string; label: string; hint: string; value: number; max: number; onChange: (v: number) => void }) {
  return (
    <Field label={`${label} · ${value.toFixed(2)}`} hint={hint} htmlFor={id}>
      <input id={id} type="range" min={0} max={max} step={0.05} value={value} onChange={(e) => onChange(Number(e.target.value))} className="w-full accent-primary sm:w-72" />
    </Field>
  )
}

export function AutoRouteSection() {
  const { t } = useI18n()
  const qc = useQueryClient()
  const q = useQuery({ queryKey: ['auto-route'], queryFn: () => get<Config>('/auto-route') })
  const { draft, patch, dirty } = useDraft(q.data)
  const save = useMutation({
    mutationFn: () => post('/auto-route', draft),
    onSuccess: () => {
      toast.success(t('settings.autoRoute.saved'))
      qc.invalidateQueries({ queryKey: ['auto-route'] })
    },
    onError: (e) => toast.error(errorMessage(e, t('common.saveFailed'))),
  })
  const enabled = !!q.data?.enabled
  const dq = useQuery({
    queryKey: ['auto-route-decisions'],
    queryFn: () => get<Decisions>('/auto-route/decisions'),
    refetchInterval: enabled ? 10_000 : false,
  })
  const decisions = dq.data?.decisions?.slice(0, 20) ?? []
  const candidates = dq.data?.candidates ?? []

  return (
    <Section
      id="auto-route"
      title={t('settings.autoRoute.title')}
      description={t('settings.autoRoute.hint')}
      footer={<SaveButton onClick={() => save.mutate()} disabled={!dirty} busy={save.isPending} label={t('settings.autoRoute.save')} />}
    >
      {!draft ? (
        <LoadingBlock />
      ) : (
        <>
          <SwitchRow label={t('settings.autoRoute.enabled')} hint={t('settings.autoRoute.enabledHint')} checked={draft.enabled} onChange={(v) => patch({ enabled: v })} />
          <div className="grid gap-4 sm:grid-cols-2">
            <Slider id="ar-quality" label={t('settings.autoRoute.quality')} hint={t('settings.autoRoute.qualityHint')} value={draft.qualityWeight} max={1} onChange={(v) => patch({ qualityWeight: v })} />
            <Slider id="ar-cost" label={t('settings.autoRoute.cost')} hint={t('settings.autoRoute.costHint')} value={draft.costWeight} max={1} onChange={(v) => patch({ costWeight: v })} />
            <Slider id="ar-speed" label={t('settings.autoRoute.speed')} hint={t('settings.autoRoute.speedHint')} value={draft.speedWeight} max={1} onChange={(v) => patch({ speedWeight: v })} />
            <Slider id="ar-explore" label={t('settings.autoRoute.explore')} hint={t('settings.autoRoute.exploreHint')} value={draft.explore} max={0.5} onChange={(v) => patch({ explore: v })} />
          </div>
          <p className="text-xs text-muted-foreground">{t('settings.autoRoute.tiersHint')}</p>
          {/* Inputs are uncontrolled so a trailing comma survives typing; remount when the server copy changes. */}
          <div key={q.dataUpdatedAt} className="grid gap-4 sm:grid-cols-3">
            {TIERS.map((tier) => (
              <Field key={tier} label={t(`settings.autoRoute.tier.${tier}`)} htmlFor={`ar-${tier}`}>
                <Input id={`ar-${tier}`} defaultValue={draft[tier].join(', ')} onChange={(e) => patch({ [tier]: splitPatterns(e.target.value) } as Partial<Config>)} placeholder={t('settings.autoRoute.tierPlaceholder')} />
                <div className="flex flex-wrap gap-1">
                  {draft[tier].map((p) => (
                    <Badge key={p} variant="outline" className="font-mono">
                      {p}
                    </Badge>
                  ))}
                </div>
              </Field>
            ))}
          </div>

          <Field label={t('settings.autoRoute.blacklist')} hint={t('settings.autoRoute.blacklistHint')} htmlFor="ar-blacklist">
            <Input
              key={'bl' + q.dataUpdatedAt}
              id="ar-blacklist"
              defaultValue={(draft.blacklist ?? []).join(', ')}
              onChange={(e) => patch({ blacklist: splitPatterns(e.target.value) })}
              placeholder={t('settings.autoRoute.blacklistPlaceholder')}
            />
            <div className="flex flex-wrap gap-1">
              {(draft.blacklist ?? []).map((p) => (
                <Badge key={p} variant="destructive" className="font-mono">
                  {p}
                  <button type="button" className="ml-1 opacity-70 hover:opacity-100" onClick={() => patch({ blacklist: (draft.blacklist ?? []).filter((x) => x !== p) })}>
                    <LuX className="size-3" />
                  </button>
                </Badge>
              ))}
            </div>
          </Field>

          <div className="space-y-3 rounded-lg border p-3">
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
                      <TableCell className="font-mono" title={d.accountId}>
                        {short(d.accountId)}
                      </TableCell>
                      <TableCell className="text-right tabular-nums">{d.score.toFixed(3)}</TableCell>
                      <TableCell className="space-x-1">
                        {d.explored && <Badge variant="secondary">{t('settings.autoRoute.explored')}</Badge>}
                        {d.pinned && <Badge variant="outline">{t('settings.autoRoute.pinned')}</Badge>}
                      </TableCell>
                      <TableCell className="max-w-48 truncate text-muted-foreground" title={d.reason}>
                        {d.reason}
                      </TableCell>
                    </TableRow>
                  ))}
                </TableBody>
              </Table>
            )}

            <div className="text-sm font-medium">{t('settings.autoRoute.candidates')}</div>
            {candidates.length === 0 ? (
              <p className="py-4 text-center text-xs text-muted-foreground">{t('settings.autoRoute.noCandidates')}</p>
            ) : (
              <Table className="text-xs">
                <TableHeader>
                  <TableRow>
                    <TableHead>{t('settings.autoRoute.col.model')}</TableHead>
                    <TableHead>{t('settings.autoRoute.col.account')}</TableHead>
                    <TableHead className="text-right">{t('settings.autoRoute.col.reliability')}</TableHead>
                    <TableHead className="text-right">{t('settings.autoRoute.col.latency')}</TableHead>
                    <TableHead className="text-right">{t('settings.autoRoute.col.outcomes')}</TableHead>
                    <TableHead />
                  </TableRow>
                </TableHeader>
                <TableBody>
                  {candidates.map((c) => (
                    <TableRow key={c.accountId + c.model}>
                      <TableCell className="font-mono">{c.model}</TableCell>
                      <TableCell className="font-mono" title={c.accountId}>
                        {short(c.accountId)}
                      </TableCell>
                      <TableCell className="text-right tabular-nums">{Math.round(c.reliability * 100)}%</TableCell>
                      <TableCell className="text-right tabular-nums">{Math.round(c.ewmaLatencyMs)} ms</TableCell>
                      <TableCell className="text-right tabular-nums">
                        {Math.round(c.successes)} / {Math.round(c.failures)}
                      </TableCell>
                      <TableCell className="text-right">
                        {(() => {
                          const entry = `${c.provider || '*'}:${c.model}`
                          const listed = (draft.blacklist ?? []).some((x) => x.toLowerCase() === entry.toLowerCase())
                          return (
                            <Button size="sm" variant="ghost" className="h-6 px-2 text-[11px]" disabled={listed} title={t('settings.autoRoute.blacklistThis')} onClick={() => patch({ blacklist: [...(draft.blacklist ?? []), entry] })}>
                              <LuBan className="size-3" /> {listed ? t('settings.autoRoute.blacklisted') : t('settings.autoRoute.blacklistThis')}
                            </Button>
                          )
                        })()}
                      </TableCell>
                    </TableRow>
                  ))}
                </TableBody>
              </Table>
            )}
          </div>
        </>
      )}
    </Section>
  )
}
