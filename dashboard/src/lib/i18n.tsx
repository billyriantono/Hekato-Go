import { createContext, useCallback, useContext, useMemo, useState, type ReactNode } from 'react'
import en from '@/locales/en.json'
import zh from '@/locales/zh.json'

export type Lang = 'en' | 'zh'
// Page-scoped keys live in src/locales/extra/*.ts (each exports { en, zh }) and are merged here.
const extras = import.meta.glob<{ en: Record<string, string>; zh: Record<string, string> }>('@/locales/extra/*.ts', { eager: true })
const dict: Record<Lang, Record<string, string>> = { en: { ...en }, zh: { ...zh } }
for (const mod of Object.values(extras)) {
  Object.assign(dict.en, mod.en)
  Object.assign(dict.zh, mod.zh)
}
const LANG_KEY = 'kiro_lang'

type I18n = {
  lang: Lang
  setLang: (l: Lang) => void
  /** Translate `key`, substituting {0}, {1}... with args. Falls back to zh, then the key itself. */
  t: (key: string, ...args: (string | number)[]) => string
}

const Ctx = createContext<I18n | null>(null)

export function I18nProvider({ children }: { children: ReactNode }) {
  const [lang, setLangState] = useState<Lang>(() => (localStorage.getItem(LANG_KEY) as Lang) || 'en')
  const setLang = useCallback((l: Lang) => {
    localStorage.setItem(LANG_KEY, l)
    setLangState(l)
    document.documentElement.lang = l === 'zh' ? 'zh-CN' : 'en'
  }, [])
  const t = useCallback(
    (key: string, ...args: (string | number)[]) => {
      let text = dict[lang][key] ?? dict.zh[key] ?? key
      args.forEach((a, i) => {
        text = text.replace('{' + i + '}', String(a))
      })
      return text
    },
    [lang],
  )
  const value = useMemo(() => ({ lang, setLang, t }), [lang, setLang, t])
  return <Ctx.Provider value={value}>{children}</Ctx.Provider>
}

export function useI18n() {
  const v = useContext(Ctx)
  if (!v) throw new Error('useI18n outside I18nProvider')
  return v
}
