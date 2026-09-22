// Thin fetch wrapper for the Go admin API (/admin/api/*).
// Auth: X-Admin-Password header, stored in session/localStorage by lib/auth.

export class ApiError extends Error {
  status: number
  setupRequired: boolean
  constructor(status: number, message: string, setupRequired = false) {
    super(message)
    this.status = status
    this.setupRequired = setupRequired
  }
}

const PASSWORD_KEY = 'admin_password'
const REMEMBER_KEY = 'kiro_remember'

export function getPassword(): string {
  return sessionStorage.getItem(PASSWORD_KEY) || localStorage.getItem(PASSWORD_KEY) || ''
}

export function setPassword(pw: string, remember: boolean) {
  sessionStorage.setItem(PASSWORD_KEY, pw)
  if (remember) {
    localStorage.setItem(PASSWORD_KEY, pw)
    localStorage.setItem(REMEMBER_KEY, '1')
  } else {
    localStorage.removeItem(PASSWORD_KEY)
    localStorage.removeItem(REMEMBER_KEY)
  }
}

export function clearPassword() {
  sessionStorage.removeItem(PASSWORD_KEY)
  localStorage.removeItem(PASSWORD_KEY)
  localStorage.removeItem(REMEMBER_KEY)
}

let onUnauthorized: ((setupRequired: boolean) => void) | null = null
export function setUnauthorizedHandler(fn: typeof onUnauthorized) {
  onUnauthorized = fn
}

export async function api<T = unknown>(path: string, init: RequestInit = {}): Promise<T> {
  const headers = new Headers(init.headers)
  headers.set('X-Admin-Password', getPassword())
  if (init.body && !headers.has('Content-Type')) headers.set('Content-Type', 'application/json')
  const res = await fetch('/admin/api' + path, { ...init, headers })
  const text = await res.text()
  let data: unknown = null
  try {
    data = text ? JSON.parse(text) : null
  } catch {
    data = text
  }
  if (!res.ok) {
    const obj = (data && typeof data === 'object' ? data : {}) as Record<string, unknown>
    const setupRequired = obj.setupRequired === 'true' || obj.setupRequired === true
    if (res.status === 401) onUnauthorized?.(setupRequired)
    throw new ApiError(res.status, String(obj.error || res.statusText || 'Request failed'), setupRequired)
  }
  return data as T
}

export const get = <T = unknown>(path: string) => api<T>(path)
export const post = <T = unknown>(path: string, body?: unknown) =>
  api<T>(path, { method: 'POST', body: body === undefined ? undefined : JSON.stringify(body) })
export const put = <T = unknown>(path: string, body?: unknown) =>
  api<T>(path, { method: 'PUT', body: body === undefined ? undefined : JSON.stringify(body) })
export const del = <T = unknown>(path: string) => api<T>(path, { method: 'DELETE' })
