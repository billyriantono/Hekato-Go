import { createContext, useCallback, useContext, useEffect, useMemo, useState, type ReactNode } from 'react'
import { clearToken, get, getToken, login as apiLogin, logout as apiLogout, post, setToken, setUnauthorizedHandler } from '@/lib/api'

type Status = 'loading' | 'setup' | 'anonymous' | 'authed'

type Auth = {
  status: Status
  login: (password: string, remember: boolean) => Promise<void>
  completeSetup: (password: string) => Promise<void>
  logout: () => void
}

const Ctx = createContext<Auth | null>(null)

export function AuthProvider({ children }: { children: ReactNode }) {
  const [status, setStatus] = useState<Status>('loading')

  useEffect(() => {
    setUnauthorizedHandler((setupRequired) => setStatus(setupRequired ? 'setup' : 'anonymous'))
    ;(async () => {
      try {
        const s = await get<{ configured: boolean }>('/setup/status')
        if (!s.configured) return setStatus('setup')
        if (!getToken()) return setStatus('anonymous')
        await get('/version') // validates the stored session token
        setStatus('authed')
      } catch {
        setStatus((cur) => (cur === 'loading' ? 'anonymous' : cur))
      }
    })()
    return () => setUnauthorizedHandler(null)
  }, [])

  const login = useCallback(async (password: string, remember: boolean) => {
    // The password goes to the server exactly once, in the login request.
    // What the browser keeps is the session token it returns.
    const token = await apiLogin(password, remember)
    setToken(token, remember)
    try {
      await get('/version')
      setStatus('authed')
    } catch (e) {
      clearToken()
      throw e
    }
  }, [])

  const completeSetup = useCallback(async (password: string) => {
    const res = await post<{ success?: boolean; token?: string }>('/setup', { password })
    const token = res?.token || ''
    if (!token) throw new Error('Setup did not return a session token')
    setToken(token, true)
    setStatus('authed')
  }, [])

  const logout = useCallback(() => {
    // Revoke server-side first, then drop the local copy. The token is
    // cleared even if the revoke call fails.
    void apiLogout().finally(() => {
      clearToken()
      setStatus('anonymous')
    })
  }, [])

  const value = useMemo(() => ({ status, login, completeSetup, logout }), [status, login, completeSetup, logout])
  return <Ctx.Provider value={value}>{children}</Ctx.Provider>
}

export function useAuth() {
  const v = useContext(Ctx)
  if (!v) throw new Error('useAuth outside AuthProvider')
  return v
}
