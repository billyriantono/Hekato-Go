// Light helpers shared across the Overview — kept apart from charts.tsx so that
// importing them never pulls recharts into the first paint.
import { cn } from '@/lib/utils'

export type Range = '1h' | '6h' | '24h' | '7d'
export const RANGES: Range[] = ['1h', '6h', '24h', '7d']

export type Point = { t: number; requests: number; errors: number; tokens: number; credits: number; avgLatencyMs: number; p50Ms: number; p95Ms: number; cacheReadTokens?: number; cacheWriteTokens?: number }
export type Bucket = { key: string; requests: number; errors: number; tokens: number; credits: number; avgLatencyMs: number }
export type Metrics = { rangeMinutes: number; stepMinutes: number; series: Point[]; totals: Point; byModel: Bucket[]; byAccount: Bucket[]; byEndpoint: Bucket[] }

export const short = (s: string) => (s.length > 14 ? s.slice(0, 6) + '…' + s.slice(-4) : s)

/** Horizontal stacked bar; segments are [colour (css class or hex), value]. */
export function StackedBar({ segments, className }: { segments: [string, number][]; className?: string }) {
  const total = segments.reduce((n, [, v]) => n + v, 0) || 1
  return (
    <div className={cn('flex h-2 w-full gap-px overflow-hidden rounded-full bg-muted', className)}>
      {segments.map(([c, v], i) => {
        const css = c.startsWith('#') || c.startsWith('var(')
        return v > 0 ? (
          <div
            key={i}
            className={cn('origin-left animate-[grow_0.9s_var(--ease-out-expo)_both] first:rounded-l-full last:rounded-r-full', !css && c)}
            style={{ width: `${(v / total) * 100}%`, background: css ? c : undefined, animationDelay: `${i * 80}ms` }}
          />
        ) : null
      })}
    </div>
  )
}
