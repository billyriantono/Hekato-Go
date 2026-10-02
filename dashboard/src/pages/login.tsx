import { useEffect, useMemo, useState, type ReactNode } from 'react'
import { LuArrowRight, LuEye, LuEyeOff, LuLanguages, LuLoader, LuLock, LuMoon, LuSun } from 'react-icons/lu'
import { iconUrl, Mark, Wordmark } from '@/components/brand'
import { Button } from '@/components/ui/button'
import { Checkbox } from '@/components/ui/checkbox'
import { Input } from '@/components/ui/input'
import { Label } from '@/components/ui/label'
import { useAuth } from '@/lib/auth'
import { useI18n } from '@/lib/i18n'
import { useTheme } from '@/lib/theme'
import { ApiError } from '@/lib/api'
import { cn } from '@/lib/utils'

const NODE = { x: 560, y: 300 }
const LANES = 9

/**
 * The crossroads field: requests stream in from every lane, merge at the lit
 * gateway node, and leave along three roads. Pure SVG + SMIL — no JS per frame.
 */
function CrossroadsField() {
  const reduced = useMemo(() => window.matchMedia('(prefers-reduced-motion: reduce)').matches, [])
  const inbound = Array.from({ length: LANES }, (_, i) => {
    const y = 40 + (i * 520) / (LANES - 1)
    return `M -20 ${y} C 250 ${y}, 330 ${NODE.y}, ${NODE.x - 14} ${NODE.y}`
  })
  const outbound = [170, 300, 430].map((y) => `M ${NODE.x + 14} ${NODE.y} C 670 ${NODE.y}, 700 ${y}, 840 ${y}`)

  return (
    <svg viewBox="0 0 800 600" preserveAspectRatio="xMidYMid slice" className="absolute inset-0 size-full" aria-hidden="true">
      <defs>
        <radialGradient id="cf-glow">
          <stop offset="0%" stopColor="var(--signal)" stopOpacity="0.55" />
          <stop offset="100%" stopColor="var(--signal)" stopOpacity="0" />
        </radialGradient>
        <linearGradient id="cf-fade" x1="0" x2="1">
          <stop offset="0%" stopColor="currentColor" stopOpacity="0" />
          <stop offset="35%" stopColor="currentColor" stopOpacity="0.16" />
          <stop offset="100%" stopColor="currentColor" stopOpacity="0.22" />
        </linearGradient>
      </defs>

      {inbound.map((d, i) => (
        <path key={d} d={d} fill="none" stroke="url(#cf-fade)" strokeWidth="1" pathLength={1} strokeDasharray="1" style={{ animation: `draw 1.6s ${0.15 + i * 0.06}s var(--ease-out-expo) both`, ['--len' as string]: 1 }} />
      ))}
      {outbound.map((d, i) => (
        <path key={d} d={d} fill="none" stroke={['var(--signal)', 'var(--pop)', 'var(--success)'][i]} strokeOpacity="0.55" strokeWidth="1.4" pathLength={1} strokeDasharray="1" style={{ animation: `draw 1.2s ${1 + i * 0.1}s var(--ease-out-expo) both`, ['--len' as string]: 1 }} />
      ))}

      {!reduced && (
        <g>
          {inbound.flatMap((d, i) =>
            [0, 1].map((k) => {
              const dur = 3.2 + ((i * 7 + k * 3) % 5) * 0.45
              // Every third request glows azure, every fifth tangerine; the rest are quiet.
              const lit = (i + k) % 3 === 0
              const pop = (i * 2 + k) % 5 === 0
              return (
                <circle key={`${i}-${k}`} r={lit || pop ? 2.4 : 1.6} fill={pop ? 'var(--pop)' : lit ? 'var(--signal)' : 'currentColor'} opacity="0">
                  <animateMotion dur={`${dur}s`} begin={`${1.2 + ((i * 0.37 + k * 1.6) % dur)}s`} repeatCount="indefinite" path={d} keyPoints="0;1" keyTimes="0;1" calcMode="spline" keySplines="0.45 0 0.2 1" />
                  <animate attributeName="opacity" values="0;0.9;0.9;0" keyTimes="0;0.1;0.85;1" dur={`${dur}s`} begin={`${1.2 + ((i * 0.37 + k * 1.6) % dur)}s`} repeatCount="indefinite" />
                </circle>
              )
            }),
          )}
          {outbound.map((d, i) => (
            <circle key={d} r="2.6" fill={['var(--signal)', 'var(--pop)', 'var(--success)'][i]} opacity="0">
              <animateMotion dur="1.8s" begin={`${1.6 + i * 0.6}s`} repeatCount="indefinite" path={d} />
              <animate attributeName="opacity" values="0;1;1;0" keyTimes="0;0.1;0.8;1" dur="1.8s" begin={`${1.6 + i * 0.6}s`} repeatCount="indefinite" />
            </circle>
          ))}
        </g>
      )}

      {/* The gateway node */}
      <circle cx={NODE.x} cy={NODE.y} r="120" fill="url(#cf-glow)" opacity="0.5" className="animate-[fade_2s_0.8s_both]" />
      <g style={{ transformOrigin: `${NODE.x}px ${NODE.y}px` }} className="animate-[spin_40s_linear_infinite]">
        <circle cx={NODE.x} cy={NODE.y} r="46" fill="none" stroke="currentColor" strokeOpacity="0.18" strokeDasharray="2 6" />
      </g>
      <circle cx={NODE.x} cy={NODE.y} r="30" fill="var(--background)" stroke="currentColor" strokeOpacity="0.25" />
      <image href={iconUrl} x={NODE.x - 21} y={NODE.y - 21} width="42" height="42" className="animate-[rise_0.8s_1s_var(--ease-spring)_both]" />
    </svg>
  )
}

function Corner({ children }: { children: ReactNode }) {
  return <div className="flex items-center gap-1">{children}</div>
}

/** Split-screen auth frame: the living crossroads on the left, the form on the right. */
function AuthFrame({ eyebrow, title, subtitle, children }: { eyebrow: string; title: string; subtitle: string; children: ReactNode }) {
  const { t, lang, setLang } = useI18n()
  const { theme, toggle } = useTheme()
  return (
    <div className="grid min-h-svh bg-background lg:grid-cols-[1.2fr_1fr]">
      {/* Brand canvas */}
      <section className="relative hidden overflow-hidden border-r bg-surface text-foreground lg:block">
        <div className="atmosphere absolute inset-0 opacity-70" />
        <CrossroadsField />
        <div className="absolute inset-x-0 top-0 h-40 bg-gradient-to-b from-surface to-transparent" />
        <div className="absolute inset-x-0 bottom-0 h-72 bg-gradient-to-t from-surface via-surface/85 to-transparent" />
        <div className="relative flex h-full flex-col justify-between p-10 xl:p-14">
          <Wordmark className="animate-[rise_0.8s_var(--ease-out-expo)_both]" />
          <div className="max-w-xl">
            <h2 className="display text-[clamp(3rem,5vw,4.75rem)] text-balance [animation:rise_1s_0.3s_var(--ease-out-expo)_both]">{t('login.tagline')}</h2>
            <p className="mt-5 max-w-md text-[15px] leading-relaxed text-muted-foreground [animation:rise_1s_0.45s_var(--ease-out-expo)_both]">{t('login.taglineSub')}</p>
          </div>
        </div>
      </section>

      {/* Form */}
      <section className="relative flex flex-col px-6 py-6 sm:px-10">
        <div className="flex items-center justify-between">
          <Mark className="size-7 lg:invisible" />
          <Corner>
            <Button variant="ghost" size="icon-sm" onClick={() => setLang(lang === 'en' ? 'zh' : 'en')} aria-label={t('header.language')}>
              <LuLanguages />
            </Button>
            <Button variant="ghost" size="icon-sm" onClick={(e) => toggle(e)} aria-label={t('header.theme')}>
              {theme === 'dark' ? <LuSun /> : <LuMoon />}
            </Button>
          </Corner>
        </div>
        <div className="flex flex-1 items-center justify-center py-10">
          <div className="stagger w-full max-w-[22rem]">
            <div className="eyebrow mb-4 flex items-center gap-2">
              <span className="text-signal">§</span>
              {eyebrow}
            </div>
            <h1 className="display text-[3rem]">{title}</h1>
            <p className="mt-3 mb-9 text-[13.5px] leading-relaxed text-muted-foreground">{subtitle}</p>
            <div>{children}</div>
          </div>
        </div>
        <div className="flex items-center justify-center gap-2 font-mono text-[10.5px] text-muted-foreground/70">
          <LuLock className="size-3" /> {t('login.secure')}
        </div>
      </section>
    </div>
  )
}

/** An error line that shakes once each time a new error arrives. */
function FormError({ message, nonce }: { message: string; nonce: number }) {
  if (!message) return null
  return (
    <p key={nonce} role="alert" className="flex items-center gap-2 rounded-lg border border-destructive/25 bg-destructive/8 px-3 py-2 text-[12.5px] text-destructive [animation:shake_0.45s_var(--ease-out-expo)]">
      <span className="lamp size-1.5" data-live="true" />
      {message}
      <style>{'@keyframes shake{20%{transform:translateX(-6px)}40%{transform:translateX(5px)}60%{transform:translateX(-3px)}80%{transform:translateX(2px)}}'}</style>
    </p>
  )
}

function SubmitButton({ busy, disabled, children }: { busy: boolean; disabled: boolean; children: ReactNode }) {
  return (
    <Button type="submit" variant="signal" size="lg" className="group/submit h-11 w-full text-[14px]" disabled={busy || disabled}>
      {busy ? <LuLoader className="animate-spin" /> : null}
      {children}
      {!busy && <LuArrowRight className="transition-transform duration-300 ease-(--ease-out-expo) group-hover/submit:translate-x-1" />}
    </Button>
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
  const [nonce, setNonce] = useState(0)

  useEffect(() => {
    document.title = `${t('login.title')} · Hekato`
  }, [t])

  const submit = async (e: React.SyntheticEvent) => {
    e.preventDefault()
    setBusy(true)
    setError('')
    try {
      await login(password, remember)
    } catch (err) {
      setError(err instanceof ApiError && err.status === 401 ? t('login.error') : t('login.connectError'))
      setNonce((n) => n + 1)
    } finally {
      setBusy(false)
    }
  }

  return (
    <AuthFrame eyebrow="Admin console" title={t('login.title')} subtitle={t('login.subtitle')}>
      <form onSubmit={submit} className="space-y-5">
        <div className="space-y-2">
          <Label htmlFor="pw" className="eyebrow text-[10.5px]">{t('login.password')}</Label>
          <div className="relative">
            <Input
              id="pw"
              type={show ? 'text' : 'password'}
              value={password}
              onChange={(e) => setPw(e.target.value)}
              placeholder={t('login.passwordPlaceholder')}
              autoFocus
              autoComplete="current-password"
              aria-invalid={!!error || undefined}
              className={cn('h-11 pr-10 font-mono text-[14px] md:text-[14px]', !show && password && 'tracking-[0.2em]')}
            />
            <button
              type="button"
              onClick={() => setShow((s) => !s)}
              className="absolute inset-y-0 right-1.5 my-auto grid size-8 place-items-center rounded-md text-muted-foreground transition-colors hover:bg-accent hover:text-foreground"
              aria-label={show ? t('login.hidePassword') : t('login.showPassword')}
            >
              {show ? <LuEyeOff className="size-4" /> : <LuEye className="size-4" />}
            </button>
          </div>
        </div>
        <label className="flex cursor-pointer items-center gap-2.5 text-[13px] text-muted-foreground select-none hover:text-foreground">
          <Checkbox checked={remember} onCheckedChange={(v) => setRemember(v === true)} />
          {t('login.remember')}
        </label>
        <FormError message={error} nonce={nonce} />
        <SubmitButton busy={busy} disabled={!password}>{t('login.submit')}</SubmitButton>
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
  const [nonce, setNonce] = useState(0)
  const strength = Math.min(4, Math.floor(pw.length / 4) + (/[^a-zA-Z0-9]/.test(pw) ? 1 : 0))

  const fail = (msg: string) => {
    setError(msg)
    setNonce((n) => n + 1)
  }
  const submit = async (e: React.SyntheticEvent) => {
    e.preventDefault()
    if (pw.trim().length < 8) return fail(t('setup.tooShort'))
    if (pw !== confirm) return fail(t('setup.mismatch'))
    setBusy(true)
    setError('')
    try {
      await completeSetup(pw)
    } catch (err) {
      fail(err instanceof Error ? err.message : t('common.unknownError'))
    } finally {
      setBusy(false)
    }
  }

  return (
    <AuthFrame eyebrow="First run" title={t('setup.title')} subtitle={t('setup.subtitle')}>
      <form onSubmit={submit} className="space-y-5">
        <div className="space-y-2">
          <Label htmlFor="npw" className="eyebrow text-[10.5px]">{t('setup.password')}</Label>
          <Input id="npw" type="password" value={pw} onChange={(e) => setPw(e.target.value)} placeholder={t('setup.passwordPlaceholder')} autoFocus className="h-11 font-mono" />
          {/* Four signal segments light up as the password gets stronger. */}
          <div className="flex gap-1 pt-1" aria-hidden="true">
            {[0, 1, 2, 3].map((i) => (
              <div
                key={i}
                className={cn(
                  'h-1 flex-1 rounded-full transition-all duration-500 ease-(--ease-out-expo)',
                  i < strength ? (strength >= 3 ? 'bg-success' : 'bg-signal') : 'bg-muted',
                )}
                style={{ transitionDelay: `${i * 40}ms` }}
              />
            ))}
          </div>
        </div>
        <div className="space-y-2">
          <Label htmlFor="cpw" className="eyebrow text-[10.5px]">{t('setup.confirm')}</Label>
          <Input id="cpw" type="password" value={confirm} onChange={(e) => setConfirm(e.target.value)} placeholder={t('setup.confirmPlaceholder')} className="h-11 font-mono" />
        </div>
        <FormError message={error} nonce={nonce} />
        <SubmitButton busy={busy} disabled={false}>{t('setup.submit')}</SubmitButton>
      </form>
    </AuthFrame>
  )
}
