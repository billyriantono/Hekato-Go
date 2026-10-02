import { createRootRoute, createRoute, createRouter, lazyRouteComponent, Outlet } from '@tanstack/react-router'
import { AppShell } from '@/components/layout/app-shell'

// Every page is its own chunk: the first paint after login only pays for the
// shell and the page you landed on. The rest prefetch once the browser idles.
const pages = [
  ['/', () => import('@/pages/overview').then((m) => ({ default: m.OverviewPage }))],
  ['/accounts', () => import('@/pages/accounts').then((m) => ({ default: m.AccountsPage }))],
  ['/api-keys', () => import('@/pages/api-keys').then((m) => ({ default: m.ApiKeysPage }))],
  ['/settings', () => import('@/pages/settings').then((m) => ({ default: m.SettingsPage }))],
  ['/logs', () => import('@/pages/logs').then((m) => ({ default: m.LogsPage }))],
  ['/model-prices', () => import('@/pages/model-prices').then((m) => ({ default: m.ModelPricesPage }))],
] as const

function PagePending() {
  return (
    <div className="space-y-8 animate-[fade_0.4s_0.1s_var(--ease-out-expo)_both]" role="status" aria-label="Loading">
      <div className="space-y-3 border-b pb-6">
        <div className="skeleton h-2.5 w-24 rounded-full" />
        <div className="skeleton h-10 w-64 rounded-lg" />
        <div className="skeleton h-3 w-96 max-w-full rounded-full" />
      </div>
      <div className="grid grid-cols-2 gap-3 md:grid-cols-4">
        {[0, 1, 2, 3].map((i) => <div key={i} className="skeleton h-[92px] rounded-xl" />)}
      </div>
      <div className="skeleton h-72 rounded-xl" />
    </div>
  )
}

const rootRoute = createRootRoute({ component: () => <Outlet /> })
const shellRoute = createRoute({ getParentRoute: () => rootRoute, id: 'shell', component: AppShell })

const routes = pages.map(([path, load]) =>
  createRoute({ getParentRoute: () => shellRoute, path, component: lazyRouteComponent(load) }),
)

const routeTree = rootRoute.addChildren([shellRoute.addChildren(routes)])

// The Go server mounts the SPA at /admin/ and falls back to index.html for unknown paths.
export const router = createRouter({
  routeTree,
  basepath: '/admin',
  defaultPreload: 'intent',
  defaultPendingComponent: PagePending,
  defaultPendingMs: 150,
  defaultPendingMinMs: 250,
})

/** Warm every page chunk in the background — skipped when the user asks to save data. */
export function prefetchPages() {
  const conn = (navigator as Navigator & { connection?: { saveData?: boolean; effectiveType?: string } }).connection
  if (conn?.saveData || /2g/.test(conn?.effectiveType ?? '')) return
  const idle = window.requestIdleCallback ?? ((cb: () => void) => setTimeout(cb, 1500))
  idle(() => pages.forEach(([, load]) => load().catch(() => {})))
}

declare module '@tanstack/react-router' {
  interface Register {
    router: typeof router
  }
}
