import { Link, Outlet, useNavigate, useRouterState } from '@tanstack/react-router'
import { useQuery } from '@tanstack/react-query'
import { lazy, Suspense, useEffect, useLayoutEffect, useRef, useState, type MouseEvent } from 'react'
import { LuBookOpen, LuLanguages, LuLogOut, LuMenu, LuMoon, LuSearch, LuSun, LuX } from 'react-icons/lu'
import { Wordmark, Mark } from '@/components/brand'
import { Button } from '@/components/ui/button'
import { Tooltip, TooltipContent, TooltipTrigger } from '@/components/ui/tooltip'
import { get } from '@/lib/api'
import { useAuth } from '@/lib/auth'
import { useI18n } from '@/lib/i18n'
import { Tween } from '@/lib/motion'
import { useTheme } from '@/lib/theme'
import { cn } from '@/lib/utils'
import { formatNumber } from '@/components/common'
import { NAV, navIndex } from './nav'

const CommandPalette = lazy(() => import('./command-palette'))

type Status = { version?: string; accounts?: number; available?: number; totalRequests?: number; uptime?: number }

function NavLinks({ onNavigate }: { onNavigate?: () => void }) {
  const { t } = useI18n()
  const pathname = useRouterState({ select: (s) => s.location.pathname })
  const active = navIndex(pathname)
  const navRef = useRef<HTMLElement>(null)
  const [lamp, setLamp] = useState<{ y: number; h: number; ready: boolean } | null>(null)

  // One lamp element glides to whichever item is active.
  useLayoutEffect(() => {
    const el = navRef.current?.querySelector<HTMLElement>('[data-active="true"]')
    setLamp((prev) => (el ? { y: el.offsetTop, h: el.offsetHeight, ready: prev !== null } : null))
  }, [active])

  const sections = (['manage', 'system'] as const).map((section) => (
    <div key={section} className="space-y-0.5">
      <div className="eyebrow px-3 pb-2 text-[9.5px] tracking-[0.2em] text-muted-foreground/70">{t(`nav.section.${section}`)}</div>
      {NAV.filter((n) => n.section === section).map((item) => {
        const on = NAV[active] === item
        return (
          <Link
            key={item.to}
            to={item.to}
            onClick={onNavigate}
            data-active={on}
            className={cn(
              'group/nav relative z-10 flex h-9 items-center gap-3 rounded-lg px-3 text-[13px] transition-colors duration-200',
              on ? 'font-medium text-foreground' : 'text-muted-foreground hover:text-foreground',
            )}
          >
            <item.icon className={cn('size-4 transition-all duration-300 ease-(--ease-out-expo)', on ? 'text-signal' : 'group-hover/nav:translate-x-0.5')} />
            <span className="flex-1">{t(item.labelKey)}</span>
            <span className="hidden gap-0.5 opacity-0 transition-opacity duration-200 group-hover/nav:opacity-100 md:flex">
              <kbd className="key h-4.5 min-w-4.5 text-[9.5px]">g</kbd>
              <kbd className="key h-4.5 min-w-4.5 text-[9.5px]">{item.key}</kbd>
            </span>
          </Link>
        )
      })}
    </div>
  ))

  return (
    <nav ref={navRef} className="relative flex flex-col gap-7">
      {lamp && (
        <div
          aria-hidden="true"
          className={cn('pointer-events-none absolute inset-x-0 top-0 rounded-lg bg-sidebar-accent ring-1 ring-border', lamp.ready && 'transition-[transform,height] duration-500 ease-(--ease-spring)')}
          style={{ transform: `translateY(${lamp.y}px)`, height: lamp.h }}
        >
          <span className="absolute inset-y-2 -left-px w-[3px] rounded-full bg-signal shadow-[0_0_12px_2px_var(--signal-glow)]" />
        </div>
      )}
      {sections}
    </nav>
  )
}

function formatUptime(s = 0) {
  const d = Math.floor(s / 86400)
  const h = Math.floor((s % 86400) / 3600)
  const m = Math.floor((s % 3600) / 60)
  return d ? `${d}d ${h}h` : h ? `${h}h ${m}m` : `${m}m`
}

/** Live readout pinned to the sidebar foot: is traffic able to route right now? */
function SystemPanel() {
  const { t } = useI18n()
  const status = useQuery({ queryKey: ['status'], queryFn: () => get<Status>('/status'), refetchInterval: 15_000 })
  const version = useQuery({ queryKey: ['version'], queryFn: () => get<{ version?: string }>('/version'), staleTime: 600_000 })
  const s = status.data
  const total = s?.accounts ?? 0
  const live = s?.available ?? 0
  const state = !s ? 'connecting' : total > 0 && live === 0 ? 'down' : 'up'
  const tone = { connecting: 'text-muted-foreground', down: 'text-destructive', up: 'text-success' }[state]

  return (
    <div className="mx-3 rounded-xl border bg-card/60 p-3 shadow-[inset_0_1px_0_var(--hairline-hi)]">
      <div className="flex items-center justify-between">
        <span className="eyebrow text-[9.5px] tracking-[0.2em]">{t('shell.system')}</span>
        <span className={cn('flex items-center gap-1.5 font-mono text-[10.5px]', tone)}>
          <span className="lamp size-1.5 shadow-[0_0_8px_currentColor]" data-live={state !== 'connecting'} />
          {t(state === 'up' ? 'shell.operational' : state === 'down' ? 'shell.noRoute' : 'shell.connecting')}
        </span>
      </div>
      <div className="mt-3 space-y-2.5">
        <div>
          <div className="flex items-baseline justify-between text-[11px] text-muted-foreground">
            <span>{t('shell.accounts')}</span>
            <span className="font-mono text-foreground tabular-nums">
              <Tween value={live} />
              <span className="text-muted-foreground">/{total}</span>
            </span>
          </div>
          <div className="mt-1.5 flex h-1 gap-px overflow-hidden rounded-full bg-muted">
            <div className="rounded-full bg-success transition-[width] duration-1000 ease-(--ease-out-expo)" style={{ width: total ? `${(live / total) * 100}%` : '0%' }} />
          </div>
        </div>
        <div className="flex items-baseline justify-between text-[11px] text-muted-foreground">
          <span>{t('shell.requests')}</span>
          <span className="font-mono text-foreground tabular-nums"><Tween value={s?.totalRequests ?? 0} format={formatNumber} /></span>
        </div>
      </div>
      <div className="mt-3 flex items-center justify-between border-t pt-2.5 font-mono text-[10px] text-muted-foreground/80">
        <span>{version.data?.version ? `v${version.data.version.replace(/^v/, '')}` : '—'}</span>
        <span>{t('shell.uptime')} {formatUptime(s?.uptime)}</span>
      </div>
    </div>
  )
}

function Sidebar({ onNavigate }: { onNavigate?: () => void }) {
  return (
    <div className="flex h-full flex-col gap-8 pt-5 pb-4">
      <Link to="/" onClick={onNavigate} className="px-5">
        <Wordmark />
      </Link>
      <div className="flex-1 overflow-y-auto px-3">
        <NavLinks onNavigate={onNavigate} />
      </div>
      <SystemPanel />
    </div>
  )
}

function HeaderIcon({ label, onClick, children }: { label: string; onClick: (e: MouseEvent) => void; children: React.ReactNode }) {
  return (
    <Tooltip>
      <TooltipTrigger
        render={
          <Button variant="ghost" size="icon" onClick={onClick} aria-label={label}>
            {children}
          </Button>
        }
      />
      <TooltipContent side="bottom">{label}</TooltipContent>
    </Tooltip>
  )
}

const typing = (el: EventTarget | null) => el instanceof HTMLElement && (el.isContentEditable || /^(INPUT|TEXTAREA|SELECT)$/.test(el.tagName))

export function AppShell() {
  const { t, lang, setLang } = useI18n()
  const { theme, toggle } = useTheme()
  const { logout } = useAuth()
  const navigate = useNavigate()
  const pathname = useRouterState({ select: (s) => s.location.pathname })
  const [mobileOpen, setMobileOpen] = useState(false)
  const [paletteOpen, setPaletteOpen] = useState(false)
  const [paletteLoaded, setPaletteLoaded] = useState(false)
  const page = NAV[navIndex(pathname)]

  const openPalette = () => {
    setPaletteLoaded(true)
    setPaletteOpen(true)
  }

  // ⌘K / Ctrl+K opens the palette; `g` then a nav key jumps straight there.
  useEffect(() => {
    let pendingG = 0
    const onKey = (e: KeyboardEvent) => {
      if ((e.metaKey || e.ctrlKey) && e.key.toLowerCase() === 'k') {
        e.preventDefault()
        setPaletteLoaded(true)
        setPaletteOpen((o) => !o)
        return
      }
      if (e.metaKey || e.ctrlKey || e.altKey || typing(e.target)) return
      if (e.key === 'g') {
        pendingG = Date.now()
        return
      }
      if (Date.now() - pendingG < 1000) {
        const hit = NAV.find((n) => n.key === e.key)
        if (hit) navigate({ to: hit.to })
        pendingG = 0
      }
    }
    window.addEventListener('keydown', onKey)
    return () => window.removeEventListener('keydown', onKey)
  }, [navigate])

  useEffect(() => {
    document.title = page ? `${t(page.labelKey)} · Hekato` : 'Hekato'
  }, [page, t])

  return (
    <div className="flex min-h-svh bg-background text-foreground">
      {/* Desktop sidebar */}
      <aside className="sticky top-0 hidden h-svh w-64 shrink-0 border-r bg-sidebar text-sidebar-foreground md:block">
        <Sidebar />
      </aside>

      {/* Mobile drawer — stays mounted so it can slide both ways */}
      <div className={cn('fixed inset-0 z-40 md:hidden', !mobileOpen && 'pointer-events-none')} aria-hidden={!mobileOpen}>
        <div
          className={cn('absolute inset-0 bg-[oklch(0.12_0.01_245/0.45)] backdrop-blur-[2px] transition-opacity duration-500 ease-(--ease-out-expo)', mobileOpen ? 'opacity-100' : 'opacity-0')}
          onClick={() => setMobileOpen(false)}
        />
        <aside
          className={cn(
            'absolute inset-y-0 left-0 w-72 border-r bg-sidebar shadow-2xl transition-transform duration-500 ease-(--ease-out-expo)',
            mobileOpen ? 'translate-x-0' : '-translate-x-full',
          )}
        >
          <Button variant="ghost" size="icon-sm" className="absolute top-5 right-3" onClick={() => setMobileOpen(false)} aria-label="Close">
            <LuX />
          </Button>
          <Sidebar onNavigate={() => setMobileOpen(false)} />
        </aside>
      </div>

      <div className="flex min-w-0 flex-1 flex-col">
        <header className="sticky top-0 z-30 flex h-14 items-center gap-1.5 border-b bg-background/75 px-4 backdrop-blur-xl backdrop-saturate-150 md:px-6">
          <Button variant="ghost" size="icon" className="md:hidden" onClick={() => setMobileOpen(true)} aria-label={t('shell.menu')}>
            <LuMenu />
          </Button>
          <Mark className="size-5 md:hidden" />
          <div className="ml-1 flex min-w-0 items-center gap-2 font-mono text-[11.5px] text-muted-foreground">
            <span className="hidden md:inline">hekato</span>
            <span className="hidden text-muted-foreground/40 md:inline">/</span>
            <span key={page?.to} className="animate-[rise_0.5s_var(--ease-out-expo)_both] truncate text-foreground">
              {page ? t(page.labelKey).toLowerCase() : ''}
            </span>
          </div>
          <div className="flex-1" />
          <button
            type="button"
            onClick={openPalette}
            className="group/k mr-2 hidden h-8 w-56 items-center gap-2 rounded-lg border bg-card/60 px-2.5 text-[12.5px] text-muted-foreground shadow-[inset_0_1px_0_var(--hairline-hi)] transition-[border-color,color,width] duration-300 ease-(--ease-out-expo) hover:w-60 hover:border-foreground/15 hover:text-foreground lg:flex"
          >
            <LuSearch className="size-3.5" />
            <span className="flex-1 text-left">{t('palette.trigger')}</span>
            <kbd className="key">⌘K</kbd>
          </button>
          <Button variant="ghost" size="icon" className="lg:hidden" onClick={openPalette} aria-label={t('palette.title')}>
            <LuSearch />
          </Button>
          <HeaderIcon label={t('header.docs')} onClick={() => window.open('/docs', '_blank')}>
            <LuBookOpen />
          </HeaderIcon>
          <HeaderIcon label={`${t('header.language')}: ${lang === 'en' ? 'English' : '中文'}`} onClick={() => setLang(lang === 'en' ? 'zh' : 'en')}>
            <LuLanguages />
          </HeaderIcon>
          <HeaderIcon label={t('header.theme')} onClick={(e) => toggle(e)}>
            <span className="relative grid size-4 place-items-center [&>svg]:col-start-1 [&>svg]:row-start-1 [&>svg]:size-4 [&>svg]:transition-all [&>svg]:duration-500 [&>svg]:ease-(--ease-spring)">
              <LuSun className={cn(theme === 'dark' ? 'rotate-0 opacity-100' : '-rotate-90 scale-50 opacity-0')} />
              <LuMoon className={cn(theme === 'dark' ? 'rotate-90 scale-50 opacity-0' : 'rotate-0 opacity-100')} />
            </span>
          </HeaderIcon>
          <HeaderIcon label={t('common.logout')} onClick={logout}>
            <LuLogOut />
          </HeaderIcon>
        </header>
        <main className="atmosphere flex-1 px-4 py-6 md:px-8 md:py-8">
          <div key={pathname} className="route mx-auto w-full max-w-7xl">
            <Outlet />
          </div>
        </main>
      </div>

      {paletteLoaded && (
        <Suspense fallback={null}>
          <CommandPalette open={paletteOpen} onOpenChange={setPaletteOpen} />
        </Suspense>
      )}
    </div>
  )
}
