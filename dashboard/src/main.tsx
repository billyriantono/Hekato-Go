import { lazy, StrictMode, Suspense, useEffect } from 'react'
import { createRoot } from 'react-dom/client'
import { QueryClient, QueryClientProvider } from '@tanstack/react-query'
import { Splash } from '@/components/brand'
import { Toaster } from '@/components/ui/sonner'
import { AuthProvider, useAuth } from '@/lib/auth'
import { I18nProvider } from '@/lib/i18n'
import { ThemeProvider } from '@/lib/theme'
import { LoginPage, SetupPage } from '@/pages/login'
import './index.css'

// Only the auth gate and login ship in the entry chunk; everything behind the
// password — and the public pages — arrive as separate chunks.
const loadApp = () => import('./app')
const App = lazy(loadApp)
const UsagePage = lazy(() => import('@/pages/usage').then((m) => ({ default: m.UsagePage })))
const DocsPage = lazy(() => import('@/pages/docs').then((m) => ({ default: m.DocsPage })))

const queryClient = new QueryClient({
  defaultOptions: { queries: { retry: 1, refetchOnWindowFocus: false, staleTime: 10_000 } },
})

function Gate() {
  const { status } = useAuth()
  // While the operator types their password, fetch the console in the background.
  useEffect(() => {
    if (status === 'anonymous' || status === 'setup') void loadApp()
  }, [status])
  // Public self-service page: reachable without admin login.
  const publicPath = window.location.pathname.replace(/\/+$/, '')
  if (publicPath === '/usage') return <UsagePage />
  if (publicPath === '/docs' || publicPath.startsWith('/docs/')) return <DocsPage />
  if (status === 'loading') return <Splash />
  if (status === 'setup') return <SetupPage />
  if (status === 'anonymous') return <LoginPage />
  return <App />
}

createRoot(document.getElementById('root')!).render(
  <StrictMode>
    <ThemeProvider>
      <I18nProvider>
        <QueryClientProvider client={queryClient}>
          <AuthProvider>
            <Suspense fallback={<Splash />}>
              <Gate />
            </Suspense>
            <Toaster position="top-right" />
          </AuthProvider>
        </QueryClientProvider>
      </I18nProvider>
    </ThemeProvider>
  </StrictMode>,
)
