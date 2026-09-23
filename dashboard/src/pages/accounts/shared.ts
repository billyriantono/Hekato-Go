// Types + tiny helpers shared by the Accounts page components.
export type Account = {
  id: string
  email: string
  userId: string
  nickname: string
  authMethod: string
  probeModel?: string
  provider: string
  region: string
  enabled: boolean
  banStatus: string
  banReason: string
  banTime: number
  expiresAt: number
  hasToken: boolean
  machineId: string
  weight: number
  overageStatus: string
  overageCapability: string
  overageCap: number
  overageRate: number
  currentOverages: number
  overageCheckedAt: number
  proxyURL: string
  relayURL: string
  hasRelaySecret: boolean
  subscriptionType: string
  subscriptionTitle: string
  daysRemaining: number
  usageCurrent: number
  usageLimit: number
  usagePercent: number
  nextResetDate: string
  lastRefresh: number
  trialUsageCurrent: number
  trialUsageLimit: number
  trialUsagePercent: number
  trialStatus: string
  trialExpiresAt: number
  requestCount: number
  errorCount: number
  totalTokens: number
  totalCredits: number
  lastUsed: number
  warmupStatus: '' | 'ok' | 'error'
  warmupError: string
  lastWarmup: number
}

export type WarmupResult = {
  accountId: string
  email: string
  status: 'ok' | 'error'
  error?: string
  recovered?: boolean
  probed?: boolean
  latencyMs?: number
}

export function warmupCounts(results: WarmupResult[] | undefined) {
  const r = results ?? []
  return [r.filter((x) => x.status === 'ok').length, r.filter((x) => x.status === 'error').length, r.filter((x) => x.recovered).length] as const
}

/** Relative time in the past, e.g. "5m ago". Empty when unixSeconds is 0. */
export function ago(unixSeconds: number, lang = 'en'): string {
  if (!unixSeconds) return ''
  const s = Math.round(unixSeconds - Date.now() / 1000)
  const rtf = new Intl.RelativeTimeFormat(lang, { numeric: 'auto' })
  const a = Math.abs(s)
  if (a < 60) return rtf.format(s, 'second')
  if (a < 3600) return rtf.format(Math.round(s / 60), 'minute')
  if (a < 86400) return rtf.format(Math.round(s / 3600), 'hour')
  return rtf.format(Math.round(s / 86400), 'day')
}

export type Status = 'active' | 'disabled' | 'banned' | 'noToken' | 'expired' | 'overQuota'

export function accountStatus(a: Account): Status {
  if (!a.enabled) return 'disabled'
  if (a.banStatus && a.banStatus !== 'ACTIVE') return 'banned'
  if (!a.hasToken) return 'noToken'
  if (a.expiresAt && a.expiresAt * 1000 < Date.now()) return 'expired'
  if (a.usageLimit > 0 && a.usageCurrent >= a.usageLimit) return 'overQuota'
  return 'active'
}

export const statusTone: Record<Status, 'success' | 'warning' | 'danger' | 'muted'> = {
  active: 'success',
  disabled: 'muted',
  banned: 'danger',
  noToken: 'warning',
  expired: 'warning',
  overQuota: 'warning',
}

export const statusKey: Record<Status, string> = {
  active: 'accounts.normal',
  disabled: 'accounts.disabled',
  banned: 'accounts.banned',
  noToken: 'accounts.noToken',
  expired: 'accounts.expired',
  overQuota: 'accounts.overQuota',
}

const subKeys: Record<string, string> = {
  FREE: 'subscription.free',
  PRO: 'subscription.pro',
  PRO_PLUS: 'subscription.proPlus',
  POWER: 'subscription.power',
}
export function subscriptionLabel(a: Account, t: (k: string) => string): string {
  const key = subKeys[(a.subscriptionType || '').toUpperCase()]
  return key ? t(key) : a.subscriptionTitle || a.subscriptionType || '—'
}

/** "2h 15m" style countdown; empty when in the past. */
export function countdown(unixSeconds: number): string {
  const diff = unixSeconds * 1000 - Date.now()
  if (diff <= 0) return ''
  const m = Math.floor(diff / 60000)
  if (m < 60) return `${m}m`
  const h = Math.floor(m / 60)
  if (h < 48) return `${h}h ${m % 60}m`
  return `${Math.floor(h / 24)}d`
}

// pct converts a usage ratio to a 0-100 integer for Progress/labels. Every
// provider reports usagePercent as a 0-1 fraction (codebuddy 530/600 = 0.883,
// kiro/grok/codex likewise), so the value is scaled — a raw Math.round would
// render 88% as 1%. Values already expressed as percentages (>= 1) pass through.
export const pct = (v: number | undefined) => {
  const n = v ?? 0
  const scaled = n > 0 && n <= 1 ? n * 100 : n
  return Math.max(0, Math.min(100, Math.round(scaled)))
}

export const sleep = (ms: number) => new Promise((r) => setTimeout(r, ms))

export function downloadJson(filename: string, text: string) {
  const url = URL.createObjectURL(new Blob([text], { type: 'application/json' }))
  const a = Object.assign(document.createElement('a'), { href: url, download: filename })
  a.click()
  URL.revokeObjectURL(url)
}

export const isProxyUrl = (v: string) => /^(https?|socks5h?):\/\//.test(v)
export const isHttpUrl = (v: string) => /^https?:\/\//.test(v)
export const isMachineId = (v: string) =>
  /^[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}$/i.test(v) || /^[0-9a-f]{32}$/i.test(v)


/** Kiro (AWS) accounts have upstream overage billing; other providers do not. */
export function isKiroAccount(a: { authMethod?: string; provider?: string }): boolean {
  const key = `${a.authMethod ?? ''} ${a.provider ?? ''}`.toLowerCase()
  return !/codebuddy|grok|xai|codex|openai|cline/.test(key)
}
