import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query'
import { LuTrash2 } from 'react-icons/lu'
import { toast } from 'sonner'
import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'
import { Switch } from '@/components/ui/switch'
import { LoadingBlock, errorMessage } from '@/components/common'
import { get, post } from '@/lib/api'
import { useI18n } from '@/lib/i18n'
import { SaveButton, Section, SwitchRow, useDraft } from './shared'

type Rule = { id: string; name: string; type: 'regex' | 'lines-containing'; match: string; replace: string; enabled: boolean }
type Config = { filterClaudeCode: boolean; filterEnvNoise: boolean; filterStripBoundaries: boolean; rules: Rule[] }

export function PromptFilterSection() {
  const { t } = useI18n()
  const qc = useQueryClient()
  const q = useQuery({ queryKey: ['prompt-filter'], queryFn: () => get<Config>('/prompt-filter') })
  const { draft, patch, dirty } = useDraft(q.data)
  const save = useMutation({
    mutationFn: () => post('/prompt-filter', draft),
    onSuccess: () => {
      toast.success(t('settings.promptFilterSaved'))
      qc.invalidateQueries({ queryKey: ['prompt-filter'] })
    },
    onError: (e) => toast.error(errorMessage(e, t('common.saveFailed'))),
  })

  const rules = draft?.rules ?? []
  const setRule = (i: number, p: Partial<Rule>) => patch({ rules: rules.map((r, j) => (j === i ? { ...r, ...p } : r)) })
  const addRule = (type: Rule['type']) =>
    patch({ rules: [...rules, { id: crypto.randomUUID(), name: '', type, match: '', replace: '', enabled: true }] })
  const invalid = rules.some((r) => !r.match.trim())

  return (
    <Section
      id="prompt-filter"
      title={t('settings.promptFilter')}
      footer={<SaveButton onClick={() => save.mutate()} disabled={!dirty || invalid} busy={save.isPending} label={t('settings.savePromptFilter')} />}
    >
      {!draft ? (
        <LoadingBlock />
      ) : (
        <>
          <div className="text-xs font-medium uppercase tracking-wider text-muted-foreground">{t('settings.builtinFilters')}</div>
          <SwitchRow label={t('settings.filterClaudeCode')} hint={t('settings.filterClaudeCodeHint')} checked={draft.filterClaudeCode} onChange={(v) => patch({ filterClaudeCode: v })} />
          <SwitchRow label={t('settings.filterEnvNoise')} hint={t('settings.filterEnvNoiseHint')} checked={draft.filterEnvNoise} onChange={(v) => patch({ filterEnvNoise: v })} />
          <SwitchRow label={t('settings.filterStripBoundaries')} hint={t('settings.filterStripBoundariesHint')} checked={draft.filterStripBoundaries} onChange={(v) => patch({ filterStripBoundaries: v })} />

          <div className="flex flex-wrap items-center justify-between gap-2 pt-2">
            <div className="text-xs font-medium uppercase tracking-wider text-muted-foreground">{t('settings.customRules')}</div>
            <div className="flex gap-2">
              <Button size="xs" variant="outline" onClick={() => addRule('regex')}>
                {t('promptFilter.addRegex')}
              </Button>
              <Button size="xs" variant="outline" onClick={() => addRule('lines-containing')}>
                {t('promptFilter.addContains')}
              </Button>
            </div>
          </div>
          {rules.length === 0 && <p className="text-xs text-muted-foreground">{t('promptFilter.noRules')}</p>}
          {rules.map((r, i) => (
            <div key={r.id} className="space-y-2 rounded-lg border p-3">
              <div className="flex items-center gap-2">
                <Switch size="sm" checked={r.enabled} onCheckedChange={(v) => setRule(i, { enabled: v })} />
                <Input className="h-7 flex-1 text-xs" value={r.name} onChange={(e) => setRule(i, { name: e.target.value })} placeholder={t('promptFilter.unnamed')} />
                <span className="shrink-0 rounded-md bg-muted px-2 py-0.5 text-[11px] text-muted-foreground">
                  {r.type === 'regex' ? t('promptFilter.typeRegex') : t('promptFilter.typeContains')}
                </span>
                <Button size="icon-xs" variant="ghost" aria-label={t('common.remove')} onClick={() => patch({ rules: rules.filter((_, j) => j !== i) })}>
                  <LuTrash2 />
                </Button>
              </div>
              <div className="grid gap-2 sm:grid-cols-2">
                <Input
                  className="h-7 font-mono text-xs"
                  value={r.match}
                  aria-invalid={!r.match.trim()}
                  onChange={(e) => setRule(i, { match: e.target.value })}
                  placeholder={t('promptFilter.match') + ': ' + (r.type === 'regex' ? t('promptFilter.matchPlaceholderRegex') : t('promptFilter.matchPlaceholderContains'))}
                />
                {r.type === 'regex' && (
                  <Input
                    className="h-7 font-mono text-xs"
                    value={r.replace}
                    onChange={(e) => setRule(i, { replace: e.target.value })}
                    placeholder={t('promptFilter.replace') + ': ' + t('promptFilter.emptyRemove')}
                  />
                )}
              </div>
            </div>
          ))}
        </>
      )}
    </Section>
  )
}
