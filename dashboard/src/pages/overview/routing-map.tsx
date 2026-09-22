import { useQuery } from '@tanstack/react-query'
import { useMemo } from 'react'
import { EmptyState, LoadingBlock, errorMessage, formatNumber } from '@/components/common'
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from '@/components/ui/card'
import { get } from '@/lib/api'
import { useI18n } from '@/lib/i18n'

type RequestLog = {
  time: number
  endpoint: string
  model: string
  status: 'success' | 'error'
}

type RouteNode = {
  key: string
  requests: number
  successes: number
  failures: number
  hiddenModels?: number
}

const WINDOW_SECONDS = 10 * 60
const REFETCH_MS = 3000
const MAX_MODEL_NODES = 8

function aggregate(logs: RequestLog[]) {
  const cutoff = Date.now() / 1000 - WINDOW_SECONDS
  const recent = logs.filter((log) => log.time >= cutoff)
  const group = (key: (log: RequestLog) => string) => {
    const rows = new Map<string, RouteNode>()
    for (const log of recent) {
      const value = key(log) || 'unknown'
      const row = rows.get(value) ?? { key: value, requests: 0, successes: 0, failures: 0 }
      row.requests++
      if (log.status === 'success') row.successes++
      else row.failures++
      rows.set(value, row)
    }
    return [...rows.values()].sort((a, b) => b.requests - a.requests || a.key.localeCompare(b.key))
  }
  return { recent, sources: group((log) => log.endpoint), models: group((log) => log.model) }
}

function compactModels(models: RouteNode[]) {
  if (models.length <= MAX_MODEL_NODES) return models
  const visible = models.slice(0, MAX_MODEL_NODES - 1)
  const hidden = models.slice(MAX_MODEL_NODES - 1)
  visible.push({
    key: '__other_models__',
    requests: hidden.reduce((sum, model) => sum + model.requests, 0),
    successes: hidden.reduce((sum, model) => sum + model.successes, 0),
    failures: hidden.reduce((sum, model) => sum + model.failures, 0),
    hiddenModels: hidden.length,
  })
  return visible
}

function laneY(index: number, count: number, height: number) {
  if (count <= 1) return height / 2
  return 70 + (index * (height - 140)) / (count - 1)
}

function endpointLabel(endpoint: string) {
  if (endpoint === 'claude') return 'Claude API'
  if (endpoint === 'openai') return 'OpenAI API'
  if (endpoint === 'responses') return 'Responses API'
  return endpoint
}

function tone(node: RouteNode) {
  if (!node.failures) return '#34d399'
  if (!node.successes) return '#fb7185'
  return '#fbbf24'
}

export function LiveRoutingMap() {
  const { t } = useI18n()
  const query = useQuery({
    queryKey: ['logs'],
    queryFn: () => get<{ logs: RequestLog[] }>('/logs'),
    refetchInterval: REFETCH_MS,
  })
  const routes = useMemo(() => aggregate(query.data?.logs ?? []), [query.data])
  const displayedModels = useMemo(() => compactModels(routes.models), [routes.models])
  const height = Math.max(360, Math.max(routes.sources.length, displayedModels.length) * 78 + 90)
  const totalFailures = routes.recent.reduce((sum, log) => sum + (log.status === 'error' ? 1 : 0), 0)
  const routerY = height / 2

  return (
    <Card className="overflow-hidden border-emerald-500/20 bg-card/95 shadow-[0_0_35px_-24px_rgba(16,185,129,0.7)]">
      <CardHeader className="flex flex-row items-start justify-between gap-4 border-b border-emerald-500/10">
        <div>
          <div className="mb-2 flex items-center gap-2">
            <span className="relative flex size-2.5" aria-hidden="true">
              <span className="absolute inline-flex size-full animate-ping rounded-full bg-emerald-400 opacity-50 motion-reduce:animate-none" />
              <span className="relative inline-flex size-2.5 rounded-full bg-emerald-400" />
            </span>
            <span className="font-mono text-[10px] font-semibold tracking-[0.2em] text-emerald-500 uppercase">{t('overview.routing.live')}</span>
          </div>
          <CardTitle>{t('overview.routing.title')}</CardTitle>
          <CardDescription>{t('overview.routing.hint')}</CardDescription>
        </div>
        <div className="shrink-0 text-right font-mono text-[10px] leading-5 text-muted-foreground">
          <div>{t('overview.routing.refresh')}</div>
          {query.dataUpdatedAt > 0 && <div>{new Date(query.dataUpdatedAt).toLocaleTimeString()}</div>}
        </div>
      </CardHeader>
      <CardContent className="p-0">
        {query.isPending ? (
          <div className="p-6"><LoadingBlock /></div>
        ) : query.isError ? (
          <div className="p-6"><EmptyState title={t('overview.routing.loadFailed', errorMessage(query.error))} /></div>
        ) : routes.recent.length === 0 ? (
          <div className="p-6"><EmptyState title={t('overview.routing.empty')} /></div>
        ) : (
          <div className="relative overflow-x-auto bg-[radial-gradient(circle_at_50%_50%,rgba(16,185,129,0.08),transparent_42%)]">
            <svg
              className="block min-w-[920px] text-foreground"
              style={{ height: `${height}px`, width: '100%' }}
              viewBox={`0 0 1000 ${height}`}
              role="img"
              aria-label={t('overview.routing.aria', routes.recent.length, routes.models.length)}
            >
              <defs>
                <pattern id="routing-grid" width="24" height="24" patternUnits="userSpaceOnUse">
                  <path d="M 24 0 L 0 0 0 24" fill="none" stroke="var(--border)" strokeWidth="0.7" opacity="0.55" />
                </pattern>
                <filter id="router-glow" x="-50%" y="-50%" width="200%" height="200%">
                  <feGaussianBlur stdDeviation="8" result="blur" />
                  <feMerge><feMergeNode in="blur" /><feMergeNode in="SourceGraphic" /></feMerge>
                </filter>
              </defs>
              <style>{`
                .routing-flow { animation: routing-flow 2.2s linear infinite; }
                @keyframes routing-flow { to { stroke-dashoffset: -28; } }
                @media (prefers-reduced-motion: reduce) { .routing-flow { animation: none; } }
              `}</style>
              <rect width="1000" height={height} fill="url(#routing-grid)" opacity="0.7" />

              <text x="42" y="30" fill="var(--muted-foreground)" fontSize="10" fontWeight="700" letterSpacing="2">{t('overview.routing.sources').toUpperCase()}</text>
              <text x="405" y="30" fill="var(--muted-foreground)" fontSize="10" fontWeight="700" letterSpacing="2">{t('overview.routing.gateway').toUpperCase()}</text>
              <text x="718" y="30" fill="var(--muted-foreground)" fontSize="10" fontWeight="700" letterSpacing="2">{t('overview.routing.destinations').toUpperCase()}</text>

              {routes.sources.map((source, index) => {
                const y = laneY(index, routes.sources.length, height)
                const color = tone(source)
                const path = `M 250 ${y} C 318 ${y}, 334 ${routerY}, 402 ${routerY}`
                return (
                  <g key={source.key}>
                    <path d={path} fill="none" stroke={color} strokeWidth="6" opacity="0.08" />
                    <path className="routing-flow" d={path} fill="none" stroke={color} strokeWidth="1.5" strokeDasharray="5 9" opacity="0.85" />
                    <rect x="42" y={y - 31} width="208" height="62" rx="9" fill="var(--card)" stroke={color} strokeOpacity="0.55" />
                    <circle cx="60" cy={y - 10} r="4" fill={color} />
                    <text x="72" y={y - 6} fill="currentColor" fontSize="13" fontWeight="650">{endpointLabel(source.key)}</text>
                    <text x="60" y={y + 16} fill="var(--muted-foreground)" fontSize="10" fontFamily="monospace">
                      {t('overview.routing.requestCount', formatNumber(source.requests))}
                    </text>
                  </g>
                )
              })}

              {displayedModels.map((model, index) => {
                const y = laneY(index, displayedModels.length, height)
                const color = tone(model)
                const path = `M 598 ${routerY} C 662 ${routerY}, 652 ${y}, 718 ${y}`
                return (
                  <g key={model.key}>
                    <path d={path} fill="none" stroke={color} strokeWidth="6" opacity="0.08" />
                    <path className="routing-flow" d={path} fill="none" stroke={color} strokeWidth="1.5" strokeDasharray="5 9" opacity="0.85" />
                    <rect x="718" y={y - 31} width="240" height="62" rx="9" fill="var(--card)" stroke={color} strokeOpacity="0.62" />
                    <circle cx="736" cy={y - 11} r="4" fill={color} />
                    <text x="748" y={y - 7} fill="currentColor" fontSize="11" fontWeight="650" fontFamily="monospace">
                      {model.hiddenModels ? t('overview.routing.otherModels', model.hiddenModels) : model.key}
                    </text>
                    <text x="736" y={y + 16} fill="var(--muted-foreground)" fontSize="10" fontFamily="monospace">
                      {t('overview.routing.modelStats', formatNumber(model.requests), formatNumber(model.successes), formatNumber(model.failures))}
                    </text>
                  </g>
                )
              })}

              <g filter="url(#router-glow)">
                <rect x="402" y={routerY - 62} width="196" height="124" rx="14" fill="var(--card)" stroke="#34d399" strokeWidth="1.5" />
                <rect x="414" y={routerY - 50} width="172" height="100" rx="10" fill="none" stroke="#34d399" strokeOpacity="0.2" />
                <circle cx="500" cy={routerY - 29} r="5" fill="#34d399" />
                <text x="500" y={routerY - 8} textAnchor="middle" fill="currentColor" fontSize="17" fontWeight="750" letterSpacing="2">HEKATO</text>
                <text x="500" y={routerY + 9} textAnchor="middle" fill="#34d399" fontSize="9" fontFamily="monospace" letterSpacing="2.5">ROUTER</text>
                <text x="500" y={routerY + 34} textAnchor="middle" fill="var(--muted-foreground)" fontSize="10" fontFamily="monospace">
                  {t('overview.routing.routerStats', formatNumber(routes.recent.length), formatNumber(totalFailures))}
                </text>
              </g>
            </svg>
          </div>
        )}
      </CardContent>
    </Card>
  )
}
