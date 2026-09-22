// Small helpers shared by the settings sections only.
import { useEffect, useState, type ReactNode } from 'react'
import { LuLoader } from 'react-icons/lu'
import { Button } from '@/components/ui/button'
import { Card, CardContent, CardDescription, CardFooter, CardHeader, CardTitle } from '@/components/ui/card'
import { Label } from '@/components/ui/label'
import { Switch } from '@/components/ui/switch'
import { useI18n } from '@/lib/i18n'

/** Local editable copy of server data; re-syncs whenever `data` changes (after refetch). */
export function useDraft<T>(data: T | undefined) {
  const [draft, setDraft] = useState<T | undefined>(data)
  useEffect(() => setDraft(data), [data])
  const dirty = draft !== undefined && JSON.stringify(draft) !== JSON.stringify(data)
  const patch = (p: Partial<T>) => setDraft((d) => (d ? { ...d, ...p } : d))
  return { draft, setDraft, patch, dirty }
}

export function Section({
  id,
  title,
  description,
  children,
  footer,
}: {
  id: string
  title: string
  description?: string
  children: ReactNode
  footer?: ReactNode
}) {
  return (
    <Card id={id} className="scroll-mt-20">
      <CardHeader>
        <CardTitle>{title}</CardTitle>
        {description && <CardDescription>{description}</CardDescription>}
      </CardHeader>
      <CardContent className="space-y-4">{children}</CardContent>
      {footer && <CardFooter className="justify-end gap-2">{footer}</CardFooter>}
    </Card>
  )
}

export function SaveButton({ disabled, busy, label, onClick }: { disabled?: boolean; busy?: boolean; label?: string; onClick: () => void }) {
  const { t } = useI18n()
  return (
    <Button size="sm" onClick={onClick} disabled={disabled || busy}>
      {busy && <LuLoader className="size-3.5 animate-spin" />}
      {label ?? t('common.save')}
    </Button>
  )
}

export function Field({ label, hint, htmlFor, children }: { label: string; hint?: string; htmlFor?: string; children: ReactNode }) {
  return (
    <div className="space-y-1.5">
      <Label htmlFor={htmlFor}>{label}</Label>
      {children}
      {hint && <p className="text-xs text-muted-foreground">{hint}</p>}
    </div>
  )
}

export function SwitchRow({ label, hint, checked, onChange }: { label: string; hint?: string; checked: boolean; onChange: (v: boolean) => void }) {
  return (
    <div className="flex items-start justify-between gap-4 rounded-lg border p-3">
      <div className="min-w-0">
        <div className="text-sm font-medium">{label}</div>
        {hint && <div className="mt-0.5 text-xs text-muted-foreground">{hint}</div>}
      </div>
      <Switch checked={checked} onCheckedChange={(v) => onChange(v)} />
    </div>
  )
}
