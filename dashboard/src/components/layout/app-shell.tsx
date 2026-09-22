import { Link, Outlet, useRouterState } from '@tanstack/react-router'
import { useQuery } from '@tanstack/react-query'
import { useState, type ReactNode } from 'react'
import {
  LuLayoutDashboard,
  LuUsers,
  LuKeyRound,
  LuSettings,
  LuScrollText,
  LuMoon,
  LuSun,
  LuLogOut,
  LuMenu,
  LuLanguages,
  LuX,
  LuBookOpen,
} from 'react-icons/lu'
import { Button } from '@/components/ui/button'
import { Tooltip, TooltipContent, TooltipTrigger } from '@/components/ui/tooltip'
import { useAuth } from '@/lib/auth'
import { useI18n } from '@/lib/i18n'
import { useTheme } from '@/lib/theme'
import { get } from '@/lib/api'
import { cn } from '@/lib/utils'

type NavItem = { to: string; labelKey: string; icon: ReactNode }

const manage: NavItem[] = [
  { to: '/', labelKey: 'nav.overview', icon: <LuLayoutDashboard className="size-4" /> },
  { to: '/accounts', labelKey: 'nav.accounts', icon: <LuUsers className="size-4" /> },
  { to: '/api-keys', labelKey: 'nav.apiKeys', icon: <LuKeyRound className="size-4" /> },
]
const system: NavItem[] = [
  { to: '/logs', labelKey: 'nav.logs', icon: <LuScrollText className="size-4" /> },
  { to: '/settings', labelKey: 'nav.settings', icon: <LuSettings className="size-4" /> },
]

function Logo() {
  return (
    <div className="flex items-center gap-2.5 px-2">
      <div className="flex size-8 items-center justify-center rounded-lg bg-primary text-primary-foreground shadow-sm">
        <span className="text-sm font-bold tracking-tight">H</span>
      </div>
      <div className="leading-tight">
        <div className="text-sm font-semibold">Hekato</div>
        <div className="text-[10px] uppercase tracking-widest text-muted-foreground">AI Gateway</div>
      </div>
    </div>
  )
}

function NavLinks({ onNavigate }: { onNavigate?: () => void }) {
  const { t } = useI18n()
  const pathname = useRouterState({ select: (s) => s.location.pathname })
  const group = (title: string, items: NavItem[]) => (
    <div className="space-y-1">
      <div className="px-3 pb-1 text-[11px] font-medium uppercase tracking-wider text-muted-foreground">{t(title)}</div>
      {items.map((item) => {
        const active = item.to === '/' ? pathname === '/' : pathname.startsWith(item.to)
        return (
          <Link
            key={item.to}
            to={item.to}
            onClick={onNavigate}
            className={cn(
              'flex items-center gap-3 rounded-md px-3 py-2 text-sm transition-colors',
              active
                ? 'bg-sidebar-accent font-medium text-sidebar-accent-foreground'
                : 'text-muted-foreground hover:bg-sidebar-accent/60 hover:text-foreground',
            )}
          >
            {item.icon}
            {t(item.labelKey)}
          </Link>
        )
      })}
    </div>
  )
  return (
    <nav className="flex flex-col gap-6">
      {group('nav.section.manage', manage)}
      {group('nav.section.system', system)}
    </nav>
  )
}

function VersionBadge() {
  const { t } = useI18n()
  const { data } = useQuery({ queryKey: ['version'], queryFn: () => get<{ version?: string }>('/version'), staleTime: 600_000 })
  return (
    <div className="px-3 text-[11px] text-muted-foreground">
      {t('common.version')} {data?.version ?? '—'}
    </div>
  )
}

export function AppShell() {
  const { t, lang, setLang } = useI18n()
  const { theme, toggle } = useTheme()
  const { logout } = useAuth()
  const [mobileOpen, setMobileOpen] = useState(false)

  const sidebar = (onNavigate?: () => void) => (
    <div className="flex h-full flex-col gap-6 py-4">
      <Logo />
      <div className="flex-1 px-2">
        <NavLinks onNavigate={onNavigate} />
      </div>
      <VersionBadge />
    </div>
  )

  return (
    <div className="flex min-h-svh bg-background text-foreground">
      {/* Desktop sidebar */}
      <aside className="hidden w-60 shrink-0 border-r bg-sidebar text-sidebar-foreground md:block">{sidebar()}</aside>

      {/* Mobile drawer */}
      {mobileOpen && (
        <div className="fixed inset-0 z-40 md:hidden">
          <div className="absolute inset-0 bg-black/50" onClick={() => setMobileOpen(false)} />
          <aside className="absolute inset-y-0 left-0 w-64 bg-sidebar shadow-xl">
            <button className="absolute top-3 right-3 rounded p-1 hover:bg-sidebar-accent" onClick={() => setMobileOpen(false)}>
              <LuX className="size-4" />
            </button>
            {sidebar(() => setMobileOpen(false))}
          </aside>
        </div>
      )}

      <div className="flex min-w-0 flex-1 flex-col">
        <header className="sticky top-0 z-30 flex h-14 items-center gap-2 border-b bg-background/80 px-4 backdrop-blur">
          <Button variant="ghost" size="icon" className="md:hidden" onClick={() => setMobileOpen(true)}>
            <LuMenu className="size-4" />
          </Button>
          <div className="flex-1" />
          <Tooltip>
            <TooltipTrigger
              render={
                <Button variant="ghost" size="icon" onClick={() => window.open('/docs', '_blank')}>
                  <LuBookOpen className="size-4" />
                </Button>
              }
            />
            <TooltipContent>{t('header.docs')}</TooltipContent>
          </Tooltip>
          <Tooltip>
            <TooltipTrigger
              render={
                <Button variant="ghost" size="icon" onClick={() => setLang(lang === 'en' ? 'zh' : 'en')}>
                  <LuLanguages className="size-4" />
                </Button>
              }
            />
            <TooltipContent>{t('header.language')}: {lang === 'en' ? 'English' : '中文'}</TooltipContent>
          </Tooltip>
          <Tooltip>
            <TooltipTrigger
              render={
                <Button variant="ghost" size="icon" onClick={toggle}>
                  {theme === 'dark' ? <LuSun className="size-4" /> : <LuMoon className="size-4" />}
                </Button>
              }
            />
            <TooltipContent>{t('header.theme')}</TooltipContent>
          </Tooltip>
          <Tooltip>
            <TooltipTrigger
              render={
                <Button variant="ghost" size="icon" onClick={logout}>
                  <LuLogOut className="size-4" />
                </Button>
              }
            />
            <TooltipContent>{t('common.logout')}</TooltipContent>
          </Tooltip>
        </header>
        <main className="flex-1 p-4 md:p-6">
          <div className="mx-auto w-full max-w-7xl">
            <Outlet />
          </div>
        </main>
      </div>
    </div>
  )
}
