// Types + tiny helpers shared by the Accounts page components.
export type Account = {
  id: string
  email: string
  userId: string
  nickname: string
  authMethod: string
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

export const pct = (v: number | undefined) => Math.max(0, Math.min(100, Math.round(v ?? 0)))

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

