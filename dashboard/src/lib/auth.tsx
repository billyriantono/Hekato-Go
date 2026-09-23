import { createContext, useCallback, useContext, useEffect, useMemo, useState, type ReactNode } from 'react'
import { clearPassword, get, getPassword, post, setPassword, setUnauthorizedHandler } from '@/lib/api'

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
        if (!getPassword()) return setStatus('anonymous')
        await get('/version') // validates the stored password
        setStatus('authed')
      } catch {
        setStatus((cur) => (cur === 'loading' ? 'anonymous' : cur))
      }
    })()
    return () => setUnauthorizedHandler(null)
  }, [])

  const login = useCallback(async (password: string, remember: boolean) => {
    setPassword(password, remember)
    try {
      await get('/version')
      setStatus('authed')
    } catch (e) {
      clearPassword()
      throw e
    }
  }, [])

  const completeSetup = useCallback(async (password: string) => {
    await post('/setup', { password })
    setPassword(password, true)
    setStatus('authed')
  }, [])

  const logout = useCallback(() => {
    clearPassword()
    setStatus('anonymous')
  }, [])

  const value = useMemo(() => ({ status, login, completeSetup, logout }), [status, login, completeSetup, logout])
  return <Ctx.Provider value={value}>{children}</Ctx.Provider>
}

export function useAuth() {
  const v = useContext(Ctx)
  if (!v) throw new Error('useAuth outside AuthProvider')
  return v
}
