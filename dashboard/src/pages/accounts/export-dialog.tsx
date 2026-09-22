import { useState } from 'react'
import { LuDownload, LuLoader } from 'react-icons/lu'
import { toast } from 'sonner'
import { Button } from '@/components/ui/button'
import { Checkbox } from '@/components/ui/checkbox'
import { Dialog, DialogContent, DialogDescription, DialogFooter, DialogHeader, DialogTitle } from '@/components/ui/dialog'
import { ScrollArea } from '@/components/ui/scroll-area'
import { CopyButton, errorMessage } from '@/components/common'
import { post } from '@/lib/api'
import { useI18n } from '@/lib/i18n'
import { downloadJson, type Account } from './shared'

export function ExportDialog({
  open,
  onOpenChange,
  accounts,
  preselected,
}: {
  open: boolean
  onOpenChange: (o: boolean) => void
  accounts: Account[]
  preselected: string[]
}) {
  const { t } = useI18n()
  const [ids, setIds] = useState<Set<string>>(new Set())
  const [json, setJson] = useState('')
  const [busy, setBusy] = useState(false)

  // Reset selection/output each time the dialog opens (derived-state pattern, no effect).
  const [wasOpen, setWasOpen] = useState(open)
  if (open !== wasOpen) {
    setWasOpen(open)
    if (open) {
      setIds(new Set(preselected.length ? preselected : accounts.map((a) => a.id)))
      setJson('')
    }
  }

  const toggle = (id: string, on: boolean) =>
    setIds((s) => {
      const n = new Set(s)
      if (on) n.add(id)
      else n.delete(id)
      return n
    })

  const exportNow = async () => {
    if (!ids.size) return toast.error(t('export.noSelection'))
    setBusy(true)
    try {
      const doc = await post('/export', { ids: [...ids] })
      setJson(JSON.stringify(doc, null, 2))
    } catch (e) {
      toast.error(errorMessage(e))
    } finally {
      setBusy(false)
    }
  }
  const filename = `hekato-accounts-${new Date().toISOString().slice(0, 10)}.json`

  return (
    <Dialog open={open} onOpenChange={onOpenChange}>
      <DialogContent className="sm:max-w-lg">
        <DialogHeader>
          <DialogTitle>{t('export.title')}</DialogTitle>
          <DialogDescription>{t('accounts.exportHint')}</DialogDescription>
        </DialogHeader>
        {json ? (
          <ScrollArea className="h-72 rounded-md border bg-muted/40">
            <pre className="p-3 font-mono text-[11px] leading-relaxed whitespace-pre">{json}</pre>
          </ScrollArea>
        ) : (
          <div className="space-y-2">
            <div className="flex items-center justify-between text-xs">
              <span className="text-muted-foreground">{t('export.selected', ids.size)}</span>
              <div className="flex gap-1">
                <Button variant="ghost" size="xs" onClick={() => setIds(new Set(accounts.map((a) => a.id)))}>
                  {t('export.selectAll')}
                </Button>
                <Button variant="ghost" size="xs" onClick={() => setIds(new Set())}>
                  {t('export.deselectAll')}
                </Button>
              </div>
            </div>
            <ScrollArea className="h-64 rounded-md border">
              <div className="divide-y">
                {accounts.map((a) => (
                  <label key={a.id} className="flex cursor-pointer items-center gap-3 px-3 py-2 text-sm hover:bg-muted/50">
                    <Checkbox checked={ids.has(a.id)} onCheckedChange={(v) => toggle(a.id, !!v)} />
                    <span className="min-w-0 flex-1 truncate">{a.nickname || a.email}</span>
                    <span className="text-xs text-muted-foreground">{a.provider}</span>
                  </label>
                ))}
              </div>
            </ScrollArea>
          </div>
        )}
        <DialogFooter>
          {json ? (
            <>
              <Button variant="outline" onClick={() => setJson('')}>
                {t('common.back')}
              </Button>
              <CopyButton value={json} size="sm" label={t('export.copyJson')} />
              <Button onClick={() => downloadJson(filename, json)}>
                <LuDownload /> {t('export.downloadJson')}
              </Button>
            </>
          ) : (
            <>
              <Button variant="outline" onClick={() => onOpenChange(false)}>
                {t('common.cancel')}
              </Button>
              <Button onClick={exportNow} disabled={busy || !ids.size}>
                {busy && <LuLoader className="animate-spin" />} {t('export.showJson')}
              </Button>
            </>
          )}
        </DialogFooter>
      </DialogContent>
    </Dialog>
  )
}
