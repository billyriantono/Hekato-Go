// Public documentation site, served at /docs and /docs/<slug> without admin login.
import { useEffect, useMemo, useState, type ComponentType } from 'react'
import { LuLanguages, LuMoon, LuSun } from 'react-icons/lu'
import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'
import { useI18n } from '@/lib/i18n'
import { useTheme } from '@/lib/theme'
import { cn } from '@/lib/utils'
import { DocsCtx } from './content'
import { Endpoints, GettingStarted, Limits, Models, Troubleshooting } from './core'
import { Aider, ClaudeCode, ClineRoo, CodexCli, Continue, Cursor } from './guides/agents'
import { Curl, OpenWebUiLibreChat } from './guides/apps'
import { AnthropicSdk, LangChain, OpenAISdk } from './guides/sdks'

type Page = { slug: string; title: string; component: ComponentType }
type Group = { key: string; pages: Page[] }

const GROUPS: Group[] = [
  {
    key: 'docs.group.overview',
    pages: [
      { slug: 'getting-started', title: 'Getting started', component: GettingStarted },
      { slug: 'endpoints', title: 'Endpoints', component: Endpoints },
      { slug: 'models', title: 'Models & thinking', component: Models },
      { slug: 'limits', title: 'Limits & usage', component: Limits },
    ],
  },
  {
    key: 'docs.group.agents',
    pages: [
      { slug: 'claude-code', title: 'Claude Code', component: ClaudeCode },
      { slug: 'codex-cli', title: 'Codex CLI', component: CodexCli },
      { slug: 'cursor', title: 'Cursor', component: Cursor },
      { slug: 'cline-roo', title: 'Cline / Roo Code', component: ClineRoo },
      { slug: 'continue', title: 'Continue', component: Continue },
      { slug: 'aider', title: 'Aider', component: Aider },
    ],
  },
  {
    key: 'docs.group.sdks',
    pages: [
      { slug: 'openai-sdk', title: 'OpenAI SDK', component: OpenAISdk },
      { slug: 'anthropic-sdk', title: 'Anthropic SDK', component: AnthropicSdk },
      { slug: 'langchain', title: 'LangChain', component: LangChain },
    ],
  },
  {
    key: 'docs.group.apps',
    pages: [
      { slug: 'open-webui-librechat', title: 'Open WebUI / LibreChat', component: OpenWebUiLibreChat },
      { slug: 'curl', title: 'curl reference', component: Curl },
    ],
  },
  {
    key: 'docs.group.help',
    pages: [{ slug: 'troubleshooting', title: 'Troubleshooting', component: Troubleshooting }],
  },
]
const PAGES = GROUPS.flatMap((g) => g.pages)
const DEFAULT_SLUG = 'getting-started'
const KEY_STORAGE = 'hekato_docs_key'

function slugFromPath(): string {
  const m = window.location.pathname.match(/^\/docs\/?([^/]*)/)
  const s = m?.[1] ?? ''
  return PAGES.some((p) => p.slug === s) ? s : DEFAULT_SLUG
}

export function DocsPage() {
  const { t, lang, setLang } = useI18n()
  const { theme, toggle } = useTheme()
  const [slug, setSlug] = useState(slugFromPath)
  const [apiKey, setApiKey] = useState(() => sessionStorage.getItem(KEY_STORAGE) ?? '')
  const [toc, setToc] = useState<{ id: string; text: string; level: number }[]>([])
  const base = window.location.origin
  const page = PAGES.find((p) => p.slug === slug) ?? PAGES[0]

  const navigate = (s: string) => {
    if (s === slug) return
    history.pushState(null, '', '/docs/' + s)
    setSlug(s)
    window.scrollTo({ top: 0 })
  }

  useEffect(() => {
    const onPop = () => setSlug(slugFromPath())
    window.addEventListener('popstate', onPop)
    return () => window.removeEventListener('popstate', onPop)
  }, [])

  // Intercept same-origin /docs/<slug> links inside the article so they use pushState.
  useEffect(() => {
    const onClick = (e: MouseEvent) => {
      const a = (e.target as HTMLElement).closest('a[href^="/docs/"]')
      if (!a || e.metaKey || e.ctrlKey) return
      const [path, hash] = (a.getAttribute('href') ?? '').split('#')
      const s = path.replace('/docs/', '')
      if (!PAGES.some((p) => p.slug === s)) return
      e.preventDefault()
      navigate(s)
      if (hash) setTimeout(() => document.getElementById(hash)?.scrollIntoView(), 0)
    }
    document.addEventListener('click', onClick)
    return () => document.removeEventListener('click', onClick)
  })

  useEffect(() => {
    sessionStorage.setItem(KEY_STORAGE, apiKey)
  }, [apiKey])

  useEffect(() => {
    const hs = Array.from(document.querySelectorAll<HTMLElement>('article h2[id], article h3[id]'))
    setToc(hs.map((h) => ({ id: h.id, text: h.textContent ?? '', level: h.tagName === 'H2' ? 2 : 3 })))
    document.title = `${page.title} · Hekato Docs`
  }, [slug, page.title])

  const ctx = useMemo(() => ({ base, apiKey: apiKey.trim() }), [base, apiKey])
  const Body = page.component

  const nav = (
    <nav className="space-y-5 text-sm">
      {GROUPS.map((g) => (
        <div key={g.key}>
          <div className="mb-1.5 px-2 text-[11px] font-semibold uppercase tracking-wider text-muted-foreground">{t(g.key)}</div>
          <ul className="space-y-0.5">
            {g.pages.map((p) => (
              <li key={p.slug}>
                <a
                  href={'/docs/' + p.slug}
                  onClick={(e) => {
                    e.preventDefault()
                    navigate(p.slug)
                  }}
                  className={cn(
                    'block rounded-md px-2 py-1.5 transition-colors hover:bg-muted',
                    p.slug === slug ? 'bg-muted font-medium text-foreground' : 'text-muted-foreground',
                  )}
                >
                  {p.title}
                </a>
              </li>
            ))}
          </ul>
        </div>
      ))}
    </nav>
  )

  return (
    <DocsCtx.Provider value={ctx}>
      <div className="flex min-h-svh flex-col bg-background">
        <header className="sticky top-0 z-30 border-b bg-background/90 backdrop-blur">
          <div className="mx-auto flex h-14 max-w-screen-2xl items-center justify-between gap-3 px-4">
            <a href="/docs" className="flex items-center gap-2">
              <div className="flex size-8 items-center justify-center rounded-lg bg-primary text-sm font-bold text-primary-foreground">H</div>
              <span className="text-sm font-semibold">{t('docs.title')}</span>
            </a>
            <div className="flex items-center gap-1">
              <Button variant="ghost" size="sm" render={<a href="/usage" />}>
                {t('docs.usageLink')}
              </Button>
              <Button variant="ghost" size="sm" render={<a href="/admin" />}>
                {t('docs.adminLink')}
              </Button>
              <Button variant="ghost" size="icon" onClick={() => setLang(lang === 'en' ? 'zh' : 'en')} aria-label={t('header.language')}>
                <LuLanguages className="size-4" />
              </Button>
              <Button variant="ghost" size="icon" onClick={toggle} aria-label={t('header.theme')}>
                {theme === 'dark' ? <LuSun className="size-4" /> : <LuMoon className="size-4" />}
              </Button>
            </div>
          </div>
        </header>

        <div className="mx-auto flex w-full max-w-screen-2xl flex-1 gap-8 px-4">
          <aside className="sticky top-14 hidden h-[calc(100svh-3.5rem)] w-56 shrink-0 overflow-y-auto py-6 md:block">{nav}</aside>

          <main className="min-w-0 flex-1 py-6">
            <div className="mb-6 flex flex-col gap-3 rounded-lg border bg-muted/30 p-3 sm:flex-row sm:items-center">
              <label htmlFor="docs-key" className="shrink-0 text-sm font-medium">
                {t('docs.yourKey')}
              </label>
              <Input
                id="docs-key"
                type="password"
                autoComplete="off"
                placeholder={t('docs.keyPlaceholder')}
                value={apiKey}
                onChange={(e) => setApiKey(e.target.value)}
                className="font-mono"
              />
              <span className="shrink-0 text-xs text-muted-foreground">{t('docs.keyHint')}</span>
            </div>

            <select
              className="mb-6 h-9 w-full rounded-lg border bg-background px-2 text-sm md:hidden"
              value={slug}
              onChange={(e) => navigate(e.target.value)}
              aria-label={t('docs.navigation')}
            >
              {GROUPS.map((g) => (
                <optgroup key={g.key} label={t(g.key)}>
                  {g.pages.map((p) => (
                    <option key={p.slug} value={p.slug}>
                      {p.title}
                    </option>
                  ))}
                </optgroup>
              ))}
            </select>

            <article key={slug} className="mx-auto max-w-3xl text-[15px]">
              <Body />
            </article>
          </main>

          <aside className="sticky top-14 hidden h-[calc(100svh-3.5rem)] w-52 shrink-0 overflow-y-auto py-6 xl:block">
            {toc.length > 0 && (
              <>
                <div className="mb-2 text-[11px] font-semibold uppercase tracking-wider text-muted-foreground">{t('docs.onThisPage')}</div>
                <ul className="space-y-1 border-l text-[13px]">
                  {toc.map((h) => (
                    <li key={h.id}>
                      <a
                        href={'#' + h.id}
                        className={cn('block border-l-2 border-transparent py-0.5 text-muted-foreground hover:text-foreground', h.level === 2 ? 'pl-3' : 'pl-6')}
                      >
                        {h.text}
                      </a>
                    </li>
                  ))}
                </ul>
              </>
            )}
          </aside>
        </div>
      </div>
    </DocsCtx.Provider>
  )
}
