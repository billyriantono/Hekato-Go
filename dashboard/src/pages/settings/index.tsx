import { useEffect, useRef, useState } from 'react'
import { PageHeader } from '@/components/common'
import { useI18n } from '@/lib/i18n'
import { cn } from '@/lib/utils'
import { GeneralSection } from './general'
import { ThinkingSection } from './thinking'
import { EndpointSection } from './endpoint'
import { AutoRouteSection } from './auto-route'
import { ProxySection } from './proxy'
import { RelaySection } from './relay'
import { PromptFilterSection } from './prompt-filter'
import { DangerSection } from './danger'

const NAV: { id: string; key: string; danger?: boolean }[] = [
  { id: 'general', key: 'settings.general' },
  { id: 'password', key: 'settings.adminPassword' },
  { id: 'thinking', key: 'settings.thinkingSettings' },
  { id: 'endpoint', key: 'settings.endpointSettings' },
  { id: 'auto-route', key: 'settings.autoRoute.title' },
  { id: 'egress', key: 'settings.proxySettings' },
  { id: 'relay', key: 'relay.settings' },
  { id: 'prompt-filter', key: 'settings.promptFilter' },
  { id: 'danger', key: 'settings.dangerZone', danger: true },
]

/** Scroll-spy: the topmost section currently in view (in menu order). */
function useActiveSection() {
  const [active, setActive] = useState(NAV[0].id)
  const visible = useRef(new Set<string>())
  useEffect(() => {
    const io = new IntersectionObserver(
      (entries) => {
        for (const e of entries) e.isIntersecting ? visible.current.add(e.target.id) : visible.current.delete(e.target.id)
        const first = NAV.find((n) => visible.current.has(n.id))
        if (first) setActive(first.id)
      },
      { rootMargin: '-72px 0px -55% 0px' },
    )
    NAV.forEach((n) => {
      const el = document.getElementById(n.id)
      if (el) io.observe(el)
    })
    return () => io.disconnect()
  }, [])
  return [active, setActive] as const
}

export function SettingsPage() {
  const { t } = useI18n()
  const [active, setActive] = useActiveSection()
  const index = Math.max(0, NAV.findIndex((n) => n.id === active))
  return (
    <div>
      <PageHeader title={t('settings.title')} description={t('settings.description')} />
      <div className="flex gap-8">
        <nav className="sticky top-20 hidden w-52 shrink-0 self-start md:block">
          {/* A rail with one lit segment that glides to the section in view. */}
          <div className="relative pl-3">
            <span aria-hidden="true" className="absolute inset-y-0 left-0 w-px bg-border" />
            <span
              aria-hidden="true"
              className={cn(
                'absolute left-[-1px] h-8 w-[3px] rounded-full shadow-[0_0_10px_var(--signal-glow)] transition-[transform,background-color] duration-500 ease-(--ease-spring)',
                NAV[index].danger ? 'bg-destructive' : 'bg-signal',
              )}
              style={{ transform: `translateY(${index * 2}rem)` }}
            />
            <ul>
              {NAV.map((n) => (
                <li key={n.id}>
                  <button
                    type="button"
                    onClick={() => {
                      setActive(n.id)
                      document.getElementById(n.id)?.scrollIntoView({ behavior: 'smooth', block: 'start' })
                    }}
                    aria-current={n.id === active ? 'true' : undefined}
                    className={cn(
                      'flex h-8 w-full items-center rounded-md px-3 text-left text-[13px] transition-[color,transform] duration-300 ease-(--ease-out-expo) hover:translate-x-0.5',
                      n.id === active ? 'font-medium text-foreground' : 'text-muted-foreground hover:text-foreground',
                      n.danger && (n.id === active ? 'text-destructive' : 'hover:text-destructive'),
                    )}
                  >
                    {t(n.key)}
                  </button>
                </li>
              ))}
            </ul>
          </div>
        </nav>
        <div className="min-w-0 flex-1 space-y-6 [&_[id]]:scroll-mt-20">
          <GeneralSection />
          <ThinkingSection />
          <EndpointSection />
          <AutoRouteSection />
          <ProxySection />
          <RelaySection />
          <PromptFilterSection />
          <DangerSection />
        </div>
      </div>
    </div>
  )
}
