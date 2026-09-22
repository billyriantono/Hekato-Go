import { useState } from 'react'
import { LuEye, LuEyeOff, LuLoader } from 'react-icons/lu'
import { Button } from '@/components/ui/button'
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from '@/components/ui/card'
import { Checkbox } from '@/components/ui/checkbox'
import { Input } from '@/components/ui/input'
import { Label } from '@/components/ui/label'
import { useAuth } from '@/lib/auth'
import { useI18n } from '@/lib/i18n'
import { ApiError } from '@/lib/api'

export function AuthFrame({ title, subtitle, children }: { title: string; subtitle: string; children: React.ReactNode }) {
  return (
    <div className="flex min-h-svh items-center justify-center bg-muted/40 p-4">
      <Card className="w-full max-w-sm shadow-lg">
        <CardHeader className="space-y-2 text-center">
          <div className="mx-auto flex size-11 items-center justify-center rounded-xl bg-primary text-primary-foreground">
            <span className="text-lg font-bold">H</span>
          </div>
          <CardTitle className="text-xl">{title}</CardTitle>
          <CardDescription>{subtitle}</CardDescription>
        </CardHeader>
        <CardContent>{children}</CardContent>
      </Card>
    </div>
  )
}

export function LoginPage() {
  const { t } = useI18n()
  const { login } = useAuth()
  const [password, setPw] = useState('')
  const [remember, setRemember] = useState(true)
  const [show, setShow] = useState(false)
  const [busy, setBusy] = useState(false)
  const [error, setError] = useState('')

  const submit = async (e: React.SyntheticEvent) => {
    e.preventDefault()
    setBusy(true)
    setError('')
    try {
      await login(password, remember)
    } catch (err) {
      setError(err instanceof ApiError && err.status === 401 ? t('login.error') : t('login.connectError'))
    } finally {
      setBusy(false)
    }
  }

  return (
    <AuthFrame title={t('login.title')} subtitle={t('login.subtitle')}>
      <form onSubmit={submit} className="space-y-4">
        <div className="space-y-2">
          <Label htmlFor="pw">{t('login.password')}</Label>
          <div className="relative">
            <Input
              id="pw"
              type={show ? 'text' : 'password'}
              value={password}
              onChange={(e) => setPw(e.target.value)}
              placeholder={t('login.passwordPlaceholder')}
              autoFocus
              autoComplete="current-password"
              className="pr-9"
            />
            <button
              type="button"
              onClick={() => setShow((s) => !s)}
              className="absolute inset-y-0 right-2 flex items-center text-muted-foreground hover:text-foreground"
              aria-label={show ? t('login.hidePassword') : t('login.showPassword')}
            >
              {show ? <LuEyeOff className="size-4" /> : <LuEye className="size-4" />}
            </button>
          </div>
        </div>
        <label className="flex items-center gap-2 text-sm">
          <Checkbox checked={remember} onCheckedChange={(v) => setRemember(v === true)} />
          {t('login.remember')}
        </label>
        {error && <p className="text-sm text-destructive">{error}</p>}
        <Button type="submit" className="w-full" disabled={busy || !password}>
          {busy && <LuLoader className="size-4 animate-spin" />}
          {t('login.submit')}
        </Button>
      </form>
    </AuthFrame>
  )
}

export function SetupPage() {
  const { t } = useI18n()
  const { completeSetup } = useAuth()
  const [pw, setPw] = useState('')
  const [confirm, setConfirm] = useState('')
  const [busy, setBusy] = useState(false)
  const [error, setError] = useState('')

  const submit = async (e: React.SyntheticEvent) => {
    e.preventDefault()
    if (pw.trim().length < 8) return setError(t('setup.tooShort'))
    if (pw !== confirm) return setError(t('setup.mismatch'))
    setBusy(true)
    setError('')
    try {
      await completeSetup(pw)
    } catch (err) {
      setError(err instanceof Error ? err.message : t('common.unknownError'))
    } finally {
      setBusy(false)
    }
  }

  return (
    <AuthFrame title={t('setup.title')} subtitle={t('setup.subtitle')}>
      <form onSubmit={submit} className="space-y-4">
        <div className="space-y-2">
          <Label htmlFor="npw">{t('setup.password')}</Label>
          <Input id="npw" type="password" value={pw} onChange={(e) => setPw(e.target.value)} placeholder={t('setup.passwordPlaceholder')} autoFocus />
        </div>
        <div className="space-y-2">
          <Label htmlFor="cpw">{t('setup.confirm')}</Label>
          <Input id="cpw" type="password" value={confirm} onChange={(e) => setConfirm(e.target.value)} placeholder={t('setup.confirmPlaceholder')} />
        </div>
        {error && <p className="text-sm text-destructive">{error}</p>}
        <Button type="submit" className="w-full" disabled={busy}>
          {busy && <LuLoader className="size-4 animate-spin" />}
          {t('setup.submit')}
        </Button>
      </form>
    </AuthFrame>
  )
}
