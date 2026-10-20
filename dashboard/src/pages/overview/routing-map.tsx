import { useQuery } from '@tanstack/react-query'
import { useMemo } from 'react'
import { EmptyState, LoadingBlock, errorMessage, formatNumber } from '@/components/common'
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from '@/components/ui/card'
import { iconUrl } from '@/components/brand'
import { get } from '@/lib/api'
import { useI18n } from '@/lib/i18n'

export type RequestLog = {
  time: number
  endpoint: string
  model: string
  status: 'success' | 'error'
  requestedModel?: string
  apiKeyId?: string
  pool?: string
}

type RouteNode = {
  key: string
  requests: number
  successes: number
  failures: number
  hiddenModels?: number
  endpoints?: string[]
}

const WINDOW_SECONDS = 10 * 60
const REFETCH_MS = 3000
const MAX_MODEL_NODES = 8
// ponytail: five theme colours, cycled; a sixth concurrent key repeats the first.
const KEY_COLORS = ['var(--chart-1)', 'var(--chart-2)', 'var(--chart-3)', 'var(--chart-4)', 'var(--chart-5)']
const OTHER_MODELS = '__other_models__'

const sourceKey = (log: RequestLog) => log.apiKeyId || ''
const modelKey = (log: RequestLog) => (log.pool ? `${log.pool}/${log.model}` : log.model)

function aggregate(logs: RequestLog[]) {
  const cutoff = Date.now() / 1000 - WINDOW_SECONDS
  // Only traffic that asked for the virtual "auto" model: the map explains
  // where the router sent it, explicit model requests are just pass-through.
  const recent = logs.filter((log) => log.time >= cutoff && log.requestedModel === 'auto')
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
  const sources = group(sourceKey)
  for (const source of sources) {
    source.endpoints = [...new Set(recent.filter((log) => (sourceKey(log) || 'unknown') === source.key).map((log) => log.endpoint))]
  }
  // Request count per API key → model strand.
  const edges = new Map<string, number>()
  for (const log of recent) {
    const id = `${sourceKey(log) || 'unknown'}\n${modelKey(log) || 'unknown'}`
    edges.set(id, (edges.get(id) ?? 0) + 1)
  }
  return { recent, sources, models: group(modelKey), edges }
}

function compactModels(models: RouteNode[]) {
  if (models.length <= MAX_MODEL_NODES) return models
  const visible = models.slice(0, MAX_MODEL_NODES - 1)
  const hidden = models.slice(MAX_MODEL_NODES - 1)
  visible.push({
    key: OTHER_MODELS,
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

function endpoints(node: RouteNode) {
  return (node.endpoints ?? []).map(endpointLabel).join(' + ')
}

function truncate(text: string, max: number) {
  return text.length > max ? text.slice(0, max - 1) + '…' : text
}

function tone(node: RouteNode) {
  if (!node.failures) return 'var(--success)'
  if (!node.successes) return 'var(--destructive)'
  return 'var(--warning)'
}

export function LiveRoutingMap({ logs: externalLogs }: { logs?: RequestLog[] } = {}) {
  const { t } = useI18n()
  const query = useQuery({
    queryKey: ['logs'],
    queryFn: () => get<{ logs: RequestLog[] }>('/logs'),
    refetchInterval: REFETCH_MS,
    enabled: !externalLogs,
  })
  // Key names are admin-only; the key holder's usage page passes its own logs and falls back to endpoint labels.
  const keysQ = useQuery({ queryKey: ['api-keys'], queryFn: () => get<{ apiKeys: { id: string; name?: string }[] }>('/api-keys'), enabled: !externalLogs })
  const keyNames = useMemo(() => new Map((keysQ.data?.apiKeys ?? []).map((k) => [k.id, k.name || k.id.slice(0, 8)])), [keysQ.data])
  const rawLogs = externalLogs ?? query.data?.logs ?? []
  const routes = useMemo(() => aggregate(rawLogs), [rawLogs])
  const displayedModels = useMemo(() => compactModels(routes.models), [routes.models])
  const sourceColor = (index: number) => KEY_COLORS[index % KEY_COLORS.length]
  const sourceName = (source: RouteNode) =>
    externalLogs ? endpoints(source) : (keyNames.get(source.key) ?? (source.key === 'unknown' ? t('overview.routing.noKey') : source.key.slice(0, 8)))
  const height = Math.max(360, Math.max(routes.sources.length, displayedModels.length) * 78 + 90)
  const totalFailures = routes.recent.reduce((sum, log) => sum + (log.status === 'error' ? 1 : 0), 0)
  const routerY = height / 2
  const modelY = new Map(displayedModels.map((m, i) => [m.key, laneY(i, displayedModels.length, height)]))

  return (
    <Card className="overflow-hidden">
      <CardHeader className="flex flex-row items-start justify-between gap-4 border-b">
        <div>
          <div className="mb-2 flex items-center gap-2">
            <span className="lamp size-1.5 text-success shadow-[0_0_8px_currentColor]" data-live="true" aria-hidden="true" />
            <span className="eyebrow text-[10px] tracking-[0.2em] text-success">{t('overview.routing.live')}</span>
          </div>
          <CardTitle>{t('overview.routing.title')}</CardTitle>
          <CardDescription>{t('overview.routing.hint')}</CardDescription>
        </div>
        {!externalLogs && (
          <div className="shrink-0 text-right font-mono text-[10px] leading-5 text-muted-foreground">
            <div>{t('overview.routing.refresh')}</div>
            {query.dataUpdatedAt > 0 && <div>{new Date(query.dataUpdatedAt).toLocaleTimeString()}</div>}
          </div>
        )}
      </CardHeader>
      <CardContent className="p-0">
        {!externalLogs && query.isPending ? (
          <div className="p-6"><LoadingBlock /></div>
        ) : !externalLogs && query.isError ? (
          <div className="p-6"><EmptyState title={t('overview.routing.loadFailed', errorMessage(query.error))} /></div>
        ) : routes.recent.length === 0 ? (
          <div className="p-6"><EmptyState title={t('overview.routing.empty')} /></div>
        ) : (
          <div className="relative overflow-x-auto bg-[radial-gradient(circle_at_50%_50%,color-mix(in_oklch,var(--signal)_9%,transparent),transparent_45%)]">
            <svg
              className="routing-map block min-w-[920px] text-foreground"
              style={{ height: `${height}px`, width: '100%' }}
              viewBox={`0 0 1000 ${height}`}
              role="img"
              aria-label={t('overview.routing.aria', routes.recent.length, routes.models.length)}
            >
              <defs>
                <pattern id="routing-grid" width="24" height="24" patternUnits="userSpaceOnUse">
                  <path d="M 24 0 L 0 0 0 24" fill="none" stroke="var(--border)" strokeWidth="0.7" opacity="0.55" />
                </pattern>
                              </defs>
              <style>{`
                .routing-flow { animation: flow 2.2s linear infinite; }
                .routing-map text { font-family: var(--font-mono); }
                @media (prefers-reduced-motion: reduce) { .routing-flow { animation: none; } }
              `}</style>
              <rect width="1000" height={height} fill="url(#routing-grid)" opacity="0.7" />

              <text x="42" y="30" fill="var(--muted-foreground)" fontSize="10" fontWeight="700" letterSpacing="2">{t('overview.routing.sources').toUpperCase()}</text>
              <text x="405" y="30" fill="var(--muted-foreground)" fontSize="10" fontWeight="700" letterSpacing="2">{t('overview.routing.gateway').toUpperCase()}</text>
              <text x="718" y="30" fill="var(--muted-foreground)" fontSize="10" fontWeight="700" letterSpacing="2">{t('overview.routing.destinations').toUpperCase()}</text>

              {routes.sources.map((source, index) => {
                const y = laneY(index, routes.sources.length, height)
                const color = sourceColor(index)
                const path = `M 250 ${y} C 318 ${y}, 334 ${routerY}, 402 ${routerY}`
                const name = sourceName(source)
                return (
                  <g key={source.key}>
                    <path d={path} fill="none" stroke={color} strokeWidth="6" opacity="0.08" />
                    <path className="routing-flow" d={path} fill="none" stroke={color} strokeWidth="1.5" strokeDasharray="5 9" opacity="0.85" />
                    <rect x="42" y={y - 31} width="208" height="62" rx="9" fill="var(--card)" stroke={color} strokeOpacity="0.55" />
                    <circle cx="60" cy={y - 10} r="4" fill={color} />
                    <title>{name}</title>
                    <text x="72" y={y - 6} fill="currentColor" fontSize="13" fontWeight="650">{truncate(name, 20)}</text>
                    <text x="60" y={y + 16} fill={source.failures ? 'var(--destructive)' : 'var(--muted-foreground)'} fontSize="10">
                      {truncate(externalLogs ? t('overview.routing.requestCount', formatNumber(source.requests)) : `${t('overview.routing.requestCount', formatNumber(source.requests))} · ${endpoints(source)}`, 32)}
                    </text>
                  </g>
                )
              })}

              {/* One strand per API key → model, in the key's colour, fanned so shared models show every key. */}
              {routes.sources.flatMap((source, si) => {
                const fan = (si - (routes.sources.length - 1) / 2) * 5
                return [...routes.edges].flatMap(([id]) => {
                  const [src, model] = id.split('\n')
                  if (src !== source.key) return []
                  const y = (modelY.get(model) ?? modelY.get(OTHER_MODELS) ?? routerY) + fan
                  const path = `M 598 ${routerY + fan} C 662 ${routerY + fan}, 652 ${y}, 718 ${y}`
                  const color = sourceColor(si)
                  return [
                    <g key={id}>
                      <path d={path} fill="none" stroke={color} strokeWidth="6" opacity="0.08" />
                      <path className="routing-flow" d={path} fill="none" stroke={color} strokeWidth="1.5" strokeDasharray="5 9" opacity="0.85" />
                    </g>,
                  ]
                })
              })}

              {displayedModels.map((model, index) => {
                const y = laneY(index, displayedModels.length, height)
                const color = tone(model)
                return (
                  <g key={model.key}>
                    <title>{model.key}</title>
                    <rect x="718" y={y - 31} width="240" height="62" rx="9" fill="var(--card)" stroke={color} strokeOpacity="0.62" />
                    <circle cx="736" cy={y - 11} r="4" fill={color} />
                    <text x="748" y={y - 7} fill="currentColor" fontSize="11" fontWeight="650" >
                      {model.hiddenModels ? t('overview.routing.otherModels', model.hiddenModels) : truncate(model.key, 30)}
                    </text>
                    <text x="736" y={y + 16} fill="var(--muted-foreground)" fontSize="10" >
                      {t('overview.routing.modelStats', formatNumber(model.requests), formatNumber(model.successes), formatNumber(model.failures))}
                    </text>
                  </g>
                )
              })}

              <g>
                <circle cx="500" cy={routerY} r="92" fill="var(--signal)" opacity="0.06" />
                <rect x="402" y={routerY - 62} width="196" height="124" rx="16" fill="var(--card)" stroke="var(--signal)" strokeOpacity="0.55" strokeWidth="1.2" />
                <rect x="410" y={routerY - 54} width="180" height="108" rx="11" fill="none" stroke="var(--border)" strokeDasharray="2 4" />
                <image href={iconUrl} x="476" y={routerY - 54} width="48" height="48" />
                <text x="500" y={routerY + 12} textAnchor="middle" fill="currentColor" fontSize="15" fontWeight="700" letterSpacing="0.5" style={{ fontFamily: 'var(--font-sans)' }}>Hekato</text>
                <text x="500" y={routerY + 24} textAnchor="middle" fill="var(--signal)" fontSize="8.5" letterSpacing="3">ROUTER</text>
                <text x="500" y={routerY + 44} textAnchor="middle" fill="var(--muted-foreground)" fontSize="10">
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
