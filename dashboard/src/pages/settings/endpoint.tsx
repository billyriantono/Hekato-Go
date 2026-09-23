import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query'
import { toast } from 'sonner'
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from '@/components/ui/select'
import { LoadingBlock, errorMessage } from '@/components/common'
import { get, post } from '@/lib/api'
import { useI18n } from '@/lib/i18n'
import { Field, SaveButton, Section, SwitchRow, useDraft } from './shared'

type Endpoint = { preferredEndpoint: string; endpointFallback: boolean }

export function EndpointSection() {
  const { t } = useI18n()
  const qc = useQueryClient()
  const q = useQuery({ queryKey: ['endpoint'], queryFn: () => get<Endpoint>('/endpoint') })
  const { draft, patch, dirty } = useDraft(q.data)
  const save = useMutation({
    mutationFn: () => post('/endpoint', draft),
    onSuccess: () => {
      toast.success(t('settings.endpointSaved'))
      qc.invalidateQueries({ queryKey: ['endpoint'] })
    },
    onError: (e) => toast.error(errorMessage(e, t('common.saveFailed'))),
  })
  const endpoints: Record<string, string> = {
    auto: t('settings.endpointAuto'),
    kiro: t('settings.endpointKiro'),
    codewhisperer: t('settings.endpointCodeWhisperer'),
    amazonq: t('settings.endpointAmazonQ'),
  }

  return (
    <Section
      id="endpoint"
      title={t('settings.endpointSettings')}
      footer={<SaveButton onClick={() => save.mutate()} disabled={!dirty} busy={save.isPending} label={t('settings.saveEndpoint')} />}
    >
      {!draft ? (
        <LoadingBlock />
      ) : (
        <>
          <Field label={t('settings.preferredEndpoint')} hint={t('settings.endpointHint')}>
            <Select items={endpoints} value={draft.preferredEndpoint} onValueChange={(v) => patch({ preferredEndpoint: String(v) })}>
              <SelectTrigger className="w-full sm:w-72">
                <SelectValue />
              </SelectTrigger>
              <SelectContent>
                {Object.entries(endpoints).map(([v, label]) => (
                  <SelectItem key={v} value={v}>
                    {label}
                  </SelectItem>
                ))}
              </SelectContent>
            </Select>
          </Field>
          <SwitchRow
            label={t('settings.endpointFallback')}
            hint={t('settings.endpointFallbackHint')}
            checked={draft.endpointFallback}
            onChange={(v) => patch({ endpointFallback: v })}
          />
        </>
      )}
    </Section>
  )
}
