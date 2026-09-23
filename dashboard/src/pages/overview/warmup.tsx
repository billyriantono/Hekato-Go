import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query'
import { LuFlame, LuLoader } from 'react-icons/lu'
import { toast } from 'sonner'
import { StatusDot, errorMessage, formatTime } from '@/components/common'
import { Button } from '@/components/ui/button'
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from '@/components/ui/card'
import { get, post } from '@/lib/api'
import { useI18n } from '@/lib/i18n'
import { ago, warmupCounts, type WarmupResult } from '@/pages/accounts/shared'

type WarmupStatus = {
  running: boolean
  lastRun: number
  lastTookMs: number
  nextRun: number
  intervalMinutes: number
  probe: boolean
  recover: boolean
  summary: { Total: number; OK: number; Errors: number; Recovered: number }
  results: WarmupResult[]
}

export function WarmupCard() {
  const { t, lang } = useI18n()
  const qc = useQueryClient()
  const q = useQuery({ queryKey: ['warmup-status'], queryFn: () => get<WarmupStatus>('/warmup/status'), refetchInterval: 15000 })
  const run = useMutation({
    mutationFn: () => post<{ results: WarmupResult[] }>('/warmup', {}),
    onSuccess: (r) => toast.success(t('overview.warmup.done', ...warmupCounts(r.results))),
    onError: (e) => toast.error(errorMessage(e)),
    onSettled: () => {
      qc.invalidateQueries({ queryKey: ['warmup-status'] })
      qc.invalidateQueries({ queryKey: ['accounts'] })
    },
  })
  const d = q.data
  const running = !!d?.running || run.isPending
  const sum = d?.summary
  const cells: [string, React.ReactNode][] = [
    [t('overview.warmup.lastRun'), d?.lastRun ? `${ago(d.lastRun, lang)} · ${t('overview.warmup.took', `${((d.lastTookMs ?? 0) / 1000).toFixed(1)}s`)}` : t('overview.warmup.never')],
    [t('overview.warmup.nextRun'), d?.nextRun ? formatTime(d.nextRun) : '—'],
    [t('overview.warmup.summary'), sum ? t('overview.warmup.summaryValue', sum.OK ?? 0, sum.Errors ?? 0, sum.Recovered ?? 0) : '—'],
  ]

  return (
    <Card>
      <CardHeader className="flex flex-row items-start justify-between gap-3">
        <div>
          <CardTitle className="flex items-center gap-2">
            {t('overview.warmup.title')}
            <StatusDot tone={running ? 'warning' : 'success'} label={t(running ? 'overview.warmup.running' : 'overview.warmup.idle')} />
          </CardTitle>
          <CardDescription>{t('overview.warmup.hint', d?.intervalMinutes ?? '—')}</CardDescription>
        </div>
        <Button variant="outline" size="sm" onClick={() => run.mutate()} disabled={running}>
          {running ? <LuLoader className="size-4 animate-spin" /> : <LuFlame className="size-4" />}
          {t('overview.warmup.runNow')}
        </Button>
      </CardHeader>
      <CardContent className="grid grid-cols-1 gap-3 text-sm sm:grid-cols-3">
        {cells.map(([l, v]) => (
          <div key={l} className="rounded-md bg-muted/50 p-2">
            <div className="text-[11px] text-muted-foreground">{l}</div>
            <div className="font-medium tabular-nums">{v}</div>
          </div>
        ))}
      </CardContent>
    </Card>
  )
}
