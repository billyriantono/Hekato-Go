// Shared building blocks used by every page. Keep this file small and generic.
import { useState, type ReactNode } from 'react'
import { LuCheck, LuCopy, LuInbox, LuLoader } from 'react-icons/lu'
import { toast } from 'sonner'
import { Button } from '@/components/ui/button'
import { Card, CardContent } from '@/components/ui/card'
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogFooter,
  DialogHeader,
  DialogTitle,
} from '@/components/ui/dialog'
import { useI18n } from '@/lib/i18n'
import { cn } from '@/lib/utils'

/** Page title row with optional description and right-aligned actions. */
export function PageHeader({ title, description, actions }: { title: string; description?: string; actions?: ReactNode }) {
  return (
    <div className="mb-6 flex flex-wrap items-start justify-between gap-3">
      <div>
        <h1 className="text-2xl font-semibold tracking-tight">{title}</h1>
        {description && <p className="mt-1 text-sm text-muted-foreground">{description}</p>}
      </div>
      {actions && <div className="flex flex-wrap items-center gap-2">{actions}</div>}
    </div>
  )
}

/** KPI tile. `tone` colours the value; `hint` is small muted text under it. */
export function StatCard({
  label,
  value,
  hint,
  icon,
  tone,
}: {
  label: string
  value: ReactNode
  hint?: ReactNode
  icon?: ReactNode
  tone?: 'default' | 'success' | 'warning' | 'danger'
}) {
  const toneCls = {
    default: '',
    success: 'text-emerald-600 dark:text-emerald-400',
    warning: 'text-amber-600 dark:text-amber-400',
    danger: 'text-red-600 dark:text-red-400',
  }[tone ?? 'default']
  return (
    <Card size="sm">
      <CardContent className="flex items-start justify-between gap-3">
        <div className="min-w-0">
          <div className="text-xs font-medium text-muted-foreground">{label}</div>
          <div className={cn('mt-1 truncate text-2xl font-semibold tabular-nums', toneCls)}>{value}</div>
          {hint && <div className="mt-1 text-xs text-muted-foreground">{hint}</div>}
        </div>
        {icon && <div className="rounded-md bg-muted p-2 text-muted-foreground">{icon}</div>}
      </CardContent>
    </Card>
  )
}

export function EmptyState({ title, description, action }: { title: string; description?: string; action?: ReactNode }) {
  return (
    <div className="flex flex-col items-center justify-center rounded-lg border border-dashed px-6 py-14 text-center">
      <LuInbox className="mb-3 size-8 text-muted-foreground/60" />
      <div className="text-sm font-medium">{title}</div>
      {description && <div className="mt-1 max-w-sm text-xs text-muted-foreground">{description}</div>}
      {action && <div className="mt-4">{action}</div>}
    </div>
  )
}

export function LoadingBlock({ label }: { label?: string }) {
  const { t } = useI18n()
  return (
    <div className="flex items-center justify-center gap-2 py-14 text-sm text-muted-foreground">
      <LuLoader className="size-4 animate-spin" /> {label ?? t('common.loading')}
    </div>
  )
}

/** Copies `value` to the clipboard with a toast. */
export function CopyButton({ value, size = 'icon-sm', label }: { value: string; size?: 'icon-sm' | 'sm'; label?: string }) {
  const { t } = useI18n()
  const [done, setDone] = useState(false)
  const copy = async () => {
    try {
      await navigator.clipboard.writeText(value)
      setDone(true)
      toast.success(t('common.copied'))
      setTimeout(() => setDone(false), 1500)
    } catch {
      toast.error(t('common.failed'))
    }
  }
  return (
    <Button type="button" variant="ghost" size={size} onClick={copy} aria-label={t('common.copy')}>
      {done ? <LuCheck className="size-3.5" /> : <LuCopy className="size-3.5" />}
      {label && <span>{label}</span>}
    </Button>
  )
}

/**
 * Controlled confirmation dialog. Usage:
 *   const [open, setOpen] = useState(false)
 *   <ConfirmDialog open={open} onOpenChange={setOpen} title=... onConfirm={async () => ...} />
 */
export function ConfirmDialog({
  open,
  onOpenChange,
  title,
  description,
  confirmLabel,
  destructive,
  onConfirm,
}: {
  open: boolean
  onOpenChange: (o: boolean) => void
  title: string
  description?: ReactNode
  confirmLabel?: string
  destructive?: boolean
  onConfirm: () => Promise<void> | void
}) {
  const { t } = useI18n()
  const [busy, setBusy] = useState(false)
  const run = async () => {
    setBusy(true)
    try {
      await onConfirm()
      onOpenChange(false)
    } finally {
      setBusy(false)
    }
  }
  return (
    <Dialog open={open} onOpenChange={onOpenChange}>
      <DialogContent className="sm:max-w-md">
        <DialogHeader>
          <DialogTitle>{title}</DialogTitle>
          {description && <DialogDescription>{description}</DialogDescription>}
        </DialogHeader>
        <DialogFooter>
          <Button variant="outline" onClick={() => onOpenChange(false)} disabled={busy}>
            {t('common.cancel')}
          </Button>
          <Button variant={destructive ? 'destructive' : 'default'} onClick={run} disabled={busy}>
            {busy && <LuLoader className="size-4 animate-spin" />}
            {confirmLabel ?? t('common.confirm')}
          </Button>
        </DialogFooter>
      </DialogContent>
    </Dialog>
  )
}

/** Small coloured status pill. */
export function StatusDot({ tone, label }: { tone: 'success' | 'warning' | 'danger' | 'muted'; label: string }) {
  const cls = {
    success: 'bg-emerald-500',
    warning: 'bg-amber-500',
    danger: 'bg-red-500',
    muted: 'bg-muted-foreground/40',
  }[tone]
  return (
    <span className="inline-flex items-center gap-1.5 text-xs">
      <span className={cn('size-2 rounded-full', cls)} />
      {label}
    </span>
  )
}

export function formatNumber(n: number | undefined | null): string {
  if (n === undefined || n === null || Number.isNaN(n)) return '0'
  if (Math.abs(n) >= 1_000_000) return (n / 1_000_000).toFixed(2).replace(/\.?0+$/, '') + 'M'
  if (Math.abs(n) >= 10_000) return (n / 1_000).toFixed(1).replace(/\.?0+$/, '') + 'K'
  return n.toLocaleString()
}

export function formatTime(unixSeconds: number | undefined | null): string {
  if (!unixSeconds) return '—'
  return new Date(unixSeconds * 1000).toLocaleString()
}

export function errorMessage(e: unknown, fallback = 'Unknown error'): string {
  return e instanceof Error ? e.message : fallback
}
