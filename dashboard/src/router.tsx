import { createRootRoute, createRoute, createRouter, Outlet } from '@tanstack/react-router'
import { AppShell } from '@/components/layout/app-shell'
import { OverviewPage } from '@/pages/overview'
import { AccountsPage } from '@/pages/accounts'
import { ApiKeysPage } from '@/pages/api-keys'
import { SettingsPage } from '@/pages/settings'
import { LogsPage } from '@/pages/logs'

const rootRoute = createRootRoute({ component: () => <Outlet /> })
const shellRoute = createRoute({ getParentRoute: () => rootRoute, id: 'shell', component: AppShell })

const routes = [
  createRoute({ getParentRoute: () => shellRoute, path: '/', component: OverviewPage }),
  createRoute({ getParentRoute: () => shellRoute, path: '/accounts', component: AccountsPage }),
  createRoute({ getParentRoute: () => shellRoute, path: '/api-keys', component: ApiKeysPage }),
  createRoute({ getParentRoute: () => shellRoute, path: '/settings', component: SettingsPage }),
  createRoute({ getParentRoute: () => shellRoute, path: '/logs', component: LogsPage }),
]

const routeTree = rootRoute.addChildren([shellRoute.addChildren(routes)])

// The Go server mounts the SPA at /admin/ and falls back to index.html for unknown paths.
export const router = createRouter({ routeTree, basepath: '/admin', defaultPreload: 'intent' })

declare module '@tanstack/react-router' {
  interface Register {
    router: typeof router
  }
}
