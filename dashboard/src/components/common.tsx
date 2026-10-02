// Shared building blocks used by every page. Keep this file small and generic.
import { useState, type ReactNode } from 'react'
import { LuCheck, LuCopy, LuLoader } from 'react-icons/lu'
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
import { NAV, navIndex } from '@/components/layout/nav'
import { useI18n } from '@/lib/i18n'
import { cn } from '@/lib/utils'

/** Page masthead: section eyebrow, serif display title, description, actions. */
export function PageHeader({ title, description, actions }: { title: string; description?: string; actions?: ReactNode }) {
  const { t } = useI18n()
  const i = navIndex(window.location.pathname)
  return (
    <header className="mb-8 flex flex-wrap items-end justify-between gap-x-6 gap-y-4 border-b pb-6">
      <div className="min-w-0">
        {i >= 0 && (
          <div className="eyebrow mb-3 flex items-center gap-2">
            <span className="text-signal">§ {String(i + 1).padStart(2, '0')}</span>
            <span className="h-px w-6 bg-border" />
            <span>{t(NAV[i].section === 'manage' ? 'nav.section.manage' : 'nav.section.system')}</span>
          </div>
        )}
        <h1 className="display text-[2.6rem] text-foreground md:text-[3rem]">{title}</h1>
        {description && <p className="mt-3 max-w-2xl text-[13.5px] leading-relaxed text-muted-foreground">{description}</p>}
      </div>
      {actions && <div className="flex flex-wrap items-center gap-2">{actions}</div>}
    </header>
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
    default: 'text-foreground',
    success: 'text-success-ink',
    warning: 'text-warning-ink',
    danger: 'text-destructive',
  }[tone ?? 'default']
  return (
    <Card
      size="sm"
      className="stat group/stat relative transition-transform duration-500 ease-(--ease-out-expo) hover:-translate-y-0.5"
      // A status card speaks its status colour; neutral cards take the palette cycle.
      style={tone && tone !== 'default' ? ({ '--c': `var(--${tone === 'danger' ? 'destructive' : tone})` } as React.CSSProperties) : undefined}
    >
      {/* Each card in a row takes the next palette colour (see .stat in index.css). */}
      <span aria-hidden="true" className="absolute inset-x-6 top-0 h-px bg-gradient-to-r from-transparent via-(--c) to-transparent opacity-80" />
      <CardContent className="flex min-h-[92px] flex-col justify-between gap-3">
        <div className="flex items-center justify-between gap-2">
          <div className="eyebrow min-w-0 truncate text-[10px] tracking-[0.09em]" title={label}>{label}</div>
          {icon && (
            <div className="grid size-7 shrink-0 place-items-center rounded-lg bg-[color-mix(in_oklch,var(--c)_15%,transparent)] text-(--c) transition-transform duration-500 ease-(--ease-spring) group-hover/stat:scale-110 group-hover/stat:-rotate-6 [&_svg]:size-3.5">
              {icon}
            </div>
          )}
        </div>
        <div className="min-w-0">
          <div className={cn('truncate font-mono text-[1.6rem] leading-none font-medium tracking-[-0.03em] tabular-nums', toneCls)}>{value}</div>
          {/* Hint row is always reserved so values line up across a row of cards. */}
          <div className="mt-1.5 min-h-[17px] text-[11.5px] text-muted-foreground">{hint}</div>
        </div>
      </CardContent>
    </Card>
  )
}

/** Idle crossroads: the roads are there, nothing is travelling yet. */
function IdleRoads() {
  return (
    <svg viewBox="0 0 120 48" className="mb-4 h-12 w-[120px] text-muted-foreground/35" fill="none" aria-hidden="true">
      <g stroke="currentColor" strokeWidth="1.3" strokeLinecap="round" strokeDasharray="2 5" className="[animation:flow_3s_linear_infinite]">
        <path d="M4 8c34 0 40 16 76 16" />
        <path d="M4 24h76" />
        <path d="M4 40c34 0 40-16 76-16" />
      </g>
      <circle cx="90" cy="24" r="6" stroke="currentColor" strokeWidth="1.3" />
      <circle cx="90" cy="24" r="2" fill="currentColor" />
    </svg>
  )
}

export function EmptyState({ title, description, action }: { title: string; description?: string; action?: ReactNode }) {
  return (
    <div className="flex animate-[fade_0.6s_var(--ease-out-expo)_both] flex-col items-center justify-center rounded-xl border border-dashed bg-[repeating-linear-gradient(135deg,transparent_0_10px,color-mix(in_oklch,var(--foreground)_2.5%,transparent)_10px_11px)] px-6 py-14 text-center">
      <IdleRoads />
      <div className="text-sm font-medium">{title}</div>
      {description && <div className="mt-1 max-w-sm text-xs text-muted-foreground">{description}</div>}
      {action && <div className="mt-4">{action}</div>}
    </div>
  )
}

/** Shimmering placeholder rows — reads as content arriving, not as waiting. */
export function LoadingBlock({ label, rows = 3 }: { label?: string; rows?: number }) {
  const { t } = useI18n()
  return (
    <div className="space-y-3 py-4" role="status" aria-label={label ?? t('common.loading')}>
      {Array.from({ length: rows }, (_, i) => (
        <div key={i} className="flex items-center gap-3">
          <div className="skeleton size-7 shrink-0 rounded-md" />
          <div className="flex-1 space-y-1.5">
            <div className="skeleton h-2.5 rounded-full" style={{ width: `${72 - i * 14}%` }} />
            <div className="skeleton h-2 w-1/3 rounded-full opacity-70" />
          </div>
        </div>
      ))}
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
      <span className="relative grid size-3.5 place-items-center [&>svg]:col-start-1 [&>svg]:row-start-1 [&>svg]:size-3.5 [&>svg]:transition-all [&>svg]:duration-300 [&>svg]:ease-(--ease-spring)">
        <LuCopy className={cn(done && 'scale-50 rotate-[-30deg] opacity-0')} />
        <LuCheck className={cn('text-success', !done && 'scale-50 rotate-[30deg] opacity-0')} />
      </span>
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
    success: 'text-success',
    warning: 'text-warning',
    danger: 'text-destructive',
    muted: 'text-muted-foreground/40',
  }[tone]
  return (
    <span className="inline-flex items-center gap-1.5 text-xs">
      <span className={cn('lamp size-1.5', cls, tone !== 'muted' && 'shadow-[0_0_6px_currentColor]')} data-live={tone === 'danger'} />
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

/** Compact duration: 850ms, 2.6s, 14s. */
export function formatMs(v: number | undefined | null): string {
  const n = Number(v ?? 0)
  return n >= 1000 ? `${(n / 1000).toFixed(n >= 10_000 ? 0 : 1).replace(/\.0$/, '')}s` : `${Math.round(n)}ms`
}

export function formatTime(unixSeconds: number | undefined | null): string {
  if (!unixSeconds) return '—'
  return new Date(unixSeconds * 1000).toLocaleString()
}

export function errorMessage(e: unknown, fallback = 'Unknown error'): string {
  return e instanceof Error ? e.message : fallback
}
