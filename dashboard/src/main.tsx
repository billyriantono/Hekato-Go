import { StrictMode } from 'react'
import { createRoot } from 'react-dom/client'
import { QueryClient, QueryClientProvider } from '@tanstack/react-query'
import { RouterProvider } from '@tanstack/react-router'
import { Toaster } from '@/components/ui/sonner'
import { AuthProvider, useAuth } from '@/lib/auth'
import { I18nProvider } from '@/lib/i18n'
import { ThemeProvider } from '@/lib/theme'
import { LoginPage, SetupPage } from '@/pages/login'
import { UsagePage } from '@/pages/usage'
import { DocsPage } from '@/pages/docs'
import { router } from '@/router'
import './index.css'

const queryClient = new QueryClient({
  defaultOptions: { queries: { retry: 1, refetchOnWindowFocus: false, staleTime: 10_000 } },
})

function Gate() {
  const { status } = useAuth()
  // Public self-service page: reachable without admin login.
  const publicPath = window.location.pathname.replace(/\/+$/, '')
  if (publicPath === '/usage') return <UsagePage />
  if (publicPath === '/docs' || publicPath.startsWith('/docs/')) return <DocsPage />
  if (status === 'loading') return <div className="flex min-h-svh items-center justify-center text-sm text-muted-foreground">…</div>
  if (status === 'setup') return <SetupPage />
  if (status === 'anonymous') return <LoginPage />
  return <RouterProvider router={router} />
}

createRoot(document.getElementById('root')!).render(
  <StrictMode>
    <ThemeProvider>
      <I18nProvider>
        <QueryClientProvider client={queryClient}>
          <AuthProvider>
            <Gate />
            <Toaster richColors position="top-right" />
          </AuthProvider>
        </QueryClientProvider>
      </I18nProvider>
    </ThemeProvider>
  </StrictMode>,
)
