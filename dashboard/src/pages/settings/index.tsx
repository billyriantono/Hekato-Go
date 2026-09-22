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

export function SettingsPage() {
  const { t } = useI18n()
  return (
    <div>
      <PageHeader title={t('settings.title')} description={t('settings.description')} />
      <div className="flex gap-6">
        <nav className="sticky top-20 hidden w-48 shrink-0 self-start md:block">
          <ul className="space-y-0.5">
            {NAV.map((n) => (
              <li key={n.id}>
                <button
                  type="button"
                  onClick={() => document.getElementById(n.id)?.scrollIntoView({ behavior: 'smooth', block: 'start' })}
                  className={cn(
                    'w-full rounded-md px-3 py-1.5 text-left text-sm text-muted-foreground transition-colors hover:bg-muted hover:text-foreground',
                    n.danger && 'hover:text-destructive',
                  )}
                >
                  {t(n.key)}
                </button>
              </li>
            ))}
          </ul>
        </nav>
        <div className="min-w-0 flex-1 space-y-6">
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
