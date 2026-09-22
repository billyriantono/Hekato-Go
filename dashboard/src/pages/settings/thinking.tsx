import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query'
import { toast } from 'sonner'
import { Input } from '@/components/ui/input'
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from '@/components/ui/select'
import { LoadingBlock, errorMessage } from '@/components/common'
import { get, post } from '@/lib/api'
import { useI18n } from '@/lib/i18n'
import { Field, SaveButton, Section, useDraft } from './shared'

type Thinking = { suffix: string; openaiFormat: string; claudeFormat: string }
// '' = no tag; the Select needs a non-empty item value so map it to 'none'.
const NONE = 'none'

export function ThinkingSection() {
  const { t } = useI18n()
  const qc = useQueryClient()
  const q = useQuery({ queryKey: ['thinking'], queryFn: () => get<Thinking>('/thinking') })
  const { draft, patch, dirty } = useDraft(q.data)
  const save = useMutation({
    mutationFn: () => post('/thinking', draft),
    onSuccess: () => {
      toast.success(t('settings.thinkingSaved'))
      qc.invalidateQueries({ queryKey: ['thinking'] })
    },
    onError: (e) => toast.error(errorMessage(e, t('common.saveFailed'))),
  })

  const formats: Record<string, string> = {
    [NONE]: t('settings.noTag'),
    reasoning_content: t('settings.formatReasoningContent'),
    thinking: t('settings.formatThinkingClaude'),
    think: t('settings.formatThinkOpenAI'),
  }
  const formatSelect = (value: string, onChange: (v: string) => void) => (
    <Select items={formats} value={value || NONE} onValueChange={(v) => onChange(v === NONE ? '' : String(v))}>
      <SelectTrigger className="w-full sm:w-72">
        <SelectValue />
      </SelectTrigger>
      <SelectContent>
        {Object.entries(formats).map(([v, label]) => (
          <SelectItem key={v} value={v}>
            {label}
          </SelectItem>
        ))}
      </SelectContent>
    </Select>
  )

  return (
    <Section
      id="thinking"
      title={t('settings.thinkingSettings')}
      description={t('settings.thinkingHint')}
      footer={<SaveButton onClick={() => save.mutate()} disabled={!dirty} busy={save.isPending} label={t('settings.saveThinking')} />}
    >
      {!draft ? (
        <LoadingBlock />
      ) : (
        <>
          <Field label={t('settings.thinkingSuffix')} hint={t('settings.thinkingSuffixHint')} htmlFor="th-suffix">
            <Input id="th-suffix" className="sm:w-72" value={draft.suffix} onChange={(e) => patch({ suffix: e.target.value })} placeholder="-thinking" />
          </Field>
          <Field label={t('settings.openaiFormat')} hint={t('settings.openaiFormatHint')}>
            {formatSelect(draft.openaiFormat, (v) => patch({ openaiFormat: v }))}
          </Field>
          <Field label={t('settings.claudeFormat')} hint={t('settings.claudeFormatHint')}>
            {formatSelect(draft.claudeFormat, (v) => patch({ claudeFormat: v }))}
          </Field>
        </>
      )}
    </Section>
  )
}
