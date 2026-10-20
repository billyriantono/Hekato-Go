// Thin fetch wrapper for the Go admin API (/admin/api/*).
// Auth: X-Admin-Session header, holding an opaque server-issued session
// token. The admin password is exchanged for that token once, at login, and
// is never written to browser storage.

export class ApiError extends Error {
  status: number
  setupRequired: boolean
  constructor(status: number, message: string, setupRequired = false) {
    super(message)
    this.status = status
    this.setupRequired = setupRequired
  }
}

const TOKEN_KEY = 'admin_session_token'
const REMEMBER_KEY = 'kiro_remember'
// LEGACY_PASSWORD_KEY is where the old console kept the raw admin password.
// Upgrading servers leave it behind, so it is purged on first load.
const LEGACY_PASSWORD_KEY = 'admin_password'

// purgeLegacyPassword removes the raw admin password that older console
// builds stored in browser storage. Without this, upgrading the server would
// leave the password sitting in the user's browser forever.
function purgeLegacyPassword() {
  try {
    sessionStorage.removeItem(LEGACY_PASSWORD_KEY)
    localStorage.removeItem(LEGACY_PASSWORD_KEY)
  } catch {
    /* storage may be unavailable (private mode); nothing to clean up then */
  }
}
purgeLegacyPassword()

export function getToken(): string {
  return sessionStorage.getItem(TOKEN_KEY) || localStorage.getItem(TOKEN_KEY) || ''
}

export function setToken(token: string, remember: boolean) {
  sessionStorage.setItem(TOKEN_KEY, token)
  if (remember) {
    localStorage.setItem(TOKEN_KEY, token)
    localStorage.setItem(REMEMBER_KEY, '1')
  } else {
    localStorage.removeItem(TOKEN_KEY)
    localStorage.removeItem(REMEMBER_KEY)
  }
}

export function clearToken() {
  sessionStorage.removeItem(TOKEN_KEY)
  localStorage.removeItem(TOKEN_KEY)
  localStorage.removeItem(REMEMBER_KEY)
}

let onUnauthorized: ((setupRequired: boolean) => void) | null = null
export function setUnauthorizedHandler(fn: typeof onUnauthorized) {
  onUnauthorized = fn
}

export async function api<T = unknown>(path: string, init: RequestInit = {}): Promise<T> {
  const headers = new Headers(init.headers)
  const token = getToken()
  if (token) headers.set('X-Admin-Session', token)
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

// login exchanges the admin password for a session token. The password is
// sent once and never stored client-side.
export async function login(password: string, remember: boolean): Promise<string> {
  const res = await fetch('/admin/api/login', {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify({ password, remember }),
  })
  const text = await res.text()
  let data: any = null
  try {
    data = text ? JSON.parse(text) : null
  } catch {
    data = null
  }
  if (!res.ok) {
    const obj = (data && typeof data === 'object' ? data : {}) as Record<string, unknown>
    const setupRequired = obj.setupRequired === 'true' || obj.setupRequired === true
    if (res.status === 401) onUnauthorized?.(setupRequired)
    throw new ApiError(res.status, String(obj.error || res.statusText || 'Request failed'), setupRequired)
  }
  const token = String(data?.token || '')
  if (!token) throw new ApiError(res.status, 'Login did not return a session token')
  return token
}

// logout revokes the session server-side. Failures are ignored: the local
// token is dropped either way.
export async function logout(): Promise<void> {
  try {
    await api('/logout', { method: 'POST' })
  } catch {
    /* the session may already be gone; clearing locally is what matters */
  }
}

export const get = <T = unknown>(path: string) => api<T>(path)
export const post = <T = unknown>(path: string, body?: unknown) =>
  api<T>(path, { method: 'POST', body: body === undefined ? undefined : JSON.stringify(body) })
export const put = <T = unknown>(path: string, body?: unknown) =>
  api<T>(path, { method: 'PUT', body: body === undefined ? undefined : JSON.stringify(body) })
export const del = <T = unknown>(path: string) => api<T>(path, { method: 'DELETE' })
