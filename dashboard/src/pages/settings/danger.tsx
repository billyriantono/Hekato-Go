import { useState } from 'react'
import { useQueryClient } from '@tanstack/react-query'
import { toast } from 'sonner'
import { Button } from '@/components/ui/button'
import { ConfirmDialog, errorMessage } from '@/components/common'
import { del, post } from '@/lib/api'
import { useI18n } from '@/lib/i18n'
import { Section } from './shared'

export function DangerSection() {
  const { t } = useI18n()
  const qc = useQueryClient()
  const [open, setOpen] = useState<'stats' | 'logs' | null>(null)

  const run = async () => {
    try {
      if (open === 'stats') {
        await post('/stats/reset')
        toast.success(t('settings.statsReset'))
        qc.invalidateQueries({ queryKey: ['status'] })
        qc.invalidateQueries({ queryKey: ['stats'] })
      } else {
        await del('/logs')
        toast.success(t('logs.cleared'))
        qc.invalidateQueries({ queryKey: ['logs'] })
      }
    } catch (e) {
      toast.error(errorMessage(e, t('common.failed')))
    }
  }

  const row = (label: string, hint: string, kind: 'stats' | 'logs') => (
    <div className="flex flex-wrap items-center justify-between gap-3 rounded-lg border border-destructive/30 p-3">
      <div>
        <div className="text-sm font-medium">{label}</div>
        <div className="mt-0.5 text-xs text-muted-foreground">{hint}</div>
      </div>
      <Button size="sm" variant="destructive" onClick={() => setOpen(kind)}>
        {label}
      </Button>
    </div>
  )

  return (
    <Section id="danger" title={t('settings.dangerZone')} description={t('settings.dangerZoneHint')}>
      {row(t('settings.resetStats'), t('settings.resetStatsHint'), 'stats')}
      {row(t('settings.clearLogs'), t('settings.clearLogsHint'), 'logs')}
      <ConfirmDialog
        open={open !== null}
        onOpenChange={(o) => !o && setOpen(null)}
        title={open === 'stats' ? t('settings.confirmReset') : t('logs.clearConfirm')}
        destructive
        onConfirm={run}
      />
    </Section>
  )
}
